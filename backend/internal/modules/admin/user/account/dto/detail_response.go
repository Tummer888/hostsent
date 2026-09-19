package dto

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
// 职责边界：本接口只负责「资料 + 各域计数」。各业务域的完整列表由既有的分页
// 接口承担（/orders、/tickets、/instances、/finance/bills、/finance/transactions、
// /security/login-logs …）。
//
// recent_instances / recent_orders / recent_bills / recent_transactions /
// recent_tickets 五个「近期摘要」数组**已于本次移除**（doc104 §3.3，F20）：
// 它们每段 8 条、从未被任何前端页面读取，却让详情页每次都要多打 5 条查询；
// 各 Tab 的列表本来就是实时分页查询，摘要是纯冗余。
//
// Permissions 是**客户侧权限码**（sub_account_permissions，固定枚举），
// RbacRoles 是用户被绑定的后台角色，两者语义不同，必须分开呈现。
//
// Degraded 记录采集失败的段名（如 ["summary"]）：前端据此在对应 Tab 显示
// 「数据暂不可用」，而不是让整页失败。
type UserDetailAggregateResponse struct {
	Profile     UserInfo          `json:"profile"`
	RbacRoles   []UserRoleBrief   `json:"rbac_roles"`
	Permissions []string          `json:"permissions"`
	Summary     UserDetailSummary `json:"summary"`
	Degraded    []string          `json:"degraded"`
}
