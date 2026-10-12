// 生命周期域装配（批次四 4b；含自 assembly.go 迁入的上游续费器与阶段推进器）。

package server

import (
	"context"
	"fmt"
	referralservice "hostsent/backend/internal/modules/admin/finance/referral/service"
	lifecyclehandler "hostsent/backend/internal/modules/admin/instance/lifecycle/handler"
	lifecyclemodel "hostsent/backend/internal/modules/admin/instance/lifecycle/model"
	lifecyclerepo "hostsent/backend/internal/modules/admin/instance/lifecycle/repository"
	lifecycleservice "hostsent/backend/internal/modules/admin/instance/lifecycle/service"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	notifydto "hostsent/backend/internal/modules/admin/system/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/system/notification/model"
	notifyservice "hostsent/backend/internal/modules/admin/system/notification/service"
	salesservice "hostsent/backend/internal/modules/admin/system/sales/service"
	"hostsent/backend/internal/pkg/observability"
	"hostsent/backend/internal/pkg/upstream"

	"go.uber.org/zap"
)

// buildLifecycle 构造生命周期续费/策略服务、调度器与阶段推进器。
func (a *asm) buildLifecycle() {
	// 生命周期与续费域（doc60）
	a.lifecycleRenewalRepo = lifecyclerepo.NewRenewalRepository(a.database)
	a.lifecyclePolicyRepo = lifecyclerepo.NewPolicyRepository(a.database)
	a.lifecycleAutoRepo = lifecyclerepo.NewAutoRenewRepository(a.database)
	a.lifecycleInstanceReader = lifecyclerepo.NewInstanceReader(a.database)
	a.lifecycleOrderWriter = lifecyclerepo.NewOrderWriter(a.database)
	a.lifecycleRenewalSvc = lifecycleservice.NewRenewalService(a.database, a.lifecycleRenewalRepo, a.lifecyclePolicyRepo, a.lifecycleAutoRepo, a.lifecycleInstanceReader, a.lifecycleOrderWriter, a.walletService, a.pricePipeline, a.logger)
	// 续费接上游/平台（T5.2）：链路 A 以上游返回账期为权威，链路 B 回落本地顺延。
	a.lifecycleRenewalSvc.SetUpstreamRenewer(buildUpstreamRenewer(a.providerService, a.upstreamMgr))
	// 续费完成 → 给邀请人按续费比率计提返现；同一闭包内串行计提销售提成
	//（SetCashbackHook 单槽，分别 Set 会互相覆盖）。
	a.lifecycleRenewalSvc.SetCashbackHook(func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, amount float64, isRenewal bool) error {
		if err := a.referralSvc.AccrueForOrder(ctx, referralservice.AccrualInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID, BaseAmount: amount, IsRenewal: isRenewal,
		}); err != nil {
			a.logger.Warn("referral renewal cashback accrual failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo),
				zap.Uint64("user_id", buyerUserID), zap.Error(err))
		}
		if err := a.salesBundle.commissionService.AccrueForOrder(ctx, salesservice.AccrualInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID, BaseAmount: amount, IsRenewal: isRenewal,
		}); err != nil {
			a.logger.Warn("sales commission renewal accrual failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo), zap.Error(err))
		}
		return nil
	})
	// 销售归属快照：续费跟源订单（doc86 S4）。
	a.lifecycleRenewalSvc.SetSalesOwnerResolver(a.salesBundle.renewalOwnerPort())
	a.lifecycleSvc = lifecycleservice.NewLifecycleService(a.database, a.lifecyclePolicyRepo, a.lifecycleInstanceReader, a.lifecycleRenewalSvc, a.logger)
	a.lifecycleExpiringHandler = lifecyclehandler.NewExpiringHandler(a.lifecycleSvc)
	a.lifecycleAdminHandler = lifecyclehandler.NewLifecycleAdminHandler(a.lifecycleSvc, a.lifecycleRenewalSvc)
	a.lifecycleUserHandler = lifecyclehandler.NewLifecycleUserHandler(a.lifecycleSvc, a.lifecycleRenewalSvc)
	a.lifecycleScheduler = lifecycleservice.NewLifecycleScheduler(a.lifecycleSvc, a.logger)
	// 生命周期阶段推进器（T5.4）：宽限→暂停→销毁幂等落库；审计写入实例运维流水。
	a.lifecycleAdvancer = buildLifecycleAdvancer(
		a.lifecycleInstanceReader, a.lifecyclePolicyRepo,
		a.providerService, a.upstreamMgr,
		&stageActionRecorderBridge{svc: a.instanceOpsService},
		a.logger,
	)
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
// 两个模块不能互相 import（实例运维 → 生命周期方向已存在），故用最小接口桥接；
// 动作名与阶段值均为字符串，转换不涉及业务语义。

// buildRefundPolicyReader 读取退款后实例处置策略（lifecycle_policies.refund_action）。
// 读取失败或值为空一律回落 none：处置线上实例是不可逆动作，配置不可用时必须选「不动作」。
func buildRefundPolicyReader(policyRepo lifecyclerepo.PolicyRepository, logger *zap.Logger) func(ctx context.Context) string {
	return func(ctx context.Context) string {
		if policyRepo == nil {
			return lifecyclemodel.RefundActionNone
		}
		policy, err := policyRepo.Get(ctx)
		if err != nil || policy == nil {
			if logger != nil {
				logger.Warn("read refund action policy failed, fallback to none", zap.Error(err))
			}
			return lifecyclemodel.RefundActionNone
		}
		switch policy.RefundAction {
		case lifecyclemodel.RefundActionSuspend, lifecyclemodel.RefundActionDestroy:
			return policy.RefundAction
		default:
			return lifecyclemodel.RefundActionNone
		}
	}
}

// RecordStageAction 写一条系统操作人流水；未注入实例服务时静默跳过（无审计源）。
// buildStageCapabilityNotifier 阶段推进遇上游缺能力时的显式告警（T5.4/T5.5）。
//
// 语义：平台确实不支持暂停/销毁时，推进器只落阶段并告警，必须让运营可见，
// 避免"看起来已暂停"的静默假完成。告警复用 system 事件，不新增模板。
func buildStageCapabilityNotifier(notifySvc notifyservice.NotificationService, logger *zap.Logger) lifecycleservice.StageCapabilityNotifier {
	return func(ctx context.Context, inst *syncmodel.Instance, stage string, cause error) {
		if inst == nil {
			return
		}
		err := notifySvc.Publish(ctx, notifydto.PublishInput{
			Event:  notifymodel.EventSystem,
			Target: notifymodel.TargetAdmin,
			Vars: map[string]string{
				"title": "实例到期处置缺少上游能力",
				"content": fmt.Sprintf("实例 #%d（用户 #%d，服务商 #%d）进入阶段 %s 时上游不支持对应操作，已仅落库标记，需人工跟进：%v。",
					inst.ID, inst.UserID, inst.ProviderID, stage, cause),
			},
			SourceModule: "lifecycle",
			SourceID:     fmt.Sprintf("lifecycle-capability-%d-%s", inst.ID, stage),
		})
		if err != nil {
			logger.Warn("publish lifecycle capability notification failed",
				zap.Uint64("instance_id", inst.ID), zap.String("stage", stage), zap.Error(err))
		}
	}
}

// buildRecordedInstance 将标准实例转换为同步实例记录（订单履约开通成功后落库）。
// sourceMode 为商品链路判据（D6），upstreamProductID 为代理商品对应的上游资源商品 ID。
