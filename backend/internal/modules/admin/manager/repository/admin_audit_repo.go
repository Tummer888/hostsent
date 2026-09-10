package repository

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/model"
)

// AdminAuditRepository 管理端操作审计落库与查询。
type AdminAuditRepository interface {
	CreateAuditLog(ctx context.Context, log *model.AdminAuditLog) error
	// ListAuditLogs 查询审计日志（倒序分页）。
	ListAuditLogs(ctx context.Context, q dto.AdminAuditLogQuery) ([]model.AdminAuditLog, int64, error)
}

type adminAuditRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewAdminAuditRepository 创建审计日志仓储。
func NewAdminAuditRepository(db *gorm.DB, logger *zap.Logger) AdminAuditRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &adminAuditRepository{db: db, logger: logger}
}

func (r *adminAuditRepository) CreateAuditLog(ctx context.Context, log *model.AdminAuditLog) error {
	if log == nil {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		// 审计写入失败不影响业务响应，仅记录
		r.logger.Warn("admin audit: create failed",
			zap.Uint64("admin_id", log.AdminID),
			zap.String("path", log.RequestPath),
			zap.Error(err))
		return err
	}
	return nil
}

func (r *adminAuditRepository) ListAuditLogs(ctx context.Context, q dto.AdminAuditLogQuery) ([]model.AdminAuditLog, int64, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	base := r.db.WithContext(ctx).Model(&model.AdminAuditLog{})
	if q.AdminID > 0 {
		base = base.Where("admin_id = ?", q.AdminID)
	}
	if q.ResourceType != "" {
		base = base.Where("resource_type = ?", q.ResourceType)
	}
	if q.Action != "" {
		base = base.Where("action = ?", q.Action)
	}
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("admin_name ILIKE ? OR request_path ILIKE ?", like, like)
	}
	if start, err := parseTime(q.StartTime); err == nil && start != nil {
		base = base.Where("created_at >= ?", *start)
	}
	if end, err := parseTime(q.EndTime); err == nil && end != nil {
		base = base.Where("created_at <= ?", *end)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.AdminAuditLog
	if err := base.Order("created_at desc, id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// parseTime 解析 YYYY-MM-DD 或 RFC3339；空串返回 nil。
func parseTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, gorm.ErrInvalidValue
}
