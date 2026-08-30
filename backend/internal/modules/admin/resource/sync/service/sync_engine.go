package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"hostsent/backend/internal/pkg/upstream"
)

// 同步任务类型
const (
	TaskTypeProduct  = "product"
	TaskTypePool     = "pool"
	TaskTypeInstance = "instance"
)

// 同步任务状态
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// ErrTaskRunning 同一提供商同类型已有同步任务执行中（防重入）。
var ErrTaskRunning = errors.New("已有同步任务在执行中")

// ProviderConfigProvider 根据提供商 ID 构建适配器配置（含解密密钥）。
// 由 providerService 实现，避免 sync 包依赖 provider 具体实现。
type ProviderConfigProvider interface {
	BuildProviderConfig(ctx context.Context, providerID uint64) (*upstream.ProviderConfig, error)
}

// SyncEngine 资源同步引擎：拉取上游商品/资源池/实例数据，标准化后幂等落库，
// 并维护任务状态机（pending → running → success/failed）与同步日志。
type SyncEngine struct {
	upmgr       *upstream.ProviderManager
	provider    ProviderConfigProvider
	productRepo productrepo.ProductRepository
	poolRepo    providerrepo.PoolRepository
	providerRepo providerrepo.ProviderRepository
	syncRepo    syncrepo.SyncRepository
	logger      *zap.Logger
}

// NewSyncEngine 创建同步引擎
func NewSyncEngine(
	upmgr *upstream.ProviderManager,
	provider ProviderConfigProvider,
	productRepo productrepo.ProductRepository,
	poolRepo providerrepo.PoolRepository,
	providerRepo providerrepo.ProviderRepository,
	syncRepo syncrepo.SyncRepository,
	logger *zap.Logger,
) *SyncEngine {
	return &SyncEngine{
		upmgr:        upmgr,
		provider:     provider,
		productRepo:  productRepo,
		poolRepo:     poolRepo,
		providerRepo: providerRepo,
		syncRepo:     syncRepo,
		logger:       logger,
	}
}

// StartSync 创建同步任务并异步执行。
// 同一提供商同类型存在 pending/running 任务时拒绝（防重入）。
func (e *SyncEngine) StartSync(ctx context.Context, providerID uint64, taskType string) (*syncmodel.SyncTask, error) {
	n, err := e.syncRepo.CountRunning(ctx, providerID, taskType)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrTaskRunning
	}
	item := &syncmodel.SyncTask{
		ProviderID: providerID,
		TaskType:   taskType,
		Status:     StatusPending,
	}
	if err := e.syncRepo.CreateTask(ctx, item); err != nil {
		return nil, err
	}
	go e.RunTask(context.Background(), item.ID)
	return item, nil
}

// SyncAll 对一个提供商依次触发三类同步（定时调度用）。
// 单个类型失败（含防重入拒绝）不影响其他类型。
func (e *SyncEngine) SyncAll(ctx context.Context, providerID uint64) {
	for _, taskType := range []string{TaskTypeProduct, TaskTypePool, TaskTypeInstance} {
		if _, err := e.StartSync(ctx, providerID, taskType); err != nil {
			e.logger.Warn("sync trigger skipped",
				zap.Uint64("provider_id", providerID),
				zap.String("task_type", taskType),
				zap.Error(err))
		}
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

	var runErr error
	switch task.TaskType {
	case TaskTypeProduct:
		runErr = e.syncProducts(ctx, task)
	case TaskTypePool:
		runErr = e.syncPools(ctx, task)
	case TaskTypeInstance:
		runErr = e.syncInstances(ctx, task)
	default:
		runErr = fmt.Errorf("未知同步类型 %q", task.TaskType)
	}

	completed := time.Now()
	task.CompletedAt = &completed
	if runErr != nil {
		task.Status = StatusFailed
		task.ErrorMessage = describeSyncError(runErr)
	} else {
		task.Status = StatusSuccess
	}
	if err := e.syncRepo.UpdateTask(ctx, task); err != nil {
		e.logger.Error("sync task finish failed", zap.Uint64("task_id", taskID), zap.Error(err))
	}
	e.writeLog(ctx, task, runErr)
}

// syncProducts 拉取上游商品并幂等写入 resource_products。
func (e *SyncEngine) syncProducts(ctx context.Context, task *syncmodel.SyncTask) error {
	provider, err := e.buildProvider(ctx, task.ProviderID)
	if err != nil {
		return err
	}
	items, err := provider.ListProducts(ctx)
	if err != nil {
		return err
	}
	rows := make([]model.ResourceProduct, 0, len(items))
	for _, it := range items {
		rows = append(rows, e.convertProduct(task.ProviderID, it))
	}
	if err := e.productRepo.UpsertMany(ctx, task.ProviderID, rows); err != nil {
		return err
	}
	task.TotalCount = len(items)
	task.SuccessCount = len(items)
	return nil
}

// syncPools 拉取上游资源池（节点）并幂等写入 resource_pools，
// 同时将各池容量求和回写上游提供商资源统计。
func (e *SyncEngine) syncPools(ctx context.Context, task *syncmodel.SyncTask) error {
	provider, err := e.buildProvider(ctx, task.ProviderID)
	if err != nil {
		return err
	}
	pools, err := provider.ListPools(ctx)
	if err != nil {
		return err
	}
	rows := make([]providermodel.ResourcePool, 0, len(pools))
	var totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk int
	for _, p := range pools {
		rows = append(rows, e.convertPool(task.ProviderID, p))
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
	if err := e.providerRepo.UpdateStats(ctx, task.ProviderID, totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk); err != nil {
		return err
	}
	task.TotalCount = len(pools)
	task.SuccessCount = len(pools)
	return nil
}

// syncInstances 拉取上游实例并幂等写入 instances。
func (e *SyncEngine) syncInstances(ctx context.Context, task *syncmodel.SyncTask) error {
	provider, err := e.buildProvider(ctx, task.ProviderID)
	if err != nil {
		return err
	}
	items, err := provider.ListInstances(ctx, nil)
	if err != nil {
		return err
	}
	rows := make([]syncmodel.Instance, 0, len(items))
	for _, it := range items {
		rows = append(rows, e.convertInstance(task.ProviderID, it))
	}
	if err := e.syncRepo.UpsertInstances(ctx, rows); err != nil {
		return err
	}
	task.TotalCount = len(items)
	task.SuccessCount = len(items)
	return nil
}

// buildProvider 构建提供商适配器实例（解密配置 → 工厂实例化）。
func (e *SyncEngine) buildProvider(ctx context.Context, providerID uint64) (upstream.Provider, error) {
	cfg, err := e.provider.BuildProviderConfig(ctx, providerID)
	if err != nil {
		return nil, err
	}
	provider, err := e.upmgr.Build(cfg.Type, cfg)
	if err != nil {
		return nil, err
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
		ProductID:  uint64(in.ProductID),
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
