package dto

// TicketListQuery 工单列表查询（管理端与用户端共用，用户端强制注入 UserID）
type TicketListQuery struct {
	Keyword     string `form:"keyword" json:"keyword"`           // 工单号 / 标题
	UserKeyword string `form:"user_keyword" json:"user_keyword"` // 用户账号（用户名/邮箱/手机号）
	UserID      uint64 `form:"user_id" json:"user_id"`           // 用户 ID
	Category    string `form:"category" json:"category"`         // 分类编码
	Priority    string `form:"priority" json:"priority"`         // 优先级
	Status      string `form:"status" json:"status"`             // 工单状态
	AssignedTo  uint64 `form:"assigned_to" json:"assigned_to"`   // 处理人 ID
	StartTime   string `form:"start_time" json:"start_time"`     // 开始时间（YYYY-MM-DD HH:MM:SS）
	EndTime     string `form:"end_time" json:"end_time"`         // 结束时间
	// View 工单工作台视图（P2-07）：my_todo / unassigned / involved / sla_breached
	View string `form:"view" json:"view"`
	// AdminID 当前登录员工 ID，工作台视图按此过滤（由 handler 注入，不接受前端传参）
	AdminID uint64 `form:"-" json:"-"`
	// UserIDs 账号家族 ID（主账号 + 子账号），用户端按归属过滤用（由 service 注入，P4-05）
	UserIDs  []uint64 `form:"-" json:"-"`
	Page     int      `form:"page" json:"page"`
	PageSize int      `form:"page_size" json:"page_size"`
}

// TicketInfo 工单列表项
type TicketInfo struct {
	ID           uint64 `json:"id"`
	TicketNo     string `json:"ticket_no"`
	UserID       uint64 `json:"user_id"`
	Username     string `json:"username"` // 提交用户账号（列表展示用）
	Title        string `json:"title"`
	Category     string `json:"category"`      // 分类编码
	CategoryName string `json:"category_name"` // 分类名称
	Priority     string `json:"priority"`
	Status       string `json:"status"`
	AssignedTo   uint64 `json:"assigned_to"`
	AssignedName string `json:"assigned_name"` // 处理人名称
	OrderID      uint64 `json:"order_id"`
	InstanceID   uint64 `json:"instance_id"`
	ReplyCount   int64  `json:"reply_count"` // 回复数
	// SLAHours 分类的首次响应时限（0 表示未启用）
	SLAHours int `json:"sla_hours"`
	// SLABreached 是否已超时未首次响应（P2-05）
	SLABreached bool   `json:"sla_breached"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// TicketListResponse 工单列表响应
type TicketListResponse struct {
	Items []TicketInfo `json:"items"`
	Meta  ListMeta     `json:"meta"`
}

// TicketReplyInfo 工单回复信息
type TicketReplyInfo struct {
	ID         uint64 `json:"id"`
	TicketID   uint64 `json:"ticket_id"`
	SenderType string `json:"sender_type"` // user / admin
	SenderID   uint64 `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

// TicketDetail 工单详情（含对话回复记录）
type TicketDetail struct {
	TicketInfo
	Description  string            `json:"description"` // 首条描述
	FirstReplyAt string            `json:"first_reply_at"`
	ResolvedAt   string            `json:"resolved_at"`
	ClosedAt     string            `json:"closed_at"`
	Replies      []TicketReplyInfo `json:"replies"`
	Logs         []TicketLogInfo   `json:"logs"` // 操作日志时间线（P2-02）
}

// TicketLogInfo 工单操作日志项
type TicketLogInfo struct {
	ID           uint64 `json:"id"`
	TicketID     uint64 `json:"ticket_id"`
	OperatorID   uint64 `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	Action       string `json:"action"` // create/assign/claim/transfer/reply/status/close/cancel
	FromValue    string `json:"from_value"`
	ToValue      string `json:"to_value"`
	Note         string `json:"note"`
	CreatedAt    string `json:"created_at"`
}

// TicketCreateRequest 用户提交工单
type TicketCreateRequest struct {
	Title       string `json:"title" binding:"required"`    // 标题
	Description string `json:"description"`                 // 问题描述
	Category    string `json:"category" binding:"required"` // 分类编码
	Priority    string `json:"priority"`                    // 优先级（默认 medium）
	OrderID     uint64 `json:"order_id"`                    // 关联订单（可选）
	InstanceID  uint64 `json:"instance_id"`                 // 关联实例（可选）
}

// TicketReplyRequest 发表回复
type TicketReplyRequest struct {
	Content string `json:"content" binding:"required"` // 回复内容
}

// TicketAssignRequest 分配工单
type TicketAssignRequest struct {
	AssignedTo uint64 `json:"assigned_to" binding:"required"` // 管理员 ID
}

// TicketTransferRequest 转派工单（P2-03）
type TicketTransferRequest struct {
	ToID uint64 `json:"to_id" binding:"required"` // 目标管理员 ID
	Note string `json:"note"`                     // 转派说明
}

// TicketStatusRequest 更新工单状态
type TicketStatusRequest struct {
	Status string `json:"status" binding:"required"` // 目标状态
}

// CategoryInfo 工单分类信息
type CategoryInfo struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	Status      string `json:"status"`
	// DefaultRoleCode 自动派单目标角色（P2-03）
	DefaultRoleCode string `json:"default_role_code"`
	// DefaultGroupID 自动派单目标客服组
	DefaultGroupID uint64 `json:"default_group_id"`
	// SLAHours 首次响应时限（小时，0 表示不启用）
	SLAHours  int    `json:"sla_hours"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CategorySaveRequest 创建/更新分类请求
type CategorySaveRequest struct {
	Name            string `json:"name" binding:"required"` // 显示名称
	Code            string `json:"code" binding:"required"` // 分类编码
	Description     string `json:"description"`             // 描述
	SortOrder       int    `json:"sort_order"`              // 排序值
	Status          string `json:"status"`                  // active / disabled（默认 active）
	DefaultRoleCode string `json:"default_role_code"`       // 自动派单目标角色 code
	DefaultGroupID  uint64 `json:"default_group_id"`        // 自动派单目标客服组
	SLAHours        int    `json:"sla_hours"`               // 首次响应时限（小时）
}

// TicketStatusStat 工单状态分布统计项
type TicketStatusStat struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// TicketTrendPoint 工单趋势点（近 N 日工单量）
type TicketTrendPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// TicketCategoryStat 分类工单分布项
type TicketCategoryStat struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// TicketStatsResponse 工单统计概览
type TicketStatsResponse struct {
	Total                int64                `json:"total"`                   // 工单总量
	Open                 int64                `json:"open"`                    // 待处理
	InProgress           int64                `json:"in_progress"`             // 处理中
	WaitingUser          int64                `json:"waiting_user"`            // 等待用户
	Resolved             int64                `json:"resolved"`                // 已解决
	Closed               int64                `json:"closed"`                  // 已关闭（含已取消）
	AvgFirstReplySeconds int64                `json:"avg_first_reply_seconds"` // 平均首次响应时长（秒）
	StatusDistribution   []TicketStatusStat   `json:"status_distribution"`     // 状态分布
	Trend                []TicketTrendPoint   `json:"trend"`                   // 近 7 日工单量趋势
	CategoryDistribution []TicketCategoryStat `json:"category_distribution"`   // 分类分布
}
