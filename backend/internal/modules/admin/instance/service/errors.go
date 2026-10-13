package service

import "errors"

// 实例运维台哨兵错误，由 handler 统一映射为业务错误码。
var (
	// ErrInstanceNotFound 实例不存在。
	ErrInstanceNotFound = errors.New("实例不存在")
	// ErrInvalidAction 操作入参非法（通用，具体原因由 fmt.Errorf("%w: ...") 补充）。
	//
	// 措辞刻意保持中性：本哨兵被电源/重装/重置密码/救援/快照/账期/重新关联等
	// 十几处复用，旧文案"不支持的电源操作"会让"商品不存在"这类错误
	// 也报成电源问题（实测困惑）。
	ErrInvalidAction = errors.New("操作入参不合法")
	// ErrBatchDestroyInvalid 批量销毁入参非法（空列表或超出单次上限）。
	//
	// 单独一条而非复用 ErrInvalidAction：批量销毁参数有问题时，
	// 用它自己的措辞比通用"入参不合法"更能指明该改哪里。
	ErrBatchDestroyInvalid = errors.New("批量销毁参数不合法")
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
	// ErrNothingToSync 实例尚未在本地完成开通，同步接口专用提示。
	//
	// 措辞说明：这条提示现在随"已自动重投核对"一起返回（上游可能已开通，本地只是
	// 还没拿到实例号），因此不再断言"上游没有这台机器"，只说明核对已发起。
	ErrNothingToSync = errors.New("实例尚未完成开通：已向上游发起核对，若上游已有本单机器将自动接管，请稍后刷新；若上游确认未开通，可点「重新开通」重试（失败原因见「来源订单」）")
)
