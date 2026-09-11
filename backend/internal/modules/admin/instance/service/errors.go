package service

import "errors"

// 实例运维台哨兵错误，由 handler 统一映射为业务错误码。
var (
	// ErrInstanceNotFound 实例不存在。
	ErrInstanceNotFound = errors.New("实例不存在")
	// ErrInvalidAction 电源动作非法。
	ErrInvalidAction = errors.New("不支持的电源操作")
	// ErrCapabilityUnsupported 上游服务商不支持该操作。
	ErrCapabilityUnsupported = errors.New("该服务商不支持此操作")
	// ErrPermissionDenied 高风险操作未满足前置条件（如销毁二次确认不匹配）。
	ErrPermissionDenied = errors.New("操作未通过安全校验")
	// ErrStatusConflict 当前状态不允许该操作。
	ErrStatusConflict = errors.New("当前实例状态不允许该操作")
	// ErrProviderUnavailable 上游服务商配置不可用。
	ErrProviderUnavailable = errors.New("上游服务商不可用")
)
