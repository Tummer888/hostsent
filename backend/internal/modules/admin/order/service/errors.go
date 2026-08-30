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
)
