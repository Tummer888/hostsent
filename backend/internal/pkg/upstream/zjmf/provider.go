// Package zjmf 为魔方财务（ZJMF）上游提供商的适配器实现。
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
package zjmf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/safego"
	"hostsent/backend/internal/pkg/traceid"
	"hostsent/backend/internal/pkg/upstream"
)

// ProviderType 魔方财务提供商类型标识
const ProviderType = "zjmf"

// LegacyProviderType 历史类型码（重命名前叫 mofangfinance）。存量渠道行可能仍是它，
// 工厂构建、描述符查询与能力判定都要认这个别名，直到数据迁移（迁移 088）跑完。
const LegacyProviderType = "mofangfinance"

// jwtTTL 与源码 zjmfApiLogin 一致：JWT 缓存 5400 秒。
const jwtTTL = 90 * time.Minute

func init() {
	// 类型码别名：重命名前该平台叫 mofangyun/mofangfinance，存量渠道行仍可能是旧码，
	// 别名让构建与描述符查询在数据迁移跑完前也认旧码。
	upstream.RegisterTypeAlias(LegacyProviderType, ProviderType)
	upstream.GetProviderManager().RegisterFactory(ProviderType, func(cfg *upstream.ProviderConfig) upstream.Provider {
		return NewProvider(cfg)
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
			// 维护类：上游 /provision/default 支持 func=reinstall|crack_pass，
			// 源码 Host::reinstall / Host::crackPass 的 zjmf_api 分支即走这两条指令。
			upstream.OpReinstall, upstream.OpResetPassword,
			// 性能监控读数（只读）：目录透传 host/header.module_chart，
			// 数据透传 provision/chart/{host_id}；上游未装图表模块时返回空目录。
			upstream.OpMetrics,
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
			"source": "实测（依据 zjmf/provider.go 与上游源码 app/zjmf.php、app/common/logic/Host.php）",
			"paths": []string{"cart/all", "cart/get_product_config", "cart/credit", "credit_record",
				"v1/transactions/funds", "v1/hosts", "api/product/proinfo",
				"host/header", "/dcim/on", "/dcim/off", "/dcim/reboot",
				"/host/renew", "/apply_credit", "/host/cancel", "/provision/default"},
			"note": "登录换 JWT（Bearer），status=405 表示 JWT 失效需重登；余额=cart/credit；账本=credit_record(消费)+v1/transactions/funds(充值)+v1/hosts(到期金额)（成本管理口径，doc111 §5.2）；续费=host/renew+apply_credit(1001)；暂停/恢复=provision/default func=suspend|unsuspend；终止=host/cancel type=Immediate",
		},
	})
}

// Provider 魔方财务适配器
type Provider struct {
	config *upstream.ProviderConfig
	client *http.Client

	jwtMu    sync.Mutex
	jwt      string
	jwtUntil time.Time
}

// NewProvider 按配置创建魔方财务适配器实例
func NewProvider(config *upstream.ProviderConfig) *Provider {
	timeout := time.Duration(config.Timeout) * time.Second
	if config.Timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Provider{
		config: config,
		client: &http.Client{Timeout: timeout},
	}
}

// GetType 返回提供商类型标识
func (p *Provider) GetType() string { return ProviderType }

// GetName 返回提供商名称
func (p *Provider) GetName() string { return "魔方财务" }

// Capabilities 返回能力描述符（T2.1）。
func (p *Provider) Capabilities() upstream.CapabilityDescriptor {
	d, _ := upstream.Descriptor(ProviderType)
	return d
}

// isResource 是否资源型上游（is_resource=1，登录与业务路径不同）。
func (p *Provider) isResource() bool {
	return p.config.UpstreamType == "resource"
}

// baseURL 去掉协议前缀与尾部斜杠，并按配置拼接 https/端口（与财务 servers 表同构）。
func (p *Provider) baseURL() (string, error) {
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
func (p *Provider) loginURL(base string) string {
	if p.isResource() {
		return base + "/resource_login"
	}
	return base + "/zjmf_api_login"
}

// login 获取上游 JWT，带本地缓存；force 时强制重新登录（对应 zjmfApiLogin）。
func (p *Provider) login(ctx context.Context, force bool) (string, error) {
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
	var shell Resp[json.RawMessage]
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
func (p *Provider) httpForm(ctx context.Context, method, rawURL string, form url.Values, bearer string) ([]byte, int, error) {
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
	started := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		upstream.Capture(ctx, method, rawURL, upstream.RequestBodyOf(req.GetBody), 0, nil, err, time.Since(started))
		return nil, 0, &upstream.ProviderError{Op: "request", Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	upstream.Capture(ctx, method, rawURL, upstream.RequestBodyOf(req.GetBody), resp.StatusCode, respBody, err, time.Since(started))
	if err != nil {
		return nil, resp.StatusCode, &upstream.ProviderError{Op: "request", Err: err}
	}
	return respBody, resp.StatusCode, nil
}

// providerInfo 采集记录上要带的渠道标识。
func (p *Provider) providerInfo() (uint, string, string) {
	name := firstNonEmpty(p.config.Name, "魔方财务")
	return p.config.ID, name, ProviderType
}

// call 调用上游业务接口（zjmfCurl 等价实现）：
// 自动登录 → 业务请求 → status==405 时强制重登录重试一次 → 解析统一响应壳。
// out 非 nil 时将 data 字段解析到 out。
// 成功状态码限定 200（业务通用成功码）；需要额外状态码（如支付成功的 1001）的接口
// 请改用 callShell 自行判定。
func (p *Provider) call(ctx context.Context, op, method, path string, form url.Values, out interface{}) error {
	shell, status, body, err := p.callShell(ctx, op, method, path, form)
	if err != nil {
		return err
	}
	if status >= 400 {
		return &upstream.ProviderError{Op: op, StatusCode: status, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
	}
	if shell.Status != StatusOK && shell.Code != StatusOK && (shell.Status != 0 || shell.Code != 0) {
		return &upstream.ProviderError{Op: op, Code: shell.Status.Int(), Msg: firstNonEmpty(shell.Msg, "上游接口调用失败")}
	}
	return p.unmarshalData(op, shell, out)
}

// callShell 执行一次上游业务调用并返回原始响应壳，不做业务状态码判定。
// 供 call（判定 200）与 applyCredit（额外接受 1001 支付成功码）复用。
//
// 埋点（doc92 §3.2）：本函数的返回值就是「HTTP 2xx 也可能是业务失败」的原始证据，
// 因此采集记录的业务判定在这里显式给出（FinishBusiness），而不是靠 err 推断 ——
// 否则上游返回 {status:500,msg:"账号密码错误"} 会被记成成功。
func (p *Provider) callShell(ctx context.Context, op, method, path string, form url.Values) (shell Resp[json.RawMessage], status int, body []byte, err error) {
	ctx, tr := upstream.NewTrace(ctx, op, traceid.From(ctx))
	tr.SetProvider(p.providerInfo)
	defer func() {
		if err != nil {
			tr.FinishBusiness(err, false, "", "")
			return
		}
		// 业务成功码有两个：200（通用成功）与 1001（余额支付完成，仅 apply_credit/cart/settle
		// 的 0 元单返回）。若漏掉 1001，"支付成功"会被采集成失败记录 —— 而这张表正是
		// 运维排查开通问题的入口，误报失败会把排查引向错误方向。
		success := status < 400 && (shell.Status == StatusOK || shell.Code == StatusOK ||
			shell.Status == StatusPaidSuccess || shell.Code == StatusPaidSuccess)
		code := ""
		if shell.Status != 0 {
			code = strconv.Itoa(shell.Status.Int())
		}
		tr.FinishBusiness(nil, success, code, shell.Msg)
	}()

	base, err := p.baseURL()
	if err != nil {
		return shell, 0, nil, err
	}
	jwt, err := p.login(ctx, false)
	if err != nil {
		return shell, 0, nil, &upstream.ProviderError{Op: op, Err: err}
	}
	body, status, err = p.callOnce(ctx, base, jwt, method, path, form)
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
		shell = Resp[json.RawMessage]{}
		_ = json.Unmarshal(body, &shell)
		if shell.Status == StatusTokenExpired {
			return shell, status, body, &upstream.ProviderError{Op: op, Msg: "API账号密码错误"}
		}
	}
	return shell, status, body, nil
}

// callTopLevel 调用「响应不在 data 壳内」的接口。
//
// 实测 cart/index 不带 data 字段（商品数组直接挂在顶层 products），只认 data 的
// unmarshalData 会静默解出空结果 —— 价格回填与在售判定都会悄悄失效。
// 这里 data 有值走原路径，没有则把整个响应体解到 out（多余字段自动忽略）。
func (p *Provider) callTopLevel(ctx context.Context, op, method, path string, form url.Values, out interface{}) error {
	shell, status, body, err := p.callShell(ctx, op, method, path, form)
	if err != nil {
		return err
	}
	if status >= 400 {
		return &upstream.ProviderError{Op: op, StatusCode: status, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
	}
	if shell.Status != StatusOK && shell.Code != StatusOK && (shell.Status != 0 || shell.Code != 0) {
		return &upstream.ProviderError{Op: op, Code: shell.Status.Int(), Msg: firstNonEmpty(shell.Msg, "上游接口调用失败")}
	}
	if err := p.unmarshalData(op, shell, out); err != nil {
		return err
	}
	if len(shell.Data) == 0 || string(shell.Data) == "null" {
		if uerr := json.Unmarshal(body, out); uerr != nil {
			return &upstream.ProviderError{Op: op, Err: fmt.Errorf("解析上游响应失败: %w", uerr)}
		}
	}
	return nil
}

// unmarshalData 将响应壳的 data 字段解析到 out。
func (p *Provider) unmarshalData(op string, shell Resp[json.RawMessage], out interface{}) error {
	if out != nil && len(shell.Data) > 0 && string(shell.Data) != "null" {
		if err := json.Unmarshal(shell.Data, out); err != nil {
			return &upstream.ProviderError{Op: op, Err: fmt.Errorf("解析上游响应 data 失败: %w", err)}
		}
	}
	return nil
}

// callOnce 以给定 JWT 执行一次业务调用。GET 的表单参数转 query，路径可带前导斜杠。
func (p *Provider) callOnce(ctx context.Context, base, jwt, method, path string, form url.Values) ([]byte, int, error) {
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
func (p *Provider) HealthCheck(ctx context.Context) error {
	_, err := p.login(ctx, true)
	return err
}

// GetAccountInfo 读取「我方（下游账号）在该上游的账户余额」（AccountReader，doc111 §5）。
//
// 协议取自上游源码：
//
//	GET {base}/cart/credit        （Bearer JWT）
//	  data.credit      客户余额（clients.credit；上游以 decimal 存储，JSON 可能是字符串 "199.00"）
//	  data.currency    币种 {id, code, prefix, suffix}；客户未设币种且无默认币种时为 []
//
// 对应旧 PHP 项目的取余额实现
// （app/admin/controller/ZjmfFinanceApiController::upstreamCredit() →
// zjmfCurl($id, "cart/credit", [], 30, "GET")），服务端实现为
// app/home/controller/CartController::getCredit() —— 它按 JWT 里的客户 uid 取 clients.credit，
// 而 /zjmf_api_login 正是以客户身份签发 JWT，因此这里拿到的就是「我方在上游的余额」。
// 资源型登录（/resource_login，type=agent）同样是客户身份，一并适用。
//
// 与其它读接口一样只读不动账：成本管理用它落余额快照，再由快照推算消耗。
func (p *Provider) GetAccountInfo(ctx context.Context) (*upstream.AccountInfo, error) {
	var data struct {
		// Credit 用指针区分「上游返回 0」与「响应里根本没有这个字段」：
		// 后者多半是账号没开通客户余额权限，静默落 0 会被成本台账读成「钱花光了」。
		Credit   *FlexFloat      `json:"credit"`
		Currency json.RawMessage `json:"currency"`
	}
	if err := p.call(ctx, "GetAccountInfo", http.MethodGet, "cart/credit", nil, &data); err != nil {
		return nil, err
	}
	if data.Credit == nil {
		return nil, &upstream.ProviderError{Op: "GetAccountInfo", Msg: "上游未返回余额字段（cart/credit 响应缺少 credit），请确认该 API 账号具备客户余额权限"}
	}
	return &upstream.AccountInfo{
		Balance:  float64(*data.Credit),
		Currency: accountCurrency(data.Currency),
	}, nil
}

// accountCurrency 解析 cart/credit 的 data.currency：取 ISO 码并大写；
// 上游在客户未设币种且无默认币种时返回 []（空数组），此时回落 CNY。
func accountCurrency(raw json.RawMessage) string {
	if len(raw) > 0 {
		var cur struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(raw, &cur); err == nil {
			if code := strings.ToUpper(strings.TrimSpace(cur.Code)); code != "" {
				return code
			}
		}
	}
	return "CNY"
}

// ListConsumptionRecords 余额支付消费流水（成本管理主口径）。
//
// 上游接口：GET /credit_record（home/user_invoice/creditRecord，非 openapi 路由）
//
//	→ data.accounts[] = {id, relid(上游账单号), create_time, description(余额支付),
//	                     type(订购产品/续费), amount, refund[](关联退款行，金额为负)}
//
// 服务端 SQL 只看 descriptions 含 "Credit Applied to Invoice #" / "... to Renew Invoice #" 的行，
// 因此返回的就是「本账号用余额支付的每一笔开通/续费」。
//
// 分页：上游按 page/limit 返回并在 data.total 给总数（实测 limit=60 稳定）。
func (p *Provider) ListConsumptionRecords(ctx context.Context, page, limit int) ([]upstream.LedgerEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	var resp struct {
		Total    int            `json:"total"`
		Accounts []CreditRecord `json:"accounts"`
	}
	if err := p.call(ctx, "ListConsumptionRecords", http.MethodGet, "credit_record", url.Values{
		"page":  {strconv.Itoa(page)},
		"limit": {strconv.Itoa(limit)},
	}, &resp); err != nil {
		return nil, 0, err
	}
	out := make([]upstream.LedgerEntry, 0, len(resp.Accounts))
	for _, item := range resp.Accounts {
		entry := upstream.LedgerEntry{
			ExternalID:  strconv.FormatInt(item.ID, 10),
			Kind:        upstream.LedgerKindConsume,
			OccurredAt:  time.Unix(item.CreateTime, 0).UTC(),
			Amount:      math.Round(float64(item.Amount)*100) / 100,
			Category:    item.Type,
			RefNo:       strconv.FormatInt(item.RelID, 10),
			Description: item.Description,
		}
		// 退款行金额为负，取绝对值累计为本笔的已退金额（消费净额 = amount − refund）。
		for _, line := range item.Refund {
			if line.Amount < 0 {
				entry.RefundAmount += -float64(line.Amount)
			} else {
				entry.RefundAmount += float64(line.Amount)
			}
		}
		entry.RefundAmount = math.Round(entry.RefundAmount*100) / 100
		out = append(out, entry)
	}
	return out, resp.Total, nil
}

// ListTopupRecords 充值/入账流水。
//
// 上游接口：GET /v1/transactions/funds（openapi/Invoices/accountsRecord）
//
//	→ data.accounts[] = {id, invoice_id, pay_time, payment_zh, description, type, amount_in, trans_id}
//
// 描述区分为「用户充值」「人工入账」，支付方式在 payment_zh（支付宝支付…）。
func (p *Provider) ListTopupRecords(ctx context.Context, page, limit int) ([]upstream.LedgerEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	var resp struct {
		Total    int           `json:"total"`
		Accounts []TopupRecord `json:"accounts"`
	}
	if err := p.call(ctx, "ListTopupRecords", http.MethodGet, "v1/transactions/funds", url.Values{
		"page":  {strconv.Itoa(page)},
		"limit": {strconv.Itoa(limit)},
	}, &resp); err != nil {
		return nil, 0, err
	}
	out := make([]upstream.LedgerEntry, 0, len(resp.Accounts))
	for _, item := range resp.Accounts {
		desc := item.Description
		if item.PaymentZh != "" {
			desc = strings.TrimSpace(desc + " " + item.PaymentZh)
		}
		out = append(out, upstream.LedgerEntry{
			ExternalID:  strconv.FormatInt(item.ID, 10),
			Kind:        upstream.LedgerKindTopup,
			OccurredAt:  time.Unix(int64(item.PayTime), 0).UTC(),
			Amount:      math.Round(float64(item.AmountIn)*100) / 100,
			Category:    item.Description,
			RefNo:       item.TransID,
			Description: desc,
		})
	}
	return out, resp.Total, nil
}

// ListDueHosts 上游主机清单的续费信息（余额水位告警：余额够不够付未来 30 天的续费）。
//
// 上游接口：GET /v1/hosts（openapi/Host/getHosts）→ data.host[] 每台机带
// amount（下一期续费金额）、nextduedate（到期时间戳）、domainstatus。
// 本方法只取告警需要的字段，不落实例同步（实例台账走同步引擎，职责分离）。
func (p *Provider) ListDueHosts(ctx context.Context) ([]upstream.DueHost, error) {
	const perPage = 100
	out := make([]upstream.DueHost, 0, 32)
	for page := 1; page <= 20; page++ {
		var resp struct {
			Total int            `json:"total"`
			Host  []UpstreamHost `json:"host"`
		}
		if err := p.call(ctx, "ListDueHosts", http.MethodGet, "v1/hosts", url.Values{
			"page":  {strconv.Itoa(page)},
			"limit": {strconv.Itoa(perPage)},
		}, &resp); err != nil {
			return nil, err
		}
		for _, h := range resp.Host {
			out = append(out, upstream.DueHost{
				UpstreamID:  strconv.FormatInt(h.ID, 10),
				ProductName: h.ProductName,
				Domain:      h.Domain,
				Amount:      math.Round(float64(h.Amount)*100) / 100,
				NextDueAt:   time.Unix(int64(h.NextDueDate), 0).UTC(),
				Status:      h.DomainStatus,
			})
		}
		if len(resp.Host) == 0 || len(out) >= resp.Total {
			break
		}
	}
	return out, nil
}

// ListProducts 拉取上游商品列表并补全分类/价格/规格（全量商品接口）。
//
// 财务型上游（对齐上游自身下游对接逻辑 app/zjmf.php 的 getZjmfUpstreamProducts +
// getZjmfUpstreamProductConfig + upstreamBates）：
//  1. GET cart/all                     —— 未隐藏分组（分类名）+ 前台在售分组集合；
//  2. GET api/product/proinfo          —— 全量商品 id（不过滤 hidden/retired）；
//  3. GET api/product/prodetail?pids[]= —— 分批(<500)拉取完整详情：product_pricings（标价）、
//     config_groups（CPU/内存/系统盘等开通规格）、customfields、advanced、pay_type、description；
//  4. GET cart/get_product_config?pid=  —— **逐商品**取 flag（本客户折扣）：折扣记录按
//     (客户分组 × 商品) 命中，同分组内商品折扣可能完全不同，按分组采样会算错成本价；
//  5. GET cart/index?gid=               —— 逐分组取上游前台展示价（含默认配置项价）与在售清单，
//     用于回填「价格只挂在配置项子项」的活动款（product_pricings 全 0/-1）。
//
// 价格口径（对齐上游 zjmf.php::upstreamBates）：
//   - cost_price = 折后价（我方实际采购价，raw_specs.product_pricings）；
//   - sale_price = 上游标价（raw_specs.product_pricings_list），供展示与折扣对比。
//
// 资源型上游：GET resource/agentproductsarray（data 兼容数组/包裹两种形态）。
func (p *Provider) ListProducts(ctx context.Context) ([]*model.StandardProduct, error) {
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

	// 1) 商品分组（分类）：cart/all 只含**未隐藏分组**，其键集合同时是「前台在售分组」。
	groups, groupsOK := p.fetchCartGroups(ctx)

	// 2) 全量商品 id
	var proinfo OpenapiProinfoResp
	if err := p.call(ctx, "ListProducts", http.MethodGet, "api/product/proinfo", nil, &proinfo); err != nil {
		return nil, err
	}
	// 3) 分批拉取完整详情（价格 + 配置规格）
	details := p.fetchAllDetails(ctx, proinfo.Info)

	// 4) 逐商品探测本客户折扣；
	// 5) 逐分组拉上游前台商品目录（补活动款价格 + 在售判定旁证）。
	flags := p.fetchProductFlags(ctx, details)
	catalog := p.fetchCatalogRows(ctx, details)

	// 以 proinfo 的商品清单为准遍历，而不是 details 的键。
	//
	// 关键（实测故障）：个别商品详情拉取失败时，details 里就没有它 —— 用 details
	// 遍历会让这些商品从本次结果里整体消失，同步引擎的全量对账据此判定"上游已下架"
	// 把本地行置为未在售（实测一次超时导致 200 个商品被误下线）。商品清单来自
	// proinfo（一次请求就能拿全），是"上游有哪些商品"的权威来源；详情缺失只是
	// 暂时拿不到价格/规格，不该当成商品消失。
	for _, info := range proinfo.Info {
		pid := strconv.FormatInt(info.ID, 10)
		d, hasDetail := details[pid]
		// 详情缺失：保留该商品在清单中的存在，名称取 proinfo 的值，
		// 价格/规格留空（下次同步补上），避免被误判下架。
		name := info.Name
		if hasDetail && strings.TrimSpace(d.Name) != "" {
			name = d.Name
		}
		var spec model.StandardProductSpec
		if hasDetail {
			spec = parseCloudSpecs(d.ConfigGroups)
			if spec.OS == "" {
				spec.OS = d.Type
			}
		}
		flag := flags[pid]
		cat, hasCat := catalog[pid]
		listRows, priceSource := resolveListRows(d.ProductPricings, cat, hasCat)
		costRows := scalePricingRows(listRows, flag)
		groupName, groupSource := resolveGroupName(d.GID, name, groups, groupsOK)
		visibility := classifyVisibility(d, groups, groupsOK)
		rawSpecs := buildProductRawSpecs(d, productMeta{
			GroupName: groupName, GroupSource: groupSource,
			ListRows: listRows, CostRows: costRows, Flag: flag,
			PriceSource: priceSource, Visibility: visibility,
			Catalog: cat, HasCatalog: hasCat,
		})
		// 详情缺失标记：本次的价格/规格/config_groups 都是空的（占位值），
		// 同步引擎据此**只保留商品存在性、不覆盖本地已有的价格与开通参数** ——
		// 否则一次超时就会把 config_groups 抹掉，商品从此开不了通。
		if !hasDetail {
			rawSpecs["detail_missing"] = true
		}
		products = append(products, &model.StandardProduct{
			ProviderType: ProviderType, UpstreamID: pid, Name: name,
			CostPrice: upstreamPrice(costRows), SalePrice: upstreamPrice(listRows),
			Specs: spec, Status: statusForVisibility(visibility),
			RawSpecs: rawSpecs,
		})
	}
	return products, nil
}

// productMeta 商品落 raw_specs 的元信息（分类来源、价格来源、在售判定、上游前台目录价）。
// 单列结构只为让 buildProductRawSpecs 的入参可读（此前 5 个位置参数已到可读性上限）。
type productMeta struct {
	GroupName   string
	GroupSource string // upstream=上游公示；derived=按商品名派生；fallback=分组 #id
	ListRows    []PricingRow
	CostRows    []PricingRow
	Flag        SaleFlag
	PriceSource string // pricings=商品周期价；catalog=上游前台价回填；none=上游未定价
	Visibility  string // visible/group_hidden/product_hidden/retired
	Catalog     CartIndexProduct
	HasCatalog  bool
}

// 在售判定与价格/分类来源的取值（落 raw_specs，前端与同步后处理按值判定）。
const (
	visibilityVisible   = "visible"
	visibilityGroupHid  = "group_hidden"
	visibilityProductHi = "product_hidden"
	visibilityRetired   = "retired"

	priceSourcePricings = "pricings"
	priceSourceCatalog  = "catalog"
	priceSourceNone     = "none"

	groupSourceUpstream = "upstream"
	groupSourceDerived  = "derived"
	groupSourceFallback = "fallback"

	// catalogProbeConcurrency 上游探测并发度（限流保守取值）。
	catalogProbeConcurrency = 6
)

// classifyVisibility 判定商品在上游前台是否真的在售。
// 注意 proinfo 永远返回 hidden=0/retired=0，只有 prodetail 的这两列可信；
// 分组层面：cart/all 只列未隐藏分组，商品 gid 不在其中 → 整组被上游隐藏（不上前台）。
func classifyVisibility(d UpstreamProductDetail, groups map[int64]string, groupsOK bool) string {
	switch {
	case d.Retired > 0:
		return visibilityRetired
	case d.Hidden > 0:
		return visibilityProductHi
	case groupsOK && len(groups) > 0:
		if _, ok := groups[d.GID]; !ok {
			return visibilityGroupHid
		}
	}
	return visibilityVisible
}

// statusForVisibility 在售判定 → 标准商品状态（同步引擎把 active/inactive 归一为 1/0）。
func statusForVisibility(visibility string) string {
	if visibility == visibilityVisible {
		return "active"
	}
	return "inactive"
}

// resolveGroupName 解析商品分类名与来源：
//   - 上游公示（cart/all 的未隐藏分组）→ 直接用分组名，来源 upstream；
//   - 分组被上游隐藏 → 按商品名前缀派生（「香港|二区·轻量 8-8」→「香港|二区」），
//     来源 derived，前端打「派生」标记（是近似标签，不是上游公示名）；
//   - 派生不出（名字里没有「地区|区域」段）→「分组 #<gid>」，来源 fallback。
func resolveGroupName(gid int64, productName string, groups map[int64]string, groupsOK bool) (string, string) {
	if groupsOK {
		if name := strings.TrimSpace(groups[gid]); name != "" {
			return name, groupSourceUpstream
		}
	}
	if label := deriveGroupLabel(productName); label != "" {
		return label, groupSourceDerived
	}
	return fmt.Sprintf("分组 #%d", gid), groupSourceFallback
}

// groupLabelRuneLimit 派生分类名长度上限（「宁波电信|一区」6 字，留余量到 16）。
const groupLabelRuneLimit = 16

// deriveGroupLabel 从商品名前缀派生分组标签：取首个空格/「·」/「-」之前、且含「|」的片段。
// 只认「地区|区域」这种惯例（香港|二区、宁波电信|一区、美国|五区）；
// 派生不出返回空串（调用方回退「分组 #id」），避免把营销词（2025·海外…）当分类名。
func deriveGroupLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	cut := len(name)
	for i, r := range name {
		if r == ' ' || r == '·' || r == '　' || r == '-' {
			cut = i
			break
		}
	}
	head := strings.TrimSpace(name[:cut])
	if head == "" || !strings.Contains(head, "|") {
		return ""
	}
	if utf8.RuneCountInString(head) > groupLabelRuneLimit {
		return ""
	}
	return head
}

// resolveListRows 解析商品标价周期行：
//   - product_pricings 有正价 → 直接用（口径最细，含各周期）；
//   - 全为 0/-1（活动款价格只挂在配置项子项上）→ 用上游前台展示价回填其实际售卖周期，
//     来源标 catalog，前端提示「价格取自上游前台展示价」；
//   - 前台目录也没有 → 保持原样（上游未定价），来源 none。
func resolveListRows(raw []PricingRow, cat CartIndexProduct, hasCat bool) ([]PricingRow, string) {
	if upstreamPrice(raw) > 0 {
		return raw, priceSourcePricings
	}
	if !hasCat || float64(cat.ProductPrice) <= 0 {
		return raw, priceSourceNone
	}
	cycle := strings.ToLower(strings.TrimSpace(cat.BillingCycle))
	if cycle == "" {
		return raw, priceSourceNone
	}
	row := PricingRow{}
	if len(raw) > 0 {
		row = raw[0].Clone() // 必须克隆：原始行要按上游未改动形态落 raw_specs
	}
	row[cycle] = float64(cat.ProductPrice)
	if key := cycleSetupField(cycle); key != "" && float64(cat.SetupFee) > 0 {
		row[key] = float64(cat.SetupFee)
	}
	return []PricingRow{row}, priceSourceCatalog
}

// cycleSetupField 周期的安装费字段名（annually → asetupfee）；不认识的周期返回空。
func cycleSetupField(cycle string) string {
	prefix, ok := cycleSetupPrefix[cycle]
	if !ok {
		return ""
	}
	return prefix + "setupfee"
}

// cycleSetupPrefix 周期 → 价格字段前缀（对齐上游 pricing 表列名：asetupfee/msetupfee...）。
var cycleSetupPrefix = map[string]string{
	"hour": "h", "day": "d", "monthly": "m", "quarterly": "q", "semiannually": "s",
	"annually": "a", "biennially": "b", "triennially": "t", "onetime": "o",
}

// fetchCartGroups 拉取商品分组（分类）：返回 gid → 分组名，以及是否取到
// （取不到时不做分类/在售判定，避免 cart/all 一次失败把全量商品误标为未在售）。
func (p *Provider) fetchCartGroups(ctx context.Context) (map[int64]string, bool) {
	var resp CartAllResp
	if err := p.call(ctx, "ListProducts", http.MethodGet, "cart/all", nil, &resp); err != nil {
		return nil, false
	}
	out := make(map[int64]string, len(resp.Products))
	for _, g := range resp.Products {
		if g.ID == 0 {
			continue
		}
		out[g.ID] = strings.TrimSpace(g.Name)
	}
	return out, true
}

// fetchProductFlags 逐商品探测本客户折扣（flag），返回 pid → 折扣。
// 探测失败的 pid 视为无折扣：宁可按标价高估成本，也不按未知折扣压低成本。
func (p *Provider) fetchProductFlags(ctx context.Context, details map[string]UpstreamProductDetail) map[string]SaleFlag {
	out := make(map[string]SaleFlag, len(details))
	if len(details) == 0 {
		return out
	}
	type result struct {
		pid  string
		flag SaleFlag
	}
	ch := make(chan result, len(details))
	sem := make(chan struct{}, catalogProbeConcurrency)
	var wg sync.WaitGroup
	for pid := range details {
		wg.Add(1)
		// 无 logger 注入的基础库 goroutine：Recover 降级输出 stderr，panic 不击穿同步流程。
		safego.Go(nil, func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ch <- result{pid: pid, flag: p.fetchProductFlag(ctx, pid)}
		})
	}
	wg.Wait()
	close(ch)
	for r := range ch {
		if r.flag.Enabled() {
			out[r.pid] = r.flag
		}
	}
	return out
}

// fetchCatalogRows 逐分组拉上游前台商品目录（cart/index?gid=），返回 pid → 目录行。
// 隐藏分组也能取到（上游对直连 gid 不拦），但隐藏商品本身不在其中。
// 失败不阻塞同步：价格回退到周期价、在售判定只看 prodetail 的 hidden/retired。
func (p *Provider) fetchCatalogRows(ctx context.Context, details map[string]UpstreamProductDetail) map[string]CartIndexProduct {
	gidSet := map[int64]struct{}{}
	for _, d := range details {
		gidSet[d.GID] = struct{}{}
	}
	out := make(map[string]CartIndexProduct, len(details))
	if len(gidSet) == 0 {
		return out
	}
	type result struct {
		rows []CartIndexProduct
	}
	ch := make(chan result, len(gidSet))
	sem := make(chan struct{}, catalogProbeConcurrency)
	var wg sync.WaitGroup
	for gid := range gidSet {
		wg.Add(1)
		safego.Go(nil, func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var resp CartIndexResp
			if err := p.callTopLevel(ctx, "ListProducts", http.MethodGet, "cart/index",
				url.Values{"gid": {strconv.FormatInt(gid, 10)}}, &resp); err != nil {
				ch <- result{}
				return
			}
			ch <- result{rows: resp.Products}
		})
	}
	wg.Wait()
	close(ch)
	for r := range ch {
		for _, row := range r.rows {
			if row.PID() > 0 {
				out[strconv.FormatInt(row.PID(), 10)] = row
			}
		}
	}
	return out
}

// scalePricingRows 按折扣换算全部标价行；无折扣时原样返回（避免无谓克隆）。
func scalePricingRows(rows []PricingRow, flag SaleFlag) []PricingRow {
	if !flag.Enabled() {
		return rows
	}
	out := make([]PricingRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Scale(flag.Factor()))
	}
	return out
}

// buildProductRawSpecs 组装资源商品原始规格快照（落 resource_products.raw_specs）。
//
// 下游消费者契约（勿随意改名）：
//   - gid / group_name：分类（resource_products.group_id / group_name 由此派生）；
//   - product_pricings：我方采购价（折后）——周期价格矩阵与成本由此推导；
//   - product_pricings_list / list_price：上游标价，供展示与折扣对比；
//   - config_groups：开通参数（含选项/子项的上游 id，下单时必须用它们）；
//   - description：上游富文本描述；pay_type：可售周期与是否可续费。
func buildProductRawSpecs(d UpstreamProductDetail, meta productMeta) map[string]interface{} {
	raw := map[string]interface{}{
		"gid":                   d.GID,
		"type":                  d.Type,
		"description":           d.Description,
		"product_pricings":      meta.CostRows,
		"product_pricings_list": meta.ListRows,
		"list_price":            rowPrice(meta.ListRows, "monthly"),
		"cost_price_monthly":    rowPrice(meta.CostRows, "monthly"),
		"config_groups":         d.ConfigGroups,
		"customfields":          d.CustomFields,
		"advanced":              d.Advanced,
		"config_links":          d.ConfigLinks,
		"pay_type":              d.PayType,
		"upstream_api_type":     d.APIType,
		"hidden":                d.Hidden,
		"retired":               d.Retired,
		"stock_control":         d.StockControl,
		"qty":                   d.Qty,
		// 分类来源与前台在售判定（下游页面据此标记「派生分类」「未在售」）。
		"group_name_source": meta.GroupSource,
		"visibility":        meta.Visibility,
		"price_source":      meta.PriceSource,
	}
	if meta.GroupName != "" {
		raw["group_name"] = meta.GroupName
	}
	// 上游前台展示价（cart/index）：与周期价口径不同（含默认配置项价、按上游前台逻辑取周期），
	// 仅在活动款回填价格时出现，供运营核对。
	if meta.HasCatalog {
		raw["catalog"] = map[string]interface{}{
			"price":         float64(meta.Catalog.ProductPrice),
			"sale_price":    float64(meta.Catalog.SalePrice),
			"setup_fee":     float64(meta.Catalog.SetupFee),
			"billing_cycle": meta.Catalog.BillingCycle,
			"has_bates":     meta.Catalog.HasBates,
			"bates":         float64(meta.Catalog.Bates),
			"listing_name":  meta.Catalog.Name,
		}
	}
	if d.UpstreamPID > 0 {
		raw["upstream_pid"] = d.UpstreamPID
	}
	if d.UpstreamPriceType != "" {
		raw["upstream_price_type"] = d.UpstreamPriceType
		raw["upstream_price_value"] = d.UpstreamPriceValue
	}
	raw["upstream_discount"] = map[string]interface{}{
		"type": float64(meta.Flag.Type), "bates": float64(meta.Flag.Bates), "desc": meta.Flag.DiscountDesc(),
	}
	return raw
}

// rowPrice 读取首行的指定周期价格（无则 0）。
func rowPrice(rows []PricingRow, key string) float64 {
	if len(rows) == 0 {
		return 0
	}
	v, _ := rows[0].Num(key)
	return v
}

// detailBatchSize prodetail 单次请求的商品数。
//
// 实测（2026-10）：上游这个接口是**同步逐商品组装**的，响应体积与耗时都线性增长
// （45 个商品 ≈ 2MB / 8.8s，即约 200ms/个）。按上游 concurrent=500 一次要 200 个，
// 请求必然撞上 15s 的 HTTP 超时，而超时在 fetchAllDetails 里此前是静默跳过 ——
// 结果 245 个商品只同步进来 45 个（恰好是没超时的那一批），价格/库存/规格全是旧的。
// 取 50 并配合超时二分重试：单批约 10s，留出余量。
const detailBatchSize = 50

// fetchAllDetails 分批拉取全量商品详情，返回 pid → 详情。
//
// 失败处理（重要）：上游 prodetail 是逐商品同步组装，批量越大越容易超时。
// 单批失败时**二分拆小重试**而不是跳过 —— 跳过会让那批商品的价格/规格/库存
// 永远停留在上一次成功的值，同步任务却仍报 success（实测：245 个商品只有 45 个
// 真的同步进来，其余仍是旧值，页面看起来"同步成功但数据没变"）。
func (p *Provider) fetchAllDetails(ctx context.Context, infos []struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}) map[string]UpstreamProductDetail {
	out := map[string]UpstreamProductDetail{}
	p.fetchDetailsRange(ctx, infos, out)
	return out
}

// fetchDetailsRange 递归拉取 infos 区间：失败时对半拆分重试，直到单个商品。
func (p *Provider) fetchDetailsRange(ctx context.Context, infos []struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}, out map[string]UpstreamProductDetail) {
	if len(infos) == 0 {
		return
	}
	for start := 0; start < len(infos); start += detailBatchSize {
		end := start + detailBatchSize
		if end > len(infos) {
			end = len(infos)
		}
		chunk := infos[start:end]
		form := url.Values{}
		for _, it := range chunk {
			form.Add("pids[]", strconv.FormatInt(it.ID, 10))
		}
		var resp OpenapiProdetailResp
		if err := p.call(ctx, "ListProducts", http.MethodGet, "api/product/prodetail", form, &resp); err != nil {
			if len(chunk) == 1 {
				// 单个商品仍失败：放弃这一条（一个坏商品不该拖垮整次同步），
				// 但留下可排查的记录。
				log.Printf("[zjmf] 商品详情拉取失败 pid=%d: %v", chunk[0].ID, err)
				continue
			}
			mid := len(chunk) / 2
			p.fetchDetailsRange(ctx, chunk[:mid], out)
			p.fetchDetailsRange(ctx, chunk[mid:], out)
			continue
		}
		for pid, d := range resp.Detail {
			out[pid] = d
		}
	}
}

// upstreamPrice 取上游代表价：优先 monthly，其次 quarterly/semiannually/annually/onetime。
// 入参为 pricing 原始行（PricingRow）；旧调用方若传 ProductPricing，见 upstreamPriceLegacy。
func upstreamPrice(rows []PricingRow) float64 {
	if len(rows) == 0 {
		return 0
	}
	return rows[0].HeadlinePrice()
}

// parseCloudSpecs 从 prodetail 的 config_groups（嵌套：分组→配置项→子值）提取标准规格。
//
// 两轮匹配：先**精确**键名，再宽松包含。为什么需要（实测数据）：
// 上游同时下发 `bw`（带宽，15Mbps）与 `in_bw`（流入带宽，30Mbps）、
// `system_disk_size`（系统盘）与 `data_disk_size`（数据盘），而按"包含"匹配时
// `in_bw` 含 `bw`、`data_disk_size` 含 `disk` —— 后出现的配置项会把真正的售卖规格
// 覆盖掉。实测六区「4-4 15Mbps」商品带宽被写成 30（流入带宽），商品名与实际规格不符。
//
// 先精确、后宽松 + 字段一次性赋值，保证 `bw`/`system_disk_size` 永远优先于
// `in_bw`/`data_disk_size`；只有上游没给精确键的商品才回落到包含匹配。
func parseCloudSpecs(groups []ConfigGroup) model.StandardProductSpec {
	var spec model.StandardProductSpec
	for _, exact := range []bool{true, false} {
		for _, g := range groups {
			for _, opt := range g.Options {
				val := firstSubValue(opt)
				if val == "" {
					continue
				}
				applySpecField(&spec, normalizeOptionKey(opt.OptionName), val, exact)
			}
		}
	}
	return spec
}

// applySpecField 把单个配置项写入标准规格；exact=false 时按"包含"匹配。
// 已赋值的字段不再被后续配置项改写（同名字段只认第一个命中的键）。
func applySpecField(spec *model.StandardProductSpec, key, val string, exact bool) {
	into := func(dst *int, fn func(string) int) {
		if *dst == 0 {
			*dst = fn(val)
		}
	}
	strInto := func(dst *string, fn func(string) string) {
		if *dst == "" {
			*dst = fn(val)
		}
	}
	switch {
	case matchKeyMode(key, exact, "cpu", "核"):
		into(&spec.CPU, leadingInt)
	case matchKeyMode(key, exact, "memory", "mem", "内存"):
		into(&spec.Memory, memoryToMB)
	// 系统盘优先：`data_disk_size`（数据盘）也含 "disk"，但它不是售卖的系统盘规格。
	case matchKeyMode(key, exact, "system_disk_size", "systemdisk", "disk", "系统盘", "磁盘", "硬盘"):
		into(&spec.Disk, leadingInt)
	case matchKeyMode(key, exact, "os", "操作系统", "镜像", "系统"):
		strInto(&spec.OS, normalizeOS)
	case matchKeyMode(key, exact, "area", "数据中心", "区域"):
		strInto(&spec.Region, strings.TrimSpace)
	case matchKeyMode(key, exact, "node", "zone", "节点"):
		strInto(&spec.Zone, strings.TrimSpace)
	// 售卖带宽优先：`in_bw`（流入带宽）含 "bw"，不能覆盖对外售卖带宽。
	case matchKeyMode(key, exact, "bw", "bandwidth", "带宽"):
		into(&spec.Bandwidth, leadingInt)
	case matchKeyMode(key, exact, "disk_type", "disktype", "磁盘类型"):
		strInto(&spec.DiskType, strings.TrimSpace)
	}
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

// normalizeOptionKey 归一化配置键：取 `键|说明` 的键部分、转小写、去空格与说明分隔符。
//
// 必须截断 `|`：上游配置项名形如 `bw|带宽`、`system_disk_size|系统盘`，
// 把 `|说明` 一起带进匹配会让精确匹配全部失效（`bw|带宽` != `bw`），
// 于是退化成"包含"匹配 —— 正是 `in_bw` 覆盖 `bw`、`data_disk_size` 覆盖
// `system_disk_size` 的成因（实测带宽被写成流入带宽）。
func normalizeOptionKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if idx := strings.IndexAny(s, "|｜"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.ReplaceAll(s, "（", "(")
	s = strings.ReplaceAll(s, "）", ")")
	s = strings.ReplaceAll(s, " ", "")
	return strings.TrimSpace(s)
}

// matchKey 判断归一化后的配置键是否命中任一候选（自身相等或包含候选词）。
func matchKey(key string, cands ...string) bool {
	return matchKeyMode(key, false, cands...)
}

// matchKeyMode 键匹配；exact=true 时只认完全相等，false 时允许包含。
//
// 为什么需要 exact："包含"匹配会把语义不同的键混到一起 ——
// `in_bw` 含 `bw`、`data_disk_size` 含 `disk`。解析规格时先跑一轮 exact
// （精确键优先），再跑一轮宽松兜底，才能保证售卖规格不被附属配置覆盖。
func matchKeyMode(key string, exact bool, cands ...string) bool {
	for _, c := range cands {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if key == c {
			return true
		}
		if !exact && strings.Contains(key, c) {
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

// GetProduct 拉取单个上游商品配置（api/product/prodetail，product_id）。
// 与 ListProducts 同口径：分类名（cart/all）与代理折扣（cart/get_product_config 的 flag）一并取回，
// 保证「新建商品前预览上游商品」看到的价格就是采购价。
func (p *Provider) GetProduct(ctx context.Context, upstreamID string) (*model.StandardProduct, error) {
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
	groups, groupsOK := p.fetchCartGroups(ctx)
	flag := p.fetchProductFlag(ctx, upstreamID)
	cat, hasCat := p.fetchCatalogRow(ctx, d.GID, upstreamID)
	listRows, priceSource := resolveListRows(d.ProductPricings, cat, hasCat)
	costRows := scalePricingRows(listRows, flag)
	groupName, groupSource := resolveGroupName(d.GID, d.Name, groups, groupsOK)
	visibility := classifyVisibility(d, groups, groupsOK)
	return &model.StandardProduct{
		ProviderType: ProviderType,
		UpstreamID:   upstreamID,
		Name:         d.Name,
		Specs:        spec,
		CostPrice:    upstreamPrice(costRows),
		SalePrice:    upstreamPrice(listRows),
		RawSpecs: buildProductRawSpecs(d, productMeta{
			GroupName: groupName, GroupSource: groupSource,
			ListRows: listRows, CostRows: costRows, Flag: flag,
			PriceSource: priceSource, Visibility: visibility,
			Catalog: cat, HasCatalog: hasCat,
		}),
		Status: statusForVisibility(visibility),
	}, nil
}

// fetchProductFlag 探测单个商品的客户折扣（flag）；失败或无折扣返回零值。
// 折扣记录按 (客户分组 × 商品) 命中，必须逐商品探（见 ListProducts 注释）。
func (p *Provider) fetchProductFlag(ctx context.Context, pid string) SaleFlag {
	var resp CartProductConfigResp
	if err := p.call(ctx, "GetProduct", http.MethodGet, "cart/get_product_config",
		url.Values{"pid": {pid}}, &resp); err != nil {
		return SaleFlag{}
	}
	return resp.Flag
}

// fetchCatalogRow 取单商品的上游前台目录行（cart/index?gid=，从中挑出 pid）。
func (p *Provider) fetchCatalogRow(ctx context.Context, gid int64, pid string) (CartIndexProduct, bool) {
	var resp CartIndexResp
	if err := p.callTopLevel(ctx, "GetProduct", http.MethodGet, "cart/index",
		url.Values{"gid": {strconv.FormatInt(gid, 10)}}, &resp); err != nil {
		return CartIndexProduct{}, false
	}
	target, err := strconv.ParseInt(strings.TrimSpace(pid), 10, 64)
	if err != nil {
		return CartIndexProduct{}, false
	}
	for _, row := range resp.Products {
		if row.PID() == target {
			return row, true
		}
	}
	return CartIndexProduct{}, false
}

// downstreamForm 下游身份三件套（上游协议的幂等键，见 CreateInstanceRequest 注释）。
// 三项齐全（http(s) 前缀 + 32 位 token + 数字 id）时上游才会启用按 token 反查既有单，
// 缺一项上游就按"普通客户"处理 —— 重试时无从找回既有账单，只能又下一单。
func downstreamForm(req *model.CreateInstanceRequest) url.Values {
	form := url.Values{}
	if req == nil {
		return form
	}
	form.Set("downstream_url", req.DownstreamURL)
	form.Set("downstream_token", req.DownstreamToken)
	if req.DownstreamID > 0 {
		form.Set("downstream_id", strconv.FormatUint(req.DownstreamID, 10))
	} else {
		form.Set("downstream_id", "")
	}
	return form
}

// downstreamReady 三项齐全且格式满足上游校验（DownstreamURL 以 http(s):// 开头、
// token 长度 32、id 为正数）时才参与握手；否则退回旧的"普通客户"行为。
func downstreamReady(req *model.CreateInstanceRequest) bool {
	if req == nil || req.DownstreamID == 0 || len(req.DownstreamToken) != 32 {
		return false
	}
	u := req.DownstreamURL
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// payUpstreamInvoice 用上游余额支付一张已生成的账单，返回开通出的主机 id。
//
// 只认 status 1001：上游用 200 表示"余额不足，账单未清、主机不会开通"
// （PayController::applyCredit 的 else 分支）。若把 200 当成功，会把"没付成钱"
// 记成开通成功，随后取不到 hostid 才报错，排查方向就全错了。
//
// 例外（实测故障）：账单**已经支付过**时上游回 406「账单已支付」。
// 这不是失败 —— 典型场景是"余额不足 → 用户/运营在上游侧充值并把账单付掉"，
// 此时机器正在上游开出来，本地重试应当**接管**它而不是报错。
// 这种情况没有 hostid 回执，交由调用方按下游令牌握手反查出主机 id。
func (p *Provider) payUpstreamInvoice(ctx context.Context, req *model.CreateInstanceRequest, invoiceID int64) (int64, error) {
	if invoiceID <= 0 {
		return 0, &upstream.ProviderError{Op: "CreateInstance", Msg: "缺少上游账单ID，无法支付开通"}
	}
	payForm := downstreamForm(req)
	payForm.Set("invoiceid", strconv.FormatInt(invoiceID, 10))
	payForm.Set("use_credit", "1")
	// enough=1：余额不够就整单失败，不允许上游扣部分余额（enough=0 会扣了钱但账单仍未结清）。
	payForm.Set("enough", "1")
	shell, httpStatus, body, err := p.callShell(ctx, "CreateInstance", http.MethodPost, "apply_credit", payForm)
	if err != nil {
		return 0, err
	}
	if httpStatus >= 400 {
		return 0, &upstream.ProviderError{Op: "CreateInstance", StatusCode: httpStatus, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
	}
	if shell.Status != StatusPaidSuccess && shell.Code != StatusPaidSuccess {
		if isInvoiceAlreadyPaid(shell) {
			// 账单已付：主机在上游已存在（或正在创建）。按下游令牌握手取回主机 id，
			// 让调用方回源接管。"已支付"必须按成功处理，否则本地会一直卡在创建中。
			if hostID := p.hostIDByDownstream(ctx, req); hostID > 0 {
				return hostID, nil
			}
			return 0, &upstream.ProviderError{Op: "CreateInstance",
				Msg: "上游账单已支付但未能反查到对应主机，请人工核对该账单与主机"}
		}
		return 0, &upstream.ProviderError{Op: "CreateInstance", Code: shell.Status.Int(), Msg: firstNonEmpty(shell.Msg, "上游余额支付未完成（余额不足或账单未结清）")}
	}
	var payData struct {
		// hostid 实测下发 ["2058"]（字符串数组）；apply_credit 有时给数字数组。
		// 用 FlexIDList 容忍两种形态，避免整段响应解析失败。
		HostID FlexIDList `json:"hostid"`
	}
	if err := p.unmarshalData("CreateInstance", shell, &payData); err != nil {
		return 0, err
	}
	hostID := payData.HostID.First()
	if hostID == 0 {
		return 0, &upstream.ProviderError{Op: "CreateInstance", Msg: "上游支付成功但未返回主机ID"}
	}
	return hostID, nil
}

// isInvoiceAlreadyPaid 判断上游返回是否为「账单已支付」（apply_credit 的 406 分支）。
//
// 上游原文：{"status":"406","msg":"账单已支付","data":{"PayStatus":"Paid"}}。
// 注意 406 是**共用码**：同一码下还有「账单未找到」（{"status":"406","msg":"账单未找到"}），
// 两者语义完全相反，必须按 msg / data.PayStatus 区分，绝不能只看状态码。
func isInvoiceAlreadyPaid(shell Resp[json.RawMessage]) bool {
	if shell.Status.Int() != 406 && shell.Code.Int() != 406 {
		return false
	}
	if strings.Contains(shell.Msg, "已支付") || strings.Contains(shell.Msg, "已付款") {
		return true
	}
	var data struct {
		PayStatus string `json:"PayStatus"`
	}
	if len(shell.Data) > 0 {
		if err := json.Unmarshal(shell.Data, &data); err == nil && strings.EqualFold(data.PayStatus, "Paid") {
			return true
		}
	}
	return false
}

// isInvoiceMissingError 判断错误是否为「上游账单已不存在」。
//
// 语义：该账单被上游清理/归档（如未支付账单超期回收），本地记录的账单号已不可用。
// 处理方式不是报错终态，而是继续握手 → 无主机则重新下单（旧的尝试在上游已消失）。
func isInvoiceMissingError(err error) bool {
	var perr *upstream.ProviderError
	if !errors.As(err, &perr) {
		return false
	}
	for _, kw := range []string{"账单未找到", "账单不存在", "未找到该账单"} {
		if strings.Contains(perr.Msg, kw) {
			return true
		}
	}
	return false
}

// hostIDByDownstream 用下游三件套向上游反查本订单已存在的主机 id（不存在返回 0）。
//
// 用途：账单已支付时 apply_credit 不回 hostid，只能靠握手（cart/clear）反查。
// 查不到不报错 —— 调用方会把它当作"确实没有主机"处理。
func (p *Provider) hostIDByDownstream(ctx context.Context, req *model.CreateInstanceRequest) int64 {
	if !downstreamReady(req) {
		return 0
	}
	_, _, body, err := p.callShell(ctx, "CreateInstance", http.MethodPost, "cart/clear", downstreamForm(req))
	if err != nil {
		return 0
	}
	var resp clearResp
	_ = json.Unmarshal(body, &resp)
	return resp.HostID.Int64()
}

// settleExistingInvoice 复用已生成的上游账单完成开通（重试路径）：
// 只做"支付账单 → 拉取主机详情"，不再走"加入购物车 + 结算"。
// 自定义字段与配置项首次结算时已随 cart_data 写入，无需补录。
func (p *Provider) settleExistingInvoice(ctx context.Context, req *model.CreateInstanceRequest, invoiceID int64, _ string) (*model.StandardInstance, error) {
	hostID, err := p.payUpstreamInvoice(ctx, req, invoiceID)
	if err != nil {
		return nil, err
	}
	return p.fetchProvisionedInstance(ctx, hostID, req)
}

// clearResp cart/clear 的响应体（平铺无 data 壳）：上游在这里回答"本下游订单在上游
// 是否已有账单/主机"（CartController::clearCart 的 is_api 分支）。
//
// 实测三种形态：
//
//	{"hostid":2057,"invoiceid":808043,"msg":"请求成功","status":200}   账单已生成未支付
//	{"hostid":2056,"domainstatus":"Active","msg":"该订单已开通...","status":400}  已开通
//	{"msg":"请求成功","status":200}                                    无记录
//
// status 也有数字与字符串（"400"）两种下发，故用 FlexCode。
type clearResp struct {
	Status FlexCode `json:"status"`
	Code   FlexCode `json:"code"`
	Msg    string   `json:"msg"`
	// 实测 clear 返回 {"hostid":2056,...}（数字）；个别版本可能是 "2056" 或 [2056]。
	HostID       FlexID `json:"hostid"`       // 已存在的主机（400 已开通；200 时为占号主机）
	DomainStatus string `json:"domainstatus"` // 该主机状态（Active 才算真的开好了）
	InvoiceID    FlexID `json:"invoiceid"`    // 已生成的待支付账单：直接支付这张即可
}

// reconcileDownstream 上游幂等握手（下单前调用，等价源码 createFinal 的 /cart/clear 一步）：
//   - 上游已有本订单的待支付账单（status 200 + invoiceid）→ 返回账单 id，
//     调用方直接支付这张即可（"重新开通"，不新建）；
//   - 上游已有本订单的主机（400 "该订单已开通"）→ 回源返回该实例，绝不重复开通；
//   - 上游没有记录 → (nil, 0, nil)，调用方正常下单；
//   - 其它业务码 → 返回错误（终结性，转人工）。
//
// 这一步是"重试不再重复下单"的关键：没有它，每次重试都会在上游重新结算，
// 上游侧表现为同一订单反复生成账单与主机（实测事故）。
//
// 分支顺序很关键：上游对"账单已生成但未支付"的回答是
// `{"hostid":2057,"invoiceid":808043,"status":200}` —— **hostid 与 invoiceid 同时出现**。
// 此时机器虽然占了号（Pending、dcimid=0）但并未付款开通，正确动作是"付这张账单"，
// 所以必须先判 invoiceid 分支，不能因为看到 hostid 就当成"已开通"接管。
//
// 为什么"已有主机但非 Active"不能直接报错（实测故障）：用户在上游余额不足后
// **自己在上游后台充值并开通**，那时主机是 Active；但若主机还在创建中（Pending），
// 旧逻辑会报"不可重复开通，请人工核对该主机"——本地看起来失败，机器其实正在上游
// 开出来。现在改为：只要上游确认已有主机就回源接管，状态由上游说了算
// （Pending → creating，交给滞留推进器继续跟进）。
func (p *Provider) reconcileDownstream(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, int64, error) {
	if !downstreamReady(req) {
		return nil, 0, nil
	}
	shell, httpStatus, body, err := p.callShell(ctx, "CreateInstance", http.MethodPost, "cart/clear", downstreamForm(req))
	if err != nil {
		// 网络/连接类错误：无法确认上游是否已有本订单的单据，冒然下单会重复 —— 交给调用方重试。
		return nil, 0, err
	}
	if httpStatus >= 400 {
		return nil, 0, &upstream.ProviderError{Op: "CreateInstance", StatusCode: httpStatus, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
	}
	var resp clearResp
	_ = json.Unmarshal(body, &resp)
	status := resp.Status.Int()
	if status == 0 {
		status = resp.Code.Int()
	}
	switch {
	case resp.InvoiceID > 0:
		// 已生成但未支付的账单：直接复用（这就是"重新开通"）。
		// 该形态可能同时带 hostid（占号的主机，未付款不会开通），以账单为准。
		return nil, resp.InvoiceID.Int64(), nil
	case resp.HostID > 0:
		// 上游明确回答「该订单已有主机」：回源接管，绝不重新下单。
		//
		// 终态（已删除/取消/风控冻结）除外：那种主机接管不了、重下单又可能与上游
		// 账务冲突，保持"交人工核对"的旧语义。
		if hostStatusTerminal(resp.DomainStatus) {
			return nil, 0, &upstream.ProviderError{Op: "CreateInstance", Code: status,
				Msg: fmt.Sprintf("上游已存在本订单的主机（#%d，状态 %s），不可重复开通，请人工核对该主机后处理", resp.HostID.Int64(), firstNonEmpty(resp.DomainStatus, "未知"))}
		}
		inst, derr := p.fetchProvisionedInstance(ctx, resp.HostID.Int64(), req)
		if derr != nil {
			return nil, 0, derr
		}
		return inst, 0, nil
	case status == StatusOK || status == 0:
		// 上游没有本订单的任何记录：正常走新单流程。
		return nil, 0, nil
	default:
		// 其它业务码：状态不明，不下单，交人工。
		return nil, 0, &upstream.ProviderError{Op: "CreateInstance", Code: status, Msg: firstNonEmpty(resp.Msg, "上游幂等握手失败")}
	}
}

// hostStatusTerminal 上游主机状态是否为终态（已删除/取消/终止/风控冻结）。
func hostStatusTerminal(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "deleted", "cancelled", "canceled", "terminated", "fraud":
		return true
	}
	return false
}

// CreateInstance 「财务对接财务」开通：由本系统（下游）向上游魔方财务下单。
// 等价于源码 Host::createAccount 的 zjmf_api 分支：
//  1. cart/add_to_shop 将商品加入上游购物车；
//  2. cart/settle      结算生成账单（invoiceid，主机为待开通 Pending）；
//  3. apply_credit     用上游 API 账户余额支付账单（status 1001=支付成功并开通），返回 hostid；
//  4. host/header      按 hostid 拉取主机信息映射标准实例。
//
// 账单金额自上而下扣减上游 API 账户余额，需确保该账户有足够余额。
func (p *Provider) CreateInstance(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
	if p.isResource() {
		return nil, p.notSupported("CreateInstance", "资源型上游请走资源池开通流程处理")
	}
	pid := firstNonEmpty(strFromMap(req.Extra, "upstream_pid"), strFromMap(req.Extra, "pid"))
	if pid == "" {
		return nil, &upstream.ProviderError{Op: "CreateInstance", Msg: "商品未绑定上游商品ID（upstream_pid），无法在财务上游下单"}
	}
	// 主机名/密码：客户在商品详情页可自定义主机名（extra.hostname，走客户选配项下发），
	// 未指定时用 defaultHostname（商品名 + 随机后缀）：上游对重名主机会把新机器卡在
	// Pending/dcimid=0 永不激活，唯一性是开通成功的必要条件。
	// 密码同理（extra.root_password 仅内部调用方使用，默认随机强密码）。
	host := defaultHostname(firstNonEmpty(strFromMap(req.Extra, "hostname"), req.Name), req.DownstreamToken)
	password := firstNonEmpty(req.Password, strFromMap(req.Extra, "root_password"), randPassword())
	billingCycle := normalizeBillingCycle(req.BillingMode)
	configOpt := buildCartConfigOption(req.Extra)
	// 币种：上游结算要求 currencyid 为该账号（clients.currency）生效的币种 id。
	// 源码 Host::createFinal 用 `/user_info` 的 user.currency；固定写 1 只有在账号
	// 恰好用默认币种时才正确，账号换过币种就会按错误价格结算甚至直接失败。
	currencyID := p.accountCurrencyID(ctx)

	// 幂等分支①：调用方已记录过本次开通的上游账单 → 直接复用，跳过"下单 + 结算"。
	//
	// 为什么必须这样：上游结算是**生成新账单**的动作，重试一次就多一张账单。
	// 上游余额不足这类失败属于"账单已建、钱没付"，重试要做的只是再支付同一张账单
	// （充值后即可结清），而不是重新下单 —— 否则一个订单会在上游留下多张待支付账单。
	//
	// 例外（实测故障）：该账单可能已被上游清理（未支付账单超期回收 → "账单未找到"），
	// 或已由人工在上游侧付掉（"账单已支付"，hostIDByDownstream 反查主机）。
	// 前者继续走握手分流（无主机则重新下单），不能死在这里。
	if req.UpstreamInvoiceID > 0 {
		inst, err := p.settleExistingInvoice(ctx, req, req.UpstreamInvoiceID, host)
		if err == nil {
			return inst, nil
		}
		if !isInvoiceMissingError(err) {
			return nil, err
		}
		// 账单已不存在：清掉调用方记录，按"全新订单"重新走握手 + 下单。
		if req.OnInvoiceCreated != nil {
			req.OnInvoiceCreated(0)
		}
	}

	// 0) 上游幂等握手（源码 Host::createFinal 的 /cart/clear 一步，带 downstream 三件套）。
	//
	// 这一步不是"清购物车"那么简单：上游按 downstream_token 反查本下游订单是否已有
	// 账单/主机，是上游协议**自带的防重复下单机制**。没有它（或 token 每次不同），
	// 每次重试都会重新结算，上游侧留下多张账单与多台 Pending 主机。
	//   · 已有主机（Active / Pending 皆可） → 回源接管，不再下单；
	//   · 有待支付账单   → 支付同一张账单（"重新开通"）；
	//   · 无记录         → 继续下面的新单流程。
	if resumed, invoiceID, err := p.reconcileDownstream(ctx, req); err != nil {
		return nil, err
	} else if resumed != nil {
		return resumed, nil
	} else if invoiceID > 0 {
		if req.OnInvoiceCreated != nil {
			req.OnInvoiceCreated(invoiceID)
		}
		return p.settleExistingInvoice(ctx, req, invoiceID, host)
	}

	// 0.1) 清空上游购物车（源码 Host::createFinal 下单前的固定动作）。
	// 结算走 cart_data 显式覆盖购物车，因此这一步不是正确性必需；但上游购物车是
	// 按账号会话累积的，重试/中断后残留的商品会在上游后台"购物车"里碍事。
	// 失败只忽略：真正的下单结果以 cart/settle 为准。
	// 带下游三件套时上面的握手已经清过购物车，这里不再重复调用。
	if !downstreamReady(req) {
		_ = p.call(ctx, "CreateInstance", http.MethodPost, "cart/clear", downstreamForm(req), nil)
	}

	// 1) 加入购物车
	addForm := url.Values{}
	addForm.Set("pid", pid)
	addForm.Set("billingcycle", billingCycle)
	addForm.Set("host", host)
	addForm.Set("password", password)
	addForm.Set("currencyid", currencyID)
	addForm.Set("qty", "1")
	for k, v := range configOpt {
		addForm.Set("configoption["+k+"]", v)
	}
	// 自定义字段：上游商品若把必填项做成 customfield，不传会被结算拒绝
	// （源码 createFinal 从 customfieldsvalues 取 upstream_id → value 原样透传）。
	for k, v := range customFieldsFromExtra(req.Extra) {
		addForm.Set("customfield["+k+"]", v)
	}
	if err := p.call(ctx, "CreateInstance", http.MethodPost, "cart/add_to_shop", addForm, nil); err != nil {
		return nil, err
	}

	// 2) 结算生成账单
	// 下游三件套同时挂在顶层与 cart_data 下（上游 input("post.") 读顶层；
	// 上游自己的下游实现把 cart_data 整体复制后也带上了这三项，此处与之对齐）。
	settleForm := downstreamForm(req)
	if downstreamReady(req) {
		for k, v := range downstreamForm(req) {
			settleForm.Set("cart_data["+k+"]", firstNonEmpty(v...))
		}
	}
	settleForm.Set("cart_data[pid]", pid)
	settleForm.Set("cart_data[billingcycle]", billingCycle)
	settleForm.Set("cart_data[host]", host)
	settleForm.Set("cart_data[password]", password)
	settleForm.Set("cart_data[currencyid]", currencyID)
	settleForm.Set("cart_data[qty]", "1")
	for k, v := range configOpt {
		settleForm.Set("cart_data[configoptions]["+k+"]", v)
	}
	for k, v := range customFieldsFromExtra(req.Extra) {
		settleForm.Set("cart_data[customfield]["+k+"]", v)
	}
	var settleData struct {
		// 实测 cart/settle 返回 {"hostid":["2056"],"invoiceid":808041}：
		// hostid 是**字符串数组**、invoiceid 是数字；apply_credit 的 hostid 又是数字数组。
		// 用宽容类型统一吸收，否则一个字段类型不符就丢掉整张账单 id（开通直接失败）。
		InvoiceID FlexID     `json:"invoiceid"`
		HostID    FlexIDList `json:"hostid"`
	}
	// cart/settle 有两条成功返回形态（源码 CartController::settle）：
	//   total>0 → status 200 + data.invoiceid（待支付）；
	//   total=0（免费/一次性 0 元）→ status 1001 + data.hostid（已开通）。
	// 因此这里不能只认 200（p.call 的行为），否则 0 元商品会被判成"接口调用失败"。
	settleShell, settleHTTP, settleBody, err := p.callShell(ctx, "CreateInstance", http.MethodPost, "cart/settle", settleForm)
	if err != nil {
		return nil, err
	}
	if settleHTTP >= 400 {
		return nil, &upstream.ProviderError{Op: "CreateInstance", StatusCode: settleHTTP, Msg: firstNonEmpty(settleShell.Msg, strings.TrimSpace(string(settleBody)))}
	}
	if settleShell.Status != StatusOK && settleShell.Code != StatusOK && settleShell.Status != StatusPaidSuccess && settleShell.Code != StatusPaidSuccess {
		return nil, &upstream.ProviderError{Op: "CreateInstance", Code: settleShell.Status.Int(), Msg: firstNonEmpty(settleShell.Msg, "上游结算失败")}
	}
	if err := p.unmarshalData("CreateInstance", settleShell, &settleData); err != nil {
		return nil, err
	}
	// 0 元单（免费/一次性 0 元）结算即开通：只有 hostid 没有 invoiceid，属正常。
	if settleData.InvoiceID == 0 && settleData.HostID.First() == 0 {
		return nil, &upstream.ProviderError{Op: "CreateInstance", Msg: "上游结算成功但未返回账单ID或主机ID"}
	}

	// 账单已生成：**先落库再支付** —— 支付失败（余额不足等）是最常见的重试场景，
	// 只有把账单 id 记下来，重试才能复用这张账单而不是再下一单。
	if settleData.InvoiceID > 0 && req.OnInvoiceCreated != nil {
		req.OnInvoiceCreated(settleData.InvoiceID.Int64())
	}

	// 3) 用上游余额支付并开通（status 1001 支付完成）。
	// 有账单就必须付清：结算返回的 hostid 只代表单据已建（主机此时是 Pending），
	// 没付钱就不会真正开通；只有 invoiceid==0（0 元单）才是"结算即开通"。
	hostID := settleData.HostID.First()
	if settleData.InvoiceID > 0 {
		paidHostID, err := p.payUpstreamInvoice(ctx, req, settleData.InvoiceID.Int64())
		if err != nil {
			return nil, err
		}
		hostID = paidHostID
	}

	// 4) 拉取主机信息并映射标准实例
	return p.fetchProvisionedInstance(ctx, hostID, req)
}

// fetchProvisionedInstance 按主机 id 拉取上游详情并映射标准实例。
// 开通成功但详情拉取失败时返回最小可用实例，避免把已开通的资源当成失败回滚。
func (p *Provider) fetchProvisionedInstance(ctx context.Context, hostID int64, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
	// 与 CreateInstance 同一算式：这里只在上游详情拉取失败时用作实例名兜底，
	// 后缀不稳定会让"同一个订单两次尝试的实例名"不一致。
	host := defaultHostname(firstNonEmpty(strFromMap(req.Extra, "hostname"), req.Name), req.DownstreamToken)
	inst, err := p.waitInstanceReady(ctx, strconv.FormatInt(hostID, 10), 6)
	if err != nil {
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
func (p *Provider) waitInstanceReady(ctx context.Context, instanceID string, tries int) (*model.StandardInstance, error) {
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
func (p *Provider) GetInstance(ctx context.Context, instanceID string) (*model.StandardInstance, error) {
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
		// Specs 取上游主机的权威值：操作系统尤其关键 —— 客户下单时选的镜像
		// （如 Debian）只有上游知道最终落成什么，本地订单快照里存的是商品基线
		// （常常是商品默认的 CentOS）。此前 Specs 完全不填，实例落库时
		// 规格回落取订单快照，实例详情页就显示成 CentOS（实测故障：
		// 选 Debian 开通、上游 host_data.os=Debian-12.0_x64，本地却显示 CentOS）。
		Specs: model.StandardProductSpec{
			OS: h.OS,
		},
		RawData: map[string]interface{}{
			"assignedips": h.AssignedIPs,
			// 端口归一为字符串（详情页连接信息按字符串读取；上游为 int 列，
			// 未开放端口时回 0，这里统一转 "0"→空串，避免界面显示端口 0）。
			"port":          portString(h.Port),
			"username":      h.Username,
			"os":            h.OS,
			"productid":     h.ProductID,
			"productname":   h.ProductName,
			"nextduedate":   h.NextDueDate,
			"upstream_cost": float64(h.UpstreamCost),
			"domainstatus":  h.DomainStatus,
			"dcimid":        h.DcimID,
			"invoiceid":     h.InvoiceID,
		},
	}
	switch strings.ToLower(h.DomainStatus) {
	case "active":
		inst.Status = model.InstanceStatusRunning
	case "pending":
		inst.Status = model.InstanceStatusCreating
	case "suspended":
		inst.Status = model.InstanceStatusStopped
	case "deleted", "cancelled", "terminated":
		// 终态：上游已删除/取消。此前与 suspended 一起映射为 stopped（"已关机"），
		// 已删除的机器会被显示成"关机"，用户以为还能开机（实测：订单支付后上游
		// 侧主机已是 Deleted，本地回源却写成 stopped）。
		inst.Status = model.InstanceStatusDeleted
	case "fraud":
		// 风控冻结：上游不再服务，本地按 stopped 展示并提示人工处理最合适
		// （没有独立的 fraud 状态枚举，映射成 deleted 会误导为"已销毁可重建"）。
		inst.Status = model.InstanceStatusStopped
	default:
		inst.Status = model.InstanceStatusRunning
	}
	return inst, nil
}

// portString 端口归一为字符串：0/负数视为"未开放"返回空串，避免详情页显示端口 0。
func portString(p FlexNumber) string {
	if v := p.Int(); v > 0 && v <= 65535 {
		return strconv.FormatInt(v, 10)
	}
	return ""
}

// provisionCmd 财务型上游电源/控制台统一入口：POST /provision/default（源码 Host::on/off/reboot/vnc 的 zjmf_api 分支）。
func (p *Provider) provisionCmd(ctx context.Context, op, instanceID, funcName string) error {
	return p.call(ctx, op, http.MethodPost, "provision/default", url.Values{
		"id": {instanceID}, "func": {funcName}, "is_api": {"1"},
	}, nil)
}

// StartInstance 开机：POST /provision/default（func=on，id 为财务主机ID）。
func (p *Provider) StartInstance(ctx context.Context, instanceID string) error {
	return p.provisionCmd(ctx, "StartInstance", instanceID, "on")
}

// StopInstance 关机：POST /provision/default（func=off）；force 时 hard_off。
func (p *Provider) StopInstance(ctx context.Context, instanceID string, force bool) error {
	fn := "off"
	if force {
		fn = "hard_off"
	}
	return p.provisionCmd(ctx, "StopInstance", instanceID, fn)
}

// RestartInstance 重启：POST /provision/default（func=reboot）。
func (p *Provider) RestartInstance(ctx context.Context, instanceID string) error {
	return p.provisionCmd(ctx, "RestartInstance", instanceID, "reboot")
}

// HardStopInstance 硬关机：POST /provision/default（func=hard_off）。
// 与 StopInstance(force=true) 等价，单独暴露是为了让「强制关机」这个入口无论走
// InstanceControl 还是 InstancePowerHard 都落到同一条上游指令。
func (p *Provider) HardStopInstance(ctx context.Context, instanceID string) error {
	return p.provisionCmd(ctx, "HardStopInstance", instanceID, "hard_off")
}

// HardRestartInstance 硬重启：POST /provision/default（func=hard_reboot）。
//
// 上游本就有独立硬重启指令（源码 Host::hardReboot 的 zjmf_api 分支），此前适配器未实现
// InstancePowerHard，服务层只好退化为"硬关 + 开机"两步，中间会经历一次完整断电，
// 与面板行为不一致且多一次失败面。这里直接下发 hard_reboot。
func (p *Provider) HardRestartInstance(ctx context.Context, instanceID string) error {
	return p.provisionCmd(ctx, "HardRestartInstance", instanceID, "hard_reboot")
}

// ReinstallInstance 重装系统：POST /provision/default（func=reinstall）。
//
// 参数取自源码 Host::reinstall 的 zjmf_api 分支：os 是**上游配置子项 id**（而非系统名），
// 另可带 port / format_data_disk。ReinstallRequest.OS 在本适配器里即该子项 id
// （调用方从商品配置项 sub.upstream_key 取得）。
//
// 上游重装是异步长任务，本方法返回 nil 只代表"已受理"，不返回新凭据：
// 凭据由上游在重装完成后写入主机记录，本系统不落盘（项目内 instances 表无凭据列）。
func (p *Provider) ReinstallInstance(ctx context.Context, req *upstream.ReinstallRequest) (*upstream.ReinstallResult, error) {
	if req == nil || strings.TrimSpace(req.ProviderInstanceID) == "" {
		return nil, &upstream.ProviderError{Op: "ReinstallInstance", Msg: "缺少上游主机ID"}
	}
	os := strings.TrimSpace(req.OS)
	if os == "" {
		return nil, &upstream.ProviderError{Op: "ReinstallInstance", Msg: "缺少目标镜像（上游配置子项 id）"}
	}
	form := url.Values{}
	form.Set("id", req.ProviderInstanceID)
	form.Set("func", "reinstall")
	form.Set("os", os)
	form.Set("is_api", "1")
	if req.Port > 0 && req.Port <= 65535 {
		form.Set("port", strconv.Itoa(req.Port))
	}
	if req.FormatDataDisk {
		form.Set("format_data_disk", "1")
	} else {
		form.Set("format_data_disk", "0")
	}
	if err := p.call(ctx, "ReinstallInstance", http.MethodPost, "provision/default", form, nil); err != nil {
		return nil, err
	}
	return &upstream.ReinstallResult{}, nil
}

// ResetInstancePassword 重置登录密码：POST /provision/default（func=crack_pass）。
// 源码 Host::crackPass 的 zjmf_api 分支：{id, func=crack_pass, password, is_api=1}。
func (p *Provider) ResetInstancePassword(ctx context.Context, instanceID, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return &upstream.ProviderError{Op: "ResetInstancePassword", Msg: "新密码不能为空"}
	}
	return p.call(ctx, "ResetInstancePassword", http.MethodPost, "provision/default", url.Values{
		"id": {instanceID}, "func": {"crack_pass"}, "password": {newPassword}, "is_api": {"1"},
	}, nil)
}

// VNC 获取远程控制台：POST /provision/default（func=vnc），返回上游 noVNC 地址。
func (p *Provider) VNC(ctx context.Context, instanceID string) (upstream.VNCResult, error) {
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
func (p *Provider) SuspendInstance(ctx context.Context, instanceID, reason string) error {
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
func (p *Provider) UnsuspendInstance(ctx context.Context, instanceID string) error {
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
func (p *Provider) TerminateInstance(ctx context.Context, instanceID, reason string) error {
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
func (p *Provider) RenewInstance(ctx context.Context, req *upstream.RenewRequest) (*upstream.RenewResult, error) {
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
		InvoiceID FlexID `json:"invoiceid"`
	}
	if err := p.call(ctx, "RenewInstance", http.MethodPost, "host/renew", renewForm, &renewData); err != nil {
		return nil, err
	}
	if renewData.InvoiceID == 0 {
		return nil, &upstream.ProviderError{Op: "RenewInstance", Msg: "上游续费成功但未返回账单ID"}
	}

	// 2) 用上游余额支付（status 1001 = 支付完成）
	payForm := url.Values{}
	payForm.Set("invoiceid", renewData.InvoiceID.String())
	payForm.Set("use_credit", "1")
	shell, httpStatus, body, err := p.callShell(ctx, "RenewInstance", http.MethodPost, "apply_credit", payForm)
	if err != nil {
		return nil, err
	}
	if httpStatus >= 400 {
		return nil, &upstream.ProviderError{Op: "RenewInstance", StatusCode: httpStatus, Msg: firstNonEmpty(shell.Msg, strings.TrimSpace(string(body)))}
	}
	// 只认 1001：200 表示"余额不足，账单未清"（源码 Host::renew 也把 200 当失败处理），
	// 接受它会把"没续上"记成"续费成功"。
	if shell.Status != StatusPaidSuccess && shell.Code != StatusPaidSuccess {
		return nil, &upstream.ProviderError{Op: "RenewInstance", Code: shell.Status.Int(), Msg: firstNonEmpty(shell.Msg, "上游续费余额支付未完成（余额不足或账单未结清）")}
	}

	res := &upstream.RenewResult{
		UpstreamOrderRef: renewData.InvoiceID.String(),
		Raw:              map[string]interface{}{"invoiceid": renewData.InvoiceID.Int64(), "billingcycles": cycle},
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

// DeleteInstance / ResizeInstance / ListInstances / ListPools 在「财务对接财务」模式下
// 不由本适配器直接提供（删除走 /host/cancel、升降配走上游订单/upgrade/*，魔方财务无
// 资源池概念），因此 Provider 不实现 InstanceAdministration / PoolReader。
// 消费方通过类型断言按需取用，失败即返回「上游不支持」。
//
// 已实现的能力接口（编译期断言，改动时能立即发现契约漂移）：
// Provider / ProductCatalog / InstanceProvisioning / InstanceControl / InstanceRenewal /
// InstanceSuspension / InstanceTermination / InstancePowerHard / InstanceReinstall /
// InstancePasswordReset / AccountReader / FinanceLedgerReader。
var (
	_ upstream.Provider              = (*Provider)(nil)
	_ upstream.ProductCatalog        = (*Provider)(nil)
	_ upstream.InstanceProvisioning  = (*Provider)(nil)
	_ upstream.InstanceControl       = (*Provider)(nil)
	_ upstream.InstanceRenewal       = (*Provider)(nil)
	_ upstream.InstanceSuspension    = (*Provider)(nil)
	_ upstream.InstanceTermination   = (*Provider)(nil)
	_ upstream.InstancePowerHard     = (*Provider)(nil)
	_ upstream.InstanceReinstall     = (*Provider)(nil)
	_ upstream.InstancePasswordReset = (*Provider)(nil)
	_ upstream.AccountReader         = (*Provider)(nil)
	_ upstream.FinanceLedgerReader   = (*Provider)(nil)
)

// notSupported 返回能力不支持错误（用于运行期分支：如资源型上游下的 GetProduct/CreateInstance）。
func (p *Provider) notSupported(op, msg string) error {
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

// normalizeBillingCycle 归一化计费周期为上游 billing_cycle 约定的枚举值。
//
// 上游 config("billing_cycle") 的键含 free/onetime/hour/day/ontrial/monthly/
// quarterly/semiannually/annually/biennially/triennially 等；平台规范值见 pkg/billingcycle。
// 未识别的值回落 monthly（与上游 renew 自身的兜底口径一致）。
func normalizeBillingCycle(cycle string) string {
	switch strings.ToLower(strings.TrimSpace(cycle)) {
	case "hour", "hourly":
		return "hour"
	case "day", "daily":
		// 漏掉这一分支会把日付实例的续费周期发给上游成 monthly：本地按天推进到期时间，
		// 上游却按 monthly 生成账单，两边账期与金额同时错位。
		return "day"
	case "quarterly":
		return "quarterly"
	case "semiannually", "semiannual":
		return "semiannually"
	case "annually", "yearly", "annual":
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

// accountCurrencyID 读取上游账号生效币种 id（GET cart/credit → data.currency.id），
// 供 cart 下单使用。读取失败回落 "1"（上游默认币种）：下单流程不能因为一个
// 只影响取价的字段而完全中断，且多数账号就是默认币种。
func (p *Provider) accountCurrencyID(ctx context.Context) string {
	var data struct {
		Currency struct {
			ID int64 `json:"id"`
		} `json:"currency"`
	}
	if err := p.call(ctx, "accountCurrencyID", http.MethodGet, "cart/credit", nil, &data); err != nil {
		return "1"
	}
	if data.Currency.ID <= 0 {
		return "1"
	}
	return strconv.FormatInt(data.Currency.ID, 10)
}

// customFieldsFromExtra 读取 extra.customfield（上游自定义字段：字段 upstream_id → 值）。
// 支持 map[string]string、map[string]interface{} 与 JSON 文本三种形态。
func customFieldsFromExtra(extra map[string]interface{}) map[string]string {
	out := map[string]string{}
	if extra == nil {
		return out
	}
	switch raw := extra["customfield"].(type) {
	case map[string]string:
		for k, v := range raw {
			out[k] = v
		}
	case map[string]interface{}:
		for k, v := range raw {
			out[k] = fmt.Sprintf("%v", v)
		}
	case string:
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for k, v := range m {
				out[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	return out
}

// buildCartConfigOption 从开通请求的 configoption（显式覆盖）或 config_groups（上游配置组）
// 构建 cart 下单所需的 configoption[option.id] = sub.id 映射。
//
// 键必须是**上游自己的** product_config_options.id / _sub.id：上游
// common/logic/ConfigOptions.php::filterConfigOptions 只按 option 自身 id 匹配请求键，
// 且取值必须是该 option 下的 sub id。曾经误用 upstream_id（那是上游再上一层的 id，
// 且原生商品该字段为 0），会导致参数被过滤后按上游默认值开通——配置与售卖不一致。
// 仅当 id 缺失（0）时才退回 upstream_id，保证老数据仍可下单。
func buildCartConfigOption(extra map[string]interface{}) map[string]string {
	out := map[string]string{}
	raw, ok := extra["config_groups"]
	if !ok {
		// 没有配置组时只能用显式覆盖（如上游只回传了部分参数）。
		return mergeExplicitConfigOption(out, extra)
	}
	var groups []ConfigGroup
	if b, err := json.Marshal(raw); err == nil {
		_ = json.Unmarshal(b, &groups)
	}
	// 先按配置组铺满全部选项（含单值固定配置：CPU/内存/区域/带宽），
	// 缺失任何一项上游都会按自己的默认值开通，与本地售卖配置不一致。
	for _, g := range groups {
		for _, opt := range g.Options {
			key := cartOptionKey(opt.ID, opt.UpstreamID)
			if key == "" {
				continue
			}
			sub := pickConfigSub(opt, extra)
			if sub == nil {
				continue
			}
			if val := cartOptionKey(sub.ID, sub.UpstreamID); val != "" {
				out[key] = val
			}
		}
	}
	// 再用显式选择覆盖：客户在界面上实际选的取值优先级最高（多值的 os/网络类型
	// 无法靠本地词汇猜准，必须由显式映射定死）。
	return mergeExplicitConfigOption(out, extra)
}

// mergeExplicitConfigOption 把显式 configoption（键=上游选项ID，值=上游子项ID）合并进 out。
//
// 合并而非替换：显式表可能只含客户改过的那几项，直接以它为准会丢掉其余选项
// （配置组已铺好的固定配置），上游便按默认值开通。
func mergeExplicitConfigOption(out map[string]string, extra map[string]interface{}) map[string]string {
	raw, ok := extra["configoption"].(map[string]interface{})
	if !ok {
		return out
	}
	for k, v := range raw {
		out[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", v)
	}
	return out
}

// cartOptionKey 取购物车下单键：优先上游自身 id，缺失时退回 upstream_id。
func cartOptionKey(ownID, upstreamID int64) string {
	if ownID != 0 {
		return strconv.FormatInt(ownID, 10)
	}
	if upstreamID != 0 {
		return strconv.FormatInt(upstreamID, 10)
	}
	return ""
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
//
// 只做主规范化，不保证跨订单唯一 —— 默认主机名的唯一性由 defaultHostname 负责。
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
	// 上游要求主机名至少 10 位：商品名短（如「4核4G」）时规范化结果会不足，
	// 提交后被上游以「主机名长度至少10位」拒绝——此时款已扣、账单已建，
	// 只能人工重试。这里补随机后缀凑够长度，避免因主机名把整笔开通拖进人工。
	const minHostnameLen = 10
	if len(h) < minHostnameLen {
		h += randStr(minHostnameLen - len(h))
	}
	return h
}

// defaultHostname 未指定主机名时的默认名：商品名规范化 + 唯一后缀。
//
// 为什么必须唯一（实测根因）：主机名直接取自商品名时，同一商品第二次开通必然与
// 上一台重名。上游（魔方财务）对重名主机的处理不是拒绝，而是把新主机卡在
// domainstatus=Pending、dcimid=0 —— 账单已付、机器永远不开通，重试也无济于事
// （实测：订单 842/843 的主机 2057/2059 卡 Pending 数十分钟，同批唯一命名的
// 主机 2056/2058 一分钟内 Active）。主机名是本地下单参数里唯一能保证唯一性的
// 标识，后缀是最小改动。
//
// 后缀来源优先取 downstream_token 的哈希：该令牌按订单生成后落库、重试复用，
// 因此同一订单的每次尝试都得到同一个主机名（对上游而言是"同一个开通请求"），
// 订单之间又互不相同。没有令牌（老订单/未装配）时退回随机后缀。
func defaultHostname(name, token string) string {
	base := safeHostname(name)
	const suffixLen = 6
	// 截短给后缀留位置（上游上限 50 位）。
	if len(base) > 50-suffixLen-1 {
		base = base[:50-suffixLen-1]
	}
	suffix := tokenSuffix(token, suffixLen)
	return strings.Trim(base, "-") + "-" + suffix
}

// tokenSuffix 由令牌派生 n 位主机名后缀（稳定、可复现）。
//
// 取**整个令牌**的哈希而不是前几位：令牌是 32 位随机十六进制，只截前 6 位
// 相当于只用了 24 bit 熵，且若上游/调用方生成的令牌前缀相同就会撞后缀。
// base36 编码让 6 位后缀承载约 31 bit（约 22 亿），对"在售主机不重名"绰绰有余。
// 令牌为空时退回随机串（老订单未装配令牌的场景）。
func tokenSuffix(token string, n int) string {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return randStr(n)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	encoded := strconv.FormatUint(uint64(h.Sum32()), 36)
	if len(encoded) < n {
		encoded = strings.Repeat("0", n-len(encoded)) + encoded
	}
	return encoded[len(encoded)-n:]
}
