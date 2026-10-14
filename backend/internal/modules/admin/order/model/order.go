// Package model 提供订单域的数据模型。
package model

import "time"

// 订单状态
const (
	OrderStatusPending      string = "pending"      // 待支付
	OrderStatusPaid         string = "paid"         // 已支付
	OrderStatusProvisioning string = "provisioning" // 开通中
	OrderStatusActive       string = "active"       // 服务中
	OrderStatusRefunding    string = "refunding"    // 退款中
	OrderStatusRefunded     string = "refunded"     // 已退款
	OrderStatusCancelled    string = "cancelled"    // 已取消
	OrderStatusClosed       string = "closed"       // 已关闭（过期/作废）
	OrderStatusCompleted    string = "completed"    // 已完成
)

// 支付方式
const (
	PayMethodBalance string = "balance" // 余额支付
	PayMethodAlipay  string = "alipay"  // 支付宝
	PayMethodWeChat  string = "wechat"  // 微信支付
	PayMethodManual  string = "manual"  // 线下/人工
)

// 下单渠道（P6/T6.3）：空串为平台自有渠道（存量语义），open 为下游应用代客下单。
const OrderChannelOpen string = "open"

// Order 订单
type Order struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	OrderNo     string `gorm:"column:order_no;size:64;uniqueIndex;not null"` // 订单号
	UserID      uint64 `gorm:"column:user_id;index;not null"`                // 下单用户
	ProductID   uint64 `gorm:"column:product_id;index"`                      // 主产品ID(简化快照，明细见 order_items)
	ProductName string `gorm:"column:product_name;size:128"`                 // 产品名称快照
	Specs       string `gorm:"type:text"`                                    // 规格快照 JSON
	// SpecCode 所选 SKU 编码（T4.2）：履约时据此回查 spec_bindings 的平台参数；
	// 空表示按商品级配置（存量订单语义）。
	SpecCode string `gorm:"column:spec_code;size:64;index"`
	// UpstreamInvoiceID 本次开通在上游生成的账单 id（开通幂等）。
	// 上游开通 = 下单 → 结算生成账单 → 余额支付；账单 id 一旦拿到就落库，
	// 重试时复用同一张账单再支付，绝不重新下单（否则一个订单会在上游留多张账单）。
	UpstreamInvoiceID int64 `gorm:"column:upstream_invoice_id;not null;default:0"`
	// DownstreamToken 上游对接令牌（32 位，首次履约时生成后不再变化）。
	// 上游按它反查"这个下游订单在上游是否已有账单/主机"，是上游协议的幂等键；
	// 每次重试必须是同一个值，否则握手查不到既有订单，又会在上游新建一份。
	DownstreamToken string `gorm:"column:downstream_token;size:32;not null;default:''"`
	// CredentialPassword 实例登录密码的**密文**（enc:v1: AES-256-GCM，迁移 101）。
	// 与 downstream_token 同理：开通由异步 worker 重读订单执行，密码必须随单固化，
	// 否则重试会生成新密码、与上游机器上的实际密码不一致。开通成功后原样搬进实例行。
	CredentialPassword string `gorm:"column:credential_password;size:512"`
	// ConfigOptions 客户选配项选择 JSON（T4.5）：配置项参数名 → 选中取值。
	// 这是履约的唯一真源——开通参数以它为准（worker 重读订单即可取到，无需改任务表）。
	ConfigOptions string `gorm:"column:config_options;type:jsonb"`
	Quantity      int    `gorm:"default:1"`                                  // 数量
	PriceModel    string `gorm:"column:price_model;size:20;default:'fixed'"` // 价格模型
	// Cycle 计费周期快照（doc25 §3.3）：monthly/quarterly/annually 等规范值。
	// 与 price_model 并存（后者作为兼容列继续写）；开通时优先取本列作为实例计费周期。
	Cycle       string     `gorm:"column:cycle;size:20"`                                      // 计费周期
	TotalAmount float64    `gorm:"column:total_amount;type:decimal(15,2);not null;default:0"` // 应付总额
	PaidAmount  float64    `gorm:"column:paid_amount;type:decimal(15,2);not null;default:0"`  // 实付金额
	Status      string     `gorm:"size:32;not null;default:pending;index"`                    // 订单状态
	PayMethod   string     `gorm:"column:pay_method;size:20"`                                 // 支付方式
	PayTime     *time.Time `gorm:"column:pay_time"`                                           // 支付时间
	// ActivatedAt 开通成功时间（订单转为服务中的时刻）：退款时限策略的起算点。
	// pay_time 是「付钱」，本列是「交付」—— 退款窗口按交付时间算（doc 退款策略）。
	ActivatedAt *time.Time `gorm:"column:activated_at"`
	ExpireTime  *time.Time `gorm:"column:expire_time"` // 过期/计费到期时间
	Remark      string     `gorm:"size:255"`           // 管理员备注（内部可见）
	// CustomerRemark 客户备注：客户下单时填写（或事后自助修改），随履约带入实例
	// （instances.customer_remark）。与 Remark 严格分开，避免内部备注泄漏给客户。
	CustomerRemark string `gorm:"column:customer_remark;size:255"`   // 客户备注
	OperatorID     uint64 `gorm:"column:operator_id"`                // 最近操作人
	RenewalID      uint64 `gorm:"column:renewal_id;index;default:0"` // 关联续费记录 ID（续费订单，doc60）
	// 开放平台渠道（P6/T6.3）：channel='open' 表示下游应用代客下单；
	// open_app_id 指向 open_apps.id，channel_customer_ref 存下游自己的终端客户标识（对账用）。
	Channel            string `gorm:"column:channel;size:16;default:'';index"`
	OpenAppID          uint64 `gorm:"column:open_app_id;index"`
	ChannelCustomerRef string `gorm:"column:channel_customer_ref;size:128"`
	// 算价快照（P5-01/P5-04）：原价、优惠、实付与命中的折扣来源，便于对账与展示。
	OriginalAmount float64 `gorm:"column:original_amount;type:decimal(15,2);not null;default:0"` // 优惠前金额
	DiscountAmount float64 `gorm:"column:discount_amount;type:decimal(15,2);not null;default:0"` // 优惠金额
	FinalAmount    float64 `gorm:"column:final_amount;type:decimal(15,2);not null;default:0"`    // 实付金额
	PricePolicyID  *uint64 `gorm:"column:price_policy_id"`                                       // 命中的折扣策略 ID
	DiscountSource string  `gorm:"column:discount_source;size:32"`                               // agent / group / promotion / manual
	PriceSnapshot  string  `gorm:"column:price_snapshot;type:jsonb"`                             // 命中规则明细 JSON
	// SalesAdminID 销售归属快照（doc86 §1.3）：下单瞬间锁定，退款/换归属都不回改。
	// 0 表示该单不参与销售提成（下单时客户无归属）。
	SalesAdminID uint64     `gorm:"column:sales_admin_id;index;default:0"` // 归属销售（admins.id）
	CreatedAt    time.Time  `gorm:"autoCreateTime;index"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
	DeletedAt    *time.Time `gorm:"index"`
	// RenewalPrice 随单绑定的续费单价（元/周期），NULL 表示未指定（按商品定价实时算）。
	// 订单是异步开通链路里唯一贯穿的载体（占位实例 / worker 转正都重读订单），
	// 因此运营承诺的单机续费价必须在这里留快照，开通后落到 instances.renewal_price。
	RenewalPrice *float64 `gorm:"column:renewal_price;type:decimal(15,2)"`
	// 核验标记（运营侧人工核对开通结果）：只做标记，不参与状态机流转。
	// 系统配置 order_verify_required 开启时，「服务中且未核验」的订单在读取期推导为「待核验」。
	// 独立成列而不是新增订单状态：核验不阻断开通/退款，塞进 status 会把状态机与统计口径全部拖下水。
	VerifiedAt     *time.Time `gorm:"column:verified_at"`              // 核验时间（NULL=未核验）
	VerifiedBy     uint64     `gorm:"column:verified_by;default:0"`    // 核验人（admins.id）
	VerifiedByName string     `gorm:"column:verified_by_name;size:50"` // 核验人姓名快照
	VerifyRemark   string     `gorm:"column:verify_remark;size:255"`   // 核验备注
	// 查询期附加字段（不落库，doc36 §3.1）：由支付中心 payment_orders 反查填充，
	// 列表与详情据此展示并检索支付单号 / 渠道流水号。
	PaymentNo string `gorm:"-"`
	ChannelTx string `gorm:"-"`
	// 查询期附加字段（不落库）：订单关联实例（装配层注入读取端口回填，未开通为占位实例）。
	InstanceID uint64 `gorm:"-"`
	InstanceNo string `gorm:"-"`
}

// TableName 指定表名
func (Order) TableName() string {
	return "orders"
}

// OrderStatusCount 订单状态计数（统计聚合结果）
type OrderStatusCount struct {
	Status string `gorm:"column:status"`
	Count  int64  `gorm:"column:count"`
}

// OrderTrendPoint 订单趋势点（按日聚合：销售额 + 订单量）
type OrderTrendPoint struct {
	Date   string  `gorm:"column:date"`
	Sales  float64 `gorm:"column:sales"`
	Orders int64   `gorm:"column:orders"`
}
