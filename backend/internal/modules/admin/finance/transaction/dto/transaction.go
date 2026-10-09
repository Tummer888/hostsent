// Package dto 提供资金流水子域的数据传输结构。
package dto

// TransactionListQuery 资金流水分页查询
type TransactionListQuery struct {
	UserID uint64 `form:"user_id" json:"user_id"`
	// Keyword 关键词：模糊匹配流水号 / 订单号 / 关联单号 / 用户名。
	// 「查某个用户的流水」不必先知道 user_id，这是财务对账最常用的入口。
	Keyword   string `form:"keyword" json:"keyword"`
	Type      string `form:"type" json:"type"`           // 流水类型
	Direction int    `form:"direction" json:"direction"` // 1=收入 -1=支出
	StartTime string `form:"start_time" json:"start_time"`
	EndTime   string `form:"end_time" json:"end_time"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// TransactionInfo 资金流水信息
type TransactionInfo struct {
	ID            uint64  `json:"id"`
	TxNo          string  `json:"tx_no"`
	UserID        uint64  `json:"user_id"`
	Username      string  `json:"username"` // 用户展示名（对账时不必再按 ID 反查）
	Type          string  `json:"type"`
	BizType       string  `json:"biz_type"`  // 业务标识（幂等键，排查重复记账用）
	Direction     int     `json:"direction"` // 1=收入 -1=支出
	Amount        float64 `json:"amount"`
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	OrderID       uint64  `json:"order_id"`
	OrderNo       string  `json:"order_no"`
	RefNo         string  `json:"ref_no"`
	Remark        string  `json:"remark"`
	OperatorID    uint64  `json:"operator_id"`
	CreatedAt     string  `json:"created_at"`
}

// TransactionSummary 当前筛选条件下的汇总（全量、非当前页）：
// 资金口径收入/支出/净额与笔数，另计冻结/解冻内部划转笔数。
type TransactionSummary struct {
	IncomeTotal   float64 `json:"income_total"`
	ExpenseTotal  float64 `json:"expense_total"`
	NetTotal      float64 `json:"net_total"`
	TxCount       int64   `json:"tx_count"`
	InternalCount int64   `json:"internal_count"`
}

// TransactionListResponse 资金流水列表响应
type TransactionListResponse struct {
	Items   []TransactionInfo  `json:"items"`
	Meta    ListMeta           `json:"meta"`
	Summary TransactionSummary `json:"summary"`
}
