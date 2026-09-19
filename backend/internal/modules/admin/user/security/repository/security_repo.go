package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/security/dto"
	"hostsent/backend/internal/modules/admin/user/security/model"
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
	ListBlacklistHits(ctx context.Context, id uint64, query dto.BlacklistHitListQuery) ([]model.LoginLog, int64, error)
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

// ListBlacklistHits 该黑名单在登录日志里的命中记录。
//
// 关联键必须按黑名单类型选列：
//   - ip      → login_logs.ip
//   - device  → login_logs.device_fingerprint
//   - user    → login_logs.username（target_value 存的是账号名）
//
// 旧实现是 `user_id = ?`（拿黑名单行的 ID 去比用户 ID），既错类型又错语义，
// 页面上恒为空——这正是 doc104 F6 记录的问题。
func (r *securityRepository) ListBlacklistHits(ctx context.Context, id uint64, query dto.BlacklistHitListQuery) ([]model.LoginLog, int64, error) {
	var entry model.Blacklist
	if err := r.db.WithContext(ctx).First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, gorm.ErrRecordNotFound
		}
		return nil, 0, err
	}
	target := strings.TrimSpace(entry.TargetValue)
	if target == "" {
		return []model.LoginLog{}, 0, nil
	}
	base := r.db.WithContext(ctx).Model(&model.LoginLog{})
	switch strings.ToLower(strings.TrimSpace(entry.Type)) {
	case "ip":
		base = base.Where("ip = ?", target)
	case "device":
		base = base.Where("device_fingerprint = ?", target)
	default:
		base = base.Where("username = ?", target)
	}
	return paginateModelQuery[model.LoginLog](base, query.Page, query.PageSize)
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
	return applyCreatedRange(db, query.StartTime, query.EndTime)
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
	return applyCreatedRange(db, query.StartTime, query.EndTime)
}

func applyRiskEventFilters(db *gorm.DB, query dto.RiskEventListQuery) *gorm.DB {
	if query.UserID > 0 {
		db = db.Where("user_id = ?", query.UserID)
	}
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
	// 风险事件按「最后一次发生」筛时间：运营关心的是「最近还在发生吗」，
	// 用 first_occurred_at 会把持续发生的旧事件筛掉。
	return applyRange(db, "last_occurred_at", query.StartTime, query.EndTime)
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
	// 黑名单按生效时间筛：与列表里展示的「生效时间」列一致，运营才不会
	// 出现「筛了 7 天却看到一条生效于上月的记录」这种对不上的情况。
	return applyRange(db, "effective_at", query.StartTime, query.EndTime)
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
	// 会话按登录时间筛（列表默认排序依据），与「登录时间」列一致。
	return applyRange(db, "login_at", query.StartTime, query.EndTime)
}

// applyCreatedRange 按 created_at 筛时间范围。
func applyCreatedRange(db *gorm.DB, startTime, endTime string) *gorm.DB {
	return applyRange(db, "created_at", startTime, endTime)
}

// applyRange 按指定列筛时间范围。
//
// 时间参数解析失败时**忽略该边界**而不是报错：筛选条件写错不该让整个列表打不开，
// 与 logcenter 的宽松口径一致。前端传的是本地时区的 "YYYY-MM-DD HH:mm:ss"，
// 因此用 ParseInLocation 走 time.Local，避免 UTC 偏移把当天数据切掉一截。
func applyRange(db *gorm.DB, column, startTime, endTime string) *gorm.DB {
	if t, ok := parseFilterTime(startTime); ok {
		db = db.Where(column+" >= ?", t)
	}
	if t, ok := parseFilterTime(endTime, endOfDay); ok {
		db = db.Where(column+" <= ?", t)
	}
	return db
}

// boundary 时间边界取法。
type boundary bool

const (
	startOfDay boundary = false
	endOfDay   boundary = true
)

// parseFilterTime 宽松解析筛选时间（支持 RFC3339、日期时间与纯日期）。
//
// 纯日期（"2006-01-02"）作为上界时补齐到当天 23:59:59：否则「结束日期 = 今天」
// 会被解析成今天 00:00:00，当天的记录一条都查不出来——这是日期筛选最经典的坑。
func parseFilterTime(raw string, bounds ...boundary) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		t, err := time.ParseInLocation(layout, raw, time.Local)
		if err != nil {
			continue
		}
		if len(bounds) > 0 && bounds[0] == endOfDay && len(raw) == 10 {
			t = t.Add(24*time.Hour - time.Second)
		}
		return t, true
	}
	return time.Time{}, false
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
