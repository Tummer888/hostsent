package model

import (
	"time"

	"gorm.io/gorm"
)

// 渠道类别（与 notifier.Category* 同值）。
const (
	ChannelCategoryMail string = "mail"
	ChannelCategorySMS  string = "sms"
)

// 渠道健康状态。
const (
	HealthHealthy string = "healthy"
	HealthDown    string = "down"
	HealthPending string = "pending" // 适配器待接入（占位 provider）
)

// 渠道场景（notification_channels.scenes 数组取值）。
const (
	ChannelSceneOTP       string = "otp"       // 验证码（强制送达）
	ChannelSceneNotify    string = "notify"    // 业务通知
	ChannelSceneAlert     string = "alert"     // 告警
	ChannelSceneMarketing string = "marketing" // 营销
	ChannelSceneTest      string = "test"      // 测试发送
)

// 渠道状态。
const (
	ChannelStatusEnabled  int = 1
	ChannelStatusDisabled int = 0
)

// NotificationChannelType 渠道类型注册表：一行 = 一种可接入的通道类型。
// Descriptor 存 CapabilityDescriptor（凭证 schema），前端据此渲染动态表单。
type NotificationChannelType struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	Type           string    `gorm:"column:type;size:50;not null;uniqueIndex:uk_notify_channel_types_type"`
	Name           string    `gorm:"size:100;not null"`
	Category       string    `gorm:"size:20;not null;index"`
	Mode           string    `gorm:"size:20;not null;default:api"`
	DescriptorJSON string    `gorm:"column:descriptor;type:jsonb"`
	Icon           string    `gorm:"size:64;not null;default:''"`
	DocURL         string    `gorm:"column:doc_url;size:255;not null;default:''"`
	AdapterVersion string    `gorm:"column:adapter_version;size:32;not null;default:''"`
	SortOrder      int       `gorm:"column:sort_order;not null;default:0"`
	Status         int       `gorm:"not null;default:1"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

func (NotificationChannelType) TableName() string { return "notification_channel_types" }

// NotificationChannel 渠道实例：同一类型可有多个实例（不同签名/发件人/额度）。
// Credentials 为字段级加密后的凭证 JSON（复用 pkg/credentials，enc:v1: 前缀）。
type NotificationChannel struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement"`
	ChannelCode  string         `gorm:"column:channel_code;size:64;not null;uniqueIndex:uk_notify_channels_code"`
	Name         string         `gorm:"size:100;not null"`
	Category     string         `gorm:"size:20;not null;index"`
	Type         string         `gorm:"size:50;not null;index"`
	Credentials  string         `gorm:"column:credentials;type:jsonb"`
	Endpoint     string         `gorm:"size:255;not null;default:''"`
	SignName     string         `gorm:"column:sign_name;size:64;not null;default:''"`
	Sender       string         `gorm:"size:128;not null;default:''"`
	TemplateCode string         `gorm:"column:template_code;size:64;not null;default:''"`
	Scenes       string         `gorm:"type:jsonb"` // 启用场景数组
	Priority     int            `gorm:"not null;default:0"`
	Weight       int            `gorm:"not null;default:0"`
	DailyLimit   int            `gorm:"column:daily_limit;not null;default:0"` // 0=不限
	HealthStatus string         `gorm:"column:health_status;size:20;not null;default:''"`
	LastError    string         `gorm:"column:last_error;type:text"`
	LastCheckAt  *time.Time     `gorm:"column:last_check_at"`
	Status       int            `gorm:"not null;default:1;index"`
	IsDefault    bool           `gorm:"column:is_default;not null;default:false"`
	Remark       string         `gorm:"size:255;not null;default:''"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (NotificationChannel) TableName() string { return "notification_channels" }

// 短信模板状态。
const (
	SmsTemplateStatusActive   string = "active"
	SmsTemplateStatusDisabled string = "disabled"
)

// 模板变量状态。
const (
	TemplateVarStatusActive   string = "active"
	TemplateVarStatusDisabled string = "disabled"
)

// SmsTemplate 短信模板。Content 含 {var} 占位符，保存时由后端解析并回写 VarNames。
type SmsTemplate struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	Code         string    `gorm:"size:64;not null;uniqueIndex:uk_sms_templates_code"`
	Name         string    `gorm:"size:100;not null"`
	Scene        string    `gorm:"size:32;not null;default:notify;index"`
	Content      string    `gorm:"size:1000;not null"`
	UpstreamCode string    `gorm:"column:upstream_code;size:64;not null;default:''"`
	VarNames     string    `gorm:"column:var_names;type:jsonb"`
	Status       string    `gorm:"size:20;not null;default:active"`
	Remark       string    `gorm:"size:255;not null;default:''"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (SmsTemplate) TableName() string { return "sms_templates" }

// SmsTemplateVar 模板变量注册表（D7）：变量可枚举、可选、可校验。
type SmsTemplateVar struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	VarKey      string    `gorm:"column:var_key;size:64;not null;uniqueIndex:uk_sms_template_vars_key"`
	Label       string    `gorm:"size:64;not null"`
	Category    string    `gorm:"size:32;not null;default:instance"`
	ValueType   string    `gorm:"column:value_type;size:20;not null;default:string"`
	Sample      string    `gorm:"size:255;not null;default:''"`
	Description string    `gorm:"size:255;not null;default:''"`
	Scenes      string    `gorm:"type:jsonb"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0"`
	Status      string    `gorm:"size:20;not null;default:active"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (SmsTemplateVar) TableName() string { return "sms_template_vars" }
