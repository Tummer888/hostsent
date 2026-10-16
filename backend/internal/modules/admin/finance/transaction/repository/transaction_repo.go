package repository

import (
	"context"
	"math"
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
	// ReconTotals 按真实收付口径汇总：收入、支出、可用余额合计、冻结合计。
	// 口径见 ReconOffenders；收入减支出应等于「可用余额 + 冻结」。
	ReconTotals(ctx context.Context) (*ReconTotalsRow, error)
	// ReconOffenders 逐户校验「账户记账值 = 台账推出值」，只返回不符的账户。
	ReconOffenders(ctx context.Context, tolerance float64) ([]ReconOffender, error)
	// ReconDocOffenders 校验业务单据与台账是否对得上（充值到账、退款回补）。
	ReconDocOffenders(ctx context.Context, tolerance float64) ([]ReconOffender, error)
}

// ReconTotalsRow 真实收付口径汇总。
type ReconTotalsRow struct {
	// Income 真实收入：direction=1 且非 unfreeze（解冻是冻结转回可用，非新增收入）。
	Income float64 `gorm:"column:income"`
	// Expense 真实支出：direction=-1 且非 freeze（冻结只是把钱挪进冻结列）。
	// 提现出账在 settle 时计入（结算流水的 direction=-1，不在排除之列）。
	Expense float64 `gorm:"column:expense"`
	// Balance / Frozen 钱包账户当前可用与冻结合计。
	Balance float64 `gorm:"column:balance"`
	Frozen  float64 `gorm:"column:frozen"`
}

// ReconOffender 对账差异：某账户（或某单据）的记账值与台账推出值不符。
//
// 只报差异行而不是全量：全平台几万个账户，全量返回既没人看也传不动，
// 财务要的是「哪几个对不上、差多少」，逐条能追到 user_id 才有用。
type ReconOffender struct {
	UserID   uint64  `gorm:"column:user_id"`
	Username string  `gorm:"column:username"`
	Check    string  `gorm:"column:check_key"`     // balance/frozen/income/expense/consume/recharge/refund
	Account  float64 `gorm:"column:account_value"` // 账户/单据上记的值
	Ledger   float64 `gorm:"column:ledger_value"`  // 台账推出的值
	Diff     float64 `gorm:"column:diff"`          // account - ledger
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

// internalTxTypes 冻解/解冻：可用余额 ↔ 冻结余额的内部划转，不属于真实收付口径。
// 定义收敛到 model.InternalTransferTypes，避免统计与对账各维护一份而漏改。
var internalTxTypes = model.InternalTransferTypes

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
//
// 消费类型取 model.ConsumeTypes（consume + 后台代客下单 order）：此前只认 consume，
// 代客下单扣款不进账单，账单金额比实际收的钱少（实测用户 2730：代客单 18 元不在账单里）。
func (r *transactionRepository) SumConsumeSplit(ctx context.Context, userID uint64, start, end *time.Time) (purchase, renewal float64, err error) {
	base := r.db.WithContext(ctx).Model(&model.WalletTransaction{}).
		Where("type IN ?", model.ConsumeTypes).
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

// ReconTotals 按真实收付口径汇总（剔除内部划转），并给出钱包可用/冻结合计。
// 恒等式：Income - Expense == Balance + Frozen。对不上说明台账与账户已脱节。
func (r *transactionRepository) ReconTotals(ctx context.Context) (*ReconTotalsRow, error) {
	var row ReconTotalsRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE((SELECT SUM(amount) FROM wallet_transactions
				WHERE direction = 1 AND type <> 'unfreeze'), 0) AS income,
			COALESCE((SELECT SUM(amount) FROM wallet_transactions
				WHERE direction = -1 AND type <> 'freeze'), 0) AS expense,
			COALESCE((SELECT SUM(balance) FROM wallet_accounts), 0) AS balance,
			COALESCE((SELECT SUM(frozen) FROM wallet_accounts), 0) AS frozen`).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ReconOffenders 逐户校验「账户记账值 = 台账推出值」，返回不符的账户明细。
//
// 三项恒等式（均由 wallet_transactions 推出，是唯一可信来源）：
//
//	balance  = Σ(balance_after - balance_before)
//	           直接累加每笔流水自报的余额变动。比按类型拼加减法可靠：
//	           类型语义会演进（settle 从可用扣还是从冻结扣改过），而 delta 是
//	           记账那一刻真实发生的变动，不会随口径调整而失效。
//	frozen   = freeze - unfreeze - settlement
//	           冻结列只被这三种操作改：申请冻结加、驳回解冻减、打款结算减。
//	income   = Σ(direction=1 且非 unfreeze)
//	expense  = Σ(direction=-1 且非 freeze)
//	           累计收付剔除冻结/解冻：那是可用↔冻结的内部划转，钱没进出账户；
//	           提现出账在 settle 时计支出（settle 的 direction=-1，不在排除之列）。
//
// 只返回超容差的账户：全量账户里绝大多数是平的，逐条返回只会淹没有问题的行。
func (r *transactionRepository) ReconOffenders(ctx context.Context, tolerance float64) ([]ReconOffender, error) {
	tol := normalizeTolerance(tolerance)
	var rows []ReconOffender
	// UNION ALL 四个检查：任一不符即出一行，check_key 标明是哪一项。
	// 用 LEFT JOIN 而非 INNER：完全没有流水的账户也要能被查出
	// （账户有余额却一笔流水都没有，本身就是最严重的对不上）。
	sql := `
		WITH ledger AS (
			SELECT user_id,
				SUM(balance_after - balance_before) AS delta,
				SUM(CASE WHEN type = 'freeze' THEN amount
				         WHEN type IN ('unfreeze','settlement') THEN -amount
				         ELSE 0 END) AS frozen,
				SUM(CASE WHEN direction = 1 AND type <> 'unfreeze' THEN amount ELSE 0 END) AS income,
				SUM(CASE WHEN direction = -1 AND type <> 'freeze' THEN amount ELSE 0 END) AS expense
			FROM wallet_transactions
			GROUP BY user_id
		)
		SELECT wa.user_id, COALESCE(u.username,'') AS username,
		       'balance' AS check_key, wa.balance AS account_value, COALESCE(l.delta,0) AS ledger_value,
		       wa.balance - COALESCE(l.delta,0) AS diff
		FROM wallet_accounts wa
		LEFT JOIN ledger l ON l.user_id = wa.user_id
		LEFT JOIN users u ON u.id = wa.user_id
		WHERE ABS(wa.balance - COALESCE(l.delta,0)) > ?
		UNION ALL
		SELECT wa.user_id, COALESCE(u.username,''), 'frozen', wa.frozen, COALESCE(l.frozen,0),
		       wa.frozen - COALESCE(l.frozen,0)
		FROM wallet_accounts wa
		LEFT JOIN ledger l ON l.user_id = wa.user_id
		LEFT JOIN users u ON u.id = wa.user_id
		WHERE ABS(wa.frozen - COALESCE(l.frozen,0)) > ?
		UNION ALL
		SELECT wa.user_id, COALESCE(u.username,''), 'income', wa.total_income, COALESCE(l.income,0),
		       wa.total_income - COALESCE(l.income,0)
		FROM wallet_accounts wa
		LEFT JOIN ledger l ON l.user_id = wa.user_id
		LEFT JOIN users u ON u.id = wa.user_id
		WHERE ABS(wa.total_income - COALESCE(l.income,0)) > ?
		UNION ALL
		SELECT wa.user_id, COALESCE(u.username,''), 'expense', wa.total_expense, COALESCE(l.expense,0),
		       wa.total_expense - COALESCE(l.expense,0)
		FROM wallet_accounts wa
		LEFT JOIN ledger l ON l.user_id = wa.user_id
		LEFT JOIN users u ON u.id = wa.user_id
		WHERE ABS(wa.total_expense - COALESCE(l.expense,0)) > ?
		ORDER BY diff DESC`
	if err := r.db.WithContext(ctx).Raw(sql, tol, tol, tol, tol).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ReconDocOffenders 校验业务单据与台账是否对得上，返回不一致的单据。
//
//	充值：recharges.status='success' 的每笔都应在台账里有一条同号 recharge 流水。
//	      只写单据不记账 = 用户付了钱余额没到；只记账无单据 = 账面凭空多钱。
//	退款：order_refunds.status='approved' 且 refund_mode='balance' 的每笔都应有
//	      一条同号 refund 流水。原路退回（channel）不产生钱包流水，不在本检查内。
//
// 这两类是「钱进出平台」的入口，单据与台账一旦脱钩，账面数就不再有业务依据。
func (r *transactionRepository) ReconDocOffenders(ctx context.Context, tolerance float64) ([]ReconOffender, error) {
	tol := normalizeTolerance(tolerance)
	var rows []ReconOffender
	sql := `
		WITH doc AS (
			SELECT user_id, recharge_no AS doc_no, 'recharge' AS doc_kind, amount
			FROM recharges WHERE status = 'success'
			UNION ALL
			SELECT user_id, refund_no, 'refund', amount
			FROM order_refunds WHERE status = 'approved' AND refund_mode = 'balance'
		),
		ledger AS (
			SELECT user_id, ref_no, SUM(amount) AS amount
			FROM wallet_transactions
			WHERE type IN ('recharge','refund')
			GROUP BY user_id, ref_no
		)
		SELECT COALESCE(d.user_id, l.user_id) AS user_id,
		       COALESCE(u.username,'') AS username,
		       COALESCE(d.doc_kind, 'orphan_ledger') AS check_key,
		       COALESCE(d.amount, 0) AS account_value,
		       COALESCE(l.amount, 0) AS ledger_value,
		       COALESCE(d.amount, 0) - COALESCE(l.amount, 0) AS diff
		FROM doc d
		FULL OUTER JOIN ledger l
		  ON l.user_id = d.user_id AND l.ref_no = d.doc_no
		LEFT JOIN users u ON u.id = COALESCE(d.user_id, l.user_id)
		WHERE ABS(COALESCE(d.amount,0) - COALESCE(l.amount,0)) > ?
		ORDER BY diff DESC`
	if err := r.db.WithContext(ctx).Raw(sql, tol).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// normalizeTolerance 容差归零：负数或非有限值一律按 0 处理（严格对账），
// 避免调用方传负容差把「任何差异都不算差异」当成通过。
func normalizeTolerance(tolerance float64) float64 {
	if tolerance < 0 || math.IsNaN(tolerance) || math.IsInf(tolerance, 0) {
		return 0
	}
	return tolerance
}
