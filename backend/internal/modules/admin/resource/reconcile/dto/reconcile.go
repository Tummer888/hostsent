// Package dto 提供实例对账模块的数据传输结构。
//
// 实例对账（本轮 S4）面向「已开通、对接上游」的实例（instances.source_mode='upstream'），
// 逐实例比对本地的售价与到期时间同上游侧的成本价与到期时间，回答两个问题：
//  1. 本地售价是否低于上游成本（卖一台亏一台）、毛利是否过薄；
//  2. 本地到期时间与上游是否一致（过早 = 用户少用、过晚 = 上游已断服而本地仍显示有效）。
//
// 对账只读，不写任何业务表；异常等级与判据集中在 repository 的 SQL 中，保证筛选与展示同源。
package dto

import commondto "hostsent/backend/internal/modules/admin/resource/common/dto"

// ListMeta 通用分页元信息（复用 common 包定义）
type ListMeta = commondto.ListMeta

// 价格向异常码。
const (
	// PriceBelowCost 本地售价低于上游成本。
	PriceBelowCost = "below_cost"
	// PriceThinMargin 毛利低于阈值（默认 5%）。
	PriceThinMargin = "thin_margin"
	// PriceMissingLocal 实例未绑定售出商品本地售价，无法比对。
	PriceMissingLocal = "missing_local_price"
	// PriceMissingCost 上游侧未提供成本价，无法比对。
	PriceMissingCost = "missing_cost"
	// PriceOK 价格一致（无异常）。
	PriceOK = "ok"
)

// 到期向异常码。
const (
	// ExpireEarly 本地到期早于上游（用户已付费区间被截短）。
	ExpireEarly = "expire_early"
	// ExpireLate 本地到期晚于上游（上游可能已停服而本地仍视为有效）。
	ExpireLate = "expire_late"
	// ExpireMissingLocal 本地无到期时间，无法比对。
	ExpireMissingLocal = "missing_local_expire"
	// ExpireMissingUpstream 上游未返回到期时间，无法比对。
	ExpireMissingUpstream = "missing_upstream_expire"
	// ExpireOK 到期时间一致（在容差内）。
	ExpireOK = "ok"
)

// 异常等级。
const (
	SeverityDanger  = "danger"
	SeverityWarning = "warning"
	SeverityOK      = "ok"
)

// ReconcileListQuery 实例对账列表查询。
type ReconcileListQuery struct {
	// ProviderID 按渠道过滤。
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	// Keyword 按实例名 / 实例号 / 商品名模糊搜索。
	Keyword string `form:"keyword" json:"keyword"`
	// Anomaly 异常过滤：danger/warning/ok/below_cost/thin_margin/expire_early/expire_late/missing，空表示全部。
	Anomaly  string `form:"anomaly" json:"anomaly"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// AnomalyItem 单个异常（供前端渲染标签）。
type AnomalyItem struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Severity     string `json:"severity"`
	SeverityName string `json:"severity_name"`
}

// ReconcileItem 对账行。
type ReconcileItem struct {
	ID             uint64 `json:"id"`
	InstanceID     string `json:"instance_id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	StatusName     string `json:"status_name"`
	LifecycleStage string `json:"lifecycle_stage"`
	ProviderID     uint64 `json:"provider_id"`
	ProviderName   string `json:"provider_name"`
	UserID         uint64 `json:"user_id"`
	Username       string `json:"username"`

	// ---- 价格向 ----
	SellProductID   uint64 `json:"sell_product_id"`
	SellProductName string `json:"sell_product_name"`
	// LocalPrice 本地售价（products.price）。
	LocalPrice          *float64 `json:"local_price"`
	UpstreamProductID   uint64   `json:"upstream_product_id"`
	UpstreamProductName string   `json:"upstream_product_name"`
	UpstreamSKU         string   `json:"upstream_sku"`
	// UpstreamCost 上游成本价（实例原始数据优先，回退上游商品 cost_price）。
	UpstreamCost *float64 `json:"upstream_cost"`
	// MarginAmount / MarginRate 毛利额与毛利率（售价 - 成本）。
	MarginAmount     *float64 `json:"margin_amount"`
	MarginRate       *float64 `json:"margin_rate"`
	PriceAnomaly     string   `json:"price_anomaly"`
	PriceAnomalyName string   `json:"price_anomaly_name"`

	// ---- 到期向 ----
	LocalExpireAt    *string `json:"local_expire_at"`
	UpstreamExpireAt *string `json:"upstream_expire_at"`
	// ExpireDiffDays 上游到期 - 本地到期（天）；正数表示本地到期过早。
	ExpireDiffDays    *float64 `json:"expire_diff_days"`
	ExpireAnomaly     string   `json:"expire_anomaly"`
	ExpireAnomalyName string   `json:"expire_anomaly_name"`

	// Anomalies 该实例命中的全部异常（价格 + 到期）。
	Anomalies    []AnomalyItem `json:"anomalies"`
	Severity     string        `json:"severity"`
	SeverityName string        `json:"severity_name"`
	// Detail 一句话结论，便于直接复制到工单。
	Detail string `json:"detail"`
}

// ReconcileSummary 对账汇总（按当前筛选，不受分页影响）。
type ReconcileSummary struct {
	Total   int64 `json:"total"`
	Danger  int64 `json:"danger"`
	Warning int64 `json:"warning"`
	OK      int64 `json:"ok"`

	BelowCost     int64 `json:"below_cost"`
	ThinMargin    int64 `json:"thin_margin"`
	MissingPrice  int64 `json:"missing_price"`
	ExpireEarly   int64 `json:"expire_early"`
	ExpireLate    int64 `json:"expire_late"`
	MissingExpire int64 `json:"missing_expire"`

	// NegativeMargin 毛利为负的实例数（与 BelowCost 同源但按金额口径）。
	NegativeMargin int64 `json:"negative_margin"`
	// TotalMargin 全部可比实例的毛利额合计。
	TotalMargin float64 `json:"total_margin"`
	// AvgMarginRate 可比实例的平均毛利率（%，直观反映整体定价水位）。
	AvgMarginRate float64 `json:"avg_margin_rate"`
	// ToleranceDays 判为「到期一致」的容差天数（前端展示口径用）。
	ToleranceDays int `json:"tolerance_days"`
	// ThinMarginRate 毛利预警阈值（%）。
	ThinMarginRate float64 `json:"thin_margin_rate"`
}

// ReconcileListResponse 实例对账列表响应。
type ReconcileListResponse struct {
	Items   []ReconcileItem  `json:"items"`
	Meta    ListMeta         `json:"meta"`
	Summary ReconcileSummary `json:"summary"`
}
