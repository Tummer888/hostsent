// Package repository 提供积分体系的数据访问实现。
//
// 与资金账本同构：账户行锁 + 幂等键，保证并发发放不超发、重复回调不重发。
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/point/dto"
	"hostsent/backend/internal/modules/admin/point/model"
)

// PointRepository 积分数据访问能力。
type PointRepository interface {
	// —— 账户 ——
	EnsureAccount(db *gorm.DB, userID uint64) (*model.PointAccount, error)
	LockAccount(db *gorm.DB, userID uint64) (*model.PointAccount, error)
	FindAccount(ctx context.Context, userID uint64) (*model.PointAccount, error)
	SaveAccount(db *gorm.DB, acc *model.PointAccount) error

	// —— 流水 ——
	FindTxByBiz(db *gorm.DB, userID uint64, bizType, refNo string) (*model.PointTransaction, error)
	CreateTx(db *gorm.DB, tx *model.PointTransaction) error
	ListTransactions(ctx context.Context, q dto.TransactionListQuery) ([]model.PointTransactionRow, int64, error)

	// —— 规则 ——
	FindRuleByScene(ctx context.Context, scene string) (*model.PointRule, error)
	ListRules(ctx context.Context, q dto.RuleListQuery) ([]model.PointRule, int64, error)
	FindRuleByID(ctx context.Context, id uint64) (*model.PointRule, error)
	FindRuleByCode(ctx context.Context, code string) (*model.PointRule, error)
	CreateRule(ctx context.Context, r *model.PointRule) error
	UpdateRule(ctx context.Context, r *model.PointRule) error

	// —— 管理端账户列表 ——
	ListAccounts(ctx context.Context, q dto.AccountListQuery) ([]model.PointAccountRow, int64, error)
	// SumBalance 全平台积分余额合计（管理端概览用）。
	SumBalance(ctx context.Context) (balance, earned, spent int64, err error)
}

type pointRepository struct {
	db *gorm.DB
}

// NewPointRepository 创建积分仓储。
func NewPointRepository(db *gorm.DB) PointRepository {
	return &pointRepository{db: db}
}

// —— 账户 ——

// LockAccount 行锁读取积分账户（必须已存在，否则返回 ErrRecordNotFound）。
func (r *pointRepository) LockAccount(db *gorm.DB, userID uint64) (*model.PointAccount, error) {
	var acc model.PointAccount
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

// EnsureAccount 幂等建户（并发下唯一键冲突静默跳过），再行锁读取。
func (r *pointRepository) EnsureAccount(db *gorm.DB, userID uint64) (*model.PointAccount, error) {
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(&model.PointAccount{UserID: userID}).Error; err != nil {
		return nil, err
	}
	return r.LockAccount(db, userID)
}

// FindAccount 读取积分账户；不存在时返回 ErrRecordNotFound，由服务层映射为 0 值。
func (r *pointRepository) FindAccount(ctx context.Context, userID uint64) (*model.PointAccount, error) {
	var acc model.PointAccount
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

func (r *pointRepository) SaveAccount(db *gorm.DB, acc *model.PointAccount) error {
	return db.Save(acc).Error
}

// —— 流水 ——

func (r *pointRepository) FindTxByBiz(db *gorm.DB, userID uint64, bizType, refNo string) (*model.PointTransaction, error) {
	var tx model.PointTransaction
	if err := db.Where("user_id = ? AND biz_type = ? AND ref_no = ?", userID, bizType, refNo).First(&tx).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *pointRepository) CreateTx(db *gorm.DB, tx *model.PointTransaction) error {
	return db.Create(tx).Error
}

// ListTransactions 分页查询积分流水（附带用户名，支持管理端与用户端复用）。
//
// 计数与取数各用独立构建器：GORM 的 Statement 会被复用，链式条件在同一条
// 引用上累加会导致第二次查询多出重复 WHERE（资金流水仓储注释已记录该坑）。
func (r *pointRepository) ListTransactions(ctx context.Context, q dto.TransactionListQuery) ([]model.PointTransactionRow, int64, error) {
	var total int64
	if err := r.txQuery(ctx, q).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.PointTransactionRow
	err := r.txQuery(ctx, q).
		Select("pt.*, COALESCE(u.username, '') AS username").
		Joins("LEFT JOIN users u ON u.id = pt.user_id").
		Order("pt.id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// txQuery 构造积分流水查询（含全部筛选条件），每次调用返回新的构建器。
func (r *pointRepository) txQuery(ctx context.Context, q dto.TransactionListQuery) *gorm.DB {
	base := r.db.WithContext(ctx).Table("point_transactions pt")
	if q.UserID > 0 {
		base = base.Where("pt.user_id = ?", q.UserID)
	}
	if q.Type != "" {
		base = base.Where("pt.type = ?", q.Type)
	}
	if q.Direction != 0 {
		base = base.Where("pt.direction = ?", q.Direction)
	}
	if start := normalizeTime(q.StartTime); start != nil {
		base = base.Where("pt.created_at >= ?", *start)
	}
	if end := normalizeTime(q.EndTime); end != nil {
		base = base.Where("pt.created_at <= ?", *end)
	}
	return base
}

// —— 规则 ——

// FindRuleByScene 取该场景生效的第一条规则（按 sort_order 升序）。
func (r *pointRepository) FindRuleByScene(ctx context.Context, scene string) (*model.PointRule, error) {
	var rule model.PointRule
	if err := r.db.WithContext(ctx).
		Where("scene = ? AND status = ?", scene, model.RuleStatusEnabled).
		Order("sort_order asc, id asc").First(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *pointRepository) ListRules(ctx context.Context, q dto.RuleListQuery) ([]model.PointRule, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PointRule{})
	if q.Scene != "" {
		base = base.Where("scene = ?", q.Scene)
	}
	if q.Status != nil {
		base = base.Where("status = ?", *q.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PointRule
	err := base.Order("sort_order asc, id asc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *pointRepository) FindRuleByID(ctx context.Context, id uint64) (*model.PointRule, error) {
	var rule model.PointRule
	if err := r.db.WithContext(ctx).First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *pointRepository) FindRuleByCode(ctx context.Context, code string) (*model.PointRule, error) {
	var rule model.PointRule
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *pointRepository) CreateRule(ctx context.Context, rule *model.PointRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *pointRepository) UpdateRule(ctx context.Context, rule *model.PointRule) error {
	return r.db.WithContext(ctx).Model(&model.PointRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
		"name":                 rule.Name,
		"scene":                rule.Scene,
		"earn_mode":            rule.EarnMode,
		"fixed_points":         rule.FixedPoints,
		"points_per_yuan":      rule.PointsPerYuan,
		"min_amount":           rule.MinAmount,
		"max_points_per_order": rule.MaxPointsPerOrder,
		"valid_days":           rule.ValidDays,
		"status":               rule.Status,
		"sort_order":           rule.SortOrder,
		"remark":               rule.Remark,
	}).Error
}

// —— 管理端账户列表 ——

func (r *pointRepository) ListAccounts(ctx context.Context, q dto.AccountListQuery) ([]model.PointAccountRow, int64, error) {
	var total int64
	if err := r.accountQuery(ctx, q).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.PointAccountRow
	err := r.accountQuery(ctx, q).
		Select("pa.*, COALESCE(u.username, '') AS username, COALESCE(u.email, '') AS email").
		Joins("LEFT JOIN users u ON u.id = pa.user_id").
		Order("pa.balance desc, pa.user_id asc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// accountQuery 构造积分账户查询（含用户名/余额门槛筛选）。
func (r *pointRepository) accountQuery(ctx context.Context, q dto.AccountListQuery) *gorm.DB {
	base := r.db.WithContext(ctx).Table("point_accounts pa")
	if q.UserID > 0 {
		base = base.Where("pa.user_id = ?", q.UserID)
	}
	if kw := strings.TrimSpace(q.UserKeyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("pa.user_id IN (SELECT id FROM users WHERE username ILIKE ? OR email ILIKE ?)", like, like)
	}
	if q.MinPoints > 0 {
		base = base.Where("pa.balance >= ?", q.MinPoints)
	}
	return base
}

func (r *pointRepository) SumBalance(ctx context.Context) (balance, earned, spent int64, err error) {
	type agg struct {
		Balance int64
		Earned  int64
		Spent   int64
	}
	var a agg
	err = r.db.WithContext(ctx).Model(&model.PointAccount{}).
		Select("COALESCE(SUM(balance),0) AS balance, COALESCE(SUM(total_earned),0) AS earned, COALESCE(SUM(total_spent),0) AS spent").
		Scan(&a).Error
	return a.Balance, a.Earned, a.Spent, err
}

// —— 工具 ——

func normalizeTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00", "2006-01-02", time.RFC3339}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, raw, time.Local); err == nil {
			return &t
		}
	}
	return nil
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(size int) int {
	if size <= 0 {
		return 20
	}
	if size > 100 {
		return 100
	}
	return size
}
