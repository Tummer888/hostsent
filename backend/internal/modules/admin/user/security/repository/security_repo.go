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
	ExpireStaleSessions(ctx context.Context, limit int) ([]string, error)
	// AdminNames 批量取管理员账号名（风险事件的处置人、黑名单的创建/更新人）。
	//
	// 页面只回 ID 的话运营看到的是「处置人 3」这种无从判断的数字；
	// 逐个查会变成 N+1（一页 10 条事件就是 10 次查询），所以一次性批量取。
	AdminNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// CountRiskEvents 按状态统计风险事件数（页面汇总卡片用）。
	CountRiskEvents(ctx context.Context, query dto.RiskEventListQuery) (map[string]int64, error)
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
	// 只取 active：已失效/已过期的行再「撤销」一次会覆盖掉首次的操作人留痕。
	if err := r.db.WithContext(ctx).Where("id IN ? AND status = ?", ids, "active").Find(&sessions).Error; err != nil {
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

// RevokeUserAllSessions 撤销某用户全部**有效**会话。
//
// 只动 status='active' 的行：已 revoked 的行有独立的操作人留痕（谁踢的、为什么），
// 已 expired 的行是自然到期。把它们一起覆盖成「本次操作撤销」会让审计线索失真。
//
// 限定 subject_type='user'：本方法由用户详情的「强制下线」触发，目标必须是客户
// 会话。user_id 同时承载 admins.id，不加域过滤会把 ID 相同的员工后台会话一起踢掉。
func (r *securityRepository) RevokeUserAllSessions(ctx context.Context, userID uint64, reason string, revokedBy uint64) ([]model.Session, error) {
	now := time.Now()
	var sessions []model.Session
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND subject_type = ?", userID, "active", model.SubjectTypeUser).
		Find(&sessions).Error; err != nil {
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

// ExpireStaleSessions 把已过期的 active 会话收成 expired，返回被收的 session_id。
//
// 为什么需要回写：expired_at 是会话的「有效期终点」，status 是「当前状态」，
// 两者必须一致。没有这条回写，expired_at 到点后 status 仍停在 active，
// 会话表会永久堆积「看起来还活着」的行 —— 安全页的会话列表与用户详情的
// 「有效会话数」都会长期虚高（在线口径已按 expired_at 过滤，但 status 列本身
// 仍在骗人，运营按状态筛选/导出时拿到的是错的数据）。
//
// 只处理 status='active' 的行：revoked（人工撤销）有独立的语义与操作人留痕，
// 不能被这个批处理覆盖成 expired，否则「谁踢的」就丢了。
func (r *securityRepository) ExpireStaleSessions(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 500
	}
	var ids []uint64
	if err := r.db.WithContext(ctx).Model(&model.Session{}).
		Where("status = ?", "active").
		Where("expired_at IS NOT NULL AND expired_at <= ?", time.Now()).
		Order("id").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	// 逐行回写而不是一条 UPDATE ... RETURNING：需要拿到 session_id 去失效缓存，
	// 而 GORM 的 UPDATE 不回传旧值。批量上限 500，代价可控。
	var sessions []model.Session
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&sessions).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	sessionIDs := make([]string, 0, len(sessions))
	for i := range sessions {
		if err := r.db.WithContext(ctx).Model(&model.Session{}).
			Where("id = ? AND status = ?", sessions[i].ID, "active").
			Updates(map[string]any{"status": "expired", "updated_at": now}).Error; err != nil {
			return nil, err
		}
		sessionIDs = append(sessionIDs, sessions[i].SessionID)
	}
	return sessionIDs, nil
}

// AdminNames 批量取管理员账号名（处置人 / 创建人 / 更新人显示用）。
//
// 不 join users：风险事件与黑名单的操作人一定是**员工**（管理端接口鉴权决定的），
// 去 users 表查会拿不到人（两个 ID 空间还会撞号）。
func (r *securityRepository) AdminNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	unique := make([]uint64, 0, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return out, nil
	}
	type row struct {
		ID       uint64
		Username string
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("admins").
		Select("id, username").Where("id IN ?", unique).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, item := range rows {
		out[item.ID] = item.Username
	}
	return out, nil
}

// CountRiskEvents 按状态统计风险事件（页面汇总卡片）。
//
// 复用 applyRiskEventFilters：汇总数与列表数必须同一个口径，
// 否则会出现「卡片说 3 条待处理、列表筛出来 5 条」这种对不上的情况。
func (r *securityRepository) CountRiskEvents(ctx context.Context, query dto.RiskEventListQuery) (map[string]int64, error) {
	out := map[string]int64{}
	for _, status := range []string{"pending", "handled", "ignored"} {
		scoped := query
		scoped.Status = status
		base := applyRiskEventFilters(r.db.WithContext(ctx).Model(&model.RiskEvent{}), scoped)
		var n int64
		if err := base.Count(&n).Error; err != nil {
			return out, err
		}
		out[status] = n
	}
	// 总数按调用方传入的筛选（通常是「全部状态」）算，与列表页的 total 对齐。
	base := applyRiskEventFilters(r.db.WithContext(ctx).Model(&model.RiskEvent{}), query)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return out, err
	}
	out["total"] = total
	return out, nil
}

func applyLoginLogFilters(db *gorm.DB, query dto.LoginLogListQuery) *gorm.DB {
	if query.UserID > 0 {
		db = db.Where("user_id = ?", query.UserID)
	}
	if query.SubjectType != "" {
		db = db.Where("subject_type = ?", strings.TrimSpace(query.SubjectType))
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
	if query.SubjectType != "" {
		db = db.Where("subject_type = ?", strings.TrimSpace(query.SubjectType))
	}
	if query.Username != "" {
		db = db.Where("username ILIKE ?", "%"+strings.TrimSpace(query.Username)+"%")
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
		// 筛「在线」时必须连过期一起判：expired_at 到点后 status 要等回写调度器
		// （最多 10 分钟）才变成 expired，这段窗口里只看 status 会把已失效的会话
		// 列成在线 —— 而「在线」对运营的含义就是「这个登录态现在还能用」，
		// 口径必须与 pkg/sessionguard、总览页在线统计一致。
		if query.Status == "active" {
			db = db.Where("expired_at IS NULL OR expired_at > ?", time.Now())
		}
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
