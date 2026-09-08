package service

import "errors"

// 钱包账户子域业务错误
var (
	// ErrInsufficientBalance 余额不足
	ErrInsufficientBalance = errors.New("余额不足")
	// ErrFrozenInsufficient 冻结余额不足
	ErrFrozenInsufficient = errors.New("冻结余额不足")
	// ErrStatusConflict 状态不允许该操作
	ErrStatusConflict = errors.New("状态不允许该操作")
	// ErrBizExist 重复记账（幂等冲突）
	ErrBizExist = errors.New("重复记账")
	// ErrWalletNotFound 钱包不存在
	ErrWalletNotFound = errors.New("钱包不存在")
)
