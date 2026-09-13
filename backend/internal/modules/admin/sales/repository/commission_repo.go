package repository

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/sales/model"
)

// TxFilter 提成台账查询条件。
type TxFilter struct {
	AdminID      uint64 // 提成归属销售；0 表示不限（主管/超管）
	DepartmentID uint64 // 限定销售所属部门；0 表示不限
	Type         string
	OrderNo      string
	CustomerID   uint64
	StartAt      *time.Time
	EndAt        *time.Time
	Page         int
	PageSize     int
}

// TxRow 提成台账展示行：带出销售、部门与客户信息。
type TxRow struct {
	model.CommissionTransaction
	AdminName      string `gorm:"column:admin_name"`
	AdminRealName  string `gorm:"column:admin_real_name"`
	DepartmentName string `gorm:"column:department_name"`
	CustomerName   string `gorm:"column:customer_name"`
}

// PerformanceRow 业绩排行行（按订单聚合）。
type PerformanceRow struct {
	AdminID        uint64  `gorm:"column:admin_id"`
	AdminName      string  `gorm:"column:admin_name"`
	AdminRealName  string  `gorm:"column:admin_real_name"`
	DepartmentID   uint64  `gorm:"column:department_id"`
	DepartmentName string  `gorm:"column:department_name"`
	Amount         float64 `gorm:"column:amount"`
	Orders         int64   `gorm:"column:orders"`
}

// TargetRow 业绩目标展示行。
type TargetRow struct {
	model.SalesTarget
	AdminName      string `gorm:"column:admin_name"`
	AdminRealName  string `gorm:"column:admin_real_name"`
	DepartmentName string `gorm:"column:department_name"`
}

// CommissionRepository 提成账本数据访问。
type CommissionRepository interface {
	// —— 提成账户 ——
	LockAccount(db *gorm.DB, adminID uint64) (*model.CommissionAccount, error)
	EnsureAccount(db *gorm.DB, adminID uint64) (*model.CommissionAccount, error)
	FindAccount(ctx context.Context, adminID uint64) (*model.CommissionAccount, error)
	UpdateAccount(db *gorm.DB, acc *model.CommissionAccount) error

	// —— 提成台账 ——
	CreateTx(db *gorm.DB, tx *model.CommissionTransaction) error
	FindTxByBiz(db *gorm.DB, adminID uint64, bizType, refNo string) (*model.CommissionTransaction, error)
	SumTxAmount(ctx context.Context, adminID, orderID uint64, types ...string) (float64, error)
	ListTxRows(ctx context.Context, f TxFilter) ([]TxRow, int64, error)
	// ListReleasable 取已到解冻期的计提流水并加行锁（跳过被其他事务占用的行）。
	ListReleasable(db *gorm.DB, now time.Time, limit int) ([]model.CommissionTransaction, error)
	// MarkReleased 把计提流水标记为已解冻（release_at 置 NULL，幂等标记）。
	MarkReleased(db *gorm.DB, id uint64) error
	// SumPendingRelease 汇总解冻中金额（概览展示）。
	SumPendingRelease(ctx context.Context, adminID uint64) (float64, error)

	// —— 归属与业绩数据源 ——
	// OrderSalesOwner 读订单归属快照（orders.sales_admin_id）。
	OrderSalesOwner(ctx context.Context, orderID uint64) (uint64, error)
	// HasSettledOrder 判断客户是否已有成交订单（首单判定），excludeOrderID 排除当前单。
	HasSettledOrder(ctx context.Context, userID, excludeOrderID uint64) (bool, error)
	// PerformanceRanking 按订单聚合某月销售额与单量。
	PerformanceRanking(ctx context.Context, period string, departmentID uint64, limit int) ([]PerformanceRow, error)
	// AdminPerformance 单个销售某月的销售额与单量。
	AdminPerformance(ctx context.Context, adminID uint64, period string) (PerformanceRow, error)

	// —— 目标 ——
	ListTargets(ctx context.Context, period string, departmentID uint64) ([]TargetRow, error)
	FindTarget(ctx context.Context, period, scope string, adminID, departmentID uint64) (*model.SalesTarget, error)
	UpsertTarget(db *gorm.DB, t *model.SalesTarget) error

	// —— 全局配置 ——
	Rates(ctx context.Context) (model.Rates, error)
}

type commissionRepository struct {
	db *gorm.DB
}

// NewCommissionRepository 创建提成账本仓储。
func NewCommissionRepository(db *gorm.DB) CommissionRepository {
	return &commissionRepository{db: db}
}

// —— 提成账户 ——

func (r *commissionRepository) FindAccount(ctx context.Context, adminID uint64) (*model.CommissionAccount, error) {
	var acc model.CommissionAccount
	if err := r.db.WithContext(ctx).Where("admin_id = ?", adminID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

func (r *commissionRepository) LockAccount(db *gorm.DB, adminID uint64) (*model.CommissionAccount, error) {
	var acc model.CommissionAccount
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("admin_id = ?", adminID).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

// EnsureAccount 先按 admin_id 幂等插入（并发下唯一键冲突静默跳过），再行锁读取。
func (r *commissionRepository) EnsureAccount(db *gorm.DB, adminID uint64) (*model.CommissionAccount, error) {
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "admin_id"}},
		DoNothing: true,
	}).Create(&model.CommissionAccount{AdminID: adminID}).Error; err != nil {
		return nil, err
	}
	return r.LockAccount(db, adminID)
}

func (r *commissionRepository) UpdateAccount(db *gorm.DB, acc *model.CommissionAccount) error {
	return db.Save(acc).Error
}

// —— 提成台账 ——

func (r *commissionRepository) CreateTx(db *gorm.DB, tx *model.CommissionTransaction) error {
	return db.Create(tx).Error
}

func (r *commissionRepository) FindTxByBiz(db *gorm.DB, adminID uint64, bizType, refNo string) (*model.CommissionTransaction, error) {
	var tx model.CommissionTransaction
	if err := db.Where("admin_id = ? AND biz_type = ? AND ref_no = ?", adminID, bizType, refNo).First(&tx).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *commissionRepository) SumTxAmount(ctx context.Context, adminID, orderID uint64, types ...string) (float64, error) {
	if len(types) == 0 {
		return 0, nil
	}
	var sum float64
	err := r.db.WithContext(ctx).Model(&model.CommissionTransaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("admin_id = ? AND order_id = ? AND type IN ?", adminID, orderID, types).
		Scan(&sum).Error
	return sum, err
}

// ListTxRows 分页查询提成台账（关联销售/部门/客户名）。
func (r *commissionRepository) ListTxRows(ctx context.Context, f TxFilter) ([]TxRow, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	base := r.db.WithContext(ctx).Table("sales_commission_transactions sct").
		Joins("LEFT JOIN admins a ON a.id = sct.admin_id").
		Joins("LEFT JOIN users u ON u.id = sct.customer_user_id")
	base = applyTxFilters(base, f)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []TxRow
	err := applyTxFilters(r.db.WithContext(ctx).Table("sales_commission_transactions sct").
		Joins("LEFT JOIN admins a ON a.id = sct.admin_id").
		Joins("LEFT JOIN departments d ON d.id = a.department_id").
		Joins("LEFT JOIN users u ON u.id = sct.customer_user_id"), f).
		Select(`sct.*,
			COALESCE(a.username, '') AS admin_name,
			COALESCE(a.real_name, '') AS admin_real_name,
			COALESCE(d.name, '') AS department_name,
			COALESCE(u.username, '') AS customer_name`).
		Order("sct.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// applyTxFilters 抽取公共筛选条件，保证计数与取数的口径完全一致。
func applyTxFilters(q *gorm.DB, f TxFilter) *gorm.DB {
	if f.AdminID > 0 {
		q = q.Where("sct.admin_id = ?", f.AdminID)
	}
	if f.DepartmentID > 0 {
		q = q.Where("a.department_id = ?", f.DepartmentID)
	}
	if f.Type != "" {
		q = q.Where("sct.type = ?", f.Type)
	}
	if f.OrderNo != "" {
		q = q.Where("sct.order_no ILIKE ?", "%"+f.OrderNo+"%")
	}
	if f.CustomerID > 0 {
		q = q.Where("sct.customer_user_id = ?", f.CustomerID)
	}
	if f.StartAt != nil {
		q = q.Where("sct.created_at >= ?", *f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where("sct.created_at < ?", *f.EndAt)
	}
	return q
}

// ListReleasable 取已到解冻期的计提流水并加行锁；SKIP LOCKED 避免多实例重复解冻同一条。
func (r *commissionRepository) ListReleasable(db *gorm.DB, now time.Time, limit int) ([]model.CommissionTransaction, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var rows []model.CommissionTransaction
	err := db.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("type IN ?", []string{model.TxTypeAccrueFirst, model.TxTypeAccrueSubsequent, model.TxTypeAccrueRenewal}).
		Where("release_at IS NOT NULL AND release_at <= ?", now).
		Order("id asc").Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *commissionRepository) MarkReleased(db *gorm.DB, id uint64) error {
	return db.Model(&model.CommissionTransaction{}).Where("id = ?", id).
		Update("release_at", nil).Error
}

func (r *commissionRepository) SumPendingRelease(ctx context.Context, adminID uint64) (float64, error) {
	var sum float64
	err := r.db.WithContext(ctx).Model(&model.CommissionTransaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("admin_id = ? AND release_at IS NOT NULL", adminID).
		Scan(&sum).Error
	return sum, err
}

// —— 归属与业绩数据源 ——

func (r *commissionRepository) OrderSalesOwner(ctx context.Context, orderID uint64) (uint64, error) {
	var owner *uint64
	err := r.db.WithContext(ctx).Table("orders").Select("sales_admin_id").
		Where("id = ?", orderID).Scan(&owner).Error
	if err != nil || owner == nil {
		return 0, err
	}
	return *owner, nil
}

// HasSettledOrder 判断客户是否存在已成交订单（销售口径首单判定）。
func (r *commissionRepository) HasSettledOrder(ctx context.Context, userID, excludeOrderID uint64) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("orders").
		Where("user_id = ? AND id <> ? AND status IN ?", userID, excludeOrderID,
			[]string{"paid", "provisioning", "active", "completed"}).
		Limit(1).Count(&n).Error
	return n > 0, err
}

// PerformanceRanking 按订单归属聚合某月销售额与单量。
// 排行数据源选 orders 而非提成台账：销售额是「成交」口径（含未产生提成的单），
// 与目标额同口径，避免用提成额排行导致目标达成率失真。
func (r *commissionRepository) PerformanceRanking(ctx context.Context, period string, departmentID uint64, limit int) ([]PerformanceRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []PerformanceRow
	err := r.db.WithContext(ctx).Table("orders o").
		Joins("JOIN admins a ON a.id = o.sales_admin_id").
		Joins("LEFT JOIN departments d ON d.id = a.department_id").
		Select(`o.sales_admin_id AS admin_id,
			COALESCE(a.username, '') AS admin_name,
			COALESCE(a.real_name, '') AS admin_real_name,
			a.department_id AS department_id,
			COALESCE(d.name, '') AS department_name,
			COALESCE(SUM(o.paid_amount), 0) AS amount,
			COUNT(*) AS orders`).
		Where("o.status IN ?", []string{"paid", "provisioning", "active", "completed"}).
		Where("o.paid_amount > 0").
		Where("to_char(o.created_at, 'YYYY-MM') = ?", period).
		Where("(? = 0 OR a.department_id = ?)", departmentID, departmentID).
		Group("o.sales_admin_id, a.username, a.real_name, a.department_id, d.name").
		Order("amount DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *commissionRepository) AdminPerformance(ctx context.Context, adminID uint64, period string) (PerformanceRow, error) {
	var row PerformanceRow
	err := r.db.WithContext(ctx).Table("orders o").
		Joins("LEFT JOIN admins a ON a.id = o.sales_admin_id").
		Joins("LEFT JOIN departments d ON d.id = a.department_id").
		Select(`COALESCE(a.username, '') AS admin_name,
			COALESCE(a.real_name, '') AS admin_real_name,
			COALESCE(a.department_id, 0) AS department_id,
			COALESCE(d.name, '') AS department_name,
			COALESCE(SUM(o.paid_amount), 0) AS amount,
			COUNT(*) AS orders`).
		Where("o.sales_admin_id = ?", adminID).
		Where("o.status IN ?", []string{"paid", "provisioning", "active", "completed"}).
		Where("o.paid_amount > 0").
		Where("to_char(o.created_at, 'YYYY-MM') = ?", period).
		// 聚合列必须与 GROUP BY 一致，否则 PostgreSQL 报 42803（该销售当周期无单时返回空行）。
		Group("a.username, a.real_name, a.department_id, d.name").
		Scan(&row).Error
	if err != nil {
		return row, err
	}
	row.AdminID = adminID
	return row, nil
}

// —— 目标 ——

func (r *commissionRepository) ListTargets(ctx context.Context, period string, departmentID uint64) ([]TargetRow, error) {
	var rows []TargetRow
	q := r.db.WithContext(ctx).Table("sales_targets st").
		Joins("LEFT JOIN admins a ON a.id = st.admin_id").
		Joins("LEFT JOIN departments d ON d.id = st.department_id").
		Select(`st.*,
			COALESCE(a.username, '') AS admin_name,
			COALESCE(a.real_name, '') AS admin_real_name,
			COALESCE(d.name, '') AS department_name`)
	if period != "" {
		q = q.Where("st.period = ?", period)
	}
	if departmentID > 0 {
		q = q.Where("(st.department_id = ? OR (st.scope = ? AND a.department_id = ?))",
			departmentID, model.TargetScopeAdmin, departmentID)
	}
	err := q.Order("st.period desc, st.scope asc, st.admin_id asc").Scan(&rows).Error
	return rows, err
}

func (r *commissionRepository) FindTarget(ctx context.Context, period, scope string, adminID, departmentID uint64) (*model.SalesTarget, error) {
	var t model.SalesTarget
	err := r.db.WithContext(ctx).
		Where("period = ? AND scope = ? AND admin_id = ? AND department_id = ?", period, scope, adminID, departmentID).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpsertTarget 按（周期+范围）唯一键 upsert，支持重复下发目标。
func (r *commissionRepository) UpsertTarget(db *gorm.DB, t *model.SalesTarget) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "period"}, {Name: "scope"}, {Name: "admin_id"}, {Name: "department_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"target_amount", "target_orders", "created_by", "updated_at",
		}),
	}).Create(t).Error
}

// —— 全局配置 ——

// Rates 读取 sales 分组配置；缺失项按默认值兜底，非法值忽略。
func (r *commissionRepository) Rates(ctx context.Context) (model.Rates, error) {
	rates := model.DefaultRates()
	var rows []struct {
		Key   string `gorm:"column:config_key"`
		Value string `gorm:"column:config_value"`
	}
	err := r.db.WithContext(ctx).Table("system_configs").
		Select("config_key, config_value").
		Where("config_group = ? AND status = ?", model.ConfigGroup, "active").
		Scan(&rows).Error
	if err != nil {
		return rates, err
	}
	for _, row := range rows {
		switch row.Key {
		case model.ConfigKeyEnabled:
			rates.Enabled = row.Value == "true" || row.Value == "1"
		case model.ConfigKeyFirstOrder:
			rates.FirstOrder = parseRate(row.Value, rates.FirstOrder)
		case model.ConfigKeySubsequent:
			rates.Subsequent = parseRate(row.Value, rates.Subsequent)
		case model.ConfigKeyRenewal:
			rates.Renewal = parseRate(row.Value, rates.Renewal)
		case model.ConfigKeyReleaseDays:
			if v, err := strconv.Atoi(row.Value); err == nil && v >= 0 {
				rates.ReleaseDays = v
			}
		case model.ConfigKeyMinWithdraw:
			if v, err := strconv.ParseFloat(row.Value, 64); err == nil && v >= 0 {
				rates.MinWithdraw = v
			}
		case model.ConfigKeyProtectDays:
			if v, err := strconv.Atoi(row.Value); err == nil && v >= 0 {
				rates.ProtectDays = v
			}
		case model.ConfigKeyRenewalCommission:
			if row.Value == model.RenewalModeFollowOwner || row.Value == model.RenewalModeFollowOrder {
				rates.RenewalMode = row.Value
			}
		case model.ConfigKeyAllowNegative:
			rates.AllowNegative = row.Value == "true" || row.Value == "1"
		}
	}
	return rates, nil
}

func parseRate(value string, fallback float64) float64 {
	v, err := strconv.ParseFloat(value, 64)
	if err != nil || v < 0 || v > 1 {
		return fallback
	}
	return v
}
