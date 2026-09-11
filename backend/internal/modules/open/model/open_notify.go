package model

import "time"

// 回调投递状态（T6.5）。
const (
	OpenNotifyPending string = "pending" // 待投递
	OpenNotifySuccess string = "success" // 已投递（对端 2xx）
	OpenNotifyFailed  string = "failed"  // 未投递完，等待下次重试
	OpenNotifyDead    string = "dead"    // 次数耗尽，转死信（可人工重投）
)

// OpenNotifyDelivery 事件回调投递记录（T6.5）。
// 工作池按 (status, next_retry_at) 领取：投递成功转 success；失败累加 attempts
// 并按指数退避排 next_retry_at；attempts 耗尽转 dead。
type OpenNotifyDelivery struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	AppID       uint64     `gorm:"column:app_id;not null;index"`
	Event       string     `gorm:"column:event;size:64;not null"`
	Payload     string     `gorm:"column:payload;type:jsonb;not null"`
	Status      string     `gorm:"column:status;size:16;not null;default:'pending';index:idx_open_notify_deliveries_claim"`
	Attempts    int        `gorm:"column:attempts;not null;default:0"`
	MaxAttempts int        `gorm:"column:max_attempts;not null;default:8"`
	LastError   string     `gorm:"column:last_error;type:text"`
	NextRetryAt *time.Time `gorm:"column:next_retry_at;index:idx_open_notify_deliveries_claim"`
	DeliveredAt *time.Time `gorm:"column:delivered_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (OpenNotifyDelivery) TableName() string { return "open_notify_deliveries" }
