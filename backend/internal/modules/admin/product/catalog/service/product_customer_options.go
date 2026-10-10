package service

// 客户可选配置项的读取与计价（T4.5）。
//
// 数据来源：product_config_options / _sub（source=self），由建品时按配置档生成，
// 或运营在商品详情页手工维护。这层只做"读 + 校验 + 计价"，不写库：
//   - CustomerOptions：给用户侧渲染控件（控件类型、必选、默认、取值、加价）；
//   - ResolveOptionPricing：给算价管线（选配加价小计）与履约（归一后的 selections）。

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	catalogmodel "hostsent/backend/internal/modules/admin/product/catalog/model"
	"hostsent/backend/internal/pkg/billingcycle"
)

// 客户配置项控件类型（与 catalog model.ConfigWidget* 同口径）。
const (
	optionWidgetSelect = catalogmodel.ConfigWidgetSelect
	optionWidgetRadio  = catalogmodel.ConfigWidgetRadio
	optionWidgetQty    = catalogmodel.ConfigWidgetQty
	optionWidgetBool   = catalogmodel.ConfigWidgetBool
	// optionWidgetGroupSelect 分组下拉（按取值自带的 group_label 分组，如操作系统家族）。
	optionWidgetGroupSelect = "group_select"
)

// customerOptionKey 取配置项的平台参数名：优先 option_key，其次 source_key，回退展示名。
func customerOptionKey(row *catalogmodel.ProductConfigOption) string {
	return firstNonEmptyStr(row.OptionKey, row.SourceKey, row.OptionName)
}

// customerSubValue 取子项的下发取值：优先 source_key，回退展示名。
func customerSubValue(sub catalogmodel.ProductConfigOptionSub) string {
	return firstNonEmptyStr(sub.SourceKey, sub.OptionName)
}

// customerWidget 归一控件类型：运营没显式配时按 option_type 与是否有分组推断。
func customerWidget(row *catalogmodel.ProductConfigOption) string {
	if strings.TrimSpace(row.Widget) != "" {
		return row.Widget
	}
	if row.MinValue != nil || row.MaxValue != nil {
		return optionWidgetQty
	}
	switch row.OptionType {
	case 2:
		return optionWidgetRadio
	case 3:
		return optionWidgetBool
	case 4:
		return optionWidgetQty
	}
	return optionWidgetSelect
}

// CustomerOptions 返回商品对客户开放的选配项（只含自营项）。
func (s *productService) CustomerOptions(ctx context.Context, productID uint64) ([]CustomerOptionGroup, error) {
	rows, err := s.repo.SelfConfigOptionRows(ctx, productID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	options := make([]CustomerOption, 0, len(rows))
	for _, row := range rows {
		key := customerOptionKey(row)
		if key == "" {
			continue
		}
		widget := customerWidget(row)
		opt := CustomerOption{
			OptionKey:    key,
			Name:         firstNonEmptyStr(row.OptionName, key),
			Widget:       widget,
			Required:     row.Required,
			DefaultValue: row.DefaultValue,
			Unit:         row.Unit,
			Help:         row.Help,
			MinValue:     row.MinValue,
			MaxValue:     row.MaxValue,
		}
		if widget == optionWidgetQty {
			opt.UnitPrice = quantityUnitPrice(row)
		}
		grouped := false
		for _, sub := range row.Subs {
			if sub.Hidden != 0 {
				continue // 停用取值不给客户选
			}
			value := customerSubValue(sub)
			if value == "" {
				continue
			}
			if strings.TrimSpace(sub.GroupLabel) != "" {
				grouped = true
			}
			opt.Values = append(opt.Values, CustomerOptionItem{
				Value:          value,
				Label:          firstNonEmptyStr(sub.OptionName, value),
				Group:          sub.GroupLabel,
				Default:        sub.IsDefault,
				PriceMonthly:   sub.PriceMonthly,
				PriceQuarterly: sub.PriceQuarterly,
				PriceAnnually:  sub.PriceAnnually,
				PriceOnetime:   sub.PriceOnetime,
			})
		}
		// 有分组标签时升级为分组下拉（操作系统按 Ubuntu/Windows 分组）。
		if grouped && widget == optionWidgetSelect {
			opt.Widget = optionWidgetGroupSelect
		}
		// 枚举型没有任何可见取值时跳过（客户没得选）；数量型即便无取值也要下发。
		if len(opt.Values) == 0 && opt.Widget != optionWidgetQty {
			continue
		}
		options = append(options, opt)
	}
	if len(options) == 0 {
		return nil, nil
	}
	return []CustomerOptionGroup{{Options: options}}, nil
}

// ResolveOptionPricing 按客户选择算加价并归一选择（T4.5）。
//
// 归一规则（结果用于算价与履约，必须与下单口径一致）：
//   - 未选的**必选**项：用默认取值补齐（IsDefault 优先，其次首个可见项）；
//   - 未选的非必选项：不参与加价，也不下发（平台用自己的默认）；
//   - 传入的取值不在候选内：报错（客户选了运营已下架的取值）；
//   - 数量型选项：值必须是 [min, max] 内的数字；子项单价即"每单位加价"，
//     以**默认值**为档位内含量，超出默认值的部分才加价（默认值缺省时用最小值）。
//     这样"客户不改配置就是档位价、改大才加价"，与档位定价的直觉一致。
func (s *productService) ResolveOptionPricing(ctx context.Context, productID uint64, cycle string, selections map[string]string) (*OptionPricing, error) {
	rows, err := s.repo.SelfConfigOptionRows(ctx, productID)
	if err != nil {
		return nil, err
	}
	result := &OptionPricing{Selections: map[string]string{}, Items: []OptionPricingItem{}}
	if len(rows) == 0 {
		// 商品没有选配项：原样回传选择（便于上层统一落库），不做校验。
		for k, v := range selections {
			result.Selections[k] = v
		}
		return result, nil
	}
	cycleCode := billingcycle.Normalize(cycle)

	for _, row := range rows {
		key := customerOptionKey(row)
		if key == "" {
			continue
		}
		widget := customerWidget(row)
		chosen := strings.TrimSpace(selections[key])

		// 数量型：校验区间并按超量部分计加价。
		if widget == optionWidgetQty {
			if chosen == "" {
				chosen = firstNonEmptyStr(row.DefaultValue, stringOfFloat(row.MinValue))
			}
			if chosen == "" {
				continue
			}
			qty, perr := strconv.ParseFloat(chosen, 64)
			if perr != nil {
				return nil, fmt.Errorf("配置项「%s」的取值必须是数字", row.OptionName)
			}
			if row.MinValue != nil && qty < *row.MinValue {
				return nil, fmt.Errorf("配置项「%s」不能小于 %v", row.OptionName, *row.MinValue)
			}
			if row.MaxValue != nil && qty > *row.MaxValue {
				return nil, fmt.Errorf("配置项「%s」不能大于 %v", row.OptionName, *row.MaxValue)
			}
			result.Selections[key] = chosen
			if unit := quantityUnitPrice(row); unit > 0 {
				// 档位内含量 = 默认值（缺省回落最小值）：客户按默认配置购买时加价为 0。
				base := 0.0
				if row.DefaultValue != "" {
					if parsed, derr := strconv.ParseFloat(row.DefaultValue, 64); derr == nil {
						base = parsed
					}
				}
				if base == 0 && row.MinValue != nil {
					base = *row.MinValue
				}
				billable := qty - base
				if billable > 0 {
					amount := round2(billable * unit)
					result.Amount = round2(result.Amount + amount)
					result.Items = append(result.Items, OptionPricingItem{
						OptionKey:  key,
						OptionName: row.OptionName,
						Value:      chosen,
						ValueName:  chosen + strings.TrimSpace(row.Unit),
						Quantity:   billable,
						UnitPrice:  unit,
						Amount:     amount,
					})
				}
			}
			continue
		}

		// 枚举型：校验取值存在，并取该取值的周期加价。
		value, sub, verr := pickEnumValue(row, chosen)
		if verr != nil {
			return nil, verr
		}
		if value == "" {
			continue // 非必选且客户没选：不下发
		}
		result.Selections[key] = value
		price := priceForCycle(sub, cycleCode)
		if price > 0 {
			result.Amount = round2(result.Amount + price)
			result.Items = append(result.Items, OptionPricingItem{
				OptionKey:   key,
				OptionName:  row.OptionName,
				Value:       value,
				ValueName:   firstNonEmptyStr(sub.OptionName, value),
				Quantity:    1,
				UnitPrice:   price,
				Amount:      price,
				IsDefaulted: strings.TrimSpace(selections[key]) == "",
			})
		}
	}
	return result, nil
}

// pickEnumValue 从配置项子项里挑选客户选中的取值。
// 客户未选时：必选项取默认（默认值/IsDefault 优先，其次首个可见项）；非必选项返回空。
func pickEnumValue(row *catalogmodel.ProductConfigOption, chosen string) (string, catalogmodel.ProductConfigOptionSub, error) {
	visible := make([]catalogmodel.ProductConfigOptionSub, 0, len(row.Subs))
	for _, sub := range row.Subs {
		if sub.Hidden != 0 {
			continue
		}
		visible = append(visible, sub)
	}
	if chosen != "" {
		for _, sub := range visible {
			if customerSubValue(sub) == chosen {
				return chosen, sub, nil
			}
		}
		return "", catalogmodel.ProductConfigOptionSub{},
			fmt.Errorf("配置项「%s」的取值「%s」不可用，请重新选择", row.OptionName, chosen)
	}
	if !row.Required {
		return "", catalogmodel.ProductConfigOptionSub{}, nil
	}
	if row.DefaultValue != "" {
		for _, sub := range visible {
			if customerSubValue(sub) == row.DefaultValue {
				return row.DefaultValue, sub, nil
			}
		}
	}
	for _, sub := range visible {
		if sub.IsDefault {
			return customerSubValue(sub), sub, nil
		}
	}
	if len(visible) > 0 {
		return customerSubValue(visible[0]), visible[0], nil
	}
	return "", catalogmodel.ProductConfigOptionSub{}, nil
}

// priceForCycle 取某取值在给定周期的加价（周期为空按月度）。
func priceForCycle(sub catalogmodel.ProductConfigOptionSub, cycle string) float64 {
	switch cycle {
	case billingcycle.Quarterly:
		if sub.PriceQuarterly > 0 {
			return sub.PriceQuarterly
		}
		return sub.PriceMonthly
	case billingcycle.Annually:
		if sub.PriceAnnually > 0 {
			return sub.PriceAnnually
		}
		return sub.PriceMonthly
	case billingcycle.Onetime:
		if sub.PriceOnetime > 0 {
			return sub.PriceOnetime
		}
		return sub.PriceMonthly
	default:
		return sub.PriceMonthly
	}
}

// quantityUnitPrice 数量型选项的"每单位加价"：取首个可见子项的月度价（运营约定）。
func quantityUnitPrice(row *catalogmodel.ProductConfigOption) float64 {
	for _, sub := range row.Subs {
		if sub.Hidden != 0 {
			continue
		}
		if sub.PriceMonthly > 0 {
			return sub.PriceMonthly
		}
	}
	return 0
}

func stringOfFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
