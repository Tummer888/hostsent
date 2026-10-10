// Package repository 提供财务统计的聚合查询（全部为 SQL 侧全量聚合）。
//
// 为什么不用列表接口在前端累加：列表单页上限 100 条，累加结果是「前 100 条的口径」，
// 数据量上来后必然错。此处所有汇总都在数据库侧完成，与分页无关。
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	accountmodel "hostsent/backend/internal/modules/admin/finance/account/model"
	rechargemodel "hostsent/backend/internal/modules/admin/finance/account/recharge/model"
	billmodel "hostsent/backend/internal/modules/admin/finance/bill/model"
	withdrawmodel "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/model"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
)

// internalTxTypes 冻结/解冻：可用余额 ↔ 冻结余额的内部划转，
// 不是真实的收入/支出。口径中剔除，否则提现会被 freeze + settle 记两次支出。
var internalTxTypes = []string{transmodel.TxTypeFreeze, transmodel.TxTypeUnfreeze}

// TxAmountRow 金额+笔数聚合行。
type TxAmountRow struct {
	Income  float64 `gorm:"column:income"`
	Expense float64 `gorm:"column:expense"`
	Count   int64   `gorm:"column:count"`
}

// TxTypeRow 按流水类型的聚合行。
type TxTypeRow struct {
	Type    string  `gorm:"column:type"`
	Income  float64 `gorm:"column:income"`
	Expense float64 `gorm:"column:expense"`
	Count   int64   `gorm:"column:count"`
}

// TrendRow 趋势聚合行。
type TrendRow struct {
	Period  string  `gorm:"column:period"`
	Income  float64 `gorm:"column:income"`
	Expense float64 `gorm:"column:expense"`
	Count   int64   `gorm:"column:count"`
}

// BillRangeRow 账单期间聚合行。
type BillRangeRow struct {
	Count          int64   `gorm:"column:count"`
	TotalAmount    float64 `gorm:"column:total_amount"`
	RefundAmount   float64 `gorm:"column:refund_amount"`
	RechargeCount  int64   `gorm:"column:recharge_count"`
	RechargeAmount float64 `gorm:"column:recharge_amount"`
}

// BillStatusRow 账单状态聚合行。
type BillStatusRow struct {
	Status string  `gorm:"column:status"`
	Count  int64   `gorm:"column:count"`
	Amount float64 `gorm:"column:amount"`
}

// StatsRepository 财务统计聚合数据访问。
type StatsRepository interface {
	// TxSummary 期间资金口径汇总（剔除冻结/解冻内部划转）。
	TxSummary(ctx context.Context, start, end time.Time) (*TxAmountRow, error)
	// TxInternalCount 期间冻结/解冻内部划转笔数。
	TxInternalCount(ctx context.Context, start, end time.Time) (int64, error)
	// TxTrend 按日/月聚合趋势（资金口径）。
	TxTrend(ctx context.Context, start, end time.Time, granularity string) ([]TrendRow, error)
	// TxByType 按流水类型聚合（含冻结/解冻，供明细分布展示）。
	TxByType(ctx context.Context, start, end time.Time) ([]TxTypeRow, error)
	// BillRange 期间账单聚合（按生成时间）。
	BillRange(ctx context.Context, start, end time.Time) (*BillRangeRow, error)
	// BillByStatus 期间账单状态分布。
	BillByStatus(ctx context.Context, start, end time.Time) ([]BillStatusRow, error)
	// BillUnpaid 全量未结账单数与应结金额。
	BillUnpaid(ctx context.Context) (int64, float64, error)
	// RechargeByStatus 充值单按状态聚合。
	RechargeByStatus(ctx context.Context, status string) (int64, float64, error)
	// WithdrawByStatuses 提现单按状态聚合。
	WithdrawByStatuses(ctx context.Context, statuses []string) (int64, float64, error)
	// InvoicePending 待开票申请数。
	InvoicePending(ctx context.Context) (int64, error)
	// WalletTotals 钱包可用余额/冻结合计与账户数。
	WalletTotals(ctx context.Context) (balance, frozen float64, count int64, err error)
	// WalletBelow 余额低于阈值的钱包数与余额合计。
	WalletBelow(ctx context.Context, threshold float64) (int64, float64, error)
}

type statsRepository struct {
	db *gorm.DB
}

// NewStatsRepository 创建财务统计仓储。
func NewStatsRepository(db *gorm.DB) StatsRepository {
	return &statsRepository{db: db}
}

func (r *statsRepository) TxSummary(ctx context.Context, start, end time.Time) (*TxAmountRow, error) {
	var row TxAmountRow
	err := r.db.WithContext(ctx).Model(&transmodel.WalletTransaction{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("type NOT IN ?", internalTxTypes).
		Select(`COALESCE(SUM(CASE WHEN direction = 1 THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN direction = -1 THEN amount ELSE 0 END), 0) AS expense,
			COUNT(*) AS count`).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *statsRepository) TxInternalCount(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&transmodel.WalletTransaction{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("type IN ?", internalTxTypes).
		Count(&count).Error
	return count, err
}

func (r *statsRepository) TxTrend(ctx context.Context, start, end time.Time, granularity string) ([]TrendRow, error) {
	format := "YYYY-MM-DD"
	if granularity == "month" {
		format = "YYYY-MM"
	}
	var rows []TrendRow
	err := r.db.WithContext(ctx).Model(&transmodel.WalletTransaction{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("type NOT IN ?", internalTxTypes).
		Select(`to_char(created_at, '` + format + `') AS period,
			COALESCE(SUM(CASE WHEN direction = 1 THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN direction = -1 THEN amount ELSE 0 END), 0) AS expense,
			COUNT(*) AS count`).
		Group("period").
		Order("period asc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *statsRepository) TxByType(ctx context.Context, start, end time.Time) ([]TxTypeRow, error) {
	var rows []TxTypeRow
	err := r.db.WithContext(ctx).Model(&transmodel.WalletTransaction{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Select(`type,
			COALESCE(SUM(CASE WHEN direction = 1 THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN direction = -1 THEN amount ELSE 0 END), 0) AS expense,
			COUNT(*) AS count`).
		Group("type").
		Order("type asc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *statsRepository) BillRange(ctx context.Context, start, end time.Time) (*BillRangeRow, error) {
	var row BillRangeRow
	err := r.db.WithContext(ctx).Model(&billmodel.Bill{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Select(`COUNT(*) AS count,
			COALESCE(SUM(CASE WHEN source_type <> 'recharge' THEN total_amount ELSE 0 END), 0) AS total_amount,
			COALESCE(SUM(refund_amount), 0) AS refund_amount,
			COALESCE(SUM(CASE WHEN source_type = 'recharge' THEN 1 ELSE 0 END), 0) AS recharge_count,
			COALESCE(SUM(CASE WHEN source_type = 'recharge' THEN recharge_amount ELSE 0 END), 0) AS recharge_amount`).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *statsRepository) BillByStatus(ctx context.Context, start, end time.Time) ([]BillStatusRow, error) {
	var rows []BillStatusRow
	err := r.db.WithContext(ctx).Model(&billmodel.Bill{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Select(`status, COUNT(*) AS count, COALESCE(SUM(total_amount), 0) AS amount`).
		Group("status").
		Order("status asc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *statsRepository) BillUnpaid(ctx context.Context) (int64, float64, error) {
	var row struct {
		Count  int64   `gorm:"column:count"`
		Amount float64 `gorm:"column:amount"`
	}
	err := r.db.WithContext(ctx).Model(&billmodel.Bill{}).
		Where("status = ?", "unpaid").
		Select("COUNT(*) AS count, COALESCE(SUM(total_amount), 0) AS amount").
		Scan(&row).Error
	if err != nil {
		return 0, 0, err
	}
	return row.Count, row.Amount, nil
}

func (r *statsRepository) RechargeByStatus(ctx context.Context, status string) (int64, float64, error) {
	var row struct {
		Count  int64   `gorm:"column:count"`
		Amount float64 `gorm:"column:amount"`
	}
	err := r.db.WithContext(ctx).Model(&rechargemodel.Recharge{}).
		Where("status = ?", status).
		Select("COUNT(*) AS count, COALESCE(SUM(amount), 0) AS amount").
		Scan(&row).Error
	if err != nil {
		return 0, 0, err
	}
	return row.Count, row.Amount, nil
}

func (r *statsRepository) WithdrawByStatuses(ctx context.Context, statuses []string) (int64, float64, error) {
	var row struct {
		Count  int64   `gorm:"column:count"`
		Amount float64 `gorm:"column:amount"`
	}
	err := r.db.WithContext(ctx).Model(&withdrawmodel.Withdraw{}).
		Where("status IN ?", statuses).
		Select("COUNT(*) AS count, COALESCE(SUM(amount), 0) AS amount").
		Scan(&row).Error
	if err != nil {
		return 0, 0, err
	}
	return row.Count, row.Amount, nil
}

func (r *statsRepository) InvoicePending(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&billmodel.InvoiceRequest{}).
		Where("status = ?", billmodel.InvoiceReqPending).
		Count(&count).Error
	return count, err
}

func (r *statsRepository) WalletTotals(ctx context.Context) (balance, frozen float64, count int64, err error) {
	var row struct {
		Balance float64 `gorm:"column:balance"`
		Frozen  float64 `gorm:"column:frozen"`
		Count   int64   `gorm:"column:count"`
	}
	if err = r.db.WithContext(ctx).Model(&accountmodel.WalletAccount{}).
		Select("COALESCE(SUM(balance), 0) AS balance, COALESCE(SUM(frozen), 0) AS frozen, COUNT(*) AS count").
		Scan(&row).Error; err != nil {
		return 0, 0, 0, err
	}
	return row.Balance, row.Frozen, row.Count, nil
}

func (r *statsRepository) WalletBelow(ctx context.Context, threshold float64) (int64, float64, error) {
	var row struct {
		Count  int64   `gorm:"column:count"`
		Amount float64 `gorm:"column:amount"`
	}
	err := r.db.WithContext(ctx).Model(&accountmodel.WalletAccount{}).
		Where("balance < ?", threshold).
		Select("COUNT(*) AS count, COALESCE(SUM(balance), 0) AS amount").
		Scan(&row).Error
	if err != nil {
		return 0, 0, err
	}
	return row.Count, row.Amount, nil
}
