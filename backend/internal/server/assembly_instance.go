// 实例域装配（批次四 4b；含自 assembly.go 迁入的上游解析器与阶段审计桥）。

package server

import (
	"context"
	instancehandler "hostsent/backend/internal/modules/admin/instance/handler"
	lifecycleservice "hostsent/backend/internal/modules/admin/instance/lifecycle/service"
	instancerepo "hostsent/backend/internal/modules/admin/instance/repository"
	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	orderrepo "hostsent/backend/internal/modules/admin/order/repository"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	ucinstancehandler "hostsent/backend/internal/modules/uc/instance/handler"
	ucinstancerepo "hostsent/backend/internal/modules/uc/instance/repository"
	ucinstanceservice "hostsent/backend/internal/modules/uc/instance/service"
	"hostsent/backend/internal/pkg/upstream"
)

// buildInstanceOps 构造用户中心主机与实例运维台。
func (a *asm) buildInstanceOps() {
	// 用户中心主机管理：列表/详情/电源/VNC（复用上游适配器）
	a.ucInstanceRepo = ucinstancerepo.NewInstanceRepository(a.database)
	a.ucInstanceService = ucinstanceservice.NewInstanceService(a.ucInstanceRepo, buildProviderResolver(a.providerService, a.upstreamMgr))
	a.ucInstanceHandler = ucinstancehandler.NewInstanceHandler(a.ucInstanceService)
	// 实例运维台（管理端跨用户）：列表/详情/电源/控制台/回源/变配/销毁/流水/关联（见 docs/实施计划/61）
	a.instanceOpsRepo = instancerepo.NewInstanceRepository(a.database)
	a.instanceOpRepo = instancerepo.NewOperationRepository(a.database)
	a.instanceRelatedRepo = instancerepo.NewRelatedRepository(a.database)
	a.instanceOpsService = instanceservice.NewInstanceService(
		a.instanceOpsRepo,
		a.instanceOpRepo,
		a.instanceRelatedRepo,
		buildInstanceOpsResolver(a.providerService, a.upstreamMgr),
		a.logger,
	)
	a.instanceOpsHandler = instancehandler.NewInstanceHandler(a.instanceOpsService)
	// 详情页关联数据（订单/商品/上游价/配置/凭证）：只读仓储 + 订单选配读取。
	// 未注入时 Detail 仍返回实例本体，仅缺少这些附加分组。
	a.instanceOpsService.SetDetailRepository(instancerepo.NewInstanceDetailRepository(a.database, a.cfg.App.EncryptKey))
	a.instanceOpsService.SetDetailOrderConfigReader(buildInstanceOrderConfigReader(a.orderRepo))
	// 密码留存密钥（迁移 100）：重置密码/重装产生的新密码据此加密回写。
	a.instanceOpsService.SetCredentialSecretKey(a.cfg.App.EncryptKey)
	// 用户侧自助销毁（doc91 §6.4）：薄委派给运维台，销毁逻辑（能力分派/流水/事件）不重写。
	a.ucInstanceService.SetDestructor(a.instanceOpsService)
	// 用户侧维护类自助（重装/重置密码/救援/快照）：同样委派运维台。
	// 归属校验在 UC 侧用带 user_id 的查询完成，委派后复用运维台的能力分派与操作流水。
	a.ucInstanceService.SetOperator(a.instanceOpsService)
	// 实例维度「重新开通」（占位/未开通实例）：重投来源订单的履约任务，
	// 上游充值后点一次即可复用已生成账单完成开通。管理端与用户中心共用同一实现。
	a.instanceOpsService.SetProvisionRetry(a.buildInstanceProvisionRetry())
	a.ucInstanceService.SetProvisionRetry(a.buildInstanceProvisionRetry())
	// 实例操作事件（P6/T6.5）：电源/暂停恢复/阶段推进 → instance.status_changed。
	a.instanceOpsService.SetEventListener(buildInstanceEventListener(a.openEventPublisher))
	// 退款后实例处置策略（doc61 §8 第 2 条）：读 lifecycle_policies.refund_action，
	// 读不到一律 none（不动作）。装配在此处（lifecyclePolicyRepo 构造于下方生命周期域块），
	// 因此延迟到该块之后再 SetRefundPolicyReader。
	// 退款钩子实体见下方 orderService.SetInstanceRefundHook（同处依赖 instanceOpsService）。
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

// buildInstanceOpsResolver 返回实例运维台（管理端跨用户）的 ProviderResolver。
// 与用户中心复用同一解析实现，仅做具名函数类型转换（两者底层类型一致）。
func buildInstanceOpsResolver(provider providerservice.ProviderService, upmgr *upstream.ProviderManager) instanceservice.ProviderResolver {
	return instanceservice.ProviderResolver(buildProviderResolver(provider, upmgr))
}

// buildInstanceOrderConfigReader 返回实例详情页读取订单客户选配 JSON 的读取器（orders.config_options）。
//
// 详情页需要在配置区展示"客户下单时实际选择的规格"，该选择落库在订单而非实例，
// 故由装配层把订单仓储的读取能力以最小闭包注入实例域，避免 instance 包反向依赖 order 包。

// buildInstanceOrderConfigReader 返回实例详情页读取订单客户选配 JSON 的读取器（orders.config_options）。
//
// 详情页需要在配置区展示"客户下单时实际选择的规格"，该选择落库在订单而非实例，
// 故由装配层把订单仓储的读取能力以最小闭包注入实例域，避免 instance 包反向依赖 order 包。
func buildInstanceOrderConfigReader(repo orderrepo.OrderRepository) func(ctx context.Context, orderID uint64) (string, error) {
	return func(ctx context.Context, orderID uint64) (string, error) {
		if repo == nil || orderID == 0 {
			return "", nil
		}
		order, err := repo.FindByID(ctx, orderID)
		if err != nil {
			return "", err
		}
		if order == nil {
			return "", nil
		}
		return order.ConfigOptions, nil
	}
}

// buildUpstreamRenewer 返回生命周期续费的上游/平台执行器（T5.2）。
//
// 按实例 provider_id 解析适配器后调用 InstanceRenewal；适配器未实现该能力时
// 返回 upstream.ErrCapabilityMissing，由续费服务按链路判据决定：
// 链路 A（upstream）显式失败不得只改本地账期；链路 B（self）回落本地顺延。

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
		Result:       in.Result,
		Message:      in.Message,
		Err:          in.Err,
	})
}

// stageActionRecorderBridge 把实例运维服务的审计写入适配为生命周期模块的 StageActionRecorder。
// 两个模块不能互相 import（实例运维 → 生命周期方向已存在），故用最小接口桥接；
// 动作名与阶段值均为字符串，转换不涉及业务语义。
type stageActionRecorderBridge struct {
	svc instanceservice.InstanceService
}

// buildRefundInstanceHook 组装退款审核通过后的实例侧处置钩子（doc61 §8 第 2 条）。
//
// 钩子只做转译：把订单侧的基础类型搬成实例域的入参，规则判断（策略/全额判定）在
// 实例服务内完成。失败只记日志 —— 退款审核结果不因实例侧问题回滚。
