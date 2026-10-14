package dto

// OrderListQuery 订单列表查询
type OrderListQuery struct {
	Keyword     string `form:"keyword" json:"keyword"`           // 订单号 / 产品名称
	UserKeyword string `form:"user_keyword" json:"user_keyword"` // 用户账号（用户名/邮箱/手机号）
	UserID      uint64 `form:"user_id" json:"user_id"`           // 用户 ID
	ProductID   uint64 `form:"product_id" json:"product_id"`     // 产品 ID
	Status      string `form:"status" json:"status"`             // 订单状态
	PayMethod   string `form:"pay_method" json:"pay_method"`     // 支付方式
	// PaymentNo 支付单号（doc36 §3.1）：关联支付中心 payment_orders 反查，支持部分匹配。
	PaymentNo string `form:"payment_no" json:"payment_no"`
	// ChannelTx 渠道流水号（doc36 §3.1）：第三方交易号，对账排障用。
	ChannelTx string `form:"channel_tx" json:"channel_tx"`
	// OrderType 订单类型（doc36 §3.4）：consume=普通购买 renewal=续费；续费由 renewal_id>0 判定。
	OrderType string `form:"order_type" json:"order_type"`
	// VerifyState 核验状态筛选（运营人工核验开通结果）：pending=待核验（服务中未核验）
	// verified=已核验；空=不筛选。纯数据口径，不依赖 order_verify_required 开关。
	VerifyState string `form:"verify_state" json:"verify_state"`
	StartTime   string `form:"start_time" json:"start_time"` // 开始时间（YYYY-MM-DD HH:MM:SS）
	EndTime     string `form:"end_time" json:"end_time"`     // 结束时间
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"page_size" json:"page_size"`
}

// OrderItemInfo 订单项信息
type OrderItemInfo struct {
	ID          uint64 `json:"id"`
	OrderID     uint64 `json:"order_id"`
	ProductID   uint64 `json:"product_id"`
	ProductName string `json:"product_name"`
	SpecCode    string `json:"spec_code"`
	Specs       string `json:"specs"`
	// Cycle 行级计费周期快照（订单明细卡片展示用）。
	Cycle     string  `json:"cycle"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Amount    float64 `json:"amount"`
	CreatedAt string  `json:"created_at"`
}

// OrderInfo 订单列表项
type OrderInfo struct {
	ID          uint64  `json:"id"`
	OrderNo     string  `json:"order_no"`
	UserID      uint64  `json:"user_id"`
	ProductID   uint64  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Specs       string  `json:"specs"`
	Quantity    int     `json:"quantity"`
	PriceModel  string  `json:"price_model"`
	TotalAmount float64 `json:"total_amount"`
	PaidAmount  float64 `json:"paid_amount"`
	Status      string  `json:"status"`
	PayMethod   string  `json:"pay_method"`
	PayTime     string  `json:"pay_time"`
	// ActivatedAt 开通成功时间（退款时限起算点）；未开通用空串。
	ActivatedAt string `json:"activated_at"`
	ExpireTime  string `json:"expire_time"`
	Remark      string `json:"remark"`
	// 计费周期与续费标记（doc36 §3.4）：cycle 为计费周期快照，renewal_id>0 即续费订单。
	Cycle     string `json:"cycle"`
	RenewalID uint64 `json:"renewal_id"`
	// 算价快照（P5-01）：原价 / 优惠 / 实付，便于核对折扣来源。
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	DiscountSource string  `json:"discount_source"`
	// 支付单号与渠道流水号（doc36 §3.1）：由支付中心反查，列表与详情均可直接检索。
	PaymentNo string `json:"payment_no"`
	ChannelTx string `json:"channel_tx"`
	// 开通履约状态（provision_tasks）：pending/running/success/failed/manual。
	// manual 表示"重试也不会变好的上游错误，需人工处理"（如上游余额不足充值后重试），
	// 失败原因随 provision_error 返回 —— 后台订单列表是运营发现开通卡壳的入口。
	ProvisionStatus   string `json:"provision_status,omitempty"`
	ProvisionError    string `json:"provision_error,omitempty"`
	ProvisionAttempts int    `json:"provision_attempts,omitempty"`
	// 核验标记（运营人工核对开通结果）：独立于状态机，仅记录 + 展示。
	// VerifiedAt 为空即未核验；VerifyRequired 表示当前是否开启核验要求，
	// VerifyState 为读取期推导结果：pending（待核验）/ verified / ""（不涉及）。
	VerifiedAt     string `json:"verified_at,omitempty"`
	VerifiedBy     uint64 `json:"verified_by,omitempty"`
	VerifiedByName string `json:"verified_by_name,omitempty"`
	VerifyRemark   string `json:"verify_remark,omitempty"`
	VerifyRequired bool   `json:"verify_required"`
	VerifyState    string `json:"verify_state,omitempty"`
	// 关联实例（装配层注入端口批量回填）：InstanceID>0 时可跳转实例详情；
	// InstancePending 表示该实例仍是「待开通」占位行（未转正）。
	InstanceID      uint64 `json:"instance_id,omitempty"`
	InstanceNo      string `json:"instance_no,omitempty"`
	InstancePending bool   `json:"instance_pending,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// OrderListResponse 订单列表响应
type OrderListResponse struct {
	Items []OrderInfo `json:"items"`
	Meta  ListMeta    `json:"meta"`
}

// OrderDetail 订单详情（含明细项与退款记录）
type OrderDetail struct {
	OrderInfo
	Items   []OrderItemInfo `json:"items"`
	Refunds []RefundInfo    `json:"refunds"`
}

// OrderRemarkUpdateRequest 更新订单备注
type OrderRemarkUpdateRequest struct {
	Remark string `json:"remark"`
}

// OrderVerifyRequest 订单核验（运营人工核对开通结果，仅标记不流转状态）
type OrderVerifyRequest struct {
	// Remark 核验备注（可选）：记录核对结论，如"配置与订单一致"。
	Remark string `json:"remark"`
}

// BatchVerifyRequest 批量核验请求：ids + 统一核验备注。
type BatchVerifyRequest struct {
	IDs    []uint64 `json:"ids" binding:"required,min=1"`
	Remark string   `json:"remark"`
}

// BatchRefundAuditRequest 批量退款审核请求：ids + 统一审核备注。
type BatchRefundAuditRequest struct {
	IDs    []uint64 `json:"ids" binding:"required,min=1"`
	Remark string   `json:"remark"`
}

// VerifyState 取值（读取期推导，不落库）
const (
	VerifyStatePending  = "pending"  // 待核验：开通成功（服务中）且未核验
	VerifyStateVerified = "verified" // 已核验
)

// BatchIDsRequest 批量操作请求：ids 为订单/退款单主键集合。
// 上限 100 与用户批量注销口径一致（逐条复用单条逻辑，防止一次请求打爆上游）。
type BatchIDsRequest struct {
	IDs []uint64 `json:"ids" binding:"required,min=1"`
}

// BatchItemResult 批量操作单条结果：success=已执行；skipped=状态不允许等可预期跳过；
// failed=执行出错。调用方按 status 分类展示，失败项带原因。
type BatchItemResult struct {
	ID      uint64 `json:"id"`
	Status  string `json:"status"` // success / skipped / failed
	Message string `json:"message,omitempty"`
}

// BatchActionResponse 批量操作响应：逐条结果 + 分类计数。
type BatchActionResponse struct {
	Items     []BatchItemResult `json:"items"`
	Succeeded int               `json:"succeeded"`
	Skipped   int               `json:"skipped"`
	Failed    int               `json:"failed"`
}

// BatchStatus 取值
const (
	BatchStatusSuccess = "success"
	BatchStatusSkipped = "skipped"
	BatchStatusFailed  = "failed"
)

// RefundCreateRequest 创建退款单
type RefundCreateRequest struct {
	Amount float64 `json:"amount" binding:"required"`
	Reason string  `json:"reason"`
	// RefundMode 退款去向（doc36 §3.2）：balance=退回余额（默认，消费口径不变）；
	// channel=原路退回支付来源（收入口径需扣减本金与渠道扣点）。
	RefundMode string `json:"refund_mode"`
	// FeeAmount 渠道扣点（仅 channel 模式有意义）：原路退回时渠道不退还的手续费。
	FeeAmount float64 `json:"fee_amount"`
}

// OrderStatusStat 订单状态分布统计项
type OrderStatusStat struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// OrderTrendPoint 订单趋势点（近 N 日销售额 / 订单量）
type OrderTrendPoint struct {
	Date   string  `json:"date"`
	Sales  float64 `json:"sales"`
	Orders int64   `json:"orders"`
}

// OrderStatsResponse 订单统计概览
type OrderStatsResponse struct {
	TodaySales         float64           `json:"today_sales"`         // 今日销售额
	TodayOrders        int64             `json:"today_orders"`        // 今日订单量
	AvgOrderValue      float64           `json:"avg_order_value"`     // 客单价（总销售额/总订单量）
	RefundRate         float64           `json:"refund_rate"`         // 退款率（退款金额/销售额，百分比）
	StatusDistribution []OrderStatusStat `json:"status_distribution"` // 订单状态分布
	Trend              []OrderTrendPoint `json:"trend"`               // 近 7 日趋势
}
