// Package repository 提供对账差异台账（reconcile_findings）的读写与
// 「当前时刻全部异常行」的取数。
//
// 与同包的 reconcile_repo.go 分工：那边是**只读列表视图**（分页、筛选、汇总），
// 这边是**扫描与台账**（全量异常行、upsert 差异、把消失的差异置 resolved）。
// 两者共用同一份 baseCTE 与 severityExpr，保证"页面看到的异常"与"落库的差异"
// 口径完全一致——两处各写一份 SQL 迟早会分叉。
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/resource/common/providerkind"
	reconmodel "hostsent/backend/internal/modules/admin/resource/reconcile/model"
)

// FindingRepository 差异台账数据访问能力。
type FindingRepository interface {
	// ListAnomalies 取当前全部命中异常的实例行（不分页、不筛选）——扫描的输入。
	ListAnomalies(ctx context.Context) ([]ReconcileRow, error)
	// UpsertFindings 幂等写入差异行：已存在则刷新 last_seen_at/数值/等级与状态，
	// 返回"新开的行"（first_seen_at == last_seen_at 且此前不存在）。
	UpsertFindings(ctx context.Context, items []reconmodel.ReconcileFinding) (opened []reconmodel.ReconcileFinding, err error)
	// ResolveAbsent 把本次扫描未复现的 open 差异置 resolved，返回处理行数。
	ResolveAbsent(ctx context.Context, seenKeys []string) (int64, error)
	// ListOpen 查当前未处理差异（页面「差异台账」与通知补发的输入）。
	// providerKind 为链路筛选（upstream/compute，空=不过滤）。
	ListOpen(ctx context.Context, providerID uint64, providerKind string, page, pageSize int) ([]reconmodel.ReconcileFinding, int64, error)
	// FindingsForInstances 取指定实例的全部台账行（对账详情页展示"这条差异存在了多久"）。
	FindingsForInstances(ctx context.Context, instanceIDs []uint64) (map[uint64][]reconmodel.ReconcileFinding, error)
	// MarkNotified 落"已发预警"序号与时间。
	MarkNotified(ctx context.Context, id uint64, seq int, at time.Time) error
	// UpdateStatus 处置一条差异（ack/ignored/open），ignored 后不再提醒。
	UpdateStatus(ctx context.Context, id uint64, status string, adminID uint64, note string) error
	// InstanceLabels 批量取实例展示名与渠道名（台账页与通知文案用）。
	InstanceLabels(ctx context.Context, ids []uint64) (map[uint64]InstanceLabel, error)
}

// InstanceLabel 实例的展示信息。
type InstanceLabel struct {
	Name         string
	InstanceID   string
	ProviderName string
	Username     string
}

type findingRepository struct {
	db *gorm.DB
}

// NewFindingRepository 创建差异台账仓储。
func NewFindingRepository(db *gorm.DB) FindingRepository {
	return &findingRepository{db: db}
}

func (r *findingRepository) ListAnomalies(ctx context.Context) ([]ReconcileRow, error) {
	sql := baseCTE + `
        SELECT id, instance_id, name, status, lifecycle_stage, provider_id, user_id,
               sell_product_id, sell_product_name, local_price,
               upstream_product_id, upstream_product_name, upstream_sku, upstream_cost,
               margin_amount, margin_rate, price_anomaly, expire_anomaly, expire_diff_days,
               status_anomaly, COALESCE(raw_domainstatus, '') AS upstream_status,
               expire_at AS local_expire, upstream_expire,
               ` + severityExpr + ` AS severity
          FROM marked
         WHERE ` + severityExpr + ` <> 'ok'
         ORDER BY id`
	var rows []ReconcileRow
	if err := r.db.WithContext(ctx).Raw(sql).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpsertFindings 逐行 upsert。
//
// 为什么不用批量 INSERT ... ON CONFLICT：需要精确知道哪些是"本次新开"的差异
// （只有新开/等级升级才发预警，否则每轮扫描都刷一遍通知）。批量写法拿不到
// 逐行 RowsAffected，只能先查一遍再比，反而更绕。差异行数量是"当前异常实例数"，
// 量级在几百以内，逐行开销可接受（且每个扫描周期只跑一次）。
func (r *findingRepository) UpsertFindings(ctx context.Context, items []reconmodel.ReconcileFinding) ([]reconmodel.ReconcileFinding, error) {
	if len(items) == 0 {
		return nil, nil
	}
	now := time.Now()
	opened := make([]reconmodel.ReconcileFinding, 0)
	for i := range items {
		it := items[i]
		it.FirstSeenAt = now
		it.LastSeenAt = now
		it.CreatedAt = now
		it.UpdatedAt = now

		// 已 resolved 的行若差异复现，要重新按 open 计（first_seen_at 重置为本次），
		// 否则"已持续 N 天"会把上一轮的历史时长也算进来。
		var existing reconmodel.ReconcileFinding
		err := r.db.WithContext(ctx).
			Where("instance_id = ? AND kind = ?", it.InstanceID, it.Kind).
			First(&existing).Error
		switch {
		case err == gorm.ErrRecordNotFound:
			if cerr := r.db.WithContext(ctx).Create(&it).Error; cerr != nil {
				return nil, cerr
			}
			opened = append(opened, it)
			continue
		case err != nil:
			return nil, err
		}

		wasResolved := existing.Status == reconmodel.StatusResolved
		escalated := severityRank(it.Severity) > severityRank(existing.Severity)
		updates := map[string]any{
			"severity":       it.Severity,
			"local_value":    it.LocalValue,
			"upstream_value": it.UpstreamValue,
			"delta":          it.Delta,
			"last_seen_at":   now,
			"updated_at":     now,
			"provider_id":    it.ProviderID,
		}
		if wasResolved {
			// 复现：回到 open，并重置计时起点。
			updates["status"] = reconmodel.StatusOpen
			updates["first_seen_at"] = now
			updates["ack_by"] = nil
			updates["ack_at"] = nil
			updates["note"] = ""
		}
		if err := r.db.WithContext(ctx).Model(&reconmodel.ReconcileFinding{}).
			Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		if wasResolved || escalated {
			it.ID = existing.ID
			it.Status = reconmodel.StatusOpen
			opened = append(opened, it)
		}
	}
	return opened, nil
}

// severityRank 等级排序：danger 高于 warning，用于判定"是否升级"。
func severityRank(s string) int {
	if s == reconmodel.SeverityDanger {
		return 2
	}
	return 1
}

// ResolveAbsent 把不在本次扫描结果里的 open 差异置 resolved。
//
// 判据只有一个：本次仍命中的 (instance_id, kind) 组合集合。**不能**再用
// "本次扫到异常的实例集合" 去限定范围——那会漏掉最典型的一类收敛：实例还是那个
// 实例，但它的异常消失了（比如上游状态改回 Active，或操作员补齐了售价）。
// 这种实例不在异常列表里，用实例集合限定就永远收敛不掉，台账会残留一条
// 已经不存在的问题，持续时长一直涨。
//
// 只处理 open 的行：ack/ignored 是人的决定，不该被自动扫描覆盖。
func (r *findingRepository) ResolveAbsent(ctx context.Context, seenKeys []string) (int64, error) {
	query := r.db.WithContext(ctx).Model(&reconmodel.ReconcileFinding{}).
		Where("status = ?", reconmodel.StatusOpen)
	if len(seenKeys) > 0 {
		query = query.Where("CONCAT(instance_id, ':', kind) NOT IN ?", seenKeys)
	}
	res := query.Updates(map[string]any{
		"status":     reconmodel.StatusResolved,
		"updated_at": time.Now(),
	})
	return res.RowsAffected, res.Error
}

func (r *findingRepository) ListOpen(ctx context.Context, providerID uint64, providerKind string, page, pageSize int) ([]reconmodel.ReconcileFinding, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	base := r.db.WithContext(ctx).Model(&reconmodel.ReconcileFinding{}).Where("status IN ?", []string{reconmodel.StatusOpen, reconmodel.StatusAck})
	if providerID > 0 {
		base = base.Where("provider_id = ?", providerID)
	}
	// 链路过滤：台账表只存 provider_id，链路在 resource_providers.kind 上。
	if cond, condArgs := providerkind.Condition(providerKind, "provider_id"); cond != "" {
		base = base.Where(cond, condArgs...)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []reconmodel.ReconcileFinding
	if err := base.
		Order(clause.OrderByColumn{Column: clause.Column{Name: "severity"}, Desc: true}).
		Order("last_seen_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *findingRepository) FindingsForInstances(ctx context.Context, instanceIDs []uint64) (map[uint64][]reconmodel.ReconcileFinding, error) {
	out := map[uint64][]reconmodel.ReconcileFinding{}
	if len(instanceIDs) == 0 {
		return out, nil
	}
	var items []reconmodel.ReconcileFinding
	if err := r.db.WithContext(ctx).
		Where("instance_id IN ?", instanceIDs).
		Order("instance_id, kind").Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.InstanceID] = append(out[it.InstanceID], it)
	}
	return out, nil
}

func (r *findingRepository) MarkNotified(ctx context.Context, id uint64, seq int, at time.Time) error {
	return r.db.WithContext(ctx).Model(&reconmodel.ReconcileFinding{}).
		Where("id = ?", id).
		Updates(map[string]any{"notice_seq": seq, "notified_at": at, "updated_at": at}).Error
}

func (r *findingRepository) UpdateStatus(ctx context.Context, id uint64, status string, adminID uint64, note string) error {
	updates := map[string]any{"status": status, "updated_at": time.Now()}
	if note != "" {
		updates["note"] = note
	}
	if adminID > 0 {
		now := time.Now()
		updates["ack_by"] = adminID
		updates["ack_at"] = now
	}
	return r.db.WithContext(ctx).Model(&reconmodel.ReconcileFinding{}).Where("id = ?", id).Updates(updates).Error
}

func (r *findingRepository) InstanceLabels(ctx context.Context, ids []uint64) (map[uint64]InstanceLabel, error) {
	out := map[uint64]InstanceLabel{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID           uint64
		Name         string
		InstanceID   string
		ProviderName string
		Username     string
	}
	err := r.db.WithContext(ctx).
		Table("instances i").
		Select("i.id AS id, COALESCE(i.name, '') AS name, COALESCE(i.instance_id, '') AS instance_id, "+
			"COALESCE(p.name, '') AS provider_name, COALESCE(u.username, '') AS username").
		Joins("LEFT JOIN resource_providers p ON p.id = i.provider_id").
		Joins("LEFT JOIN users u ON u.id = i.user_id").
		Where("i.id IN ?", ids).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = InstanceLabel{
			Name: row.Name, InstanceID: row.InstanceID,
			ProviderName: row.ProviderName, Username: row.Username,
		}
	}
	return out, nil
}
