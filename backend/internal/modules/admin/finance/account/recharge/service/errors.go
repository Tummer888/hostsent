package service

import "errors"

// 充值子域业务错误
var (
	// ErrRechargeNotFound 充值单不存在
	ErrRechargeNotFound = errors.New("充值单不存在")
)
