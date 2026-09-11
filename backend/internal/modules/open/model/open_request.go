package model

import "time"

// OpenRequest 写接口幂等记录（doc16 §8.5）。
// (app_id, client_request_id) 唯一：completed 后 response_body 存首次响应快照，
// 重放直接返回；processing 停滞超时的行由服务层判定可安全重执行。
type OpenRequest struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement"`
	AppID           uint64    `gorm:"column:app_id;not null;uniqueIndex:uk_open_requests_app_client"`
	ClientRequestID string    `gorm:"column:client_request_id;size:128;not null;uniqueIndex:uk_open_requests_app_client"`
	Method          string    `gorm:"column:method;size:8;default:''"`
	Path            string    `gorm:"column:path;size:255;default:''"`
	Status          string    `gorm:"column:status;size:16;not null;default:'processing'"`
	ResponseStatus  *int      `gorm:"column:response_status"`
	ResponseBody    *string   `gorm:"column:response_body"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (OpenRequest) TableName() string { return "open_requests" }

// 幂等记录状态。
const (
	OpenRequestProcessing string = "processing"
	OpenRequestCompleted  string = "completed"
)

// OpenAPILog 请求/响应摘要（排障与对账）。
// request_digest / response_digest 为截断后的原文片段（request 侧另有 SHA256 摘要参与签名）。
type OpenAPILog struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement"`
	AppID           uint64    `gorm:"column:app_id"`
	Method          string    `gorm:"column:method;size:8;default:''"`
	Path            string    `gorm:"column:path;size:255;default:''"`
	Query           string    `gorm:"column:query;type:text"`
	ClientRequestID string    `gorm:"column:client_request_id;size:128"`
	StatusCode      int       `gorm:"column:status_code"`
	ErrorCode       int       `gorm:"column:error_code"`
	DurationMS      int64     `gorm:"column:duration_ms"`
	RequestDigest   string    `gorm:"column:request_digest;type:text"`
	ResponseDigest  string    `gorm:"column:response_digest;type:text"`
	IP              string    `gorm:"column:ip;size:64"`
	CreatedAt       time.Time `gorm:"autoCreateTime;index"`
}

// TableName 指定表名。
func (OpenAPILog) TableName() string { return "open_api_logs" }
