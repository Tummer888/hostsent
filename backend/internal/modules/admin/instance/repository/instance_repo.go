// Package repository 提供实例运维管理台的数据访问实现。
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
)

// ErrNotFound 实例不存在。
var ErrNotFound = errors.New("实例不存在")

// InstanceRow 实例 + 归属用户 + 操作人 + 服务商 的联表行。
//
// 内嵌 syncmodel.Instance 复用既有实例主模型（不新建第三份映射）；
// 查询一律显式 .Table("instances AS instances")，避免内嵌结构体提升的 TableName 干扰。
type InstanceRow struct {
	syncmodel.Instance
	Username     string `gorm:"column:username"`
	UserEmail    string `gorm:"column:user_email"`
	UserPhone    string `gorm:"column:user_phone"`
	ActorName    string `gorm:"column:actor_name"`
	ProviderName string `gorm:"column:provider_name"`
	ProviderType string `gorm:"column:provider_type"`
}

// InstanceRepository 实例运维仓储接口。
type InstanceRepository interface {
	List(ctx context.Context, query *dto.ListQuery) ([]InstanceRow, int64, error)
	FindByID(ctx context.Context, id uint64) (*InstanceRow, error)
	Stats(ctx context.Context, withinDays int) (*dto.StatsResponse, error)
	UpdateRemark(ctx context.Context, id uint64, remark string) error
	UpdateStatus(ctx context.Context, id uint64, status string) error
	ApplySync(ctx context.Context, id uint64, status, publicIP, privateIP, rawData string, syncedAt time.Time) error
	UpdateSpecs(ctx context.Context, id uint64, cpu, memory, disk int, diskType string) error
}

type instanceRepository struct {
	db *gorm.DB
}

// NewInstanceRepository 创建实例运维仓储。
func NewInstanceRepository(db *gorm.DB) InstanceRepository {
	return &instanceRepository{db: db}
}

// instanceSelect 列表/详情统一投影列。
const instanceSelect = "instances.*, " +
	"u.username AS username, u.email AS user_email, u.phone AS user_phone, " +
	"a.username AS actor_name, " +
	"p.name AS provider_name, p.provider_type AS provider_type"

// baseQuery 构造带归属用户/操作人/服务商联表的过滤查询（列表与计数共用）。
func (r *instanceRepository) baseQuery(ctx context.Context, query *dto.ListQuery) *gorm.DB {
	db := r.db.WithContext(ctx).
		Table("instances AS instances").
		Joins("LEFT JOIN users AS u ON u.id = instances.user_id").
		Joins("LEFT JOIN users AS a ON a.id = instances.actor_user_id").
		Joins("LEFT JOIN resource_providers AS p ON p.id = instances.provider_id")

	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("instances.instance_id ILIKE ? OR instances.name ILIKE ?", like, like)
	}
	if kw := strings.TrimSpace(query.UserKeyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("u.username ILIKE ? OR u.email ILIKE ? OR u.phone ILIKE ?", like, like, like)
	}
	if query.UserID > 0 {
		db = db.Where("instances.user_id = ?", query.UserID)
	}
	if query.ProviderID > 0 {
		db = db.Where("instances.provider_id = ?", query.ProviderID)
	}
	if s := strings.TrimSpace(query.Status); s != "" {
		db = db.Where("instances.status = ?", s)
	}
	if sm := strings.TrimSpace(query.SourceMode); sm != "" {
		db = db.Where("instances.source_mode = ?", sm)
	}

	days := query.ExpireWithinDays
	if days <= 0 {
		days = 7
	}
	now := time.Now()
	switch query.ExpireState {
	case dto.ExpireStateExpiring:
		db = db.Where("instances.expire_at IS NOT NULL AND instances.expire_at >= ? AND instances.expire_at <= ?", now, now.AddDate(0, 0, days))
	case dto.ExpireStateExpired:
		db = db.Where("instances.expire_at IS NOT NULL AND instances.expire_at < ?", now)
	case dto.ExpireStateNone:
		db = db.Where("instances.expire_at IS NULL")
	}
	return db
}

// List 跨用户实例分页列表。
func (r *instanceRepository) List(ctx context.Context, query *dto.ListQuery) ([]InstanceRow, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)

	var total int64
	if err := r.baseQuery(ctx, query).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []InstanceRow
	err := r.baseQuery(ctx, query).
		Select(instanceSelect).
		Order("instances.id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// FindByID 按主键查询实例（联表归属用户/服务商）。
func (r *instanceRepository) FindByID(ctx context.Context, id uint64) (*InstanceRow, error) {
	var row InstanceRow
	err := r.db.WithContext(ctx).
		Table("instances AS instances").
		Select(instanceSelect).
		Joins("LEFT JOIN users AS u ON u.id = instances.user_id").
		Joins("LEFT JOIN users AS a ON a.id = instances.actor_user_id").
		Joins("LEFT JOIN resource_providers AS p ON p.id = instances.provider_id").
		Where("instances.id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Stats 全局概览统计（不受列表筛选影响）。
func (r *instanceRepository) Stats(ctx context.Context, withinDays int) (*dto.StatsResponse, error) {
	if withinDays <= 0 {
		withinDays = 7
	}
	now := time.Now()
	var out dto.StatsResponse
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'running')  AS running,
			COUNT(*) FILTER (WHERE status = 'stopped')  AS stopped,
			COUNT(*) FILTER (WHERE status = 'creating') AS creating,
			COUNT(*) FILTER (WHERE status = 'error')    AS "error",
			COUNT(*) FILTER (WHERE expire_at IS NOT NULL AND expire_at >= ? AND expire_at <= ?) AS expiring,
			COUNT(*) FILTER (WHERE expire_at IS NOT NULL AND expire_at < ?) AS expired
		FROM instances`, now, now.AddDate(0, 0, withinDays), now).Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateRemark 更新管理员内部备注。
func (r *instanceRepository) UpdateRemark(ctx context.Context, id uint64, remark string) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("remark", remark).Error
}

// UpdateStatus 更新实例服务状态。
func (r *instanceRepository) UpdateStatus(ctx context.Context, id uint64, status string) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// ApplySync 回源刷新：回写状态、IP、原始数据与回源时间。
func (r *instanceRepository) ApplySync(ctx context.Context, id uint64, status, publicIP, privateIP, rawData string, syncedAt time.Time) error {
	updates := map[string]any{
		"status":         status,
		"last_synced_at": syncedAt,
		"updated_at":     syncedAt,
	}
	// 上游未返回的字段不覆盖本地值，避免把已有 IP 抹成空串。
	if publicIP != "" {
		updates["public_ip"] = publicIP
	}
	if privateIP != "" {
		updates["private_ip"] = privateIP
	}
	if rawData != "" {
		updates["raw_data"] = rawData
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateSpecs 变配后回写规格快照。
func (r *instanceRepository) UpdateSpecs(ctx context.Context, id uint64, cpu, memory, disk int, diskType string) error {
	updates := map[string]any{}
	if cpu > 0 {
		updates["cpu"] = cpu
	}
	if memory > 0 {
		updates["memory"] = memory
	}
	if disk > 0 {
		updates["disk"] = disk
	}
	if diskType != "" {
		updates["disk_type"] = diskType
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// normalizePage 规整分页参数（默认 20，单页上限 100）。
func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
