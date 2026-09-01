package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/finance/dto"
	"hostsent/backend/internal/modules/admin/finance/model"
)

// TransactionRepository 资金流水数据访问。
type TransactionRepository interface {
	FindByBiz(db *gorm.DB, userID uint64, bizType, refNo string) (*model.WalletTransaction, error)
	Create(db *gorm.DB, tx *model.WalletTransaction) error
	List(ctx context.Context, q dto.TransactionListQuery) ([]model.WalletTransaction, int64, error)
	// SumByType 在时间范围内汇总指定类型的有符号金额（收入为 +，支出为 -）。
	SumByType(ctx context.Context, userID uint64, types []string, start, end *time.Time) (float64, error)
	// Stats 汇总全部流水：收入合计、支出合计、条数。
	Stats(ctx context.Context) (income, expense float64, count int64, err error)
}

type transactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository 创建资金流水仓储。
func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) FindByBiz(db *gorm.DB, userID uint64, bizType, refNo string) (*model.WalletTransaction, error) {
	var tx model.WalletTransaction
	if err := db.Where("user_id = ? AND biz_type = ? AND ref_no = ?", userID, bizType, refNo).First(&tx).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *transactionRepository) Create(db *gorm.DB, tx *model.WalletTransaction) error {
	return db.Create(tx).Error
}

func (r *transactionRepository) List(ctx context.Context, q dto.TransactionListQuery) ([]model.WalletTransaction, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.WalletTransaction{})
	if q.UserID > 0 {
		base = base.Where("user_id = ?", q.UserID)
	}
	if q.Type != "" {
		base = base.Where("type = ?", q.Type)
	}
	if q.Direction != 0 {
		base = base.Where("direction = ?", q.Direction)
	}
	if start := normalizeTime(q.StartTime); start != nil {
		base = base.Where("created_at >= ?", *start)
	}
	if end := normalizeTime(q.EndTime); end != nil {
		base = base.Where("created_at <= ?", *end)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.WalletTransaction
	err := base.Order("id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// SumByType 汇总指定类型在时间范围内的有符号金额（收入为 +，支出为 -）。
func (r *transactionRepository) SumByType(ctx context.Context, userID uint64, types []string, start, end *time.Time) (float64, error) {
	base := r.db.WithContext(ctx).Model(&model.WalletTransaction{}).Where("type IN ?", types)
	if userID > 0 {
		base = base.Where("user_id = ?", userID)
	}
	if start != nil {
		base = base.Where("created_at >= ?", *start)
	}
	if end != nil {
		base = base.Where("created_at <= ?", *end)
	}
	var sum float64
	err := base.Select("COALESCE(SUM(direction * amount), 0)").Scan(&sum).Error
	return sum, err
}

// Stats 汇总全部流水的收入合计、支出合计与条数。
// 注意：每次查询使用独立构建器，避免 GORM 复用同一 Statement 导致 WHERE 条件累加。
func (r *transactionRepository) Stats(ctx context.Context) (income, expense float64, count int64, err error) {
	if err = r.db.WithContext(ctx).Model(&model.WalletTransaction{}).Count(&count).Error; err != nil {
		return 0, 0, 0, err
	}
	if err = r.db.WithContext(ctx).Model(&model.WalletTransaction{}).
		Where("direction = ?", model.DirectionIncome).
		Select("COALESCE(SUM(amount), 0)").Scan(&income).Error; err != nil {
		return 0, 0, 0, err
	}
	if err = r.db.WithContext(ctx).Model(&model.WalletTransaction{}).
		Where("direction = ?", model.DirectionExpense).
		Select("COALESCE(SUM(amount), 0)").Scan(&expense).Error; err != nil {
		return 0, 0, 0, err
	}
	return income, expense, count, nil
}
