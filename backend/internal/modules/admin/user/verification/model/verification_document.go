// Package model 提供实名认证模块的数据模型。
package model

import "time"

// VerificationDocument 表示实名认证资料附件。
type VerificationDocument struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	ApplicationID uint64    `gorm:"column:application_id;not null;index"`
	DocumentType  string    `gorm:"column:document_type;size:64;not null;index"`
	FileURL       string    `gorm:"column:file_url;size:500;not null"`
	Sort          int       `gorm:"column:sort;not null;default:1"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

// TableName 返回实名认证资料附件表名。
func (VerificationDocument) TableName() string {
	return "verification_documents"
}
