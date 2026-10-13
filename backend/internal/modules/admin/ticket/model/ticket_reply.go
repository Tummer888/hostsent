package model

import "time"

// TicketReply 工单回复（对话消息）
type TicketReply struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	TicketID   uint64 `gorm:"column:ticket_id;not null;index"`     // 所属工单
	SenderType string `gorm:"column:sender_type;size:10;not null"` // 发送人类型：user / admin
	SenderID   uint64 `gorm:"column:sender_id;not null"`           // 发送人ID（用户或管理员）
	SenderName string `gorm:"column:sender_name;size:64;not null"` // 发送人名称快照
	Content    string `gorm:"type:text;not null"`                  // 回复内容
	// IsInternal 内部备注（S2）：仅管理员可见，不推状态、不发用户通知、不计入首次响应。
	IsInternal bool `gorm:"column:is_internal;not null;default:false"`
	// UpstreamReplyID 上游回复号（doc120）：非空表示该回复由上游推送写入。
	// 上游推送失败会重试（zjmf_pushhost 至多 5 次），用 (ticket_id, upstream_reply_id)
	// 部分唯一索引做精确去重，避免同一句回复入库多条。
	UpstreamReplyID string `gorm:"column:upstream_reply_id;size:64;not null;default:''"`
	// —— S3 双人复核：回复级复核状态 ——
	// 空 = 无需复核；pending 时用户端不可见（SQL 层过滤）。
	ReviewStatus string     `gorm:"column:review_status;size:20;index"`
	ReviewerID   uint64     `gorm:"column:reviewer_id"`
	ReviewedAt   *time.Time `gorm:"column:reviewed_at"`
	ReviewNote   string     `gorm:"column:review_note;size:255"`
	CreatedAt    time.Time  `gorm:"autoCreateTime;index"`
}

// TableName 指定表名
func (TicketReply) TableName() string {
	return "ticket_replies"
}
