package model

// UserRoleBrief 用户绑定的角色（user_roles ⋈ roles），只读展示用。
//
// 改造前只读 user_roles 不过滤 scope，客户账号一旦被跨域绑上后台角色（如
// super_admin），详情页就会把它显示成「超级管理员」。读侧现在限定
// roles.scope='user'（见 userDetailRepository.ListRbacRolesByUserID），
// 本结构保留 Scope 字段用于展示时标注。
type UserRoleBrief struct {
	ID    uint64 `gorm:"column:id"`
	Code  string `gorm:"column:code"`
	Name  string `gorm:"column:name"`
	Scope string `gorm:"column:scope"`
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
