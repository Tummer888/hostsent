// Package model 提供生命周期模块的数据模型。
package model

import "time"

// 续费记录状态
const (
	RenewalStatusPending   string = "pending"   // 待支付（订单已生成）
	RenewalStatusSuccess   string = "success"   // 续费成功
	RenewalStatusFailed    string = "failed"    // 续费失败（余额不足等）
	RenewalStatusCancelled string = "cancelled" // 已取消
)

// 续费来源
const (
	RenewalSourceManual string = "manual" // 用户手动续费
	RenewalSourceAuto   string = "auto"   // 自动续费
	RenewalSourceAdmin  string = "admin"  // 管理员代续费
)

// 续费同步状态（instance_renewals.sync_state，T5.2）：
// 链路 A 续费成功落 upstream_ok；链路 B 无上游续费接口时落 local_only（本地账期顺延）；
// failed 表示上游续费失败（本地账期不得顺延，续费单保持待重试）。
const (
	RenewalSyncUpstreamOK string = "upstream_ok"
	RenewalSyncLocalOnly  string = "local_only"
	RenewalSyncFailed     string = "failed"
)

// 生命周期派生阶段（不落库，按 expire_at + 策略推导）
const (
	StageActive    string = "active"    // 服务中（未到期）
	StageGrace     string = "grace"     // 宽限期中（已过期未超宽限期）
	StageSuspended string = "suspended" // 已暂停（宽限期结束）
	StageDestroyed string = "destroyed" // 已销毁（终态，本地标记）
)

// InstanceRenewal 续费记录
type InstanceRenewal struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RenewalNo    string     `gorm:"column:renewal_no;size:64;uniqueIndex;not null" json:"renewal_no"`
	InstanceID   uint64     `gorm:"column:instance_id;index;not null" json:"instance_id"` // instances.id
	InstanceMark string     `gorm:"column:instance_mark;size:64" json:"instance_mark"`    // 上游 instance_id 快照
	UserID       uint64     `gorm:"column:user_id;index;not null" json:"user_id"`
	ProductID    uint64     `gorm:"column:product_id" json:"product_id"`
	ProductName  string     `gorm:"column:product_name;size:128" json:"product_name"` // 快照
	BillingMode  string     `gorm:"column:billing_mode;size:20;not null" json:"billing_mode"`
	PeriodCount  int        `gorm:"column:period_count;not null;default:1" json:"period_count"` // 续费周期数（默认 1 个周期）
	Amount       float64    `gorm:"type:decimal(15,2);not null;default:0" json:"amount"`
	Source       string     `gorm:"column:source;size:20;not null;default:manual" json:"source"`
	Status       string     `gorm:"size:32;not null;default:pending;index" json:"status"`
	OrderID      uint64     `gorm:"column:order_id;index" json:"order_id"` // 关联续费订单
	OrderNo      string     `gorm:"column:order_no;size:64" json:"order_no"`
	Remark       string     `gorm:"column:remark;size:255" json:"remark"`
	PayTime      *time.Time `gorm:"column:pay_time" json:"pay_time"`
	ExpireBefore *time.Time `gorm:"column:expire_before" json:"expire_before"` // 续费前到期时间
	ExpireAfter  *time.Time `gorm:"column:expire_after" json:"expire_after"`   // 续费后到期时间
	// UpstreamOrderID 上游续费回执（账单号/hostid），链路 A 对账与幂等用（T5.2）。
	UpstreamOrderID string `gorm:"column:upstream_order_id;size:128" json:"upstream_order_id"`
	// SyncState 续费同步状态：upstream_ok / local_only / failed（T5.2）。
	SyncState  string    `gorm:"column:sync_state;size:32" json:"sync_state"`
	FailReason string    `gorm:"column:fail_reason;size:255" json:"fail_reason"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (InstanceRenewal) TableName() string { return "instance_renewals" }

// LifecyclePolicy 生命周期全局策略（单行表，ID=1）
type LifecyclePolicy struct {
	ID               uint64    `gorm:"primaryKey" json:"id"`
	RemindDays       string    `gorm:"column:remind_days;size:64;not null;default:'7,3,1'" json:"remind_days"` // 到期前提醒天数，逗号分隔
	AutoRenewDefault bool      `gorm:"column:auto_renew_default;not null;default:false" json:"auto_renew_default"`
	GraceDays        int       `gorm:"column:grace_days;not null;default:7" json:"grace_days"`                // 宽限期天数
	DestroyKeepDays  int       `gorm:"column:destroy_keep_days;not null;default:30" json:"destroy_keep_days"` // 暂停后保留天数
	Status           string    `gorm:"size:32;not null;default:active" json:"status"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (LifecyclePolicy) TableName() string { return "lifecycle_policies" }

// InstanceAutoRenewal 实例自动续费开关（按实例持久化）
type InstanceAutoRenewal struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	InstanceID  uint64    `gorm:"column:instance_id;uniqueIndex;not null" json:"instance_id"`
	UserID      uint64    `gorm:"column:user_id;index;not null" json:"user_id"`
	Enabled     bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	PeriodCount int       `gorm:"column:period_count;not null;default:1" json:"period_count"` // 自动续费周期数
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (InstanceAutoRenewal) TableName() string { return "instance_auto_renewals" }
