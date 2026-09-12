package service

import "errors"

// 提现子域业务错误
var (
	// ErrWithdrawNotFound 提现单不存在
	ErrWithdrawNotFound = errors.New("提现单不存在")
	// ErrWithdrawChannelInvalid 收款渠道不支持
	ErrWithdrawChannelInvalid = errors.New("不支持的收款渠道（仅支持 bank/alipay）")
	// ErrWithdrawAccountRequired 收款账户信息不完整
	ErrWithdrawAccountRequired = errors.New("请填写完整的收款账户信息")
)
