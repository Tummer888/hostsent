package service

import "errors"

// 财务域业务错误
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
	// ErrRechargeNotFound 充值单不存在
	ErrRechargeNotFound = errors.New("充值单不存在")
	// ErrWithdrawNotFound 提现单不存在
	ErrWithdrawNotFound = errors.New("提现单不存在")
	// ErrBillNotFound 账单不存在
	ErrBillNotFound = errors.New("账单不存在")
)
