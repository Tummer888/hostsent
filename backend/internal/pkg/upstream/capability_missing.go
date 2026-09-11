package upstream

import (
	"context"
	"errors"
)

// ErrCapabilityMissing 上游平台确实不具备该能力（无法通过降级等价实现）。
// 调用方应显式提示运维，绝不静默假装完成（doc15 §6.3 / doc17 §4-T5.5）。
var ErrCapabilityMissing = errors.New("上游平台不支持该能力")

// MissingCapabilityError 携带缺失能力名的可读错误，便于后台直接展示原因。
type MissingCapabilityError struct {
	Capability string // 例如 suspend / unsuspend / terminate / renew
	Provider   string // 适配器类型，便于定位渠道
}

func (e *MissingCapabilityError) Error() string {
	if e.Provider != "" {
		return "上游平台（" + e.Provider + "）不支持" + capabilityLabel(e.Capability) + "操作"
	}
	return "上游平台不支持" + capabilityLabel(e.Capability) + "操作"
}

// Is 让 errors.Is(err, ErrCapabilityMissing) 成立，handler 层可统一映射错误码。
func (e *MissingCapabilityError) Is(target error) bool {
	return target == ErrCapabilityMissing
}

// capabilityLabel 能力名中文标签（提示文案用）。
func capabilityLabel(capability string) string {
	switch capability {
	case OpSuspend:
		return "暂停"
	case OpUnsuspend:
		return "恢复"
	case OpDestroy:
		return "销毁"
	case OpRenew:
		return "续费"
	default:
		return capability
	}
}

// MissingCapability 构造带能力名与渠道类型的缺失错误。
func MissingCapability(providerType, capability string) error {
	return &MissingCapabilityError{Capability: capability, Provider: providerType}
}

// RenewWithCapability 续费分派：仅当适配器实现 InstanceRenewal 时执行；
// 未实现（RenewMode=none 的自营链路）返回 MissingCapabilityError，由调用方
// 决定走「本地账期 + 平台延期」（链路 B）还是显式报错。
func RenewWithCapability(ctx context.Context, p Provider, req *RenewRequest) (*RenewResult, error) {
	if p == nil {
		return nil, ErrCapabilityMissing
	}
	r, ok := p.(InstanceRenewal)
	if !ok {
		return nil, MissingCapability(p.GetType(), OpRenew)
	}
	return r.RenewInstance(ctx, req)
}
