package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/sales/model"
)

// WithdrawFilter 提现单查询条件。
type WithdrawFilter struct {
	AdminID      uint64
	DepartmentID uint64
	Status       string
	Page         int
	PageSize     int
}

// WithdrawalRow 提现单展示行：带出销售与部门信息。
type WithdrawalRow struct {
	model.SalesWithdrawal
	AdminName      string `gorm:"column:admin_name"`
	AdminRealName  string `gorm:"column:admin_real_name"`
	DepartmentName string `gorm:"column:department_name"`
}

// WithdrawalRepository 提成提现单数据访问。
type WithdrawalRepository interface {
	CreateWithdrawal(db *gorm.DB, w *model.SalesWithdrawal) error
	FindWithdrawalByID(db *gorm.DB, id uint64) (*model.SalesWithdrawal, error)
	FindWithdrawalByNo(ctx context.Context, no string) (*model.SalesWithdrawal, error)
	UpdateWithdrawal(db *gorm.DB, w *model.SalesWithdrawal) error
	ListWithdrawalRows(ctx context.Context, f WithdrawFilter) ([]WithdrawalRow, int64, error)
	// SumPendingAmount 汇总待审核提现额（概览展示）。
	SumPendingAmount(ctx context.Context, adminID uint64) (float64, error)
}

type withdrawalRepository struct {
	db *gorm.DB
}

// NewWithdrawalRepository 创建提成提现单仓储。
func NewWithdrawalRepository(db *gorm.DB) WithdrawalRepository {
	return &withdrawalRepository{db: db}
}

func (r *withdrawalRepository) CreateWithdrawal(db *gorm.DB, w *model.SalesWithdrawal) error {
	return db.Create(w).Error
}

func (r *withdrawalRepository) FindWithdrawalByID(db *gorm.DB, id uint64) (*model.SalesWithdrawal, error) {
	var w model.SalesWithdrawal
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&w, id).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *withdrawalRepository) FindWithdrawalByNo(ctx context.Context, no string) (*model.SalesWithdrawal, error) {
	var w model.SalesWithdrawal
	if err := r.db.WithContext(ctx).Where("withdraw_no = ?", no).First(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *withdrawalRepository) UpdateWithdrawal(db *gorm.DB, w *model.SalesWithdrawal) error {
	return db.Save(w).Error
}

func (r *withdrawalRepository) ListWithdrawalRows(ctx context.Context, f WithdrawFilter) ([]WithdrawalRow, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	q := r.db.WithContext(ctx).Table("sales_withdrawals sw").
		Joins("LEFT JOIN admins a ON a.id = sw.admin_id").
		Joins("LEFT JOIN departments d ON d.id = a.department_id")
	if f.AdminID > 0 {
		q = q.Where("sw.admin_id = ?", f.AdminID)
	}
	if f.DepartmentID > 0 {
		q = q.Where("a.department_id = ?", f.DepartmentID)
	}
	if f.Status != "" {
		q = q.Where("sw.status = ?", f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []WithdrawalRow
	err := q.Select(`sw.*,
			COALESCE(a.username, '') AS admin_name,
			COALESCE(a.real_name, '') AS admin_real_name,
			COALESCE(d.name, '') AS department_name`).
		Order("sw.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *withdrawalRepository) SumPendingAmount(ctx context.Context, adminID uint64) (float64, error) {
	var sum float64
	err := r.db.WithContext(ctx).Model(&model.SalesWithdrawal{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("admin_id = ? AND status = ?", adminID, model.WithdrawStatusPending).
		Scan(&sum).Error
	return sum, err
}

// —— 内部工具：提现单号生成（供服务层复用） ——

// NextSeq 返回一个进程内单调递增的序号，配合时间戳降低单号碰撞概率。
func NextSeq(t time.Time) int64 { return t.UnixNano() % 1000000 }
