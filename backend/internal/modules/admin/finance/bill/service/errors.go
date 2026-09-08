package service

import "errors"

// 账单子域业务错误
var (
	// ErrStatusConflict 状态不允许该操作
	ErrStatusConflict = errors.New("状态不允许该操作")
	// ErrBillNotFound 账单不存在
	ErrBillNotFound = errors.New("账单不存在")
)
