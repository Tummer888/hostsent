package dto

// BroadcastTarget 群发目标定义。
type BroadcastTarget struct {
	Mode        string           `json:"mode" binding:"required,oneof=all group users filter"`
	UserGroupID uint64           `json:"user_group_id"` // mode=group
	UserIDs     []uint64         `json:"user_ids"`      // mode=users（多选，上限 500）
	Filter      *BroadcastFilter `json:"filter"`        // mode=filter
}

// BroadcastFilter 条件筛选群发目标。
type BroadcastFilter struct {
	RegisteredAfter  string `json:"registered_after"`
	RegisteredBefore string `json:"registered_before"`
	Tier             string `json:"tier"`
	HasInstance      *bool  `json:"has_instance"`
	MinBalance       string `json:"min_balance"`
	MaxBalance       string `json:"max_balance"`
	Status           string `json:"status"`
}

// BroadcastTargetQuery 目标检索查询（目标选择器）。
type BroadcastTargetQuery struct {
	Mode     string `form:"mode"` // groups / users / filter
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	// filter 模式下的条件（与 BroadcastFilter 同字段，便于 GET 检索）。
	RegisteredAfter  string `form:"registered_after"`
	RegisteredBefore string `form:"registered_before"`
	Tier             string `form:"tier"`
	HasInstance      *bool  `form:"has_instance"`
	MinBalance       string `form:"min_balance"`
	MaxBalance       string `form:"max_balance"`
	Status           string `form:"status"`
}

// BroadcastGroupItem 用户组选项。
type BroadcastGroupItem struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	MemberCount int64  `json:"member_count"`
}

// BroadcastUserItem 可群发用户（只返回打码后的联系方式，避免权限放大）。
type BroadcastUserItem struct {
	ID          uint64 `json:"id"`
	Username    string `json:"username"`
	EmailMasked string `json:"email_masked"`
	PhoneMasked string `json:"phone_masked"`
}

// BroadcastTargetResponse 目标检索响应。
type BroadcastTargetResponse struct {
	Total  int64                `json:"total"`
	Items  []BroadcastUserItem  `json:"items,omitempty"`
	Groups []BroadcastGroupItem `json:"groups,omitempty"`
}

// BroadcastPreviewRequest 预览请求（目标 + 内容）。
type BroadcastPreviewRequest struct {
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Format      string            `json:"format"`
	Channels    []string          `json:"channels"`
	Vars        map[string]string `json:"vars"`
	PerUserVars bool              `json:"per_user_vars"`
	Target      BroadcastTarget   `json:"target" binding:"required"`
	// SmsTemplateCode 短信模板编码（channels 含 sms 时必填）。
	SmsTemplateCode string `json:"sms_template_code"`
}

// BroadcastPreviewSample 预览样例（最多 10 条，联系方式打码）。
type BroadcastPreviewSample struct {
	ID          uint64 `json:"id"`
	Username    string `json:"username"`
	EmailMasked string `json:"email_masked"`
	PhoneMasked string `json:"phone_masked"`
}

// BroadcastPreviewResponse 预览响应。
type BroadcastPreviewResponse struct {
	Total               int64                    `json:"total"`
	Sample              []BroadcastPreviewSample `json:"sample"`
	Rendered            RenderedPreview          `json:"rendered"`
	EstimatedSMSCostFen int64                    `json:"estimated_sms_cost_fen"`
	EstimatedMailCount  int64                    `json:"estimated_mail_count"`
	Exceeded            bool                     `json:"exceeded"` // 超出单次上限
	MaxTargets          int64                    `json:"max_targets"`
	Message             string                   `json:"message"`
}

// RenderedPreview 渲染后的文案预览。
type RenderedPreview struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// BroadcastRequest 发送请求。
type BroadcastRequest struct {
	Title           string            `json:"title" binding:"required"`
	Content         string            `json:"content" binding:"required"`
	Format          string            `json:"format"` // text / html（HTML 仅邮件有意义）
	Channels        []string          `json:"channels" binding:"required"`
	SmsTemplateCode string            `json:"sms_template_code"`
	Vars            map[string]string `json:"vars"`
	PerUserVars     bool              `json:"per_user_vars"`
	Target          BroadcastTarget   `json:"target" binding:"required"`
	ScheduleAt      string            `json:"schedule_at"` // 空=立即；预留字段，当前即时入队
}

// BroadcastResponse 发送响应。
type BroadcastResponse struct {
	BatchID string `json:"batch_id"`
	Total   int64  `json:"total"`
	Queued  int64  `json:"queued"`
	Message string `json:"message"`
}
