package service

import (
	"context"
	"fmt"
	"reflect"

	"hostsent/backend/internal/modules/admin/order/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// ProvisionDeps 上游开通适配器依赖集合（由装配层注入，避免订单包反向依赖产品/资源包）。
type ProvisionDeps struct {
	// BuildProvisionRequest 按商品解析上游开通请求（product/catalog 服务实现）。
	// specSnapshot 为下单时落库的规格快照（SKU 原子取值 JSON，T4.1），可为空；
	// specCode 为所选 SKU 编码（T4.2），用于回查已确认的平台绑定参数，可为空。
	BuildProvisionRequest func(ctx context.Context, productID uint64, name, specSnapshot, specCode string) (interface{}, error)
	// BuildProviderConfig 按提供商 ID 构建适配器配置（含解密密钥与提供商类型）。
	BuildProviderConfig func(ctx context.Context, providerID uint64) (*upstream.ProviderConfig, error)
	// CreateInstance 创建上游实例：由装配层用 cfg 实例化 provider 并调用。
	CreateInstance func(ctx context.Context, cfg *upstream.ProviderConfig, req *pkgmodel.CreateInstanceRequest) (*pkgmodel.StandardInstance, error)
	// RecordInstance 开通成功后记录实例（落库 instances）。
	RecordInstance func(ctx context.Context, inst *pkgmodel.StandardInstance, order *model.Order) error
	// HasInstance 判断某用户是否已开通某商品（幂等，避免重复开通）。
	HasInstance func(ctx context.Context, userID, productID uint64) (bool, error)
}

// UpstreamProvisionAdapter 真实上游履约：订单支付后调用上游创建实例并记录，
// 打通「订单——上游」联动。若商品未绑定上游提供商（如虚拟商品/纯本地），
// 则回退到仅推进订单状态（与 DefaultProvisionAdapter 行为一致）。
type UpstreamProvisionAdapter struct {
	deps ProvisionDeps
}

// NewUpstreamProvisionAdapter 创建上游履约适配器。
func NewUpstreamProvisionAdapter(deps ProvisionDeps) *UpstreamProvisionAdapter {
	return &UpstreamProvisionAdapter{deps: deps}
}

// Activate 推进订单开通并触发上游创建实例（幂等）。
// 状态机（遵守 transitionTable：paid → provisioning → active）：
//   - 已支付(paid)：触发上游创建实例并记录 → 置 provisioning；失败返回错误保持 paid 可重试；
//   - 开通中(provisioning)：若实例已记录 → 置 active；否则视为重试上报一次 → active；
//   - 未绑定上游商品 → 回退仅推进状态。
func (a *UpstreamProvisionAdapter) Activate(ctx context.Context, order *model.Order) error {
	switch order.Status {
	case model.OrderStatusActive:
		return nil // 已是服务中，幂等返回
	case model.OrderStatusPaid:
		if order.ProductID == 0 {
			return a.advanceLegacy(order)
		}
		inst, err := a.provision(ctx, order)
		if err != nil {
			return err // 保持 paid，交由重试
		}
		if inst == nil {
			return a.advanceLegacy(order)
		}
		_ = a.deps.RecordInstance(ctx, inst, order)
		if err := EnsureStatus(order.Status, model.OrderStatusProvisioning); err != nil {
			return err
		}
		order.Status = model.OrderStatusProvisioning
		// 上游主机已就绪（同步开通）：直接推进到服务中，避免订单停留在 provisioning 等待人工重试。
		if inst.Status == pkgmodel.InstanceStatusRunning {
			return a.markActive(order)
		}
		return nil
	case model.OrderStatusProvisioning:
		if order.ProductID == 0 {
			return a.advanceLegacy(order)
		}
		// 幂等：已记录实例则直接激活，避免重复开通。
		if a.deps.HasInstance != nil {
			exists, err := a.deps.HasInstance(ctx, order.UserID, order.ProductID)
			if err == nil && exists {
				return a.markActive(order)
			}
		}
		inst, err := a.provision(ctx, order)
		if err != nil {
			return err
		}
		if inst == nil {
			return a.markActive(order)
		}
		_ = a.deps.RecordInstance(ctx, inst, order)
		return a.markActive(order)
	default:
		return ErrStatusConflict
	}
}

// markActive 校验并置订单为服务中。
func (a *UpstreamProvisionAdapter) markActive(order *model.Order) error {
	if err := EnsureStatus(order.Status, model.OrderStatusActive); err != nil {
		return err
	}
	order.Status = model.OrderStatusActive
	return nil
}

// provision 调用上游创建实例。
func (a *UpstreamProvisionAdapter) provision(ctx context.Context, order *model.Order) (*pkgmodel.StandardInstance, error) {
	if a.deps.BuildProvisionRequest == nil || a.deps.CreateInstance == nil || a.deps.BuildProviderConfig == nil {
		return nil, nil
	}
	raw, err := a.deps.BuildProvisionRequest(ctx, order.ProductID, order.ProductName, order.Specs, order.SpecCode)
	if err != nil {
		return nil, fmt.Errorf("构建开通请求失败: %w", err)
	}
	pid := providerID(raw)
	if pid == 0 {
		// 商品未绑定上游提供商（非云商品），回退为本地推进。
		return nil, nil
	}
	cfg, err := a.deps.BuildProviderConfig(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("构建提供商配置失败: %w", err)
	}
	req := &pkgmodel.CreateInstanceRequest{
		ProviderType: cfg.Type,
		ProductID:    uint(order.ProductID),
		Name:         order.ProductName,
		Extra:        extractConfigOptions(raw),
		// 计费周期优先取订单 cycle（doc25），空则回落 price_model（存量订单）。
		BillingMode: orderBillingCycle(order),
	}
	return a.deps.CreateInstance(ctx, cfg, req)
}

// orderBillingCycle 取订单计费周期：cycle 权威，price_model 兼容兜底。
func orderBillingCycle(order *model.Order) string {
	if order.Cycle != "" {
		return order.Cycle
	}
	return order.PriceModel
}

// advanceLegacy 无上游商品的订单：直接推进状态（与 DefaultProvisionAdapter 一致）。
func (a *UpstreamProvisionAdapter) advanceLegacy(order *model.Order) error {
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
	default:
		return ErrStatusConflict
	}
}

// providerID 从订单履约请求中提取提供商 ID（反射读取 ProviderID 字段）。
func providerID(raw interface{}) uint64 {
	if raw == nil {
		return 0
	}
	v := reflect.ValueOf(raw)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0
		}
		v = v.Elem()
	}
	f := v.FieldByName("ProviderID")
	if !f.IsValid() {
		return 0
	}
	return uint64(toUint(f))
}

// extractConfigOptions 从订单履约请求中提取可配置项（反射读取 ConfigOptions 字段）。
func extractConfigOptions(raw interface{}) map[string]interface{} {
	if raw == nil {
		return map[string]interface{}{}
	}
	v := reflect.ValueOf(raw)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return map[string]interface{}{}
		}
		v = v.Elem()
	}
	f := v.FieldByName("ConfigOptions")
	if !f.IsValid() {
		return map[string]interface{}{}
	}
	if f.IsNil() {
		return map[string]interface{}{}
	}
	if m, ok := f.Interface().(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

// toUint 将任意整型/浮点值转为 uint64（反射辅助）。
func toUint(v reflect.Value) uint64 {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return uint64(v.Float())
	}
	return 0
}
