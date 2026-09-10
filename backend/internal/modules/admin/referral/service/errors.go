package service

import "errors"

var (
	// ErrInsufficientBalance 返现可用余额不足（含为负的欠款场景）。
	ErrInsufficientBalance = errors.New("返现可用余额不足")
	// ErrInvalidAmount 金额非法（小于等于 0 或超过两位小数）。
	ErrInvalidAmount = errors.New("金额非法")
	// ErrBelowMinWithdraw 低于最低提现金额。
	ErrBelowMinWithdraw = errors.New("低于最低提现金额")
	// ErrWithdrawNotFound 提现单不存在。
	ErrWithdrawNotFound = errors.New("提现单不存在")
	// ErrWithdrawStatus 提现单状态不允许当前操作。
	ErrWithdrawStatus = errors.New("提现单状态不允许该操作")
	// ErrReferralDisabled 推广返现功能未开启。
	ErrReferralDisabled = errors.New("推广返现功能未开启")
)
