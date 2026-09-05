package model

import "time"

// TicketAttachment 工单附件
type TicketAttachment struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	TicketID   uint64    `gorm:"column:ticket_id;index"`             // 所属工单
	ReplyID    uint64    `gorm:"column:reply_id;index"`              // 关联回复ID（0=工单主附件）
	FileName   string    `gorm:"column:file_name;size:255;not null"` // 文件名
	FileURL    string    `gorm:"column:file_url;size:512;not null"`  // 文件访问地址
	FileSize   int64     `gorm:"column:file_size"`                   // 文件大小（字节）
	FileType   string    `gorm:"column:file_type;size:64"`           // MIME 类型
	UploaderID uint64    `gorm:"column:uploader_id;not null"`        // 上传者ID
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (TicketAttachment) TableName() string {
	return "ticket_attachments"
}
