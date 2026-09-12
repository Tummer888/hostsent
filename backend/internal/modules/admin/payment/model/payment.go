// Package model 提供支付中心的数据模型。
//
// 设计（docs/实施计划/35）：支付单与业务解耦（biz_type + biz_id），
// 金额一律整数分（amount_fen），出口由适配器转换渠道要求的格式。
package model

import (
	"time"

	"gorm.io/gorm"
)

// 支付模式
const (
	ModeAPI    = "api"    // 接口自动
	ModeManual = "manual" // 线下人工
)

// 渠道状态
const (
	StatusEnabled  int = 1
	StatusDisabled int = 0
)

// 支付单状态
const (
	OrderStatusPending   string = "pending"   // 待支付
	OrderStatusPaying    string = "paying"    // 支付中（已下单待回调）
	OrderStatusPaid      string = "paid"      // 已支付
	OrderStatusFailed    string = "failed"    // 支付失败
	OrderStatusClosed    string = "closed"    // 已关闭（超时/手动）
	OrderStatusRefunding string = "refunding" // 退款中
	OrderStatusRefunded  string = "refunded"  // 已退款
)

// 业务类型
const (
	BizTypeOrder    string = "order"    // 订单支付
	BizTypeRecharge string = "recharge" // 余额充值
	BizTypeBill     string = "bill"     // 账单支付
)

// 退款状态
const (
	RefundStatusPending  string = "pending"
	RefundStatusSuccess  string = "success"
	RefundStatusFailed   string = "failed"
	RefundStatusRejected string = "rejected"
)

// 打款状态
const (
	PayoutStatusPending string = "pending" // 待打款
	PayoutStatusPaying  string = "paying"  // 打款中
	PayoutStatusPaid    string = "paid"    // 已打款
	PayoutStatusFailed  string = "failed"  // 打款失败
)

// 打款模式（与 finance/withdraw 的 PayoutMode* 同值）
const (
	PayoutModeManual string = "manual" // 人工打款登记
	PayoutModeAPI    string = "api"    // 渠道接口自动打款
)

// PaymentType 支付渠道类型注册表：一行 = 一种可接入的支付方式。
// DescriptorJSON 存 CapabilityDescriptor（场景/操作/凭证 schema/证书模式/费率/结算）。
type PaymentType struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	Type           string    `gorm:"column:type;size:50;not null;uniqueIndex:uk_payment_types_type"`
	Name           string    `gorm:"size:100;not null"`
	Mode           string    `gorm:"size:20;not null;default:api"`
	DescriptorJSON string    `gorm:"column:descriptor;type:jsonb"`
	Icon           string    `gorm:"size:64"`
	DocURL         string    `gorm:"column:doc_url;size:255"`
	AdapterVersion string    `gorm:"column:adapter_version;size:32"`
	SortOrder      int       `gorm:"column:sort_order;not null;default:0"`
	Status         int       `gorm:"not null;default:1"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

func (PaymentType) TableName() string { return "payment_types" }

// PaymentChannel 支付渠道实例：同一类型可有多个实例（不同商户号/费率/结算账户）。
// Credentials 为字段级加密后的凭证 JSON（复用 pkg/credentials）。
type PaymentChannel struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	ChannelCode  string     `gorm:"column:channel_code;size:64;not null;uniqueIndex:uk_payment_channels_code"`
	Name         string     `gorm:"size:100;not null"`
	Type         string     `gorm:"size:50;not null;index"`
	Credentials  string     `gorm:"column:credentials;type:jsonb"`
	Endpoint     string     `gorm:"size:255"`
	NotifyURL    string     `gorm:"column:notify_url;size:255"`
	ReturnURL    string     `gorm:"column:return_url;size:255"`
	Scenes       string     `gorm:"type:jsonb"`                                           // 启用场景数组
	Priority     int        `gorm:"not null;default:0"`                                   // 路由优先级，越大越优先
	Weight       int        `gorm:"not null;default:0"`                                   // 同优先级内权重
	FeeRate      float64    `gorm:"column:fee_rate;type:numeric(6,4);not null;default:0"` // 费率
	SettleMode   string     `gorm:"column:settle_mode;size:20;not null;default:''"`       // T+0/T+1
	MinAmountFen int64      `gorm:"column:min_amount_fen;not null;default:0"`
	MaxAmountFen int64      `gorm:"column:max_amount_fen;not null;default:0"`         // 0=不限
	Environment  string     `gorm:"size:20;not null;default:prod"`                    // sandbox/prod
	HealthStatus string     `gorm:"column:health_status;size:20;not null;default:''"` // ''/healthy/down
	LastError    string     `gorm:"column:last_error;type:text"`
	LastCheckAt  *time.Time `gorm:"column:last_check_at"`
	Status       int        `gorm:"not null;default:1;index"`
	Remark       string     `gorm:"size:255"`
	// IsDefault 平台默认渠道（用于无用户偏好时的兜底，按场景各取一个）。
	IsDefault bool           `gorm:"column:is_default;not null;default:false"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (PaymentChannel) TableName() string { return "payment_channels" }

// PaymentOrder 支付单：以 biz_type + biz_id 与业务解耦，财务/订单不感知渠道细节。
type PaymentOrder struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	PaymentNo    string `gorm:"column:payment_no;size:64;not null;uniqueIndex;index"`
	UserID       uint64 `gorm:"column:user_id;not null;index"`
	BizType      string `gorm:"column:biz_type;size:32;not null;index"`
	BizID        uint64 `gorm:"column:biz_id;not null;default:0"`
	BizNo        string `gorm:"column:biz_no;size:64"`
	AmountFen    int64  `gorm:"column:amount_fen;not null"`
	Currency     string `gorm:"size:10;not null;default:CNY"`
	ChannelID    uint64 `gorm:"column:channel_id;not null;default:0;index"`
	ChannelCode  string `gorm:"column:channel_code;size:64;not null;default:''"`
	ChannelType  string `gorm:"column:channel_type;size:50;not null;default:''"`
	Scene        string `gorm:"size:20;not null;default:''"`
	Status       string `gorm:"size:20;not null;default:pending;index"`
	ChannelTx    string `gorm:"column:channel_tx;size:128;index"`
	PayURL       string `gorm:"column:pay_url;size:512"`
	QRCode       string `gorm:"column:qrcode;size:512"`
	PrepayParams string `gorm:"column:prepay_params;type:jsonb"`
	Instructions string `gorm:"type:text"`
	Subject      string `gorm:"size:255"`
	// BizItems 合并支付明细（doc36 §3.5）：一单多业务时记录各业务单号与金额，普通单为空数组。
	// 业务单据本身不合并，仍各自独立履约/退款/开票，避免退款与发票粒度丢失。
	// 形如 [{"biz_type":"order","biz_id":1,"biz_no":"...","amount_fen":100,"status":"paid"}]。
	BizItems  string     `gorm:"column:biz_items;type:jsonb;not null;default:'[]'"`
	ItemCount int        `gorm:"column:item_count;not null;default:1"`
	ClientIP  string     `gorm:"column:client_ip;size:64"`
	FeeFen    int64      `gorm:"column:fee_fen;not null;default:0"` // 渠道手续费（分）
	ExpireAt  *time.Time `gorm:"column:expire_at"`
	PaidAt    *time.Time `gorm:"column:paid_at"`
	Remark    string     `gorm:"size:255"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`
}

func (PaymentOrder) TableName() string { return "payment_orders" }

// PaymentCallbackLog 回调日志：记录原始报文、验签结果与处理结果，支持重放与排障。
type PaymentCallbackLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	ChannelID    uint64    `gorm:"column:channel_id;not null;default:0;index"`
	ChannelCode  string    `gorm:"column:channel_code;size:64;not null;default:''"`
	NotifyID     string    `gorm:"column:notify_id;size:128;index"` // 渠道回调唯一标识（幂等）
	PaymentNo    string    `gorm:"column:payment_no;size:64;index"`
	RawBody      string    `gorm:"column:raw_body;type:text"`
	Headers      string    `gorm:"type:text"`
	VerifyOK     bool      `gorm:"column:verify_ok;not null;default:false"`
	AmountFen    int64     `gorm:"column:amount_fen;not null;default:0"`
	HandleStatus string    `gorm:"column:handle_status;size:20;not null;default:''"` // ok/ignored/duplicate/error
	HandleMsg    string    `gorm:"column:handle_msg;type:text"`
	SourceIP     string    `gorm:"column:source_ip;size:64"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (PaymentCallbackLog) TableName() string { return "payment_callback_logs" }

// PaymentRefund 渠道退款单。
type PaymentRefund struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement"`
	RefundNo        string     `gorm:"column:refund_no;size:64;not null;uniqueIndex"`
	PaymentNo       string     `gorm:"column:payment_no;size:64;not null;index"`
	OrderRefundNo   string     `gorm:"column:order_refund_no;size:64;index"` // 关联订单退款单号
	UserID          uint64     `gorm:"column:user_id;not null;index"`
	AmountFen       int64      `gorm:"column:amount_fen;not null"`
	ChannelID       uint64     `gorm:"column:channel_id;not null;default:0"`
	ChannelRefundID string     `gorm:"column:channel_refund_id;size:128"`
	Status          string     `gorm:"size:20;not null;default:pending;index"`
	Reason          string     `gorm:"size:255"`
	FailReason      string     `gorm:"column:fail_reason;type:text"`
	RefundedAt      *time.Time `gorm:"column:refunded_at"`
	CreatedAt       time.Time  `gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime"`
}

func (PaymentRefund) TableName() string { return "payment_refunds" }

// PaymentPayout 打款单：提现审批通过后产生，人工登记或 API 自动打款。
type PaymentPayout struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	PayoutNo    string     `gorm:"column:payout_no;size:64;not null;uniqueIndex"`
	WithdrawID  uint64     `gorm:"column:withdraw_id;not null;default:0;index"`
	WithdrawNo  string     `gorm:"column:withdraw_no;size:64"`
	UserID      uint64     `gorm:"column:user_id;not null;index"`
	AmountFen   int64      `gorm:"column:amount_fen;not null"`
	ChannelID   uint64     `gorm:"column:channel_id;not null;default:0"`
	ChannelCode string     `gorm:"column:channel_code;size:64;not null;default:''"`
	Mode        string     `gorm:"size:20;not null;default:manual"` // api/manual
	Status      string     `gorm:"size:20;not null;default:pending;index"`
	ChannelTx   string     `gorm:"column:channel_tx;size:128"`
	ReceiptURL  string     `gorm:"column:receipt_url;size:512"` // 人工打款凭证
	FailReason  string     `gorm:"column:fail_reason;type:text"`
	Attempts    int        `gorm:"not null;default:0"`
	OperatorID  uint64     `gorm:"column:operator_id"`
	PaidAt      *time.Time `gorm:"column:paid_at"`
	Remark      string     `gorm:"size:255"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

func (PaymentPayout) TableName() string { return "payment_payouts" }

// UserPayoutAccount 用户收款账户（提现目标）。
type UserPayoutAccount struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `gorm:"column:user_id;not null;index"`
	Channel     string    `gorm:"size:20;not null;default:bank"`       // bank/alipay
	AccountNo   string    `gorm:"column:account_no;size:255;not null"` // 字段级加密
	AccountName string    `gorm:"column:account_name;size:64;not null"`
	BankName    string    `gorm:"column:bank_name;size:100"`
	Branch      string    `gorm:"size:100"`
	IsDefault   bool      `gorm:"column:is_default;not null;default:false"`
	Verified    bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (UserPayoutAccount) TableName() string { return "user_payout_accounts" }

// UserPaymentPreference 用户支付方式偏好（默认方式 + 优先级）。
type UserPaymentPreference struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `gorm:"column:user_id;not null;uniqueIndex:uk_user_pay_pref"`
	Scene       string    `gorm:"size:20;not null;default:'';uniqueIndex:uk_user_pay_pref"`
	ChannelCode string    `gorm:"column:channel_code;size:64;not null;uniqueIndex:uk_user_pay_pref"`
	Priority    int       `gorm:"not null;default:0"`
	IsDefault   bool      `gorm:"column:is_default;not null;default:false"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (UserPaymentPreference) TableName() string { return "user_payment_preferences" }

// PaymentReconRecord 渠道对账记录。
type PaymentReconRecord struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement"`
	ReconNo          string    `gorm:"column:recon_no;size:64;not null;uniqueIndex"`
	ChannelID        uint64    `gorm:"column:channel_id;not null;default:0;index"`
	ChannelCode      string    `gorm:"column:channel_code;size:64;not null;default:''"`
	Period           string    `gorm:"size:20;not null;default:''"`
	LocalAmountFen   int64     `gorm:"column:local_amount_fen;not null;default:0"`
	LocalCount       int       `gorm:"column:local_count;not null;default:0"`
	ChannelAmountFen int64     `gorm:"column:channel_amount_fen;not null;default:0"`
	ChannelCount     int       `gorm:"column:channel_count;not null;default:0"`
	DiffFen          int64     `gorm:"column:diff_fen;not null;default:0"`
	Status           string    `gorm:"size:20;not null;default:''"` // matched/suspicious
	Detail           string    `gorm:"type:jsonb"`
	CreatedAt        time.Time `gorm:"autoCreateTime"`
}

func (PaymentReconRecord) TableName() string { return "payment_recon_records" }
