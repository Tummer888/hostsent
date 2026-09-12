package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"hostsent/backend/internal/modules/admin/product/catalog/model"
	"hostsent/backend/internal/pkg/billingcycle"
)

// ============================================================================
// T3.4 上游调价落地（PriceApplier 契约）
//
// 上游成本价变化经人工确认后，由同步框架调用本方法把新成本写入关联的
// 售出商品（克隆/转售商品），并写 product_history 留痕。
//
// 关于售价（重要）：本项目禁止静默改售价，也禁止在同步链路里另写一套算价。
// 因此：
//   · 商品配置了 upstream_markup_type/value（上游加价规则，029 已建列）时，
//     按该规则重算售价；
//   · 未配置加价规则时，只更新成本价、保持售价不变，仅在 history 备注里
//     提示"成本变动需人工调价"，避免自动改价影响已售订单预期。
// ============================================================================

// ApplyConfirmedPrice 把已确认的上游成本价写入绑定该上游资源商品的售出商品。
// 返回受影响的商品数。operatorName/remark 用于 product_history 审计。
//
// 周期口径（doc25 §5）：若上游资源商品的 raw_specs.product_pricings 带各周期成本，
// 则逐周期按加价规则重算并写入周期价格矩阵（product_prices）；无该结构时只按传入
// 的 costPrice 处理单档（月度）与旧列，保持既有行为。
func (s *productService) ApplyConfirmedPrice(ctx context.Context, resourceProductID uint64, costPrice float64, operatorName, remark string) (int, error) {
	products, err := s.repo.ListBySourceProductID(ctx, resourceProductID)
	if err != nil {
		return 0, err
	}
	if len(products) == 0 {
		return 0, nil
	}
	// 上游各周期成本（可能为空：该上游商品未带 product_pricings 结构）。
	cycleCosts := s.upstreamCycleCosts(ctx, resourceProductID)
	if operatorName == "" {
		operatorName = "系统（同步确认）"
	}
	affected := 0
	for i := range products {
		item := products[i]
		oldCost := item.CostPrice
		newPrice := item.Price
		priceChanged := false
		if p, changed := applyMarkup(item.UpstreamMarkupType, item.UpstreamMarkupValue, costPrice); changed {
			newPrice = round2(p)
			priceChanged = newPrice != item.Price
		}
		item.CostPrice = round2(costPrice)
		item.Price = newPrice
		if err := s.repo.Update(ctx, &item); err != nil {
			return affected, err
		}
		// 周期矩阵同步：把上游各周期成本按同一加价规则推导为售价行。
		if s.cycleWriter != nil && len(cycleCosts) > 0 {
			rows := make([]CyclePriceRow, 0, len(cycleCosts))
			for cycle, cost := range cycleCosts {
				rows = append(rows, buildCyclePriceRow(item, cycle, cost))
			}
			if err := s.cycleWriter.ApplyUpstreamPrices(ctx, item.ID, rows); err != nil {
				return affected, err
			}
		}
		historyRemark := strings.TrimSpace(remark)
		if historyRemark == "" {
			switch {
			case priceChanged && len(cycleCosts) > 0:
				historyRemark = fmt.Sprintf("上游成本价变更，按加价规则重算售价并同步 %d 档周期价格（%s）", len(cycleCosts), item.UpstreamMarkupType)
			case priceChanged:
				historyRemark = fmt.Sprintf("上游成本价变更，按加价规则重算售价（%s）", item.UpstreamMarkupType)
			default:
				historyRemark = "上游成本价变更已确认，售价保持不变（未配置加价规则）"
			}
		}
		_ = s.repo.AddHistory(ctx, &model.ProductHistory{
			ProductID:    item.ID,
			ChangeType:   model.ChangeTypePrice,
			OldValue:     strconv.FormatFloat(oldCost, 'f', 2, 64),
			NewValue:     strconv.FormatFloat(item.CostPrice, 'f', 2, 64),
			OperatorName: operatorName,
			Remark:       historyRemark,
		})
		affected++
	}
	return affected, nil
}

// upstreamRawPricings 上游商品 raw_specs 中的 product_pricings 单元素结构。
// 上游按周期给价（月付/季付/年付/两年/三年/一次性），字段可能为字符串或数字。
type upstreamRawPricings struct {
	Monthly      flexNumber `json:"monthly"`
	Quarterly    flexNumber `json:"quarterly"`
	Semiannually flexNumber `json:"semiannually"`
	Annually     flexNumber `json:"annually"`
	Biennially   flexNumber `json:"biennially"`
	Triennially  flexNumber `json:"triennially"`
	Onetime      flexNumber `json:"onetime"`
	MSetupFee    flexNumber `json:"msetupfee"`
}

// flexNumber 兼容上游把数字回成字符串（"20"）或数字（20）两种编码。
type flexNumber float64

// UnmarshalJSON 数字/字符串/空值一律解析为 float64；非法值按 0 处理。
func (f *flexNumber) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		*f = 0
		return nil
	}
	*f = flexNumber(v)
	return nil
}

// upstreamCycleCosts 读取上游资源商品的各周期成本。
// 上游以 -1 或 0 表示"该周期不提供"，此类档位不落矩阵（避免出现 0 元可售档）。
func (s *productService) upstreamCycleCosts(ctx context.Context, resourceProductID uint64) map[string]float64 {
	if s.resourceReader == nil || resourceProductID == 0 {
		return nil
	}
	rp, err := s.resourceReader.FindByID(ctx, resourceProductID)
	if err != nil || rp == nil {
		return nil
	}
	return parseUpstreamCycleCosts(rp.RawSpecs)
}

// parseUpstreamCycleCosts 从上游 raw_specs 解析各周期正成本（<=0 视为该周期不提供）。
func parseUpstreamCycleCosts(rawSpecs string) map[string]float64 {
	if strings.TrimSpace(rawSpecs) == "" {
		return nil
	}
	var raw struct {
		ProductPricings []upstreamRawPricings `json:"product_pricings"`
	}
	if err := json.Unmarshal([]byte(rawSpecs), &raw); err != nil || len(raw.ProductPricings) == 0 {
		return nil
	}
	p := raw.ProductPricings[0]
	candidates := map[string]float64{
		billingcycle.Monthly:      float64(p.Monthly),
		billingcycle.Quarterly:    float64(p.Quarterly),
		billingcycle.Semiannually: float64(p.Semiannually),
		billingcycle.Annually:     float64(p.Annually),
		billingcycle.Biennially:   float64(p.Biennially),
		billingcycle.Triennially:  float64(p.Triennially),
		billingcycle.Onetime:      float64(p.Onetime),
	}
	costs := make(map[string]float64, len(candidates))
	for cycle, cost := range candidates {
		if cost > 0 {
			costs[cycle] = cost
		}
	}
	return costs
}

// buildCyclePriceRow 按商品加价规则把某周期成本推导为矩阵行。
// percent → cost×v/100；fixed → cost+v（固定额只加一次，不随周期放大）；
// 未配置加价规则 → 直接以上游成本为售价（source=upstream，运营可再人工调整）。
func buildCyclePriceRow(item model.Product, cycle string, cost float64) CyclePriceRow {
	row := CyclePriceRow{
		Cycle:     cycle,
		CostPrice: round2(cost),
		Price:     round2(cost),
		Source:    "upstream",
	}
	if p, changed := applyMarkup(item.UpstreamMarkupType, item.UpstreamMarkupValue, cost); changed {
		row.Price = round2(p)
		row.Source = "markup"
	}
	return row
}

// applyMarkup 按商品的上游加价规则计算售价；未配置规则时返回 (0,false)。
// type: percent=成本×value%；fixed=成本+value。
func applyMarkup(markupType string, markupValue, cost float64) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(markupType)) {
	case model.MarkupTypePercent:
		return cost * markupValue / 100, true
	case model.MarkupTypeFixed:
		return cost + markupValue, true
	default:
		return 0, false
	}
}

// seedCloneCyclePrices 克隆上游商品时按其 product_pricings 生成初始周期价格矩阵（doc25 §5）。
// ratio 为"售价/成本"倍率（运营克隆时定的毛利口径），<=0 时按 1 处理（原价转售）。
// 幂等性由调用时机保证（仅新建商品后调用一次）。
func (s *productService) seedCloneCyclePrices(ctx context.Context, item model.Product, rawSpecs string, ratio float64) {
	if s.cycleWriter == nil || item.ID == 0 {
		return
	}
	costs := parseUpstreamCycleCosts(rawSpecs)
	if len(costs) == 0 {
		return
	}
	if ratio <= 0 {
		ratio = 1
	}
	rows := make([]CyclePriceRow, 0, len(costs))
	for cycle, cost := range costs {
		// 克隆口径是按倍率定价，不再叠加加价规则（避免与倍率双重计价）。
		rows = append(rows, CyclePriceRow{
			Cycle:     cycle,
			Price:     round2(cost * ratio),
			CostPrice: round2(cost),
			Source:    "upstream",
		})
	}
	_ = s.cycleWriter.ApplyUpstreamPrices(ctx, item.ID, rows)
}
