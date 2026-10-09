package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/finance/transaction/dto"
	"hostsent/backend/internal/modules/admin/finance/transaction/model"
)

// TransactionRepository 资金流水数据访问。
type TransactionRepository interface {
	FindByBiz(db *gorm.DB, userID uint64, bizType, refNo string) (*model.WalletTransaction, error)
	Create(db *gorm.DB, tx *model.WalletTransaction) error
	List(ctx context.Context, q dto.TransactionListQuery) ([]TransactionRow, int64, error)
	// ListForExport 按同一组筛选条件导出明细（不分页，按 id 升序，上限 limit 条）。
	ListForExport(ctx context.Context, q dto.TransactionListQuery, limit int) ([]TransactionRow, error)
	// Summary 按同一组筛选条件汇总（全量、非当前页）：收入/支出/净额/笔数 + 内部划转笔数。
	Summary(ctx context.Context, q dto.TransactionListQuery) (*TransactionSummaryRow, error)
	// SumByType 在时间范围内汇总指定类型的有符号金额（收入为 +，支出为 -）。
	SumByType(ctx context.Context, userID uint64, types []string, start, end *time.Time) (float64, error)
	// SumConsumeSplit 按关联订单是否续费拆分消费流水（doc36 §3.4 账单分类）。
	// 返回 (普通消费, 续费消费)，均为正数金额。
	SumConsumeSplit(ctx context.Context, userID uint64, start, end *time.Time) (purchase, renewal float64, err error)
	// Stats 汇总全部流水：收入合计、支出合计、条数。
	Stats(ctx context.Context) (income, expense float64, count int64, err error)
}

// TransactionRow 流水行（附用户名，供列表与导出展示）。
type TransactionRow struct {
	model.WalletTransaction
	Username string `gorm:"column:username"`
}

// TransactionSummaryRow 筛选口径汇总行。
type TransactionSummaryRow struct {
	Income        float64 `gorm:"column:income"`
	Expense       float64 `gorm:"column:expense"`
	Count         int64   `gorm:"column:count"`
	InternalCount int64   `gorm:"column:internal_count"`
}

// internalTxTypes 冻结/解冻：可用余额 ↔ 冻结余额的内部划转，不属于真实收付口径。
var internalTxTypes = []string{model.TxTypeFreeze, model.TxTypeUnfreeze}

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

func (r *transactionRepository) List(ctx context.Context, q dto.TransactionListQuery) ([]TransactionRow, int64, error) {
	base := r.filtered(ctx, q)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []TransactionRow
	err := base.Select("wt.*, COALESCE(u.username, '') AS username").
		Order("wt.id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListForExport 导出用明细：同一组筛选条件、按时间正序、条数封顶（避免一次导出拖垮内存）。
func (r *transactionRepository) ListForExport(ctx context.Context, q dto.TransactionListQuery, limit int) ([]TransactionRow, error) {
	if limit <= 0 {
		limit = 20000
	}
	var items []TransactionRow
	err := r.filtered(ctx, q).
		Select("wt.*, COALESCE(u.username, '') AS username").
		Order("wt.id asc").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Summary 与列表同口径汇总：冻结/解冻不计入收入/支出，单独计数避免口径误读。
func (r *transactionRepository) Summary(ctx context.Context, q dto.TransactionListQuery) (*TransactionSummaryRow, error) {
	// 内部划转类型是固定常量，直接内联为字面量（与订单统计的写法一致）。
	internalIn := "'" + strings.Join(internalTxTypes, "','") + "'"
	var row TransactionSummaryRow
	err := r.filtered(ctx, q).
		Select(`COALESCE(SUM(CASE WHEN wt.direction = 1 AND wt.type NOT IN (` + internalIn + `) THEN wt.amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN wt.direction = -1 AND wt.type NOT IN (` + internalIn + `) THEN wt.amount ELSE 0 END), 0) AS expense,
			COALESCE(SUM(CASE WHEN wt.type NOT IN (` + internalIn + `) THEN 1 ELSE 0 END), 0) AS count,
			COALESCE(SUM(CASE WHEN wt.type IN (` + internalIn + `) THEN 1 ELSE 0 END), 0) AS internal_count`).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// filtered 构造列表/汇总/导出共用的查询（含用户名字段所需的 join）。
func (r *transactionRepository) filtered(ctx context.Context, q dto.TransactionListQuery) *gorm.DB {
	base := r.db.WithContext(ctx).Table("wallet_transactions AS wt").
		Joins("LEFT JOIN users u ON u.id = wt.user_id")
	if q.UserID > 0 {
		base = base.Where("wt.user_id = ?", q.UserID)
	}
	if keyword := normalizeKeyword(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("(wt.tx_no ILIKE ? OR wt.order_no ILIKE ? OR wt.ref_no ILIKE ? OR u.username ILIKE ?)",
			like, like, like, like)
	}
	if q.Type != "" {
		base = base.Where("wt.type = ?", q.Type)
	}
	if q.Direction != 0 {
		base = base.Where("wt.direction = ?", q.Direction)
	}
	if start := normalizeTime(q.StartTime); start != nil {
		base = base.Where("wt.created_at >= ?", *start)
	}
	if end := normalizeTime(q.EndTime); end != nil {
		base = base.Where("wt.created_at <= ?", *end)
	}
	return base
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

// SumConsumeSplit 按关联订单是否续费拆分消费流水（doc36 §3.4 账单分类）。
// 续费判定沿用 orders.renewal_id != 0（订单表无独立 order_type 列，见 doc36 §1.2）。
// 返回 (普通消费, 续费消费)，均为正数金额。
func (r *transactionRepository) SumConsumeSplit(ctx context.Context, userID uint64, start, end *time.Time) (purchase, renewal float64, err error) {
	base := r.db.WithContext(ctx).Model(&model.WalletTransaction{}).
		Where("type = ?", model.TxTypeConsume).
		Where("direction = ?", model.DirectionExpense)
	if userID > 0 {
		base = base.Where("user_id = ?", userID)
	}
	if start != nil {
		base = base.Where("created_at >= ?", *start)
	}
	if end != nil {
		base = base.Where("created_at <= ?", *end)
	}
	query := `COALESCE(SUM(CASE WHEN EXISTS (
			SELECT 1 FROM orders o WHERE o.order_no = wallet_transactions.order_no AND o.renewal_id <> 0
		) THEN amount ELSE 0 END), 0) AS renewal,
		COALESCE(SUM(CASE WHEN NOT EXISTS (
			SELECT 1 FROM orders o WHERE o.order_no = wallet_transactions.order_no AND o.renewal_id <> 0
		) THEN amount ELSE 0 END), 0) AS purchase`
	var agg struct {
		Purchase float64 `gorm:"column:purchase"`
		Renewal  float64 `gorm:"column:renewal"`
	}
	if err = base.Select(query).Scan(&agg).Error; err != nil {
		return 0, 0, err
	}
	return agg.Purchase, agg.Renewal, nil
}

// Stats 汇总全部流水的收入合计、支出合计与条数。
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

// normalizeKeyword 归一化关键词（去首尾空白）。
func normalizeKeyword(raw string) string {
	return strings.TrimSpace(raw)
}
