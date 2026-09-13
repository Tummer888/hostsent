package model

import "time"

// UpstreamAPILog 上游厂商接口调用日志（doc92 §2.1）。
//
// 摘要（sha256）恒写、正文可采样可截断：关掉正文采样后仍能用摘要比对
// 「当时上游返回的是什么」。success 为独立列，因为上游常用 200 + body.error
// 表达业务失败，靠状态码推断会把失败记成成功。
type UpstreamAPILog struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	ProviderID   uint64 `gorm:"column:provider_id;not null;default:0"`
	ProviderName string `gorm:"column:provider_name;size:100;not null;default:''"`
	ProviderType string `gorm:"column:provider_type;size:50;not null;default:''"`
	Op           string `gorm:"size:64;not null;default:''"`
	Method       string `gorm:"size:8;not null;default:''"`
	URL          string `gorm:"size:500;not null;default:''"`

	RequestDigest    string `gorm:"column:request_digest;size:64;not null;default:''"`
	RequestBody      string `gorm:"column:request_body;type:text"`
	RequestBytes     int    `gorm:"column:request_bytes;not null;default:0"`
	RequestTruncated bool   `gorm:"column:request_truncated;not null;default:false"`

	StatusCode        int    `gorm:"column:status_code;not null;default:0"`
	ResponseDigest    string `gorm:"column:response_digest;size:64;not null;default:''"`
	ResponseBody      string `gorm:"column:response_body;type:text"`
	ResponseBytes     int    `gorm:"column:response_bytes;not null;default:0"`
	ResponseTruncated bool   `gorm:"column:response_truncated;not null;default:false"`

	Success      bool   `gorm:"not null;default:false"`
	ErrorCode    string `gorm:"column:error_code;size:64;not null;default:''"`
	ErrorMessage string `gorm:"column:error_message;size:500;not null;default:''"`
	DurationMS   int    `gorm:"column:duration_ms;not null;default:0"`
	RetryIndex   int    `gorm:"column:retry_index;not null;default:0"`
	TraceID      string `gorm:"column:trace_id;size:64;not null;default:''"`
	CreatedAt    time.Time
}

func (UpstreamAPILog) TableName() string { return "upstream_api_logs" }

// 任务日志状态。
const (
	JobStatusRunning = "running"
	JobStatusSuccess = "success"
	JobStatusFailed  = "failed"
	JobStatusSkipped = "skipped"
)

// JobRunLog 统一定时任务执行日志（doc92 §2.2）。
//
// uk_job_run_logs_running 约束同一 job_name 至多一行 running：插入冲突即说明
// 上一轮还没跑完，本轮跳过（重入保护）。
type JobRunLog struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	JobName       string     `gorm:"column:job_name;size:64;not null"`
	JobGroup      string     `gorm:"column:job_group;size:32;not null;default:''"`
	TriggerType   string     `gorm:"column:trigger_type;size:20;not null;default:ticker"`
	Status        string     `gorm:"size:20;not null;default:running"`
	StartedAt     time.Time  `gorm:"column:started_at;not null"`
	FinishedAt    *time.Time `gorm:"column:finished_at"`
	DurationMS    int        `gorm:"column:duration_ms;not null;default:0"`
	ItemsScanned  int        `gorm:"column:items_scanned;not null;default:0"`
	ItemsAffected int        `gorm:"column:items_affected;not null;default:0"`
	Summary       string     `gorm:"type:jsonb"`
	ErrorMessage  string     `gorm:"column:error_message;size:1000;not null;default:''"`
	CreatedAt     time.Time
}

func (JobRunLog) TableName() string { return "job_run_logs" }
