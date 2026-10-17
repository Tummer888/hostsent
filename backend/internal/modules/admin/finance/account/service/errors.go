package service

import "errors"

// 钱包账户子域业务错误
var (
	// ErrInvalidAmount 金额非法（≤0）：此前复用「余额不足」，让「调负数」看起来像余额问题（doc34 F-17）。
	ErrInvalidAmount = errors.New("金额必须大于 0")
	// ErrInvalidDirection 收支方向非法（既非 1 也非 -1）：此前复用「状态不允许该操作」。
	ErrInvalidDirection = errors.New("收支方向非法")
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
	// ErrAdjustDisabled 人工调账已被财务配置关闭（finance.adjust_enabled=false）
	ErrAdjustDisabled = errors.New("人工调账已关闭，请在财务配置中开启")
	// ErrSubAccountWalletDenied 子账号没有独立钱包：财务主体是主账号。
	//
	// 所有消费路径（下单/续费）都按 EffectiveUserID 取主账号，子账号钱包里的钱
	// 既花不掉也无法提现 —— 给它建钱包只会产生一笔财务无法归集的孤儿钱。
	ErrSubAccountWalletDenied = errors.New("子账号不支持独立钱包，资金操作请对主账号执行")
)
