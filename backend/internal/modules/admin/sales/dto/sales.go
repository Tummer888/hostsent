// Package dto 提供销售归属与提成子域的请求/响应结构（doc86 §2.3）。
package dto

// ListMeta 分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// Scope 数据范围：由 handler 从鉴权快照推导，前端传参一律忽略（doc86 §2.4）。
//   - All=true：超管/看全部；
//   - AdminID>0：仅该销售本人；
//   - DepartmentID>0：仅该部门。
type Scope struct {
	All          bool   `json:"-"`
	AdminID      uint64 `json:"-"`
	DepartmentID uint64 `json:"-"`
}

// —— 客户归属 ——

// CustomerQuery 客户归属列表查询。
type CustomerQuery struct {
	Keyword      string `form:"keyword"`       // 用户名/邮箱/手机
	AdminID      uint64 `form:"admin_id"`      // 指定销售（主管视角）
	DepartmentID uint64 `form:"department_id"` // 指定部门（主管视角）
	// Unassigned=1 切到未归属池；status 仅对已归属生效。
	Unassigned int    `form:"unassigned"`
	Status     string `form:"status"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// CustomerInfo 归属客户行。
type CustomerInfo struct {
	RelationID     uint64 `json:"relation_id"`
	UserID         uint64 `json:"user_id"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	AdminID        uint64 `json:"admin_id"`
	AdminName      string `json:"admin_name"`
	AdminRealName  string `json:"admin_real_name"`
	DepartmentID   uint64 `json:"department_id"`
	DepartmentName string `json:"department_name"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	// ProtectUntil 非空且晚于当前时间时不可改派（前端据此禁用「分配」按钮）。
	ProtectUntil string  `json:"protect_until"`
	Protected    bool    `json:"protected"`
	EffectiveAt  string  `json:"effective_at"`
	ReleasedAt   string  `json:"released_at"`
	TotalConsume float64 `json:"total_consume"`
	RegisteredAt string  `json:"registered_at"`
}

// CustomerListResponse 归属客户分页响应。
type CustomerListResponse struct {
	Items []CustomerInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// UnassignedInfo 未归属客户行。
type UnassignedInfo struct {
	UserID       uint64  `json:"user_id"`
	Username     string  `json:"username"`
	Email        string  `json:"email"`
	Phone        string  `json:"phone"`
	TotalConsume float64 `json:"total_consume"`
	RegisteredAt string  `json:"registered_at"`
}

// UnassignedListResponse 未归属客户池分页响应。
type UnassignedListResponse struct {
	Items []UnassignedInfo `json:"items"`
	Meta  ListMeta         `json:"meta"`
}

// AssignRequest 分配/变更客户归属。
type AssignRequest struct {
	UserID  uint64 `json:"user_id" binding:"required"`
	AdminID uint64 `json:"admin_id" binding:"required"`
	Reason  string `json:"reason" binding:"required"`
}

// ReleaseRequest 释放客户归属。
type ReleaseRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Reason string `json:"reason"`
}

// RelationInfo 归属变更历史行。
type RelationInfo struct {
	ID             uint64 `json:"id"`
	AdminID        uint64 `json:"admin_id"`
	AdminName      string `json:"admin_name"`
	AdminRealName  string `json:"admin_real_name"`
	DepartmentName string `json:"department_name"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	OperatorID     uint64 `json:"operator_id"`
	ProtectUntil   string `json:"protect_until"`
	EffectiveAt    string `json:"effective_at"`
	ReleasedAt     string `json:"released_at"`
	CreatedAt      string `json:"created_at"`
}

// RelationListResponse 归属变更历史响应。
type RelationListResponse struct {
	Items []RelationInfo `json:"items"`
}

// SalesCandidateInfo 可分配销售。
type SalesCandidateInfo struct {
	AdminID         uint64 `json:"admin_id"`
	Username        string `json:"username"`
	RealName        string `json:"real_name"`
	DepartmentID    uint64 `json:"department_id"`
	DepartmentName  string `json:"department_name"`
	ActiveCustomers int64  `json:"active_customers"`
}

// SalesCandidateListResponse 可分配销售响应。
type SalesCandidateListResponse struct {
	Items []SalesCandidateInfo `json:"items"`
}

// —— 提成台账 ——

// CommissionQuery 提成台账查询。
type CommissionQuery struct {
	AdminID      uint64 `form:"admin_id"`
	DepartmentID uint64 `form:"department_id"`
	Type         string `form:"type"`
	OrderNo      string `form:"order_no"`
	CustomerID   uint64 `form:"customer_id"`
	StartAt      string `form:"start_at"` // YYYY-MM-DD
	EndAt        string `form:"end_at"`   // YYYY-MM-DD（含当天）
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
}

// CommissionInfo 提成台账行。
type CommissionInfo struct {
	ID             uint64  `json:"id"`
	TxNo           string  `json:"tx_no"`
	AdminID        uint64  `json:"admin_id"`
	AdminName      string  `json:"admin_name"`
	AdminRealName  string  `json:"admin_real_name"`
	DepartmentName string  `json:"department_name"`
	Type           string  `json:"type"`
	TypeLabel      string  `json:"type_label"`
	Direction      int     `json:"direction"`
	Amount         float64 `json:"amount"`
	BalanceAfter   float64 `json:"balance_after"`
	OrderID        uint64  `json:"order_id"`
	OrderNo        string  `json:"order_no"`
	CustomerUserID uint64  `json:"customer_user_id"`
	CustomerName   string  `json:"customer_name"`
	ReleaseAt      string  `json:"release_at"`
	Remark         string  `json:"remark"`
	CreatedAt      string  `json:"created_at"`
}

// CommissionListResponse 提成台账分页响应。
type CommissionListResponse struct {
	Items []CommissionInfo `json:"items"`
	Meta  ListMeta         `json:"meta"`
}

// CommissionSummary 提成账户概览（销售自助页顶部统计卡）。
type CommissionSummary struct {
	AdminID         uint64  `json:"admin_id"`
	AdminName       string  `json:"admin_name"`
	AdminRealName   string  `json:"admin_real_name"`
	DepartmentName  string  `json:"department_name"`
	Enabled         bool    `json:"enabled"`
	Balance         float64 `json:"balance"`          // 可用（已过解冻期）
	PendingRelease  float64 `json:"pending_release"`  // 解冻中
	Frozen          float64 `json:"frozen"`           // 提现冻结
	TotalIncome     float64 `json:"total_income"`     // 累计提成
	TotalOut        float64 `json:"total_out"`        // 累计出账
	PendingWithdraw float64 `json:"pending_withdraw"` // 待审核提现额
	// Withdrawable 可提现金额：可用余额与 0 取大（欠款时不可提现）。
	Withdrawable    float64 `json:"withdrawable"`
	Debt            bool    `json:"debt"` // 可用余额为负（欠款）
	MinWithdraw     float64 `json:"min_withdraw"`
	ReleaseDays     int     `json:"release_days"`
	FirstOrderRate  float64 `json:"first_order_rate"`
	SubsequentRate  float64 `json:"subsequent_rate"`
	RenewalRate     float64 `json:"renewal_rate"`
	ActiveCustomers int64   `json:"active_customers"`
}

// —— 提成提现 ——

// WithdrawApplyRequest 销售自助申请提现。
type WithdrawApplyRequest struct {
	Amount      float64 `json:"amount" binding:"required"`
	Channel     string  `json:"channel"`
	Account     string  `json:"account"`
	AccountName string  `json:"account_name"`
	BankName    string  `json:"bank_name"`
	Remark      string  `json:"remark"`
}

// WithdrawAuditRequest 提现审核（通过/驳回）。
type WithdrawAuditRequest struct {
	Remark string `json:"remark"`
}

// WithdrawPayRequest 登记打款。
type WithdrawPayRequest struct {
	ChannelTx  string `json:"channel_tx"`
	ReceiptURL string `json:"receipt_url"`
	Remark     string `json:"remark"`
}

// WithdrawalQuery 提现单查询。
type WithdrawalQuery struct {
	AdminID      uint64 `form:"admin_id"`
	DepartmentID uint64 `form:"department_id"`
	Status       string `form:"status"`
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
}

// WithdrawalInfo 提现单行。Account 已脱敏。
type WithdrawalInfo struct {
	ID             uint64  `json:"id"`
	WithdrawNo     string  `json:"withdraw_no"`
	AdminID        uint64  `json:"admin_id"`
	AdminName      string  `json:"admin_name"`
	AdminRealName  string  `json:"admin_real_name"`
	DepartmentName string  `json:"department_name"`
	Amount         float64 `json:"amount"`
	Channel        string  `json:"channel"`
	Account        string  `json:"account"`
	AccountName    string  `json:"account_name"`
	BankName       string  `json:"bank_name"`
	PayoutMode     string  `json:"payout_mode"`
	PayoutNo       string  `json:"payout_no"`
	Status         string  `json:"status"`
	StatusLabel    string  `json:"status_label"`
	AuditBy        uint64  `json:"audit_by"`
	AuditByName    string  `json:"audit_by_name"`
	AuditedAt      string  `json:"audited_at"`
	PaidAt         string  `json:"paid_at"`
	Remark         string  `json:"remark"`
	CreatedAt      string  `json:"created_at"`
}

// WithdrawalListResponse 提现单分页响应。
type WithdrawalListResponse struct {
	Items []WithdrawalInfo `json:"items"`
	Meta  ListMeta         `json:"meta"`
}

// —— 业绩与排行 ——

// RankingQuery 业绩排行查询。
type RankingQuery struct {
	Period       string `form:"period"`        // YYYY-MM，空表示当月
	DepartmentID uint64 `form:"department_id"` // 0 表示不限
	// SortBy amount/orders，默认 amount。
	SortBy string `form:"sort_by"`
}

// RankingRow 排行行（含目标与达成率）。
type RankingRow struct {
	Rank           int     `json:"rank"`
	AdminID        uint64  `json:"admin_id"`
	AdminName      string  `json:"admin_name"`
	AdminRealName  string  `json:"admin_real_name"`
	DepartmentID   uint64  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	Amount         float64 `json:"amount"`
	Orders         int64   `json:"orders"`
	TargetAmount   float64 `json:"target_amount"`
	TargetOrders   int     `json:"target_orders"`
	// Achievement 达成率（amount/target_amount），无目标时为 0。
	Achievement float64 `json:"achievement"`
}

// RankingResponse 业绩排行响应。
type RankingResponse struct {
	Period string       `json:"period"`
	Items  []RankingRow `json:"items"`
}

// TargetUpsertRequest 目标下发（按行 upsert）。
type TargetUpsertRequest struct {
	Period       string  `json:"period" binding:"required"`
	Scope        string  `json:"scope"`
	AdminID      uint64  `json:"admin_id"`
	DepartmentID uint64  `json:"department_id"`
	TargetAmount float64 `json:"target_amount"`
	TargetOrders int     `json:"target_orders"`
}

// TargetInfo 目标行（含实时达成情况）。
type TargetInfo struct {
	ID             uint64  `json:"id"`
	Period         string  `json:"period"`
	Scope          string  `json:"scope"`
	AdminID        uint64  `json:"admin_id"`
	AdminName      string  `json:"admin_name"`
	AdminRealName  string  `json:"admin_real_name"`
	DepartmentID   uint64  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	TargetAmount   float64 `json:"target_amount"`
	TargetOrders   int     `json:"target_orders"`
	// ActualAmount/ActualOrders 实时达成值（销售个人目标取个人成交，部门目标取部门合计）。
	ActualAmount float64 `json:"actual_amount"`
	ActualOrders int64   `json:"actual_orders"`
	Achievement  float64 `json:"achievement"`
}

// TargetListResponse 目标列表响应。
type TargetListResponse struct {
	Period string       `json:"period"`
	Items  []TargetInfo `json:"items"`
}

// MyPerformance 我的业绩（销售自助）。
type MyPerformance struct {
	Period         string  `json:"period"`
	AdminID        uint64  `json:"admin_id"`
	AdminName      string  `json:"admin_name"`
	DepartmentName string  `json:"department_name"`
	Amount         float64 `json:"amount"`
	Orders         int64   `json:"orders"`
	TargetAmount   float64 `json:"target_amount"`
	TargetOrders   int     `json:"target_orders"`
	Achievement    float64 `json:"achievement"`
	// Rank 在本部门内的销售额名次（1 起）；0 表示本周期无名次（无成交或未参与排行）。
	Rank              int   `json:"rank"`
	DeptMembers       int   `json:"dept_members"`
	ActiveCustomers   int64 `json:"active_customers"`
	MonthNewCustomers int64 `json:"month_new_customers"`
}
