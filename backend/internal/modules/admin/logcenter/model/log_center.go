package model

import "time"

// 保留策略动作（与 catalog.Action* 取值一致，此处不 import catalog 以避免环路）。
const (
	ActionClean       = "clean"
	ActionArchiveOnly = "archive_only"
	ActionKeep        = "keep"
)

// LogRetentionPolicy 逐源保留策略（doc92 §2.3）。
type LogRetentionPolicy struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	SourceKey     string     `gorm:"column:source_key;size:64;not null"`
	DisplayName   string     `gorm:"column:display_name;size:100;not null"`
	Action        string     `gorm:"size:20;not null;default:clean"`
	RetentionDays int        `gorm:"column:retention_days;not null;default:180"`
	ArchiveCron   string     `gorm:"column:archive_cron;size:32;not null;default:''"`
	BatchSize     int        `gorm:"column:batch_size;not null;default:5000"`
	Enabled       bool       `gorm:"not null;default:true"`
	LastRunAt     *time.Time `gorm:"column:last_run_at"`
	LastDeleted   int64      `gorm:"column:last_deleted;not null;default:0"`
	UpdatedBy     uint64     `gorm:"column:updated_by;not null;default:0"`
	Remark        string     `gorm:"size:255;not null;default:''"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (LogRetentionPolicy) TableName() string { return "log_retention_policies" }

// 导出文件状态。
const (
	ExportStatusSuccess = "success"
	ExportStatusFailed  = "failed"
	ExportStatusDeleted = "deleted" // 文件已删除，行永久保留
)

// LogExportFile 导出文件元数据（doc92 §2.4）。
type LogExportFile struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	SourceKey    string     `gorm:"column:source_key;size:64;not null"`
	JobID        uint64     `gorm:"column:job_id;not null;default:0"`
	Format       string     `gorm:"size:10;not null;default:csv"`
	StoragePath  string     `gorm:"column:storage_path;size:500;not null"`
	FileName     string     `gorm:"column:file_name;size:255;not null"`
	FileSize     int64      `gorm:"column:file_size;not null;default:0"`
	RowCount     int64      `gorm:"column:row_count;not null;default:0"`
	SHA256       string     `gorm:"column:sha256;size:64;not null;default:''"`
	PeriodFrom   *time.Time `gorm:"column:period_from"`
	PeriodTo     *time.Time `gorm:"column:period_to"`
	Filters      string     `gorm:"type:jsonb"`
	OperatorID   uint64     `gorm:"column:operator_id;not null;default:0"`
	OperatorName string     `gorm:"column:operator_name;size:64;not null;default:''"`
	Status       string     `gorm:"size:20;not null;default:success"`
	FailReason   string     `gorm:"column:fail_reason;size:500;not null;default:''"`
	ExpiresAt    *time.Time `gorm:"column:expires_at"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (LogExportFile) TableName() string { return "log_export_files" }

// 清理任务状态机（doc92 §6.1）。
const (
	CleanupStatusPending   = "pending"
	CleanupStatusExporting = "exporting"
	CleanupStatusVerifying = "verifying"
	CleanupStatusDeleting  = "deleting"
	CleanupStatusDone      = "done"
	CleanupStatusFailed    = "failed"
	CleanupStatusCancelled = "cancelled"
)

// 触发方式。
const (
	TriggerManual    = "manual"
	TriggerScheduled = "scheduled"
)

// CleanupActiveStatuses 非终态集合：互斥索引与「取消」判定都用它。
func CleanupActiveStatuses() []string {
	return []string{CleanupStatusPending, CleanupStatusExporting, CleanupStatusVerifying, CleanupStatusDeleting}
}

// IsActiveStatus 判定是否处于非终态（可取消、参与互斥）。
func IsActiveStatus(status string) bool {
	for _, s := range CleanupActiveStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// LogCleanupJob 日志清理任务（doc92 §2.5）。
type LogCleanupJob struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement"`
	JobNo          string     `gorm:"column:job_no;size:40;not null"`
	TriggerType    string     `gorm:"column:trigger_type;size:20;not null;default:manual"`
	Status         string     `gorm:"size:20;not null;default:pending"`
	Sources        string     `gorm:"type:jsonb"`
	BeforeTime     time.Time  `gorm:"column:before_time;not null"`
	DryRun         bool       `gorm:"column:dry_run;not null;default:false"`
	ExportRequired bool       `gorm:"column:export_required;not null;default:true"`
	ExportFileIDs  string     `gorm:"column:export_file_ids;type:jsonb"`
	ScannedRows    int64      `gorm:"column:scanned_rows;not null;default:0"`
	DeletedRows    int64      `gorm:"column:deleted_rows;not null;default:0"`
	ExpiredRows    int64      `gorm:"column:expired_rows;not null;default:0"`
	Progress       int        `gorm:"not null;default:0"`
	CurrentSource  string     `gorm:"column:current_source;size:64;not null;default:''"`
	ErrorMessage   string     `gorm:"column:error_message;size:1000;not null;default:''"`
	OperatorID     uint64     `gorm:"column:operator_id;not null;default:0"`
	OperatorName   string     `gorm:"column:operator_name;size:64;not null;default:''"`
	StartedAt      *time.Time `gorm:"column:started_at"`
	FinishedAt     *time.Time `gorm:"column:finished_at"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (LogCleanupJob) TableName() string { return "log_cleanup_jobs" }
