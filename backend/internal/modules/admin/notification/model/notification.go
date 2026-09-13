// Package model 提供通知与消息中心模块的数据模型。
package model

import "time"

// 通知事件类型（与模板 event 对应）
const (
	EventOrderPaid           string = "order_paid"            // 订单支付成功
	EventRenewalSuccess      string = "renewal_success"       // 续费成功
	EventRenewalFailed       string = "renewal_failed"        // 续费失败
	EventInstanceExpiring    string = "instance_expiring"     // 实例即将到期
	EventTicketReplied       string = "ticket_replied"        // 工单新回复（通知用户）
	EventTicketAssigned      string = "ticket_assigned"       // 工单指派（通知被指派员工，P2-04）
	EventTicketStatus        string = "ticket_status"         // 工单状态变更（通知用户，P2-04）
	EventTicketReviewPending string = "ticket_review_pending" // 工单回复待复核（通知复核人，S3）
	EventTicketReviewResult  string = "ticket_review_result"  // 工单回复复核结果（通知提交客服，S3）
	EventTicketTransferred   string = "ticket_transferred"    // 工单转交专人（通知用户，S2）
	EventBalanceLow          string = "balance_low"           // 余额不足预警
	EventSyncFailed          string = "sync_failed"           // 上游同步失败（管理员）
	EventSystem              string = "system"                // 系统通用
)

// 投递通道
const (
	ChannelInbox string = "inbox" // 站内信
	ChannelMail  string = "mail"  // 邮件
)

// 发送状态
const (
	SendStatusPending string = "pending" // 待发送
	SendStatusSent    string = "sent"    // 已发送
	SendStatusFailed  string = "failed"  // 发送失败（仅外发通道）
)

// 目标类型
const (
	TargetUser  string = "user"  // 用户
	TargetAdmin string = "admin" // 管理员
)

// 公告状态
const (
	AnnouncementDraft     string = "draft"     // 草稿
	AnnouncementPublished string = "published" // 已发布
	AnnouncementOffline   string = "offline"   // 已下线
)

// 公告平台
const (
	AnnouncementPlatformUser  string = "user"  // 用户端
	AnnouncementPlatformAdmin string = "admin" // 管理端
	AnnouncementPlatformBoth  string = "both"  // 双端
)

// 公告级别
const (
	AnnouncementLevelInfo     string = "info"     // 普通
	AnnouncementLevelWarning  string = "warning"  // 警告
	AnnouncementLevelCritical string = "critical" // 紧急
)

// 模板状态
const (
	TemplateStatusActive   string = "active"   // 启用
	TemplateStatusDisabled string = "disabled" // 禁用
)

// Notification 站内通知记录（同时承载外发通道的结果）。
type Notification struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64     `gorm:"column:user_id;index;not null;default:0" json:"user_id"`              // 0=全员广播（配合 reads 表）
	TargetType   string     `gorm:"column:target_type;size:10;not null;default:user" json:"target_type"` // user / admin
	Event        string     `gorm:"column:event;size:64;not null;index" json:"event"`
	Title        string     `gorm:"column:title;size:255;not null" json:"title"`
	Content      string     `gorm:"column:content;type:text" json:"content"`
	Channel      string     `gorm:"column:channel;size:20;not null;default:inbox" json:"channel"` // inbox / mail
	SendStatus   string     `gorm:"column:send_status;size:20;not null;default:sent" json:"send_status"`
	FailReason   string     `gorm:"column:fail_reason;size:255" json:"fail_reason"`
	SourceModule string     `gorm:"column:source_module;size:32" json:"source_module"`
	SourceID     string     `gorm:"column:source_id;size:64" json:"source_id"`
	ReadAt       *time.Time `gorm:"column:read_at" json:"read_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime;index" json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

// Announcement 公告。
type Announcement struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Title      string     `gorm:"column:title;size:255;not null" json:"title"`
	Content    string     `gorm:"column:content;type:text;not null" json:"content"`
	Platform   string     `gorm:"column:platform;size:10;not null;default:user" json:"platform"` // user / admin / both
	Level      string     `gorm:"column:level;size:20;not null;default:info" json:"level"`       // info / warning / critical
	Popup      bool       `gorm:"column:popup;not null;default:false" json:"popup"`
	Status     string     `gorm:"size:20;not null;default:draft;index" json:"status"`
	PublishAt  *time.Time `gorm:"column:publish_at" json:"publish_at"`
	OfflineAt  *time.Time `gorm:"column:offline_at" json:"offline_at"`
	OperatorID uint64     `gorm:"column:operator_id" json:"operator_id"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Announcement) TableName() string { return "announcements" }

// NotificationTemplate 通知模板。
type NotificationTemplate struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Event      string    `gorm:"column:event;size:64;not null;uniqueIndex" json:"event"`
	TitleTpl   string    `gorm:"column:title_tpl;size:255;not null" json:"title_tpl"`
	ContentTpl string    `gorm:"column:content_tpl;type:text" json:"content_tpl"`
	InboxOn    bool      `gorm:"column:inbox_on;not null;default:true" json:"inbox_on"`
	MailOn     bool      `gorm:"column:mail_on;not null;default:false" json:"mail_on"`
	Status     string    `gorm:"size:20;not null;default:active" json:"status"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (NotificationTemplate) TableName() string { return "notification_templates" }

// NotificationRead 全员广播的已读记录。
type NotificationRead struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	NotificationID uint64    `gorm:"column:notification_id;uniqueIndex:uk_read,priority:1;index"`
	UserID         uint64    `gorm:"column:user_id;uniqueIndex:uk_read,priority:2"`
	ReadAt         time.Time `gorm:"autoCreateTime"`
}

func (NotificationRead) TableName() string { return "notification_reads" }

// NotificationPreference 用户通知偏好。
type NotificationPreference struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"column:user_id;uniqueIndex:uk_pref,priority:1;not null"`
	Event     string    `gorm:"column:event;size:64;uniqueIndex:uk_pref,priority:2;not null"`
	InboxOn   bool      `gorm:"column:inbox_on;not null;default:true"`
	MailOn    bool      `gorm:"column:mail_on;not null;default:false"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (NotificationPreference) TableName() string { return "notification_preferences" }
