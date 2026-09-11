package upstream

import "context"

// 本文件是 T5.5「暂停/销毁按链路与能力分派」的单一落点。
//
// 两条链路的能力差异最终都收敛为「适配器是否实现某能力接口」：
//   - 链路 A（上游转售，mofangfinance）：暂停/恢复走 /provision/default，终止走 /host/cancel；
//   - 链路 B（自营，mofangyun）：暂停/恢复走 /clouds/{id}/suspend|unsuspend，销毁走 DELETE /clouds/{id}。
//
// 分派层保证：只要平台提供了等价能力就不阻断业务；确实缺失时返回 ErrCapabilityMissing，
// 由调用方显式提示运维（绝不静默假装完成）。降级只在语义等价时发生：
// 暂停→关机（StopInstance）、恢复→开机（StartInstance）、终止→删除（DeleteInstance）。

// SuspendWithFallback 暂停实例：优先 InstanceSuspension（平台暂停态），
// 退化到 InstanceControl.StopInstance（关机），都没有则 ErrCapabilityMissing。
func SuspendWithFallback(ctx context.Context, p Provider, instanceID, reason string) error {
	if p == nil {
		return ErrCapabilityMissing
	}
	if s, ok := p.(InstanceSuspension); ok {
		return s.SuspendInstance(ctx, instanceID, reason)
	}
	if c, ok := p.(InstanceControl); ok {
		return c.StopInstance(ctx, instanceID, false)
	}
	return ErrCapabilityMissing
}

// UnsuspendWithFallback 恢复实例：优先 InstanceSuspension.UnsuspendInstance，
// 退化到 InstanceControl.StartInstance，都没有则 ErrCapabilityMissing。
func UnsuspendWithFallback(ctx context.Context, p Provider, instanceID string) error {
	if p == nil {
		return ErrCapabilityMissing
	}
	if s, ok := p.(InstanceSuspension); ok {
		return s.UnsuspendInstance(ctx, instanceID)
	}
	if c, ok := p.(InstanceControl); ok {
		return c.StartInstance(ctx, instanceID)
	}
	return ErrCapabilityMissing
}

// TerminateWithFallback 终止实例：优先 InstanceTermination（语义最窄的"终止"），
// 退化到 InstanceAdministration.DeleteInstance（物理删除），都没有则 ErrCapabilityMissing。
func TerminateWithFallback(ctx context.Context, p Provider, instanceID, reason string) error {
	if p == nil {
		return ErrCapabilityMissing
	}
	if t, ok := p.(InstanceTermination); ok {
		return t.TerminateInstance(ctx, instanceID, reason)
	}
	if a, ok := p.(InstanceAdministration); ok {
		return a.DeleteInstance(ctx, instanceID)
	}
	return ErrCapabilityMissing
}
