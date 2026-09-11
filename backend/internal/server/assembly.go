package server

// 本文件把 server.New 中的内联闭包抽为具名装配函数，使 server.go 回归"读配置→new→注册"薄壳，
// 便于单独测试与复用（见 00 规划 Phase 3 · T3.2）。这些函数都只做接线，不含业务逻辑。

import (
	"context"
	"errors"

	"go.uber.org/zap"

	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	lifecyclerepo "hostsent/backend/internal/modules/admin/lifecycle/repository"
	lifecycleservice "hostsent/backend/internal/modules/admin/lifecycle/service"
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
			// 链路判据（D6）：实例归属由商品 source_mode 决定，而非"经订单开通即自营"
			// ——代理商品（upstream）经订单开通后同样是上游链路实例，否则续费会被
			// markRenewalSuccess 误判为链路 B 而静默本地顺延。
			sourceMode := ""
			var upstreamProductID uint64
			if product, perr := catalog.FindByID(ctx, order.ProductID); perr == nil && product != nil {
				sourceMode = product.SourceMode
				upstreamProductID = product.SourceProductID
			}
			row := buildRecordedInstance(inst, order, sourceMode, upstreamProductID)
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

// buildUpstreamRenewer 返回生命周期续费的上游/平台执行器（T5.2）。
//
// 按实例 provider_id 解析适配器后调用 InstanceRenewal；适配器未实现该能力时
// 返回 upstream.ErrCapabilityMissing，由续费服务按链路判据决定：
// 链路 A（upstream）显式失败不得只改本地账期；链路 B（self）回落本地顺延。
func buildUpstreamRenewer(provider providerservice.ProviderService, upmgr *upstream.ProviderManager) lifecycleservice.UpstreamRenewer {
	return func(ctx context.Context, inst *syncmodel.Instance, periodCount int, cycle string) (*upstream.RenewResult, error) {
		if inst == nil || inst.ProviderID == 0 {
			return nil, upstream.ErrCapabilityMissing
		}
		cfg, err := provider.BuildProviderConfig(ctx, inst.ProviderID)
		if err != nil {
			return nil, err
		}
		p, err := upmgr.Build(cfg.Type, cfg)
		if err != nil {
			return nil, err
		}
		req := &upstream.RenewRequest{
			ProviderInstanceID: firstNonEmpty(inst.ProviderInstanceID, inst.InstanceID),
			Period:             periodCount,
			Cycle:              cycle,
		}
		var res *upstream.RenewResult
		err = observability.Timed("upstream_renew_instance", func() error {
			var rerr error
			res, rerr = upstream.RenewWithCapability(ctx, p, req)
			return rerr
		})
		if err != nil {
			return nil, err
		}
		return res, nil
	}
}

// buildLifecycleAdvancer 组装生命周期阶段推进器（T5.4）：按实例 provider_id 解析上游适配器，
// 阶段动作审计写入实例运维流水（instance_operations）。
//
// 注意：provider 解析单独走 buildProviderResolver，避免为"仅记录阶段"的宽限步骤解析上游。
func buildLifecycleAdvancer(
	instanceRepo lifecyclerepo.InstanceReader,
	policyRepo lifecyclerepo.PolicyRepository,
	provider providerservice.ProviderService,
	upmgr *upstream.ProviderManager,
	recorder lifecycleservice.StageActionRecorder,
	logger *zap.Logger,
) *lifecycleservice.LifecycleAdvancer {
	adv := lifecycleservice.NewLifecycleAdvancer(
		instanceRepo, policyRepo,
		lifecycleservice.ProviderResolverForLifecycle(buildProviderResolver(provider, upmgr)),
		recorder, logger,
	)
	return adv
}

// stageActionRecorderBridge 把实例运维服务的审计写入适配为生命周期模块的 StageActionRecorder。
//
// 两个模块不能互相 import（实例运维 → 生命周期方向已存在），故用最小接口桥接；
// 动作名与阶段值均为字符串，转换不涉及业务语义。
type stageActionRecorderBridge struct {
	svc instanceservice.InstanceService
}

// RecordStageAction 写一条系统操作人流水；未注入实例服务时静默跳过（无审计源）。
func (b *stageActionRecorderBridge) RecordStageAction(ctx context.Context, in lifecycleservice.StageActionRecord) error {
	if b == nil || b.svc == nil {
		return nil
	}
	return b.svc.RecordStageAction(ctx, instanceservice.StageActionInput{
		InstanceID:   in.InstanceID,
		InstanceMark: in.InstanceMark,
		UserID:       in.UserID,
		Action:       in.Action,
		FromStage:    in.FromStage,
		ToStage:      in.ToStage,
		Err:          in.Err,
	})
}
