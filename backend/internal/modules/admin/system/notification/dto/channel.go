package dto

// ChannelTypeItem 渠道类型列表项（驱动前端「类型选择 → 动态凭证表单」）。
type ChannelTypeItem struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	Mode           string `json:"mode"`
	Icon           string `json:"icon"`
	DocURL         string `json:"doc_url"`
	AdapterVersion string `json:"adapter_version"`
	// Implemented 适配器是否已接入；false 时前端显示「配置占位」并可保存凭证。
	Implemented bool `json:"implemented"`
	// Capabilities 能力描述符（含 credential_schema，前端据此渲染表单）。
	Capabilities any `json:"capabilities"`
}

// ChannelListQuery 渠道列表查询。
type ChannelListQuery struct {
	Category string `form:"category"`
	Type     string `form:"type"`
	Status   *int   `form:"status"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ChannelListResponse 渠道列表响应。
type ChannelListResponse struct {
	Items []ChannelInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// ListMeta 分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ChannelInfo 渠道信息（凭证已按描述符脱敏）。
type ChannelInfo struct {
	ID           uint64            `json:"id"`
	ChannelCode  string            `json:"channel_code"`
	Name         string            `json:"name"`
	Category     string            `json:"category"`
	Type         string            `json:"type"`
	TypeName     string            `json:"type_name"`
	Credentials  map[string]string `json:"credentials"`
	Endpoint     string            `json:"endpoint"`
	SignName     string            `json:"sign_name"`
	Sender       string            `json:"sender"`
	TemplateCode string            `json:"template_code"`
	Scenes       []string          `json:"scenes"`
	Priority     int               `json:"priority"`
	Weight       int               `json:"weight"`
	DailyLimit   int               `json:"daily_limit"`
	HealthStatus string            `json:"health_status"`
	LastError    string            `json:"last_error"`
	LastCheckAt  string            `json:"last_check_at"`
	Status       int               `json:"status"`
	IsDefault    bool              `json:"is_default"`
	Remark       string            `json:"remark"`
	// Implemented 适配器是否已接入（false = 配置占位，测试发送返回待接入）。
	Implemented  bool   `json:"implemented"`
	Capabilities any    `json:"capabilities"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ChannelCreateRequest 新建渠道请求。
type ChannelCreateRequest struct {
	ChannelCode  string            `json:"channel_code" binding:"required"`
	Name         string            `json:"name" binding:"required"`
	Type         string            `json:"type" binding:"required"`
	Credentials  map[string]string `json:"credentials"`
	Endpoint     string            `json:"endpoint"`
	SignName     string            `json:"sign_name"`
	Sender       string            `json:"sender"`
	TemplateCode string            `json:"template_code"`
	Scenes       []string          `json:"scenes"`
	Priority     int               `json:"priority"`
	Weight       int               `json:"weight"`
	DailyLimit   int               `json:"daily_limit"`
	IsDefault    bool              `json:"is_default"`
	Remark       string            `json:"remark"`
}

// ChannelUpdateRequest 更新渠道请求（channel_code 与 type 不可改）。
type ChannelUpdateRequest struct {
	Name         string            `json:"name"`
	Credentials  map[string]string `json:"credentials"`
	Endpoint     string            `json:"endpoint"`
	SignName     string            `json:"sign_name"`
	Sender       string            `json:"sender"`
	TemplateCode string            `json:"template_code"`
	Scenes       []string          `json:"scenes"`
	Priority     int               `json:"priority"`
	Weight       int               `json:"weight"`
	DailyLimit   int               `json:"daily_limit"`
	IsDefault    bool              `json:"is_default"`
	Remark       string            `json:"remark"`
}

// ChannelStatusRequest 启停请求。
type ChannelStatusRequest struct {
	Status int `json:"status"`
}

// ChannelTestRequest 连通性测试请求（target 为空时仅校验配置可组装）。
type ChannelTestRequest struct {
	Target string `json:"target"`
}

// ChannelTestResponse 渠道连通性测试结果。
type ChannelTestResponse struct {
	OK      bool   `json:"ok"`
	Pending bool   `json:"pending"` // true = 适配器待接入（不是失败）
	Message string `json:"message"`
}
