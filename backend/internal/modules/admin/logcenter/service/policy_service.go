package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/logcenter/catalog"
	"hostsent/backend/internal/modules/admin/logcenter/dto"
	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
)

// ErrPolicyRejected 策略不可用/参数非法（API 层映射为 20001）。
var ErrPolicyRejected = errors.New("logcenter: policy rejected")

// PolicyService 保留策略服务（doc92 §4.3 / §8.4）。
type PolicyService interface {
	List(ctx context.Context) ([]dto.PolicyInfo, error)
	Update(ctx context.Context, sourceKey string, req dto.PolicyUpdateRequest, operatorID uint64) (*dto.PolicyInfo, error)
	Get(ctx context.Context, sourceKey string) (*logmodel.LogRetentionPolicy, error)
	// EffectiveRetention 解算某源的实际保留天数：策略行 → 注册表默认，且不低于硬地板。
	EffectiveRetention(ctx context.Context, src *catalog.Source) int
	// ResolveWatermark 解算删除水位线（doc92 §6.2）。
	ResolveWatermark(ctx context.Context, src *catalog.Source, requested time.Time) time.Time
	Seed(ctx context.Context) error
}

type policyService struct {
	repo       logrepo.PolicyRepository
	logger     *zap.Logger
	globalDays int
}

// NewPolicyService 创建策略服务；globalDays 为全局保留期（log_retention_days），
// <=0 时回落到 180。
func NewPolicyService(repo logrepo.PolicyRepository, globalDays int, logger *zap.Logger) PolicyService {
	if globalDays <= 0 {
		globalDays = catalog.DefaultRetentionDays
	}
	return &policyService{repo: repo, logger: logger, globalDays: globalDays}
}

// SetGlobalDays 刷新全局保留期（配置热更新时调用）。
func (s *policyService) setGlobalDays(days int) {
	if days > 0 {
		s.globalDays = days
	}
}

// globalRetention 当前全局保留天数（不低于硬地板）。
func (s *policyService) globalRetention() int {
	if s.globalDays < catalog.MinRetentionDays {
		return catalog.MinRetentionDays
	}
	return s.globalDays
}

// Seed 幂等写入 26 行 seed 策略（不覆盖运营已改过的行）。
func (s *policyService) Seed(ctx context.Context) error {
	rows := make([]logmodel.LogRetentionPolicy, 0, 26)
	for _, src := range catalog.All() {
		action := src.EffectiveAction()
		cron := ""
		if action == catalog.ActionArchiveOnly {
			cron = "monthly"
		}
		rows = append(rows, logmodel.LogRetentionPolicy{
			SourceKey:     src.Key,
			DisplayName:   src.DisplayName,
			Action:        action,
			RetentionDays: src.DefaultRetention(),
			ArchiveCron:   cron,
			BatchSize:     5000,
			Enabled:       true,
			Remark:        src.Remark,
		})
	}
	return s.repo.UpsertSeed(ctx, rows)
}

// List 返回全部源的生效策略（以注册表为准补齐缺失行，避免 seed 未跑时页面空白）。
func (s *policyService) List(ctx context.Context) ([]dto.PolicyInfo, error) {
	stored, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]logmodel.LogRetentionPolicy, len(stored))
	for _, row := range stored {
		byKey[row.SourceKey] = row
	}
	out := make([]dto.PolicyInfo, 0, len(catalog.All()))
	for _, src := range catalog.All() {
		item := dto.PolicyInfo{
			SourceKey:            src.Key,
			DisplayName:          src.DisplayName,
			Group:                src.Group,
			GroupLabel:           catalog.GroupLabel(src.Group),
			Class:                src.Class,
			Cleanable:            src.Cleanable,
			Action:               src.EffectiveAction(),
			RetentionDays:        src.DefaultRetention(),
			BatchSize:            5000,
			Enabled:              true,
			Remark:               src.Remark,
			IndependentPage:      src.IndependentPage,
			DefaultRetentionDays: src.DefaultRetention(),
			TimeColumn:           src.TimeColumn,
			Table:                src.Table,
			DeleteGuard:          src.DeleteGuard,
		}
		if row, ok := byKey[src.Key]; ok {
			item.Action = row.Action
			item.RetentionDays = row.RetentionDays
			item.BatchSize = row.BatchSize
			item.Enabled = row.Enabled
			item.ArchiveCron = row.ArchiveCron
			item.LastRunAt = fmtTimePtr(row.LastRunAt)
			item.LastDeleted = row.LastDeleted
			item.Configured = true
			if strings.TrimSpace(row.Remark) != "" {
				item.Remark = row.Remark
			}
		}
		// 不可清理的源在回显上也绝不能出现 clean（防止手工改库后前端展示成可清）。
		if !src.Cleanable && item.Action == catalog.ActionClean {
			item.Action = catalog.ActionArchiveOnly
		}
		item.ActionLabel = catalog.ActionLabel(item.Action)
		out = append(out, item)
	}
	return out, nil
}

// Get 取策略行；不存在时返回注册表默认值（不落库）。
func (s *policyService) Get(ctx context.Context, sourceKey string) (*logmodel.LogRetentionPolicy, error) {
	src := catalog.Get(sourceKey)
	if src == nil {
		return nil, fmt.Errorf("%w: 未知日志源 %q", ErrPolicyRejected, sourceKey)
	}
	row, err := s.repo.FindBySource(ctx, sourceKey)
	if err != nil {
		return nil, err
	}
	if row != nil {
		return row, nil
	}
	return &logmodel.LogRetentionPolicy{
		SourceKey:     src.Key,
		DisplayName:   src.DisplayName,
		Action:        src.EffectiveAction(),
		RetentionDays: src.DefaultRetention(),
		BatchSize:     5000,
		Enabled:       true,
	}, nil
}

// Update 更新策略。
//
// 三条硬约束（doc92 §4.1/§6.2）：
//  1. 只有 Cleanable=true 的源才允许 action=clean —— 这是「仅运维日志可清」的实现；
//  2. retention_days 不得低于 minRetentionDays=7（代码常量，任何来源都不能突破）；
//  3. 未知 source_key 直接拒绝（防拼错 key 静默通过）。
func (s *policyService) Update(ctx context.Context, sourceKey string, req dto.PolicyUpdateRequest, operatorID uint64) (*dto.PolicyInfo, error) {
	src := catalog.Get(sourceKey)
	if src == nil {
		return nil, fmt.Errorf("%w: 未知日志源 %q", ErrPolicyRejected, sourceKey)
	}
	action := strings.TrimSpace(req.Action)
	switch action {
	case catalog.ActionClean:
		if !src.Cleanable {
			return nil, fmt.Errorf("%w: 源 %s 属于 %s 类，只允许 archive_only 或 keep",
				ErrPolicyRejected, src.Key, src.Class)
		}
	case catalog.ActionArchiveOnly, catalog.ActionKeep:
	default:
		return nil, fmt.Errorf("%w: 不支持的动作 %q", ErrPolicyRejected, action)
	}

	days := req.RetentionDays
	if days <= 0 {
		days = src.DefaultRetention()
	}
	if days < catalog.MinRetentionDays {
		// 硬地板：不允许把保留期设成 0 导致「删光所有日志」。
		days = catalog.MinRetentionDays
	}
	batch := req.BatchSize
	if batch <= 0 {
		batch = 5000
	}
	if batch > 50000 {
		batch = 50000
	}

	row := &logmodel.LogRetentionPolicy{
		SourceKey:     src.Key,
		DisplayName:   src.DisplayName,
		Action:        action,
		RetentionDays: days,
		BatchSize:     batch,
		Enabled:       req.Enabled,
		ArchiveCron:   strings.TrimSpace(req.ArchiveCron),
		UpdatedBy:     operatorID,
		Remark:        strings.TrimSpace(req.Remark),
	}
	if row.ArchiveCron == "" && action == catalog.ActionArchiveOnly {
		row.ArchiveCron = "monthly"
	}
	if err := s.repo.Update(ctx, row); err != nil {
		return nil, err
	}
	// 策略行可能尚未 seed（例如新源上线）：回读确认，缺失则补插。
	if existing, err := s.repo.FindBySource(ctx, src.Key); err != nil {
		return nil, err
	} else if existing == nil {
		if err := s.repo.UpsertSeed(ctx, []logmodel.LogRetentionPolicy{*row}); err != nil {
			return nil, err
		}
	}
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].SourceKey == src.Key {
			return &items[i], nil
		}
	}
	return nil, nil
}

// EffectiveRetention 解算实际保留天数。
func (s *policyService) EffectiveRetention(ctx context.Context, src *catalog.Source) int {
	if row, err := s.repo.FindBySource(ctx, src.Key); err == nil && row != nil {
		if row.RetentionDays > 0 {
			if row.RetentionDays < catalog.MinRetentionDays {
				return catalog.MinRetentionDays
			}
			return row.RetentionDays
		}
	}
	return src.DefaultRetention()
}

// ResolveWatermark 解算删除水位线（doc92 §6.2）。
//
// 规则：取「请求值（若更早）」与「策略保留期下界」中更早的那个 —— 即请求只能
// 要求删得更早的东西，不能越过保留期地板把新数据删掉。7 天是代码硬地板。
func (s *policyService) ResolveWatermark(ctx context.Context, src *catalog.Source, requested time.Time) time.Time {
	days := s.EffectiveRetention(ctx, src)
	if days < catalog.MinRetentionDays {
		days = catalog.MinRetentionDays
	}
	floor := time.Now().AddDate(0, 0, -days)
	if requested.IsZero() || requested.After(floor) {
		return floor
	}
	// 请求更早：仍不得早于全局保留期（避免一次删掉整个历史）。
	globalFloor := time.Now().AddDate(0, 0, -s.globalRetention())
	if requested.Before(globalFloor) {
		return globalFloor
	}
	return requested
}
