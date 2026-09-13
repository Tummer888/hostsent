package dto

// DeliveryListQuery 投递记录列表查询。
type DeliveryListQuery struct {
	Event      string `form:"event"`
	Channel    string `form:"channel"`
	SendStatus string `form:"send_status"`
	TargetType string `form:"target_type"`
	BatchID    string `form:"batch_id"`
	Keyword    string `form:"keyword"` // 收件地址 / 标题
	StartTime  string `form:"start_time"`
	EndTime    string `form:"end_time"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// DeliveryInfo 投递记录信息（收件地址后端打码，前端不再二次打码）。
type DeliveryInfo struct {
	ID            uint64            `json:"id"`
	BatchID       string            `json:"batch_id"`
	Event         string            `json:"event"`
	Channel       string            `json:"channel"`
	TargetType    string            `json:"target_type"`
	TargetID      uint64            `json:"target_id"`
	TargetName    string            `json:"target_name"`
	Recipient     string            `json:"recipient"`
	ChannelID     uint64            `json:"channel_id"`
	Title         string            `json:"title"`
	Content       string            `json:"content"`
	ContentFormat string            `json:"content_format"`
	Vars          map[string]string `json:"vars"`
	SendStatus    string            `json:"send_status"`
	Attempts      int               `json:"attempts"`
	MaxAttempts   int               `json:"max_attempts"`
	NextRetryAt   string            `json:"next_retry_at"`
	ProviderMsgID string            `json:"provider_msg_id"`
	ProviderCode  string            `json:"provider_code"`
	CostFen       int               `json:"cost_fen"`
	FailReason    string            `json:"fail_reason"`
	SourceModule  string            `json:"source_module"`
	SourceID      string            `json:"source_id"`
	SentAt        string            `json:"sent_at"`
	CreatedAt     string            `json:"created_at"`
}

// DeliveryListResponse 投递记录列表响应。
type DeliveryListResponse struct {
	Items []DeliveryInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// DeliveryRetryRequest 批量重投请求。
type DeliveryRetryRequest struct {
	IDs []uint64 `json:"ids" binding:"required"`
}

// DeliveryRetryResponse 重投结果。
type DeliveryRetryResponse struct {
	Retried int      `json:"retried"` // 重置为 pending 的条数
	Skipped []string `json:"skipped"` // 被跳过的条数与原因（如「已发送不允许重投」）
}
