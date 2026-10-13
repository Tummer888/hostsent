package service

import "errors"

// 实例运维台哨兵错误，由 handler 统一映射为业务错误码。
var (
	// ErrInstanceNotFound 实例不存在。
	ErrInstanceNotFound = errors.New("实例不存在")
	// ErrInvalidAction 电源动作非法。
	ErrInvalidAction = errors.New("不支持的电源操作")
	// ErrBillingAdjustInvalid 账期/付款周期调整参数非法（错误提示由业务层补充）。
	ErrBillingAdjustInvalid = errors.New("账期调整参数不合法")
	// ErrCredentialUnavailable 实例未保存明文凭据（需改用重置密码）。
	ErrCredentialUnavailable = errors.New("实例未保存明文密码")
	// ErrCapabilityUnsupported 上游服务商不支持该操作。
	ErrCapabilityUnsupported = errors.New("该服务商不支持此操作")
	// ErrPermissionDenied 高风险操作未满足前置条件（如销毁二次确认不匹配）。
	ErrPermissionDenied = errors.New("操作未通过安全校验")
	// ErrStatusConflict 当前状态不允许该操作。
	ErrStatusConflict = errors.New("当前实例状态不允许该操作")
	// ErrProviderUnavailable 上游服务商配置不可用。
	ErrProviderUnavailable = errors.New("上游服务商不可用")
	// ErrInstanceNotProvisioned 实例尚未在上游开通（待开通占位行，provider_instance_id 为空）：
	// 电源/维护/变配等上游操作无从下手，应先「重新开通」或等待开通完成。
	ErrInstanceNotProvisioned = errors.New("实例尚未开通（待开通状态），请先点「重新开通」")
	// ErrProvisionRetryUnavailable 重新开通入口未装配（装配层未注入）。
	ErrProvisionRetryUnavailable = errors.New("重新开通暂不可用")
	// ErrNothingToSync 实例尚未开通，上游没有可拉取的对象（同步接口专用提示）。
	ErrNothingToSync = errors.New("实例尚未开通，上游还没有这台机器，暂无可拉取的信息；开通失败原因见「来源订单」，可点「重新开通」重试")
)
