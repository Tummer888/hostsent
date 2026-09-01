package dto

// WalletInfo 用户钱包信息
type WalletInfo struct {
	UserID       uint64  `json:"user_id"`
	Balance      float64 `json:"balance"`       // 可用余额
	Frozen       float64 `json:"frozen"`        // 冻结金额
	TotalIncome  float64 `json:"total_income"`  // 累计收入
	TotalExpense float64 `json:"total_expense"` // 累计支出
	Version      uint64  `json:"version"`
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

// RechargeInfo 充值单信息
type RechargeInfo struct {
	ID         uint64  `json:"id"`
	RechargeNo string  `json:"recharge_no"`
	UserID     uint64  `json:"user_id"`
	Amount     float64 `json:"amount"`
	Method     string  `json:"method"`
	Status     string  `json:"status"`
	ChannelTx  string  `json:"channel_tx"`
	PaidAt     string  `json:"paid_at"`
	Remark     string  `json:"remark"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// RechargeListResponse 充值单列表响应
type RechargeListResponse struct {
	Items []RechargeInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// WithdrawInfo 提现单信息
type WithdrawInfo struct {
	ID          uint64  `json:"id"`
	WithdrawNo  string  `json:"withdraw_no"`
	UserID      uint64  `json:"user_id"`
	Amount      float64 `json:"amount"`
	Channel     string  `json:"channel"`
	Account     string  `json:"account"`
	Status      string  `json:"status"`
	AuditBy     uint64  `json:"audit_by"`
	AuditByName string  `json:"audit_by_name"`
	AuditedAt   string  `json:"audited_at"`
	PaidAt      string  `json:"paid_at"`
	Remark      string  `json:"remark"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// WithdrawListResponse 提现单列表响应
type WithdrawListResponse struct {
	Items []WithdrawInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// BillInfo 账单信息
type BillInfo struct {
	ID           uint64  `json:"id"`
	BillNo       string  `json:"bill_no"`
	UserID       uint64  `json:"user_id"`
	Period       string  `json:"period"`
	TotalAmount  float64 `json:"total_amount"`
	RefundAmount float64 `json:"refund_amount"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// BillListResponse 账单列表响应
type BillListResponse struct {
	Items []BillInfo `json:"items"`
	Meta  ListMeta   `json:"meta"`
}

// ReconcileResponse 对账结果
type ReconcileResponse struct {
	Period        string  `json:"period"`         // 对账账期（空表示全量）
	IncomeTotal   float64 `json:"income_total"`   // 收入合计
	ExpenseTotal  float64 `json:"expense_total"`  // 支出合计
	TxCount       int64   `json:"tx_count"`       // 流水条数
	WalletBalance float64 `json:"wallet_balance"` // 钱包当前余额合计
	Diff          float64 `json:"diff"`           // 账实差异（应为 0）
	Status        string  `json:"status"`         // ok/suspicious
}
