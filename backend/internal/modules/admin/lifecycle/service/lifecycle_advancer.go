package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
	lifecyclerepo "hostsent/backend/internal/modules/admin/lifecycle/repository"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/pkg/upstream"
)

// StageActionRecorder 生命周期阶段动作审计写入（由实例运维模块的 instance_operations 表承接）。
// 只依赖基础类型，避免生命周期模块反向依赖实例模块。
type StageActionRecorder interface {
	RecordStageAction(ctx context.Context, in StageActionRecord) error
}

// StageActionRecord 一次阶段推进动作的审计记录。
type StageActionRecord struct {
	InstanceID   uint64
	InstanceMark string
	UserID       uint64
	Action       string // suspend / unsuspend / destroy / stage
	FromStage    string
	ToStage      string
	Err          error
}

// StageCapabilityNotifier 上游能力缺失时的显式提示（后台告警，可选）。
type StageCapabilityNotifier func(ctx context.Context, inst *syncmodel.Instance, stage string, cause error)

// LifecycleAdvancer 生命周期阶段推进器（T5.4）。
//
// 语义（doc15 §5.5 / doc17 §4-T5.4）：
//   - instances.lifecycle_stage 从"派生"改为"落库 + 幂等推进"；
//   - 按 expire_at + policy 算出目标阶段，仅在变化时执行一次动作并写审计；
//   - 只有动作成功（或平台确实不具备该能力）才落库新阶段 → 重复扫描不重复调用上游；
//   - 平台缺能力时退化为"仅标记 + 人工跟进"，并显式告警，绝不静默假装完成。
type LifecycleAdvancer struct {
	instanceRepo lifecyclerepo.InstanceReader
	policyRepo   lifecyclerepo.PolicyRepository
	resolve      ProviderResolverForLifecycle
	recorder     StageActionRecorder
	notifyCap    StageCapabilityNotifier
	logger       *zap.Logger
}

// ProviderResolverForLifecycle 按提供商 ID 解析上游适配器（装配层注入）。
type ProviderResolverForLifecycle func(ctx context.Context, providerID uint64) (upstream.Provider, error)

// NewLifecycleAdvancer 创建生命周期推进器。
func NewLifecycleAdvancer(
	instanceRepo lifecyclerepo.InstanceReader,
	policyRepo lifecyclerepo.PolicyRepository,
	resolve ProviderResolverForLifecycle,
	recorder StageActionRecorder,
	logger *zap.Logger,
) *LifecycleAdvancer {
	return &LifecycleAdvancer{
		instanceRepo: instanceRepo,
		policyRepo:   policyRepo,
		resolve:      resolve,
		recorder:     recorder,
		logger:       logger,
	}
}

// SetCapabilityNotifier 注入能力缺失告警（可选）。
func (a *LifecycleAdvancer) SetCapabilityNotifier(n StageCapabilityNotifier) {
	a.notifyCap = n
}

// AdvanceOnce 单轮推进：grace → suspended → destroyed，并把续费后回退的实例复位 active。
// 返回本轮处理的实例数（含审计写失败的记录）。
func (a *LifecycleAdvancer) AdvanceOnce(ctx context.Context) (int, error) {
	if a.instanceRepo == nil || a.policyRepo == nil {
		return 0, nil
	}
	policy, err := a.policyRepo.Get(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	processed := 0

	// 1) 正向推进（宽限 → 暂停 → 销毁），按阶段顺序执行，保证同一轮内不漏。
	for _, step := range []struct {
		stage  string
		window *lifecyclerepo.StageWindow
	}{
		{lifecyclemodel.StageGrace, stageWindowFor(lifecyclemodel.StageGrace, now, policy)},
		{lifecyclemodel.StageSuspended, stageWindowFor(lifecyclemodel.StageSuspended, now, policy)},
		{lifecyclemodel.StageDestroyed, stageWindowFor(lifecyclemodel.StageDestroyed, now, policy)},
	} {
		items, lerr := a.instanceRepo.ListStageCandidates(ctx, step.window, step.stage, 200)
		if lerr != nil {
			return processed, lerr
		}
		for i := range items {
			select {
			case <-ctx.Done():
				return processed, ctx.Err()
			default:
			}
			a.advanceOne(ctx, &items[i], step.stage)
			processed++
		}
	}

	// 2) 续费后回退 active：expire_at 已推后但阶段仍是 suspended/destroyed。
	resets, rerr := a.instanceRepo.ListActiveResetCandidates(ctx, now, 200)
	if rerr != nil {
		return processed, rerr
	}
	for i := range resets {
		select {
		case <-ctx.Done():
			return processed, ctx.Err()
		default:
		}
		a.resetActive(ctx, &resets[i])
		processed++
	}
	return processed, nil
}

// advanceOne 推进单个实例到目标阶段；动作失败时不落阶段（下轮重试），保持幂等。
func (a *LifecycleAdvancer) advanceOne(ctx context.Context, inst *syncmodel.Instance, stage string) {
	from := inst.LifecycleStage
	action := ""
	var opErr error

	switch stage {
	case lifecyclemodel.StageGrace:
		// 宽限期不触发上游动作，仅落阶段（供后台按阶段筛选与提醒口径统一）。
	case lifecyclemodel.StageSuspended:
		action = "suspend"
		opErr = a.withProvider(ctx, inst, func(p upstream.Provider) error {
			return upstream.SuspendWithFallback(ctx, p, a.instanceMark(inst), "到期未续费自动暂停")
		})
	case lifecyclemodel.StageDestroyed:
		action = "destroy"
		opErr = a.withProvider(ctx, inst, func(p upstream.Provider) error {
			return upstream.TerminateWithFallback(ctx, p, a.instanceMark(inst), "到期未续费自动销毁")
		})
	}

	if opErr != nil && !errors.Is(opErr, upstream.ErrCapabilityMissing) {
		// 真实失败（网络/上游报错）：不落阶段，下轮扫描重试。
		a.record(ctx, inst, action, from, from, opErr)
		a.logger.Warn("lifecycle advance action failed, will retry",
			zap.Uint64("instance_id", inst.ID), zap.String("target_stage", stage), zap.Error(opErr))
		return
	}
	if opErr != nil {
		// 平台确实不具备该能力：退化为"仅标记 + 人工跟进"，显式告警（不静默）。
		a.record(ctx, inst, action, from, stage, opErr)
		if err := a.setStage(ctx, inst, stage); err != nil {
			a.logger.Error("set lifecycle stage failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
			return
		}
		a.warnCapability(ctx, inst, stage, opErr)
		return
	}
	if err := a.setStage(ctx, inst, stage); err != nil {
		a.logger.Error("set lifecycle stage failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
		return
	}
	a.record(ctx, inst, action, from, stage, nil)
	a.logger.Info("lifecycle stage advanced",
		zap.Uint64("instance_id", inst.ID), zap.String("from", from), zap.String("to", stage))
}

// resetActive 续费后回退 active：曾暂停的实例先解除暂停，再落阶段。
func (a *LifecycleAdvancer) resetActive(ctx context.Context, inst *syncmodel.Instance) {
	from := inst.LifecycleStage
	var opErr error
	if from == lifecyclemodel.StageSuspended {
		opErr = a.withProvider(ctx, inst, func(p upstream.Provider) error {
			return upstream.UnsuspendWithFallback(ctx, p, a.instanceMark(inst))
		})
	}
	if opErr != nil && !errors.Is(opErr, upstream.ErrCapabilityMissing) {
		a.record(ctx, inst, "unsuspend", from, from, opErr)
		a.logger.Warn("lifecycle unsuspend failed, will retry",
			zap.Uint64("instance_id", inst.ID), zap.Error(opErr))
		return
	}
	if err := a.setStage(ctx, inst, lifecyclemodel.StageActive); err != nil {
		a.logger.Error("reset lifecycle stage failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
		return
	}
	a.record(ctx, inst, "unsuspend", from, lifecyclemodel.StageActive, opErr)
	a.logger.Info("lifecycle stage reset to active",
		zap.Uint64("instance_id", inst.ID), zap.String("from", from))
}

// withProvider 解析上游适配器并执行动作；无 provider 时视为能力缺失（仅标记）。
func (a *LifecycleAdvancer) withProvider(ctx context.Context, inst *syncmodel.Instance, fn func(upstream.Provider) error) error {
	if a.resolve == nil || inst.ProviderID == 0 {
		return upstream.MissingCapability("", "suspend")
	}
	p, err := a.resolve(ctx, inst.ProviderID)
	if err != nil {
		// 解析失败多为渠道配置问题：不落阶段，下轮重试（区别于"平台无此能力"）。
		return err
	}
	return fn(p)
}

// setStage 落库阶段（幂等：where id）。
func (a *LifecycleAdvancer) setStage(ctx context.Context, inst *syncmodel.Instance, stage string) error {
	return a.instanceRepo.UpdateLifecycleStage(ctx, inst.ID, stage)
}

// record 写审计；recorder 未注入或写入失败只记日志，不影响推进结果。
func (a *LifecycleAdvancer) record(ctx context.Context, inst *syncmodel.Instance, action string, from, to string, opErr error) {
	if a.recorder == nil {
		return
	}
	if action == "" {
		action = "stage"
	}
	if err := a.recorder.RecordStageAction(ctx, StageActionRecord{
		InstanceID:   inst.ID,
		InstanceMark: a.instanceMark(inst),
		UserID:       inst.UserID,
		Action:       action,
		FromStage:    from,
		ToStage:      to,
		Err:          opErr,
	}); err != nil {
		a.logger.Warn("record stage action failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
	}
}

// warnCapability 能力缺失显式告警（后台提示 + 可选通知）。
func (a *LifecycleAdvancer) warnCapability(ctx context.Context, inst *syncmodel.Instance, stage string, cause error) {
	a.logger.Warn("upstream capability missing for lifecycle stage, marked only",
		zap.Uint64("instance_id", inst.ID), zap.Uint64("provider_id", inst.ProviderID),
		zap.String("target_stage", stage), zap.Error(cause))
	if a.notifyCap != nil {
		a.notifyCap(ctx, inst, stage, cause)
	}
}

// instanceMark 上游实例号（优先 provider_instance_id，回落 instance_id）。
func (a *LifecycleAdvancer) instanceMark(inst *syncmodel.Instance) string {
	if inst.ProviderInstanceID != "" {
		return inst.ProviderInstanceID
	}
	return inst.InstanceID
}

// stageWindowFor 目标阶段对应的 expire_at 窗口（半开：(ExpireAfter, ExpireBefore]）。
// 与 lifecycle_service.stageWindow 的分组口径一致：active → grace → suspended → destroyed。
func stageWindowFor(stage string, now time.Time, policy *lifecyclemodel.LifecyclePolicy) *lifecyclerepo.StageWindow {
	graceEnd := now.AddDate(0, 0, -policy.GraceDays)
	destroyEnd := graceEnd.AddDate(0, 0, -policy.DestroyKeepDays)
	ptr := func(t time.Time) *time.Time { return &t }
	switch stage {
	case lifecyclemodel.StageGrace:
		return &lifecyclerepo.StageWindow{ExpireAfter: ptr(graceEnd), ExpireBefore: ptr(now)}
	case lifecyclemodel.StageSuspended:
		return &lifecyclerepo.StageWindow{ExpireAfter: ptr(destroyEnd), ExpireBefore: ptr(graceEnd)}
	case lifecyclemodel.StageDestroyed:
		return &lifecyclerepo.StageWindow{ExpireBefore: ptr(destroyEnd)}
	default:
		return nil
	}
}
