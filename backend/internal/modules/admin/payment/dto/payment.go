// Package dto 提供支付中心的数据传输结构。
package dto

import "hostsent/backend/internal/pkg/payment"

// ListMeta 通用分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ---- 渠道类型 ----

// ChannelTypeItem 渠道类型项（驱动后台动态凭证表单与能力矩阵）。
type ChannelTypeItem struct {
	Type           string                       `json:"type"`
	Name           string                       `json:"name"`
	Mode           string                       `json:"mode"`
	Implemented    bool                         `json:"implemented"`
	AdapterVersion string                       `json:"adapter_version"`
	DocURL         string                       `json:"doc_url"`
	Icon           string                       `json:"icon"`
	Capabilities   payment.CapabilityDescriptor `json:"capabilities"`
}

// ---- 渠道实例 ----

// ChannelListQuery 渠道列表查询。
type ChannelListQuery struct {
	Type     string `form:"type" json:"type"`
	Status   *int   `form:"status" json:"status"`
	Keyword  string `form:"keyword" json:"keyword"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// ChannelCreateRequest 新建渠道。
type ChannelCreateRequest struct {
	ChannelCode  string            `json:"channel_code" binding:"required"`
	Name         string            `json:"name" binding:"required"`
	Type         string            `json:"type" binding:"required"`
	Credentials  map[string]string `json:"credentials"`
	Endpoint     string            `json:"endpoint"`
	NotifyURL    string            `json:"notify_url"`
	ReturnURL    string            `json:"return_url"`
	Scenes       []string          `json:"scenes"`
	Priority     int               `json:"priority"`
	Weight       int               `json:"weight"`
	FeeRate      float64           `json:"fee_rate"`
	SettleMode   string            `json:"settle_mode"`
	MinAmountFen int64             `json:"min_amount_fen"`
	MaxAmountFen int64             `json:"max_amount_fen"`
	Environment  string            `json:"environment"`
	IsDefault    bool              `json:"is_default"`
	Remark       string            `json:"remark"`
}

// ChannelUpdateRequest 更新渠道（Credentials 为脱敏回显时跳过该字段）。
type ChannelUpdateRequest struct {
	Name         string            `json:"name"`
	Credentials  map[string]string `json:"credentials"`
	Endpoint     string            `json:"endpoint"`
	NotifyURL    string            `json:"notify_url"`
	ReturnURL    string            `json:"return_url"`
	Scenes       []string          `json:"scenes"`
	Priority     int               `json:"priority"`
	Weight       int               `json:"weight"`
	FeeRate      float64           `json:"fee_rate"`
	SettleMode   string            `json:"settle_mode"`
	MinAmountFen int64             `json:"min_amount_fen"`
	MaxAmountFen int64             `json:"max_amount_fen"`
	Environment  string            `json:"environment"`
	IsDefault    bool              `json:"is_default"`
	Remark       string            `json:"remark"`
}

// ChannelStatusRequest 启停渠道。
type ChannelStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

// ChannelInfo 渠道信息（凭证脱敏）。
type ChannelInfo struct {
	ID           uint64                       `json:"id"`
	ChannelCode  string                       `json:"channel_code"`
	Name         string                       `json:"name"`
	Type         string                       `json:"type"`
	TypeName     string                       `json:"type_name"`
	Mode         string                       `json:"mode"`
	Credentials  map[string]string            `json:"credentials"`
	Endpoint     string                       `json:"endpoint"`
	NotifyURL    string                       `json:"notify_url"`
	ReturnURL    string                       `json:"return_url"`
	Scenes       []string                     `json:"scenes"`
	Priority     int                          `json:"priority"`
	Weight       int                          `json:"weight"`
	FeeRate      float64                      `json:"fee_rate"`
	SettleMode   string                       `json:"settle_mode"`
	MinAmountFen int64                        `json:"min_amount_fen"`
	MaxAmountFen int64                        `json:"max_amount_fen"`
	Environment  string                       `json:"environment"`
	HealthStatus string                       `json:"health_status"`
	LastError    string                       `json:"last_error"`
	LastCheckAt  string                       `json:"last_check_at"`
	Status       int                          `json:"status"`
	IsDefault    bool                         `json:"is_default"`
	Remark       string                       `json:"remark"`
	Capabilities payment.CapabilityDescriptor `json:"capabilities"`
	CreatedAt    string                       `json:"created_at"`
	UpdatedAt    string                       `json:"updated_at"`
}

// ChannelListResponse 渠道列表响应。
type ChannelListResponse struct {
	Items []ChannelInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// ChannelTestResponse 渠道连通性测试结果。
type ChannelTestResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ---- 支付方式路由 ----

// MethodSceneRule 场景路由规则。
type MethodSceneRule struct {
	Scene    string   `json:"scene"`
	Channels []string `json:"channels"` // 渠道编码，按展示顺序
}

// MethodOptionsResponse 收银台可用支付方式。
type MethodOptionsResponse struct {
	Scene    string        `json:"scene"`
	Channels []ChannelInfo `json:"channels"`
	Default  string        `json:"default"` // 默认渠道编码
}

// ---- 支付单 ----

// OrderListQuery 支付单查询。
type OrderListQuery struct {
	UserID      uint64 `form:"user_id" json:"user_id"`
	PaymentNo   string `form:"payment_no" json:"payment_no"`
	BizType     string `form:"biz_type" json:"biz_type"`
	ChannelCode string `form:"channel_code" json:"channel_code"`
	Status      string `form:"status" json:"status"`
	StartTime   string `form:"start_time" json:"start_time"`
	EndTime     string `form:"end_time" json:"end_time"`
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"page_size" json:"page_size"`
}

// PrepayRequest 发起支付。
type PrepayRequest struct {
	BizType string  `json:"biz_type" binding:"required"` // order/recharge/bill
	BizID   uint64  `json:"biz_id"`
	BizNo   string  `json:"biz_no"`
	Amount  float64 `json:"amount"` // 元；与 BizID 二选一
	Subject string  `json:"subject"`
	Scene   string  `json:"scene"`
	// ChannelCode 指定渠道；空则按场景 + 用户偏好 + 优先级路由。
	ChannelCode string `json:"channel_code"`
	ClientIP    string `json:"client_ip"`
	ReturnURL   string `json:"return_url"`
}

// OrderInfo 支付单信息。
type OrderInfo struct {
	ID           uint64  `json:"id"`
	PaymentNo    string  `json:"payment_no"`
	UserID       uint64  `json:"user_id"`
	BizType      string  `json:"biz_type"`
	BizID        uint64  `json:"biz_id"`
	BizNo        string  `json:"biz_no"`
	Amount       float64 `json:"amount"`
	AmountFen    int64   `json:"amount_fen"`
	Currency     string  `json:"currency"`
	ChannelID    uint64  `json:"channel_id"`
	ChannelCode  string  `json:"channel_code"`
	ChannelType  string  `json:"channel_type"`
	ChannelName  string  `json:"channel_name"`
	Scene        string  `json:"scene"`
	Status       string  `json:"status"`
	ChannelTx    string  `json:"channel_tx"`
	PayURL       string  `json:"pay_url"`
	QRCode       string  `json:"qrcode"`
	PrepayParams string  `json:"prepay_params"`
	Instructions string  `json:"instructions"`
	Subject      string  `json:"subject"`
	Fee          float64 `json:"fee"`
	ExpireAt     string  `json:"expire_at"`
	PaidAt       string  `json:"paid_at"`
	Remark       string  `json:"remark"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// OrderListResponse 支付单列表响应。
type OrderListResponse struct {
	Items []OrderInfo `json:"items"`
	Meta  ListMeta    `json:"meta"`
}

// OrderConfirmRequest 后台人工确认到账（线下渠道）。
type OrderConfirmRequest struct {
	ChannelTx string `json:"channel_tx"`
	Remark    string `json:"remark"`
}

// ---- 回调日志 ----

// CallbackListQuery 回调日志查询。
type CallbackListQuery struct {
	ChannelCode string `form:"channel_code" json:"channel_code"`
	PaymentNo   string `form:"payment_no" json:"payment_no"`
	VerifyOK    *bool  `form:"verify_ok" json:"verify_ok"`
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"page_size" json:"page_size"`
}

// CallbackInfo 回调日志。
type CallbackInfo struct {
	ID           uint64  `json:"id"`
	ChannelID    uint64  `json:"channel_id"`
	ChannelCode  string  `json:"channel_code"`
	NotifyID     string  `json:"notify_id"`
	PaymentNo    string  `json:"payment_no"`
	RawBody      string  `json:"raw_body"`
	VerifyOK     bool    `json:"verify_ok"`
	Amount       float64 `json:"amount"`
	HandleStatus string  `json:"handle_status"`
	HandleMsg    string  `json:"handle_msg"`
	SourceIP     string  `json:"source_ip"`
	CreatedAt    string  `json:"created_at"`
}

// CallbackListResponse 回调日志列表响应。
type CallbackListResponse struct {
	Items []CallbackInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// ---- 退款单 ----

// RefundListQuery 渠道退款单查询。
type RefundListQuery struct {
	PaymentNo string `form:"payment_no" json:"payment_no"`
	UserID    uint64 `form:"user_id" json:"user_id"`
	Status    string `form:"status" json:"status"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// RefundInfo 渠道退款单。
type RefundInfo struct {
	ID            uint64  `json:"id"`
	RefundNo      string  `json:"refund_no"`
	PaymentNo     string  `json:"payment_no"`
	OrderRefundNo string  `json:"order_refund_no"`
	UserID        uint64  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	Reason        string  `json:"reason"`
	FailReason    string  `json:"fail_reason"`
	RefundedAt    string  `json:"refunded_at"`
	CreatedAt     string  `json:"created_at"`
}

// RefundListResponse 退款单列表响应。
type RefundListResponse struct {
	Items []RefundInfo `json:"items"`
	Meta  ListMeta     `json:"meta"`
}

// ---- 打款单 ----

// PayoutListQuery 打款单查询。
type PayoutListQuery struct {
	UserID uint64 `form:"user_id" json:"user_id"`
	// BizType 业务域过滤（withdraw/sales_withdraw，doc86 §2.5）；空表示全部。
	BizType   string `form:"biz_type" json:"biz_type"`
	Status    string `form:"status" json:"status"`
	Mode      string `form:"mode" json:"mode"`
	StartTime string `form:"start_time" json:"start_time"`
	EndTime   string `form:"end_time" json:"end_time"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// PayoutInfo 打款单。
type PayoutInfo struct {
	ID          uint64  `json:"id"`
	PayoutNo    string  `json:"payout_no"`
	BizType     string  `json:"biz_type"`
	WithdrawID  uint64  `json:"withdraw_id"`
	WithdrawNo  string  `json:"withdraw_no"`
	UserID      uint64  `json:"user_id"`
	Amount      float64 `json:"amount"`
	ChannelID   uint64  `json:"channel_id"`
	ChannelCode string  `json:"channel_code"`
	Mode        string  `json:"mode"`
	Status      string  `json:"status"`
	ChannelTx   string  `json:"channel_tx"`
	ReceiptURL  string  `json:"receipt_url"`
	FailReason  string  `json:"fail_reason"`
	Attempts    int     `json:"attempts"`
	OperatorID  uint64  `json:"operator_id"`
	PaidAt      string  `json:"paid_at"`
	Remark      string  `json:"remark"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// PayoutListResponse 打款单列表响应。
type PayoutListResponse struct {
	Items []PayoutInfo `json:"items"`
	Meta  ListMeta     `json:"meta"`
}

// PayoutMarkPaidRequest 人工打款登记。
type PayoutMarkPaidRequest struct {
	ChannelTx  string `json:"channel_tx"`
	ReceiptURL string `json:"receipt_url"`
	Remark     string `json:"remark"`
}

// PayoutFailRequest 标记打款失败。
type PayoutFailRequest struct {
	Reason string `json:"reason"`
}

// ---- 对账 ----

// ReconRequest 渠道对账请求。
type ReconRequest struct {
	ChannelCode string `json:"channel_code"`
	Period      string `json:"period"` // 202609，空为全量
}

// ReconResponse 渠道对账结果。
type ReconResponse struct {
	ReconNo       string  `json:"recon_no"`
	ChannelCode   string  `json:"channel_code"`
	Period        string  `json:"period"`
	LocalAmount   float64 `json:"local_amount"`
	LocalCount    int     `json:"local_count"`
	ChannelAmount float64 `json:"channel_amount"`
	ChannelCount  int     `json:"channel_count"`
	Diff          float64 `json:"diff"`
	Status        string  `json:"status"`
}

// ReconRecordInfo 对账记录。
type ReconRecordInfo struct {
	ID            uint64  `json:"id"`
	ReconNo       string  `json:"recon_no"`
	ChannelCode   string  `json:"channel_code"`
	Period        string  `json:"period"`
	LocalAmount   float64 `json:"local_amount"`
	LocalCount    int     `json:"local_count"`
	ChannelAmount float64 `json:"channel_amount"`
	ChannelCount  int     `json:"channel_count"`
	Diff          float64 `json:"diff"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

// ReconRecordListResponse 对账记录列表响应。
type ReconRecordListResponse struct {
	Items []ReconRecordInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
}

// ---- 用户端 ----

// PreferenceItem 用户支付方式偏好项。
type PreferenceItem struct {
	Scene       string `json:"scene"`
	ChannelCode string `json:"channel_code"`
	Priority    int    `json:"priority"`
	IsDefault   bool   `json:"is_default"`
}

// PreferenceSaveRequest 保存用户支付方式偏好。
type PreferenceSaveRequest struct {
	Scene      string           `json:"scene"`
	Default    string           `json:"default"`
	Priorities []PreferenceItem `json:"priorities"`
}

// PayoutAccountCreateRequest 新增收款账户。
type PayoutAccountCreateRequest struct {
	Channel     string `json:"channel" binding:"required"` // bank/alipay
	AccountNo   string `json:"account_no" binding:"required"`
	AccountName string `json:"account_name" binding:"required"`
	BankName    string `json:"bank_name"`
	Branch      string `json:"branch"`
	IsDefault   bool   `json:"is_default"`
}

// PayoutAccountInfo 收款账户（账号脱敏）。
type PayoutAccountInfo struct {
	ID          uint64 `json:"id"`
	Channel     string `json:"channel"`
	AccountNo   string `json:"account_no"`
	AccountName string `json:"account_name"`
	BankName    string `json:"bank_name"`
	Branch      string `json:"branch"`
	IsDefault   bool   `json:"is_default"`
	Verified    bool   `json:"verified"`
	CreatedAt   string `json:"created_at"`
}

// UserWithdrawCreateRequest 用户提现申请。
type UserWithdrawCreateRequest struct {
	Amount        float64 `json:"amount" binding:"required"`
	AccountID     uint64  `json:"account_id" binding:"required"`
	PayoutChannel string  `json:"payout_channel"` // api/manual，空按平台配置
	Remark        string  `json:"remark"`
}
