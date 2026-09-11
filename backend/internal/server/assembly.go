package server

// 本文件把 server.New 中的内联闭包抽为具名装配函数，使 server.go 回归"读配置→new→注册"薄壳，
// 便于单独测试与复用（见 00 规划 Phase 3 · T3.2）。这些函数都只做接线，不含业务逻辑。

import (
	"context"
	"errors"

	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
	ucinstanceservice "hostsent/backend/internal/modules/uc/instance/service"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/observability"
	"hostsent/backend/internal/pkg/upstream"
)

// buildOrderProvisionDeps 组装订单履约的上游开通适配器依赖（打通订单 paid/provisioning → 上游创建实例）。
func buildOrderProvisionDeps(
	catalog catalogservice.ProductService,
	provider providerservice.ProviderService,
	upmgr *upstream.ProviderManager,
	syncRepo syncrepo.SyncRepository,
) orderservice.ProvisionDeps {
	return orderservice.ProvisionDeps{
		BuildProvisionRequest: func(ctx context.Context, productID uint64, name, specSnapshot, specCode string) (interface{}, error) {
			return catalog.BuildProvisionRequest(ctx, productID, name, specSnapshot, specCode)
		},
		BuildProviderConfig: provider.BuildProviderConfig,
		CreateInstance: func(ctx context.Context, cfg *upstream.ProviderConfig, req *pkgmodel.CreateInstanceRequest) (*pkgmodel.StandardInstance, error) {
			provider, err := upmgr.Build(cfg.Type, cfg)
			if err != nil {
				return nil, err
			}
			provisioner, ok := provider.(upstream.InstanceProvisioning)
			if !ok {
				return nil, errors.New("上游不支持开通操作")
			}
			var inst *pkgmodel.StandardInstance
			err = observability.Timed("upstream_create_instance", func() error {
				var cerr error
				inst, cerr = provisioner.CreateInstance(ctx, req)
				return cerr
			})
			if err != nil {
				return nil, err
			}
			return inst, nil
		},
		RecordInstance: func(ctx context.Context, inst *pkgmodel.StandardInstance, order *ordermodel.Order) error {
			row := buildRecordedInstance(inst, order)
			if row.InstanceID == "" {
				return nil
			}
			return syncRepo.UpsertInstances(ctx, []syncmodel.Instance{row})
		},
		HasInstance: func(ctx context.Context, userID, productID uint64) (bool, error) {
			n, err := syncRepo.CountInstancesByProductUser(ctx, userID, productID)
			return n > 0, err
		},
	}
}

// buildEnsureOpenable 返回下单前校验函数：校验商品能否直连开通，
// 财务型上游等不支持单次开通的场景先行拒绝，避免误扣款。
func buildEnsureOpenable(catalog catalogservice.ProductService, provider providerservice.ProviderService) func(ctx context.Context, productID uint64) error {
	return func(ctx context.Context, productID uint64) error {
		preq, err := catalog.BuildProvisionRequest(ctx, productID, "", "", "")
		if err != nil {
			return err
		}
		if preq == nil || preq.ProviderID == 0 {
			return errors.New("该商品未绑定上游，暂不可在线购买")
		}
		cfg, err := provider.BuildProviderConfig(ctx, preq.ProviderID)
		if err != nil {
			return errors.New("该商品上游配置不可用，暂不可购买")
		}
		// 财务型上游（mofangfinance）现已支持下游下单开通，不再拦截。
		_ = cfg
		return nil
	}
}

// buildProviderResolver 返回用户中心主机模块的 ProviderResolver（按提供商 ID 实例化上游适配器）。
func buildProviderResolver(provider providerservice.ProviderService, upmgr *upstream.ProviderManager) ucinstanceservice.ProviderResolver {
	return func(ctx context.Context, providerID uint64) (upstream.Provider, error) {
		cfg, err := provider.BuildProviderConfig(ctx, providerID)
		if err != nil {
			return nil, err
		}
		return upmgr.Build(cfg.Type, cfg)
	}
}

// buildInstanceOpsResolver 返回实例运维台（管理端跨用户）的 ProviderResolver。
// 与用户中心复用同一解析实现，仅做具名函数类型转换（两者底层类型一致）。
func buildInstanceOpsResolver(provider providerservice.ProviderService, upmgr *upstream.ProviderManager) instanceservice.ProviderResolver {
	return instanceservice.ProviderResolver(buildProviderResolver(provider, upmgr))
}
