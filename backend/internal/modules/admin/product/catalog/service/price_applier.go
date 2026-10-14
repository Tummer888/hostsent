package service

import (
	"context"
	"encoding/json"
	"errors"
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
	// 上游各周期成本与标价（可能为空：该上游商品未带 product_pricings 结构）。
	cyclePrices := s.upstreamCyclePrices(ctx, resourceProductID)
	cycleCosts := cyclePrices.costs
	if operatorName == "" {
		operatorName = "系统（同步确认）"
	}
	affected := 0
	for i := range products {
		item := products[i]
		oldCost := item.CostPrice
		newPrice := item.Price
		priceChanged := false
		// 按标价口径（list_percent）时基数取上游标价，其余口径基数仍是成本（代理价）。
		if p, changed := applyMarkupWithList(item.UpstreamMarkupType, item.UpstreamMarkupValue, costPrice, cyclePrices.listHeadline()); changed {
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
				rows = append(rows, buildCyclePriceRow(item, cycle, cost, cyclePrices.lists[cycle]))
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

// ApplyUpstreamMarkupRule 设定上游商品的对客加价规则，并按该规则重算其已导入商品的售价。
//
// 场景（「上游商品」页调价）：运营看到某上游商品的标价/代理价后，决定"按上游标价 × 120% 卖"，
// 该动作要同时落到已导入的销售商品（售价 + 周期价格矩阵），而不是只改镜像价（下次同步会被覆盖）。
// 实现上先把规则写进绑定商品，再复用 ApplyConfirmedPrice 的算价链路（单一算价入口，不另写一套）。
func (s *productService) ApplyUpstreamMarkupRule(ctx context.Context, resourceProductID uint64, markupType string, markupValue float64, operatorName, remark string) (int, error) {
	if resourceProductID == 0 {
		return 0, errors.New("缺少上游商品 ID")
	}
	if !IsValidMarkupType(markupType) {
		return 0, fmt.Errorf("不支持的加价方式 %q（可选 percent / fixed / list_percent）", markupType)
	}
	if markupType != "" && markupValue < 0 {
		return 0, errors.New("加价数值不能为负")
	}
	products, err := s.repo.ListBySourceProductID(ctx, resourceProductID)
	if err != nil {
		return 0, err
	}
	if len(products) == 0 {
		return 0, nil
	}
	oldPrices := make(map[uint64]float64, len(products))
	for i := range products {
		item := products[i]
		oldPrices[item.ID] = item.Price
		item.UpstreamMarkupType = markupType
		item.UpstreamMarkupValue = markupValue
		if err := s.repo.Update(ctx, &item); err != nil {
			return 0, err
		}
	}
	// 规则已落库，再按当前上游价格重算售价（成本取资源商品的代理价）。
	cost := 0.0
	if s.resourceReader != nil {
		if rp, rerr := s.resourceReader.FindByID(ctx, resourceProductID); rerr == nil && rp != nil {
			if rp.CostPrice > 0 {
				cost = rp.CostPrice
			} else {
				cost = rp.SalePrice
			}
		}
	}
	ruleRemark := strings.TrimSpace(remark)
	if ruleRemark == "" {
		if markupType == "" {
			ruleRemark = "清除上游加价规则（售价保持当前值）"
		} else {
			ruleRemark = fmt.Sprintf("设定上游加价规则：%s %.2f", markupType, markupValue)
		}
	}
	affected, err := s.ApplyConfirmedPrice(ctx, resourceProductID, cost, operatorName, ruleRemark)
	if err != nil {
		return affected, err
	}
	// 补一条"售价变更"历史：ApplyConfirmedPrice 记的是成本口径，调价前后的成本不变，
	// 只留成本行会看不出这次调价到底把售价改成了多少。
	if latest, lerr := s.repo.ListBySourceProductID(ctx, resourceProductID); lerr == nil {
		for i := range latest {
			item := latest[i]
			old, ok := oldPrices[item.ID]
			if !ok || round2(old) == round2(item.Price) {
				continue
			}
			_ = s.repo.AddHistory(ctx, &model.ProductHistory{
				ProductID:    item.ID,
				ChangeType:   model.ChangeTypePrice,
				OldValue:     strconv.FormatFloat(round2(old), 'f', 2, 64),
				NewValue:     strconv.FormatFloat(round2(item.Price), 'f', 2, 64),
				OperatorName: operatorName,
				Remark:       ruleRemark + "（售价变更）",
			})
		}
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

// upstreamCyclePrices 上游资源商品的各周期成本（代理价）与标价。
// 上游以 -1 或 0 表示"该周期不提供"，此类档位不落矩阵（避免出现 0 元可售档）。
type upstreamCyclePrices struct {
	costs map[string]float64 // 周期 → 代理价（我方采购成本）
	lists map[string]float64 // 周期 → 上游标价（list_percent 加价规则的基数）
}

// listHeadline 代表标价（月度优先，与 upstreamPrice 同序）；无标价时返回 0。
func (u upstreamCyclePrices) listHeadline() float64 {
	for _, cycle := range []string{billingcycle.Monthly, billingcycle.Quarterly, billingcycle.Semiannually, billingcycle.Annually, billingcycle.Onetime} {
		if v := u.lists[cycle]; v > 0 {
			return v
		}
	}
	return 0
}

// upstreamCyclePrices 读取上游资源商品的各周期成本与标价；读取不到返回空结构。
func (s *productService) upstreamCyclePrices(ctx context.Context, resourceProductID uint64) upstreamCyclePrices {
	if s.resourceReader == nil || resourceProductID == 0 {
		return upstreamCyclePrices{}
	}
	rp, err := s.resourceReader.FindByID(ctx, resourceProductID)
	if err != nil || rp == nil {
		return upstreamCyclePrices{}
	}
	return parseUpstreamCyclePrices(rp.RawSpecs)
}

// upstreamRawListPricings 上游标价行（raw_specs.product_pricings_list，未打折扣）。
type upstreamRawListPricings struct {
	Monthly      flexNumber `json:"monthly"`
	Quarterly    flexNumber `json:"quarterly"`
	Semiannually flexNumber `json:"semiannually"`
	Annually     flexNumber `json:"annually"`
	Biennially   flexNumber `json:"biennially"`
	Triennially  flexNumber `json:"triennially"`
	Onetime      flexNumber `json:"onetime"`
}

// parseUpstreamCyclePrices 从上游 raw_specs 解析各周期正成本（product_pricings）
// 与标价（product_pricings_list，缺失时该项留空）。
func parseUpstreamCyclePrices(rawSpecs string) upstreamCyclePrices {
	out := upstreamCyclePrices{}
	if strings.TrimSpace(rawSpecs) == "" {
		return out
	}
	var raw struct {
		ProductPricings     []upstreamRawPricings     `json:"product_pricings"`
		ProductPricingsList []upstreamRawListPricings `json:"product_pricings_list"`
	}
	if err := json.Unmarshal([]byte(rawSpecs), &raw); err != nil || len(raw.ProductPricings) == 0 {
		return out
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
	out.costs = make(map[string]float64, len(candidates))
	for cycle, cost := range candidates {
		if cost > 0 {
			out.costs[cycle] = cost
		}
	}
	if len(raw.ProductPricingsList) > 0 {
		l := raw.ProductPricingsList[0]
		listCandidates := map[string]float64{
			billingcycle.Monthly:      float64(l.Monthly),
			billingcycle.Quarterly:    float64(l.Quarterly),
			billingcycle.Semiannually: float64(l.Semiannually),
			billingcycle.Annually:     float64(l.Annually),
			billingcycle.Biennially:   float64(l.Biennially),
			billingcycle.Triennially:  float64(l.Triennially),
			billingcycle.Onetime:      float64(l.Onetime),
		}
		out.lists = make(map[string]float64, len(listCandidates))
		for cycle, price := range listCandidates {
			if price > 0 {
				out.lists[cycle] = price
			}
		}
	}
	return out
}

// parseUpstreamCycleCosts 只取周期成本（兼容旧调用方与测试）。
func parseUpstreamCycleCosts(rawSpecs string) map[string]float64 {
	return parseUpstreamCyclePrices(rawSpecs).costs
}

// buildCyclePriceRow 按商品加价规则把某周期的成本/标价推导为矩阵行。
// percent → cost×v/100；list_percent → 标价×v/100；fixed → cost+v（固定额只加一次，不随周期放大）；
// 未配置加价规则 → 只带成本、标 PreservePrice 让写入方保留现有售价（见下）。
//
// 关于"未配置加价规则"（实测故障，doc120 §11.7）：这里**不能**拿成本当售价写进去。
// 本文件顶部的契约明确写着"未配置加价规则时只更新成本价、保持售价不变"，单个商品价
// 那一侧是这么做的，但周期矩阵这一侧原先直接写 Price=cost，于是运营每确认一次上游
// 成本变更，各周期售价就被改成采购价 —— 变成零毛利白卖（实测 5 个商品、20 档被改写；
// 且周期价在算价里优先级高于商品价，客户真按成本价下单）。
// 售价来源可能是导入倍率，也可能是运营手工定的，同步链路无权按成本重写，故改为保留。
func buildCyclePriceRow(item model.Product, cycle string, cost, list float64) CyclePriceRow {
	row := CyclePriceRow{
		Cycle:         cycle,
		CostPrice:     round2(cost),
		Source:        "upstream",
		PreservePrice: true,
	}
	if p, changed := applyMarkupWithList(item.UpstreamMarkupType, item.UpstreamMarkupValue, cost, list); changed {
		row.Price = round2(p)
		row.Source = "markup"
		row.PreservePrice = false
	}
	return row
}

// applyMarkupWithList 按加价规则计算售价，list 为该周期上游标价（list_percent 口径的基数）。
// type: percent=成本×value%；fixed=成本+value；list_percent=上游标价×value%。
func applyMarkupWithList(markupType string, markupValue, cost, list float64) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(markupType)) {
	case model.MarkupTypePercent:
		return cost * markupValue / 100, true
	case model.MarkupTypeFixed:
		return cost + markupValue, true
	case model.MarkupTypeListPercent:
		base := list
		if base <= 0 {
			base = cost
		}
		return base * markupValue / 100, true
	default:
		return 0, false
	}
}

// IsValidMarkupType 校验加价规则类型（空串=清除规则，合法）。
func IsValidMarkupType(markupType string) bool {
	switch strings.ToLower(strings.TrimSpace(markupType)) {
	case "", model.MarkupTypePercent, model.MarkupTypeFixed, model.MarkupTypeListPercent:
		return true
	default:
		return false
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

// ============================================================================
// 上游库存同步（StockApplier 契约）
// ============================================================================

// ApplyUpstreamStock 把上游商品的可售库存写进绑定它的售出商品（下单门禁读的就是它）。
//
// 背景（用户反馈）：上游商品列表的库存信息不准、也没从上游拉下来；上游限量
// （stock_control=1）的商品在本地可以无限下单。库存的权威在上游，本地只是镜像，
// 因此这里跟随目录同步刷新。
//
// 覆盖规则（两档，避免把"上游放开"与"运营自设量"搞混）：
//   - 上游限量（stock >= 0）：以上游余量为准，直接写。上游余量是真实可售上限，
//     本地留着更大的数就是超卖风险；此前的导入也从不带库存，需要靠这里收敛。
//   - 上游不限量（stock < 0）：仅当本地值**来自上一次同步**（prevStock >= 0，
//     即上游刚解除限量）时放开为 -1；若本地本来就是 -1，说明当前值可能是运营
//     自己设的限量，保持不动 —— 否则"只卖 10 台"会被上游的"不限量"抹掉。
func (s *productService) ApplyUpstreamStock(ctx context.Context, resourceProductID uint64, stock, prevStock int) (int, error) {
	if resourceProductID == 0 {
		return 0, nil
	}
	if stock < 0 && prevStock < 0 {
		// 上游一直不限量、本地也是不限量：没有需要同步的事实，不写库。
		return 0, nil
	}
	products, err := s.repo.ListBySourceProductID(ctx, resourceProductID)
	if err != nil {
		return 0, err
	}
	affected := 0
	for i := range products {
		item := products[i]
		if item.Stock == stock {
			continue
		}
		item.Stock = stock
		if err := s.repo.Update(ctx, &item); err != nil {
			return affected, err
		}
		affected++
	}
	return affected, nil
}
