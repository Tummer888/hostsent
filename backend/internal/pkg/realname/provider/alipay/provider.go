// Package alipay 提供支付宝实名认证适配器（doc104 §5.6）。
//
// 接口选择：支付宝官方的实名认证开放能力是 alipay.user.certify.open.* 系列：
//
//	initialize → 拿到 certify_id
//	certify    → 用户跳转到该入口完成人脸/证件照认证
//	query      → 查询认证结果
//
// 这属跳转式核验，因此本适配器实现 realname.Initializer；DirectVerify 明确返回
// ErrAdapterNotImplemented —— 支付宝没有官方的无跳转二/三要素核验接口，
// headless 核验必须走云市场第三方 API，不在本次范围（doc104 §11.2）。
//
// 签名与网关协议复用 pkg/alipay（与第三方登录的支付宝适配器共用）。
package alipay

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	alipaypkg "hostsent/backend/internal/pkg/alipay"
	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/realname"
)

// Type 服务商类型标识。
const Type = "alipay"

// 网关方法名。
const (
	methodInitialize = "alipay.user.certify.open.initialize"
	methodCertify    = "alipay.user.certify.open.certify"
	methodQuery      = "alipay.user.certify.open.query"
)

// 响应节点名（支付宝按 method 下划线化后加 _response 后缀）。
const (
	nodeInitialize = "alipay_user_certify_open_initialize_response"
	nodeQuery      = "alipay_user_certify_open_query_response"
)

// httpTimeout 单次网关调用超时。
//
// 5 秒足够：initialize/query 都是轻量元数据调用，真正的用户认证发生在支付宝侧。
// 对齐 pkg/captcha/provider/netease 的短超时口径——外部依赖的抖动不应拖垮请求链路。
const httpTimeout = 5 * time.Second

// maxRespBytes 响应体上限，防止异常大包打爆内存。
const maxRespBytes = 64 * 1024

func init() {
	realname.RegisterDescriptor(Type, realname.CapabilityDescriptor{
		Type: Type,
		Name: "支付宝实名认证",
		// redirect：需用户跳转到支付宝完成人脸/证件照认证，再回来查结果。
		Mode:           "redirect",
		AdapterVersion: "1.0.0",
		Icon:           "alipay",
		DocURL:         "https://opendocs.alipay.com/open/01cwn5",
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "应用 AppID", Type: integration.FieldTypeString, Required: true,
				Placeholder: "支付宝开放平台应用的 AppID"},
			{Key: "private_key", Label: "应用私钥", Type: integration.FieldTypeTextarea, Required: true, Secret: true,
				Help: "PKCS8 或 PKCS1 PEM 格式；控制台复制时可带换行，系统会自动规整"},
			{Key: "alipay_public_key", Label: "支付宝公钥", Type: integration.FieldTypeTextarea, Required: true, Secret: true,
				Help: "用于校验支付宝响应签名，与「应用公钥」不是同一把"},
			{Key: "gateway_url", Label: "网关地址", Type: integration.FieldTypeString, Required: false,
				Default: alipaypkg.DefaultGateway, Help: "沙箱环境填 https://openapi.alipaydev.com/gateway.do"},
			{Key: "sign_type", Label: "签名算法", Type: integration.FieldTypeSelect, Required: false, Default: "RSA2",
				Options: []integration.FieldOption{{Label: "RSA2", Value: "RSA2"}, {Label: "RSA", Value: "RSA"}}},
			{Key: "notify_url", Label: "结果通知地址", Type: integration.FieldTypeString, Required: false,
				Help: "可选；留空则平台靠主动查询获取结果"},
		},
		CertifyModes: []integration.FieldOption{
			{Label: "人脸认证", Value: "FACE"},
			{Label: "证件照认证", Value: "CERT_PHOTO"},
			{Label: "多因子认证", Value: "MULTI_FACTOR"},
		},
	})
	realname.RegisterFactory(Type, func(cfg realname.ProviderConfig) realname.Provider {
		return &Provider{cfg: cfg, client: &http.Client{Timeout: httpTimeout}}
	})
}

// Provider 支付宝实名认证适配器。
type Provider struct {
	cfg    realname.ProviderConfig
	client *http.Client
}

// Type 返回服务商类型。
func (p *Provider) Type() string { return Type }

// DirectVerify 支付宝无 headless 核验接口：明确未实现，由服务层回落人工审核。
func (p *Provider) DirectVerify(_ context.Context, _ realname.ProviderConfig, _ realname.VerifyRequest) (*realname.VerifyResult, error) {
	return nil, realname.ErrAdapterNotImplemented
}

// Initialize 生成支付宝认证入口：先 initialize 拿 certify_id，再签出 certify 跳转地址。
func (p *Provider) Initialize(ctx context.Context, cfg realname.ProviderConfig, req realname.VerifyRequest) (*realname.AuthChallenge, error) {
	key, err := p.credentials(cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.RealName) == "" || strings.TrimSpace(req.IDNumber) == "" {
		return nil, fmt.Errorf("%w: 姓名与证件号必填", realname.ErrProviderConfig)
	}
	bizCode := strings.TrimSpace(req.CertifyMode)
	if bizCode == "" {
		bizCode = "FACE"
	}
	// outer_order_no 是支付宝侧的幂等键：申请 ID + 毫秒时间戳，保证同一申请可重试
	// 而不会被支付宝判为重复订单。
	outerOrderNo := fmt.Sprintf("va-%d-%d", req.ApplicationID, time.Now().UnixMilli())

	biz := map[string]any{
		"outer_order_no": outerOrderNo,
		"biz_code":       bizCode,
		"identity_param": map[string]any{
			"identity_type": "CERT_INFO",
			"cert_type":     "IDENTITY_CARD",
			"cert_name":     strings.TrimSpace(req.RealName),
			"cert_no":       strings.TrimSpace(req.IDNumber),
		},
	}
	bizRaw, err := json.Marshal(biz)
	if err != nil {
		return nil, err
	}

	var resp struct {
		CertifyID string `json:"certify_id"`
		Code      string `json:"code"`
		Msg       string `json:"msg"`
		SubCode   string `json:"sub_code"`
		SubMsg    string `json:"sub_msg"`
	}
	if err := p.call(ctx, key, methodInitialize, nodeInitialize, string(bizRaw), &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.CertifyID) == "" {
		return nil, fmt.Errorf("%w: 支付宝未返回 certify_id（%s %s）", realname.ErrProviderConfig, resp.SubCode, resp.SubMsg)
	}

	authURL, err := p.certifyURL(key, resp.CertifyID)
	if err != nil {
		return nil, err
	}
	return &realname.AuthChallenge{
		AuthURL: authURL,
		TxnNo:   resp.CertifyID,
		// 支付宝认证入口的有效期由支付宝侧控制，这里给一个保守的 30 分钟供前端展示。
		ExpireAt: time.Now().Add(30 * time.Minute),
	}, nil
}

// certifyURL 拼装带签名的 certify 跳转地址（GET，参数进 query，biz_content 亦然）。
func (p *Provider) certifyURL(key *alipayKey, certifyID string) (string, error) {
	bizRaw, err := json.Marshal(map[string]any{"certify_id": certifyID})
	if err != nil {
		return "", err
	}
	form, err := alipaypkg.BuildForm(alipaypkg.CommonParams{
		AppID:    key.appID,
		Method:   methodCertify,
		SignType: key.signType,
	}, string(bizRaw), key.privateKey, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", realname.ErrProviderConfig, err)
	}
	return key.gateway + "?" + form.Encode(), nil
}

// Query 查询一次核验结果。
func (p *Provider) Query(ctx context.Context, cfg realname.ProviderConfig, txnNo string) (*realname.VerifyResult, error) {
	key, err := p.credentials(cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(txnNo) == "" {
		return nil, fmt.Errorf("%w: certify_id 为空", realname.ErrProviderConfig)
	}
	bizRaw, err := json.Marshal(map[string]any{"certify_id": txnNo})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Passed  string `json:"passed"`
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		SubCode string `json:"sub_code"`
		SubMsg  string `json:"sub_msg"`
	}
	if err := p.call(ctx, key, methodQuery, nodeQuery, string(bizRaw), &resp); err != nil {
		return nil, err
	}
	result := &realname.VerifyResult{BizCode: resp.SubCode, Message: resp.SubMsg, TxnNo: txnNo}
	switch strings.ToUpper(strings.TrimSpace(resp.Passed)) {
	case "T":
		result.Passed = true
		if result.Message == "" {
			result.Message = "支付宝实名核验通过"
		}
	case "F":
		result.Passed = false
		if result.Message == "" {
			result.Message = "支付宝实名核验未通过"
		}
	default:
		// passed 为空表示用户尚未完成认证：既不是通过也不是失败，
		// 用 PENDING 业务码让上层保持 pending 而不是误判为驳回。
		result.Passed = false
		result.BizCode = "PENDING"
		result.Message = "用户尚未完成支付宝认证"
	}
	return result, nil
}

// Test 连通性测试：校验凭证可解析 + 网关可达，不发起真实核验。
//
// 理由：真实核验需要一份真实的姓名+证件号，用假数据去调支付宝只会得到业务错误码，
// 那测的是「数据对不对」而不是「凭证配得对不对」。这里用「私钥/公钥能否解析 +
// 网关能否返回业务码」作为连通性判据（对齐 pkg/payment/manual 的 HealthCheck 口径）。
func (p *Provider) Test(ctx context.Context, cfg realname.ProviderConfig) error {
	key, err := p.credentials(cfg)
	if err != nil {
		return err
	}
	var resp struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		SubCode string `json:"sub_code"`
		SubMsg  string `json:"sub_msg"`
	}
	if err := p.call(ctx, key, methodQuery, nodeQuery, `{"certify_id":"connectivity-probe"}`, &resp); err != nil {
		return err
	}
	if resp.Code == "" && resp.SubCode == "" {
		return fmt.Errorf("%w: 网关未返回业务码，请核对网关地址", realname.ErrProviderConfig)
	}
	return nil
}

// alipayKey 解析后的凭证。
type alipayKey struct {
	appID      string
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	gateway    string
	signType   string
	notifyURL  string
}

func (p *Provider) credentials(cfg realname.ProviderConfig) (*alipayKey, error) {
	creds := cfg.Credentials
	appID := strings.TrimSpace(creds["app_id"])
	privRaw := strings.TrimSpace(creds["private_key"])
	pubRaw := strings.TrimSpace(creds["alipay_public_key"])
	if appID == "" || privRaw == "" || pubRaw == "" {
		return nil, fmt.Errorf("%w: 应用 AppID / 应用私钥 / 支付宝公钥 均为必填", realname.ErrProviderConfig)
	}
	priv, err := alipaypkg.ParsePrivateKey(privRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: 应用私钥解析失败", realname.ErrProviderConfig)
	}
	pub, err := alipaypkg.ParsePublicKey(pubRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: 支付宝公钥解析失败", realname.ErrProviderConfig)
	}
	gateway := strings.TrimSpace(creds["gateway_url"])
	if gateway == "" {
		gateway = strings.TrimSpace(cfg.Endpoint)
	}
	if gateway == "" {
		gateway = alipaypkg.DefaultGateway
	}
	return &alipayKey{
		appID:      appID,
		privateKey: priv,
		publicKey:  pub,
		gateway:    gateway,
		signType:   strings.TrimSpace(creds["sign_type"]),
		notifyURL:  strings.TrimSpace(creds["notify_url"]),
	}, nil
}

// call 发起一次开放平台调用：签名 → POST → 验签 → 解出响应节点。
//
// 安全约束：任何失败都只返回**类型化摘要**，绝不把支付宝原始报文回显给调用方，
// 避免把 app_id、内部错误细节透到前端（对齐 pkg/upstream 的错误处理口径）。
func (p *Provider) call(ctx context.Context, key *alipayKey, method, node, bizContent string, out any) error {
	form, err := alipaypkg.BuildForm(alipaypkg.CommonParams{
		AppID:     key.appID,
		Method:    method,
		SignType:  key.signType,
		NotifyURL: key.notifyURL,
	}, bizContent, key.privateKey, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", realname.ErrProviderConfig, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, key.gateway, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: 支付宝网关不可达", realname.ErrProviderConfig)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return fmt.Errorf("%w: 读取支付宝响应失败", realname.ErrProviderConfig)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: 支付宝网关返回 HTTP %d", realname.ErrProviderConfig, resp.StatusCode)
	}
	if err := alipaypkg.VerifyResponseSign(body, node, key.publicKey); err != nil {
		return fmt.Errorf("%w: 响应验签未通过", realname.ErrProviderConfig)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("%w: 支付宝响应不是合法 JSON", realname.ErrProviderConfig)
	}
	raw, ok := envelope[node]
	if !ok {
		return fmt.Errorf("%w: 支付宝响应缺少节点 %s", realname.ErrProviderConfig, node)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: 解析支付宝响应节点失败", realname.ErrProviderConfig)
	}
	return nil
}

// 确保 net/url 被使用（certifyURL 通过 BuildForm 的 Encode 使用），
// 显式断言避免未来重构时静默丢掉 URL 转义。
var _ = url.QueryEscape
