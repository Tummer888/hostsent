// Package dto 提供资金流水子域的数据传输结构。
package dto

// TransactionListQuery 资金流水分页查询
type TransactionListQuery struct {
	UserID    uint64 `form:"user_id" json:"user_id"`
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
	Type          string  `json:"type"`
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

// TransactionListResponse 资金流水列表响应
type TransactionListResponse struct {
	Items []TransactionInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
}
