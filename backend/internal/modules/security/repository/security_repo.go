package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/security/dto"
	"hostsent/backend/internal/modules/security/model"
)

type SecurityRepository interface {
	ListLoginLogs(ctx context.Context, query dto.LoginLogListQuery) ([]model.LoginLog, int64, error)
	GetLoginLog(ctx context.Context, id uint64) (*model.LoginLog, error)
	ListAuditLogs(ctx context.Context, query dto.AuditLogListQuery) ([]model.AuditLog, int64, error)
	GetAuditLog(ctx context.Context, id uint64) (*model.AuditLog, error)
	ListRiskEvents(ctx context.Context, query dto.RiskEventListQuery) ([]model.RiskEvent, int64, error)
	GetRiskEvent(ctx context.Context, id uint64) (*model.RiskEvent, error)
	UpdateRiskEvent(ctx context.Context, event *model.RiskEvent) error
	ListBlacklists(ctx context.Context, query dto.BlacklistListQuery) ([]model.Blacklist, int64, error)
	CreateBlacklist(ctx context.Context, item *model.Blacklist) error
	GetBlacklist(ctx context.Context, id uint64) (*model.Blacklist, error)
	UpdateBlacklist(ctx context.Context, item *model.Blacklist) error
	ListBlacklistHits(ctx context.Context, id uint64) ([]model.LoginLog, int64, error)
	ListSessions(ctx context.Context, query dto.SessionListQuery) ([]model.Session, int64, error)
	GetSession(ctx context.Context, id uint64) (*model.Session, error)
	UpdateSession(ctx context.Context, session *model.Session) error
	BatchRevokeSessions(ctx context.Context, ids []uint64, reason string, revokedBy uint64) ([]model.Session, error)
	RevokeUserAllSessions(ctx context.Context, userID uint64, reason string, revokedBy uint64) ([]model.Session, error)
}

type securityRepository struct {
	db *gorm.DB
}

func NewSecurityRepository(db *gorm.DB) SecurityRepository {
	return &securityRepository{db: db}
}

func (r *securityRepository) ListLoginLogs(ctx context.Context, query dto.LoginLogListQuery) ([]model.LoginLog, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.LoginLog{})
	base = applyLoginLogFilters(base, query)
	return paginateModelQuery[model.LoginLog](base, query.Page, query.PageSize)
}

func (r *securityRepository) GetLoginLog(ctx context.Context, id uint64) (*model.LoginLog, error) {
	var item model.LoginLog
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *securityRepository) ListAuditLogs(ctx context.Context, query dto.AuditLogListQuery) ([]model.AuditLog, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.AuditLog{})
	base = applyAuditLogFilters(base, query)
	return paginateModelQuery[model.AuditLog](base, query.Page, query.PageSize)
}

func (r *securityRepository) GetAuditLog(ctx context.Context, id uint64) (*model.AuditLog, error) {
	var item model.AuditLog
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *securityRepository) ListRiskEvents(ctx context.Context, query dto.RiskEventListQuery) ([]model.RiskEvent, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.RiskEvent{})
	base = applyRiskEventFilters(base, query)
	return paginateModelQuery[model.RiskEvent](base, query.Page, query.PageSize)
}

func (r *securityRepository) GetRiskEvent(ctx context.Context, id uint64) (*model.RiskEvent, error) {
	var item model.RiskEvent
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *securityRepository) UpdateRiskEvent(ctx context.Context, event *model.RiskEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

func (r *securityRepository) ListBlacklists(ctx context.Context, query dto.BlacklistListQuery) ([]model.Blacklist, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Blacklist{})
	base = applyBlacklistFilters(base, query)
	return paginateModelQuery[model.Blacklist](base, query.Page, query.PageSize)
}

func (r *securityRepository) CreateBlacklist(ctx context.Context, item *model.Blacklist) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *securityRepository) GetBlacklist(ctx context.Context, id uint64) (*model.Blacklist, error) {
	var item model.Blacklist
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *securityRepository) UpdateBlacklist(ctx context.Context, item *model.Blacklist) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *securityRepository) ListBlacklistHits(ctx context.Context, id uint64) ([]model.LoginLog, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.LoginLog{}).Where("risk_flag <> '' AND user_id = ?", id)
	return paginateModelQuery[model.LoginLog](base, 1, 10)
}

func (r *securityRepository) ListSessions(ctx context.Context, query dto.SessionListQuery) ([]model.Session, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Session{})
	base = applySessionFilters(base, query)
	return paginateModelQuery[model.Session](base, query.Page, query.PageSize)
}

func (r *securityRepository) GetSession(ctx context.Context, id uint64) (*model.Session, error) {
	var item model.Session
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *securityRepository) UpdateSession(ctx context.Context, session *model.Session) error {
	return r.db.WithContext(ctx).Save(session).Error
}

func (r *securityRepository) BatchRevokeSessions(ctx context.Context, ids []uint64, reason string, revokedBy uint64) ([]model.Session, error) {
	now := time.Now()
	var sessions []model.Session
	if len(ids) == 0 {
		return sessions, nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&sessions).Error; err != nil {
		return nil, err
	}
	for i := range sessions {
		sessions[i].Status = "revoked"
		sessions[i].RevokedReason = reason
		sessions[i].RevokedBy = &revokedBy
		sessions[i].RevokedAt = &now
		if err := r.db.WithContext(ctx).Save(&sessions[i]).Error; err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

func (r *securityRepository) RevokeUserAllSessions(ctx context.Context, userID uint64, reason string, revokedBy uint64) ([]model.Session, error) {
	now := time.Now()
	var sessions []model.Session
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&sessions).Error; err != nil {
		return nil, err
	}
	for i := range sessions {
		sessions[i].Status = "revoked"
		sessions[i].RevokedReason = reason
		sessions[i].RevokedBy = &revokedBy
		sessions[i].RevokedAt = &now
		if err := r.db.WithContext(ctx).Save(&sessions[i]).Error; err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

func applyLoginLogFilters(db *gorm.DB, query dto.LoginLogListQuery) *gorm.DB {
	if query.UserID > 0 {
		db = db.Where("user_id = ?", query.UserID)
	}
	if query.Username != "" {
		db = db.Where("username ILIKE ?", "%"+strings.TrimSpace(query.Username)+"%")
	}
	if query.Result != "" {
		db = db.Where("result = ?", query.Result)
	}
	if query.LoginType != "" {
		db = db.Where("login_type = ?", query.LoginType)
	}
	if query.IP != "" {
		db = db.Where("ip ILIKE ?", "%"+strings.TrimSpace(query.IP)+"%")
	}
	if query.RiskFlag != "" {
		db = db.Where("risk_flag = ?", query.RiskFlag)
	}
	return db
}

func applyAuditLogFilters(db *gorm.DB, query dto.AuditLogListQuery) *gorm.DB {
	if query.Operator != "" {
		db = db.Where("operator_name ILIKE ?", "%"+strings.TrimSpace(query.Operator)+"%")
	}
	if query.Module != "" {
		db = db.Where("module = ?", query.Module)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.ResourceType != "" {
		db = db.Where("resource_type = ?", query.ResourceType)
	}
	if query.ResourceID != "" {
		db = db.Where("resource_id = ?", query.ResourceID)
	}
	if query.Result != "" {
		if query.Result == "success" {
			db = db.Where("response_code < 400")
		} else if query.Result == "failed" {
			db = db.Where("response_code >= 400")
		}
	}
	return db
}

func applyRiskEventFilters(db *gorm.DB, query dto.RiskEventListQuery) *gorm.DB {
	if query.RiskType != "" {
		db = db.Where("risk_type = ?", query.RiskType)
	}
	if query.RiskLevel != "" {
		db = db.Where("risk_level = ?", query.RiskLevel)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Keyword != "" {
		keyword := "%" + strings.TrimSpace(query.Keyword) + "%"
		db = db.Where("username ILIKE ? OR ip ILIKE ? OR summary ILIKE ?", keyword, keyword, keyword)
	}
	return db
}

func applyBlacklistFilters(db *gorm.DB, query dto.BlacklistListQuery) *gorm.DB {
	if query.Type != "" {
		db = db.Where("type = ?", query.Type)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Source != "" {
		db = db.Where("source = ?", query.Source)
	}
	if query.Keyword != "" {
		db = db.Where("target_value ILIKE ?", "%"+strings.TrimSpace(query.Keyword)+"%")
	}
	return db
}

func applySessionFilters(db *gorm.DB, query dto.SessionListQuery) *gorm.DB {
	if query.UserID > 0 {
		db = db.Where("user_id = ?", query.UserID)
	}
	if query.Username != "" {
		db = db.Where("username ILIKE ?", "%"+strings.TrimSpace(query.Username)+"%")
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.IP != "" {
		db = db.Where("ip ILIKE ?", "%"+strings.TrimSpace(query.IP)+"%")
	}
	if query.RiskFlag != "" {
		db = db.Where("risk_flag = ?", query.RiskFlag)
	}
	return db
}

func paginateModelQuery[T any](db *gorm.DB, page, pageSize int) ([]T, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []T
	if err := db.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
