package dto

// OrderListQuery 订单列表查询
type OrderListQuery struct {
	Keyword     string `form:"keyword" json:"keyword"`           // 订单号 / 产品名称
	UserKeyword string `form:"user_keyword" json:"user_keyword"` // 用户账号（用户名/邮箱/手机号）
	UserID      uint64 `form:"user_id" json:"user_id"`           // 用户 ID
	ProductID   uint64 `form:"product_id" json:"product_id"`     // 产品 ID
	Status      string `form:"status" json:"status"`             // 订单状态
	PayMethod   string `form:"pay_method" json:"pay_method"`     // 支付方式
	StartTime   string `form:"start_time" json:"start_time"`     // 开始时间（YYYY-MM-DD HH:MM:SS）
	EndTime     string `form:"end_time" json:"end_time"`         // 结束时间
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"page_size" json:"page_size"`
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
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
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
