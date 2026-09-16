package model

import "time"

// 用户详情聚合视图载体（非持久化）。
//
// 本文件的类型**一律不带 TableName()**：它们只作为显式 Table("orders") /
// Table("tickets") / Table("instances") 查询的扫描目标存在。历史上这里定义了
// UserOrder/UserTicket 并各自声明 TableName 指向 user_orders/user_tickets，
// 导致详情页的订单与工单永远读到 Phase 2 已退役的旧表（恒为 0 行）。
// 旧表退役见 migrations/013、014、049。

// UserInstanceBrief 实例摘要，字段对齐 instances 表的真实列。
type UserInstanceBrief struct {
	ID             uint64     `gorm:"column:id"`
	InstanceID     string     `gorm:"column:instance_id"`
	Name           string     `gorm:"column:name"`
	Region         string     `gorm:"column:region"`
	Zone           string     `gorm:"column:zone"`
	CPU            int64      `gorm:"column:cpu"`
	Memory         int64      `gorm:"column:memory"`
	Disk           int64      `gorm:"column:disk"`
	OS             string     `gorm:"column:os"`
	PublicIP       string     `gorm:"column:public_ip"`
	Status         string     `gorm:"column:status"`
	BillingMode    string     `gorm:"column:billing_mode"`
	LifecycleStage string     `gorm:"column:lifecycle_stage"`
	OrderID        *uint64    `gorm:"column:order_id"`
	SourceMode     string     `gorm:"column:source_mode"`
	// ExpireAt 允许为 NULL：未设置到期时间的实例必须显示「未设置」，
	// 而不是被 COALESCE 伪造成 1970-01-01。
	ExpireAt  *time.Time `gorm:"column:expire_at"`
	CreatedAt time.Time  `gorm:"column:created_at"`
}

// UserOrderBrief 订单摘要（权威表 orders）。
type UserOrderBrief struct {
	ID          uint64     `gorm:"column:id"`
	OrderNo     string     `gorm:"column:order_no"`
	ProductName string     `gorm:"column:product_name"`
	FinalAmount float64    `gorm:"column:final_amount"`
	Status      string     `gorm:"column:status"`
	PayMethod   string     `gorm:"column:pay_method"`
	RenewalID   uint64     `gorm:"column:renewal_id"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	PaidAt      *time.Time `gorm:"column:paid_at"`
}

// UserBillBrief 账单摘要（权威表 bills）。
type UserBillBrief struct {
	ID           uint64    `gorm:"column:id"`
	BillNo       string    `gorm:"column:bill_no"`
	BillingMonth string    `gorm:"column:period"`
	Amount       float64   `gorm:"column:total_amount"`
	BillType     string    `gorm:"column:bill_type"`
	Status       string    `gorm:"column:status"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

// UserTransactionBrief 资金流水摘要（权威表 wallet_transactions）。
type UserTransactionBrief struct {
	ID           uint64    `gorm:"column:id"`
	TxnNo        string    `gorm:"column:tx_no"`
	Type         string    `gorm:"column:type"`
	Direction    int64     `gorm:"column:direction"`
	Amount       float64   `gorm:"column:amount"`
	BalanceAfter float64   `gorm:"column:balance_after"`
	Remark       string    `gorm:"column:remark"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

// UserTicketBrief 工单摘要（权威表 tickets）。
type UserTicketBrief struct {
	ID        uint64    `gorm:"column:id"`
	TicketNo  string    `gorm:"column:ticket_no"`
	Title     string    `gorm:"column:title"`
	Category  string    `gorm:"column:category"`
	Priority  string    `gorm:"column:priority"`
	Status    string    `gorm:"column:status"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// UserDetailCounts 详情页各域计数，由单条聚合 SQL 取回，供 Tab 徽标与统计卡使用。
// 字段名与 SQL 别名一一对应。
type UserDetailCounts struct {
	InstanceCount      int64   `gorm:"column:instance_count"`
	RunningInstance    int64   `gorm:"column:running_instance_count"`
	OrderCount         int64   `gorm:"column:order_count"`
	OrderTotalAmount   float64 `gorm:"column:order_total_amount"`
	BillCount          int64   `gorm:"column:bill_count"`
	UnpaidBillCount    int64   `gorm:"column:unpaid_bill_count"`
	TransactionCount   int64   `gorm:"column:transaction_count"`
	TicketCount        int64   `gorm:"column:ticket_count"`
	OpenTicketCount    int64   `gorm:"column:open_ticket_count"`
	LoginCount         int64   `gorm:"column:login_count"`
	ActiveSessionCount int64   `gorm:"column:active_session_count"`
	RiskEventCount     int64   `gorm:"column:risk_event_count"`
	OperationLogCount  int64   `gorm:"column:operation_log_count"`
	VerificationCount  int64   `gorm:"column:verification_count"`
}

// UserRoleBrief 用户绑定的角色（user_roles ⋈ roles），详情页只读展示。
// 这是后台 RBAC 角色（roles.scope='admin'），与客户侧权限是两回事：
// 客户侧权限是固定枚举，落 sub_account_permissions。
type UserRoleBrief struct {
	ID    uint64 `gorm:"column:id"`
	Code  string `gorm:"column:code"`
	Name  string `gorm:"column:name"`
	Scope string `gorm:"column:scope"`
}
