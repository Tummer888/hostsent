package service

import "errors"

var (
	ErrInstanceNotFoundOrDenied = errors.New("主机不存在或无权访问")
	// ErrDestroyUnsupported 销毁执行者未装配（部署裁剪时）。
	ErrDestroyUnsupported = errors.New("当前环境不支持自助销毁，请联系客服")
	// ErrCapabilityUnsupported 上游服务商不支持该操作（admin/instance/service 同名
	// 哨兵的本地面，装配层映射时统一转换，避免 uc import admin service）。
	ErrCapabilityUnsupported     = errors.New("该服务商不支持此操作")
	ErrMaintenanceUnsupported    = errors.New("当前环境不支持该操作，请联系客服")
	ErrInstanceNotProvisioned    = errors.New("实例尚未开通（待开通状态），请先点「重新开通」")
	errUnsupportedControl        = errors.New("该主机暂不支持电源与控制台操作，请联系客服")
	errProviderUnavailable       = errors.New("服务暂时不可用，请稍后重试")
	ErrProvisionRetryUnavailable = errors.New("重新开通暂不可用，请联系客服")
	ErrAlreadyProvisioned        = errors.New("实例已开通，无需重新开通")
	ErrMetricsUnsupported        = errors.New("当前服务商不提供性能监控")
	ErrMetricsTypeRequired       = errors.New("请指定要查询的监控类型")
	// ErrOperationInProgress 该实例上有正在执行的操作（迁移 112 过渡态）：
	// 拒绝新的电源/维护指令，避免客户连点造成任务累积（用户实测反馈）。
	//
	// 文案面向客户：说明"在做什么、要等多久、等不了怎么办"，不暴露内部机制。
	ErrOperationInProgress = errors.New("该主机上已有正在执行的操作")
)
