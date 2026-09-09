// Package mofangfinance 为魔方财务（ZJMF）上游提供商的适配器实现。
//
// 协议严格参考魔方财务源码 `app/zjmf.php` 的 zjmfCurl()（财务系统作为下游调用上游）：
//
//   - 财务型上游（upstream_type=zjmf_api，即对方也是魔方财务）：
//     登录 POST {base}/zjmf_api_login（表单 username/password）→ {status:200, jwt}；
//     业务调用 {base}/{path}，请求头 `Authorization: Bearer {jwt}`。
//   - 资源型上游（upstream_type=resource，即魔方资源池/资源型对接）：
//     登录 POST {base}/resource_login（表单 username/password/type=agent）→ {status:200, jwt}。
//   - 响应 {status, msg, data}；status==405 表示 JWT 失效，强制重新登录并重试一次；
//     JWT 按上游默认缓存 90 分钟（5400 秒）。
//   - 业务请求为 application/x-www-form-urlencoded；GET 时参数拼 query。
//
// 已实现的业务路径（与源码中的调用点一一对应）：
//   - GET  cart/all                   上游商品列表（分组 + 商品）
//   - GET  cart/get_product_config    上游商品配置（pid）
//   - GET  api/product/proinfo        上游商品详情（pids）
//   - GET  host/header                下游拉取上游云主机信息（host_id + source=API）
//   - POST /dcim/on | /dcim/off | /dcim/reboot    独立服务器/裸金属电源操作（id + is_api=1）
//
// 开通/删除/升降配在「财务对接财务」模式下由下游下单 + 上游推送 /api/host/sync 完成，
// 不在本适配器直接实现，见 notSupported 提示。
package mofangfinance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// ProviderType 魔方财务提供商类型标识
const ProviderType = "mofangfinance"

// jwtTTL 与源码 zjmfApiLogin 一致：JWT 缓存 5400 秒。
const jwtTTL = 90 * time.Minute

func init() {
	upstream.GetProviderManager().RegisterFactory(ProviderType, func(cfg *upstream.ProviderConfig) upstream.Provider {
		return NewMoFangFinanceProvider(cfg)
	})
}

// MoFangFinanceProvider 魔方财务适配器
type MoFangFinanceProvider struct {
	config *upstream.ProviderConfig
	client *http.Client

	jwtMu    sync.Mutex
	jwt      string
	jwtUntil time.Time
}

// NewMoFangFinanceProvider 按配置创建魔方财务适配器实例
func NewMoFangFinanceProvider(config *upstream.ProviderConfig) *MoFangFinanceProvider {
	timeout := time.Duration(config.Timeout) * time.Second
	if config.Timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &MoFangFinanceProvider{
		config: config,
		client: &http.Client{Timeout: timeout},
	}
}

// GetType 返回提供商类型标识
func (p *MoFangFinanceProvider) GetType() string { return ProviderType }

// GetName 返回提供商名称
func (p *MoFangFinanceProvider) GetName() string { return "魔方财务" }

// isResource 是否资源型上游（is_resource=1，登录与业务路径不同）。
func (p *MoFangFinanceProvider) isResource() bool {
	return p.config.UpstreamType == "resource"
}

// baseURL 去掉协议前缀与尾部斜杠，并按配置拼接 https/端口（与财务 servers 表同构）。
func (p *MoFangFinanceProvider) baseURL() (string, error) {
	host := strings.TrimRight(p.config.APIEndpoint, "/")
	if host == "" {
		return "", &upstream.ProviderError{Op: "request", Msg: "接口地址为空"}
	}
	if !strings.Contains(host, "://") {
		scheme := "http"
		if p.config.Secure {
			scheme = "https"
		}
		host = scheme + "://" + host
	}
	if p.config.Port != "" && !strings.Contains(strings.TrimPrefix(host, "https://"), ":") {
		host = host + ":" + strings.TrimPrefix(p.config.Port, ":")
	}
	return strings.TrimRight(host, "/"), nil
}

// loginURL 登录地址：财务型 /zjmf_api_login，资源型 /resource_login（type=agent）。
func (p *MoFangFinanceProvider) loginURL(base string) string {
	if p.isResource() {
		return base + "/resource_login"
	}
	return base + "/zjmf_api_login"
}

// login 获取上游 JWT，带本地缓存；force 时强制重新登录（对应 zjmfApiLogin）。
func (p *MoFangFinanceProvider) login(ctx context.Context, force bool) (string, error) {
	p.jwtMu.Lock()
	defer p.jwtMu.Unlock()
	if !force && p.jwt != "" && time.Now().Before(p.jwtUntil) {
		return p.jwt, nil
	}
	base, err := p.baseURL()
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("username", p.config.APIKey)
	form.Set("password", p.config.APISecret)
	if p.isResource() {
		form.Set("type", "agent")
	}
	body, status, err := p.httpForm(ctx, http.MethodPost, p.loginURL(base), form, "")
	if err != nil {
		return "", err
	}
	var shell ZjmfResp[json.RawMessage]
	_ = json.Unmarshal(body, &shell)
	if status != http.StatusOK || (shell.Status != StatusOK && shell.Code != StatusOK) {
		return "", &upstream.ProviderError{
			Op:         "login",
			StatusCode: status,
			Msg:        firstNonEmpty(shell.Msg, fmt.Sprintf("上游登录失败(HTTP %d)，请检查接口地址、用户名与密码", status)),
		}
	}
	if shell.JWT == "" {
		return "", &upstream.ProviderError{Op: "login", StatusCode: status, Msg: "上游登录成功但未返回 jwt"}
	}
	p.jwt = shell.JWT
	p.jwtUntil = time.Now().Add(jwtTTL)
	return p.jwt, nil
}

// httpForm 发送表单请求；bearer 非空时附加 Authorization 头。返回响应体与 HTTP 状态码。
func (p *MoFangFinanceProvider) httpForm(ctx context.Context, method, rawURL string, form url.Values, bearer string) ([]byte, int, error) {
	var body io.Reader
	if form != nil && method != http.MethodGet {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, 0, &upstream.ProviderError{Op: "request", Err: err}
	}
	if form != nil && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, &upstream.ProviderError{Op: "request", Err: err}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, &upstream.ProviderError{Op: "request", Err: err}
	}
	return respBody, resp.StatusCode, nil
}

// call 调用上游业务接口（zjmfCurl 等价实现）：
// 自动登录 → 业务请求 → status==405 时强制重登录重试一次 → 解析统一响应壳。
// out 非 nil 时将 data 字段解析到 out。
func (p *MoFangFinanceProvider) call(ctx context.Context, op, method, path string, form url.Values, out interface{}) error {
	base, err := p.baseURL()
	if err != nil {
		return err
	}
	jwt, err := p.login(ctx, false)
	if err != nil {
		return &upstream.ProviderError{Op: op, Err: err}
	}
	body, status, err := p.callOnce(ctx, base, jwt, method, path, form)
	if err != nil {
		return &upstream.ProviderError{Op: op, Err: err}
	}
	var shell ZjmfResp[json.RawMessage]
	_ = json.Unmarshal(body, &shell)
	// JWT 失效：重登录后重试一次（zjmfCurl 逻辑）；仍 405 则判定账号密码错误。
	if shell.Status == StatusTokenExpired {
		jwt, err = p.login(ctx, true)
		if err != nil {
			return &upstream.ProviderError{Op: op, Err: err}
		}
		body, status, err = p.callOnce(ctx, base, jwt, method, path, form)
		if err != nil {
			return &upstream.ProviderError{Op: op, Err: err}
		}
		shell = ZjmfResp[json.RawMessage]{}
		_ = json.Unmarshal(body, &shell)
		if shell.Status == StatusTokenExpired {
			return &upstream.ProviderError{Op: op, Msg: "API账号密码错误"}
		}
	}
	if status >= 400 {
		return &upstream.ProviderError{Op: op, StatusCode: status, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
	}
	if shell.Status != StatusOK && shell.Code != StatusOK && (shell.Status != 0 || shell.Code != 0) {
		return &upstream.ProviderError{Op: op, Code: shell.Status, Msg: firstNonEmpty(shell.Msg, "上游接口调用失败")}
	}
	if out != nil && len(shell.Data) > 0 && string(shell.Data) != "null" {
		if err := json.Unmarshal(shell.Data, out); err != nil {
			return &upstream.ProviderError{Op: op, Err: fmt.Errorf("解析上游响应 data 失败: %w", err)}
		}
	}
	return nil
}

// callOnce 以给定 JWT 执行一次业务调用。GET 的表单参数转 query，路径可带前导斜杠。
func (p *MoFangFinanceProvider) callOnce(ctx context.Context, base, jwt, method, path string, form url.Values) ([]byte, int, error) {
	rawURL := base + "/" + strings.TrimLeft(path, "/")
	method = strings.ToUpper(method)
	if method == http.MethodGet && form != nil {
		if enc := form.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		form = nil
	}
	return p.httpForm(ctx, method, rawURL, form, jwt)
}

// HealthCheck 连通性测试：登录成功即视为可达且凭证有效。
func (p *MoFangFinanceProvider) HealthCheck(ctx context.Context) error {
	_, err := p.login(ctx, true)
	return err
}

// ListProducts 拉取上游商品列表并补全价格/规格/分类（全量商品接口）。
// 财务型上游：
//  1. GET api/product/proinfo            —— 全量商品 id（不过滤 hidden/retired）；
//  2. GET api/product/prodetail?pids[]=   —— 分批(<500)拉取完整详情：product_pricings（售价）+ config_groups（CPU/内存/系统盘等开通规格）。
//
// 这解决了此前 v1/products 仅返回前台可见商品（显示不全）、v1/productsconfig 对部分商品拿不到配置（参数缺失）的问题。
// 资源型上游：GET resource/agentproductsarray（data 兼容数组/包裹两种形态）。
func (p *MoFangFinanceProvider) ListProducts(ctx context.Context) ([]*model.StandardProduct, error) {
	products := make([]*model.StandardProduct, 0, 16)
	if p.isResource() {
		var raw json.RawMessage
		if err := p.call(ctx, "ListProducts", http.MethodGet, "resource/agentproductsarray", nil, &raw); err != nil {
			return nil, err
		}
		rows := flattenResourceProducts(raw)
		for _, r := range rows {
			id := jsonStr(r, "id")
			if id == "" {
				continue
			}
			products = append(products, &model.StandardProduct{
				ProviderType: ProviderType, UpstreamID: id, Name: jsonStr(r, "name"), Status: "active", RawSpecs: r,
			})
		}
		return products, nil
	}

	// 1) 全量商品 id
	var proinfo OpenapiProinfoResp
	if err := p.call(ctx, "ListProducts", http.MethodGet, "api/product/proinfo", nil, &proinfo); err != nil {
		return nil, err
	}
	// 2) 分批拉取完整详情（价格 + 配置规格）
	details := p.fetchAllDetails(ctx, proinfo.Info)
	for pid, d := range details {
		spec := parseCloudSpecs(d.ConfigGroups)
		if spec.OS == "" {
			spec.OS = d.Type
		}
		price := upstreamPrice(d.ProductPricings)
		products = append(products, &model.StandardProduct{
			ProviderType: ProviderType, UpstreamID: pid, Name: d.Name,
			CostPrice: price, SalePrice: price, Specs: spec, Status: "active",
			RawSpecs: map[string]interface{}{
				"gid": d.GID, "type": d.Type, "description": d.Description,
				"product_pricings": d.ProductPricings, "config_groups": d.ConfigGroups,
			},
		})
	}
	return products, nil
}

// detailBatchSize prodetail 单次最大商品数（上游 concurrent=500）。
const detailBatchSize = 200

// fetchAllDetails 分批拉取全量商品详情，返回 pid → 详情。
func (p *MoFangFinanceProvider) fetchAllDetails(ctx context.Context, infos []struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}) map[string]UpstreamProductDetail {
	out := map[string]UpstreamProductDetail{}
	for start := 0; start < len(infos); start += detailBatchSize {
		end := start + detailBatchSize
		if end > len(infos) {
			end = len(infos)
		}
		form := url.Values{}
		for _, it := range infos[start:end] {
			form.Add("pids[]", strconv.FormatInt(it.ID, 10))
		}
		var resp OpenapiProdetailResp
		if err := p.call(ctx, "ListProducts", http.MethodGet, "api/product/prodetail", form, &resp); err != nil {
			continue
		}
		for pid, d := range resp.Detail {
			out[pid] = d
		}
	}
	return out
}

// upstreamPrice 取上游售价：优先 monthly，其次 quarterly/annually/onetime。
func upstreamPrice(prs []ProductPricing) float64 {
	if len(prs) == 0 {
		return 0
	}
	pr := prs[0]
	switch {
	case float64(pr.Monthly) > 0:
		return float64(pr.Monthly)
	case float64(pr.Quarterly) > 0:
		return float64(pr.Quarterly)
	case float64(pr.Annually) > 0:
		return float64(pr.Annually)
	case float64(pr.Onetime) > 0:
		return float64(pr.Onetime)
	}
	return 0
}

// parseCloudSpecs 从 prodetail 的 config_groups（嵌套：分组→配置项→子值）提取标准规格。
func parseCloudSpecs(groups []ConfigGroup) model.StandardProductSpec {
	var spec model.StandardProductSpec
	for _, g := range groups {
		for _, opt := range g.Options {
			val := firstSubValue(opt)
			if val == "" {
				continue
			}
			key := normalizeOptionKey(opt.OptionName)
			switch {
			case matchKey(key, "cpu", "核"):
				spec.CPU = leadingInt(val)
			case matchKey(key, "memory", "mem", "内存"):
				spec.Memory = memoryToMB(val)
			case matchKey(key, "system_disk_size", "systemdisk", "disk", "系统盘", "磁盘", "硬盘"):
				spec.Disk = leadingInt(val)
			case matchKey(key, "os", "操作系统", "镜像", "系统"):
				spec.OS = val
			case matchKey(key, "area", "数据中心", "区域"):
				spec.Region = val
			case matchKey(key, "node", "zone", "节点"):
				spec.Zone = val
			case matchKey(key, "bw", "bandwidth", "带宽"):
				spec.Bandwidth = leadingInt(val)
			case matchKey(key, "disk_type", "disktype", "磁盘类型"):
				spec.DiskType = val
			}
		}
	}
	return spec
}

// memoryToMB 将内存值统一换算为 MB：值含"G/GB"单位时 ×1024，否则视为 MB 原值。
// 上游配置项内存可能以 GB 表示（如 "16G"）或以 MB 表示（如 "16384"）。
// memoryToMB 内存值解析：上游配置值形如 "8192|8G"，数字部分已是 MB，直接取前导数字即可。
func memoryToMB(v string) int {
	n := leadingInt(v)
	if n == 0 {
		return 0
	}
	return n
}

// firstSubValue 取配置项第一子值（option_name），无子项回退 option_name 本身。
func firstSubValue(opt ConfigOption) string {
	for _, sub := range opt.Sub {
		if v := strings.TrimSpace(sub.OptionName); v != "" {
			return v
		}
	}
	return strings.TrimSpace(opt.OptionName)
}

// normalizeOptionKey 归一化配置键：转小写、去空格与说明分隔符。
func normalizeOptionKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "（", "(")
	s = strings.ReplaceAll(s, "）", ")")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// matchKey 判断归一化后的配置键是否命中任一候选（自身相等或包含候选词）。
func matchKey(key string, cands ...string) bool {
	for _, c := range cands {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if key == c || strings.Contains(key, c) {
			return true
		}
	}
	return false
}

// num 从 interface{} 读取 float64（兼容 json.Number/float64/string）。
func num(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	case int:
		return float64(t), true
	case json.RawMessage:
		// 上游价格字段可能是字符串或数字；先按浮点解析，失败再按字符串解析
		var f float64
		if err := json.Unmarshal(t, &f); err == nil {
			return f, true
		}
		var s string
		if err := json.Unmarshal(t, &s); err == nil {
			ff, perr := strconv.ParseFloat(strings.TrimSpace(s), 64)
			return ff, perr == nil
		}
	}
	return 0, false
}

// str 从 interface{} 读取字符串。
func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// leadingInt 解析字符串开头的整数（处理 "8192"/"2 核"/"30,1" 等）。
func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	end := 0
	for end < len(s) {
		c := s[end]
		if (c < '0' || c > '9') && c != '.' {
			break
		}
		end++
	}
	n, err := strconv.Atoi(strings.TrimSuffix(s[:end], "."))
	if err != nil {
		return 0
	}
	return n
}

// GetProduct 拉取单个上游商品配置（v1/productsconfig，product_id）。
func (p *MoFangFinanceProvider) GetProduct(ctx context.Context, upstreamID string) (*model.StandardProduct, error) {
	if p.isResource() {
		return nil, p.notSupported("GetProduct", "资源型上游请使用 ListProducts 获取商品列表")
	}
	var resp OpenapiProdetailResp
	if err := p.call(ctx, "GetProduct", http.MethodGet, "api/product/prodetail", url.Values{
		"pids[]": {upstreamID},
	}, &resp); err != nil {
		return nil, err
	}
	d, ok := resp.Detail[upstreamID]
	if !ok {
		return nil, &upstream.ProviderError{Op: "GetProduct", Msg: "上游商品不存在"}
	}
	spec := parseCloudSpecs(d.ConfigGroups)
	if spec.OS == "" {
		spec.OS = d.Type
	}
	return &model.StandardProduct{
		ProviderType: ProviderType,
		UpstreamID:   upstreamID,
		Name:         d.Name,
		Specs:        spec,
		CostPrice:    upstreamPrice(d.ProductPricings),
		SalePrice:    upstreamPrice(d.ProductPricings),
		RawSpecs: map[string]interface{}{
			"gid":              d.GID,
			"type":             d.Type,
			"description":      d.Description,
			"product_pricings": d.ProductPricings,
			"config_groups":    d.ConfigGroups,
		},
		Status: "active",
	}, nil
}

// CreateInstance 「财务对接财务」模式下开通需由下游向上游下单（cart 结算流程），
// 上游开通后主动推送 /api/host/sync 同步主机，不适用单次 REST 调用。
func (p *MoFangFinanceProvider) CreateInstance(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
	return nil, p.notSupported("CreateInstance", "财务型上游开通需走下游下单+上游推送同步流程，暂不支持直接开通")
}

// ListInstances 暂不支持：上游主机列表依赖资源型 resource/agenthosts 或下游订单映射。
func (p *MoFangFinanceProvider) ListInstances(ctx context.Context, filters map[string]string) ([]*model.StandardInstance, error) {
	return nil, p.notSupported("ListInstances", "请通过订单/主机模块按 host_id 逐台查询（host/header）")
}

// GetInstance 拉取上游云主机信息：GET host/header（host_id + source=API）。
func (p *MoFangFinanceProvider) GetInstance(ctx context.Context, instanceID string) (*model.StandardInstance, error) {
	var header HostHeader
	if err := p.call(ctx, "GetInstance", http.MethodGet, "host/header", url.Values{
		"host_id": {instanceID},
		"source":  {"API"},
	}, &header); err != nil {
		return nil, err
	}
	h := header.HostData
	inst := &model.StandardInstance{
		ProviderType: ProviderType,
		UpstreamID:   instanceID,
		Name:         h.Domain,
		PublicIP:     h.DedicatedIP,
		RawData: map[string]interface{}{
			"assignedips":   h.AssignedIPs,
			"port":          h.Port,
			"os":            h.OS,
			"productid":     h.ProductID,
			"nextduedate":   h.NextDueDate,
			"upstream_cost": h.UpstreamCost,
		},
	}
	switch strings.ToLower(h.DomainStatus) {
	case "active":
		inst.Status = model.InstanceStatusRunning
	case "suspended", "pending":
		inst.Status = model.InstanceStatusStopped
	default:
		inst.Status = model.InstanceStatusStopped
	}
	return inst, nil
}

// StartInstance 开机：POST /dcim/on（id + is_api=1，与源码 Dcim::on 的 zjmf_api 分支一致）。
func (p *MoFangFinanceProvider) StartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "StartInstance", http.MethodPost, "/dcim/on", url.Values{
		"id": {instanceID}, "is_api": {"1"},
	}, nil)
}

// StopInstance 关机：POST /dcim/off。
func (p *MoFangFinanceProvider) StopInstance(ctx context.Context, instanceID string, force bool) error {
	return p.call(ctx, "StopInstance", http.MethodPost, "/dcim/off", url.Values{
		"id": {instanceID}, "is_api": {"1"},
	}, nil)
}

// RestartInstance 重启：POST /dcim/reboot。
func (p *MoFangFinanceProvider) RestartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "RestartInstance", http.MethodPost, "/dcim/reboot", url.Values{
		"id": {instanceID}, "is_api": {"1"},
	}, nil)
}

// DeleteInstance 财务型上游不提供直接删除主机接口（由上游订单/推送流程管理）。
func (p *MoFangFinanceProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	return p.notSupported("DeleteInstance", "财务型上游由上游订单流程管理主机删除，不提供直接删除接口")
}

// ResizeInstance 财务型上游不提供直接升降配接口（走上游升降配订单）。
func (p *MoFangFinanceProvider) ResizeInstance(ctx context.Context, instanceID string, specs *model.StandardProductSpec) error {
	return p.notSupported("ResizeInstance", "财务型上游升降配需在上游创建升级订单，不提供直接接口")
}

// ListPools 魔方财务上游无资源池概念。
func (p *MoFangFinanceProvider) ListPools(ctx context.Context) ([]*upstream.StandardPool, error) {
	return nil, p.notSupported("ListPools", "魔方财务为账单系统，无资源池概念")
}

// GetAccountInfo 魔方财务上游无账户资源统计接口。
func (p *MoFangFinanceProvider) GetAccountInfo(ctx context.Context) (*upstream.AccountInfo, error) {
	return nil, p.notSupported("GetAccountInfo", "魔方财务上游不提供账户资源统计")
}

// notSupported 返回能力不支持错误。
func (p *MoFangFinanceProvider) notSupported(op, msg string) error {
	return &upstream.ProviderError{Op: op, Msg: msg}
}

// flattenResourceProducts 兼容资源型上游商品数组的两种形态：[...] 或 {data:[...]}。
func flattenResourceProducts(raw json.RawMessage) []map[string]interface{} {
	var arr []map[string]interface{}
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	var wrapped struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		return wrapped.Data
	}
	return nil
}

// jsonStr 从 map 取字符串值（兼容 json.Number / float64）。
func jsonStr(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
