package model

import "time"

// ============================================================================
// P3 同步框架模型（T3.2 ~ T3.5）
//
// 与 sync.go 里的 SyncTask/SyncLog 同域：本文件承载"渠道 × scope 多节奏调度"、
// 增量游标/全量对账、上游改价事件、同步差异四张表（迁移 031）。
// ============================================================================

// 同步 scope：由上游能力描述符 CapabilityDescriptor.SyncScopes 决定。
// 与 upstream.Scope* 常量同值，此处重复声明以避免 model 依赖 upstream 包。
const (
	ScopeCatalog  = "catalog"
	ScopePrice    = "price"
	ScopePool     = "pool"
	ScopeRegion   = "region"
	ScopeImage    = "image"
	ScopeInstance = "instance"
	ScopeStock    = "stock"
)

// AllScopes 返回全部受支持 scope（稳定顺序，供校验与展示）。
func AllScopes() []string {
	return []string{ScopeCatalog, ScopePrice, ScopePool, ScopeRegion, ScopeImage, ScopeInstance, ScopeStock}
}

// IsScope 判断取值是否为合法 scope。
func IsScope(s string) bool {
	for _, item := range AllScopes() {
		if item == s {
			return true
		}
	}
	return false
}

// 兼容别名：旧接口/旧页面使用 product/pool/instance 三类，映射到新 scope。
var legacyScopeAliases = map[string]string{
	"product":  ScopeCatalog,
	"pool":     ScopePool,
	"instance": ScopeInstance,
}

// NormalizeScope 把旧任务类型别名归一为新 scope；已是 scope 则原样返回。
func NormalizeScope(taskType string) string {
	if IsScope(taskType) {
		return taskType
	}
	if s, ok := legacyScopeAliases[taskType]; ok {
		return s
	}
	return taskType
}

// 调度状态（sync_schedules.last_status）。
const (
	ScheduleStatusSuccess = "success"
	ScheduleStatusFailed  = "failed"
	ScheduleStatusSkipped = "skipped"
)

// SyncSchedule 渠道 × scope 调度配置（T3.2）。
// 每个 (provider_id, scope) 一行，各自节奏；window_start/window_end 为允许
// 执行的小时区间（0-23，NULL 表示不限，支持跨夜如 22→6）。
type SyncSchedule struct {
	ID                      uint64     `gorm:"primaryKey;autoIncrement"`
	ProviderID              uint64     `gorm:"column:provider_id;not null;uniqueIndex:uk_sync_schedules_provider_scope"`
	Scope                   string     `gorm:"column:scope;size:32;not null;uniqueIndex:uk_sync_schedules_provider_scope"`
	IntervalSeconds         int        `gorm:"column:interval_seconds;not null;default:3600"`
	FullSyncIntervalSeconds int        `gorm:"column:full_sync_interval_seconds;not null;default:604800"`
	Enabled                 bool       `gorm:"not null;default:true"`
	Priority                int        `gorm:"not null;default:0"`
	WindowStart             *int16     `gorm:"column:window_start"`
	WindowEnd               *int16     `gorm:"column:window_end"`
	LastRunAt               *time.Time `gorm:"column:last_run_at"`
	NextRunAt               *time.Time `gorm:"column:next_run_at;index"`
	LastStatus              string     `gorm:"column:last_status;size:20"`
	LastError               string     `gorm:"column:last_error;type:text"`
	CreatedAt               time.Time  `gorm:"autoCreateTime"`
	UpdatedAt               time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SyncSchedule) TableName() string { return "sync_schedules" }

// SyncCursor 渠道 × scope 增量游标与全量对账时间（T3.3）。
type SyncCursor struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement"`
	ProviderID        uint64     `gorm:"column:provider_id;not null;uniqueIndex:uk_sync_cursors_provider_scope"`
	Scope             string     `gorm:"column:scope;size:32;not null;uniqueIndex:uk_sync_cursors_provider_scope"`
	Cursor            string     `gorm:"type:text"`
	LastIncrementalAt *time.Time `gorm:"column:last_incremental_at"`
	LastFullAt        *time.Time `gorm:"column:last_full_at"`
	LastSeenCount     int        `gorm:"column:last_seen_count;not null;default:0"`
	CreatedAt         time.Time  `gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SyncCursor) TableName() string { return "sync_cursors" }

// 调价事件状态（price_change_events.status）。
const (
	PriceChangeAutoApplied = "auto_applied" // 幅度在阈值内，已自动应用成本价
	PriceChangePending     = "pending"      // 超阈值，待人工确认（绝不静默改售价）
	PriceChangeConfirmed   = "confirmed"    // 人工确认并应用
	PriceChangeRejected    = "rejected"     // 人工驳回（保持原售价）
)

// 调价字段。
const (
	PriceFieldCost = "cost_price"
	PriceFieldSale = "sale_price"
)

// PriceChangeEvent 上游调价事件（T3.4）。
type PriceChangeEvent struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement"`
	ProviderID        uint64     `gorm:"column:provider_id;not null;index"`
	Scope             string     `gorm:"column:scope;size:32;not null;default:price"`
	ResourceProductID uint64     `gorm:"column:resource_product_id"`
	UpstreamID        string     `gorm:"column:upstream_id;size:128"`
	ProductID         uint64     `gorm:"column:product_id"`
	Field             string     `gorm:"column:field;size:32;not null;default:cost_price"`
	OldValue          *float64   `gorm:"column:old_value;type:numeric(12,4)"`
	NewValue          *float64   `gorm:"column:new_value;type:numeric(12,4)"`
	ChangeRatio       *float64   `gorm:"column:change_ratio;type:numeric(10,4)"`
	Threshold         *float64   `gorm:"column:threshold;type:numeric(10,4)"`
	Status            string     `gorm:"column:status;size:16;not null;default:pending"`
	Applied           bool       `gorm:"not null;default:false"`
	HandledBy         uint64     `gorm:"column:handled_by"`
	HandledAt         *time.Time `gorm:"column:handled_at"`
	Remark            string     `gorm:"size:255"`
	CreatedAt         time.Time  `gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (PriceChangeEvent) TableName() string { return "price_change_events" }

// 差异动作（sync_diffs.action）。
const (
	DiffCreated      = "created"       // 上游新增，本地新建
	DiffUpdated      = "updated"       // 上游字段变化，本地更新
	DiffOffline      = "offline"       // 本地有、上游无 → 置下线（软删除）
	DiffMissing      = "missing"       // 本地有、上游无 → 仅登记候选，不自动处置
	DiffPriceChanged = "price_changed" // 价格变化
)

// 处置结果（sync_diffs.disposition）。
const (
	DiffDispositionApplied = "applied"
	DiffDispositionPending = "pending"
	DiffDispositionSkipped = "skipped"
)

// SyncDiff 同步差异记录（T3.5）。
type SyncDiff struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	TaskID      uint64    `gorm:"column:task_id;index"`
	ProviderID  uint64    `gorm:"column:provider_id;not null;index"`
	Scope       string    `gorm:"column:scope;size:32;not null"`
	Action      string    `gorm:"column:action;size:16;not null;index"`
	LocalID     uint64    `gorm:"column:local_id"`
	ExternalID  string    `gorm:"column:external_id;size:128"`
	Field       string    `gorm:"column:field;size:64"`
	OldValue    string    `gorm:"column:old_value;type:text"`
	NewValue    string    `gorm:"column:new_value;type:text"`
	Disposition string    `gorm:"column:disposition;size:16;not null;default:applied"`
	Remark      string    `gorm:"size:255"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (SyncDiff) TableName() string { return "sync_diffs" }
