// Package service 提供生命周期模块的业务编排。
package service

import "errors"

// 业务 sentinel 错误（handler 层统一映射错误码）
var (
	ErrInstanceNotFound    = errors.New("实例不存在")          // 20002
	ErrRenewalNotFound     = errors.New("续费记录不存在")        // 20002
	ErrStatusNotAllowed    = errors.New("当前生命周期状态不允许该操作") // 20003
	ErrInsufficientBalance = errors.New("余额不足")           // 30001
	ErrInvalidPeriod       = errors.New("续费周期数不合法")       // 20001
	ErrPermissionDenied    = errors.New("无权操作该实例")        // 20003
	ErrPolicyInvalid       = errors.New("生命周期策略参数不合法")    // 20001
	// ErrStageNotActionable 实例当前无需阶段动作（未到期或阶段已一致）。// 20003
	ErrStageNotActionable = errors.New("该实例当前无需阶段处置")
	// ErrCapabilityUnsupported 上游不具备该阶段动作能力（仅标记，需人工跟进）。// 20004
	ErrCapabilityUnsupported = errors.New("上游不支持该操作，已仅标记待人工跟进")
)
