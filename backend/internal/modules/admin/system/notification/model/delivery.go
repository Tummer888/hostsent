package model

import "time"

// 投递通道（notifications.channel 与 notification_deliveries.channel 共用）。
// ChannelInbox / ChannelMail 已定义在 notification.go；此处补短信。
const (
	ChannelSMS string = "sms" // 短信
)

// 投递状态。skipped 与 failed 必须区分：
//   - skipped = 配置类问题（无可用渠道 / 适配器待接入），不重试；
//   - failed  = 网络/服务商问题，按退避重试，超限转 dead。
const (
	DeliveryStatusPending string = "pending" // 待发送
	DeliveryStatusSending string = "sending" // 发送中（已加锁）
	DeliveryStatusSent    string = "sent"    // 已发送
	DeliveryStatusFailed  string = "failed"  // 发送失败（可重试）
	DeliveryStatusSkipped string = "skipped" // 已跳过（配置类问题，不重试）
	DeliveryStatusDead    string = "dead"    // 死信（重试耗尽）
)

// 投递内容格式。
const (
	FormatText string = "text"
	FormatHTML string = "html"
)

// 投递来源模块（幂等唯一索引的一维）。
const (
	SourceModuleTest      string = "test"      // 后台测试发送
	SourceModuleBroadcast string = "broadcast" // 消息群发
)

// 默认最大尝试次数与锁定时间。
const (
	DefaultMaxAttempts = 5
	DeliveryLockTTL    = 2 * time.Minute
)

// NotificationDelivery 投递队列：一行 = 一个「模板事件 × 目标 × 通道」的投递任务。
type NotificationDelivery struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	BatchID       string     `gorm:"column:batch_id;size:40;not null;default:''"`
	Event         string     `gorm:"size:64;not null;index"`
	Channel       string     `gorm:"size:20;not null"`
	TargetType    string     `gorm:"column:target_type;size:10;not null;default:user"`
	TargetID      uint64     `gorm:"column:target_id;not null;default:0"`
	TargetName    string     `gorm:"column:target_name;size:128;not null;default:''"`
	Recipient     string     `gorm:"size:191;not null;default:''"`
	ChannelID     uint64     `gorm:"column:channel_id;not null;default:0"`
	Title         string     `gorm:"size:255;not null;default:''"`
	Content       string     `gorm:"type:text"`
	ContentFormat string     `gorm:"column:content_format;size:10;not null;default:text"`
	Vars          string     `gorm:"type:jsonb"`
	SendStatus    string     `gorm:"column:send_status;size:20;not null;default:pending"`
	Attempts      int        `gorm:"not null;default:0"`
	MaxAttempts   int        `gorm:"column:max_attempts;not null;default:5"`
	NextRetryAt   *time.Time `gorm:"column:next_retry_at"`
	LockedUntil   *time.Time `gorm:"column:locked_until"`
	ProviderMsgID string     `gorm:"column:provider_msg_id;size:128;not null;default:''"`
	ProviderCode  string     `gorm:"column:provider_code;size:64;not null;default:''"`
	CostFen       int        `gorm:"column:cost_fen;not null;default:0"`
	FailReason    string     `gorm:"column:fail_reason;size:500;not null;default:''"`
	SourceModule  string     `gorm:"column:source_module;size:32;not null;default:''"`
	SourceID      string     `gorm:"column:source_id;size:64;not null;default:''"`
	SentAt        *time.Time `gorm:"column:sent_at"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
}

func (NotificationDelivery) TableName() string { return "notification_deliveries" }
