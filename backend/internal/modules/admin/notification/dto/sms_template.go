package dto

// SmsTemplateListQuery 短信模板列表查询。
type SmsTemplateListQuery struct {
	Scene    string `form:"scene"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// SmsTemplateInfo 短信模板信息。
type SmsTemplateInfo struct {
	ID           uint64   `json:"id"`
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Scene        string   `json:"scene"`
	Content      string   `json:"content"`
	UpstreamCode string   `json:"upstream_code"`
	VarNames     []string `json:"var_names"`
	Status       string   `json:"status"`
	Remark       string   `json:"remark"`
	// CharCount 正文字符数；Segments 预计短信条数（70 字/条，超 70 按 67 字/条）。
	CharCount int    `json:"char_count"`
	Segments  int    `json:"segments"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// SmsTemplateListResponse 短信模板列表响应。
type SmsTemplateListResponse struct {
	Items []SmsTemplateInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
}

// SmsTemplateSaveRequest 新建/更新短信模板请求。
type SmsTemplateSaveRequest struct {
	Code         string `json:"code"`
	Name         string `json:"name" binding:"required"`
	Scene        string `json:"scene"`
	Content      string `json:"content" binding:"required"`
	UpstreamCode string `json:"upstream_code"`
	Status       string `json:"status"`
	Remark       string `json:"remark"`
}

// SmsTemplatePreviewRequest 用注册变量样例值渲染预览。
type SmsTemplatePreviewRequest struct {
	Content string            `json:"content" binding:"required"`
	Vars    map[string]string `json:"vars"` // 空则用注册表 sample
}

// SmsTemplatePreviewResponse 预览结果。
type SmsTemplatePreviewResponse struct {
	Rendered  string `json:"rendered"`
	CharCount int    `json:"char_count"`
	Segments  int    `json:"segments"`
}

// TemplateVarInfo 模板变量注册项。
type TemplateVarInfo struct {
	ID          uint64   `json:"id"`
	VarKey      string   `json:"var_key"`
	Label       string   `json:"label"`
	Category    string   `json:"category"`
	ValueType   string   `json:"value_type"`
	Sample      string   `json:"sample"`
	Description string   `json:"description"`
	Scenes      []string `json:"scenes"`
	SortOrder   int      `json:"sort_order"`
	Status      string   `json:"status"`
}

// TemplateVarSaveRequest 新增/更新模板变量请求。
type TemplateVarSaveRequest struct {
	VarKey      string   `json:"var_key"`
	Label       string   `json:"label" binding:"required"`
	Category    string   `json:"category"`
	ValueType   string   `json:"value_type"`
	Sample      string   `json:"sample"`
	Description string   `json:"description"`
	Scenes      []string `json:"scenes"`
	SortOrder   int      `json:"sort_order"`
	Status      string   `json:"status"`
}

// TestSendRequest 测试发送请求（短信/邮件统一入口）。
type TestSendRequest struct {
	Category     string            `json:"category" binding:"required,oneof=mail sms"`
	ChannelID    uint64            `json:"channel_id"`    // 0=用该 category 的默认渠道
	TemplateCode string            `json:"template_code"` // 短信模板 code；空则发纯文本
	Target       string            `json:"target" binding:"required"`
	Vars         map[string]string `json:"vars"`
	Content      string            `json:"content"` // 直接发文本（与 template_code 二选一）
}

// TestSendResponse 测试发送结果。
type TestSendResponse struct {
	OK            bool   `json:"ok"`
	Pending       bool   `json:"pending"` // true = 适配器待接入（不算失败）
	ChannelName   string `json:"channel_name"`
	ChannelID     uint64 `json:"channel_id"`
	ProviderCode  string `json:"provider_code"`
	ProviderMsgID string `json:"provider_msg_id"`
	CostFen       int    `json:"cost_fen"`
	Message       string `json:"message"`
	Raw           string `json:"raw"`
}
