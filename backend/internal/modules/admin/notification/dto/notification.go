// Package dto 提供通知与消息中心模块的接口出入参。
package dto

// PublishInput 发布通知入参（内部调用，非 HTTP）。
type PublishInput struct {
	Event        string            `json:"event"`         // 事件类型
	UserID       uint64            `json:"user_id"`       // 接收用户
	Target       string            `json:"target"`        // user / admin
	Vars         map[string]string `json:"vars"`          // 模板变量
	SourceModule string            `json:"source_module"` // 来源模块
	SourceID     string            `json:"source_id"`     // 来源单号（幂等去重）
}

// AnnouncementCreateRequest 创建公告请求。
type AnnouncementCreateRequest struct {
	Title     string `json:"title" binding:"required"`
	Content   string `json:"content" binding:"required"`
	Platform  string `json:"platform"` // user/admin/both，默认 user
	Level     string `json:"level"`    // info/warning/critical，默认 info
	Popup     bool   `json:"popup"`
	PublishAt string `json:"publish_at"` // ISO 时间字符串，空=立即发布
}

// AnnouncementUpdateRequest 更新公告请求。
type AnnouncementUpdateRequest struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Platform string `json:"platform"`
	Level    string `json:"level"`
	Popup    *bool  `json:"popup"`
}

// AnnouncementInfo 公告信息。
type AnnouncementInfo struct {
	ID         uint64  `json:"id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Platform   string  `json:"platform"`
	Level      string  `json:"level"`
	Popup      bool    `json:"popup"`
	Status     string  `json:"status"`
	PublishAt  *string `json:"publish_at"`
	OfflineAt  *string `json:"offline_at"`
	OperatorID uint64  `json:"operator_id"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// AnnouncementListQuery 公告列表查询。
type AnnouncementListQuery struct {
	Status   string `form:"status"`
	Platform string `form:"platform"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// NotificationListQuery 通知记录列表查询（管理端）。
type NotificationListQuery struct {
	Event      string `form:"event"`
	Channel    string `form:"channel"`
	SendStatus string `form:"send_status"`
	TargetType string `form:"target_type"`
	Keyword    string `form:"keyword"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// NotificationInfo 通知记录信息。
type NotificationInfo struct {
	ID           uint64  `json:"id"`
	UserID       uint64  `json:"user_id"`
	TargetType   string  `json:"target_type"`
	Event        string  `json:"event"`
	Title        string  `json:"title"`
	Content      string  `json:"content"`
	Channel      string  `json:"channel"`
	SendStatus   string  `json:"send_status"`
	FailReason   string  `json:"fail_reason"`
	SourceModule string  `json:"source_module"`
	SourceID     string  `json:"source_id"`
	ReadAt       *string `json:"read_at"`
	CreatedAt    string  `json:"created_at"`
	// ContentFormat 正文格式（text/html）：前端 HTML 站内信按富文本渲染（doc90 §8.6）。
	ContentFormat string `json:"content_format"`
	// DeliveryID 关联的外发投递记录 ID；0 = 纯站内信（无重发入口，doc90 §8.6）。
	DeliveryID uint64 `json:"delivery_id"`
}

// TemplateInfo 模板信息。
type TemplateInfo struct {
	ID            uint64 `json:"id"`
	Event         string `json:"event"`
	TitleTpl      string `json:"title_tpl"`
	ContentTpl    string `json:"content_tpl"`
	InboxOn       bool   `json:"inbox_on"`
	MailOn        bool   `json:"mail_on"`
	SmsOn         bool   `json:"sms_on"`
	SmsTemplateID uint64 `json:"sms_template_id"`
	MailFormat    string `json:"mail_format"`
	TitleShow     bool   `json:"title_show"`
	Status        string `json:"status"`
	UpdatedAt     string `json:"updated_at"`
}

// TemplateUpdateRequest 更新模板请求。
type TemplateUpdateRequest struct {
	TitleTpl      *string `json:"title_tpl"`
	ContentTpl    *string `json:"content_tpl"`
	InboxOn       *bool   `json:"inbox_on"`
	MailOn        *bool   `json:"mail_on"`
	SmsOn         *bool   `json:"sms_on"`
	SmsTemplateID *uint64 `json:"sms_template_id"`
	MailFormat    *string `json:"mail_format"` // text / html
	TitleShow     *bool   `json:"title_show"`
	Status        *string `json:"status"`
}

// MailTestRequest 发送测试邮件请求。
type MailTestRequest struct {
	To string `json:"to" binding:"required,email"`
}

// PreferenceItem 偏好项。
type PreferenceItem struct {
	Event   string `json:"event"`
	InboxOn bool   `json:"inbox_on"`
	MailOn  bool   `json:"mail_on"`
	SmsOn   bool   `json:"sms_on"`
	// Mandatory 强制送达事件（OTP/验证码类）：前端置灰开关并提示不可关闭（doc90 §8.7）。
	Mandatory bool `json:"mandatory"`
}

// PreferenceUpdateRequest 批量更新偏好请求。
//
// bug ① 修复：后端 json tag 是 items，用户端 preferences.vue 发的是 list，
// 字段名不匹配导致保存「成功但没生效」（items 恒为空）。这里同时接受两种键名，
// 并去掉 items 的 required —— 否则前端发 list 时 items 为空会直接 400，
// 校验改为在 handler 里对归一化结果做。
type PreferenceUpdateRequest struct {
	Items []PreferenceItem `json:"items"`
	List  []PreferenceItem `json:"list"` // 兼容用户端现有请求体
}

// Normalize 归一化两种键名为一个列表。
func (r *PreferenceUpdateRequest) Normalize() []PreferenceItem {
	if len(r.List) > 0 {
		return r.List
	}
	return r.Items
}

// UnreadCountResponse 未读数响应。
type UnreadCountResponse struct {
	Count int64 `json:"count"`
}
