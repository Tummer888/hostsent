package model

import "time"

type AuditLog struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	OperatorID     uint64    `gorm:"column:operator_id;not null;index"`
	OperatorName   string    `gorm:"column:operator_name;size:64;not null"`
	Module         string    `gorm:"size:64;not null;index"`
	ResourceType   string    `gorm:"column:resource_type;size:64;not null;index"`
	ResourceID     string    `gorm:"column:resource_id;size:64;index"`
	Action         string    `gorm:"size:64;not null;index"`
	RequestMethod  string    `gorm:"column:request_method;size:16;not null"`
	RequestPath    string    `gorm:"column:request_path;size:255;not null"`
	RequestPayload string    `gorm:"column:request_payload;type:text"`
	ResponseCode   int       `gorm:"column:response_code;not null"`
	ResponseMessage string   `gorm:"column:response_message;size:255"`
	IP             string    `gorm:"column:ip;size:64;index"`
	UserAgent      string    `gorm:"column:user_agent;size:255"`
	TraceID        string    `gorm:"column:trace_id;size:128;index"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
