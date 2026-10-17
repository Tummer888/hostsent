// Package service 提供实例对账业务编排：把仓储的对账行翻译为可读的异常结论，
// 并汇总整体的资金/账期风险水位。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/resource/reconcile/dto"
	"hostsent/backend/internal/modules/admin/resource/reconcile/repository"
)

// Repository 实例对账仓储能力（便于装配层注入与测试替身）。
type Repository = repository.Repository

// ReconcileService 实例对账业务能力。
type ReconcileService interface {
	List(ctx context.Context, query dto.ReconcileListQuery) (*dto.ReconcileListResponse, error)
}

type reconcileService struct {
	repo repository.Repository
}

// NewReconcileService 创建实例对账服务。
func NewReconcileService(repo repository.Repository) ReconcileService {
	return &reconcileService{repo: repo}
}

// priceAnomalyNames 价格向异常展示名。
var priceAnomalyNames = map[string]string{
	dto.PriceBelowCost:    "售价低于成本",
	dto.PriceThinMargin:   "毛利过薄",
	dto.PriceMissingLocal: "缺本地售价",
	dto.PriceMissingCost:  "缺上游成本",
	dto.PriceOK:           "",
}

// expireAnomalyNames 到期向异常展示名。
var expireAnomalyNames = map[string]string{
	dto.ExpireEarly:           "本地到期过早",
	dto.ExpireLate:            "本地到期过晚",
	dto.ExpireMissingLocal:    "缺本地到期",
	dto.ExpireMissingUpstream: "缺上游到期",
	dto.ExpireOK:              "",
}

// statusAnomalyNames 状态向异常展示名（doc120）。
//
// 措辞只描述**服务**维度：上游侧是 domainstatus（服务状态），本地侧是
// lifecycle_stage 归一后的服务口径。旧名"上游已暂停本地未同步""上游已删除本地仍有效"
// 用的是机器语感，用户关机后看到会以为"系统没帮我同步状态"（第二十一轮修复）。
var statusAnomalyNames = map[string]string{
	dto.StatusMismatch: "服务状态两侧不一致",
	dto.StatusStale:    "上游已终止本地仍在服务",
	dto.ExpireOK:       "", // 复用 "ok" 字面量，三个向的 ok 同值
}

// severityNames 等级展示名。
var severityNames = map[string]string{
	dto.SeverityDanger:  "严重",
	dto.SeverityWarning: "预警",
	dto.SeverityOK:      "正常",
}

// statusNames 本地**机器/电源**维度展示名（与实例运维台口径一致）。
//
// 注意它只用于展示本地 status 原文的场景；状态比对走 serviceStateNames
// （服务维度）—— 上游 domainstatus 是服务状态，拿机器状态去比会误报
// （第二十一轮：关机后被判成"状态不同步"，因为本地 stopped vs 上游 Active）。
var statusNames = map[string]string{
	"running":   "运行中",
	"stopped":   "已停止",
	"suspended": "已暂停",
	"pending":   "创建中",
	"creating":  "创建中",
	"deleted":   "已删除",
	"cancelled": "已取消",
	"unknown":   "未知",
}

// serviceStateNames 本地**服务**维度展示名（对账比对时的本地侧口径）。
//
// 与 statusNames 的区别：这台机器通电与否（machine）和这项服务是否在用
// （service）是两件事。上游 domainstatus 属于后者，所以本地也要用后者去对。
var serviceStateNames = map[string]string{
	"active":       "服务中",
	"grace":        "宽限期",
	"provisioning": "开通中",
	"suspended":    "已暂停",
	"destroyed":    "已终止",
	"cancelled":    "已取消",
}

func (s *reconcileService) List(ctx context.Context, query dto.ReconcileListQuery) (*dto.ReconcileListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	rows, total, err := s.repo.List(ctx, query, page, pageSize)
	if err != nil {
		return nil, err
	}
	summaryRow, err := s.repo.Summary(ctx, query)
	if err != nil {
		return nil, err
	}

	// 用户名与渠道名/链路批量装饰（失败不阻塞列表）。
	usernames := s.usernames(ctx, rows)
	providerMeta := s.providerMeta(ctx, rows)

	items := make([]dto.ReconcileItem, 0, len(rows))
	for i := range rows {
		items = append(items, buildItem(rows[i], usernames, providerMeta))
	}

	return &dto.ReconcileListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
		Summary: dto.ReconcileSummary{
			Total:          summaryRow.Total,
			Danger:         summaryRow.Danger,
			Warning:        summaryRow.Warning,
			OK:             summaryRow.OK,
			BelowCost:      summaryRow.BelowCost,
			ThinMargin:     summaryRow.ThinMargin,
			MissingPrice:   summaryRow.MissingPrice,
			ExpireEarly:    summaryRow.ExpireEarly,
			ExpireLate:     summaryRow.ExpireLate,
			MissingExpire:  summaryRow.MissingExpire,
			StatusMismatch: summaryRow.StatusMismatch,
			StatusStale:    summaryRow.StatusStale,
			NegativeMargin: summaryRow.NegativeMargin,
			TotalMargin:    round2(deref(summaryRow.TotalMargin)),
			AvgMarginRate:  round1(deref(summaryRow.AvgMarginRate)),
			ToleranceDays:  repository.ExpireToleranceDays,
			ThinMarginRate: repository.ThinMarginRate,
		},
	}, nil
}

func (s *reconcileService) usernames(ctx context.Context, rows []repository.ReconcileRow) map[uint64]string {
	ids := make([]uint64, 0, len(rows))
	seen := make(map[uint64]struct{}, len(rows))
	for i := range rows {
		id := rows[i].UserID
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	out, err := s.repo.Usernames(ctx, ids)
	if err != nil {
		return map[uint64]string{}
	}
	return out
}

func (s *reconcileService) providerMeta(ctx context.Context, rows []repository.ReconcileRow) map[uint64]repository.ProviderMeta {
	ids := make([]uint64, 0, len(rows))
	seen := make(map[uint64]struct{}, len(rows))
	for i := range rows {
		id := rows[i].ProviderID
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	out, err := s.repo.ProviderMeta(ctx, ids)
	if err != nil {
		return map[uint64]repository.ProviderMeta{}
	}
	return out
}

func buildItem(row repository.ReconcileRow, usernames map[uint64]string, providerMeta map[uint64]repository.ProviderMeta) dto.ReconcileItem {
	item := dto.ReconcileItem{
		ID:                  row.ID,
		InstanceID:          row.InstanceID,
		Name:                row.Name,
		Status:              row.Status,
		StatusName:          labelOf(statusNames, row.Status, "未知"),
		LifecycleStage:      row.LifecycleStage,
		ProviderID:          row.ProviderID,
		ProviderName:        providerMeta[row.ProviderID].Name,
		ProviderKind:        providerMeta[row.ProviderID].Kind,
		UserID:              row.UserID,
		Username:            usernames[row.UserID],
		SellProductID:       row.SellProductID,
		SellProductName:     row.SellProductName,
		LocalPrice:          row.LocalPrice,
		UpstreamProductID:   row.UpstreamProductID,
		UpstreamProductName: row.UpstreamProductName,
		UpstreamSKU:         row.UpstreamSKU,
		UpstreamCost:        row.UpstreamCost,
		MarginAmount:        round2Ptr(row.MarginAmount),
		MarginRate:          round1Ptr(row.MarginRate),
		PriceAnomaly:        row.PriceAnomaly,
		PriceAnomalyName:    priceAnomalyNames[row.PriceAnomaly],
		ExpireDiffDays:      round1Ptr(row.ExpireDiffDays),
		ExpireAnomaly:       row.ExpireAnomaly,
		ExpireAnomalyName:   expireAnomalyNames[row.ExpireAnomaly],
		UpstreamStatus:      row.UpstreamStatus,
		StatusAnomaly:       row.StatusAnomaly,
		StatusAnomalyName:   statusAnomalyNames[row.StatusAnomaly],
		// 上游侧给中文名：与本地侧（local_service_state_name）用同一套服务维度
		// 用词，页面并排展示时读数才是"服务中 / 已暂停"这类同类词。
		UpstreamStatusName: upstreamStatusLabel(row.UpstreamStatus),
		// 本地服务维度（比对口径）。空值兜底 "unknown"：老行没有 lifecycle_stage，
		// 展示成空白会被误读为"本地没有状态"。
		LocalServiceState:     defaultString(row.LocalServiceState, "unknown"),
		LocalServiceStateName: labelOf(serviceStateNames, row.LocalServiceState, "未知"),
		Severity:              row.Severity,
		SeverityName:          severityNames[row.Severity],
	}
	if row.LocalExpire != nil {
		v := row.LocalExpire.Format(time.RFC3339)
		item.LocalExpireAt = &v
	}
	if row.UpstreamExpire != nil {
		v := row.UpstreamExpire.Format(time.RFC3339)
		item.UpstreamExpireAt = &v
	}

	item.Anomalies = collectAnomalies(row)
	item.Severity = row.Severity
	if item.Severity == "" {
		item.Severity = dto.SeverityOK
	}
	item.SeverityName = severityNames[item.Severity]
	item.Detail = buildDetail(item, row)
	return item
}

// collectAnomalies 汇总命中项：价格向、到期向、状态向各至多一条，等级沿用 SQL 的判定口径。
func collectAnomalies(row repository.ReconcileRow) []dto.AnomalyItem {
	out := make([]dto.AnomalyItem, 0, 3)
	if name, ok := priceAnomalyNames[row.PriceAnomaly]; ok && name != "" {
		out = append(out, dto.AnomalyItem{
			Code:         row.PriceAnomaly,
			Name:         name,
			Severity:     anomalySeverity(row.PriceAnomaly),
			SeverityName: severityNames[anomalySeverity(row.PriceAnomaly)],
		})
	}
	if name, ok := expireAnomalyNames[row.ExpireAnomaly]; ok && name != "" {
		out = append(out, dto.AnomalyItem{
			Code:         row.ExpireAnomaly,
			Name:         name,
			Severity:     anomalySeverity(row.ExpireAnomaly),
			SeverityName: severityNames[anomalySeverity(row.ExpireAnomaly)],
		})
	}
	if name, ok := statusAnomalyNames[row.StatusAnomaly]; ok && name != "" {
		out = append(out, dto.AnomalyItem{
			Code:         row.StatusAnomaly,
			Name:         name,
			Severity:     anomalySeverity(row.StatusAnomaly),
			SeverityName: severityNames[anomalySeverity(row.StatusAnomaly)],
		})
	}
	return out
}

// anomalySeverity 单项异常的等级：资金/账期/服务状态直接受损为严重，数据缺失与薄利为预警。
func anomalySeverity(code string) string {
	switch code {
	case dto.PriceBelowCost, dto.ExpireEarly, dto.ExpireLate, dto.StatusStale:
		return dto.SeverityDanger
	default:
		return dto.SeverityWarning
	}
}

// buildDetail 生成一句话结论，便于运营直接复制进工单。
// 价格与到期两侧各自独立成句，再拼接；两侧都正常才给出「一致」结论。
func buildDetail(item dto.ReconcileItem, row repository.ReconcileRow) string {
	parts := make([]string, 0, 2)
	switch row.PriceAnomaly {
	case dto.PriceBelowCost:
		parts = append(parts, fmt.Sprintf("本地售价 %s 低于上游成本 %s，每期亏损 %s",
			money(item.LocalPrice), money(item.UpstreamCost), money(negPtr(row.MarginAmount))))
	case dto.PriceThinMargin:
		parts = append(parts, fmt.Sprintf("毛利率仅 %.1f%%，低于 %.0f%% 预警线",
			deref(item.MarginRate), repository.ThinMarginRate))
	case dto.PriceMissingLocal:
		parts = append(parts, "未绑定售出商品，本地售价缺失，无法比对成本")
	case dto.PriceMissingCost:
		parts = append(parts, "上游未提供成本价，无法比对售价")
	}
	switch row.ExpireAnomaly {
	case dto.ExpireEarly:
		parts = append(parts, fmt.Sprintf("本地到期比上游早 %.1f 天，用户已付费时长被截短", deref(item.ExpireDiffDays)))
	case dto.ExpireLate:
		parts = append(parts, fmt.Sprintf("本地到期比上游晚 %.1f 天，上游可能已停服而本地仍视为有效", -deref(item.ExpireDiffDays)))
	case dto.ExpireMissingLocal:
		parts = append(parts, "本地无到期时间，无法比对上游账期")
	case dto.ExpireMissingUpstream:
		parts = append(parts, "上游未返回到期时间，无法比对本地账期")
	}
	switch row.StatusAnomaly {
	case dto.StatusStale:
		parts = append(parts, fmt.Sprintf("上游状态已是「%s」，本地服务仍视为「%s」，可能仍在为已停服的机器计费",
			upstreamStatusLabel(row.UpstreamStatus), labelOf(serviceStateNames, row.LocalServiceState, "在用")))
	case dto.StatusMismatch:
		parts = append(parts, fmt.Sprintf("上游服务状态「%s」与本地「%s」不一致，需核对服务实际可用性",
			upstreamStatusLabel(row.UpstreamStatus), labelOf(serviceStateNames, row.LocalServiceState, "未知")))
	}
	if len(parts) == 0 {
		return "售价、到期时间与服务状态均与上游一致"
	}
	return strings.Join(parts, "；")
}

// upstreamStatusLabel 上游 domainstatus 的中文展示；未知值原样回显（便于发现新状态）。
//
// 用词与 localServiceStateNames 对齐（服务维度）：两侧并排展示时读起来才是一组
// 同类词（"服务中 / 已暂停"），而不是一边中文一边英文原文。
func upstreamStatusLabel(s string) string {
	switch s {
	case "Active":
		return "服务中"
	case "Pending":
		return "开通中"
	case "Suspended":
		return "已暂停"
	case "Cancelled":
		return "已取消"
	case "Deleted", "Terminated":
		return "已终止"
	case "Fraud":
		return "风控冻结"
	case "":
		return "未知"
	}
	return s
}

func labelOf(m map[string]string, key, fallback string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return fallback
}

// defaultString 空串兜底（展示层用；不改变原值语义）。
func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func money(v *float64) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("￥%.2f", *v)
}

func deref(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func negPtr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := -*v
	return &out
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5*sign(v))) / 100
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5*sign(v))) / 10
}

func round2Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := round2(*v)
	return &out
}

func round1Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := round1(*v)
	return &out
}

// sign 保留负数的四舍五入方向（负毛利不能被抹成 0）。
func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
