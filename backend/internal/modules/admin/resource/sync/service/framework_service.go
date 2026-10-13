package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/resource/common/providerkind"
	"hostsent/backend/internal/modules/admin/resource/sync/dto"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
)

// ============================================================================
// P3 同步框架业务服务（T3.2 调度配置 / T3.4 待确认调价 / T3.5 差异与对账）
// ============================================================================

// FrameworkService 同步框架业务能力。
type FrameworkService interface {
	ListSchedules(ctx context.Context, query dto.SyncScheduleListQuery) (*dto.SyncScheduleListResponse, error)
	UpdateSchedule(ctx context.Context, id uint64, req dto.SyncScheduleUpdateRequest) (*dto.SyncScheduleInfo, error)
	SyncScopeMeta(ctx context.Context) []dto.SyncScopeMeta
	// ListPriceChanges 待确认/已处置调价列表。
	ListPriceChanges(ctx context.Context, query dto.PriceChangeListQuery) (*dto.PriceChangeListResponse, error)
	// HandlePriceChanges 批量确认/驳回：确认后把新成本价应用到关联售出商品并记 product_history。
	HandlePriceChanges(ctx context.Context, req dto.PriceChangeHandleRequest, operatorID uint64, operatorName string) (int, error)
	ListDiffs(ctx context.Context, query dto.SyncDiffListQuery) (*dto.SyncDiffListResponse, error)
	DiffSummary(ctx context.Context, providerID uint64, days int) (*dto.SyncDiffSummaryResponse, error)
}

// ErrPriceChangeHandled 调价事件已被处置。
var ErrPriceChangeHandled = errors.New("该调价事件已处置")

type frameworkService struct {
	fwRepo       syncrepo.FrameworkRepository
	providerRepo syncrepo.ProviderLister
	engine       *SyncEngine
	logger       *zap.Logger
}

// NewFrameworkService 创建同步框架业务服务。
func NewFrameworkService(fwRepo syncrepo.FrameworkRepository, engine *SyncEngine, logger *zap.Logger) FrameworkService {
	return &frameworkService{fwRepo: fwRepo, engine: engine, logger: logger}
}

// ProviderMeta 渠道展示元信息（名称 + 链路），列表页据此渲染渠道列与链路标签。
type ProviderMeta struct {
	Name string
	Kind string
}

// providerIndex 返回 providerID → {名称, 链路}（失败时返回空表，不阻塞列表）。
//
// 与 providerNames 并存而不合并：后者只给名称、已被多处调用，这里多带一个链路字段，
// 两者共用同一次 ListAll 反查，不额外查库。
func (s *frameworkService) providerIndex(ctx context.Context, providerID uint64) map[uint64]ProviderMeta {
	out := map[uint64]ProviderMeta{}
	if s.engine == nil || s.engine.providerRepo == nil {
		return out
	}
	items, err := s.engine.providerRepo.ListAll(ctx)
	if err != nil {
		return out
	}
	for _, item := range items {
		if providerID > 0 && item.ID != providerID {
			continue
		}
		out[item.ID] = ProviderMeta{Name: item.Name, Kind: item.Kind}
	}
	return out
}

// providerNames 返回 providerID → 名称（失败时返回空表，不阻塞列表）。
func (s *frameworkService) providerNames(ctx context.Context, providerID uint64) map[uint64]string {
	out := map[uint64]string{}
	for id, meta := range s.providerIndex(ctx, providerID) {
		out[id] = meta.Name
	}
	return out
}

func (s *frameworkService) ListSchedules(ctx context.Context, query dto.SyncScheduleListQuery) (*dto.SyncScheduleListResponse, error) {
	// 调度行按需懒创建：列表打开时先确保该渠道的调度行存在，避免"页面空荡荡"。
	if query.ProviderID > 0 {
		s.engine.EnsureSchedulesForProvider(ctx, query.ProviderID)
	} else {
		s.engine.EnsureAllSchedules(ctx)
	}
	var items []syncmodel.SyncSchedule
	var err error
	if query.ProviderID > 0 {
		items, err = s.fwRepo.ListSchedulesByProvider(ctx, query.ProviderID)
	} else {
		items, err = s.fwRepo.ListAllSchedules(ctx)
	}
	if err != nil {
		return nil, err
	}
	meta := s.providerIndex(ctx, query.ProviderID)
	// 链路筛选在服务层补一道：调度行是从 ListAllSchedules 全量取回后在内存分页的，
	// 仓储没有对应的 SQL 过滤点，所以在装配处剔除更直接（口径与 providerkind.Normalize 一致）。
	kindFilter := providerkind.Normalize(query.ProviderKind)
	respItems := make([]dto.SyncScheduleInfo, 0, len(items))
	for _, item := range items {
		if query.Enabled != nil && item.Enabled != *query.Enabled {
			continue
		}
		m := meta[item.ProviderID]
		if kindFilter != "" && m.Kind != kindFilter {
			continue
		}
		respItems = append(respItems, buildScheduleInfo(item, m.Name, m.Kind))
	}
	return &dto.SyncScheduleListResponse{
		Items: respItems,
		Meta:  dto.ListMeta{Page: 1, PageSize: len(respItems), Total: int64(len(respItems))},
	}, nil
}

func (s *frameworkService) UpdateSchedule(ctx context.Context, id uint64, req dto.SyncScheduleUpdateRequest) (*dto.SyncScheduleInfo, error) {
	items, err := s.fwRepo.ListAllSchedules(ctx)
	if err != nil {
		return nil, err
	}
	var target *syncmodel.SyncSchedule
	for i := range items {
		if items[i].ID == id {
			target = &items[i]
			break
		}
	}
	if target == nil {
		return nil, syncrepo.ErrScheduleNotFound
	}
	if req.IntervalSeconds != nil {
		if *req.IntervalSeconds < 30 {
			return nil, errors.New("同步间隔不得小于 30 秒")
		}
		target.IntervalSeconds = *req.IntervalSeconds
	}
	if req.FullSyncIntervalSeconds != nil {
		if *req.FullSyncIntervalSeconds < 300 {
			return nil, errors.New("全量对账周期不得小于 300 秒")
		}
		target.FullSyncIntervalSeconds = *req.FullSyncIntervalSeconds
	}
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		target.Priority = *req.Priority
	}
	if req.WindowClear {
		target.WindowStart = nil
		target.WindowEnd = nil
	} else if req.WindowStart != nil || req.WindowEnd != nil {
		start, end := req.WindowStart, req.WindowEnd
		if start != nil && (*start < 0 || *start > 23) {
			return nil, errors.New("时间窗起始小时须在 0-23")
		}
		if end != nil && (*end < 0 || *end > 23) {
			return nil, errors.New("时间窗结束小时须在 0-23")
		}
		// 只传一端时，另一端沿用现值；都为空视为不限。
		if start == nil {
			start = target.WindowStart
		}
		if end == nil {
			end = target.WindowEnd
		}
		target.WindowStart = start
		target.WindowEnd = end
	}
	if req.ResetNextRun {
		now := time.Now()
		target.NextRunAt = &now
	} else if req.IntervalSeconds != nil || req.Enabled != nil {
		// 节奏变化立即按新间隔重排，避免沿用旧的长周期。
		next := time.Now().Add(time.Duration(target.IntervalSeconds) * time.Second)
		target.NextRunAt = &next
	}
	if err := s.fwRepo.UpdateScheduleConfig(ctx, target); err != nil {
		return nil, err
	}
	meta := s.providerIndex(ctx, target.ProviderID)
	m := meta[target.ProviderID]
	info := buildScheduleInfo(*target, m.Name, m.Kind)
	return &info, nil
}

func (s *frameworkService) SyncScopeMeta(_ context.Context) []dto.SyncScopeMeta {
	handlers := RegisteredScopeHandlers()
	out := make([]dto.SyncScopeMeta, 0, len(handlers))
	for _, h := range handlers {
		out = append(out, dto.SyncScopeMeta{
			Scope:           h.Scope,
			Name:            h.DisplayName,
			DefaultInterval: h.DefaultIntervalSeconds,
		})
	}
	return out
}

func (s *frameworkService) ListPriceChanges(ctx context.Context, query dto.PriceChangeListQuery) (*dto.PriceChangeListResponse, error) {
	items, total, err := s.fwRepo.ListPriceChanges(ctx, query)
	if err != nil {
		return nil, err
	}
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	respItems := make([]dto.PriceChangeInfo, 0, len(items))
	meta := s.providerIndex(ctx, query.ProviderID)
	// 商品名批量反查：调价事件列表的主体应是"哪个商品"，只有 ID 的话运营得去商品目录对号。
	// 同页最多一页的条数，一次 IN 查询即可。
	productIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.ResourceProductID > 0 {
			productIDs = append(productIDs, item.ResourceProductID)
		}
	}
	productNames, err := s.fwRepo.ResourceProductNames(ctx, productIDs)
	if err != nil {
		// 名称取不到不阻塞列表：主键与价格变化仍在，页面降级为显示 ID。
		productNames = map[uint64]string{}
	}
	for _, item := range items {
		respItems = append(respItems, buildPriceChangeInfo(item, meta[item.ProviderID], productNames[item.ResourceProductID]))
	}
	return &dto.PriceChangeListResponse{
		Items: respItems,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

// HandlePriceChanges 批量处置调价：
//   - confirm：把新成本价应用到关联售出商品（写 product_history + 通知，由 PriceApplier 负责）
//   - reject ：保持原售价，仅登记驳回原因
//
// 已处置的记录跳过（幂等）。
func (s *frameworkService) HandlePriceChanges(ctx context.Context, req dto.PriceChangeHandleRequest, operatorID uint64, operatorName string) (int, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "confirm" && action != "reject" {
		return 0, errors.New("action 仅支持 confirm / reject")
	}
	handled := 0
	for _, id := range req.IDs {
		item, err := s.fwRepo.FindPriceChange(ctx, id)
		if err != nil {
			return handled, err
		}
		if item.Status != syncmodel.PriceChangePending {
			continue
		}
		now := time.Now()
		item.HandledBy = operatorID
		item.HandledAt = &now
		item.Remark = strings.TrimSpace(req.Remark)
		if action == "reject" {
			item.Status = syncmodel.PriceChangeRejected
			item.Applied = false
			if item.Remark == "" {
				item.Remark = "人工驳回，保持原价"
			}
			if err := s.fwRepo.UpdatePriceChange(ctx, item); err != nil {
				return handled, err
			}
			handled++
			continue
		}
		item.Status = syncmodel.PriceChangeConfirmed
		item.Applied = true
		if item.NewValue != nil && item.ResourceProductID > 0 && s.engine != nil && s.engine.priceApplier != nil {
			affected, aerr := s.engine.priceApplier.ApplyConfirmedPrice(
				ctx, item.ResourceProductID, *item.NewValue, operatorName, item.Remark)
			if aerr != nil {
				s.logger.Warn("应用已确认调价失败",
					zap.Uint64("event_id", item.ID), zap.Uint64("resource_product_id", item.ResourceProductID), zap.Error(aerr))
				return handled, aerr
			}
			if item.Remark == "" {
				item.Remark = fmt.Sprintf("人工确认，已更新 %d 个售出商品", affected)
			}
		}
		if err := s.fwRepo.UpdatePriceChange(ctx, item); err != nil {
			return handled, err
		}
		handled++
	}
	return handled, nil
}

func (s *frameworkService) ListDiffs(ctx context.Context, query dto.SyncDiffListQuery) (*dto.SyncDiffListResponse, error) {
	items, total, err := s.fwRepo.ListDiffs(ctx, query)
	if err != nil {
		return nil, err
	}
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	meta := s.providerIndex(ctx, query.ProviderID)
	respItems := make([]dto.SyncDiffInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildSyncDiffInfo(item, meta[item.ProviderID]))
	}
	return &dto.SyncDiffListResponse{
		Items: respItems,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

// DiffSummary 按渠道汇总差异（对账页真实数据源）。
func (s *frameworkService) DiffSummary(ctx context.Context, providerID uint64, days int) (*dto.SyncDiffSummaryResponse, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	names := s.providerNames(ctx, providerID)
	ids := make([]uint64, 0, len(names))
	if providerID > 0 {
		ids = append(ids, providerID)
	} else {
		for id := range names {
			ids = append(ids, id)
		}
	}
	items := make([]dto.SyncDiffSummaryProvider, 0, len(ids))
	for _, id := range ids {
		byAction, err := s.fwRepo.CountDiffsByAction(ctx, id, since)
		if err != nil {
			return nil, err
		}
		var total int64
		for _, n := range byAction {
			total += n
		}
		items = append(items, dto.SyncDiffSummaryProvider{
			ProviderID: id,
			Total:      total,
			ByAction:   byAction,
		})
	}
	return &dto.SyncDiffSummaryResponse{Items: items}, nil
}

// ============================================================================
// 展示构建
// ============================================================================

func buildScheduleInfo(item syncmodel.SyncSchedule, providerName, providerKind string) dto.SyncScheduleInfo {
	info := dto.SyncScheduleInfo{
		ID:                      item.ID,
		ProviderID:              item.ProviderID,
		ProviderName:            providerName,
		ProviderKind:            providerKind,
		Scope:                   item.Scope,
		ScopeName:               ScopeDisplayName(item.Scope),
		IntervalSeconds:         item.IntervalSeconds,
		FullSyncIntervalSeconds: item.FullSyncIntervalSeconds,
		Enabled:                 item.Enabled,
		Priority:                item.Priority,
		WindowStart:             item.WindowStart,
		WindowEnd:               item.WindowEnd,
		LastStatus:              item.LastStatus,
		LastError:               item.LastError,
	}
	if item.LastRunAt != nil {
		v := item.LastRunAt.Format(time.RFC3339)
		info.LastRunAt = &v
	}
	if item.NextRunAt != nil {
		v := item.NextRunAt.Format(time.RFC3339)
		info.NextRunAt = &v
	}
	return info
}

func buildPriceChangeInfo(item syncmodel.PriceChangeEvent, meta ProviderMeta, resourceProductName string) dto.PriceChangeInfo {
	info := dto.PriceChangeInfo{
		ID:                  item.ID,
		ProviderID:          item.ProviderID,
		ProviderKind:        meta.Kind,
		Scope:               item.Scope,
		ResourceProductID:   item.ResourceProductID,
		ResourceProductName: resourceProductName,
		UpstreamID:          item.UpstreamID,
		ProductID:           item.ProductID,
		Field:               item.Field,
		OldValue:            item.OldValue,
		NewValue:            item.NewValue,
		ChangeRatio:         item.ChangeRatio,
		Threshold:           item.Threshold,
		Status:              item.Status,
		Applied:             item.Applied,
		Remark:              item.Remark,
		CreatedAt:           item.CreatedAt.Format(time.RFC3339),
	}
	if item.HandledAt != nil {
		v := item.HandledAt.Format(time.RFC3339)
		info.HandledAt = &v
	}
	return info
}

func buildSyncDiffInfo(item syncmodel.SyncDiff, meta ProviderMeta) dto.SyncDiffInfo {
	return dto.SyncDiffInfo{
		ID:           item.ID,
		TaskID:       item.TaskID,
		ProviderID:   item.ProviderID,
		ProviderName: meta.Name,
		ProviderKind: meta.Kind,
		Scope:        item.Scope,
		Action:       item.Action,
		LocalID:      item.LocalID,
		ExternalID:   item.ExternalID,
		Field:        item.Field,
		OldValue:     item.OldValue,
		NewValue:     item.NewValue,
		Disposition:  item.Disposition,
		Remark:       item.Remark,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}
}

// ParseUint 供 handler 复用的 ID 解析辅助。
func ParseUint(raw string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
}
