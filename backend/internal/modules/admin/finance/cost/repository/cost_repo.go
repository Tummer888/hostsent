// Package repository 提供成本管理子域的数据访问。
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/finance/cost/model"
)

// CostRepository 成本管理数据访问。
type CostRepository interface {
	// —— 成本项 ——
	ListItems(ctx context.Context, keyword, category, status string, offset, limit int) ([]model.CostItem, int64, error)
	ListAllActiveItems(ctx context.Context) ([]model.CostItem, error)
	FindItem(ctx context.Context, id uint64) (*model.CostItem, error)
	CreateItem(ctx context.Context, item *model.CostItem) error
	UpdateItem(ctx context.Context, item *model.CostItem) error
	DeleteItem(ctx context.Context, id uint64) error

	// —— 余额快照（只用于展示当前余额与水位告警） ——
	UpsertSnapshot(ctx context.Context, item *model.UpstreamBalanceSnapshot) error
	// LatestSnapshot 不限时间的最新一条快照（余额水位告警取当前余额）。
	LatestSnapshot(ctx context.Context, providerID uint64) (*model.UpstreamBalanceSnapshot, error)

	// —— 上游账本流水（消费/充值，doc111 §5.2） ——
	// UpsertLedgerEntries 批量幂等写入：同 (provider_id, kind, external_id) 覆盖金额/退款/类型。
	UpsertLedgerEntries(ctx context.Context, entries []model.UpstreamLedgerEntry) (int, error)
	// SumLedgerAmount 期间净额：消费=Σ(amount−refund_amount)，充值=Σamount。
	SumLedgerAmount(ctx context.Context, providerID uint64, kind string, start, end time.Time) (float64, error)
	// ListLedgerInRange 区间内全部账本条目（窗口聚合用：趋势与台账共用一次装载）。
	ListLedgerInRange(ctx context.Context, providerID uint64, kind string, start, end time.Time) ([]model.UpstreamLedgerEntry, error)
	// ListLedger 账本分页（倒序，页面流水页签用）。
	ListLedger(ctx context.Context, providerID uint64, kind string, start, end time.Time, offset, limit int) ([]model.UpstreamLedgerEntry, int64, error)
	// MaxLedgerExternalID 已同步的最大上游记录 ID（增量同步断点；无记录时返回 0）。
	MaxLedgerExternalID(ctx context.Context, providerID uint64, kind string) (int64, error)
	// LatestLedgerSyncedAt 该渠道账本最近一次写入时间（同步时间展示）。
	LatestLedgerSyncedAt(ctx context.Context, providerID uint64) (*time.Time, error)
}

type costRepository struct {
	db *gorm.DB
}

// NewCostRepository 创建成本管理仓储。
func NewCostRepository(db *gorm.DB) CostRepository {
	return &costRepository{db: db}
}

func (r *costRepository) ListItems(ctx context.Context, keyword, category, status string, offset, limit int) ([]model.CostItem, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.CostItem{})
	if keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("(name ILIKE ? OR subject ILIKE ? OR remark ILIKE ?)", like, like, like)
	}
	if category != "" {
		base = base.Where("category = ?", category)
	}
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.CostItem
	err := base.Order("status asc, category asc, id desc").
		Offset(offset).
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *costRepository) ListAllActiveItems(ctx context.Context) ([]model.CostItem, error) {
	var items []model.CostItem
	if err := r.db.WithContext(ctx).Model(&model.CostItem{}).
		Where("status = ?", model.StatusActive).
		Order("category asc, id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *costRepository) FindItem(ctx context.Context, id uint64) (*model.CostItem, error) {
	var item model.CostItem
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *costRepository) CreateItem(ctx context.Context, item *model.CostItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *costRepository) UpdateItem(ctx context.Context, item *model.CostItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *costRepository) DeleteItem(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.CostItem{}, id).Error
}

func (r *costRepository) UpsertSnapshot(ctx context.Context, item *model.UpstreamBalanceSnapshot) error {
	// (provider_id, snapshot_date) 唯一：同日重复抓取按覆盖处理（同一天只有一个余额真值）。
	return r.db.WithContext(ctx).Clauses().Transaction(func(tx *gorm.DB) error {
		var existing model.UpstreamBalanceSnapshot
		err := tx.Where("provider_id = ? AND snapshot_date = ?", item.ProviderID, item.SnapshotDate).First(&existing).Error
		if err == nil {
			existing.Balance = item.Balance
			existing.Currency = item.Currency
			existing.Source = item.Source
			existing.Remark = item.Remark
			item.ID = existing.ID
			item.CreatedAt = existing.CreatedAt
			return tx.Save(&existing).Error
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(item).Error
	})
}

func (r *costRepository) LatestSnapshot(ctx context.Context, providerID uint64) (*model.UpstreamBalanceSnapshot, error) {
	var item model.UpstreamBalanceSnapshot
	err := r.db.WithContext(ctx).
		Where("provider_id = ?", providerID).
		Order("snapshot_date desc, id desc").First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ===== 上游账本流水 =====

func (r *costRepository) UpsertLedgerEntries(ctx context.Context, entries []model.UpstreamLedgerEntry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}
	rows := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "provider_id"}, {Name: "kind"}, {Name: "external_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"occurred_at", "amount", "refund_amount", "category", "ref_no", "description", "updated_at",
		}),
	}).CreateInBatches(&entries, 200)
	if rows.Error != nil {
		return 0, rows.Error
	}
	return int(rows.RowsAffected), nil
}

func (r *costRepository) SumLedgerAmount(ctx context.Context, providerID uint64, kind string, start, end time.Time) (float64, error) {
	expr := "COALESCE(SUM(amount), 0)"
	if kind == model.LedgerKindConsume {
		// 消费按净额：退款冲回的金额不计成本（与收入侧的退款口径一致）。
		expr = "COALESCE(SUM(amount - refund_amount), 0)"
	}
	query := r.db.WithContext(ctx).Model(&model.UpstreamLedgerEntry{}).Where("kind = ?", kind)
	// 零值时间表示不加区间过滤（与 ListLedger 一致）：直接拼 zero time 会把区间比成空集，合计恒为 0。
	if !start.IsZero() {
		query = query.Where("occurred_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("occurred_at < ?", end)
	}
	if providerID > 0 {
		query = query.Where("provider_id = ?", providerID)
	}
	var sum float64
	if err := query.Select(expr).Scan(&sum).Error; err != nil {
		return 0, err
	}
	return sum, nil
}

func (r *costRepository) ListLedgerInRange(ctx context.Context, providerID uint64, kind string, start, end time.Time) ([]model.UpstreamLedgerEntry, error) {
	query := r.db.WithContext(ctx).Model(&model.UpstreamLedgerEntry{}).Where("kind = ?", kind)
	// 零值时间表示不加区间过滤（与 ListLedger 一致）：直接拼 zero time 会把区间比成空集，合计恒为 0。
	if !start.IsZero() {
		query = query.Where("occurred_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("occurred_at < ?", end)
	}
	if providerID > 0 {
		query = query.Where("provider_id = ?", providerID)
	}
	items := make([]model.UpstreamLedgerEntry, 0, 64)
	if err := query.Order("occurred_at asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *costRepository) ListLedger(ctx context.Context, providerID uint64, kind string, start, end time.Time, offset, limit int) ([]model.UpstreamLedgerEntry, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.UpstreamLedgerEntry{}).Where("kind = ?", kind)
	if providerID > 0 {
		base = base.Where("provider_id = ?", providerID)
	}
	if !start.IsZero() {
		base = base.Where("occurred_at >= ?", start)
	}
	if !end.IsZero() {
		base = base.Where("occurred_at < ?", end)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.UpstreamLedgerEntry, 0, 32)
	if err := base.Order("occurred_at desc, id desc").Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *costRepository) MaxLedgerExternalID(ctx context.Context, providerID uint64, kind string) (int64, error) {
	// external_id 是上游自增 ID 的字符串形式；非数字（异常数据）按 0 处理，不会误判断点。
	var maxID int64
	err := r.db.WithContext(ctx).Model(&model.UpstreamLedgerEntry{}).
		Where("provider_id = ? AND kind = ? AND external_id ~ '^[0-9]+$'", providerID, kind).
		Select("COALESCE(MAX(external_id::bigint), 0)").Scan(&maxID).Error
	if err != nil {
		return 0, err
	}
	return maxID, nil
}

func (r *costRepository) LatestLedgerSyncedAt(ctx context.Context, providerID uint64) (*time.Time, error) {
	// providerID=0 表示全部渠道：不加 where（否则会按 provider_id=0 查，永远查不到）。
	query := r.db.WithContext(ctx).Model(&model.UpstreamLedgerEntry{})
	if providerID > 0 {
		query = query.Where("provider_id = ?", providerID)
	}
	var latest *time.Time
	if err := query.Select("MAX(created_at)").Scan(&latest).Error; err != nil {
		return nil, err
	}
	return latest, nil
}
