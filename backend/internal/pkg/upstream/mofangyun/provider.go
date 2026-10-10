// Package mofangyun 为魔方云（DCIMCloud）上游提供商的适配器实现。
//
// 协议严格参考魔方财务源码 `app/common/logic/DcimCloud.php`（财务系统作为下游调用魔方云）：
//
//   - 管理员账号（account_type=admin）：
//     登录 POST {base}/v1/login?a=a（表单 username/password），响应体即 access token；
//     业务调用 {base}/v1{path}，请求头 `access-token: {token}`。
//   - 代理商账号（account_type=agent）：
//     登录 POST {base}/index.php?path=/token，响应 {status:"success", data:{token}}；
//     业务调用 {base}/index.php?path={path}。
//   - 业务请求为 application/x-www-form-urlencoded；GET 时参数拼 query。
//   - 响应为 JSON：含 error 字段即业务失败；2xx 即成功；401 时强制重新登录并重试一次。
//     token 按上游默认缓存 6 小时。
//   - 详情为扁平对象（id/hostname/mainip/ip/ipv6/osuser/rootpassword/port/status 等），
//     列表接口形如 {data:[...], ...}。
//
// 核心动作映射：
//   - 健康检查：登录成功即可达（等价 testLink）。
//   - 开机/关机/重启：POST /clouds/{id}/on | /off | /reboot（硬动作 /hardoff、/hard_reboot）。
//   - 详情/状态：GET /clouds/{id} 与 GET /clouds/{id}/status。
//   - 创建：POST /user 建用户 → GET /user/check 校验 → POST /clouds 下发云主机。
//   - 删除：DELETE /clouds/{id}（404 视为已删除）。
//   - 升配：参考源码 upgrade()——先关机（软关机失败转硬关机）再 PUT /clouds/{id}，完成后恢复原电源状态。
//   - 控制台：POST /clouds/{id}/vnc（注意是 POST；GET 会 404，实测确认）。
//   - 重装：PUT /clouds/{id}/reinstall；重置密码：PUT /clouds/{id}/password。
//   - 救援：POST/DELETE /clouds/{id}/rescue；快照备份：/disks/{id}/snapshots、/snapshots/{id}。
//   - 硬件：PUT /clouds/{id}/bw（带宽）、PUT /clouds/{id}/ip（加 IP）、
//     PUT /clouds/{id}/ipv6（加 IPv6）、POST /clouds/{id}/disks（挂数据盘）。
package mofangyun

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
	"hostsent/backend/internal/pkg/traceid"
	"hostsent/backend/internal/pkg/upstream"
)

// ProviderType 魔方云提供商类型标识
const ProviderType = "mofangyun"

// tokenTTL 与财务源码一致：token 缓存 6 小时（dcim_cloud_token 缓存 21600 秒）。
const tokenTTL = 6 * time.Hour

func init() {
	upstream.GetProviderManager().RegisterFactory(ProviderType, func(cfg *upstream.ProviderConfig) upstream.Provider {
		return NewMoFangYunProvider(cfg)
	})
	// 能力描述符（T2.1）：字段级声明本平台能做什么，驱动后台能力矩阵与动态表单。
	upstream.RegisterDescriptor(ProviderType, upstream.CapabilityDescriptor{
		Kind:          upstream.KindCompute,
		SyncScopes:    []string{upstream.ScopePool, upstream.ScopeRegion, upstream.ScopeInstance},
		BillingCycles: []string{"month", "year", "hour", "day"},
		Operations: []string{
			upstream.OpProvision, upstream.OpStart, upstream.OpStop, upstream.OpRestart,
			upstream.OpVNC, upstream.OpResize, upstream.OpDestroy,
			upstream.OpSuspend, upstream.OpUnsuspend,
			// 维护类（实测面板接口均存在）：重装 / 重置密码 / 救援 / 快照备份 / 硬件变更。
			upstream.OpReinstall, upstream.OpResetPassword, upstream.OpRescue, upstream.OpSnapshot,
			upstream.OpBandwidth, upstream.OpAddIP, upstream.OpAttachDisk,
		},
		RenewMode:   upstream.RenewModeNone, // 续费为本地账期 + 平台延期，尚未提供独立续费接口
		DestroyMode: upstream.DestroyModeImmediate,
		SignerType:  upstream.SignerToken,
		// 凭证字段：面板地址走 EndpointSchema，账号密码走 CredentialSchema（password 字段级加密）。
		CredentialSchema: []upstream.Field{
			{Key: "api_key", Label: "账号", Type: upstream.FieldTypeString, Required: true,
				Placeholder: "请输入魔方云用户名"},
			{Key: "api_secret", Label: "密码", Type: upstream.FieldTypePassword, Required: true, Secret: true,
				Placeholder: "请输入魔方云密码"},
			{Key: "account_type", Label: "账号类型", Type: upstream.FieldTypeSelect, Required: false,
				Default: "admin", Options: []upstream.FieldOption{
					{Label: "管理员", Value: "admin"},
					{Label: "代理商", Value: "agent"},
				}},
			{Key: "user_prefix", Label: "财务标识", Type: upstream.FieldTypeString, Required: false,
				Help: "拼在云主机用户名前，用于区分下游来源"},
		},
		EndpointSchema: []upstream.Field{
			{Key: "api_endpoint", Label: "接口地址", Type: upstream.FieldTypeString, Required: true,
				Placeholder: "IP 或域名，可含后台路径"},
			{Key: "port", Label: "接口端口", Type: upstream.FieldTypeString, Required: false,
				Placeholder: "例如 8443"},
			{Key: "secure", Label: "使用 HTTPS", Type: upstream.FieldTypeBool, Required: false},
		},
		RateLimit:      upstream.RateLimitSpec{QPS: 5, Burst: 10},
		SupportsPaging: false,
		// 平台配置项目录（T4.5）：驱动后台「平台配置项」与「配置档」界面。
		OptionCatalog: optionCatalog(),
		FieldDictionary: map[string]any{
			"source": "实测（依据 mofangyun/provider.go 与 17 号文档 §6.1）",
			"write": []string{
				"area", "node", "node_group", "node_priority", "ip_group", "store",
				"cpu", "memory", "cpu_limit", "cpu_model", "advanced_cpu", "type", "rid",
				"system_disk_size", "niccard",
				"bw", "in_bw", "out_bw", "advanced_bw", "network_type", "vpc", "vpc_name",
				"traffic_type", "traffic_quota", "flow_way", "flow_limit", "reset_flow_day",
				"bind_mac", "port", "ip_num", "ipv6_num",
				"os", "nat_acl_limit", "nat_web_limit", "backup_num", "snap_num", "link_clone",
				"other_data_disk",
			},
			"read": []string{"id", "name", "state", "cpu_num", "memory_size", "disk_size",
				"disk_type", "bandwidth", "private_ip", "public_ip", "region", "zone", "host",
				"os_image", "created_at", "expire_at"},
			"note": "读写字段名不对称：写用 cpu/memory/system_disk_size，读用 cpu_num/memory_size/disk_size",
		},
	})
}

// MoFangYunProvider 魔方云适配器
type MoFangYunProvider struct {
	config *upstream.ProviderConfig
	client *http.Client

	tokenMu    sync.Mutex
	token      string
	tokenUntil time.Time
}

// NewMoFangYunProvider 按配置创建魔方云适配器实例
func NewMoFangYunProvider(config *upstream.ProviderConfig) *MoFangYunProvider {
	timeout := time.Duration(config.Timeout) * time.Second
	if config.Timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &MoFangYunProvider{
		config: config,
		client: &http.Client{Timeout: timeout},
	}
}

// GetType 返回提供商类型标识
func (p *MoFangYunProvider) GetType() string { return ProviderType }

// GetName 返回提供商名称
func (p *MoFangYunProvider) GetName() string { return "魔方云" }

// Capabilities 返回能力描述符（T2.1）。
func (p *MoFangYunProvider) Capabilities() upstream.CapabilityDescriptor {
	d, _ := upstream.Descriptor(ProviderType)
	return d
}

// isAgent 账号类型：agent=代理商（/index.php?path= 入口），其余为管理员（/v1 入口）。
func (p *MoFangYunProvider) isAgent() bool { return p.config.AccountType == "agent" }

// baseURL 组合协议、地址与端口，去掉尾部斜杠。
func (p *MoFangYunProvider) baseURL() (string, error) {
	host := strings.TrimRight(p.config.APIEndpoint, "/")
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if host == "" {
		return "", &upstream.ProviderError{Op: "request", Msg: "接口地址为空"}
	}
	scheme := "http"
	if p.config.Secure {
		scheme = "https"
	}
	if p.config.Port != "" {
		host = host + ":" + strings.TrimPrefix(p.config.Port, ":")
	}
	return scheme + "://" + host, nil
}

// actionURL 按账号类型构造业务调用地址，path 形如 "/clouds/1/on"（可含 query）。
func (p *MoFangYunProvider) actionURL(base, path string) string {
	if p.isAgent() {
		return base + "/index.php?path=" + strings.TrimLeft(path, "/")
	}
	return base + "/v1" + path
}

// login 获取 access token，带本地缓存；force 时强制重新登录。
// 参考源码 login()：管理员账号响应体即 token；代理商账号响应 {status:"success",data:{token}}。
func (p *MoFangYunProvider) login(ctx context.Context, force bool) (string, error) {
	p.tokenMu.Lock()
	defer p.tokenMu.Unlock()
	if !force && p.token != "" && time.Now().Before(p.tokenUntil) {
		return p.token, nil
	}
	base, err := p.baseURL()
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("username", p.config.APIKey)
	form.Set("password", p.config.APISecret)

	var loginURL string
	if p.isAgent() {
		loginURL = base + "/index.php?path=" + strings.TrimLeft("/token", "/")
	} else {
		loginURL = base + "/v1/login?a=a"
	}
	body, status, err := p.httpPost(ctx, loginURL, form, "")
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound {
		return "", &upstream.ProviderError{
			Op:         "login",
			StatusCode: status,
			Msg:        "无法连接魔方云管理系统，请检查魔方云访问地址是否填写正确",
		}
	}
	if p.isAgent() {
		var shell struct {
			Status string `json:"status"`
			Msg    string `json:"msg"`
			Data   struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &shell); err != nil || shell.Data.Token == "" {
			return "", &upstream.ProviderError{Op: "login", StatusCode: status, Msg: firstNonEmpty(shell.Msg, "魔方云登录失败：响应缺少 token")}
		}
		p.token = strings.Trim(shell.Data.Token, "\"")
	} else {
		// 管理员登录响应体即 token 字符串（可能带引号）
		p.token = strings.Trim(strings.TrimSpace(string(body)), "\"")
		if p.token == "" {
			return "", &upstream.ProviderError{Op: "login", StatusCode: status, Msg: "魔方云登录失败：响应为空"}
		}
	}
	p.tokenUntil = time.Now().Add(tokenTTL)
	return p.token, nil
}

// httpPost 发送表单 POST；bearer 传入时设置 access-token 请求头。
func (p *MoFangYunProvider) httpPost(ctx context.Context, rawURL string, form url.Values, token string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, &upstream.ProviderError{Op: "request", Err: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "WHMCS") // 与源码 basecurl 一致
	if token != "" {
		req.Header.Set("access-token", token)
	}
	return p.httpDo(req)
}

// httpDo 执行请求并返回响应体与状态码。
func (p *MoFangYunProvider) httpDo(req *http.Request) ([]byte, int, error) {
	started := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		upstream.Capture(req.Context(), req.Method, req.URL.String(),
			upstream.RequestBodyOf(req.GetBody), 0, nil, err, time.Since(started))
		return nil, 0, &upstream.ProviderError{Op: "request", Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	upstream.Capture(req.Context(), req.Method, req.URL.String(),
		upstream.RequestBodyOf(req.GetBody), resp.StatusCode, body, err, time.Since(started))
	if err != nil {
		return nil, resp.StatusCode, &upstream.ProviderError{Op: "request", Err: err}
	}
	return body, resp.StatusCode, nil
}

// traceCtx 创建一次调用的采集上下文（doc92 §3.2）：
// trace_id 从请求 context 继承，渠道信息由 providerInfo 提供。
func (p *MoFangYunProvider) traceCtx(ctx context.Context, op string) (context.Context, *upstream.Trace) {
	next, tr := upstream.NewTrace(ctx, op, traceid.From(ctx))
	tr.SetProvider(p.providerInfo)
	return next, tr
}

// providerInfo 采集记录上要带的渠道标识。
func (p *MoFangYunProvider) providerInfo() (uint, string, string) {
	name := firstNonEmpty(p.config.Name, "魔方云")
	return p.config.ID, name, ProviderType
}

// call 调用魔方云业务接口：自动登录、401 重登录重试一次、解析 error 字段。
// out 非 nil 时将整个响应体 JSON 解析到 out。
func (p *MoFangYunProvider) call(ctx context.Context, op, method, path string, form url.Values, out interface{}) (err error) {
	ctx, tr := p.traceCtx(ctx, op)
	// 业务判定结果在返回时才可知（HTTP 2xx 也可能是 body.error 失败），
	// 因此用命名返回值在 defer 里回填 success/error_code（doc92 §3.2）。
	defer func() { tr.Finish(err) }()
	base, err := p.baseURL()
	if err != nil {
		return err
	}
	token, err := p.login(ctx, false)
	if err != nil {
		return &upstream.ProviderError{Op: op, Err: err}
	}
	body, status, err := p.doWithToken(ctx, base, token, method, path, form)
	if err != nil {
		return &upstream.ProviderError{Op: op, Err: err}
	}
	// 与源码 curl() 一致：401 强制重新登录后重试一次。
	if status == http.StatusUnauthorized {
		token, err = p.login(ctx, true)
		if err != nil {
			return &upstream.ProviderError{Op: op, Err: err}
		}
		body, status, err = p.doWithToken(ctx, base, token, method, path, form)
		if err != nil {
			return &upstream.ProviderError{Op: op, Err: err}
		}
	}
	if status < 200 || status >= 300 {
		// 面板在 4xx 时同样用 {"error":"..."} 说明原因（如"镜像不可用"、"主机名已存在"），
		// 只回 HTTP 状态码会把可定位的业务原因丢掉，排障时无从下手。
		var probe struct {
			Error interface{} `json:"error"`
			Msg   string      `json:"msg"`
		}
		_ = json.Unmarshal(body, &probe)
		detail := ""
		if probe.Error != nil {
			detail = fmt.Sprintf("%v", probe.Error)
		} else if probe.Msg != "" {
			detail = probe.Msg
		}
		if detail == "" {
			detail = "请求失败,HTTP状态码:" + strconv.Itoa(status)
		}
		return &upstream.ProviderError{Op: op, StatusCode: status, Msg: detail}
	}
	// 与源码 basecurl 一致：响应含 error 字段即业务失败。
	var probe struct {
		Error interface{} `json:"error"`
	}
	if json.Unmarshal(body, &probe) == nil && probe.Error != nil {
		return &upstream.ProviderError{Op: op, Code: status, Msg: fmt.Sprintf("%v", probe.Error)}
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return &upstream.ProviderError{Op: op, Err: fmt.Errorf("解析魔方云响应失败: %w", err)}
		}
	}
	return nil
}

// doWithToken 以给定 token 执行一次业务调用。GET 的表单参数转 query，其余进请求体。
func (p *MoFangYunProvider) doWithToken(ctx context.Context, base, token, method, path string, form url.Values) ([]byte, int, error) {
	rawURL := p.actionURL(base, path)
	if form == nil {
		form = url.Values{}
	}
	method = strings.ToUpper(method)
	var req *http.Request
	var err error
	if method == http.MethodGet {
		if enc := form.Encode(); enc != "" {
			sep := "?"
			if strings.Contains(rawURL, "?") {
				sep = "&"
			}
			rawURL += sep + enc
		}
		req, err = http.NewRequestWithContext(ctx, method, rawURL, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return nil, 0, &upstream.ProviderError{Op: path, Err: err}
	}
	req.Header.Set("User-Agent", "WHMCS")
	req.Header.Set("access-token", token)
	return p.httpDo(req)
}

// HealthCheck 连通性测试：登录成功即视为可达（等价源码 testLink）。
func (p *MoFangYunProvider) HealthCheck(ctx context.Context) error {
	_, err := p.login(ctx, true)
	return err
}

// CreateInstance 开通云主机。
// 参考源码 createAccount()：先确保魔方云用户（POST /user + GET /user/check），
// 再 POST /clouds 下发实例。规格参数通过 req.Extra 传递，键名与魔方云一致
// （area/node/os/cpu/memory/system_disk_size/data_disk_size/network_type/bw/flow_limit/
// flow_way/ip_num/backup_num/snap_num/nat_acl_limit/nat_web_limit/type/ipv6_num/port/
// node_group/ip_group/node_priority/resource_package 等），同时支持包一层
// Extra["configoptions"]（与财务 configoptions 同形）。
func (p *MoFangYunProvider) CreateInstance(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
	opts := collectOptions(req.Extra)

	form := url.Values{}
	// 用户：优先使用传入的魔方云用户 ID，否则按源码创建用户。
	clientID := firstNonEmpty(opts["client"], opts["client_id"], opts["user_id"])
	if clientID == "" {
		id, err := p.ensureCloudUser(ctx, opts)
		if err != nil {
			return nil, err
		}
		clientID = id
	}
	form.Set("client", clientID)
	// 主机名规则由面板强校验（实测：必须英文大小写字母开头，只允许字母/数字/_ - .，6 位以上）。
	// 商品名/订单名是中文（如"魔方云测试 2核4G"）时直接下发会被 400 拒绝，
	// 因此合法则原样用，否则按时间戳生成唯一主机名。
	form.Set("hostname", sanitizeHostname(firstNonEmpty(opts["hostname"], req.Name)))
	rootpass := firstNonEmpty(req.Password, opts["rootpass"], randPassword(12))
	form.Set("rootpass", rootpass)

	// 规格与可配置项透传（保留创建时必填校验交给上游判断）。
	for _, key := range cloudOptionKeys {
		if v, ok := opts[key]; ok && v != "" {
			form.Set(key, v)
		}
	}
	// 数据盘：other_data_disk 传数组，需按表单数组语法展开。
	// 优先用已翻译的 data_disk_size（文档口径 "30,1" 已归一为 size + data_disk_store），
	// Extra 显式给了数组形态时仍走 expandOtherDataDisk。
	if strings.TrimSpace(opts["data_disk_size"]) != "" {
		expandDataDisk(form, opts)
	} else {
		expandOtherDataDisk(form, req.Extra)
	}

	// 创建响应实测为 {"num":1,"id":"2","taskid":"1192",...}：id 是**字符串**，
	// 历史上按 int64 解析会在成功后被 JSON 解码错误掩盖成"开通失败"。
	var created struct {
		ID json.RawMessage `json:"id"`
	}
	if err := p.call(ctx, "CreateInstance", http.MethodPost, "/clouds", form, &created); err != nil {
		return nil, err
	}
	createdID := rawIDString(created.ID)
	if createdID == "" || createdID == "0" {
		return nil, &upstream.ProviderError{Op: "CreateInstance", Msg: "魔方云开通请求已提交但未返回云主机 ID"}
	}
	inst, err := p.GetInstance(ctx, createdID)
	if err != nil {
		// 详情失败不回滚开通，仅返回最小可用信息。
		return &model.StandardInstance{
			ProviderType: ProviderType,
			ProviderID:   p.config.ID,
			UpstreamID:   createdID,
			Name:         firstNonEmpty(req.Name, opts["hostname"]),
			Status:       model.InstanceStatusCreating,
		}, nil
	}
	return inst, nil
}

// ListInstances 拉取魔方云云主机列表。filters 支持 per_page/page/status/area/node。
//
// 响应有两种形态，必须都认（实测面板给的是分页包裹形态）：
//
//	{"data":[{...}],"meta":{...}}   分页包裹（当前面板实测）
//	[{...},{...}]                   裸数组（旧版/部分网关）
//
// 只按裸数组解会在分页形态下报 "cannot unmarshal object into Go value of type
// []map[string]interface {}"，表现为"实例列表全空"——线上很难发现，因此这里用
// json.RawMessage 先取原始字节再按形状分派。
func (p *MoFangYunProvider) ListInstances(ctx context.Context, filters map[string]string) ([]*model.StandardInstance, error) {
	form := url.Values{}
	form.Set("per_page", firstNonEmpty(filters["per_page"], "100"))
	form.Set("page", firstNonEmpty(filters["page"], "1"))
	for _, key := range []string{"status", "area", "node", "keywords"} {
		if v := filters[key]; v != "" {
			form.Set(key, v)
		}
	}
	var raw json.RawMessage
	if err := p.call(ctx, "ListInstances", http.MethodGet, "/clouds", form, &raw); err != nil {
		return nil, err
	}
	rows := decodeInstanceRows(raw)
	instances := make([]*model.StandardInstance, 0, len(rows))
	for _, row := range rows {
		inst := buildInstance(row)
		inst.ProviderID = p.config.ID
		instances = append(instances, inst)
	}
	return instances, nil
}

// decodeInstanceRows 把实例列表响应归一成行数组：裸数组直接用，分页包裹取 data。
// 两者都不是时返回空（不报错）：分页外壳变化不该让整个同步任务失败。
func decodeInstanceRows(raw json.RawMessage) []map[string]interface{} {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	// 裸数组
	var list []map[string]interface{}
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(raw, &list); err == nil {
			return list
		}
		return nil
	}
	// 分页包裹：{"data":[...], "meta":{...}}（data 也可能是 {"data":[...]} 的双层包裹）
	var shell struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &shell); err != nil {
		return nil
	}
	inner := strings.TrimSpace(string(shell.Data))
	if inner == "" || inner == "null" {
		return nil
	}
	if strings.HasPrefix(inner, "[") {
		if err := json.Unmarshal(shell.Data, &list); err == nil {
			return list
		}
		return nil
	}
	var nested struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(shell.Data, &nested); err == nil {
		return nested.Data
	}
	return nil
}

// GetInstance 拉取单台云主机详情并映射标准实例。
func (p *MoFangYunProvider) GetInstance(ctx context.Context, instanceID string) (*model.StandardInstance, error) {
	var detail map[string]interface{}
	if err := p.call(ctx, "GetInstance", http.MethodGet, "/clouds/"+instanceID, nil, &detail); err != nil {
		return nil, err
	}
	inst := buildInstance(detail)
	inst.ProviderID = p.config.ID
	// 状态以 /clouds/{id}/status 为准（参考源码 getCloudStatus）。
	var st struct {
		Status   string `json:"status"`
		TaskName string `json:"task_name"`
	}
	if err := p.call(ctx, "GetInstance", http.MethodGet, "/clouds/"+instanceID+"/status", nil, &st); err == nil && st.Status != "" {
		inst.Status = mapCloudStatus(st.Status)
		if st.TaskName != "" {
			if inst.RawData == nil {
				inst.RawData = map[string]interface{}{}
			}
			inst.RawData["task_name"] = st.TaskName
		}
	}
	return inst, nil
}

// StartInstance 开机：POST /clouds/{id}/on
func (p *MoFangYunProvider) StartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "StartInstance", http.MethodPost, "/clouds/"+instanceID+"/on", url.Values{}, nil)
}

// StopInstance 关机：POST /clouds/{id}/off；force 时硬关机 /hardoff。
func (p *MoFangYunProvider) StopInstance(ctx context.Context, instanceID string, force bool) error {
	action := "/off"
	if force {
		action = "/hardoff"
	}
	return p.call(ctx, "StopInstance", http.MethodPost, "/clouds/"+instanceID+action, url.Values{}, nil)
}

// RestartInstance 重启：POST /clouds/{id}/reboot；force 时硬重启 /hard_reboot。
func (p *MoFangYunProvider) RestartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "RestartInstance", http.MethodPost, "/clouds/"+instanceID+"/reboot", url.Values{}, nil)
}

// SuspendInstance 暂停云主机：POST /clouds/{id}/suspend（type=traffic|due|other）。
// reason 为 "flow"/"traffic" 时映射流量超额，"due" 映射到期，其余为 other。
func (p *MoFangYunProvider) SuspendInstance(ctx context.Context, instanceID, reason string) error {
	if instanceID == "" {
		return &upstream.ProviderError{Op: "SuspendInstance", Msg: "缺少上游云主机ID"}
	}
	typ := "other"
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "flow", "traffic":
		typ = "traffic"
	case "due", "expire", "expired":
		typ = "due"
	}
	return p.call(ctx, "SuspendInstance", http.MethodPost, "/clouds/"+instanceID+"/suspend", url.Values{"type": {typ}}, nil)
}

// UnsuspendInstance 解除暂停：POST /clouds/{id}/unsuspend。
func (p *MoFangYunProvider) UnsuspendInstance(ctx context.Context, instanceID string) error {
	if instanceID == "" {
		return &upstream.ProviderError{Op: "UnsuspendInstance", Msg: "缺少上游云主机ID"}
	}
	return p.call(ctx, "UnsuspendInstance", http.MethodPost, "/clouds/"+instanceID+"/unsuspend", url.Values{}, nil)
}

// VNC 获取远程控制台：GET /clouds/{id}/vnc。
// VNC 获取远程控制台：POST /clouds/{id}/vnc。
//
// 实测（测试面板）：**GET 返回 404「请求地址有误」，POST 返回 200 且带 vnc_url/token/vnc_pass**。
// 源码 DcimCloud::vnc() 的 curl() 默认方法就是 POST，本适配器此前写成 GET，控制台一直打不开。
//
// 上游可能返回外链（vnc_url_http/vnc_url_https，External=true）或 websocket 地址
// （vnc_url=ws/wss + token + vnc_pass，需前端用 noVNC 连接，External=false）。
func (p *MoFangYunProvider) VNC(ctx context.Context, instanceID string) (upstream.VNCResult, error) {
	var resp struct {
		VncURL      string `json:"vnc_url"`
		VncURLHTTP  string `json:"vnc_url_http"`
		VncURLHTTPS string `json:"vnc_url_https"`
		VncPass     string `json:"vnc_pass"`
		Token       string `json:"token"`
	}
	if err := p.call(ctx, "VNC", http.MethodPost, "/clouds/"+instanceID+"/vnc", url.Values{}, &resp); err != nil {
		return upstream.VNCResult{}, err
	}
	if resp.VncURLHTTP != "" || resp.VncURLHTTPS != "" {
		url := resp.VncURLHTTPS
		if url == "" {
			url = resp.VncURLHTTP
		}
		return upstream.VNCResult{URL: url, External: true}, nil
	}
	if resp.VncURL == "" {
		return upstream.VNCResult{}, &upstream.ProviderError{Op: "VNC", Msg: "魔方云未返回远程控制台地址"}
	}
	link := resp.VncURL
	if resp.Token != "" && !strings.Contains(link, "token=") {
		sep := "?"
		if strings.Contains(link, "?") {
			sep = "&"
		}
		link += sep + "token=" + resp.Token
	}
	return upstream.VNCResult{URL: link, Password: resp.VncPass, External: false}, nil
}

// HardStopInstance 硬关机：POST /clouds/{id}/hardoff（参考源码 hardOff）。
func (p *MoFangYunProvider) HardStopInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "HardStopInstance", http.MethodPost, "/clouds/"+instanceID+"/hardoff", url.Values{}, nil)
}

// HardRestartInstance 硬重启：POST /clouds/{id}/hard_reboot（参考源码 hardReboot）。
//
// 此前服务层把"硬重启"实现成硬关+开机，中间会经历一次完整断电；平台本就有独立指令，
// 走它既快又与面板行为一致。
func (p *MoFangYunProvider) HardRestartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "HardRestartInstance", http.MethodPost, "/clouds/"+instanceID+"/hard_reboot", url.Values{}, nil)
}

// ReinstallInstance 重装系统：PUT /clouds/{id}/reinstall（参考源码 reinstall）。
//
// 参数与源码一致：os（目标镜像）+ 可选 port / format_data_disk / system_disk_size。
// 平台在响应里返回重装后生成的新初始凭据（user/password），必须带回给调用方回写，
// 否则客户重装完就登不上机器（源码也是在这里写回 host.password/username）。
func (p *MoFangYunProvider) ReinstallInstance(ctx context.Context, req *upstream.ReinstallRequest) (*upstream.ReinstallResult, error) {
	if req == nil || strings.TrimSpace(req.ProviderInstanceID) == "" {
		return nil, &upstream.ProviderError{Op: "ReinstallInstance", Msg: "缺少上游云主机ID"}
	}
	if strings.TrimSpace(req.OS) == "" {
		return nil, &upstream.ProviderError{Op: "ReinstallInstance", Msg: "缺少目标镜像 ID（os）"}
	}
	form := url.Values{}
	form.Set("os", strings.TrimSpace(req.OS))
	if req.Port > 0 && req.Port <= 65535 {
		form.Set("port", strconv.Itoa(req.Port))
	}
	if req.FormatDataDisk {
		form.Set("format_data_disk", "1")
	} else {
		form.Set("format_data_disk", "0")
	}
	if req.SystemDiskSize > 0 {
		form.Set("system_disk_size", strconv.Itoa(req.SystemDiskSize))
	}
	var resp struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := p.call(ctx, "ReinstallInstance", http.MethodPut, "/clouds/"+req.ProviderInstanceID+"/reinstall", form, &resp); err != nil {
		return nil, err
	}
	return &upstream.ReinstallResult{Username: resp.User, Password: resp.Password}, nil
}

// ResetInstancePassword 重置登录密码：PUT /clouds/{id}/password（参考源码 CrackPassword）。
func (p *MoFangYunProvider) ResetInstancePassword(ctx context.Context, instanceID, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return &upstream.ProviderError{Op: "ResetInstancePassword", Msg: "新密码不能为空"}
	}
	return p.call(ctx, "ResetInstancePassword", http.MethodPut, "/clouds/"+instanceID+"/password",
		url.Values{"password": {newPassword}}, nil)
}

// RescueInstance 进入救援系统：POST /clouds/{id}/rescue（type=救援系统类型，temp_pass=临时密码）。
func (p *MoFangYunProvider) RescueInstance(ctx context.Context, instanceID string, system int, tempPassword string) error {
	if system != 1 && system != 2 {
		return &upstream.ProviderError{Op: "RescueInstance", Msg: "救援系统类型错误（1 或 2）"}
	}
	if strings.TrimSpace(tempPassword) == "" {
		return &upstream.ProviderError{Op: "RescueInstance", Msg: "请输入临时密码"}
	}
	return p.call(ctx, "RescueInstance", http.MethodPost, "/clouds/"+instanceID+"/rescue",
		url.Values{"type": {strconv.Itoa(system)}, "temp_pass": {tempPassword}}, nil)
}

// ExitRescueInstance 退出救援系统：DELETE /clouds/{id}/rescue。
func (p *MoFangYunProvider) ExitRescueInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "ExitRescueInstance", http.MethodDelete, "/clouds/"+instanceID+"/rescue", url.Values{}, nil)
}

// ListSnapshots 列出实例的快照/备份：GET /clouds/{id}/snapshots。
// snapshotType 非空时本地按 type 过滤（平台接口一次性返回两类）。
func (p *MoFangYunProvider) ListSnapshots(ctx context.Context, instanceID, snapshotType string) ([]upstream.SnapshotInfo, error) {
	var resp struct {
		Data []struct {
			ID         int64       `json:"id"`
			DiskID     int64       `json:"diskid"`
			Name       string      `json:"name"`
			Size       interface{} `json:"size"`
			Type       string      `json:"type"`
			Status     int         `json:"status"`
			DiskName   string      `json:"disk_name"`
			CreateTime string      `json:"create_time"`
			Remarks    string      `json:"remarks"`
		} `json:"data"`
	}
	if err := p.call(ctx, "ListSnapshots", http.MethodGet, "/clouds/"+instanceID+"/snapshots",
		url.Values{"page": {"1"}, "per_page": {"9999"}}, &resp); err != nil {
		return nil, err
	}
	want := strings.TrimSpace(snapshotType)
	out := make([]upstream.SnapshotInfo, 0, len(resp.Data))
	for _, s := range resp.Data {
		if want != "" && s.Type != want {
			continue
		}
		out = append(out, upstream.SnapshotInfo{
			ID:         strconv.FormatInt(s.ID, 10),
			Name:       s.Name,
			Type:       s.Type,
			Size:       fmt.Sprintf("%v", s.Size),
			Status:     s.Status,
			DiskID:     strconv.FormatInt(s.DiskID, 10),
			DiskName:   s.DiskName,
			CreateTime: s.CreateTime,
			Remarks:    s.Remarks,
		})
	}
	return out, nil
}

// CreateSnapshot 创建快照/备份：POST /disks/{diskID}/snapshots（type=snap|backup, name=名称）。
// 参考源码 createSnap/createBackup：快照挂在磁盘上，因此 diskID 必填。
func (p *MoFangYunProvider) CreateSnapshot(ctx context.Context, req *upstream.CreateSnapshotRequest) error {
	if req == nil || strings.TrimSpace(req.DiskID) == "" {
		return &upstream.ProviderError{Op: "CreateSnapshot", Msg: "缺少磁盘 ID（快照挂在磁盘上）"}
	}
	typ := strings.TrimSpace(req.Type)
	if typ == "" {
		typ = upstream.SnapshotTypeSnap
	}
	if typ != upstream.SnapshotTypeSnap && typ != upstream.SnapshotTypeBackup {
		return &upstream.ProviderError{Op: "CreateSnapshot", Msg: "快照类型只能是 snap 或 backup"}
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("hs-%s-%d", typ, time.Now().Unix())
	}
	return p.call(ctx, "CreateSnapshot", http.MethodPost, "/disks/"+req.DiskID+"/snapshots",
		url.Values{"type": {typ}, "name": {name}}, nil)
}

// DeleteSnapshot 删除快照/备份：DELETE /snapshots/{id}。
func (p *MoFangYunProvider) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	return p.call(ctx, "DeleteSnapshot", http.MethodDelete, "/snapshots/"+snapshotID, url.Values{}, nil)
}

// RestoreSnapshot 用快照/备份恢复：POST /snapshots/{id}/restore?hostid={实例ID}。
func (p *MoFangYunProvider) RestoreSnapshot(ctx context.Context, instanceID, snapshotID string) error {
	return p.call(ctx, "RestoreSnapshot", http.MethodPost, "/snapshots/"+snapshotID+"/restore",
		url.Values{"hostid": {instanceID}}, nil)
}

// UpdateBandwidth 修改带宽：PUT /clouds/{id}/bw（参考源码 upgrade 分支）。
// 单方向为 0 表示不改该方向（平台按传入字段覆盖，缺省即保持）。
func (p *MoFangYunProvider) UpdateBandwidth(ctx context.Context, instanceID string, inBw, outBw int) error {
	form := url.Values{}
	if inBw > 0 {
		form.Set("in_bw", strconv.Itoa(inBw))
	}
	if outBw > 0 {
		form.Set("out_bw", strconv.Itoa(outBw))
	}
	if len(form) == 0 {
		return &upstream.ProviderError{Op: "UpdateBandwidth", Msg: "至少需要一个方向的带宽值"}
	}
	return p.call(ctx, "UpdateBandwidth", http.MethodPut, "/clouds/"+instanceID+"/bw", form, nil)
}

// AddIPs 增加 IP：PUT /clouds/{id}/ip（num=数量，ip_group=可选 IP 分组 ID）。
func (p *MoFangYunProvider) AddIPs(ctx context.Context, instanceID string, num int, ipGroup string) error {
	if num <= 0 {
		return &upstream.ProviderError{Op: "AddIPs", Msg: "IP 数量必须大于 0"}
	}
	form := url.Values{"num": {strconv.Itoa(num)}}
	if strings.TrimSpace(ipGroup) != "" {
		form.Set("ip_group", strings.TrimSpace(ipGroup))
	}
	return p.call(ctx, "AddIPs", http.MethodPut, "/clouds/"+instanceID+"/ip", form, nil)
}

// AddIPv6 增加 IPv6：PUT /clouds/{id}/ipv6（num=数量）。平台未启用 IPv6 时会明确报错。
func (p *MoFangYunProvider) AddIPv6(ctx context.Context, instanceID string, num int) error {
	if num <= 0 {
		return &upstream.ProviderError{Op: "AddIPv6", Msg: "IPv6 数量必须大于 0"}
	}
	return p.call(ctx, "AddIPv6", http.MethodPut, "/clouds/"+instanceID+"/ipv6",
		url.Values{"num": {strconv.Itoa(num)}}, nil)
}

// AttachDataDisk 挂载数据盘：POST /clouds/{id}/disks（size=GB, store=存储 ID, driver=virtio）。
func (p *MoFangYunProvider) AttachDataDisk(ctx context.Context, instanceID string, sizeGB int, store string) error {
	if sizeGB <= 0 {
		return &upstream.ProviderError{Op: "AttachDataDisk", Msg: "数据盘大小必须大于 0"}
	}
	form := url.Values{"size": {strconv.Itoa(sizeGB)}, "driver": {"virtio"}}
	if strings.TrimSpace(store) != "" {
		form.Set("store", strings.TrimSpace(store))
	}
	return p.call(ctx, "AttachDataDisk", http.MethodPost, "/clouds/"+instanceID+"/disks", form, nil)
}

// DeleteInstance 删除云主机：DELETE /clouds/{id}；404 视为已删除（与源码 terminate 一致）。
func (p *MoFangYunProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	base, err := p.baseURL()
	if err != nil {
		return err
	}
	token, err := p.login(ctx, false)
	if err != nil {
		return &upstream.ProviderError{Op: "DeleteInstance", Err: err}
	}
	_, status, err := p.doWithToken(ctx, base, token, http.MethodDelete, "/clouds/"+instanceID, nil)
	if err != nil {
		return &upstream.ProviderError{Op: "DeleteInstance", Err: err}
	}
	if status == http.StatusUnauthorized {
		token, err = p.login(ctx, true)
		if err != nil {
			return &upstream.ProviderError{Op: "DeleteInstance", Err: err}
		}
		_, status, err = p.doWithToken(ctx, base, token, http.MethodDelete, "/clouds/"+instanceID, nil)
		if err != nil {
			return &upstream.ProviderError{Op: "DeleteInstance", Err: err}
		}
	}
	if status == http.StatusNotFound || (status >= 200 && status < 300) {
		return nil
	}
	return &upstream.ProviderError{Op: "DeleteInstance", StatusCode: status, Msg: "删除失败,HTTP状态码:" + strconv.Itoa(status)}
}

// ResizeInstance 升降配：参考源码 upgrade()——先关机（硬关机兜底），
// 再 PUT /clouds/{id} 修改 cpu/memory 等参数，完成后恢复原电源状态。
func (p *MoFangYunProvider) ResizeInstance(ctx context.Context, instanceID string, specs *model.StandardProductSpec) error {
	if specs == nil {
		return &upstream.ProviderError{Op: "ResizeInstance", Msg: "缺少目标规格"}
	}
	// 记录原电源状态
	var st struct {
		Status string `json:"status"`
	}
	wasOn := false
	if err := p.call(ctx, "ResizeInstance", http.MethodGet, "/clouds/"+instanceID+"/status", nil, &st); err == nil {
		wasOn = st.Status == "on" || st.Status == "task" || st.Status == "wait_reboot"
	}
	// 关机并等待（软关机 10×2s，未停则硬关机再等 5×2s）
	if wasOn {
		if err := p.StopInstance(ctx, instanceID, false); err != nil {
			return err
		}
		if !p.waitStatus(ctx, instanceID, "off", 10) {
			_ = p.StopInstance(ctx, instanceID, true)
			if !p.waitStatus(ctx, instanceID, "off", 5) {
				return &upstream.ProviderError{Op: "ResizeInstance", Msg: "等待云主机关机超时，已取消升降配"}
			}
		}
	}
	form := url.Values{}
	if specs.CPU > 0 {
		form.Set("cpu", strconv.Itoa(specs.CPU))
	}
	if specs.Memory > 0 {
		form.Set("memory", strconv.Itoa(specs.Memory))
	}
	for _, key := range cloudOptionKeys {
		if v := specs.Extra[key]; v != nil && fmt.Sprintf("%v", v) != "" {
			form.Set(key, fmt.Sprintf("%v", v))
		}
	}
	if err := p.call(ctx, "ResizeInstance", http.MethodPut, "/clouds/"+instanceID, form, nil); err != nil {
		return err
	}
	if wasOn {
		if err := p.StartInstance(ctx, instanceID); err != nil {
			return &upstream.ProviderError{Op: "ResizeInstance", Msg: "配置已修改，但开机恢复失败：" + err.Error()}
		}
	}
	return nil
}

// ListPools 拉取区域列表：GET /areas?sort=asc&list_type=all（参考源码 getArea）。
func (p *MoFangYunProvider) ListPools(ctx context.Context) ([]*upstream.StandardPool, error) {
	// 列表接口常规为 {data:[...]} 分页包裹；兼容裸数组形态。
	var wrapped struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	err := p.call(ctx, "ListPools", http.MethodGet, "/areas", url.Values{"sort": {"asc"}, "list_type": {"all"}}, &wrapped)
	if err != nil {
		var areas []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		if err2 := p.call(ctx, "ListPools", http.MethodGet, "/areas", url.Values{"sort": {"asc"}, "list_type": {"all"}}, &areas); err2 != nil {
			return nil, err
		}
		for _, a := range areas {
			wrapped.Data = append(wrapped.Data, struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			}{a.ID, a.Name})
		}
	}
	pools := make([]*upstream.StandardPool, 0, len(wrapped.Data))
	for _, a := range wrapped.Data {
		pools = append(pools, &upstream.StandardPool{
			ID:     strconv.FormatInt(a.ID, 10),
			Name:   a.Name,
			Type:   "region",
			Status: "active",
		})
	}
	return pools, nil
}

// GetAccountInfo 魔方云为资源管理系统，无账户余额概念，本适配器不实现 AccountReader。

// ListPlatformResources 拉取平台可售资源目录（自营规格配置用）：
// 区域 GET /areas、节点 GET /nodes、存储 GET /stores、镜像 GET /image。
// 参考源码 getArea/getOs 与 docs/魔方云对接魔方财务.html 的配置项参数表：
// area/node 必传其一，os 必传，store 为系统盘所在存储（可选，不传随机）。
func (p *MoFangYunProvider) ListPlatformResources(ctx context.Context) (*upstream.PlatformResources, error) {
	out := &upstream.PlatformResources{}

	var areas struct {
		Data []struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			CountryCode string `json:"country_code"`
			Status      int    `json:"status"`
		} `json:"data"`
	}
	if err := p.call(ctx, "ListPlatformResources", http.MethodGet, "/areas",
		url.Values{"sort": {"asc"}, "list_type": {"all"}}, &areas); err != nil {
		return nil, err
	}
	for _, a := range areas.Data {
		out.Areas = append(out.Areas, upstream.PlatformResourceItem{
			Value:  strconv.FormatInt(a.ID, 10),
			Label:  platformLabel(strconv.FormatInt(a.ID, 10), a.CountryCode, a.Name),
			Status: platformStatus(a.Status),
		})
	}

	var nodes struct {
		Data []struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			AreaID int64  `json:"area_id"`
			Status int    `json:"status"`
		} `json:"data"`
	}
	if err := p.call(ctx, "ListPlatformResources", http.MethodGet, "/nodes",
		url.Values{"per_page": {"9999"}}, &nodes); err == nil {
		for _, n := range nodes.Data {
			out.Nodes = append(out.Nodes, upstream.PlatformResourceItem{
				Value:    strconv.FormatInt(n.ID, 10),
				Label:    n.Name,
				ParentID: strconv.FormatInt(n.AreaID, 10),
				Status:   platformStatus(n.Status),
			})
		}
	}

	var stores struct {
		Data []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			ShowName string `json:"show_name"`
			AreaID   int64  `json:"area_id"`
			Type     string `json:"type"`
			Enable   int    `json:"enable"`
		} `json:"data"`
	}
	if err := p.call(ctx, "ListPlatformResources", http.MethodGet, "/stores",
		url.Values{"per_page": {"9999"}}, &stores); err == nil {
		for _, s := range stores.Data {
			out.Stores = append(out.Stores, upstream.PlatformResourceItem{
				Value:    strconv.FormatInt(s.ID, 10),
				Label:    firstNonEmpty(s.ShowName, s.Name, s.Type),
				ParentID: strconv.FormatInt(s.AreaID, 10),
				Status:   platformStatus(s.Enable),
			})
		}
	}

	var images struct {
		Data []struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			Status int    `json:"status"`
			Group  struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"group"`
			// Info 镜像在各节点上的可用性：node_id + status（1=已下载可用）。
			// 实测（测试节点）：仅顶层 status=1 的镜像在节点上未必可用，直接下发会被
			// 面板拒绝（"镜像不可用，请在镜像管理中下载该镜像"），因此必须按节点判可用性。
			Info []struct {
				NodeID   int64 `json:"node_id"`
				Download int   `json:"download"`
				Status   int   `json:"status"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := p.call(ctx, "ListPlatformResources", http.MethodGet, "/image",
		url.Values{"per_page": {"9999"}, "sort": {"asc"}}, &images); err == nil {
		for _, img := range images.Data {
			// 与源码 getOs 同口径：跳过救援镜像（数量多且不可作常规系统）。
			if isRescueImage(img.Name) {
				continue
			}
			label := img.Name
			if img.Group.Name != "" {
				label = img.Group.Name + " / " + img.Name
			}
			// Group 既是展示前缀，也是用户侧分组下拉的分组标签（Ubuntu/Windows/CentOS）。
			groupName := img.Group.Name
			// 逐个节点铺开：同一镜像在不同节点上的可用性不同，按 (镜像, 节点) 各出一项，
			// 让规格模板能把 image 与 node 一起映射（platform_params 里的 os+node 组合）。
			emitted := false
			for _, info := range img.Info {
				if info.NodeID == 0 {
					continue
				}
				emitted = true
				out.Images = append(out.Images, upstream.PlatformResourceItem{
					Value:    strconv.FormatInt(img.ID, 10),
					Label:    label,
					ParentID: strconv.FormatInt(info.NodeID, 10),
					Group:    groupName,
					Status:   platformImageStatus(img.Status, info.Status),
				})
			}
			if !emitted {
				out.Images = append(out.Images, upstream.PlatformResourceItem{
					Value:  strconv.FormatInt(img.ID, 10),
					Label:  label,
					Group:  groupName,
					Status: platformImageStatus(img.Status, 0),
				})
			}
		}
	}
	return out, nil
}

// platformImageStatus 镜像在某节点上的可用性：顶层 status=1 且该节点 status=1 才算可用。
// 不可用的镜像仍会返回（status=offline），由前端置灰——直接隐藏会让运营不知道为什么镜像缺失。
func platformImageStatus(topStatus, nodeStatus int) string {
	if topStatus == 1 && nodeStatus == 1 {
		return "active"
	}
	return "offline"
}

// platformLabel 组合"ID|国家码^名称"展示名（与源码 getArea 的配置项口径一致）。
func platformLabel(id, countryCode, name string) string {
	if countryCode == "" {
		return id + "^" + name
	}
	return id + "|" + countryCode + "^" + name
}

// platformStatus 平台状态的统一展示：1/0 转 active/offline，其余原样。
func platformStatus(status int) string {
	if status == 1 {
		return "active"
	}
	return "offline"
}

// isRescueImage 判断是否为救援镜像（源码 getOs 的同口径：文件名含 Rescue）。
func isRescueImage(name string) bool {
	return strings.Contains(strings.ToLower(name), "rescue")
}

// ensureCloudUser 确保魔方云侧用户存在并返回用户 ID。
// 实测（测试节点 v3 面板）差异：源码用的 GET /user/check 在该版本返回 404，
// 而 POST /user 成功时会直接返回新用户 id，失败时返回 error（如"用户名已存在"）。
// 因此这里按"POST 取 id → 失败再按用户名在 GET /user 列表里回查"处理，
// 同时保留 /user/check 作为旧版本面板的兼容分支。
func (p *MoFangYunProvider) ensureCloudUser(ctx context.Context, opts map[string]string) (string, error) {
	username := firstNonEmpty(opts["cloud_username"], p.config.UserPrefix+firstNonEmpty(opts["username"], fmt.Sprintf("hs%d", time.Now().Unix())))
	password := firstNonEmpty(opts["cloud_password"], randPassword(12))
	userForm := url.Values{}
	userForm.Set("username", username)
	userForm.Set("email", firstNonEmpty(opts["email"], username+"@hostsent.local"))
	userForm.Set("status", "1")
	userForm.Set("real_name", firstNonEmpty(opts["real_name"], username))
	userForm.Set("password", password)
	if p.isAgent() {
		// 代理商账号建用户必须指定资源包（与源码一致）。
		rid := opts["resource_package"]
		if rid == "" {
			return "", &upstream.ProviderError{Op: "CreateInstance", Msg: "代理商账号创建用户需要配置资源包（resource_package）"}
		}
		userForm.Set("rid", rid)
	}
	// 1) 直接建用户；成功响应体带 id（可能是字符串或数字）。
	var created struct {
		ID json.RawMessage `json:"id"`
	}
	if err := p.call(ctx, "CreateInstance", http.MethodPost, "/user", userForm, &created); err == nil {
		if id := rawIDString(created.ID); id != "" && id != "0" {
			return id, nil
		}
	}
	// 2) 已存在（或响应无 id）：旧面板走 /user/check，新面板按用户名回查列表。
	var checked struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := p.call(ctx, "CreateInstance", http.MethodGet, "/user/check", url.Values{"username": {username}}, &checked); err == nil && checked.Data.ID > 0 {
		return strconv.FormatInt(checked.Data.ID, 10), nil
	}
	var listed struct {
		Data []struct {
			ID       json.RawMessage `json:"id"`
			Username string          `json:"username"`
		} `json:"data"`
	}
	// 列表接口的过滤参数不生效（实测返回全部用户），因此在客户端按用户名精确匹配。
	if err := p.call(ctx, "CreateInstance", http.MethodGet, "/user",
		url.Values{"username": {username}, "per_page": {"9999"}}, &listed); err != nil {
		return "", err
	}
	for _, u := range listed.Data {
		if u.Username != username {
			continue
		}
		if id := rawIDString(u.ID); id != "" && id != "0" {
			return id, nil
		}
	}
	return "", &upstream.ProviderError{Op: "CreateInstance", Msg: "魔方云用户创建失败：" + firstNonEmpty(checked.Msg, username)}
}

// rawIDString 把可能是字符串或数字的 id 字段统一为十进制字符串。
func rawIDString(raw json.RawMessage) string {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "" || s == "null" {
		return ""
	}
	return s
}

// waitStatus 轮询云主机状态直到达到期望值或超时。
func (p *MoFangYunProvider) waitStatus(ctx context.Context, instanceID, want string, tries int) bool {
	for i := 0; i < tries; i++ {
		time.Sleep(2 * time.Second)
		var st struct {
			Status string `json:"status"`
		}
		if err := p.call(ctx, "ResizeInstance", http.MethodGet, "/clouds/"+instanceID+"/status", nil, &st); err == nil && st.Status == want {
			return true
		}
	}
	return false
}

// cloudOptionKeys 与魔方云 /clouds 参数对齐的可配置项（键名与财务 configoptions 一致）。
var cloudOptionKeys = []string{
	"area", "node", "os", "cpu", "memory", "node_group", "ip_group", "node_priority",
	"nat_acl_limit", "nat_web_limit", "advanced_cpu", "advanced_bw",
	"system_disk_size", "network_type", "system_read_bytes_sec", "system_write_bytes_sec",
	"system_read_iops_sec", "system_write_iops_sec", "data_read_bytes_sec", "data_write_bytes_sec",
	"data_read_iops_sec", "data_write_iops_sec", "vpc", "vpc_name", "traffic_type",
	"in_bw", "out_bw", "ip_num", "traffic_quota", "backup_num", "snap_num", "flow_limit",
	"bw", "flow_way", "bind_mac", "cpu_limit", "port", "cpu_model", "rid", "ipv6_num",
	"type", "niccard", "link_clone", "reset_flow_day", "store", "gpu_num",
}

// collectOptions 归一化创建参数：优先取 Extra["configoptions"]（与财务 configoptions 同形），
// 再合并 Extra 顶层标量键。
func collectOptions(extra map[string]interface{}) map[string]string {
	opts := map[string]string{}
	set := func(k string, v interface{}) {
		switch t := v.(type) {
		case string:
			if t != "" {
				opts[k] = t
			}
		case float64:
			opts[k] = strconv.FormatFloat(t, 'f', -1, 64)
		case json.Number:
			opts[k] = t.String()
		case bool:
			if t {
				opts[k] = "1"
			} else {
				opts[k] = "0"
			}
		case nil:
		default:
			opts[k] = fmt.Sprintf("%v", t)
		}
	}
	if raw, ok := extra["configoptions"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			for k, v := range m {
				set(k, v)
			}
		}
	}
	for k, v := range extra {
		if k == "configoptions" || k == "other_data_disk" {
			continue
		}
		set(k, v)
	}
	// flow_way → traffic_type（1 进 / 2 出 / 3 进出汇总），与源码一致。
	if opts["traffic_type"] == "" && opts["flow_way"] != "" {
		switch opts["flow_way"] {
		case "in":
			opts["traffic_type"] = "1"
		case "out":
			opts["traffic_type"] = "2"
		default:
			opts["traffic_type"] = "3"
		}
	}
	// flow_limit → traffic_quota 别名。
	if opts["traffic_quota"] == "" && opts["flow_limit"] != "" {
		opts["traffic_quota"] = opts["flow_limit"]
	}
	// 文档口径键 → 平台写键的统一翻译（rid/bind_mac/reset_flow_day/
	// 磁盘性能四件套/系统盘 lin:win:/数据盘 size,store），见 option_alias.go。
	// 放在这里而不是各调用点，保证 CreateInstance / ResizeInstance 走同一套口径。
	applyOptionAliases(opts)
	return opts
}

// expandOtherDataDisk 将 other_data_disk（[{size, store}]）展开为表单数组语法。
func expandOtherDataDisk(form url.Values, extra map[string]interface{}) {
	raw, ok := extra["other_data_disk"]
	if !ok {
		if co, ok := extra["configoptions"].(map[string]interface{}); ok {
			raw, ok = co["data_disk_size"]
		}
		if !ok {
			return
		}
		switch t := raw.(type) {
		case string:
			if t != "" {
				form.Set("other_data_disk[0][size]", t)
			}
			return
		case float64:
			form.Set("other_data_disk[0][size]", strconv.FormatFloat(t, 'f', -1, 64))
			return
		default:
			return
		}
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return
	}
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if v, ok := m["size"]; ok {
			form.Set(fmt.Sprintf("other_data_disk[%d][size]", i), fmt.Sprintf("%v", v))
		}
		if v, ok := m["store"]; ok {
			form.Set(fmt.Sprintf("other_data_disk[%d][store]", i), fmt.Sprintf("%v", v))
		}
	}
}

// buildInstance 将魔方云云主机详情（扁平对象）映射为标准实例。
//
// 读取侧字段名与写入侧不对称（docs/实施计划/17 §6.1），且实测与文档略有差异，
// 以测试节点 v3 面板为准：
//
//	cpu=核数、memory=GB（**写入侧是 MB**，这里换算）、disk[] 中 type=system 的 size=系统盘 GB、
//	bw_group[0].in_bw=带宽 Mbps、area_name/node_name=区域/可用区、os_name=镜像文件名。
//
// 这些字段不填，实例列表与对账页的规格列就是空白，对账口径也随之失真。
func buildInstance(detail map[string]interface{}) *model.StandardInstance {
	id := jsonStr(detail, "id")
	inst := &model.StandardInstance{
		ProviderType: ProviderType,
		UpstreamID:   id,
		Name:         jsonStr(detail, "hostname"),
		PublicIP:     firstNonEmpty(jsonStr(detail, "mainip"), firstIPOf(detail, "ip")),
		PrivateIP:    firstIPOf(detail, "ipv6"),
		Region:       jsonStr(detail, "area_name"),
		Zone:         jsonStr(detail, "node_name"),
		Status:       model.InstanceStatusRunning,
		RawData:      detail,
	}
	inst.Specs.CPU = jsonInt(detail, "cpu")
	// 面板 memory 单位为 GB，标准规格统一 MB。
	inst.Specs.Memory = jsonInt(detail, "memory") * 1024
	inst.Specs.Disk, inst.Specs.DiskType = systemDiskOf(detail)
	inst.Specs.Bandwidth = bwGroupOf(detail)
	inst.Specs.OS = trimImageSuffix(jsonStr(detail, "os_name"))
	if inst.PublicIP == "" {
		inst.PublicIP = firstNonEmpty(jsonStr(detail, "mainip"), firstIPOf(detail, "ip"))
	}
	// 附加 IP：osuser/port 供控制台连接信息展示。
	if v := jsonStr(detail, "osuser"); v != "" {
		inst.RawData["osuser"] = v
	}
	if v := jsonStr(detail, "port"); v != "" {
		inst.RawData["port"] = v
	}
	// 状态：详情里的 status 字段（若有）
	if s := jsonStr(detail, "status"); s != "" {
		inst.Status = mapCloudStatus(s)
	}
	return inst
}

// systemDiskOf 从 disk[] 取系统盘容量与介质类型（type=system）。
func systemDiskOf(detail map[string]interface{}) (int, string) {
	raw, ok := detail["disk"].([]interface{})
	if !ok {
		return 0, ""
	}
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if jsonStr(m, "type") != "system" {
			continue
		}
		return jsonInt(m, "size"), jsonStr(m, "fs_type")
	}
	return 0, ""
}

// bwGroupOf 从 bw_group[] 取带宽（Mbps）；无分组时回退 in_bw/out_bw。
func bwGroupOf(detail map[string]interface{}) int {
	if raw, ok := detail["bw_group"].([]interface{}); ok && len(raw) > 0 {
		if m, ok := raw[0].(map[string]interface{}); ok {
			if bw := jsonInt(m, "in_bw"); bw > 0 {
				return bw
			}
			return jsonInt(m, "out_bw")
		}
	}
	if bw := jsonInt(detail, "in_bw"); bw > 0 {
		return bw
	}
	return jsonInt(detail, "out_bw")
}

// firstIPOf 从 ip 数组（元素为字符串或对象）取第一个地址。
func firstIPOf(detail map[string]interface{}, key string) string {
	raw, ok := detail[key].([]interface{})
	if !ok || len(raw) == 0 {
		return ""
	}
	switch v := raw[0].(type) {
	case string:
		return v
	case map[string]interface{}:
		return firstNonEmpty(jsonStr(v, "ip"), jsonStr(v, "address"), jsonStr(v, "value"))
	}
	return fmt.Sprintf("%v", raw[0])
}

// trimImageSuffix 去掉镜像文件扩展名，只留可读名称（如 CentOS-7.9.2111-x64.qcow2 → CentOS-7.9.2111-x64）。
func trimImageSuffix(name string) string {
	for _, suffix := range []string{".qcow2", ".raw", ".vhd", ".img"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return name
}

// jsonInt 从 map 取整数值（兼容 json.Number / float64 / 数字字符串）。
func jsonInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return i
		}
	}
	return 0
}

// mapCloudStatus 将魔方云状态映射为标准实例状态（参考源码 getCloudStatus）。
func mapCloudStatus(s string) model.StandardInstanceStatus {
	switch s {
	case "on":
		return model.InstanceStatusRunning
	case "off", "suspend", "paused":
		return model.InstanceStatusStopped
	case "wait_reboot", "task":
		return model.InstanceStatusRestarting
	case "cold_migrate", "hot_migrate":
		return model.InstanceStatusRunning
	default:
		return model.InstanceStatusStopped
	}
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
	case bool:
		if t {
			return "1"
		}
		return "0"
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

// sanitizeHostname 生成面板可接受的云主机名。
// 面板规则（实测）：字母开头，只允许字母/数字/_ - .，6 位及以上。
// 中文商品名、含空格或过短的名字一律回退为带时间戳的唯一主机名，避免开通被 400 拒绝。
func sanitizeHostname(name string) string {
	name = strings.TrimSpace(name)
	if isValidHostname(name) {
		return name
	}
	// 取合法字符拼前缀（保留可读性），不足或为空则统一前缀。
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
		if b.Len() >= 20 {
			break
		}
	}
	prefix := b.String()
	prefix = strings.TrimLeft(prefix, "-_.")
	if len(prefix) == 0 || !isHostnameStart(prefix[0]) {
		prefix = "hs"
	}
	prefix = prefix[:min(len(prefix), 20)]
	return fmt.Sprintf("%s-%d", prefix, time.Now().Unix()%10000000)
}

// isValidHostname 校验面板主机名规则。
func isValidHostname(name string) bool {
	if len(name) < 6 || len(name) > 64 {
		return false
	}
	if !isHostnameStart(name[0]) {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

// isHostnameStart 首位必须是英文字母。
func isHostnameStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// randStr 生成随机小写字母数字串（主机名等宽松场景用）。
func randStr(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// randPassword 生成满足魔方云密码策略的随机密码（rootpass / 云用户密码）：
// 实测策略要求 6 位以上、且同时包含大写字母、小写字母与数字，
// 否则面板直接 400（"密码必须…包含小写字母，大写字母，数字"），开通静默失败。
func randPassword(n int) string {
	const (
		lower = "abcdefghijkmnopqrstuvwxyz"
		upper = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		digit = "23456789"
		all   = lower + upper + digit
	)
	if n < 8 {
		n = 8
	}
	b := make([]byte, n)
	// 前三位各取一类，保证策略命中；其余随机填充后打乱位置。
	b[0] = lower[rand.Intn(len(lower))]
	b[1] = upper[rand.Intn(len(upper))]
	b[2] = digit[rand.Intn(len(digit))]
	for i := 3; i < n; i++ {
		b[i] = all[rand.Intn(len(all))]
	}
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}
