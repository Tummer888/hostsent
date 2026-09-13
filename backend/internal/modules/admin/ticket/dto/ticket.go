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
	// ReviewStatus 复核状态筛选（S3）：pending / approved / rejected
	ReviewStatus string `form:"review_status" json:"review_status"`
	// DepartmentID 部门筛选（S2）：与数据范围取交集，不会越权放大可见范围
	DepartmentID uint64 `form:"department_id" json:"department_id"`
	// AdminID 当前登录员工 ID，工作台视图按此过滤（由 handler 注入，不接受前端传参）
	AdminID uint64 `form:"-" json:"-"`
	// UserIDs 账号家族 ID（主账号 + 子账号），用户端按归属过滤用（由 service 注入，P4-05）
	UserIDs []uint64 `form:"-" json:"-"`
	// —— S2 数据范围隔离（由 handler 按鉴权上下文注入，不信任前端传参）——
	// VisibleCategories 可见分类 code 列表：非空时仅返回这些分类的工单（超管不注入=全量）。
	VisibleCategories []string `form:"-" json:"-"`
	// OrAssignee 非 0 时与 VisibleCategories 取并集（客服：本部门分类 ∪ 指派给我的）。
	OrAssignee uint64 `form:"-" json:"-"`
	// VisibleDepartments 可见部门 ID 列表（S3 复核中心数据范围）：非空时仅返回这些部门的待复核回复。
	VisibleDepartments []uint64 `form:"-" json:"-"`
	// IgnoreCategoryRequired 供内部调用跳过分类前置条件（用户端不使用）。
	IgnoreCategoryRequired bool `form:"-" json:"-"`
	Page                   int  `form:"page" json:"page"`
	PageSize               int  `form:"page_size" json:"page_size"`
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
	// DepartmentID / DepartmentName 归属部门快照（S2）
	DepartmentID   uint64 `json:"department_id"`
	DepartmentName string `json:"department_name"`
	OrderID        uint64 `json:"order_id"`
	InstanceID     uint64 `json:"instance_id"`
	ReplyCount     int64  `json:"reply_count"` // 回复数
	// ReviewStatus 复核状态（S3）：pending 时列表高亮提示；空表示无需复核。
	ReviewStatus string `json:"review_status"`
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
	// IsInternal 内部备注（S2）：用户端不会收到该字段为 true 的回复
	IsInternal bool `json:"is_internal"`
	// ReviewStatus 复核状态（S3）：pending 待复核 / approved 已通过 / rejected 已驳回
	ReviewStatus string `json:"review_status"`
	// ReviewerID / ReviewerName 复核人（仅管理端有值）
	ReviewerID   uint64 `json:"reviewer_id,omitempty"`
	ReviewerName string `json:"reviewer_name,omitempty"`
	ReviewedAt   string `json:"reviewed_at,omitempty"`
	ReviewNote   string `json:"review_note,omitempty"`
	// Attachments 该回复携带的附件（S2）
	Attachments []TicketAttachmentInfo `json:"attachments"`
	CreatedAt   string                 `json:"created_at"`
}

// TicketAttachmentInfo 工单附件信息（S2）
type TicketAttachmentInfo struct {
	ID         uint64 `json:"id"`
	TicketID   uint64 `json:"ticket_id"`
	ReplyID    uint64 `json:"reply_id"`
	FileName   string `json:"file_name"`
	FileURL    string `json:"file_url"`
	FileSize   int64  `json:"file_size"`
	FileType   string `json:"file_type"`
	UploaderID uint64 `json:"uploader_id"`
	// UploaderName 上传者名称快照（管理端展示，用户端为空）
	UploaderName string `json:"uploader_name,omitempty"`
	// IsInternal 内部附件（S2）：用户端不可见
	IsInternal bool   `json:"is_internal"`
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
	// Attachments 工单主附件（未挂到具体回复的，S2）
	Attachments []TicketAttachmentInfo `json:"attachments"`
	// ReviewNote 工单级最近复核意见（S3，驳回原因）
	ReviewNote string `json:"review_note"`
	// ReviewerName 工单级最近复核人姓名（仅管理端填充）
	ReviewerName string `json:"reviewer_name,omitempty"`
	// ReviewedAt 工单级最近复核时间（S3）
	ReviewedAt string `json:"reviewed_at,omitempty"`
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
	// AttachmentIDs 提交时一并携带的附件 ID（S2）：先调上传接口拿 ID，再随建单提交。
	AttachmentIDs []uint64 `json:"attachment_ids"`
}

// TicketReplyRequest 发表回复
type TicketReplyRequest struct {
	Content string `json:"content" binding:"required"` // 回复内容
	// IsInternal 内部备注（S2）：需 ticket:internal_note 权限，无权限时服务端忽略该参数。
	IsInternal bool `json:"is_internal"`
	// AttachmentIDs 随回复携带的附件 ID（S2）
	AttachmentIDs []uint64 `json:"attachment_ids"`
}

// TicketReviewRequest 复核回复（S3）
type TicketReviewRequest struct {
	// Action approve / reject
	Action string `json:"action" binding:"required,oneof=approve reject"`
	// Note 复核意见；驳回时必填（服务端校验）
	Note string `json:"note"`
}

// AttachmentUploadRequest 附件上传的元信息（multipart 表单字段）
type AttachmentUploadRequest struct {
	// IsInternal 内部附件（S2）：随内部备注上传，用户端不可见。
	IsInternal bool `form:"is_internal"`
}

// UserCategoryInfo 用户端分类选项（S2）：附提交前置条件，前端据此动态渲染表单。
type UserCategoryInfo struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	// RequireRealname / RequireBinding 提交前置条件
	RequireRealname bool `json:"require_realname"`
	RequireBinding  bool `json:"require_binding"`
	// NeedReview 提交后管理员回复需复核（用户端仅作提示，不阻断提交）
	NeedReview bool `json:"need_review"`
}

// UserCategoryListResponse 用户端分类列表响应（附当前用户实名状态，减少一次请求）。
type UserCategoryListResponse struct {
	Items []UserCategoryInfo `json:"items"`
	// RealnameOK 当前用户是否已完成实名认证（require_realname 分类据此禁用提交）
	RealnameOK bool `json:"realname_ok"`
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
	// DefaultGroupID 历史裸客服组（deprecated，S2 起派单按 department_id）
	DefaultGroupID uint64 `json:"default_group_id"`
	// —— S2/S3 分类扩展 ——
	// DepartmentID 归属部门：决定派单候选与数据范围
	DepartmentID uint64 `json:"department_id"`
	// DepartmentName 只读：部门名称
	DepartmentName string `json:"department_name"`
	// RequireRealname / RequireBinding 提交前置条件
	RequireRealname bool `json:"require_realname"`
	RequireBinding  bool `json:"require_binding"`
	// NeedReview 管理员回复需双人复核（S3）
	NeedReview bool `json:"need_review"`
	// VisibleRoleCodes 限定可提交的用户角色（空=不限）
	VisibleRoleCodes []string `json:"visible_role_codes"`
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
	DefaultGroupID  uint64 `json:"default_group_id"`        // 历史裸客服组（deprecated）
	SLAHours        int    `json:"sla_hours"`               // 首次响应时限（小时）
	// —— S2/S3 分类扩展 ——
	DepartmentID     uint64   `json:"department_id"`
	RequireRealname  bool     `json:"require_realname"`
	RequireBinding   bool     `json:"require_binding"`
	NeedReview       bool     `json:"need_review"`
	VisibleRoleCodes []string `json:"visible_role_codes"`
}

// ReviewQueueItem 待复核回复队列项（S3 复核中心）
type ReviewQueueItem struct {
	ReplyID        uint64 `json:"reply_id"`
	TicketID       uint64 `json:"ticket_id"`
	TicketNo       string `json:"ticket_no"`
	Title          string `json:"title"`
	Category       string `json:"category"`
	CategoryName   string `json:"category_name"`
	DepartmentID   uint64 `json:"department_id"`
	DepartmentName string `json:"department_name"`
	// SenderID / SenderName 提交待复核回复的客服
	SenderID   uint64 `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"`
	// WaitingSeconds 已等待复核秒数，供前端展示等待时长
	WaitingSeconds int64  `json:"waiting_seconds"`
	CreatedAt      string `json:"created_at"`
}

// ReviewQueueResponse 待复核队列响应
type ReviewQueueResponse struct {
	Items []ReviewQueueItem `json:"items"`
	Meta  ListMeta          `json:"meta"`
}

// ReviewRequest 复核入参（别名，供 handler 语义化引用）
type ReviewRequest = TicketReviewRequest

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
