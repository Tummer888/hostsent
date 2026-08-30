package service

import (
	"context"

	"hostsent/backend/internal/modules/admin/order/model"
)

// ProvisionAdapter 定义订单履约能力：支付成功后触发资源开通（幂等）。
type ProvisionAdapter interface {
	// Activate 推进订单开通状态；已是服务中则幂等返回，非法状态返回错误。
	Activate(ctx context.Context, order *model.Order) error
}

// DefaultProvisionAdapter 默认履约实现：仅推进订单状态（paid → provisioning → active）。
// 现阶段为轻量起步，后续可在此接入上游 SDK 异步创建实例。
type DefaultProvisionAdapter struct{}

// NewDefaultProvisionAdapter 创建默认履约适配器。
func NewDefaultProvisionAdapter() ProvisionAdapter {
	return &DefaultProvisionAdapter{}
}

// Activate 实现幂等开通：paid → provisioning → active。
func (a *DefaultProvisionAdapter) Activate(ctx context.Context, order *model.Order) error {
	switch order.Status {
	case model.OrderStatusPaid:
		if err := EnsureStatus(order.Status, model.OrderStatusProvisioning); err != nil {
			return err
		}
		order.Status = model.OrderStatusProvisioning
		return nil
	case model.OrderStatusProvisioning:
		if err := EnsureStatus(order.Status, model.OrderStatusActive); err != nil {
			return err
		}
		order.Status = model.OrderStatusActive
		return nil
	case model.OrderStatusActive:
		return nil // 已是服务中，幂等返回
	default:
		return ErrStatusConflict
	}
}
