package dto

import "time"

// UserInstanceBrief 实例摘要项。
// ExpireAt 为指针：未设置到期时间的实例必须原样传 null，前端显示「未设置」，
// 不能被伪造成 1970-01-01。
type UserInstanceBrief struct {
	ID             uint64     `json:"id"`
	InstanceID     string     `json:"instance_id"`
	Name           string     `json:"name"`
	Region         string     `json:"region"`
	Zone           string     `json:"zone"`
	CPU            int64      `json:"cpu"`
	Memory         int64      `json:"memory"`
	Disk           int64      `json:"disk"`
	OS             string     `json:"os"`
	PublicIP       string     `json:"public_ip"`
	Status         string     `json:"status"`
	BillingMode    string     `json:"billing_mode"`
	LifecycleStage string     `json:"lifecycle_stage"`
	OrderID        *uint64    `json:"order_id"`
	SourceMode     string     `json:"source_mode"`
	ExpireAt       *time.Time `json:"expire_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// UserOrderBrief 订单摘要项（权威表 orders）。
type UserOrderBrief struct {
	ID          uint64     `json:"id"`
	OrderNo     string     `json:"order_no"`
	ProductName string     `json:"product_name"`
	FinalAmount float64    `json:"final_amount"`
	Status      string     `json:"status"`
	PayMethod   string     `json:"pay_method"`
	RenewalID   uint64     `json:"renewal_id"`
	CreatedAt   time.Time  `json:"created_at"`
	PaidAt      *time.Time `json:"paid_at"`
}

// UserBillBrief 账单摘要项（权威表 bills）。
type UserBillBrief struct {
	ID           uint64    `json:"id"`
	BillNo       string    `json:"bill_no"`
	BillingMonth string    `json:"billing_month"`
	Amount       float64   `json:"amount"`
	BillType     string    `json:"bill_type"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserTransactionBrief 资金流水摘要项（权威表 wallet_transactions）。
type UserTransactionBrief struct {
	ID           uint64    `json:"id"`
	TxnNo        string    `json:"txn_no"`
	Type         string    `json:"type"`
	Direction    int64     `json:"direction"`
	Amount       float64   `json:"amount"`
	BalanceAfter float64   `json:"balance_after"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserTicketBrief 工单摘要项（权威表 tickets）。
type UserTicketBrief struct {
	ID        uint64    `json:"id"`
	TicketNo  string    `json:"ticket_no"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Priority  string    `json:"priority"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

// UserRoleBrief 用户绑定的后台角色（只读展示）。
type UserRoleBrief struct {
	ID    uint64 `json:"id"`
	Code  string `json:"code"`
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

// UserDetailSummary 详情页各域计数，供 Tab 徽标与统计卡使用。
type UserDetailSummary struct {
	InstanceCount      int64   `json:"instance_count"`
	RunningInstance    int64   `json:"running_instance_count"`
	OrderCount         int64   `json:"order_count"`
	OrderTotalAmount   float64 `json:"order_total_amount"`
	BillCount          int64   `json:"bill_count"`
	UnpaidBillCount    int64   `json:"unpaid_bill_count"`
	TransactionCount   int64   `json:"transaction_count"`
	TicketCount        int64   `json:"ticket_count"`
	OpenTicketCount    int64   `json:"open_ticket_count"`
	LoginCount         int64   `json:"login_count"`
	ActiveSessionCount int64   `json:"active_session_count"`
	RiskEventCount     int64   `json:"risk_event_count"`
	OperationLogCount  int64   `json:"operation_log_count"`
	VerificationCount  int64   `json:"verification_count"`
}

// UserDetailAggregateResponse 用户详情聚合响应。
//
// 职责边界：本接口只负责「资料 + 各域计数 + 近期若干条摘要」。各业务域的完整
// 列表由既有的分页接口承担（/orders、/tickets、/instances、/finance/bills、
// /finance/transactions、/security/login-logs …），避免单个用户的历史数据量
// 决定详情页的响应时间。
//
// Permissions 是**客户侧权限码**（sub_account_permissions，固定枚举），
// RbacRoles 是用户被绑定的后台角色，两者语义不同，必须分开呈现。
//
// Degraded 记录采集失败的段名（如 ["logs"]）：前端据此在对应 Tab 显示
// 「数据暂不可用」，而不是让整页失败。
type UserDetailAggregateResponse struct {
	Profile            UserInfo                `json:"profile"`
	RbacRoles          []UserRoleBrief         `json:"rbac_roles"`
	Permissions        []string                `json:"permissions"`
	Summary            UserDetailSummary       `json:"summary"`
	RecentInstances    []UserInstanceBrief     `json:"recent_instances"`
	RecentOrders       []UserOrderBrief        `json:"recent_orders"`
	RecentBills        []UserBillBrief         `json:"recent_bills"`
	RecentTransactions []UserTransactionBrief  `json:"recent_transactions"`
	RecentTickets      []UserTicketBrief       `json:"recent_tickets"`
	Degraded           []string                `json:"degraded"`
}
