// Package dto 提供成本管理子域的数据传输结构。
//
// 口径（与 doc111 保持一致，页面页脚直接展示）：
//   - 收入：服务收入 = 期间消费 − 期间退款（订单口径，与「资金口径收入」区分：
//     后者含充值/提现等资金搬运，不能当营收）；资金口径作为参考值一并返回。
//   - 成本：① 上游账本消费流水（余额支付的开通/续费 − 对应退款，适配器自动同步）
//     ② 成本项配置（母机月费/工资/机房…）③ 用户佣金入账 ④ 推广返现计提。
//   - 利润 = 服务收入 − 成本合计；利润率 = 利润 / 服务收入。
//   - 周期：按自然月（月初 00:00 至次月月初，右开区间）。
package dto

// —— 成本项 ——

// CostItemInfo 成本项。
type CostItemInfo struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	CategoryLabel string  `json:"category_label"`
	Amount        float64 `json:"amount"`
	Cycle         string  `json:"cycle"`
	OccurredOn    string  `json:"occurred_on"` // once 时非空
	EffectiveFrom string  `json:"effective_from"`
	EffectiveTo   string  `json:"effective_to"` // 空=长期有效
	Subject       string  `json:"subject"`
	Remark        string  `json:"remark"`
	Status        string  `json:"status"`
	OperatorID    uint64  `json:"operator_id"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	// MonthlyAmount 该成本项对「当月」的计入金额（once 只在发生月计入，其它月为 0）。
	MonthlyAmount float64 `json:"monthly_amount"`
}

// CostItemListQuery 成本项列表查询。
type CostItemListQuery struct {
	Keyword  string `form:"keyword"`
	Category string `form:"category"`
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// CostItemListResponse 成本项列表响应。
type CostItemListResponse struct {
	Items []CostItemInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
	// MonthlyTotal 列表筛选口径下的「按月计入合计」（不受分页影响）。
	MonthlyTotal float64 `json:"monthly_total"`
}

// CostItemRequest 成本项新增/编辑请求。
type CostItemRequest struct {
	Name          string  `json:"name" binding:"required"`
	Category      string  `json:"category" binding:"required"`
	Amount        float64 `json:"amount"`
	Cycle         string  `json:"cycle"`
	OccurredOn    string  `json:"occurred_on"`
	EffectiveFrom string  `json:"effective_from"`
	EffectiveTo   string  `json:"effective_to"`
	Subject       string  `json:"subject"`
	Remark        string  `json:"remark"`
	Status        string  `json:"status"`
}

// —— 上游余额台账 ——

// UpstreamLedgerRow 单渠道的上游台账行。
//
// 全部数据自动来源：余额 = 最近一次抓取的快照；消耗/充值 = 上游账本流水（按自然月归集）。
// 渠道适配器没有账本能力时消耗/充值为 null（成本需用「成本项配置」登记）。
type UpstreamLedgerRow struct {
	ProviderID   uint64 `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
	// LatestBalance 最新余额快照（每日自动抓取 + 页面手动触发抓取），当月无快照时为 null。
	LatestBalance *float64 `json:"latest_balance"`
	LatestDate    string   `json:"latest_date"`
	Currency      string   `json:"currency"`
	// Consumption 期间消耗（流水口径净额 = amount − refund_amount）；渠道无账本数据时为 null。
	// 已同步过账本的渠道当月无流水即 0（确实没有消费），不会用别的口径顶替。
	Consumption *float64 `json:"consumption"`
	// ConsumptionEntries 期间消费笔数。
	ConsumptionEntries int `json:"consumption_entries"`
	// TopupTotal 期间充值合计（流水口径，正=充值）；渠道无账本数据时为 null。
	TopupTotal *float64 `json:"topup_total"`
	// CostSource 期间成本的取数来源：ledger（账本流水）/ none（未接入或尚未同步账本）。
	CostSource string `json:"cost_source"`
}

// UpstreamLedgerResponse 上游余额台账响应。
type UpstreamLedgerResponse struct {
	Month string              `json:"month"`
	Rows  []UpstreamLedgerRow `json:"rows"`
	// TotalConsumption 全部渠道期间消耗合计（只统计有账本数据的渠道）。
	TotalConsumption float64 `json:"total_consumption"`
	// TotalTopup 全部渠道期间充值合计（只统计有账本数据的渠道）。
	TotalTopup float64 `json:"total_topup"`
	// BalanceSupportedProviders 支持自动抓取余额的渠道 ID（适配器实现 AccountReader）。
	BalanceSupportedProviders []uint64 `json:"balance_supported_providers"`
	// LedgerSupportedProviders 支持同步上游账本的渠道 ID（适配器实现 FinanceLedgerReader）。
	LedgerSupportedProviders []uint64 `json:"ledger_supported_providers"`
	// LedgerSyncedAt 最近一次账本同步时间（RFC3339，空=未同步过）。
	LedgerSyncedAt string `json:"ledger_synced_at"`
	// Alerts 上游余额水位告警（余额 < 未来 30 天到期金额，或 < 配置阈值）。
	Alerts []BalanceAlert `json:"alerts"`
}

// BalanceSnapshotInfo 一次余额抓取的结果（落库后的当日快照）。
type BalanceSnapshotInfo struct {
	ProviderID   uint64  `json:"provider_id"`
	ProviderName string  `json:"provider_name"`
	SnapshotDate string  `json:"snapshot_date"`
	Balance      float64 `json:"balance"`
	Currency     string  `json:"currency"`
}

// BalanceFetchRequest 余额抓取请求。
type BalanceFetchRequest struct {
	ProviderID uint64 `json:"provider_id" binding:"required"`
}

// BalanceAlert 上游余额水位告警。
//
// 判定（doc111 §5.3）：critical = 余额 < 未来 30 天到期金额（不够付下月续费）；
// warning = 余额 < 配置阈值（finance.upstream_balance_warning，0=不启用固定阈值）；
// ok = 两者都不触发（也返回，便于页面统一展示「余额水位」）。
type BalanceAlert struct {
	ProviderID   uint64  `json:"provider_id"`
	ProviderName string  `json:"provider_name"`
	Level        string  `json:"level"` // ok | warning | critical
	Balance      float64 `json:"balance"`
	Currency     string  `json:"currency"`
	// DueWithin30d 未来 30 天到期金额合计（上游主机清单口径）。
	DueWithin30d float64 `json:"due_within_30d"`
	DueCount     int     `json:"due_count"`
	// Threshold 配置的低水位阈值（0=未配置）。
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`
}

// —— 上游账本同步 ——

// LedgerSyncRequest 账本同步请求。
type LedgerSyncRequest struct {
	// ProviderID 0=全部支持账本的渠道。
	ProviderID uint64 `json:"provider_id"`
	// Full true=全量回填（首次接入/数据修复），false=增量（按上游自增 ID 断点续传）。
	Full bool `json:"full"`
}

// LedgerSyncProviderResult 单渠道同步结果。
type LedgerSyncProviderResult struct {
	ProviderID   uint64 `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	// Consumption/Topup 本次写入的消费/充值条数（含更新的历史行）。
	Consumption int `json:"consumption"`
	Topup       int `json:"topup"`
	// ConsumptionTotal/TopupTotal 上游侧总数（便于判断是否追平）。
	ConsumptionTotal int `json:"consumption_total"`
	TopupTotal       int `json:"topup_total"`
	// Failed 失败原因（空=成功）。
	Failed string `json:"failed"`
}

// LedgerSyncResponse 账本同步响应。
type LedgerSyncResponse struct {
	Results []LedgerSyncProviderResult `json:"results"`
}

// —— 上游账本流水（页面「上游流水」页签） ——

// LedgerEntryQuery 账本流水查询。
type LedgerEntryQuery struct {
	ProviderID uint64 `form:"provider_id"`
	// Kind 空=消费+充值；consume/topup 指定单边。
	Kind     string `form:"kind"`
	Month    string `form:"month"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// LedgerEntryInfo 账本流水条目。
type LedgerEntryInfo struct {
	ID           uint64  `json:"id"`
	ProviderID   uint64  `json:"provider_id"`
	ProviderName string  `json:"provider_name"`
	Kind         string  `json:"kind"`
	OccurredAt   string  `json:"occurred_at"`
	Amount       float64 `json:"amount"`
	RefundAmount float64 `json:"refund_amount"`
	// NetAmount 净额 = amount − refund_amount（消费口径）。
	NetAmount   float64 `json:"net_amount"`
	Category    string  `json:"category"`
	RefNo       string  `json:"ref_no"`
	Description string  `json:"description"`
	Currency    string  `json:"currency"`
}

// LedgerEntryListResponse 账本流水列表（含筛选口径合计）。
type LedgerEntryListResponse struct {
	Items []LedgerEntryInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
	// ConsumptionTotal/TopupTotal 当前筛选口径的净额合计（消费为净额）。
	ConsumptionTotal float64 `json:"consumption_total"`
	TopupTotal       float64 `json:"topup_total"`
}

// —— 成本总览 ——

// CostLine 成本构成行（自动项与配置项同构，来源字段区分）。
type CostLine struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Amount   float64 `json:"amount"`
	Auto     bool    `json:"auto"`     // true=系统按数据自动计算，false=来自成本项配置
	Category string  `json:"category"` // 配置项的分类（自动项为空）
	Usage    string  `json:"usage"`    // 数据来源说明
}

// CostTrendPoint 近 N 月趋势点。
type CostTrendPoint struct {
	Month          string  `json:"month"` // YYYY-MM
	ServiceRevenue float64 `json:"service_revenue"`
	CostTotal      float64 `json:"cost_total"`
	UpstreamCost   float64 `json:"upstream_cost"`
	FixedCost      float64 `json:"fixed_cost"`
	OtherCost      float64 `json:"other_cost"`
	Profit         float64 `json:"profit"`
	ProfitRate     float64 `json:"profit_rate"` // 百分比，保留 2 位
}

// CostOverviewResponse 成本总览响应。
type CostOverviewResponse struct {
	Month   string `json:"month"`   // 统计月 YYYY-MM
	AsOf    string `json:"as_of"`   // 数据截至日期（当月为今天，历史月为月末）
	Current bool   `json:"current"` // 是否当月（月中视图：固定成本按天摊）
	// Revenue 收入：服务收入为主口径，资金口径为参考值。
	ServiceRevenue float64 `json:"service_revenue"`
	FundIncome     float64 `json:"fund_income"`
	ConsumeTotal   float64 `json:"consume_total"`
	RefundTotal    float64 `json:"refund_total"`
	// Cost 成本合计（口径内：上游消耗 + 成本项 + 佣金 + 返现计提）。
	CostTotal float64 `json:"cost_total"`
	// CostToDate 成本按「固定成本按天摊到截至日」重算后的合计（对比整月口径，月中参考）。
	CostToDate      float64 `json:"cost_to_date"`
	UpstreamCost    float64 `json:"upstream_cost"`
	FixedCost       float64 `json:"fixed_cost"`         // 成本项合计（整月口径）
	FixedCostToDate float64 `json:"fixed_cost_to_date"` // 成本项按天摊到截至日（当月才有意义）
	CommissionCost  float64 `json:"commission_cost"`
	ReferralCost    float64 `json:"referral_cost"`
	Profit          float64 `json:"profit"`
	ProfitRate      float64 `json:"profit_rate"`
	// ProfitToDate/ProfitRateToDate 用按天摊后的固定成本计算的利润与利润率（月中参考）。
	ProfitToDate     float64 `json:"profit_to_date"`
	ProfitRateToDate float64 `json:"profit_rate_to_date"`
	// UpstreamCostSource 上游成本取数来源：ledger（有渠道同步了账本）/ none（无账本数据）。
	UpstreamCostSource string `json:"upstream_cost_source"`
	// LedgerSyncedAt 最近一次账本同步时间（RFC3339，空=未同步过）。
	LedgerSyncedAt string `json:"ledger_synced_at"`
	// BalanceAlerts 上游余额水位告警。
	BalanceAlerts []BalanceAlert      `json:"balance_alerts"`
	CostLines     []CostLine          `json:"cost_lines"`
	UpstreamRows  []UpstreamLedgerRow `json:"upstream_rows"`
	Trend         []CostTrendPoint    `json:"trend"`
	Caliber       string              `json:"caliber"`
	// UnconfiguredHint 未配置成本项时的提示（前端直接展示，避免「成本为 0」被误读）。
	UnconfiguredHint string `json:"unconfigured_hint"`
}

// ListMeta 通用分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}
