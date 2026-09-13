// Package notifier 定义跨模块「统一发送端口」：一条消息、一个渠道配置、一次发送。
//
// 设计（doc89 §2 铁律 3、doc90 §2、doc91 §5.1）：
//   - notifier 只管「发」，不管「排队」。投递队列（notification_deliveries）属消息中心，
//     验证码/OTP 走本端口直发，不经排队、不受用户偏好开关影响。
//   - 接口先于实现：doc91 阶段只有 mail 直发（directOTPSender），doc90 落地渠道表后
//     换成 ChannelResolver 包装的发送器，调用方代码不变。
//   - 未接入的服务商注册描述符 + 工厂但 Send 返回 ErrAdapterNotImplemented，
//     保证「渠道能建、能存、能测出待接入」，绝不是 500。
package notifier

import (
	"context"
	"errors"

	"hostsent/backend/internal/pkg/integration"
)

// 消息类别。
const (
	CategoryMail = "mail"
	CategorySMS  = "sms"
)

// 正文格式。
const (
	FormatText = "text"
	FormatHTML = "html"
)

// ErrAdapterNotImplemented 渠道类型已登记但发送实现尚未接入（四家短信占位 provider）。
// 上层据此返回明确业务错误码，UI 展示「待接入」，不得冒泡为 500。
var ErrAdapterNotImplemented = errors.New("notifier: 该渠道适配器尚未接入")

// ErrChannelDisabled 渠道被停用或未配置。
var ErrChannelDisabled = errors.New("notifier: 渠道未启用或未配置")

// ErrSMSNotConfigured doc91 阶段短信通道尚未落地（doc90 完成后由 ChannelResolver 接管）。
var ErrSMSNotConfigured = errors.New("notifier: 短信通道尚未接入")

// Message 一条待发送消息。与「投递记录」解耦：notifier 只管发，不管排队。
type Message struct {
	Category   string            // mail / sms
	Recipient  string            // 邮箱 / 手机号
	Subject    string            // 邮件主题（短信为空）
	Body       string            // 正文
	Format     string            // text / html
	TemplateID string            // 服务商侧模板号（短信）
	SignName   string            // 短信签名
	Vars       map[string]string // 模板变量（走模板发送时使用）

	SourceModule string
	SourceID     string
}

// Result 发送结果。
type Result struct {
	ProviderMsgID string
	ProviderCode  string
	CostFen       int
}

// ChannelConfig 已解密、可直接使用的渠道配置。
type ChannelConfig struct {
	ChannelID    uint64
	ChannelCode  string
	Type         string
	Category     string
	Endpoint     string
	SignName     string
	Sender       string
	TemplateCode string
	Credentials  map[string]string // 已解密明文
}

// Sender 单一通道发送能力。
type Sender interface {
	// Category 通道类别：mail / sms。
	Category() string
	// Type 通道类型：smtp / aliyun_sms / ...
	Type() string
	// Send 发送一条消息；未接入返回 ErrAdapterNotImplemented。
	Send(ctx context.Context, cfg ChannelConfig, msg Message) (*Result, error)
	// Test 测试发送，只验证配置连通性。
	Test(ctx context.Context, cfg ChannelConfig, target string) error
}

// Descriptor 通道能力描述符：驱动后台「类型选择 → 动态凭证表单」。
// 与 payment.CapabilityDescriptor 同构，但只保留通知域需要的字段。
type CapabilityDescriptor struct {
	// Type 类型标识：smtp / aliyun_sms / ...
	Type string `json:"type"`
	// Name 中文名：SMTP 邮件 / 阿里云短信。
	Name string `json:"name"`
	// Category mail / sms。
	Category string `json:"category"`
	// Mode api / manual。
	Mode string `json:"mode"`
	// CredentialSchema 凭证字段（驱动动态表单 + 字段级加密）。
	CredentialSchema []integration.Field `json:"credential_schema"`
	// EndpointSchema 端点/连接参数字段。
	EndpointSchema []integration.Field `json:"endpoint_schema"`
	// Icon 图标标识（前端映射）。
	Icon string `json:"icon"`
	// DocURL 官方文档地址（后台帮助入口）。
	DocURL string `json:"doc_url"`
	// AdapterVersion 适配器版本。
	AdapterVersion string `json:"adapter_version"`
	// Implemented 适配器是否已注册真实实现（运行时计算，非落库）。
	Implemented bool `json:"implemented"`
}
