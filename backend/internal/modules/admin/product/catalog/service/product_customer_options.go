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
	// optionWidgetText 文本输入（对接商品的开通选项，如主机名）：无候选取值，
	// 客户填什么就下发什么，不参与加价。
	optionWidgetText = catalogmodel.ConfigWidgetText
	// optionWidgetGroupSelect 分组下拉（按取值自带的 group_label 分组，如操作系统家族）。
	optionWidgetGroupSelect = "group_select"
)

// customerOptionKey 取配置项的选配键。
//
// 自营项（source=self）：平台参数名（option_key/source_key），下发给平台时即该键。
// 上游项（source=upstream）：上游配置项**自身 id**（product_config_options.upstream_key）。
// 该链路的下发格式是 configoption[上游选项id] = 上游取值id（见 zjmf buildCartConfigOption），
// 因此键只能用上游 id；用展示名（"os|操作系统"）既下不去也校验不了。
func customerOptionKey(row *catalogmodel.ProductConfigOption) string {
	if row.Source == catalogmodel.ConfigSourceUpstream && row.UpstreamKey > 0 {
		return strconv.FormatInt(row.UpstreamKey, 10)
	}
	return firstNonEmptyStr(row.OptionKey, row.SourceKey, row.OptionName)
}

// customerSubValue 取子项的下发取值：上游项用上游取值 id（上游 cart 按 id 匹配取值），
// 自营项优先 source_key（平台参数取值），都没有才回落展示名。
func customerSubValue(sub catalogmodel.ProductConfigOptionSub, source string) string {
	if source == catalogmodel.ConfigSourceUpstream && sub.UpstreamKey > 0 {
		return strconv.FormatInt(sub.UpstreamKey, 10)
	}
	return firstNonEmptyStr(sub.SourceKey, sub.OptionName)
}

// customerSelectableUpstream 判断上游配置项是否对客户可选。
// 上游配置项里只有一个可见取值的属于「固定配置」（锁定项，改不了也开不出来），
// 由商品详情只读展示，不作为选配项；有多个可见取值的才允许客户在下单时选择。
//
// 例外：数量型配置项（系统盘/数据盘/带宽/IP…）按定义就是"客户填一个数值"，
// 上游按数量收值、按区间计价。这类项即使只有一个价档（如数据盘只有一个
// 0~1024 的 0.8 元/GB 档）也必须开放给客户 —— 否则客户完全无法指定盘的大小，
// 只能按默认值开通，等于商品卖不出它该有的形态。
func customerSelectableUpstream(row *catalogmodel.ProductConfigOption) bool {
	if isQuantityOption(row.OptionType) {
		return true
	}
	visible := 0
	for _, sub := range row.Subs {
		if sub.Hidden != 0 {
			continue
		}
		visible++
		if visible > 1 {
			return true
		}
	}
	return false
}

// isQuantityOption 是否为数量型配置项（客户填数值，而不是从候选值里挑一个）。
//
// 口径对齐上游 common.php::judgeQuantity（4/7/9/11/14/15/16/17/18/19）。
// 这类项上游收的是**数量本身**，不是子项 id —— 详见 upstreamConfigOptionOverride。
func isQuantityOption(optionType int) bool {
	switch optionType {
	case 4, 7, 9, 11, 14, 15, 16, 17, 18, 19:
		return true
	}
	return false
}

// customerWidget 归一控件类型：运营没显式配时按 option_type 与是否有分组推断。
func customerWidget(row *catalogmodel.ProductConfigOption) string {
	if strings.TrimSpace(row.Widget) != "" {
		return row.Widget
	}
	if row.MinValue != nil || row.MaxValue != nil {
		return optionWidgetQty
	}
	if isQuantityOption(row.OptionType) {
		return optionWidgetQty
	}
	switch row.OptionType {
	case 2:
		return optionWidgetRadio
	case 3:
		return optionWidgetBool
	}
	return optionWidgetSelect
}

// CustomerOptions 返回商品对客户开放的选配项。
//
// 两类都下发（T4.5 扩展）：
//   - source=self 自营项：平台参数名即下发键，全部下发（含主机名这类自由填写项）；
//   - source=upstream 对接项：仅下发**多可见取值**的项（操作系统/网络类型），
//     单值的固定配置属锁定项，由商品详情只读展示，不在此重复。
func (s *productService) CustomerOptions(ctx context.Context, productID uint64) ([]CustomerOptionGroup, error) {
	rows, err := s.repo.AllConfigOptionRows(ctx, productID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	// 平台目录的「可自由填写」标记：manual 来源的项平台只给了示例值，
	// 但允许填其它值（port 只声明 auto，文档允许填具体端口号）。
	// 目录按平台类型读一次、全商品共用；读不到就不放开（宁可不放开，也不能让客户乱填）。
	creatableKeys := s.creatableOptionKeys(ctx, rows)
	// 已作为 SKU 维度的参数不再开放给客户（T4.8）：它的取值由所选规格决定，
	// 客户再选一次会覆盖 SKU 的 pin —— 与自营链路同一条规则
	// （自营侧见 collectOptionAccumulatorsFiltered）。
	axisKeys := s.axisKeysForProduct(ctx, productID)

	options := make([]CustomerOption, 0, len(rows))
	for _, row := range rows {
		if row.Source == catalogmodel.ConfigSourceUpstream && !customerSelectableUpstream(row) {
			continue
		}
		if axisKeys[upstreamOptionKey(row.OptionName)] {
			continue
		}
		key := customerOptionKey(row)
		if key == "" {
			continue
		}
		widget := customerWidget(row)
		name := firstNonEmptyStr(row.OptionName, key)
		// 「值|展示名」是平台通用写法（上游 configoption 与平台目录取值库都用它），
		// 与来源无关：自营商品从平台目录带过来的取值同样是「1|CN^默认」这种原始串。
		// 早先只在 source=upstream 时解析，自营项于是把上游原文直接显示给客户
		// （实测：数据中心下拉里出现「1|CN^默认」）。
		name = upstreamOptionLabel(name, key)
		opt := CustomerOption{
			OptionKey:    key,
			Name:         name,
			Widget:       widget,
			Creatable:    creatableKeys[row.ProviderType+"\x00"+key],
			Required:     row.Required,
			DefaultValue: row.DefaultValue,
			Unit:         row.Unit,
			Help:         row.Help,
			MinValue:     row.MinValue,
			MaxValue:     row.MaxValue,
		}
		if widget == optionWidgetQty {
			// 数量型没有可选取值，但要给前端一个可读的默认文案（不少商品把默认值
			// 声明成「不限量」这类语义，数字本身看不出含义）。
			// 取值列表清空：占位 sub 不是"可选项"，渲染成下拉会误导客户。
			opt.Values = nil
			// 四档单位加价都下发：前端按当前所选周期挑对应的那档估算，
			// 只给月度值会让年付订单的预览价远低于实付（用户反馈过的"加配不按年叠加"）。
			opt.UnitPrice = quantityUnitPriceForCycle(row, billingcycle.Monthly)
			opt.UnitPriceQuarterly = quantityUnitPriceForCycle(row, billingcycle.Quarterly)
			opt.UnitPriceAnnually = quantityUnitPriceForCycle(row, billingcycle.Annually)
			opt.UnitPriceOnetime = quantityUnitPriceForCycle(row, billingcycle.Onetime)
		}
		grouped := false
		rawNames := make([]string, 0, len(row.Subs))
		for _, sub := range row.Subs {
			if sub.Hidden != 0 {
				continue // 停用取值不给客户选
			}
			value := customerSubValue(sub, row.Source)
			if value == "" {
				continue
			}
			if strings.TrimSpace(sub.GroupLabel) != "" {
				grouped = true
			}
			rawNames = append(rawNames, sub.OptionName)
			opt.Values = append(opt.Values, CustomerOptionItem{
				Value:          value,
				Label:          upstreamSubLabel(sub.OptionName, row.Source, value),
				Group:          sub.GroupLabel,
				Help:           sub.Help,
				Default:        sub.IsDefault,
				PriceMonthly:   sub.PriceMonthly,
				PriceQuarterly: sub.PriceQuarterly,
				PriceAnnually:  sub.PriceAnnually,
				PriceOnetime:   sub.PriceOnetime,
			})
		}
		// 分组下拉需要分组标签：config_groups 里没有 group_label 时，
		// 取值名自带家族（"12|CentOS^CentOS-7.6.1810-x64"）也能推出来。
		// 家族取自**原始取值名**而非展示名（展示名已剥掉家族前缀）。
		// 同样不限来源：自营商品的镜像取值来自平台目录，形态一致。
		if !grouped {
			grouped = labelUpstreamSubGroups(opt.Values, rawNames)
		}
		// 有分组标签时升级为分组下拉（操作系统按 Ubuntu/Windows 分组）。
		if grouped && widget == optionWidgetSelect {
			opt.Widget = optionWidgetGroupSelect
		}
		// 枚举型没有任何可见取值时跳过（客户没得选）；数量型与文本型即便无取值也要下发
		// （数量型靠区间，文本型靠客户自由填写）。
		if len(opt.Values) == 0 && opt.Widget != optionWidgetQty && opt.Widget != optionWidgetText {
			continue
		}
		options = append(options, opt)
	}
	if len(options) == 0 {
		return nil, nil
	}
	return []CustomerOptionGroup{{Options: options}}, nil
}

// labelUpstreamSubGroups 为上游取值补分组标签：从取值展示名里抽取家族
// （"12|CentOS^CentOS-7.6.1810-x64" → CentOS）。
//
// 只有家族真正重复出现（同一家族 ≥2 个取值）才启用分组：网络类型每个取值都是
// 独立家族，强行分组会得到一堆单元素分组，反而比平铺下拉更难用。返回 false 时
// 调用方维持平铺下拉。
func labelUpstreamSubGroups(items []CustomerOptionItem, rawNames []string) bool {
	families := make([]string, len(items))
	counts := map[string]int{}
	for i := range items {
		if i < len(rawNames) {
			families[i] = upstreamSubFamily(rawNames[i])
		}
		if families[i] == "" {
			families[i] = upstreamSubFamily(items[i].Label)
		}
		if families[i] != "" {
			counts[families[i]]++
		}
	}
	grouped := false
	for _, count := range counts {
		if count > 1 {
			grouped = true
			break
		}
	}
	if !grouped {
		return false
	}
	for i := range items {
		if families[i] != "" {
			items[i].Group = families[i]
		}
	}
	return true
}

// upstreamSubFamily 从上游取值展示名里取家族（^ 前那段，去掉「值|」前缀与版本号）。
func upstreamSubFamily(label string) string {
	name := strings.TrimSpace(label)
	if i := strings.Index(name, "|"); i >= 0 {
		name = strings.TrimSpace(name[i+1:])
	}
	if i := strings.Index(name, "^"); i >= 0 {
		name = strings.TrimSpace(name[:i])
	}
	// 家族名不该带版本号；只保留字母数字与常见分隔（Ubuntu-24.04 → Ubuntu）。
	name = strings.TrimSpace(strings.SplitN(name, "-", 2)[0])
	name = strings.TrimSpace(strings.SplitN(name, " ", 2)[0])
	return name
}

// upstreamOptionLabel 取上游配置项的中文名：上游下发形如「cpu|CPU」，取竖线后的说明。
func upstreamOptionLabel(optionName, fallback string) string {
	name := strings.TrimSpace(optionName)
	if idx := strings.Index(name, "|"); idx >= 0 {
		if label := strings.TrimSpace(name[idx+1:]); label != "" {
			return label
		}
	}
	if name != "" {
		return name
	}
	return fallback
}

// upstreamSubLabel 取上游取值的展示名。
//
// 上游取值名形如「vpc|VPC网络」「12|CentOS^CentOS-7.6.1810-x64」：
// 去掉「值|」前缀后，`^` 后面那段是版本明细（CentOS-7.6.1810-x64），比家族的重复
// 后缀更适合作为下拉项文字；没有 `^` 时用剩余部分。全空则回落下发 id，
// 避免界面出现空白选项。
func upstreamSubLabel(optionName, source, value string) string {
	_ = source
	name := strings.TrimSpace(optionName)
	if name == "" {
		return value
	}
	if i := strings.Index(name, "|"); i >= 0 {
		name = strings.TrimSpace(name[i+1:])
	}
	if i := strings.Index(name, "^"); i >= 0 {
		if detail := strings.TrimSpace(name[i+1:]); detail != "" {
			name = detail
		} else {
			name = strings.TrimSpace(name[:i])
		}
	}
	if name == "" {
		return value
	}
	return name
}

// axisKeysForProduct 取该商品已作为 SKU 维度的参数名集合（T4.8）。
//
// 只对上游转售商品有意义（轴选择存在 products.sku_axes 上）；自营链路的轴在规格
// 模板的 option_selections 里，生成配置项时就已排除，这里返回空集即可。
// 读不到商品时返回空集：宁可多开放一个配置项，也不要因为读失败把正常选项全藏掉。
func (s *productService) axisKeysForProduct(ctx context.Context, productID uint64) map[string]bool {
	product, err := s.repo.FindByID(ctx, productID)
	if err != nil || product == nil || product.SourceMode != catalogmodel.SourceModeUpstream {
		return map[string]bool{}
	}
	return parseUpstreamAxisSelection(product.SkuAxes)
}

// creatableOptionKeys 取该商品涉及哪些「允许自由填写」的配置项。
//
// 键是 providerType\x00optionKey：同一商品可能混用多个平台的项（上游镜像 + 自营），
// 只用 optionKey 会在跨平台同名参数上串味。
//
// 目录读不到时返回空集：宁可不让填，也不能让客户填出平台不认的值。
// （上游镜像项 source=upstream 不带 ProviderType，不会命中，属预期。）
func (s *productService) creatableOptionKeys(ctx context.Context, rows []*catalogmodel.ProductConfigOption) map[string]bool {
	out := map[string]bool{}
	if s.optionCatalog == nil {
		return out
	}
	seen := map[string]bool{}
	for _, row := range rows {
		pt := strings.TrimSpace(row.ProviderType)
		if pt == "" || seen[pt] {
			continue
		}
		seen[pt] = true
		list, err := s.optionCatalog.OptionCatalogByProvider(ctx, pt)
		if err != nil {
			continue
		}
		for _, m := range list {
			if m.Creatable {
				out[pt+"\x00"+m.OptionKey] = true
			}
		}
	}
	return out
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
//
// 上游对接项（source=upstream，操作系统/网络类型）与自营项共用同一套归一与校验，
// 只是下发键/取值换成上游 id（见 customerOptionKey/customerSubValue）；单值的固定
// 配置项不参与（列表里就没有，传了也直接忽略）。
func (s *productService) ResolveOptionPricing(ctx context.Context, productID uint64, cycle string, selections map[string]string) (*OptionPricing, error) {
	rows, err := s.repo.AllConfigOptionRows(ctx, productID)
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
		// 上游单值固定配置项不是客户可选项：既不下发也不参与校验（客户就算传了也忽略），
		// 其取值由 config_groups 在履约侧还原。
		if row.Source == catalogmodel.ConfigSourceUpstream && !customerSelectableUpstream(row) {
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
			// 单位加价按**本单周期**取：上游每个周期各有单价（月 0.8 / 季 2.4 / 年 9.6），
			// 恒取月价会让长周期订单少收配置费用。
			if unit := quantityUnitPriceForCycle(row, cycleCode); unit > 0 {
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

		// 文本型：无候选值，客户填什么就下发什么（留空表示用默认/自动生成），不参与加价。
		if widget == optionWidgetText {
			if chosen != "" {
				result.Selections[key] = chosen
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
				ValueName:   upstreamSubLabel(sub.OptionName, row.Source, value),
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
			if customerSubValue(sub, row.Source) == chosen {
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
			if customerSubValue(sub, row.Source) == row.DefaultValue {
				return row.DefaultValue, sub, nil
			}
		}
	}
	for _, sub := range visible {
		if sub.IsDefault {
			return customerSubValue(sub, row.Source), sub, nil
		}
	}
	if len(visible) > 0 {
		return customerSubValue(visible[0], row.Source), visible[0], nil
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

// quantityUnitPriceForCycle 数量型选项在**指定周期**下的每单位加价。
//
// 必须按周期取价（实测故障）：上游给价档的每个周期各有一个单价
// （系统盘 0.80/月、2.40/季、9.60/年），而原实现恒取 PriceMonthly ——
// 于是选年付时"套餐主体按年计价、加配的盘却仍只收一个月"，客户少付一大截
// （实测：数据盘加 20GB，年付应加 192 元，实际只加 16 元）。
//
// 回落链与枚举型 priceForCycle 保持一致：该周期无独立价时回落月度价，
// 避免上游只填了月价的老数据算成 0 元加价。
func quantityUnitPriceForCycle(row *catalogmodel.ProductConfigOption, cycle string) float64 {
	for _, sub := range row.Subs {
		if sub.Hidden != 0 {
			continue
		}
		if sub.PriceMonthly <= 0 && sub.PriceQuarterly <= 0 && sub.PriceAnnually <= 0 && sub.PriceOnetime <= 0 {
			continue
		}
		return priceForCycle(sub, cycle)
	}
	return 0
}

func stringOfFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
