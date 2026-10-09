// Package dto 提供财务统计子域的数据传输结构。
//
// 单一入口 GET /finance/stats 同时服务「财务总览」与「财务报表」，
// 避免总览与报表各自用列表首屏数据在前端累加（曾出现只统计前 100 条的错误口径）。
package dto

// 统计粒度。
const (
	GranularityDay   = "day"   // 按日（period 形如 2026-10-09）
	GranularityMonth = "month" // 按月（period 形如 2026-10）
)

// StatsQuery 财务统计查询。
type StatsQuery struct {
	StartTime   string `form:"start_time"`  // 开始日期 YYYY-MM-DD
	EndTime     string `form:"end_time"`    // 结束日期 YYYY-MM-DD
	Granularity string `form:"granularity"` // day/month，默认 day；跨度 > 366 天时自动升为 month
}

// StatsResponse 财务统计聚合结果。
type StatsResponse struct {
	Range         StatsRange           `json:"range"`
	Summary       StatsSummary         `json:"summary"`
	Trend         []StatsTrendPoint    `json:"trend"`
	TypeBreakdown []StatsTypeBreakdown `json:"type_breakdown"`
	Bills         StatsBillSummary     `json:"bills"`
	Pending       StatsPendingSummary  `json:"pending"`
	Wallet        StatsWalletSummary   `json:"wallet"`
	// Caliber 口径说明：前端直接在页脚展示，避免「看懂数字要读代码」。
	Caliber string `json:"caliber"`
}

// StatsRange 实际生效的统计区间。
type StatsRange struct {
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Granularity string `json:"granularity"`
}

// StatsSummary 期间汇总（资金口径：不含冻结/解冻内部划转）。
type StatsSummary struct {
	IncomeTotal   float64 `json:"income_total"`   // 期间收入合计
	ExpenseTotal  float64 `json:"expense_total"`  // 期间支出合计
	NetTotal      float64 `json:"net_total"`      // 净额 = 收入 - 支出
	TxCount       int64   `json:"tx_count"`       // 口径内流水笔数
	AvgAmount     float64 `json:"avg_amount"`     // 笔均变动金额
	InternalCount int64   `json:"internal_count"` // 冻结/解冻内部划转笔数（不计入收支）
}

// StatsTrendPoint 趋势点（按粒度聚合）。
type StatsTrendPoint struct {
	Period  string  `json:"period"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
	Count   int64   `json:"count"`
}

// StatsTypeBreakdown 按流水类型的分布。
type StatsTypeBreakdown struct {
	Type     string  `json:"type"`
	Income   float64 `json:"income"`
	Expense  float64 `json:"expense"`
	Net      float64 `json:"net"`
	Count    int64   `json:"count"`
	Internal bool    `json:"internal"` // 冻结/解冻：可用↔冻结内部划转，不计入收支口径
}

// StatsBillStatusRow 账单状态分布行。
type StatsBillStatusRow struct {
	Status string  `json:"status"`
	Count  int64   `json:"count"`
	Amount float64 `json:"amount"`
}

// StatsBillSummary 账单口径汇总。期间字段按账单生成时间过滤，未结字段为全量。
type StatsBillSummary struct {
	Count          int64                `json:"count"`           // 期间账单数
	TotalAmount    float64              `json:"total_amount"`    // 期间应收（按期应结）
	RefundAmount   float64              `json:"refund_amount"`   // 期间退款
	NetAmount      float64              `json:"net_amount"`      // 期间净应收
	RechargeCount  int64                `json:"recharge_count"`  // 期间充值账单数（逐笔凭证）
	RechargeAmount float64              `json:"recharge_amount"` // 期间充值账单金额
	ByStatus       []StatsBillStatusRow `json:"by_status"`       // 期间账单状态分布
	UnpaidCount    int64                `json:"unpaid_count"`    // 全量未结账单数
	UnpaidAmount   float64              `json:"unpaid_amount"`   // 全量未结应收
}

// StatsPendingSummary 待办口径：需要财务处理的单据。
type StatsPendingSummary struct {
	RechargePendingCount  int64   `json:"recharge_pending_count"`  // 待确认充值笔数
	RechargePendingAmount float64 `json:"recharge_pending_amount"` // 待确认充值金额
	WithdrawPendingCount  int64   `json:"withdraw_pending_count"`  // 待审核提现笔数
	WithdrawPendingAmount float64 `json:"withdraw_pending_amount"` // 待审核提现金额
	WithdrawPayingCount   int64   `json:"withdraw_paying_count"`   // 已通过待打款笔数
	WithdrawPayingAmount  float64 `json:"withdraw_paying_amount"`  // 已通过待打款金额
	InvoicePendingCount   int64   `json:"invoice_pending_count"`   // 待开票申请数
}

// StatsWalletSummary 平台钱包口径。
type StatsWalletSummary struct {
	BalanceTotal        float64 `json:"balance_total"`         // 可用余额合计
	FrozenTotal         float64 `json:"frozen_total"`          // 冻结金额合计
	Count               int64   `json:"count"`                 // 钱包账户数
	LowBalanceThreshold float64 `json:"low_balance_threshold"` // 低余额预警阈值（财务配置）
	LowBalanceCount     int64   `json:"low_balance_count"`     // 低于阈值的钱包数
	LowBalanceAmount    float64 `json:"low_balance_amount"`    // 低于阈值钱包的余额合计
}
