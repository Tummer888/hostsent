package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/resource/product/model"
	productrepo "hostsent/backend/internal/modules/admin/resource/product/repository"
	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	providerrepo "hostsent/backend/internal/modules/admin/resource/provider/repository"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/observability"
	"hostsent/backend/internal/pkg/specatom"
	"hostsent/backend/internal/pkg/upstream"
)

// 同步任务类型（兼容别名，P3 起内部按 scope 调度）。
// 保留常量是因为后台旧接口与旧页面仍以 product/pool/instance 传参。
const (
	TaskTypeProduct  = syncmodel.ScopeCatalog
	TaskTypePool     = syncmodel.ScopePool
	TaskTypeInstance = syncmodel.ScopeInstance
)

// 同步任务状态
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	// StatusSkipped 渠道能力未覆盖该 scope：不产生上游调用，也不计入失败熔断。
	StatusSkipped = "skipped"
)

// ErrTaskRunning 同一提供商同类型已有同步任务执行中（防重入）。
var ErrTaskRunning = errors.New("已有同步任务在执行中")

// maxConsecutiveSyncFailures 连续失败达到该次数即自动熔断（sync_paused）。
const maxConsecutiveSyncFailures = 5

// defaultPriceChangeThreshold 渠道未配置时的调价自动应用阈值（5%）。
const defaultPriceChangeThreshold = 0.05

// providerUnavailableError 适配器构建/凭证加载失败——属永久性错误，应立即熔断而非重试。
// Error() 透传底层信息，describeSyncError 仍能给出可读描述。
type providerUnavailableError struct{ err error }

func (e *providerUnavailableError) Error() string { return e.err.Error() }
func (e *providerUnavailableError) Unwrap() error { return e.err }

// scopeUnsupportedError 渠道不支持该 scope（描述符未声明或适配器未实现）：
// 属"跳过"而非"失败"，不参与熔断计数。
type scopeUnsupportedError struct{ scope string }

func (e *scopeUnsupportedError) Error() string {
	return fmt.Sprintf("%s：该渠道不支持此同步类型", ScopeDisplayName(e.scope))
}

// ProviderConfigProvider 根据提供商 ID 构建适配器配置（含解密密钥）。
// 由 providerService 实现，避免 sync 包依赖 provider 具体实现。
type ProviderConfigProvider interface {
	BuildProviderConfig(ctx context.Context, providerID uint64) (*upstream.ProviderConfig, error)
}

// ProviderPricePolicy 渠道调价策略（可选实现）：读取阈值 + 应用已确认调价。
// 未实现时按 defaultPriceChangeThreshold 处理，且确认动作只更新资源商品。
type ProviderPricePolicy interface {
	// PriceChangeThreshold 返回渠道级调价阈值（比例，0.05=5%）。
	PriceChangeThreshold(ctx context.Context, providerID uint64) (float64, error)
}

// PriceApplier 已确认调价的落地动作（由 catalog 侧实现，避免包循环）。
type PriceApplier interface {
	// ApplyConfirmedPrice 把已确认的成本价写入关联售出商品并记 product_history，返回受影响商品数。
	ApplyConfirmedPrice(ctx context.Context, resourceProductID uint64, costPrice float64, operatorName, remark string) (int, error)
}

// PriceNotifier 调价通知（桥接到通知中心，target=admin）。
type PriceNotifier interface {
	NotifyPriceChangePending(ctx context.Context, providerID uint64, providerName string, pending int) error
}

// SyncEngine 资源同步引擎：按 scope 拉取上游数据，标准化后幂等落库，
// 并维护任务状态机（pending → running → success/failed/skipped）与同步日志。
type SyncEngine struct {
	upmgr         *upstream.ProviderManager
	provider      ProviderConfigProvider
	productRepo   productrepo.ProductRepository
	poolRepo      providerrepo.PoolRepository
	providerRepo  providerrepo.ProviderRepository
	syncRepo      syncrepo.SyncRepository
	frameworkRepo syncrepo.FrameworkRepository
	priceApplier  PriceApplier
	notifier      PriceNotifier
	logger        *zap.Logger
}

// NewSyncEngine 创建同步引擎
func NewSyncEngine(
	upmgr *upstream.ProviderManager,
	provider ProviderConfigProvider,
	productRepo productrepo.ProductRepository,
	poolRepo providerrepo.PoolRepository,
	providerRepo providerrepo.ProviderRepository,
	syncRepo syncrepo.SyncRepository,
	frameworkRepo syncrepo.FrameworkRepository,
	logger *zap.Logger,
) *SyncEngine {
	return &SyncEngine{
		upmgr:         upmgr,
		provider:      provider,
		productRepo:   productRepo,
		poolRepo:      poolRepo,
		providerRepo:  providerRepo,
		syncRepo:      syncRepo,
		frameworkRepo: frameworkRepo,
		logger:        logger,
	}
}

// SetPriceApplier / SetNotifier 在装配阶段注入（避免 New 参数继续膨胀）。
func (e *SyncEngine) SetPriceApplier(a PriceApplier) { e.priceApplier = a }
func (e *SyncEngine) SetNotifier(n PriceNotifier)    { e.notifier = n }

// StartSync 创建同步任务并异步执行（taskType 兼容旧别名，内部归一为 scope）。
// 同一提供商同类型存在 pending/running 任务时拒绝（防重入）。
func (e *SyncEngine) StartSync(ctx context.Context, providerID uint64, taskType string) (*syncmodel.SyncTask, error) {
	scope := syncmodel.NormalizeScope(taskType)
	n, err := e.syncRepo.CountRunning(ctx, providerID, scope)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrTaskRunning
	}
	item := &syncmodel.SyncTask{
		ProviderID: providerID,
		TaskType:   scope,
		Status:     StatusPending,
	}
	if err := e.syncRepo.CreateTask(ctx, item); err != nil {
		return nil, err
	}
	go e.RunTask(context.Background(), item.ID)
	return item, nil
}

// SyncAll 对一个提供商触发其能力覆盖的全部 scope（定时调度兜底入口）。
// 单个类型失败（含防重入拒绝）不影响其他类型。
func (e *SyncEngine) SyncAll(ctx context.Context, providerID uint64) {
	for _, scope := range e.PlannedScopes(ctx, providerID) {
		if _, err := e.StartSync(ctx, providerID, scope); err != nil {
			e.logger.Warn("sync trigger skipped",
				zap.Uint64("provider_id", providerID),
				zap.String("scope", scope),
				zap.Error(err))
		}
	}
}

// PlannedScopes 计算渠道实际可调度的 scope（适配器不可构建时返回空）。
func (e *SyncEngine) PlannedScopes(ctx context.Context, providerID uint64) []string {
	provider, err := e.buildProvider(ctx, providerID)
	if err != nil {
		return nil
	}
	return PlanScopes(provider.Capabilities(), provider)
}

// EnsureAllSchedules 依各渠道能力描述符补齐缺失的调度行（幂等，供后台列表/定时任务调用）。
func (e *SyncEngine) EnsureAllSchedules(ctx context.Context) {
	providers, err := e.providerRepo.ListAll(ctx)
	if err != nil {
		e.logger.Warn("ensure schedules: list providers failed", zap.Error(err))
		return
	}
	for _, p := range providers {
		if !p.SyncEnabled || p.Status != 1 {
			continue
		}
		e.EnsureSchedulesForProvider(ctx, p.ID)
	}
}

// EnsureSchedulesForProvider 为单个渠道补齐调度行；不覆盖已存在的人工配置。
func (e *SyncEngine) EnsureSchedulesForProvider(ctx context.Context, providerID uint64) {
	if e.frameworkRepo == nil {
		return
	}
	scopes := e.PlannedScopes(ctx, providerID)
	if len(scopes) == 0 {
		return
	}
	existing, err := e.frameworkRepo.ListSchedulesByProvider(ctx, providerID)
	if err != nil {
		e.logger.Warn("ensure schedules: list failed", zap.Uint64("provider_id", providerID), zap.Error(err))
		return
	}
	have := make(map[string]bool, len(existing))
	for _, item := range existing {
		have[item.Scope] = true
	}
	created := 0
	for _, scope := range scopes {
		if have[scope] {
			continue
		}
		handler, ok := GetScopeHandler(scope)
		if !ok {
			continue
		}
		row := &syncmodel.SyncSchedule{
			ProviderID:              providerID,
			Scope:                   scope,
			IntervalSeconds:         handler.DefaultIntervalSeconds,
			FullSyncIntervalSeconds: handler.DefaultFullSyncIntervalSeconds,
			Enabled:                 true,
		}
		if err := e.frameworkRepo.CreateSchedule(ctx, row); err != nil {
			e.logger.Warn("ensure schedules: create failed",
				zap.Uint64("provider_id", providerID), zap.String("scope", scope), zap.Error(err))
			continue
		}
		created++
	}
	if created > 0 {
		e.logger.Info("ensured sync schedules", zap.Uint64("provider_id", providerID), zap.Int("created", created))
	}
}

// RunTask 执行一个同步任务（异步 goroutine 入口）。
func (e *SyncEngine) RunTask(ctx context.Context, taskID uint64) {
	task, err := e.syncRepo.FindTaskByID(ctx, taskID)
	if err != nil {
		e.logger.Error("sync task not found", zap.Uint64("task_id", taskID), zap.Error(err))
		return
	}

	now := time.Now()
	task.Status = StatusRunning
	task.StartedAt = &now
	if err := e.syncRepo.UpdateTask(ctx, task); err != nil {
		e.logger.Error("sync task mark running failed", zap.Uint64("task_id", taskID), zap.Error(err))
	}

	scope := syncmodel.NormalizeScope(task.TaskType)
	task.TaskType = scope
	var runErr error
	handler, ok := GetScopeHandler(scope)
	switch {
	case !ok:
		runErr = &scopeUnsupportedError{scope: scope}
	case true:
		provider, berr := e.buildProvider(ctx, task.ProviderID)
		if berr != nil {
			runErr = berr
		} else if handler.Supported(provider.Capabilities(), provider) {
			runErr = observability.Timed("sync_"+scope, func() error {
				return handler.Run(ctx, e, task, provider)
			})
		} else {
			runErr = &scopeUnsupportedError{scope: scope}
		}
	}

	completed := time.Now()
	task.CompletedAt = &completed
	var skip *scopeUnsupportedError
	switch {
	case errors.As(runErr, &skip):
		// 能力未覆盖：记为"跳过"，不计失败，避免污染渠道健康度。
		task.Status = StatusSkipped
		task.ErrorMessage = runErr.Error()
	case runErr != nil:
		task.Status = StatusFailed
		task.ErrorMessage = describeSyncError(runErr)
	default:
		task.Status = StatusSuccess
	}
	if err := e.syncRepo.UpdateTask(ctx, task); err != nil {
		e.logger.Error("sync task finish failed", zap.Uint64("task_id", taskID), zap.Error(err))
	}
	e.writeLog(ctx, task, runErr)
	e.recordScheduleResult(ctx, task, runErr)
	// T0.2/T0.3：无论成功失败都要 touch last_sync_at，让 sync_interval 真正生效，
	// 并维护熔断健康字段；否则坏渠道会每 60 秒重试、好渠道永不再同步。
	// 跳过不计入健康统计（既不算成功也不算失败）。
	if task.Status != StatusSkipped {
		e.recordProviderHealth(ctx, task, runErr)
	}
}

// recordScheduleResult 回写对应 (provider, scope) 调度行的执行结果与下次到期时间。
func (e *SyncEngine) recordScheduleResult(ctx context.Context, task *syncmodel.SyncTask, runErr error) {
	if e.frameworkRepo == nil {
		return
	}
	schedules, err := e.frameworkRepo.ListSchedulesByProvider(ctx, task.ProviderID)
	if err != nil {
		return
	}
	var current *syncmodel.SyncSchedule
	for i := range schedules {
		if schedules[i].Scope == task.TaskType {
			current = &schedules[i]
			break
		}
	}
	if current == nil {
		return
	}
	status := syncmodel.ScheduleStatusSuccess
	errMsg := ""
	if runErr != nil {
		status = syncmodel.ScheduleStatusFailed
		errMsg = describeSyncError(runErr)
		var skip *scopeUnsupportedError
		if errors.As(runErr, &skip) {
			status = syncmodel.ScheduleStatusSkipped
		}
	}
	interval := current.IntervalSeconds
	if interval <= 0 {
		interval = 3600
	}
	next := time.Now().Add(time.Duration(interval) * time.Second)
	if err := e.frameworkRepo.RecordScheduleRun(ctx, current.ID, status, errMsg, time.Now(), next); err != nil {
		e.logger.Warn("record schedule run failed",
			zap.Uint64("provider_id", task.ProviderID), zap.String("scope", task.TaskType), zap.Error(err))
	}
}

// recordProviderHealth 依据任务终态维护渠道同步健康：成功清零，失败累加计数并在
// 达到阈值时熔断；适配器不可用属永久性错误，立即熔断避免无意义重试刷日志。
func (e *SyncEngine) recordProviderHealth(ctx context.Context, task *syncmodel.SyncTask, runErr error) {
	if runErr == nil {
		if err := e.providerRepo.RecordSyncSuccess(ctx, task.ProviderID); err != nil {
			e.logger.Error("record provider sync success failed",
				zap.Uint64("provider_id", task.ProviderID), zap.Error(err))
		}
		return
	}

	msg := describeSyncError(runErr)
	var unavailable *providerUnavailableError
	if errors.As(runErr, &unavailable) {
		if err := e.providerRepo.PauseSync(ctx, task.ProviderID, msg); err != nil {
			e.logger.Error("pause provider sync failed",
				zap.Uint64("provider_id", task.ProviderID), zap.Error(err))
			return
		}
		e.logger.Warn("provider sync paused: adapter unavailable",
			zap.Uint64("provider_id", task.ProviderID), zap.String("reason", msg))
		return
	}

	if err := e.providerRepo.RecordSyncFailure(ctx, task.ProviderID, msg, maxConsecutiveSyncFailures); err != nil {
		e.logger.Error("record provider sync failure failed",
			zap.Uint64("provider_id", task.ProviderID), zap.Error(err))
	}
}

// CheckProviderReady 调度前置校验：适配器可构建才允许触发；不可用时立即暂停该渠道
// 并写 last_sync_error，返回 false 让调度器跳过。用于避免未接入/错配渠道反复产失败任务。
func (e *SyncEngine) CheckProviderReady(ctx context.Context, providerID uint64) bool {
	if _, err := e.buildProvider(ctx, providerID); err != nil {
		msg := describeSyncError(err)
		if perr := e.providerRepo.PauseSync(ctx, providerID, msg); perr != nil {
			e.logger.Error("pause unavailable provider failed",
				zap.Uint64("provider_id", providerID), zap.Error(perr))
		}
		// 降级日志：仅在暂停时记一条 warn，而非每轮 error。
		e.logger.Warn("skip provider sync: adapter unavailable",
			zap.Uint64("provider_id", providerID), zap.String("reason", msg))
		return false
	}
	return true
}

// ============================================================================
// scope 处理器实现
// ============================================================================

// syncCatalog 拉取上游商品目录并幂等写入 resource_products，
// 同时记录 created/updated/price_changed 差异，超阈值调价进待确认队列（T3.4/T3.5）。
func (e *SyncEngine) syncCatalog(ctx context.Context, task *syncmodel.SyncTask, provider upstream.Provider) error {
	catalog, ok := provider.(upstream.ProductCatalog)
	if !ok {
		return &scopeUnsupportedError{scope: syncmodel.ScopeCatalog}
	}
	local, err := e.localProductIndex(ctx, task.ProviderID)
	if err != nil {
		return err
	}
	items, err := catalog.ListProducts(ctx)
	if err != nil {
		return err
	}
	rows := make([]model.ResourceProduct, 0, len(items))
	diffs := make([]syncmodel.SyncDiff, 0)
	threshold := e.priceThreshold(ctx, task.ProviderID)
	pendingPrices := 0
	for _, it := range items {
		// T2.6：Extra 由"万能袋"升级为受 spec_atoms 字典约束的扩展位——
		// 未登记 key / 越界值只告警不阻断（预埋期避免误伤存量），但会留下可排查记录。
		if issues := specatom.Validate(it.Specs); len(issues) > 0 {
			e.logger.Warn("上游商品规格超出原子字典约束",
				zap.Uint64("provider_id", task.ProviderID),
				zap.String("upstream_id", it.UpstreamID),
				zap.Int("issue_count", len(issues)),
				zap.String("first_issue", issues[0].Error()))
		}
		rows = append(rows, e.convertProduct(task.ProviderID, it))
		prev := local[it.UpstreamID]
		if prev == nil {
			diffs = append(diffs, syncmodel.SyncDiff{
				TaskID: task.ID, ProviderID: task.ProviderID, Scope: syncmodel.ScopeCatalog,
				Action: syncmodel.DiffCreated, ExternalID: it.UpstreamID, NewValue: it.Name,
				Disposition: syncmodel.DiffDispositionApplied,
			})
			continue
		}
		diffs = append(diffs, e.detectProductFieldDiffs(task, prev, it)...)
		n, derr := e.detectPriceChanges(ctx, task, prev, it, threshold)
		if derr != nil {
			e.logger.Warn("记录调价事件失败", zap.Uint64("provider_id", task.ProviderID), zap.Error(derr))
		}
		pendingPrices += n
	}
	if err := e.productRepo.UpsertMany(ctx, task.ProviderID, rows); err != nil {
		return err
	}
	if err := e.frameworkRepo.CreateDiffs(ctx, diffs); err != nil {
		e.logger.Warn("写入同步差异失败", zap.Uint64("provider_id", task.ProviderID), zap.Error(err))
	}
	// 全量对账：上游已不存在的本地商品置 offline（软删除），不物理删除。
	if e.fullSyncDue(ctx, task.ProviderID, syncmodel.ScopeCatalog) {
		keep := make([]string, 0, len(items))
		for _, it := range items {
			keep = append(keep, it.UpstreamID)
		}
		offline, oerr := e.frameworkRepo.MarkResourceProductsOffline(ctx, task.ProviderID, keep)
		if oerr != nil {
			e.logger.Warn("全量对账下线商品失败", zap.Uint64("provider_id", task.ProviderID), zap.Error(oerr))
		} else if offline > 0 {
			offlineDiffs := make([]syncmodel.SyncDiff, 0, offline)
			for id, snap := range local {
				if !containsString(keep, id) {
					offlineDiffs = append(offlineDiffs, syncmodel.SyncDiff{
						TaskID: task.ID, ProviderID: task.ProviderID, Scope: syncmodel.ScopeCatalog,
						Action: syncmodel.DiffOffline, LocalID: snap.ID, ExternalID: id, OldValue: snap.Name,
						Disposition: syncmodel.DiffDispositionApplied, Remark: "上游已下架，本地置为下线",
					})
				}
			}
			if len(offlineDiffs) > 0 {
				_ = e.frameworkRepo.CreateDiffs(ctx, offlineDiffs)
			}
			e.logger.Info("全量对账下线本地商品",
				zap.Uint64("provider_id", task.ProviderID), zap.Int64("rows", offline))
		}
		e.touchCursor(ctx, task.ProviderID, syncmodel.ScopeCatalog, len(items), true)
	} else {
		e.touchCursor(ctx, task.ProviderID, syncmodel.ScopeCatalog, len(items), false)
	}
	if pendingPrices > 0 && e.notifier != nil {
		if err := e.notifier.NotifyPriceChangePending(ctx, task.ProviderID, provider.GetName(), pendingPrices); err != nil {
			e.logger.Warn("调价待确认通知发送失败", zap.Uint64("provider_id", task.ProviderID), zap.Error(err))
		}
	}
	task.TotalCount = len(items)
	task.SuccessCount = len(items)
	return nil
}

// syncPrices 只核对价格（T3.1 catalog 之外的独立节奏）：
// 遍历上游商品，比较 cost_price/sale_price，按阈值写 auto_applied / pending 事件。
func (e *SyncEngine) syncPrices(ctx context.Context, task *syncmodel.SyncTask, provider upstream.Provider) error {
	catalog, ok := provider.(upstream.ProductCatalog)
	if !ok {
		return &scopeUnsupportedError{scope: syncmodel.ScopePrice}
	}
	local, err := e.localProductIndex(ctx, task.ProviderID)
	if err != nil {
		return err
	}
	items, err := catalog.ListProducts(ctx)
	if err != nil {
		return err
	}
	threshold := e.priceThreshold(ctx, task.ProviderID)
	pending := 0
	changed := 0
	for _, it := range items {
		prev := local[it.UpstreamID]
		if prev == nil {
			continue
		}
		n, derr := e.detectPriceChanges(ctx, task, prev, it, threshold)
		if derr != nil {
			return derr
		}
		pending += n
		if n > 0 || !sameFloat(prev.CostPrice, it.CostPrice) {
			changed++
		}
	}
	e.touchCursor(ctx, task.ProviderID, syncmodel.ScopePrice, len(items), false)
	if pending > 0 && e.notifier != nil {
		if err := e.notifier.NotifyPriceChangePending(ctx, task.ProviderID, provider.GetName(), pending); err != nil {
			e.logger.Warn("调价待确认通知发送失败", zap.Uint64("provider_id", task.ProviderID), zap.Error(err))
		}
	}
	task.TotalCount = len(items)
	task.SuccessCount = changed
	return nil
}

// syncPools 拉取上游资源池（节点）并幂等写入 resource_pools，
// 同时将各池容量求和回写上游提供商资源统计。
func (e *SyncEngine) syncPools(ctx context.Context, task *syncmodel.SyncTask, provider upstream.Provider) error {
	return e.syncPoolsInternal(ctx, task, provider, true)
}

// syncRegions 与 syncPools 同源，但只做登记与差异记录，不回写容量统计。
func (e *SyncEngine) syncRegions(ctx context.Context, task *syncmodel.SyncTask, provider upstream.Provider) error {
	return e.syncPoolsInternal(ctx, task, provider, false)
}

func (e *SyncEngine) syncPoolsInternal(ctx context.Context, task *syncmodel.SyncTask, provider upstream.Provider, updateStats bool) error {
	scope := task.TaskType
	poolReader, ok := provider.(upstream.PoolReader)
	if !ok {
		return &scopeUnsupportedError{scope: scope}
	}
	local, err := e.localPoolIndex(ctx, task.ProviderID)
	if err != nil {
		return err
	}
	pools, err := poolReader.ListPools(ctx)
	if err != nil {
		return err
	}
	rows := make([]providermodel.ResourcePool, 0, len(pools))
	var totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk int
	diffs := make([]syncmodel.SyncDiff, 0)
	keep := make([]string, 0, len(pools))
	for _, p := range pools {
		rows = append(rows, e.convertPool(task.ProviderID, p))
		keep = append(keep, p.ID)
		if local[p.ID] == nil {
			diffs = append(diffs, syncmodel.SyncDiff{
				TaskID: task.ID, ProviderID: task.ProviderID, Scope: scope,
				Action: syncmodel.DiffCreated, ExternalID: p.ID, NewValue: p.Name,
				Disposition: syncmodel.DiffDispositionApplied,
			})
		} else if local[p.ID].Name != p.Name {
			diffs = append(diffs, syncmodel.SyncDiff{
				TaskID: task.ID, ProviderID: task.ProviderID, Scope: scope,
				Action: syncmodel.DiffUpdated, LocalID: local[p.ID].ID, ExternalID: p.ID,
				Field: "name", OldValue: local[p.ID].Name, NewValue: p.Name,
				Disposition: syncmodel.DiffDispositionApplied,
			})
		}
		totalCPU += p.TotalCPU
		totalMemory += p.TotalMemory
		totalDisk += p.TotalDisk
		usedCPU += p.UsedCPU
		usedMemory += p.UsedMemory
		usedDisk += p.UsedDisk
	}
	if err := e.poolRepo.UpsertPools(ctx, task.ProviderID, rows); err != nil {
		return err
	}
	if updateStats {
		if err := e.providerRepo.UpdateStats(ctx, task.ProviderID, totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk); err != nil {
			return err
		}
	}
	// 全量对账：上游已消失的池置离线（软删除）。
	if e.fullSyncDue(ctx, task.ProviderID, scope) {
		if offline, oerr := e.frameworkRepo.MarkPoolsOffline(ctx, task.ProviderID, keep); oerr != nil {
			e.logger.Warn("全量对账下线资源池失败", zap.Uint64("provider_id", task.ProviderID), zap.Error(oerr))
		} else if offline > 0 {
			for id, snap := range local {
				if !containsString(keep, id) {
					diffs = append(diffs, syncmodel.SyncDiff{
						TaskID: task.ID, ProviderID: task.ProviderID, Scope: scope,
						Action: syncmodel.DiffOffline, LocalID: snap.ID, ExternalID: id, OldValue: snap.Name,
						Disposition: syncmodel.DiffDispositionApplied, Remark: "上游已消失，本地置为离线",
					})
				}
			}
		}
		e.touchCursor(ctx, task.ProviderID, scope, len(pools), true)
	} else {
		e.touchCursor(ctx, task.ProviderID, scope, len(pools), false)
	}
	if err := e.frameworkRepo.CreateDiffs(ctx, diffs); err != nil {
		e.logger.Warn("写入同步差异失败", zap.Uint64("provider_id", task.ProviderID), zap.Error(err))
	}
	task.TotalCount = len(pools)
	task.SuccessCount = len(pools)
	return nil
}

// syncInstances 拉取上游实例并幂等写入 instances。
func (e *SyncEngine) syncInstances(ctx context.Context, task *syncmodel.SyncTask, provider upstream.Provider) error {
	admin, ok := provider.(upstream.InstanceAdministration)
	if !ok {
		return &scopeUnsupportedError{scope: syncmodel.ScopeInstance}
	}
	items, err := admin.ListInstances(ctx, nil)
	if err != nil {
		return err
	}
	rows := make([]syncmodel.Instance, 0, len(items))
	keep := make([]string, 0, len(items))
	for _, it := range items {
		rows = append(rows, e.convertInstance(task.ProviderID, it))
		keep = append(keep, it.UpstreamID)
	}
	if err := e.syncRepo.UpsertInstances(ctx, rows); err != nil {
		return err
	}
	if e.fullSyncDue(ctx, task.ProviderID, syncmodel.ScopeInstance) {
		offline, oerr := e.frameworkRepo.MarkInstancesOffline(ctx, task.ProviderID, keep)
		if oerr != nil {
			e.logger.Warn("全量对账下线实例失败", zap.Uint64("provider_id", task.ProviderID), zap.Error(oerr))
		} else if offline > 0 {
			e.logger.Info("全量对账下线上游实例",
				zap.Uint64("provider_id", task.ProviderID), zap.Int64("rows", offline))
		}
		e.touchCursor(ctx, task.ProviderID, syncmodel.ScopeInstance, len(items), true)
	} else {
		e.touchCursor(ctx, task.ProviderID, syncmodel.ScopeInstance, len(items), false)
	}
	task.TotalCount = len(items)
	task.SuccessCount = len(items)
	return nil
}

// ============================================================================
// 差异与调价
// ============================================================================

// detectProductFieldDiffs 比较本地快照与上游商品，产出字段级差异（不含价格，价格单独处理）。
func (e *SyncEngine) detectProductFieldDiffs(task *syncmodel.SyncTask, prev *syncrepo.ResourceProductSnapshot, it *pkgmodel.StandardProduct) []syncmodel.SyncDiff {
	type fieldPair struct {
		field string
		old   string
		new   string
	}
	pairs := []fieldPair{
		{"name", prev.Name, it.Name},
		{"cpu", fmt.Sprintf("%d", prev.CPU), fmt.Sprintf("%d", it.Specs.CPU)},
		{"memory", fmt.Sprintf("%d", prev.Memory), fmt.Sprintf("%d", it.Specs.Memory)},
		{"disk", fmt.Sprintf("%d", prev.Disk), fmt.Sprintf("%d", it.Specs.Disk)},
		{"disk_type", prev.DiskType, it.Specs.DiskType},
		{"bandwidth", fmt.Sprintf("%d", prev.Bandwidth), fmt.Sprintf("%d", it.Specs.Bandwidth)},
		{"os", prev.OS, it.Specs.OS},
		{"region", prev.Region, it.Specs.Region},
		{"zone", prev.Zone, it.Specs.Zone},
	}
	out := make([]syncmodel.SyncDiff, 0)
	for _, p := range pairs {
		if strings.TrimSpace(p.old) == strings.TrimSpace(p.new) {
			continue
		}
		out = append(out, syncmodel.SyncDiff{
			TaskID: task.ID, ProviderID: task.ProviderID, Scope: syncmodel.ScopeCatalog,
			Action: syncmodel.DiffUpdated, LocalID: prev.ID, ExternalID: it.UpstreamID,
			Field: p.field, OldValue: p.old, NewValue: p.new,
			Disposition: syncmodel.DiffDispositionApplied,
		})
	}
	return out
}

// detectPriceChanges 比较成本价/建议售价变化：幅度 ≤ 阈值写 auto_applied，
// 超阈值写 pending（绝不静默改售价）。返回本次新增的待确认条数。
func (e *SyncEngine) detectPriceChanges(ctx context.Context, task *syncmodel.SyncTask, prev *syncrepo.ResourceProductSnapshot, it *pkgmodel.StandardProduct, threshold float64) (int, error) {
	pending := 0
	changes := []struct {
		field string
		old   float64
		new   float64
	}{
		{syncmodel.PriceFieldCost, prev.CostPrice, it.CostPrice},
		{syncmodel.PriceFieldSale, prev.SalePrice, it.SalePrice},
	}
	for _, c := range changes {
		if sameFloat(c.old, c.new) {
			continue
		}
		ratio := priceChangeRatio(c.old, c.new)
		status := syncmodel.PriceChangeAutoApplied
		applied := true
		if c.old > 0 && ratio > threshold {
			status = syncmodel.PriceChangePending
			applied = false
		}
		// 同一商品同一字段已有待确认记录时不重复堆积，更新为最新值即可。
		if status == syncmodel.PriceChangePending {
			if existing, err := e.frameworkRepo.FindPendingPriceChange(ctx, task.ProviderID, prev.ID, c.field); err == nil && existing != nil {
				existing.NewValue = floatPtr(c.new)
				existing.ChangeRatio = floatPtr(ratio)
				if err := e.frameworkRepo.UpdatePriceChange(ctx, existing); err != nil {
					return pending, err
				}
				pending++
				continue
			}
			pending++
		}
		event := &syncmodel.PriceChangeEvent{
			ProviderID: task.ProviderID, Scope: syncmodel.ScopePrice,
			ResourceProductID: prev.ID, UpstreamID: it.UpstreamID,
			Field: c.field, OldValue: floatPtr(c.old), NewValue: floatPtr(c.new),
			ChangeRatio: floatPtr(ratio), Threshold: floatPtr(threshold),
			Status: status, Applied: applied,
		}
		if status == syncmodel.PriceChangePending {
			event.Remark = fmt.Sprintf("变动 %.2f%% 超过阈值 %.2f%%，待人工确认", ratio*100, threshold*100)
		}
		if err := e.frameworkRepo.CreatePriceChange(ctx, event); err != nil {
			return pending, err
		}
		if c.field == syncmodel.PriceFieldCost {
			_ = e.frameworkRepo.CreateDiffs(ctx, []syncmodel.SyncDiff{{
				TaskID: task.ID, ProviderID: task.ProviderID, Scope: syncmodel.ScopePrice,
				Action: syncmodel.DiffPriceChanged, LocalID: prev.ID, ExternalID: it.UpstreamID,
				Field: c.field, OldValue: fmt.Sprintf("%.4f", c.old), NewValue: fmt.Sprintf("%.4f", c.new),
				Disposition: dispositionFor(status),
			}})
		}
	}
	return pending, nil
}

func dispositionFor(status string) string {
	if status == syncmodel.PriceChangePending {
		return syncmodel.DiffDispositionPending
	}
	return syncmodel.DiffDispositionApplied
}

// ============================================================================
// 游标与本地快照
// ============================================================================

// fullSyncDue 判断该 (provider, scope) 是否到了全量对账周期。
// 未建立游标 / 无 last_full_at / 超过 full_sync_interval 均视为到期。
func (e *SyncEngine) fullSyncDue(ctx context.Context, providerID uint64, scope string) bool {
	cursor, err := e.frameworkRepo.GetCursor(ctx, providerID, scope)
	if err != nil || cursor == nil || cursor.LastFullAt == nil {
		return true
	}
	interval := e.scheduleFullInterval(ctx, providerID, scope)
	if interval <= 0 {
		interval = 86400
	}
	return time.Since(*cursor.LastFullAt) >= time.Duration(interval)*time.Second
}

func (e *SyncEngine) scheduleFullInterval(ctx context.Context, providerID uint64, scope string) int {
	schedules, err := e.frameworkRepo.ListSchedulesByProvider(ctx, providerID)
	if err != nil {
		return 86400
	}
	for _, s := range schedules {
		if s.Scope == scope && s.FullSyncIntervalSeconds > 0 {
			return s.FullSyncIntervalSeconds
		}
	}
	return 86400
}

// touchCursor 刷新增量游标时间与最近见到的记录数；full=true 时一并刷新全量时间。
func (e *SyncEngine) touchCursor(ctx context.Context, providerID uint64, scope string, seen int, full bool) {
	now := time.Now()
	cursor := &syncmodel.SyncCursor{
		ProviderID: providerID, Scope: scope,
		LastIncrementalAt: &now, LastSeenCount: seen,
	}
	if full {
		cursor.LastFullAt = &now
	}
	if err := e.frameworkRepo.UpsertCursor(ctx, cursor); err != nil {
		e.logger.Warn("更新同步游标失败",
			zap.Uint64("provider_id", providerID), zap.String("scope", scope), zap.Error(err))
	}
}

// localProductIndex 读取本地商品快照并按 upstream_id 建索引。
func (e *SyncEngine) localProductIndex(ctx context.Context, providerID uint64) (map[string]*syncrepo.ResourceProductSnapshot, error) {
	items, err := e.frameworkRepo.ListResourceProductsByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*syncrepo.ResourceProductSnapshot, len(items))
	for i := range items {
		out[items[i].UpstreamID] = &items[i]
	}
	return out, nil
}

// localPoolIndex 读取本地资源池快照。
func (e *SyncEngine) localPoolIndex(ctx context.Context, providerID uint64) (map[string]*syncrepo.PoolSnapshot, error) {
	items, err := e.frameworkRepo.ListPoolSnapshotsByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*syncrepo.PoolSnapshot, len(items))
	for i := range items {
		out[items[i].UpstreamID] = &items[i]
	}
	return out, nil
}

// priceThreshold 读取渠道级调价阈值；未实现 ProviderPricePolicy 或读取失败取默认值。
func (e *SyncEngine) priceThreshold(ctx context.Context, providerID uint64) float64 {
	if policy, ok := e.provider.(ProviderPricePolicy); ok {
		if v, err := policy.PriceChangeThreshold(ctx, providerID); err == nil && v > 0 {
			return v
		}
	}
	return defaultPriceChangeThreshold
}

// ============================================================================
// 工具
// ============================================================================

// buildProvider 构建提供商适配器实例（解密配置 → 工厂实例化）。
// 任一环节失败都视为渠道不可用（永久性），由调用方熔断，不做无限重试。
func (e *SyncEngine) buildProvider(ctx context.Context, providerID uint64) (upstream.Provider, error) {
	cfg, err := e.provider.BuildProviderConfig(ctx, providerID)
	if err != nil {
		return nil, &providerUnavailableError{err: err}
	}
	provider, err := e.upmgr.Build(cfg.Type, cfg)
	if err != nil {
		return nil, &providerUnavailableError{err: err}
	}
	return provider, nil
}

// convertProduct 标准化商品（标准模型 → 存储模型）。
func (e *SyncEngine) convertProduct(providerID uint64, p *pkgmodel.StandardProduct) model.ResourceProduct {
	specJSON, _ := json.Marshal(p.Specs)
	rawJSON, _ := json.Marshal(p.RawSpecs)
	return model.ResourceProduct{
		ProviderID: providerID,
		UpstreamID: p.UpstreamID,
		Name:       p.Name,
		GroupID:    jsonToInt64(p.RawSpecs, "gid"),
		GroupName:  jsonToString(p.RawSpecs, "group_name"),
		CPU:        p.Specs.CPU,
		Memory:     p.Specs.Memory,
		Disk:       p.Specs.Disk,
		DiskType:   p.Specs.DiskType,
		Bandwidth:  p.Specs.Bandwidth,
		OS:         p.Specs.OS,
		Region:     p.Specs.Region,
		Zone:       p.Specs.Zone,
		Specs:      string(specJSON),
		RawSpecs:   string(rawJSON),
		CostPrice:  p.CostPrice,
		SalePrice:  p.SalePrice,
		Status:     normalizeStatus(p.Status, true),
	}
}

// jsonToInt64 从 RawSpecs map 读取 int64 字段（兼容 json.Number/float64）。
func jsonToInt64(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		var n int64
		if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

// jsonToString 从 RawSpecs map 读取 string 字段。
func jsonToString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// convertPool 标准化资源池（标准模型 → 存储模型）。
func (e *SyncEngine) convertPool(providerID uint64, p *upstream.StandardPool) providermodel.ResourcePool {
	return providermodel.ResourcePool{
		ProviderID:  providerID,
		UpstreamID:  p.ID,
		Name:        p.Name,
		PoolType:    p.Type,
		TotalCPU:    p.TotalCPU,
		TotalMemory: p.TotalMemory,
		TotalDisk:   p.TotalDisk,
		UsedCPU:     p.UsedCPU,
		UsedMemory:  p.UsedMemory,
		UsedDisk:    p.UsedDisk,
		Status:      normalizeStatus(p.Status, true),
	}
}

// convertInstance 标准化实例（标准模型 → 存储模型）。
func (e *SyncEngine) convertInstance(providerID uint64, in *pkgmodel.StandardInstance) syncmodel.Instance {
	rawJSON, _ := json.Marshal(in.RawData)
	out := syncmodel.Instance{
		InstanceID: in.UpstreamID,
		ProviderID: providerID,
		UserID:     uint64(in.UserID),
		Name:       in.Name,
		CPU:        in.Specs.CPU,
		Memory:     in.Specs.Memory,
		Disk:       in.Specs.Disk,
		DiskType:   in.Specs.DiskType,
		Bandwidth:  in.Specs.Bandwidth,
		OS:         in.Specs.OS,
		Region:     in.Region,
		Zone:       in.Zone,
		Status:     string(in.Status),
		PrivateIP:  in.PrivateIP,
		PublicIP:   in.PublicIP,
		RawData:    string(rawJSON),
		CreatedAt:  in.CreatedAt,
		// 双链路语义（P1/T1.2）：同步发现的实例 = 上游链路，
		// upstream_product_id 指向 resource_products.id，provider_instance_id 记录上游实例号。
		SourceMode:         syncmodel.SourceModeUpstream,
		UpstreamProductID:  uint64(in.ProductID),
		ProviderInstanceID: in.UpstreamID,
	}
	if !in.ExpireAt.IsZero() {
		exp := in.ExpireAt
		out.ExpireAt = &exp
	}
	return out
}

// normalizeStatus 将上游状态字符串归一化为存储状态整数。
// 空值/未知值按 defaultOn 兜底，避免误判为停用。
func normalizeStatus(s string, defaultOn bool) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "active", "on", "enabled", "online", "up", "running":
		return 1
	case "0", "inactive", "off", "disabled", "offline", "down", "stopped":
		return 0
	}
	if defaultOn {
		return 1
	}
	return 0
}

// writeLog 每次同步写一条 sync_logs 明细。
func (e *SyncEngine) writeLog(ctx context.Context, task *syncmodel.SyncTask, runErr error) {
	logItem := &syncmodel.SyncLog{
		TaskID:       task.ID,
		ProviderID:   task.ProviderID,
		SyncType:     task.TaskType,
		TotalCount:   task.TotalCount,
		SuccessCount: task.SuccessCount,
		Status:       StatusSuccess,
	}
	if runErr != nil {
		logItem.Status = StatusFailed
		logItem.ErrorMessage = describeSyncError(runErr)
		var skip *scopeUnsupportedError
		if errors.As(runErr, &skip) {
			logItem.Status = StatusSkipped
		}
	}
	if err := e.syncRepo.CreateLog(ctx, logItem); err != nil {
		e.logger.Error("sync log write failed", zap.Uint64("task_id", task.ID), zap.Error(err))
	}
}

// describeSyncError 将同步错误格式化为可读描述；适配器未实现时提示「上游不支持」。
func describeSyncError(err error) string {
	if err == nil {
		return ""
	}
	var skip *scopeUnsupportedError
	if errors.As(err, &skip) {
		return skip.Error()
	}
	var notImpl upstream.ErrNotImplemented
	if errors.As(err, &notImpl) {
		return "上游不支持该同步类型"
	}
	var pe *upstream.ProviderError
	if errors.As(err, &pe) {
		return pe.Error()
	}
	return err.Error()
}

// priceChangeRatio 计算变动幅度（相对旧值绝对值）；旧值为 0 时返回 1（视为全量变动）。
func priceChangeRatio(oldValue, newValue float64) float64 {
	if oldValue == 0 {
		if newValue == 0 {
			return 0
		}
		return 1
	}
	return math.Abs(newValue-oldValue) / math.Abs(oldValue)
}

// sameFloat 以 4 位小数精度比较金额/规格浮点，避免 decimal 往返产生伪变更。
func sameFloat(a, b float64) bool {
	return math.Abs(a-b) < 0.00005
}

func floatPtr(v float64) *float64 { return &v }

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
