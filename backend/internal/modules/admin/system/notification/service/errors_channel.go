package service

import "errors"

// 通知渠道域 sentinel errors。
var (
	// ErrChannelNotFound 渠道实例不存在。
	ErrChannelNotFound = errors.New("通知渠道不存在")
	// ErrChannelCodeExists 渠道标识重复。
	ErrChannelCodeExists = errors.New("渠道标识已存在")
	// ErrChannelTypeUnknown 渠道类型未在注册表登记。
	ErrChannelTypeUnknown = errors.New("未知的通知渠道类型")
	// ErrChannelNotConfigured 该类别下没有可用渠道（worker 据此转 skipped，不重试）。
	ErrChannelNotConfigured = errors.New("未配置可用的通知渠道")
)

// 模板体系 sentinel errors。
var (
	// ErrSmsTemplateNotFound 短信模板不存在。
	ErrSmsTemplateNotFound = errors.New("短信模板不存在")
	// ErrSmsTemplateCodeExists 短信模板编码重复。
	ErrSmsTemplateCodeExists = errors.New("短信模板编码已存在")
	// ErrSmsTemplateInUse 短信模板被通知模板引用，不允许删除。
	ErrSmsTemplateInUse = errors.New("该短信模板已被通知模板引用，请先解除引用")
	// ErrTemplateVarNotFound 模板变量不存在。
	ErrTemplateVarNotFound = errors.New("模板变量不存在")
	// ErrTemplateVarKeyExists 模板变量名重复。
	ErrTemplateVarKeyExists = errors.New("模板变量名已存在")
	// ErrUnregisteredVar 模板正文使用了未注册的变量，错误信息带变量名。
	ErrUnregisteredVar = errors.New("模板变量未注册")
)

// 投递队列 sentinel errors。
var (
	// ErrDeliveryNotFound 投递记录不存在。
	ErrDeliveryNotFound = errors.New("投递记录不存在")
	// ErrDeliveryNotRetryable 只允许对 failed / dead / skipped 重投，sent 与 sending 不允许。
	ErrDeliveryNotRetryable = errors.New("该投递状态不允许重投")
	// ErrNoDeliveryForNotification 该通知记录没有关联的外发投递（纯站内信）。
	ErrNoDeliveryForNotification = errors.New("该通知没有关联的外发投递记录，无需重发")
)

// 群发 sentinel errors。
var (
	// ErrBroadcastTargetEmpty 目标解析命中 0 人。
	ErrBroadcastTargetEmpty = errors.New("目标人群为空")
	// ErrBroadcastTooMany 命中人数超过上限。
	ErrBroadcastTooMany = errors.New("命中人数超过单次群发上限")
	// ErrBroadcastChannelEmpty 未选择任何通道。
	ErrBroadcastChannelEmpty = errors.New("请至少选择一个发送通道")
	// ErrBroadcastSmsTemplateRequired 选择短信通道时必须指定短信模板。
	ErrBroadcastSmsTemplateRequired = errors.New("选择短信通道时必须指定短信模板")
)
