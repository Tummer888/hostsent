package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/security/model"
)

// RiskRepository 风险规则引擎与黑名单校验需要的数据访问能力。
//
// 与 SecurityRepository（管理页面的读写）分开：这里的方法只服务登录链路上的
// 实时判定，都带主体域参数且都要求「按索引可查」，不能和页面查询混在一起演化。
type RiskRepository interface {
	// —— 登录信号统计（规则判定输入）——

	// CountFailedLogins 某账号在窗口内的失败次数。
	CountFailedLogins(ctx context.Context, subjectType, username string, since time.Time) (int64, error)
	// CountFailedAccountsByIP 某 IP 在窗口内失败过的**不同账号**数（撞库判据）。
	//
	// 用不同账号数而不是失败次数：单账号连错 5 次是忘记密码（已有 login_guard
	// 兜底），一个 IP 打 8 个不同账号才是撞库 —— 后者是 login_guard 完全看不到的。
	CountFailedAccountsByIP(ctx context.Context, subjectType, ip string, since time.Time) (int64, error)
	// CountLoginsByIP 某 IP 在窗口内的登录尝试数（成功 + 失败，频率判据）。
	CountLoginsByIP(ctx context.Context, subjectType, ip string, since time.Time) (int64, error)
	// KnownLoginIPs 某账号在 before 之前出现过的成功登录 IP（去重，最近的在前）。
	//
	// before 是「本次尝试的时刻」，必须带上：登录日志先于判定落库，
	// 不排除本次自己写的那行，新 IP 在写下日志的瞬间就变成「已知」。
	KnownLoginIPs(ctx context.Context, subjectType string, userID uint64, before time.Time) ([]string, error)
	// KnownDevices 某账号在 before 之前出现过的成功登录设备指纹（去重、剔除空值）。
	KnownDevices(ctx context.Context, subjectType string, userID uint64, before time.Time) ([]string, error)

	// —— 风险事件写入（含同类聚合）——

	// FindOpenEvent 查同一主体、同一账号、同一规则下最近的一条**未处置**事件（聚合目标）。
	//
	// 必须带上 username：撞库打的是一批**不存在**的账号名，这些登录的 user_id 全是 0，
	// 只按 user_id 聚合会把「IP 打 100 个不同账号」折叠成一条事件，行上还挂着第一个账号名，
	// 运营看到的是「某账号失败 5 次」，实际是几十个账号被扫。
	FindOpenEvent(ctx context.Context, subjectType string, userID uint64, username, ruleCode string, since time.Time) (*model.RiskEvent, error)
	CreateEvent(ctx context.Context, event *model.RiskEvent) error
	UpdateEvent(ctx context.Context, event *model.RiskEvent) error

	// —— 黑名单校验 ——

	// MatchBlacklist 在「启用且在有效期内」的黑名单里按 类型→命中值 查一条。
	//
	// 一次查多个维度（IP/账号/设备/手机/邮箱），返回第一个命中的（按传入顺序优先）。
	MatchBlacklist(ctx context.Context, candidates []BlacklistCandidate) (*model.Blacklist, error)
	// BumpBlacklistHit 命中计数 +1（不做读改写，直接 UPDATE ... hit_count = hit_count + 1）。
	BumpBlacklistHit(ctx context.Context, id uint64) error
}

// BlacklistCandidate 一个待校验的维度。
type BlacklistCandidate struct {
	Type  string
	Value string
}

type riskRepository struct {
	db *gorm.DB
}

// NewRiskRepository 创建风险规则数据访问实例。
func NewRiskRepository(db *gorm.DB) RiskRepository {
	return &riskRepository{db: db}
}

// riskSubjectColumn 把主体域映射到 login_logs 的过滤条件。
//
// 为什么不直接让调用方传列名：调用方（规则引擎）是业务逻辑，不应该知道
// 「admin 落在 platform='admin'」这种存储细节；口径写错一次就会把员工登录
// 算进客户统计里，而且很难在页面上一眼看出来。
func riskSubjectColumn(db *gorm.DB, subjectType string) *gorm.DB {
	if strings.EqualFold(strings.TrimSpace(subjectType), model.SubjectTypeAdmin) {
		return db.Where("subject_type = ?", model.SubjectTypeAdmin)
	}
	return db.Where("subject_type = ? OR subject_type = '' OR subject_type IS NULL", model.SubjectTypeUser)
}

func (r *riskRepository) CountFailedLogins(ctx context.Context, subjectType, username string, since time.Time) (int64, error) {
	if strings.TrimSpace(username) == "" {
		return 0, nil
	}
	q := riskSubjectColumn(r.db.WithContext(ctx).Table("login_logs"), subjectType).
		Where("result = ?", "failed").
		Where("username = ?", strings.TrimSpace(username)).
		Where("created_at >= ?", since)
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *riskRepository) CountFailedAccountsByIP(ctx context.Context, subjectType, ip string, since time.Time) (int64, error) {
	if strings.TrimSpace(ip) == "" {
		return 0, nil
	}
	q := riskSubjectColumn(r.db.WithContext(ctx).Table("login_logs"), subjectType).
		Where("result = ?", "failed").
		Where("ip = ?", strings.TrimSpace(ip)).
		Where("created_at >= ?", since)
	var n int64
	if err := q.Distinct("username").Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *riskRepository) CountLoginsByIP(ctx context.Context, subjectType, ip string, since time.Time) (int64, error) {
	if strings.TrimSpace(ip) == "" {
		return 0, nil
	}
	q := riskSubjectColumn(r.db.WithContext(ctx).Table("login_logs"), subjectType).
		Where("ip = ?", strings.TrimSpace(ip)).
		Where("created_at >= ?", since)
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// loginHistory 取某账号在 login_logs 里的去重列值（IP / 设备指纹共用），最近的在前。
//
// 只取**成功**登录：失败尝试里记下的 IP/设备恰恰是攻击者的，把它算进「常用」
// 会让真正的异常登录在第二次就变成「已知」，规则直接失效。
//
// 必须用 GROUP BY 而不是 `SELECT DISTINCT col ... ORDER BY created_at DESC`：
// 后者在 PostgreSQL 下直接报 42703（ORDER BY 表达式不在 select list 里），
// 而错误会被规则引擎吞掉 → 历史设备恒为空 → 设备变更规则永远不触发。
// 这条 SQL 曾经就是这么写的，live 用例把它抓了出来。
func (r *riskRepository) loginHistory(ctx context.Context, subjectType string, userID uint64, column string, before time.Time) ([]string, error) {
	if userID == 0 {
		return nil, nil
	}
	const limit = 50
	// 列名来自本包内的固定字面量（"ip" / "device_fingerprint"），不来自外部输入。
	var rows []struct {
		Value  string
		LastAt time.Time
	}
	q := riskSubjectColumn(r.db.WithContext(ctx).Table("login_logs"), subjectType).
		Where("user_id = ?", userID).
		Where("result = ?", "success").
		Where(column+" <> ''").
		Where("created_at < ?", before).
		Select(column + " AS value, MAX(created_at) AS last_at").
		Group(column).
		Order("last_at DESC").
		Limit(limit)
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Value) != "" {
			values = append(values, row.Value)
		}
	}
	return values, nil
}

func (r *riskRepository) KnownLoginIPs(ctx context.Context, subjectType string, userID uint64, before time.Time) ([]string, error) {
	return r.loginHistory(ctx, subjectType, userID, "ip", before)
}

func (r *riskRepository) KnownDevices(ctx context.Context, subjectType string, userID uint64, before time.Time) ([]string, error) {
	return r.loginHistory(ctx, subjectType, userID, "device_fingerprint", before)
}

// querySubject 归一化主体域：空值一律按客户域落库查询（默认侧安全）。
func querySubject(subjectType string) string {
	if strings.EqualFold(strings.TrimSpace(subjectType), model.SubjectTypeAdmin) {
		return model.SubjectTypeAdmin
	}
	return model.SubjectTypeUser
}

func (r *riskRepository) FindOpenEvent(ctx context.Context, subjectType string, userID uint64, username, ruleCode string, since time.Time) (*model.RiskEvent, error) {
	// 同样用 Find + Limit(1)：没有未结案的同类事件是最常见的情况
	// （每条规则的第一次命中都会走到这里），不该每次都在日志里报一次 not found。
	var items []model.RiskEvent
	err := r.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("rule_code = ?", ruleCode).
		Where("user_id = ?", userID).
		Where("username = ?", username).
		Where("subject_type = ?", querySubject(subjectType)).
		Where("status = ?", model.RiskStatusPending).
		Where("last_occurred_at >= ?", since).
		Order("id DESC").Limit(1).Find(&items).Error
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (r *riskRepository) CreateEvent(ctx context.Context, event *model.RiskEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *riskRepository) UpdateEvent(ctx context.Context, event *model.RiskEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

func (r *riskRepository) MatchBlacklist(ctx context.Context, candidates []BlacklistCandidate) (*model.Blacklist, error) {
	now := time.Now()
	for _, c := range candidates {
		value := strings.TrimSpace(c.Value)
		if value == "" {
			continue
		}
		// 用 Find + Limit(1) 而不是 First：未命中是这个方法**最常见的返回**
		// （绝大多数登录都不在任何黑名单里），而 First 会返回 ErrRecordNotFound
		// 并让 GORM 在 error 级别打一条日志。生产里每次正常登录都刷一行错误日志，
		// 既淹没了真正的错误，也会让日志告警误报。
		var items []model.Blacklist
		err := r.db.WithContext(ctx).Model(&model.Blacklist{}).
			Where("type = ?", c.Type).
			Where("target_value = ?", value).
			Where("status = ?", model.BlacklistStatusActive).
			Where("effective_at <= ?", now).
			// 永久生效（expired_at IS NULL）与限时生效并存，doc06 §4.4 关键规则 2。
			Where("expired_at IS NULL OR expired_at > ?", now).
			Order("id").Limit(1).Find(&items).Error
		if err != nil {
			return nil, err
		}
		if len(items) > 0 {
			return &items[0], nil
		}
	}
	return nil, nil
}

func (r *riskRepository) BumpBlacklistHit(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Blacklist{}).
		Where("id = ?", id).
		UpdateColumn("hit_count", gorm.Expr("hit_count + 1")).Error
}
