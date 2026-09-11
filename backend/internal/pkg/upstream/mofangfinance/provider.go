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
//   - POST /host/renew               续费生成账单（hostid + billingcycles）
//   - POST /apply_credit             余额支付账单（invoiceid + use_credit=1，status 1001=完成）
//   - POST /host/cancel              终止主机（id + type=Immediate + reason）
//   - POST /provision/default        电源/暂停/恢复/控制台（id + func=on|off|reboot|hard_off|hard_reboot|suspend|unsuspend|vnc）
//
// 开通直连走 cart/add_to_shop → cart/settle → apply_credit（见 CreateInstance）；
// 资源型上游（upstream_type=resource）的开通/续费不在本适配器直接实现，见 notSupported 提示。
package mofangfinance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
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
	// 能力描述符（T2.1）：财务型上游目录权威在上游，开通/续费/暂停/终止按 zjmf_api 分支直连。
	upstream.RegisterDescriptor(ProviderType, upstream.CapabilityDescriptor{
		Kind:          upstream.KindUpstream,
		SyncScopes:    []string{upstream.ScopeCatalog, upstream.ScopePrice, upstream.ScopeInstance},
		BillingCycles: []string{"monthly", "quarterly", "semiannually", "annually", "biennially", "triennially"},
		Operations: []string{
			upstream.OpProvision, upstream.OpRenew,
			upstream.OpStart, upstream.OpStop, upstream.OpRestart, upstream.OpVNC,
			upstream.OpSuspend, upstream.OpUnsuspend, upstream.OpDestroy,
		},
		// zjmf_api 分支：续费走 /host/renew + /apply_credit（上游下单），终止走 /host/cancel（立即生效）。
		RenewMode:   upstream.RenewModeOrder,
		DestroyMode: upstream.DestroyModeImmediate,
		SignerType:  upstream.SignerBearer,
		CredentialSchema: []upstream.Field{
			{Key: "api_key", Label: "用户名", Type: upstream.FieldTypeString, Required: true,
				Placeholder: "上游 API 用户名"},
			{Key: "api_secret", Label: "API 密钥", Type: upstream.FieldTypePassword, Required: true, Secret: true,
				Placeholder: "上游 API 密码"},
			{Key: "upstream_type", Label: "接口类型", Type: upstream.FieldTypeSelect, Required: false,
				Default: "zjmf_api", Options: []upstream.FieldOption{
					{Label: "智简魔方", Value: "zjmf_api"},
					{Label: "资源型", Value: "resource"},
				}},
		},
		EndpointSchema: []upstream.Field{
			{Key: "api_endpoint", Label: "接口地址", Type: upstream.FieldTypeString, Required: true,
				Placeholder: "https://上游财务系统地址"},
			{Key: "port", Label: "接口端口", Type: upstream.FieldTypeString, Required: false},
			{Key: "secure", Label: "使用 HTTPS", Type: upstream.FieldTypeBool, Required: false},
		},
		RateLimit:      upstream.RateLimitSpec{QPS: 3, Burst: 6},
		SupportsPaging: false,
		FieldDictionary: map[string]any{
			"source": "实测（依据 mofangfinance/provider.go 与上游源码 app/zjmf.php、app/common/logic/Host.php）",
			"paths": []string{"cart/all", "cart/get_product_config", "api/product/proinfo",
				"host/header", "/dcim/on", "/dcim/off", "/dcim/reboot",
				"/host/renew", "/apply_credit", "/host/cancel", "/provision/default"},
			"note": "登录换 JWT（Bearer），status=405 表示 JWT 失效需重登；续费=host/renew+apply_credit(1001)；暂停/恢复=provision/default func=suspend|unsuspend；终止=host/cancel type=Immediate",
		},
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

// Capabilities 返回能力描述符（T2.1）。
func (p *MoFangFinanceProvider) Capabilities() upstream.CapabilityDescriptor {
	d, _ := upstream.Descriptor(ProviderType)
	return d
}

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
// 成功状态码限定 200（业务通用成功码）；需要额外状态码（如支付成功的 1001）的接口
// 请改用 callShell 自行判定。
func (p *MoFangFinanceProvider) call(ctx context.Context, op, method, path string, form url.Values, out interface{}) error {
	shell, status, body, err := p.callShell(ctx, op, method, path, form)
	if err != nil {
		return err
	}
	if status >= 400 {
		return &upstream.ProviderError{Op: op, StatusCode: status, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
	}
	if shell.Status != StatusOK && shell.Code != StatusOK && (shell.Status != 0 || shell.Code != 0) {
		return &upstream.ProviderError{Op: op, Code: shell.Status, Msg: firstNonEmpty(shell.Msg, "上游接口调用失败")}
	}
	return p.unmarshalData(op, shell, out)
}

// callShell 执行一次上游业务调用并返回原始响应壳，不做业务状态码判定。
// 供 call（判定 200）与 applyCredit（额外接受 1001 支付成功码）复用。
func (p *MoFangFinanceProvider) callShell(ctx context.Context, op, method, path string, form url.Values) (ZjmfResp[json.RawMessage], int, []byte, error) {
	var shell ZjmfResp[json.RawMessage]
	base, err := p.baseURL()
	if err != nil {
		return shell, 0, nil, err
	}
	jwt, err := p.login(ctx, false)
	if err != nil {
		return shell, 0, nil, &upstream.ProviderError{Op: op, Err: err}
	}
	body, status, err := p.callOnce(ctx, base, jwt, method, path, form)
	if err != nil {
		return shell, status, body, &upstream.ProviderError{Op: op, Err: err}
	}
	_ = json.Unmarshal(body, &shell)
	// JWT 失效：重登录后重试一次（zjmfCurl 逻辑）；仍 405 则判定账号密码错误。
	if shell.Status == StatusTokenExpired {
		jwt, err = p.login(ctx, true)
		if err != nil {
			return shell, status, body, &upstream.ProviderError{Op: op, Err: err}
		}
		body, status, err = p.callOnce(ctx, base, jwt, method, path, form)
		if err != nil {
			return shell, status, body, &upstream.ProviderError{Op: op, Err: err}
		}
		shell = ZjmfResp[json.RawMessage]{}
		_ = json.Unmarshal(body, &shell)
		if shell.Status == StatusTokenExpired {
			return shell, status, body, &upstream.ProviderError{Op: op, Msg: "API账号密码错误"}
		}
	}
	return shell, status, body, nil
}

// unmarshalData 将响应壳的 data 字段解析到 out。
func (p *MoFangFinanceProvider) unmarshalData(op string, shell ZjmfResp[json.RawMessage], out interface{}) error {
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
				spec.OS = normalizeOS(val)
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

// num 从 interface{} 读取 float64（兼容 json.Number/float64/string 与各整型）。
// 注意必须覆盖 int64：host/header 的 nextduedate 经 HostHeader 解析后即为 int64，
// 早先漏掉整型会让链路 A 续费拿不到上游权威账期而静默回退本地顺延。
func num(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	case int:
		return float64(t), true
	case int8:
		return float64(t), true
	case int16:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint8:
		return float64(t), true
	case uint16:
		return float64(t), true
	case uint32:
		return float64(t), true
	case uint64:
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

// CreateInstance 「财务对接财务」开通：由本系统（下游）向上游魔方财务下单。
// 等价于源码 Host::createAccount 的 zjmf_api 分支：
//  1. cart/add_to_shop 将商品加入上游购物车；
//  2. cart/settle      结算生成账单（invoiceid，主机为待开通 Pending）；
//  3. apply_credit     用上游 API 账户余额支付账单（status 1001=支付成功并开通），返回 hostid；
//  4. host/header      按 hostid 拉取主机信息映射标准实例。
//
// 账单金额自上而下扣减上游 API 账户余额，需确保该账户有足够余额。
func (p *MoFangFinanceProvider) CreateInstance(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
	if p.isResource() {
		return nil, p.notSupported("CreateInstance", "资源型上游请走资源池开通流程处理")
	}
	pid := firstNonEmpty(strFromMap(req.Extra, "upstream_pid"), strFromMap(req.Extra, "pid"))
	if pid == "" {
		return nil, &upstream.ProviderError{Op: "CreateInstance", Msg: "商品未绑定上游商品ID（upstream_pid），无法在财务上游下单"}
	}
	host := safeHostname(req.Name)
	password := firstNonEmpty(req.Password, randPassword())
	billingCycle := normalizeBillingCycle(req.BillingMode)
	configOpt := buildCartConfigOption(req.Extra)

	// 1) 加入购物车
	addForm := url.Values{}
	addForm.Set("pid", pid)
	addForm.Set("billingcycle", billingCycle)
	addForm.Set("host", host)
	addForm.Set("password", password)
	addForm.Set("currencyid", "1")
	addForm.Set("qty", "1")
	for k, v := range configOpt {
		addForm.Set("configoption["+k+"]", v)
	}
	if err := p.call(ctx, "CreateInstance", http.MethodPost, "cart/add_to_shop", addForm, nil); err != nil {
		return nil, err
	}

	// 2) 结算生成账单
	settleForm := url.Values{}
	settleForm.Set("downstream_url", "")
	settleForm.Set("downstream_token", "")
	settleForm.Set("downstream_id", "")
	settleForm.Set("cart_data[pid]", pid)
	settleForm.Set("cart_data[billingcycle]", billingCycle)
	settleForm.Set("cart_data[host]", host)
	settleForm.Set("cart_data[password]", password)
	settleForm.Set("cart_data[currencyid]", "1")
	settleForm.Set("cart_data[qty]", "1")
	for k, v := range configOpt {
		settleForm.Set("cart_data[configoptions]["+k+"]", v)
	}
	var settleData struct {
		InvoiceID int64   `json:"invoiceid"`
		HostID    []int64 `json:"hostid"`
	}
	if err := p.call(ctx, "CreateInstance", http.MethodPost, "cart/settle", settleForm, &settleData); err != nil {
		return nil, err
	}
	if settleData.InvoiceID == 0 {
		return nil, &upstream.ProviderError{Op: "CreateInstance", Msg: "上游结算成功但未返回账单ID"}
	}

	// 3) 用上游余额支付并开通（status 1001 支付完成）
	hostID := firstInt(settleData.HostID)
	if hostID != 0 {
		// 结算即返回主机ID（部分即时开通）则无需再支付后取号，直接拉详情。
	} else {
		payForm := url.Values{}
		payForm.Set("invoiceid", strconv.FormatInt(settleData.InvoiceID, 10))
		payForm.Set("use_credit", "1")
		payForm.Set("enough", "1")
		payForm.Set("downstream_url", "")
		payForm.Set("downstream_token", "")
		payForm.Set("downstream_id", "")
		shell, httpStatus, body, err := p.callShell(ctx, "CreateInstance", http.MethodPost, "apply_credit", payForm)
		if err != nil {
			return nil, err
		}
		if httpStatus >= 400 {
			return nil, &upstream.ProviderError{Op: "CreateInstance", StatusCode: httpStatus, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
		}
		if shell.Status != StatusPaidSuccess && shell.Status != StatusOK && shell.Code != StatusOK {
			return nil, &upstream.ProviderError{Op: "CreateInstance", Code: shell.Status, Msg: firstNonEmpty(shell.Msg, "上游余额支付失败")}
		}
		var payData struct {
			HostID []int64 `json:"hostid"`
		}
		if err := p.unmarshalData("CreateInstance", shell, &payData); err != nil {
			return nil, err
		}
		hostID = firstInt(payData.HostID)
		if hostID == 0 {
			return nil, &upstream.ProviderError{Op: "CreateInstance", Msg: "上游支付成功但未返回主机ID"}
		}
	}

	// 4) 拉取主机信息并映射标准实例（短暂轮询等待主机脱离创建态）
	inst, err := p.waitInstanceReady(ctx, strconv.FormatInt(hostID, 10), 6)
	if err != nil {
		// 开通成功但详情拉取失败：返回最小可用实例，避免回滚已开通资源。
		return &model.StandardInstance{
			ProviderType: ProviderType,
			ProviderID:   p.config.ID,
			UpstreamID:   strconv.FormatInt(hostID, 10),
			Name:         host,
			Status:       model.InstanceStatusCreating,
			Specs:        specsFromExtra(req.Extra),
		}, nil
	}
	inst.Specs = specsFromExtra(req.Extra)
	return inst, nil
}

// waitInstanceReady 轮询上游主机信息直到脱离创建态（或达到最大尝试次数），返回最新实例。
func (p *MoFangFinanceProvider) waitInstanceReady(ctx context.Context, instanceID string, tries int) (*model.StandardInstance, error) {
	var inst *model.StandardInstance
	var err error
	for i := 0; i < tries; i++ {
		inst, err = p.GetInstance(ctx, instanceID)
		if err == nil && inst.Status != model.InstanceStatusCreating {
			return inst, nil
		}
		select {
		case <-ctx.Done():
			return inst, err
		case <-time.After(2 * time.Second):
		}
	}
	if err != nil {
		return nil, err
	}
	return inst, nil
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
		ProviderID:   p.config.ID,
		UpstreamID:   instanceID,
		Name:         h.Domain,
		PublicIP:     h.DedicatedIP,
		RawData: map[string]interface{}{
			"assignedips":   h.AssignedIPs,
			"port":          h.Port,
			"os":            h.OS,
			"productid":     h.ProductID,
			"productname":   h.ProductName,
			"nextduedate":   h.NextDueDate,
			"upstream_cost": h.UpstreamCost,
			"dcimid":        h.DcimID,
			"invoiceid":     h.InvoiceID,
		},
	}
	switch strings.ToLower(h.DomainStatus) {
	case "active":
		inst.Status = model.InstanceStatusRunning
	case "pending":
		inst.Status = model.InstanceStatusCreating
	case "suspended", "deleted", "cancelled":
		inst.Status = model.InstanceStatusStopped
	default:
		inst.Status = model.InstanceStatusRunning
	}
	return inst, nil
}

// provisionCmd 财务型上游电源/控制台统一入口：POST /provision/default（源码 Host::on/off/reboot/vnc 的 zjmf_api 分支）。
func (p *MoFangFinanceProvider) provisionCmd(ctx context.Context, op, instanceID, funcName string) error {
	return p.call(ctx, op, http.MethodPost, "provision/default", url.Values{
		"id": {instanceID}, "func": {funcName}, "is_api": {"1"},
	}, nil)
}

// StartInstance 开机：POST /provision/default（func=on，id 为财务主机ID）。
func (p *MoFangFinanceProvider) StartInstance(ctx context.Context, instanceID string) error {
	return p.provisionCmd(ctx, "StartInstance", instanceID, "on")
}

// StopInstance 关机：POST /provision/default（func=off）；force 时 hard_off。
func (p *MoFangFinanceProvider) StopInstance(ctx context.Context, instanceID string, force bool) error {
	fn := "off"
	if force {
		fn = "hard_off"
	}
	return p.provisionCmd(ctx, "StopInstance", instanceID, fn)
}

// RestartInstance 重启：POST /provision/default（func=reboot）。
func (p *MoFangFinanceProvider) RestartInstance(ctx context.Context, instanceID string) error {
	return p.provisionCmd(ctx, "RestartInstance", instanceID, "reboot")
}

// VNC 获取远程控制台：POST /provision/default（func=vnc），返回上游 noVNC 地址。
func (p *MoFangFinanceProvider) VNC(ctx context.Context, instanceID string) (upstream.VNCResult, error) {
	var resp struct {
		URL          string `json:"url"`
		Pass         string `json:"pass"`
		ZjmfCloudOut bool   `json:"zjmfcloud_out_vnc"`
	}
	if err := p.call(ctx, "VNC", http.MethodPost, "provision/default", url.Values{
		"id": {instanceID}, "func": {"vnc"}, "is_api": {"1"},
	}, &resp); err != nil {
		return upstream.VNCResult{}, err
	}
	if resp.URL == "" {
		return upstream.VNCResult{}, &upstream.ProviderError{Op: "VNC", Msg: "上游未返回远程控制台地址"}
	}
	return upstream.VNCResult{URL: resp.URL, Password: resp.Pass, External: resp.ZjmfCloudOut}, nil
}

// SuspendInstance 暂停主机：POST /provision/default（func=suspend，reason 为暂停原因）。
func (p *MoFangFinanceProvider) SuspendInstance(ctx context.Context, instanceID, reason string) error {
	if instanceID == "" {
		return &upstream.ProviderError{Op: "SuspendInstance", Msg: "缺少上游主机ID"}
	}
	if reason == "" {
		reason = "代理商暂停"
	}
	return p.call(ctx, "SuspendInstance", http.MethodPost, "provision/default", url.Values{
		"id": {instanceID}, "func": {"suspend"}, "reason": {reason}, "is_api": {"1"},
	}, nil)
}

// UnsuspendInstance 解除暂停：POST /provision/default（func=unsuspend）。
func (p *MoFangFinanceProvider) UnsuspendInstance(ctx context.Context, instanceID string) error {
	if instanceID == "" {
		return &upstream.ProviderError{Op: "UnsuspendInstance", Msg: "缺少上游主机ID"}
	}
	return p.call(ctx, "UnsuspendInstance", http.MethodPost, "provision/default", url.Values{
		"id": {instanceID}, "func": {"unsuspend"}, "is_api": {"1"},
	}, nil)
}

// TerminateInstance 终止主机：POST /host/cancel（type=Immediate 立即停用，reason 为原因）。
//
// 注意：上游 /host/cancel 的语义是"提交停用申请并生效"（对应 domainstatus=Deleted），
// 而非物理删除记录；这是魔方财务对外提供的唯一终止入口。
func (p *MoFangFinanceProvider) TerminateInstance(ctx context.Context, instanceID, reason string) error {
	if instanceID == "" {
		return &upstream.ProviderError{Op: "TerminateInstance", Msg: "缺少上游主机ID"}
	}
	if reason == "" {
		reason = "立即删除"
	}
	return p.call(ctx, "TerminateInstance", http.MethodPost, "host/cancel", url.Values{
		"id": {instanceID}, "type": {"Immediate"}, "reason": {reason},
	}, nil)
}

// RenewInstance 续费主机（等价源码 Host::renew 的 zjmf_api 分支）：
//  1. POST /host/renew   {hostid, billingcycles} → data.invoiceid（上游生成续费账单）；
//  2. POST /apply_credit {invoiceid, use_credit:1} → status 1001（余额支付完成，账期顺延）；
//  3. GET  host/header     → nextduedate 作为权威新到期时间返回。
//
// 上游任一步失败即整体失败，调用方不得回退为"只改本地账期"。
func (p *MoFangFinanceProvider) RenewInstance(ctx context.Context, req *upstream.RenewRequest) (*upstream.RenewResult, error) {
	if p.isResource() {
		return nil, p.notSupported("RenewInstance", "资源型上游请走资源池续费流程处理")
	}
	if req == nil || req.ProviderInstanceID == "" {
		return nil, &upstream.ProviderError{Op: "RenewInstance", Msg: "缺少上游主机ID"}
	}
	cycle := normalizeBillingCycle(firstNonEmpty(req.Cycle, strFromMap(req.Extra, "billingcycle")))

	// 1) 生成续费账单
	renewForm := url.Values{}
	renewForm.Set("hostid", req.ProviderInstanceID)
	renewForm.Set("billingcycles", cycle)
	var renewData struct {
		InvoiceID int64 `json:"invoiceid"`
	}
	if err := p.call(ctx, "RenewInstance", http.MethodPost, "host/renew", renewForm, &renewData); err != nil {
		return nil, err
	}
	if renewData.InvoiceID == 0 {
		return nil, &upstream.ProviderError{Op: "RenewInstance", Msg: "上游续费成功但未返回账单ID"}
	}

	// 2) 用上游余额支付（status 1001 = 支付完成）
	payForm := url.Values{}
	payForm.Set("invoiceid", strconv.FormatInt(renewData.InvoiceID, 10))
	payForm.Set("use_credit", "1")
	shell, httpStatus, body, err := p.callShell(ctx, "RenewInstance", http.MethodPost, "apply_credit", payForm)
	if err != nil {
		return nil, err
	}
	if httpStatus >= 400 {
		return nil, &upstream.ProviderError{Op: "RenewInstance", StatusCode: httpStatus, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
	}
	if shell.Status != StatusPaidSuccess && shell.Status != StatusOK && shell.Code != StatusOK {
		return nil, &upstream.ProviderError{Op: "RenewInstance", Code: shell.Status, Msg: firstNonEmpty(shell.Msg, "上游续费余额支付失败")}
	}

	res := &upstream.RenewResult{
		UpstreamOrderRef: strconv.FormatInt(renewData.InvoiceID, 10),
		Raw:              map[string]interface{}{"invoiceid": renewData.InvoiceID, "billingcycles": cycle},
	}
	// 3) 拉取权威新到期时间；失败不视为续费失败（账单已支付），由调用方按本地周期兜底。
	if inst, gerr := p.GetInstance(ctx, req.ProviderInstanceID); gerr == nil && inst != nil {
		if due, ok := num(inst.RawData["nextduedate"]); ok && due > 0 {
			res.NewExpireAt = time.Unix(int64(due), 0)
			res.Raw["nextduedate"] = int64(due)
		}
	}
	return res, nil
}

// DeleteInstance / ResizeInstance / ListPools / GetAccountInfo 在「财务对接财务」模式下
// 不由本适配器直接提供（删除/升降配走上游订单，魔方财务无资源池与账户资源统计），
// 因此 MoFangFinanceProvider 不再实现 InstanceAdministration / PoolReader / AccountReader
// 能力接口。消费方通过类型断言按需取用，失败即返回「上游不支持」。

// notSupported 返回能力不支持错误（用于运行期分支：如资源型上游下的 GetProduct/CreateInstance）。
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

// normalizeOS 归一化操作系统值：上游形如 "12|CentOS^CentOS-7.6.1810-x64"，取系统系列（^前 / |后）。
func normalizeOS(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// 去掉 ^ 及之后的详细版本
	if i := strings.Index(v, "^"); i >= 0 {
		v = v[:i]
	}
	// 去掉 "值|" 前缀（如 "12|"）
	if i := strings.Index(v, "|"); i >= 0 {
		v = v[i+1:]
	}
	return strings.TrimSpace(v)
}

// normalizeBillingCycle 归一化计费周期为上游 cart 约定的枚举值。
func normalizeBillingCycle(cycle string) string {
	switch strings.ToLower(strings.TrimSpace(cycle)) {
	case "hour", "hourly":
		return "hour"
	case "quarterly":
		return "quarterly"
	case "semiannually", "semiannual":
		return "semiannually"
	case "annually", "yearly":
		return "annually"
	case "biennially":
		return "biennially"
	case "triennially":
		return "triennially"
	case "onetime", "one-time", "fixed":
		return "onetime"
	default:
		return "monthly"
	}
}

// buildCartConfigOption 从开通请求的 configoption（显式覆盖）或 config_groups（上游配置组）
// 构建 cart 下单所需的 configoption[option.upstream_id] = sub.upstream_id 映射。
func buildCartConfigOption(extra map[string]interface{}) map[string]string {
	out := map[string]string{}
	// 显式 configoption 覆盖优先（键=上游选项ID，值=上游子项ID或数量）
	if raw, ok := extra["configoption"].(map[string]interface{}); ok {
		for k, v := range raw {
			out[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", v)
		}
		return out
	}
	raw, ok := extra["config_groups"]
	if !ok {
		return out
	}
	var groups []ConfigGroup
	if b, err := json.Marshal(raw); err == nil {
		_ = json.Unmarshal(b, &groups)
	}
	for _, g := range groups {
		for _, opt := range g.Options {
			if opt.UpstreamID == 0 {
				continue
			}
			sub := pickConfigSub(opt, extra)
			if sub == nil || sub.UpstreamID == 0 {
				continue
			}
			out[strconv.FormatInt(opt.UpstreamID, 10)] = strconv.FormatInt(sub.UpstreamID, 10)
		}
	}
	return out
}

// pickConfigSub 从配置项可选值中选取一个作为默认开通值（单值取首个可见项，多值匹配期望规格）。
func pickConfigSub(opt ConfigOption, extra map[string]interface{}) *ConfigSub {
	if len(opt.Sub) == 0 {
		return nil
	}
	if len(opt.Sub) == 1 {
		if opt.Sub[0].Hidden != 0 {
			return nil
		}
		return &opt.Sub[0]
	}
	// 多值：按商品自定义规格匹配 cpu/memory/os/area 等
	key := normalizeOptionKey(opt.OptionName)
	var want string
	switch {
	case matchKey(key, "cpu", "核"):
		want = strFromMap(extra, "cpu")
	case matchKey(key, "memory", "mem", "内存"):
		if n := numFromMap(extra, "memory"); n > 0 {
			want = strconv.FormatFloat(n, 'f', -1, 64)
		}
	case matchKey(key, "os", "操作系统", "镜像", "系统"):
		want = strFromMap(extra, "os")
	case matchKey(key, "area", "数据中心", "区域"):
		want = normalizeOS(strFromMap(extra, "area"))
	}
	for i := range opt.Sub {
		if opt.Sub[i].Hidden != 0 {
			continue
		}
		if want == "" || subMatches(opt.Sub[i].OptionName, want) {
			return &opt.Sub[i]
		}
	}
	for i := range opt.Sub {
		if opt.Sub[i].Hidden == 0 {
			return &opt.Sub[i]
		}
	}
	return nil
}

// subMatches 判断配置子项值是否匹配期望值（支持名称包含与数字前缀匹配）。
func subMatches(subName, want string) bool {
	if want == "" {
		return false
	}
	norm := normalizeOS(subName)
	if strings.Contains(strings.ToLower(norm), strings.ToLower(want)) {
		return true
	}
	n := leadingInt(subName)
	if n == 0 {
		return false
	}
	w, err := strconv.ParseFloat(strings.TrimSpace(want), 64)
	return err == nil && int64(w) == int64(n)
}

// specsFromExtra 从开通请求构建标准规格（供实例记录补全）。
func specsFromExtra(extra map[string]interface{}) model.StandardProductSpec {
	var s model.StandardProductSpec
	s.CPU = intFromMap(extra, "cpu")
	s.Memory = intFromMap(extra, "memory")
	s.Disk = intFromMap(extra, "system_disk_size")
	if v := intFromMap(extra, "bw"); v > 0 {
		s.Bandwidth = v
	}
	if o := strFromMap(extra, "os"); o != "" {
		s.OS = normalizeOS(o)
	}
	if r := strFromMap(extra, "area"); r != "" {
		s.Region = normalizeOS(r)
	}
	if z := strFromMap(extra, "node"); z != "" {
		s.Zone = z
	}
	return s
}

// strFromMap 从 map 读取字符串值（兼容 float64/json.Number/string）。
func strFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
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
		return fmt.Sprintf("%v", v)
	}
}

// intFromMap 从 map 读取整数值。
func intFromMap(m map[string]interface{}, key string) int {
	return int(numFromMap(m, key))
}

// numFromMap 从 map 读取浮点值（兼容 float64/json.Number/string/int）。
func numFromMap(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case uint64:
		return float64(t)
	default:
		return 0
	}
}

// firstInt 返回切片首个元素，空则 0。
func firstInt(v []int64) int64 {
	if len(v) > 0 {
		return v[0]
	}
	return 0
}

// randStr 生成随机小写字母数字串。
func randStr(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// randPassword 生成符合上游密码规则（12 位，含大写/小写/数字）的默认密码。
func randPassword() string {
	const lower = "abcdefghijkmnpqrstuvwxyz"
	const upper = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const digit = "23456789"
	b := []byte(randStr(9))
	b = append(b, lower[rand.Intn(len(lower))], upper[rand.Intn(len(upper))], digit[rand.Intn(len(digit))])
	rand.Shuffle(len(b), func(i, j int) { b[i], b[j] = b[j], b[i] })
	return string(b)
}

// safeHostname 将任意名称规范化为合法主机名（小写、非字母数字替换为 -、字母开头）。
func safeHostname(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	h := strings.Trim(b.String(), "-")
	if h == "" {
		return "hs-" + randStr(8)
	}
	if h[0] < 'a' || h[0] > 'z' {
		h = "hs-" + h
	}
	if len(h) > 50 {
		h = h[:50]
	}
	return h
}
