package service

import "errors"

// 提现子域业务错误
var (
	// ErrWithdrawNotFound 提现单不存在
	ErrWithdrawNotFound = errors.New("提现单不存在")
)
