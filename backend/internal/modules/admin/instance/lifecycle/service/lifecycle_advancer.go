package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"

	lifecyclemodel "hostsent/backend/internal/modules/admin/instance/lifecycle/model"
	lifecyclerepo "hostsent/backend/internal/modules/admin/instance/lifecycle/repository"
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
	// Result 显式结果：success / failed / skipped。为空时由 Err 推导（有 Err 即 failed）。
	// skipped 用于「未开启自动执行（仅标记）」与「预演」两种不落上游的情况。
	Result string
	// Message 补充说明（能力缺失原因、预演提示、退避说明等）。
	Message string
	Err     error
}

// StageCapabilityNotifier 上游能力缺失时的显式提示（后台告警，可选）。
type StageCapabilityNotifier func(ctx context.Context, inst *syncmodel.Instance, stage string, cause error)

// 强制执行失败退避参数（doc61 §8.4）：连续失败时按指数退避，避免每 10 分钟无脑重试。
const (
	enforceBackoffBase = 10 * time.Minute
	enforceBackoffMax  = 6 * time.Hour
	// enforceFailThreshold 连续失败达到该次数后升级为「需人工介入」告警。
	enforceFailThreshold = 5
)

// enforceBackoff 按累计失败次数计算下次重试间隔（10min → 20 → 40 → … 上限 6h）。
func enforceBackoff(attempts int) time.Duration {
	d := enforceBackoffBase
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= enforceBackoffMax {
			return enforceBackoffMax
		}
	}
	return d
}

// PreviewAction 到期处置预演的单条动作描述。
type PreviewAction struct {
	InstanceID        uint64
	InstanceMark      string
	Name              string
	UserID            uint64
	Username          string
	ProviderID        uint64
	Stage             string
	TargetStage       string
	Action            string // suspend / destroy / unsuspend / mark
	Reason            string
	ExpireAt          *time.Time
	DaysLeft          int
	CapabilityMissing bool
}

// LifecycleAdvancer 生命周期阶段推进器（T5.4）。
//
// 语义（doc15 §5.5 / doc17 §4-T5.4 / doc61 §8.4）：
//   - instances.lifecycle_stage 从"派生"改为"落库 + 幂等推进"；
//   - 按 expire_at + policy 算出目标阶段，仅在变化时执行一次动作并写审计；
//   - 只有动作成功（或平台确实不具备该能力）才落库新阶段 → 重复扫描不重复调用上游；
//   - 平台缺能力时退化为"仅标记 + 人工跟进"，并显式告警，绝不静默假装完成；
//   - **自动执行默认关闭**（policy.AutoEnforce=false）：关闭时暂停/销毁不下发上游、不落阶段，
//     只把宽限阶段登记在册，避免"没想开自动停服却被自动停服"；
//   - 真实失败按指数退避重试（enforce_next_at），连续失败升级告警，不再形成失败风暴。
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
//
// 闸门：policy.AutoEnforce=false（默认）时只做宽限登记与回退复位，暂停/销毁一律不下发；
// policy.EnforceDryRun=true 时同样不下发（真·预演由 PreviewEnforcement / 手动单实例执行产出）。
func (a *LifecycleAdvancer) AdvanceOnce(ctx context.Context) (int, error) {
	if a.instanceRepo == nil || a.policyRepo == nil {
		return 0, nil
	}
	policy, err := a.policyRepo.Get(ctx)
	if err != nil {
		return 0, err
	}
	live := policy.AutoEnforce && !policy.EnforceDryRun
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
			a.advanceOne(ctx, &items[i], step.stage, live)
			processed++
		}
	}

	// 2) 续费后回退 active：expire_at 已推后但阶段仍是 suspended/destroyed。
	// 回退动作受同一闸门约束：未开自动执行时只复位阶段（本地登记），不调用上游解除暂停。
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
		a.resetActive(ctx, &resets[i], live)
		processed++
	}
	return processed, nil
}

// PreviewEnforcement 产出到期处置预演报告（dry-run，只读、不写库、不调上游）。
//
// 与 AdvanceOnce 的差别：不受退避闸门限制，且回显「若现在开启自动执行会发生什么」，
// 供运维在开启 AutoEnforce 之前评估影响面（doc61 §8.4）。
func (a *LifecycleAdvancer) PreviewEnforcement(ctx context.Context) (actions []PreviewAction, err error) {
	policy, err := a.policyRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	for _, step := range []struct {
		stage  string
		window *lifecyclerepo.StageWindow
	}{
		{lifecyclemodel.StageGrace, stageWindowFor(lifecyclemodel.StageGrace, now, policy)},
		{lifecyclemodel.StageSuspended, stageWindowFor(lifecyclemodel.StageSuspended, now, policy)},
		{lifecyclemodel.StageDestroyed, stageWindowFor(lifecyclemodel.StageDestroyed, now, policy)},
	} {
		items, lerr := a.instanceRepo.ListStageCandidatesWithUser(ctx, step.window, step.stage, 500)
		if lerr != nil {
			return nil, lerr
		}
		for i := range items {
			actions = append(actions, a.previewAction(&items[i], step.stage, now))
		}
	}

	resets, rerr := a.instanceRepo.ListActiveResetCandidatesWithUser(ctx, now, 500)
	if rerr != nil {
		return nil, rerr
	}
	for i := range resets {
		it := &resets[i]
		expire := time.Time{}
		if it.ExpireAt != nil {
			expire = *it.ExpireAt
		}
		actions = append(actions, PreviewAction{
			InstanceID:   it.ID,
			InstanceMark: it.InstanceID,
			Name:         it.Name,
			UserID:       it.UserID,
			Username:     it.Username,
			ProviderID:   it.ProviderID,
			Stage:        it.LifecycleStage,
			TargetStage:  lifecyclemodel.StageActive,
			Action:       "unsuspend",
			Reason:       "续费后到期时间已推后，需解除暂停并复位为服务中",
			ExpireAt:     it.ExpireAt,
			DaysLeft:     daysLeft(expire, now),
		})
	}
	return actions, nil
}

// EnforceOne 手动对单个实例执行一次目标阶段动作（运维显式点击，绕过 AutoEnforce 总开关但与
// dry-run 无关——手动即视为真实执行）。返回实际执行的动作名与错误。
//
// 用于：① 自动执行关闭期间对个别实例单独处置；② 自动执行失败后人工重试。
func (a *LifecycleAdvancer) EnforceOne(ctx context.Context, instanceID uint64, reason string) (string, error) {
	inst, err := a.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return "", err
	}
	policy, err := a.policyRepo.Get(ctx)
	if err != nil {
		return "", err
	}
	now := time.Now()
	target, action, derived := derivedTargetStage(inst, policy, now)
	if action == "" {
		return "", ErrStageNotActionable
	}
	// 运维填写的原因也要留痕：写进流水 message，事后能回答「当初为什么手动停这台」。
	message := derived
	if reason = strings.TrimSpace(reason); reason != "" {
		message = "手动执行：" + reason + "；" + derived
	}
	opErr := a.applyUpstream(ctx, inst, target)
	if opErr != nil {
		if errors.Is(opErr, upstream.ErrCapabilityMissing) {
			a.markCapabilityMissing(ctx, inst, target, opErr)
			return action, ErrCapabilityUnsupported
		}
		a.markFailure(ctx, inst, action, target, opErr)
		return action, opErr
	}
	if err := a.setStage(ctx, inst, target); err != nil {
		return action, err
	}
	_ = a.instanceRepo.ClearEnforce(ctx, inst.ID)
	a.record(ctx, inst, action, inst.LifecycleStage, target, nil, "success", message)
	a.logger.Info("lifecycle stage enforced manually",
		zap.Uint64("instance_id", inst.ID), zap.String("to", target), zap.String("action", action))
	return action, nil
}

// previewAction 计算单实例的预演动作（只读）。
func (a *LifecycleAdvancer) previewAction(it *lifecyclerepo.InstanceWithUser, stage string, now time.Time) PreviewAction {
	expire := time.Time{}
	if it.ExpireAt != nil {
		expire = *it.ExpireAt
	}
	pre := PreviewAction{
		InstanceID:   it.ID,
		InstanceMark: it.InstanceID,
		Name:         it.Name,
		UserID:       it.UserID,
		Username:     it.Username,
		ProviderID:   it.ProviderID,
		Stage:        it.LifecycleStage,
		TargetStage:  stage,
		ExpireAt:     it.ExpireAt,
		DaysLeft:     daysLeft(expire, now),
	}
	switch stage {
	case lifecyclemodel.StageGrace:
		pre.Action = "mark"
		pre.Reason = "已过到期时间，进入宽限期（不涉及上游动作，仅登记阶段）"
	case lifecyclemodel.StageSuspended:
		pre.Action = "suspend"
		pre.Reason = "宽限期已结束，将暂停实例（优先平台暂停态，退化为关机）"
		pre.CapabilityMissing = !a.hasProvider()
	case lifecyclemodel.StageDestroyed:
		pre.Action = "destroy"
		pre.Reason = "暂停保留期已结束，将销毁实例（优先终止，退化为删除）"
		pre.CapabilityMissing = !a.hasProvider()
	}
	return pre
}

// hasProvider 是否装配了上游解析器（缺装配时预演即提示能力缺失）。
func (a *LifecycleAdvancer) hasProvider() bool { return a.resolve != nil }

// advanceOne 推进单个实例到目标阶段；动作失败时不落阶段（下轮重试），保持幂等。
//
// live=false（未开启自动执行 / 预演）时：宽限阶段照常登记（无上游动作），
// 暂停/销毁一律不下发也不落阶段 —— 保证「没开开关就不会动线上实例」，
// 且不会每轮写一条 skipped 流水（避免又一次失败风暴），只留一条 debug 日志。
func (a *LifecycleAdvancer) advanceOne(ctx context.Context, inst *syncmodel.Instance, stage string, live bool) {
	from := inst.LifecycleStage

	if stage == lifecyclemodel.StageGrace {
		// 宽限期不触发上游动作，仅落阶段（供后台按阶段筛选与提醒口径统一）。
		if err := a.setStage(ctx, inst, stage); err != nil {
			a.logger.Error("set lifecycle stage failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
			return
		}
		a.record(ctx, inst, "", from, stage, nil, "success", "")
		a.logger.Info("lifecycle stage advanced",
			zap.Uint64("instance_id", inst.ID), zap.String("from", from), zap.String("to", stage))
		return
	}

	if !live {
		// 闸门关闭：只记录"本可执行"的事实，不下发、不落阶段（运维经预演页查看影响面）。
		if a.logger != nil {
			a.logger.Debug("lifecycle enforcement disabled, skip upstream action",
				zap.Uint64("instance_id", inst.ID), zap.String("target_stage", stage))
		}
		return
	}

	action := ""
	switch stage {
	case lifecyclemodel.StageSuspended:
		action = "suspend"
	case lifecyclemodel.StageDestroyed:
		action = "destroy"
	}

	opErr := a.applyUpstream(ctx, inst, stage)
	if opErr != nil && !errors.Is(opErr, upstream.ErrCapabilityMissing) {
		// 真实失败（网络/上游报错）：不落阶段，下轮（退避后）重试。
		a.markFailure(ctx, inst, action, stage, opErr)
		a.logger.Warn("lifecycle advance action failed, will retry",
			zap.Uint64("instance_id", inst.ID), zap.String("target_stage", stage), zap.Error(opErr))
		return
	}
	if opErr != nil {
		// 平台确实不具备该能力：退化为"仅标记 + 人工跟进"，显式告警（不静默）。
		a.markCapabilityMissing(ctx, inst, stage, opErr)
		return
	}
	if err := a.setStage(ctx, inst, stage); err != nil {
		a.logger.Error("set lifecycle stage failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
		return
	}
	_ = a.instanceRepo.ClearEnforce(ctx, inst.ID)
	a.record(ctx, inst, action, from, stage, nil, "success", "")
	a.logger.Info("lifecycle stage advanced",
		zap.Uint64("instance_id", inst.ID), zap.String("from", from), zap.String("to", stage))
}

// resetActive 续费后回退 active：曾暂停的实例先解除暂停，再落阶段。
// live=false 时只复位本地阶段（本地登记），不调用上游。
func (a *LifecycleAdvancer) resetActive(ctx context.Context, inst *syncmodel.Instance, live bool) {
	from := inst.LifecycleStage
	var opErr error
	if from == lifecyclemodel.StageSuspended && live {
		opErr = a.withProvider(ctx, inst, func(p upstream.Provider) error {
			return upstream.UnsuspendWithFallback(ctx, p, a.instanceMark(inst))
		})
	}
	if opErr != nil && !errors.Is(opErr, upstream.ErrCapabilityMissing) {
		a.markFailure(ctx, inst, "unsuspend", lifecyclemodel.StageActive, opErr)
		a.logger.Warn("lifecycle unsuspend failed, will retry",
			zap.Uint64("instance_id", inst.ID), zap.Error(opErr))
		return
	}
	if err := a.setStage(ctx, inst, lifecyclemodel.StageActive); err != nil {
		a.logger.Error("reset lifecycle stage failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
		return
	}
	_ = a.instanceRepo.ClearEnforce(ctx, inst.ID)
	a.record(ctx, inst, "unsuspend", from, lifecyclemodel.StageActive, opErr, "", "")
	a.logger.Info("lifecycle stage reset to active",
		zap.Uint64("instance_id", inst.ID), zap.String("from", from))
}

// applyUpstream 按目标阶段下发上游动作（不落库、不审计，供 AdvanceOnce 与 EnforceOne 复用）。
func (a *LifecycleAdvancer) applyUpstream(ctx context.Context, inst *syncmodel.Instance, stage string) error {
	switch stage {
	case lifecyclemodel.StageSuspended:
		return a.withProvider(ctx, inst, func(p upstream.Provider) error {
			return upstream.SuspendWithFallback(ctx, p, a.instanceMark(inst), "到期未续费自动暂停")
		})
	case lifecyclemodel.StageDestroyed:
		return a.withProvider(ctx, inst, func(p upstream.Provider) error {
			return upstream.TerminateWithFallback(ctx, p, a.instanceMark(inst), "到期未续费自动销毁")
		})
	default:
		return nil
	}
}

// markFailure 记录一次真实失败并设置退避（不落阶段，等待下轮重试）。
func (a *LifecycleAdvancer) markFailure(ctx context.Context, inst *syncmodel.Instance, action, target string, opErr error) {
	attempts := inst.EnforceAttempts + 1
	nextAt := time.Now().Add(enforceBackoff(attempts))
	if a.instanceRepo != nil {
		if err := a.instanceRepo.MarkEnforceFailure(ctx, inst.ID, opErr.Error(), nextAt, attempts); err != nil {
			a.logger.Warn("mark enforce failure failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
		}
	}
	a.record(ctx, inst, action, inst.LifecycleStage, inst.LifecycleStage, opErr, "failed", "")
	// 连续失败达阈值：升级为需人工介入的告警（复用能力缺失告警通道）。
	if attempts >= enforceFailThreshold && a.notifyCap != nil {
		a.notifyCap(ctx, inst, target, opErr)
	}
}

// markCapabilityMissing 平台缺能力：落阶段（仅标记）+ 显式告警，记录 skipped 流水。
func (a *LifecycleAdvancer) markCapabilityMissing(ctx context.Context, inst *syncmodel.Instance, stage string, cause error) {
	if err := a.setStage(ctx, inst, stage); err != nil {
		a.logger.Error("set lifecycle stage failed", zap.Uint64("instance_id", inst.ID), zap.Error(err))
		return
	}
	a.record(ctx, inst, stageActionOf(stage), inst.LifecycleStage, stage, cause, "skipped", "上游不具备该能力，已仅标记待人工跟进")
	a.warnCapability(ctx, inst, stage, cause)
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
func (a *LifecycleAdvancer) record(ctx context.Context, inst *syncmodel.Instance, action string, from, to string, opErr error, result, message string) {
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
		Result:       result,
		Message:      message,
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

// stageActionOf 阶段 → 审计动作名。
func stageActionOf(stage string) string {
	switch stage {
	case lifecyclemodel.StageSuspended:
		return "suspend"
	case lifecyclemodel.StageDestroyed:
		return "destroy"
	default:
		return "stage"
	}
}

// derivedTargetStage 按实例当前到期时间与策略推导「应当推进到的阶段」及其动作。
// action 为空表示当前无需动作（阶段已一致或未到期）。
func derivedTargetStage(inst *syncmodel.Instance, policy *lifecyclemodel.LifecyclePolicy, now time.Time) (target, action, reason string) {
	if inst.ExpireAt == nil {
		return "", "", ""
	}
	stage := deriveStage(*inst.ExpireAt, now, policy)
	switch stage {
	case lifecyclemodel.StageGrace:
		return stage, "stage", "已过到期时间，进入宽限期（仅登记阶段，不涉及上游动作）"
	case lifecyclemodel.StageSuspended:
		return stage, "suspend", "宽限期已结束，暂停实例（优先平台暂停态，退化为关机）"
	case lifecyclemodel.StageDestroyed:
		return stage, "destroy", "暂停保留期已结束，销毁实例（优先终止，退化为删除）"
	default:
		return stage, "", ""
	}
}

// deriveStage 按到期时间与策略推导生命周期阶段（与 lifecycleService.DeriveStage 同口径）。
func deriveStage(expireAt, now time.Time, policy *lifecyclemodel.LifecyclePolicy) string {
	graceEnd := expireAt.AddDate(0, 0, policy.GraceDays)
	destroyEnd := graceEnd.AddDate(0, 0, policy.DestroyKeepDays)
	switch {
	case now.Before(expireAt):
		return lifecyclemodel.StageActive
	case now.Before(graceEnd):
		return lifecyclemodel.StageGrace
	case now.Before(destroyEnd):
		return lifecyclemodel.StageSuspended
	default:
		return lifecyclemodel.StageDestroyed
	}
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
