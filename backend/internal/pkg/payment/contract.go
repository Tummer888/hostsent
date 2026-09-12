// Package payment 定义统一支付网关抽象（对标 pkg/upstream 的契约层）。
//
// 设计（docs/实施计划/35）：
//   - 支付渠道与上游云渠道同构：多种类型、凭证各异、签名不同、场景有别，
//     因此复用「契约 + 能力描述符 + 工厂注册表 + 动态凭证表单」这一套解法。
//   - 能力靠类型断言发现（Collector/PaymentQuerier/Payouter/...），
//     后台靠 CapabilityDescriptor 提前展示"这个渠道支持什么"。
//   - 新增一家支付渠道 = 一个适配器包 + 描述符 + 一行 payment_types，核心零改动。
//
// 边界：本包只管"钱进出的通道与确认"，不触碰账本。
//
//	收款成功 → 发布事件 → finance 调 WalletService.Change 入账；
//	出款由 finance 通过最小接口（Payouter）调用，payment 不依赖 finance。
package payment

import (
	"context"
	"net/http"
)

// ---- 场景 ----
const (
	SceneScan   = "scan"   // 主扫/被扫（二维码）
	SceneH5     = "h5"     // 手机网页
	SceneJSAPI  = "jsapi"  // 公众号内
	SceneMini   = "mini"   // 小程序
	SceneApp    = "app"    // APP SDK
	SceneNative = "native" // PC 原生（跳转/二维码）
)

// ScenesAll 全部场景（后台多选与校验用）。
var ScenesAll = []string{SceneScan, SceneH5, SceneJSAPI, SceneMini, SceneApp, SceneNative}

// ---- 能力操作 ----
const (
	OpCollect = "collect" // 收款下单
	OpQuery   = "query"   // 主动查单
	OpClose   = "close"   // 关单
	OpRefund  = "refund"  // 渠道退款
	OpPayout  = "payout"  // 打款（代付/转账）
)

// ---- 支付方式模式 ----
const (
	ModeAPI    = "api"    // 接口自动
	ModeManual = "manual" // 线下人工（登记 + 后台确认）
)

// ---- 签名/证书模式（供后台能力矩阵展示） ----
const (
	SignerNone = "none"
	SignerMD5  = "md5"  // 微信 v2 / 支付宝旧版
	SignerRSA2 = "rsa2" // 支付宝
	SignerV3   = "v3"   // 微信支付 v3（证书）
)

const (
	CertModeNone = ""
	CertModeKey  = "key"
	CertModeCert = "cert"
)

// ---- 结算模式 ----
const (
	SettleT0 = "T+0"
	SettleT1 = "T+1"
)

// Gateway 支付网关统一接口 —— 仅保留全适配器共有的最小能力。
type Gateway interface {
	GetType() string
	Capabilities() CapabilityDescriptor
	HealthCheck(ctx context.Context) error
}

// CredentialMap 渠道凭证键值对（由装配层解密后传入）。
type CredentialMap map[string]string

// Get 读取凭证。
func (c CredentialMap) Get(k string) string {
	if c == nil {
		return ""
	}
	return c[k]
}

// ChannelConfig 渠道实例运行配置（凭证已解密）。
type ChannelConfig struct {
	ID          uint64
	Code        string
	Name        string
	Type        string
	Endpoint    string
	NotifyURL   string
	ReturnURL   string
	Credentials CredentialMap
	Extra       map[string]string
}

// Collector 收款下单能力。
type Collector interface {
	Prepay(ctx context.Context, cfg *ChannelConfig, req *PrepayRequest) (*PrepayResult, error)
}

// PaymentQuerier 主动查单能力。
type PaymentQuerier interface {
	Query(ctx context.Context, cfg *ChannelConfig, req *QueryRequest) (*PaymentState, error)
}

// PaymentCloser 关单能力。
type PaymentCloser interface {
	Close(ctx context.Context, cfg *ChannelConfig, req *CloseRequest) error
}

// Refunder 渠道退款能力。
type Refunder interface {
	Refund(ctx context.Context, cfg *ChannelConfig, req *RefundRequest) (*RefundResult, error)
}

// RefundQuerier 退款查询能力。
type RefundQuerier interface {
	QueryRefund(ctx context.Context, cfg *ChannelConfig, req *QueryRequest) (*RefundState, error)
}

// Payouter 打款（代付/转账）能力。
type Payouter interface {
	Payout(ctx context.Context, cfg *ChannelConfig, req *PayoutRequest) (*PayoutResult, error)
}

// PayoutQuerier 打款查询能力。
type PayoutQuerier interface {
	QueryPayout(ctx context.Context, cfg *ChannelConfig, req *QueryRequest) (*PayoutState, error)
}

// NotifyParser 回调解析 + 验签能力。
type NotifyParser interface {
	ParseNotify(ctx context.Context, cfg *ChannelConfig, r *http.Request, rawBody []byte) (*NotifyEvent, error)
}

// ---- 请求/响应 ----

// PrepayRequest 收款下单请求。金额一律整数分。
type PrepayRequest struct {
	OutTradeNo string // 商户支付单号（payment_no）
	AmountFen  int64
	Currency   string
	Subject    string
	Scene      string
	ClientIP   string
	ReturnURL  string
	Extra      map[string]string
}

// PrepayResult 收款下单结果。
type PrepayResult struct {
	// PayURL 跳转链接或二维码内容（H5/PC/扫码）。
	PayURL string
	// QRCode 二维码内容（如有）。
	QRCode string
	// ChannelTx 渠道预下单号（如微信 prepay_id）。
	ChannelTx string
	// Params 前端 SDK 所需参数（JSAPI/APP）。
	Params map[string]string
	// Instructions 线下人工模式的操作指引（转账账户/备注要求）。
	Instructions string
	// Raw 渠道原始响应（排障用）。
	Raw map[string]interface{}
}

// QueryRequest 查单/查退款/查打款请求。
type QueryRequest struct {
	OutTradeNo string
	ChannelTx  string
	Extra      map[string]string
}

// CloseRequest 关单请求。
type CloseRequest struct {
	OutTradeNo string
	ChannelTx  string
	Extra      map[string]string
}

// RefundRequest 渠道退款请求。
type RefundRequest struct {
	OutTradeNo  string
	OutRefundNo string
	AmountFen   int64
	TotalFen    int64
	Reason      string
	Extra       map[string]string
}

// RefundResult 渠道退款结果。
type RefundResult struct {
	ChannelRefundID string
	Status          string // pending/success/failed
	Raw             map[string]interface{}
}

// PayoutRequest 打款请求。
type PayoutRequest struct {
	OutPayoutNo string
	AmountFen   int64
	AccountType string // bank/alipay
	AccountNo   string
	AccountName string
	BankName    string
	Branch      string
	Remark      string
	Extra       map[string]string
}

// PayoutResult 打款结果。
type PayoutResult struct {
	ChannelTx string
	Status    string // paying/paid/failed
	Receipt   string // 打款凭证 URL
	Message   string
	Raw       map[string]interface{}
}

// PaymentState 支付状态。
type PaymentState struct {
	Status    string // pending/paid/failed/closed
	ChannelTx string
	PaidAt    int64 // Unix 秒，0 表示无
	AmountFen int64
}

// RefundState 退款状态。
type RefundState struct {
	Status          string
	ChannelRefundID string
	AmountFen       int64
}

// PayoutState 打款状态。
type PayoutState struct {
	Status    string
	ChannelTx string
	Message   string
}

// NotifyEvent 回调解析后的结构化事件。
type NotifyEvent struct {
	// NotifyID 渠道回调唯一标识（用于幂等去重）；渠道未提供时用渠道交易号。
	NotifyID string
	// OutTradeNo 商户支付单号。
	OutTradeNo string
	// ChannelTx 渠道交易号。
	ChannelTx string
	// AmountFen 渠道回报金额（用于与本地支付单比对）。
	AmountFen int64
	// Currency 币种。
	Currency string
	// Status 归一状态：paid/failed/closed/refunded。
	Status string
	// PaidAt Unix 秒。
	PaidAt int64
	// Raw 原始报文（留痕）。
	Raw map[string]interface{}
}
