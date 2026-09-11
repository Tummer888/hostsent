package model

import "time"

// 开通履约任务状态（T5.1）。与 sync_tasks 同构，独立成表避免污染同步任务。
const (
	// ProvisionTaskPending 待处理（可被工作池领取）。
	ProvisionTaskPending string = "pending"
	// ProvisionTaskRunning 执行中（持有 locked_until 租约）。
	ProvisionTaskRunning string = "running"
	// ProvisionTaskSuccess 开通成功。
	ProvisionTaskSuccess string = "success"
	// ProvisionTaskFailed 本轮失败，等待下次重试（attempts < max_attempts）。
	ProvisionTaskFailed string = "failed"
	// ProvisionTaskManual 连续失败达上限，转人工处理队列并告警。
	ProvisionTaskManual string = "manual"
)

// DefaultProvisionMaxAttempts 连续失败上限：达到后转人工队列（doc17 §4-T5.1）。
const DefaultProvisionMaxAttempts = 5

// ProvisionTask 开通履约任务。
//
// 背景（doc15 §6.1）：上游开通约 50s，超过 app.write_timeout=10s，必须异步化。
// 下单只落库并投递本表，工作池领取后调用订单履约（paid→provisioning→active）；
// 失败保留订单 paid 可重试，连续失败进人工队列。
//
// order_id 唯一：重试复用同一行并累加 attempts，天然幂等（不会重复投递重复开通）。
type ProvisionTask struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	OrderID     uint64 `gorm:"column:order_id;uniqueIndex;not null"`
	UserID      uint64 `gorm:"column:user_id;not null;index"`
	ProductID   uint64 `gorm:"column:product_id"`
	SourceMode  string `gorm:"column:source_mode;size:16"`
	Status      string `gorm:"size:20;not null;default:pending;index"`
	Attempts    int    `gorm:"not null;default:0"`
	MaxAttempts int    `gorm:"column:max_attempts;not null;default:5"`
	LastError   string `gorm:"column:last_error;type:text"`
	// LockedUntil 工作池领取租约：租约到期仍 running 的任务可被回收重试。
	LockedUntil *time.Time `gorm:"column:locked_until;index"`
	StartedAt   *time.Time `gorm:"column:started_at"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (ProvisionTask) TableName() string { return "provision_tasks" }
