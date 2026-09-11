package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"hostsent/backend/internal/modules/admin/product/catalog/model"
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
func (s *productService) ApplyConfirmedPrice(ctx context.Context, resourceProductID uint64, costPrice float64, operatorName, remark string) (int, error) {
	products, err := s.repo.ListBySourceProductID(ctx, resourceProductID)
	if err != nil {
		return 0, err
	}
	if len(products) == 0 {
		return 0, nil
	}
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
		historyRemark := strings.TrimSpace(remark)
		if historyRemark == "" {
			if priceChanged {
				historyRemark = fmt.Sprintf("上游成本价变更，按加价规则重算售价（%s）", item.UpstreamMarkupType)
			} else {
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
