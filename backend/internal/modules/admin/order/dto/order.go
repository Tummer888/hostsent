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
	StartTime string `form:"start_time" json:"start_time"` // 开始时间（YYYY-MM-DD HH:MM:SS）
	EndTime   string `form:"end_time" json:"end_time"`     // 结束时间
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// OrderItemInfo 订单项信息
type OrderItemInfo struct {
	ID          uint64  `json:"id"`
	OrderID     uint64  `json:"order_id"`
	ProductID   uint64  `json:"product_id"`
	ProductName string  `json:"product_name"`
	SpecCode    string  `json:"spec_code"`
	Specs       string  `json:"specs"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
	Amount      float64 `json:"amount"`
	CreatedAt   string  `json:"created_at"`
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
	ExpireTime  string  `json:"expire_time"`
	Remark      string  `json:"remark"`
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
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
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
