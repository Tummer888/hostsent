// Package dto 定义用户中心订单模块数据结构。
package dto

import (
	"time"

	"hostsent/backend/internal/pkg/pricing"
)

// 支付方式（doc88 §6.2「待支付 + 收银台」）：
//   - balance 余额支付：下单即扣款、订单直接 paid，与存量行为一致；
//   - channel 渠道支付：订单落 pending 等用户到收银台付款，支付成功回调后自动开通。
const (
	PayModeBalance = "balance"
	PayModeChannel = "channel"
)

// 订单状态常量（本模块本地面，语义与 admin/order model 同名常量一致）。
// 只收敛服务层内部状态判断，不参与表结构定义（CONTRIBUTING §3：
// uc 端不得 import admin 域 model）。
const (
	OrderStatusPending   = "pending"   // 待支付
	OrderStatusPaid      = "paid"      // 已支付
	OrderStatusActive    = "active"    // 服务中
	OrderStatusCancelled = "cancelled" // 已取消
	OrderStatusClosed    = "closed"    // 已关闭（过期/作废）
)

// ProvisionTaskPending 开通履约任务「待处理」状态（语义同 admin/order model）。
const ProvisionTaskPending = "pending"

// OrderItemRow 订单项行结构（order_items 表的本地面投影，P5-04 行级折扣快照）。
type OrderItemRow struct {
	OrderID        uint64
	ProductID      uint64
	ProductName    string
	SpecCode       string
	Specs          string
	ConfigOptions  string
	Cycle          string
	Price          float64
	Quantity       int
	Amount         float64
	OriginalAmount float64
	DiscountAmount float64
	FinalAmount    float64
}

// OrderRow 用户中心订单服务使用的订单行结构（admin/order 模型的本地面投影）。
//
// 该结构只包含下单/读单/关单链路实际使用的字段，与 orders 表的 GORM 模型
// 逐字段同名同型；装配层负责把 admin 域的订单模型投影成本结构（或反向）。
type OrderRow struct {
	ID          uint64
	OrderNo     string
	UserID      uint64
	ProductID   uint64
	ProductName string
	Specs       string
	SpecCode    string
	Quantity    int
	PriceModel  string
	Cycle       string
	TotalAmount float64
	PaidAmount  float64
	Status      string
	PayMethod   string
	PayTime     *time.Time
	ExpireTime  *time.Time
	// ConfigOptions 客户选配项选择 JSON（T4.5），履约与回显真源。
	ConfigOptions string
	// 算价快照（P5-01）：原价/优惠/实付与折扣来源。
	OriginalAmount float64
	DiscountAmount float64
	FinalAmount    float64
	PricePolicyID  *uint64
	DiscountSource string
	PriceSnapshot  string
	// OperatorID 真实操作人（子账号下单可追溯，P4-09）。
	OperatorID uint64
	// CustomerRemark 客户备注（随履约带入实例）。
	CustomerRemark string
	// 开放平台渠道（P6/T6.3）。
	Channel            string
	OpenAppID          uint64
	ChannelCustomerRef string
	// SalesAdminID 销售归属快照（doc86 §3.4）。
	SalesAdminID uint64
	// RenewalID 续费订单标记（占位钩子按此跳过，不建新实例）。
	RenewalID uint64
	// RenewalPrice 随单绑定的续费单价（占位钩子/转正链路读取）。
	RenewalPrice *float64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ProvisionTaskRow 开通履约任务状态行（admin/order model.ProvisionTask 的本地面投影）。
type ProvisionTaskRow struct {
	OrderID   uint64
	Status    string
	LastError string
}

// CreateRequest 用户下单请求。
type CreateRequest struct {
	ProductID uint64 `json:"product_id" binding:"required"`
	// SpecCode 规格变体编码（SKU，T4.1）；商品挂有规格时必填，未挂规格可留空。
	SpecCode string `json:"spec_code"`
	Quantity int    `json:"quantity"`
	// Cycle 计费周期（doc25）：monthly/quarterly/annually 等规范值。
	// 留空回落商品 price_model 对应周期（存量行为不变）；该周期未开放则被拒。
	Cycle string `json:"cycle"`
	// PayMode 支付方式：balance（默认）/ channel。空值按 balance 处理，保证存量调用方行为不变。
	PayMode string `json:"pay_mode"`
	// PayMethod 具体收款方式（alipay/wechat/balance/manual 等）：**下单即写入订单**，
	// 使「订单 ↔ 支付方式」强绑定 —— 待支付订单在后台列表里就能看出客户打算用什么付，
	// 场外收款人工入账时也能据此预填。
	//
	// 与 PayMode 的分工：PayMode 决定「余额支付还是走收银台」这条流程分支，
	// PayMethod 记录「实际用什么收的钱」。留空时按用户默认支付方式解析
	// （用户中心设置的默认渠道 > 平台默认渠道 > 余额）。
	// 到账后一律以实际收款方式覆盖（渠道回调 / 人工入账选择）。
	PayMethod string `json:"pay_method"`
	// ConfigSelections 客户选配项选择（T4.5）：配置项参数名 → 选中取值
	// （如 {"cpu":"4","os":"12","ip_num":"2"}）。留空表示全部按默认值。
	ConfigSelections map[string]string `json:"config_selections"`
	// CustomerRemark 客户备注（选填）：客户对这台机器的说明（如用途/归属项目），
	// 随履约带入实例 instances.customer_remark，与运营内部备注分开存放。
	CustomerRemark string `json:"customer_remark"`
	// ChannelMeta 开放平台代客下单渠道信息（P6/T6.3）。仅由 open 模块程序化注入，
	// json:"-" 保证 UC 自有 HTTP 路由无法伪造渠道标记。
	ChannelMeta ChannelMeta `json:"-"`
}

// PayRequest 发起收银台支付。
type PayRequest struct {
	// ChannelCode 指定支付渠道（来自 GET /uc/payment/methods）；留空按场景 + 用户偏好路由。
	ChannelCode string `json:"channel_code"`
	// Scene 支付场景：native/h5/jsapi/scan，留空按 native。
	Scene string `json:"scene"`
}

// ChannelMeta 下单渠道归属：channel='open' 的订单由下游应用代客下单，
// open_app_id 指向 open_apps.id，customer_ref 存下游自己的终端客户标识。
type ChannelMeta struct {
	Channel            string
	OpenAppID          uint64
	ChannelCustomerRef string
}

// OrderInfo 用户可见订单信息。
// 注意：orders 表无 spec_code 列，SKU 编码落在 order_items（后台订单详情可见）；
// 订单的 specs 已是所选 SKU 的规格快照。
type OrderInfo struct {
	ID          uint64  `json:"id"`
	OrderNo     string  `json:"order_no"`
	ProductID   uint64  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Specs       string  `json:"specs"`
	PriceModel  string  `json:"price_model"`
	TotalAmount float64 `json:"total_amount"`
	PaidAmount  float64 `json:"paid_amount"`
	Status      string  `json:"status"`
	PayMethod   string  `json:"pay_method"`
	// ActorID/ActorName 下单的真实操作人（子账号下单可见，P4-09）
	ActorID   uint64 `json:"actor_user_id"`
	ActorName string `json:"actor_name"`
	// SpecCode 所选 SKU 编码（T4.1）；Quantity 数量；Cycle 计费周期（doc25）。
	// 列表与详情都返回，详情页据此展示「规格 / 周期 / 数量」。
	SpecCode string `json:"spec_code"`
	Quantity int    `json:"quantity"`
	Cycle    string `json:"cycle"`
	// ConfigSelections 下单时选定的客户选配项（T4.5），详情页展示与售后核对用。
	// 注意：键在对接商品上是**上游选项 id**（如 "163744"），值也是上游取值 id，
	// 属于对接口径。前端不要直接展示，用下面的 ConfigSelectionLabels。
	ConfigSelections map[string]string `json:"config_selections,omitempty"`
	// ConfigSelectionLabels 选配项的可读展示名（键值同序，如 ["网络类型：VPC网络","操作系统：CentOS 7.9"]）。
	// 由商品目录把上游 id 翻译成客户看得懂的名称；翻译不到的项不出现在这里
	// （宁可不显示，也不把 "163744: 565652" 这种内部编号摊给客户）。
	ConfigSelectionLabels []string `json:"config_selection_labels,omitempty"`
	// OptionAmount 选配项加价小计（T4.5），用于把「档位价 + 选配」拆开展示。
	OptionAmount float64 `json:"option_amount,omitempty"`
	// PayTime 支付时间（RFC3339，未支付为空串）；列表不保证填充，详情页用于展示。
	PayTime string `json:"pay_time"`
	// 算价明细（P5-04/P5-06）：原价、优惠、实付与折扣来源，用于订单列表/详情展示。
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	DiscountSource string  `json:"discount_source"`
	CreatedAt      string  `json:"created_at"`
	// 开通履约状态（T5.1）：pending/running/success/failed/manual，供前端轮询展示；
	// 未装配工作池或非异步开通的订单为空。
	ProvisionStatus string `json:"provision_status,omitempty"`
	// ProvisionError 开通失败原因（failed/manual 时展示，便于用户/运维排查）。
	ProvisionError string `json:"provision_error,omitempty"`
	// PriceSnapshot 命中规则明细 JSON（P5-04），详情页展示优惠构成；列表不填充。
	PriceSnapshot string `json:"price_snapshot,omitempty"`
	// ExpireTime 订单过期/计费到期时间（RFC3339，无则为空串）；详情页展示。
	ExpireTime string `json:"expire_time,omitempty"`
	// PayExpireAt 待支付订单的支付截止时间（RFC3339）：由「下单时间 + order_expire_minutes」
	// 在读接口实时推导，不落库（orders.expire_time 是实例计费到期时间，两者语义不同）。
	// 仅 pending 订单有值，供前端提示「请在 xx 前完成支付」。
	PayExpireAt string `json:"pay_expire_at,omitempty"`
	// Remark 订单备注。用户侧恒为空：orders.remark 是运营在管理端写的**内部备注**，
	// 不在用户端暴露（客户自己的备注见 CustomerRemark）。字段保留是为了共用结构体。
	Remark string `json:"remark,omitempty"`
	// CustomerRemark 客户备注（客户下单填写的那条）。
	CustomerRemark string `json:"customer_remark,omitempty"`
	// RefundHint 可退性提示（详情接口填充）：前端据此展示「申请退款」入口与时限。
	RefundHint *OrderRefundHint `json:"refund_hint,omitempty"`
}

// PayInfo 发起收银台支付的结果：收银台组件渲染所需参数（与支付中心 OrderInfo 同口径裁剪）。
type PayInfo struct {
	OrderID   uint64 `json:"order_id"`
	OrderNo   string `json:"order_no"`
	PaymentID uint64 `json:"payment_order_id"`
	PaymentNo string `json:"payment_no"`
	// Amount 应付金额（元）；与订单应付一致，由服务端取算价快照，不接受前端传入。
	Amount       float64 `json:"amount"`
	ChannelCode  string  `json:"channel_code"`
	ChannelName  string  `json:"channel_name"`
	Scene        string  `json:"scene"`
	Status       string  `json:"status"`
	PayURL       string  `json:"pay_url"`
	QRCode       string  `json:"qrcode"`
	Instructions string  `json:"instructions"`
	Subject      string  `json:"subject"`
	ExpireAt     string  `json:"expire_at"`
}

// QuoteRequest 预结算请求（P5-05）：不落库、不扣款。
type QuoteRequest struct {
	ProductID uint64 `json:"product_id" binding:"required"`
	// SpecCode 规格变体编码（SKU，T4.1）；与下单口径一致。
	SpecCode string `json:"spec_code"`
	Quantity int    `json:"quantity"`
	// Cycle 计费周期（doc25）；与下单口径一致，留空回落商品 price_model。
	Cycle string `json:"cycle"`
	// ConfigSelections 客户选配项选择（T4.5）：与下单口径一致。
	ConfigSelections map[string]string `json:"config_selections"`
}

// QuoteInfo 预结算价格明细。
type QuoteInfo struct {
	SpecCode       string  `json:"spec_code"`
	Cycle          string  `json:"cycle"`
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	// OptionAmount 选配项加价小计（T4.5）：已含在 OriginalAmount 内，前端逐项展示用。
	OptionAmount float64 `json:"option_amount"`
	// Options 已选选项明细（参数名 / 选中值 / 加价），供预结算页展示加价来源。
	Options  []SelectedOption `json:"options"`
	PolicyID *uint64          `json:"price_policy_id"`
	Source   string           `json:"discount_source"`
	Snapshot []pricing.Rule   `json:"price_snapshot"`
}

// SelectedOption 单个已选配置项的加价明细。
type SelectedOption struct {
	OptionKey   string  `json:"option_key"`
	OptionName  string  `json:"option_name"`
	Value       string  `json:"value"`
	ValueName   string  `json:"value_name"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	IsDefaulted bool    `json:"is_defaulted"`
}

// ListQuery 我的订单查询。
type ListQuery struct {
	Status   string `form:"status" json:"status"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// ListResponse 我的订单列表响应。
type ListResponse struct {
	Items    []OrderInfo `json:"items"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int64       `json:"total"`
}

// UserRefundRequest 客户自助退款申请。
type UserRefundRequest struct {
	// Reason 退款原因（必填：审核与后续复盘依据）。
	Reason string `json:"reason" binding:"required"`
	// RefundMode 期望去向：balance=退回余额（默认）/ channel=原路退回。
	// 余额支付的订单只能选 balance；前端按 RefundHint.ChannelAllowed 锁定。
	RefundMode string `json:"refund_mode"`
}

// UserRefundResult 退款申请结果。
type UserRefundResult struct {
	RefundNo string `json:"refund_no"`
	// AutoApproved=true 表示已免审核自动退款（财务会再复核一次）。
	AutoApproved bool    `json:"auto_approved"`
	Amount       float64 `json:"amount"`
	Status       string  `json:"status"`
	// Hint 展示给客户的提示文案。
	Hint string `json:"hint"`
}

// OrderRefundHint 订单可退性提示（详情页据此决定是否展示「申请退款」入口）。
type OrderRefundHint struct {
	Refundable bool `json:"refundable"`
	// WindowHours 可退时限（小时）；0 = 不限时。
	WindowHours int `json:"window_hours"`
	// Deadline 可退截止时间（RFC3339）；不限时为空串。
	Deadline string `json:"deadline,omitempty"`
	// AutoApprove 命中免审核自动退款（退回余额 + 全额 + 在时限内）。
	AutoApprove bool `json:"auto_approve"`
	// ChannelAllowed 是否允许选原路退回（有渠道支付记录的订单才为 true）。
	ChannelAllowed bool `json:"channel_allowed"`
	// MaxAmount 可退金额上限（实付 - 已生效退款）。
	MaxAmount float64 `json:"max_amount"`
	// Reason 不可退原因（Refundable=false 时展示）。
	Reason string `json:"reason,omitempty"`
}
