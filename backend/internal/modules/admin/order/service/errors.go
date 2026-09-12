package service

import "errors"

// 订单域业务错误
var (
	// ErrStatusConflict 订单/退款状态不允许该操作
	ErrStatusConflict = errors.New("订单状态不允许该操作")
	// ErrOrderNotFound 订单不存在
	ErrOrderNotFound = errors.New("订单不存在")
	// ErrRefundNotFound 退款单不存在
	ErrRefundNotFound = errors.New("退款单不存在")
	// ErrRefundExceeded 退款金额超过可退金额
	ErrRefundExceeded = errors.New("退款金额超过可退金额")
	// ErrRefundModeInvalid 退款去向非法（仅支持 balance / channel，doc36 §3.2）
	ErrRefundModeInvalid = errors.New("退款去向非法，仅支持 balance（退回余额）或 channel（原路退回）")
	// ErrRefundFeeInvalid 渠道扣点非法（不得为负或超过退款本金）
	ErrRefundFeeInvalid = errors.New("渠道扣点不得为负或超过退款本金")
)
