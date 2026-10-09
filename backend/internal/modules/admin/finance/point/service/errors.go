package service

import "errors"

// 积分子域业务错误。
var (
	// ErrRuleNotFound 规则不存在。
	ErrRuleNotFound = errors.New("积分规则不存在")
	// ErrRuleCodeExists 规则编码重复。
	ErrRuleCodeExists = errors.New("积分规则编码已存在")
	// ErrInvalidRule 规则参数不合法。
	ErrInvalidRule = errors.New("积分规则参数不合法")
	// ErrInvalidPoints 积分调整数值不合法（0 或为空）。
	ErrInvalidPoints = errors.New("调整积分不能为 0")
	// ErrInsufficientPoints 积分不足（扣减超过可用余额）。
	ErrInsufficientPoints = errors.New("积分不足")
	// ErrAccountNotFound 积分账户不存在。
	ErrAccountNotFound = errors.New("积分账户不存在")
)
