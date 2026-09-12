package service

import "errors"

// 支付中心业务错误。
var (
	// ErrChannelNotFound 渠道不存在。
	ErrChannelNotFound = errors.New("支付渠道不存在")
	// ErrChannelCodeExists 渠道编码已存在。
	ErrChannelCodeExists = errors.New("渠道编码已存在")
	// ErrChannelTypeUnknown 渠道类型未注册或无适配器。
	ErrChannelTypeUnknown = errors.New("支付渠道类型未注册")
	// ErrOrderNotFound 支付单不存在。
	ErrOrderNotFound = errors.New("支付单不存在")
	// ErrOrderNotPayable 支付单当前状态不可支付。
	ErrOrderNotPayable = errors.New("支付单当前状态不可支付")
	// ErrNoChannelAvailable 无可用支付渠道。
	ErrNoChannelAvailable = errors.New("无可用支付渠道")
	// ErrPayoutNotFound 打款单不存在。
	ErrPayoutNotFound = errors.New("打款单不存在")
	// ErrPayoutStatusConflict 打款单状态不允许该操作。
	ErrPayoutStatusConflict = errors.New("打款单当前状态不允许该操作")
	// ErrRefundAmountExceeded 退款额超过支付单可退金额。
	ErrRefundAmountExceeded = errors.New("退款金额超过可退金额")
	// ErrRefundNotSupported 渠道不支持退款。
	ErrRefundNotSupported = errors.New("该支付渠道不支持退款")
	// ErrPayoutNotSupported 渠道不支持打款，请使用人工打款。
	ErrPayoutNotSupported = errors.New("该支付渠道不支持接口打款，请使用人工打款")
	// ErrAccountNotFound 收款账户不存在。
	ErrAccountNotFound = errors.New("收款账户不存在")
	// ErrBalanceInsufficient 余额不足。
	ErrBalanceInsufficient = errors.New("余额不足")
)
