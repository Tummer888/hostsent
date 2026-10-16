package service

// 上游转售商品的 SKU 维度展开（T4.8）。
//
// 背景：对接魔方财务商品时，上游用「配置项」表达一切差异 —— CPU 4核/8核、内存
// 8G/16G、系统盘 40G/80G 都是配置项的不同取值。自营链路靠规格模板声明哪些参数是
// SKU 维度（option_selections.sku_axis）；上游没有模板，所以轴选择落在商品行
// products.sku_axes 上。
//
// 为什么不能复用配置项表存这个标记：上游配置项由镜像生成，SaveConfigOptions 是
// 覆盖式重写（先删后插），标记会被下次同步抹掉。轴是运营决策，必须独立存活。
//
// 生成语义：对选中的轴取笛卡尔积，每个组合建一条 SKU，并把该组合「钉」到上游：
//
//	platform_params = {"configoption": {"<上游选项id>": "<上游取值id>", ...}}
//
// 该格式正是履约下发格式（同 upstreamConfigOptionOverride 的输出），因此 SKU 一旦
// 钉住，履约时按它下发即可，不需要再依赖客户选择。
//
// 与自营链路的差异（必须理解）：
//   - 自营的 SKU 价格来自 product_prices 矩阵，按 SKU 独立定价；
//   - 上游的取值加价在上游手里（option sub 的 pricings.fullMonth 等），本地只能拿到
//     "基准周期价 + 该组合的加价小计"再乘运营的加价倍率，写进矩阵。上游改价不会
//     自动改写已生成的 SKU 价 —— 这是有意为之（禁止静默改价），运营可重新生成。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
)

const (
	// upstreamAxisMaxCombinations 上游轴展开的 SKU 数上限（与自营同口径）。
	//
	// 上游更容易撞上限：CPU 8 档 × 内存 11 档就是 88 个，全库实测有 4 个商品超标、
	// 把系统盘也算进去则有 9 个（最大 560）。超限时拒绝并提示，让运营自己收敛维度 ——
	// 一次建出几百个 SKU 既撑爆页面，也没人维护得住。
	upstreamAxisMaxCombinations = 64
)

// upstreamAxisCandidate 一个「可做 SKU 维度」的上游配置项。
type upstreamAxisCandidate struct {
	// Key 本地参数名（cpu / memory / system_disk_size…），落 products.sku_axes 的键。
	Key string
	// OptionName 上游原始展示名（"cpu|CPU"）。
	OptionName string
	// Label 展示名（"CPU"）。
	Label string
	// UpstreamOptionID 上游配置项 id，下发键。
	UpstreamOptionID int64
	// Values 可见取值（去重、保持上游顺序）。
	Values []upstreamAxisValue
	// Locked 上游锁定（仅一个可见取值）：本就是单值，做轴无意义但也不报错。
	Locked bool
}

// upstreamAxisValue 一个轴取值。
type upstreamAxisValue struct {
	// UpstreamSubID 上游取值 id，下发值。
	UpstreamSubID int64
	// Raw 上游原始取值名（"4|4核"），用于派生编码/名称。
	Raw string
	// Label 展示名（"4核"）。
	Label string
	// Monthly 月度加价（上游口径，元）。
	Monthly float64
	// Quarterly / Annually / Onetime 其他周期加价。
	Quarterly float64
	Annually  float64
	Onetime   float64
}

// upstreamAxisPlan 一次上游轴展开的计划。
type upstreamAxisPlan struct {
	Axes []upstreamAxisCandidate
}

// Empty 没有轴（或全部轴都只有单值 → 无正交收益）。
func (p *upstreamAxisPlan) Empty() bool { return p == nil || len(p.Axes) == 0 }

// CombinationCount 组合数（笛卡尔积）。
func (p *upstreamAxisPlan) CombinationCount() int {
	if p.Empty() {
		return 0
	}
	total := 1
	for _, axis := range p.Axes {
		total *= len(axis.Values)
	}
	return total
}

// candidates 从商品的配置项行里挑出「可以做 SKU 维度」的项。
//
// 判定（保守，只放行"确实是规格差异"的项）：
//   - 排除数量型（系统盘可填 30~1000G、带宽可填 1~1000M）：客户填的是连续值，
//     枚举不出有限 SKU —— 这是上游最容易误做成轴的一类，必须挡住；
//   - 排除自营项（source=self，主机名/密码这类开通选项）；
//   - 只保留有上游选项 id 的项（没有 id 就钉不住，生成出来的 SKU 无法履约）；
//   - 可见取值多于 1 个才是有意义的轴（单值项做轴只产出 1 条 SKU）。
func (s *productService) upstreamAxisCandidates(ctx context.Context, productID uint64) ([]upstreamAxisCandidate, error) {
	rows, err := s.repo.AllConfigOptionRows(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]upstreamAxisCandidate, 0, len(rows))
	for _, row := range rows {
		if row.Source != model.ConfigSourceUpstream {
			continue
		}
		// 数量型：客户填连续值，不能枚举成 SKU。
		if isQuantityOption(row.OptionType) {
			continue
		}
		if row.UpstreamKey <= 0 {
			continue
		}
		values := make([]upstreamAxisValue, 0, len(row.Subs))
		seen := map[int64]bool{}
		for _, sub := range row.Subs {
			if sub.Hidden != 0 || sub.UpstreamKey <= 0 {
				continue
			}
			if seen[sub.UpstreamKey] {
				continue
			}
			seen[sub.UpstreamKey] = true
			values = append(values, upstreamAxisValue{
				UpstreamSubID: sub.UpstreamKey,
				Raw:           sub.OptionName,
				Label:         upstreamOptionLabel(sub.OptionName, strconv.FormatInt(sub.UpstreamKey, 10)),
				Monthly:       sub.PriceMonthly,
				Quarterly:     sub.PriceQuarterly,
				Annually:      sub.PriceAnnually,
				Onetime:       sub.PriceOnetime,
			})
		}
		if len(values) == 0 {
			continue
		}
		key := upstreamOptionKey(row.OptionName)
		if key == "" {
			key = strings.TrimSpace(row.OptionKey)
		}
		if key == "" {
			continue
		}
		out = append(out, upstreamAxisCandidate{
			Key:              key,
			OptionName:       row.OptionName,
			Label:            upstreamOptionLabel(row.OptionName, row.OptionKey),
			UpstreamOptionID: row.UpstreamKey,
			Values:           values,
			Locked:           len(values) <= 1,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// upstreamAxisSelection 解析商品的轴选择（products.sku_axes）。
func parseUpstreamAxisSelection(raw string) map[string]bool {
	out := map[string]bool{}
	s := strings.TrimSpace(raw)
	if s == "" || s == "null" || s == "{}" {
		return out
	}
	// 存的是 {"cpu":true,"memory":true}；容错解析成 map[string]any 再看真假值。
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return out
	}
	for k, v := range m {
		if b, ok := v.(bool); ok && b {
			out[strings.TrimSpace(k)] = true
			continue
		}
		// 兼容 {"cpu": 1} / {"cpu": "true"} 这类写法。
		switch t := v.(type) {
		case float64:
			if t != 0 {
				out[strings.TrimSpace(k)] = true
			}
		case string:
			if strings.EqualFold(strings.TrimSpace(t), "true") || t == "1" {
				out[strings.TrimSpace(k)] = true
			}
		}
	}
	return out
}

// buildUpstreamAxisPlan 按运营的轴选择组装计划，并做与自营同口径的限额校验。
func (s *productService) buildUpstreamAxisPlan(ctx context.Context, product *model.Product) (*upstreamAxisPlan, error) {
	selected := parseUpstreamAxisSelection(product.SkuAxes)
	plan := &upstreamAxisPlan{}
	if len(selected) == 0 {
		return plan, nil
	}
	candidates, err := s.upstreamAxisCandidates(ctx, product.ID)
	if err != nil {
		return nil, err
	}
	byKey := map[string]upstreamAxisCandidate{}
	for _, c := range candidates {
		byKey[c.Key] = c
	}
	// 稳定顺序：按 Key 排序，保证组合顺序与编码可复现。
	keys := make([]string, 0, len(selected))
	for k := range selected {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		cand, ok := byKey[key]
		if !ok {
			return nil, fmt.Errorf(
				"「%s」不能作为 SKU 维度：它要么是数量型配置项（客户填连续值，枚举不出规格），"+
					"要么已被上游移除。请到 SKU 维度设置里重新选择", key)
		}
		plan.Axes = append(plan.Axes, cand)
	}
	if total := plan.CombinationCount(); total > upstreamAxisMaxCombinations {
		return nil, fmt.Errorf(
			"所选 SKU 维度的取值组合出 %d 个规格，超出上限 %d 个；"+
				"请减少维度或取值（带宽/系统盘/数据盘这类连续量不应作为维度）",
			total, upstreamAxisMaxCombinations)
	}
	return plan, nil
}

// upstreamAxisCombos 展开笛卡尔积；顺序稳定（第一个轴变化最慢）。
func upstreamAxisCombos(axes []upstreamAxisCandidate) []map[string]upstreamAxisValue {
	if len(axes) == 0 {
		return nil
	}
	out := []map[string]upstreamAxisValue{{}}
	for _, axis := range axes {
		next := make([]map[string]upstreamAxisValue, 0, len(out)*len(axis.Values))
		for _, base := range out {
			for _, val := range axis.Values {
				combo := make(map[string]upstreamAxisValue, len(base)+1)
				for k, v := range base {
					combo[k] = v
				}
				combo[axis.Key] = val
				next = append(next, combo)
			}
		}
		out = next
	}
	return out
}

// upstreamAxisPins 把组合转成钉住上游的下发参数。
//
// 形态与履约一致：{"configoption": {"<选项id>": "<取值id>"}}。
// 数量型轴已被上游轴候选排除，因此这里按枚举型（取值 id）处理。
//
// 键必须是**上游配置项 id**（不是本地参数名）：上游 cart 按
// configoption[选项id]=取值id 取参（见 zjmf buildCartConfigOption 与
// upstreamConfigOptionOverride），用本地词汇（"cpu"）下不去，上游会按默认配置开通。
func upstreamAxisPins(axes []upstreamAxisCandidate, combo map[string]upstreamAxisValue) map[string]interface{} {
	pins := map[string]interface{}{}
	for _, axis := range axes {
		val, ok := combo[axis.Key]
		if !ok || val.UpstreamSubID <= 0 || axis.UpstreamOptionID <= 0 {
			continue
		}
		pins[strconv.FormatInt(axis.UpstreamOptionID, 10)] = strconv.FormatInt(val.UpstreamSubID, 10)
	}
	return pins
}

// upstreamAxisSpecCode 由轴组合派生确定性编码。
//
// 用轴的**本地参数名 + 取值标签**拼段（cpu4-memory8g），因为上游取值 id 是数字，
// 拼进编码没人看得懂；同组合恒得同一编码 → 重新生成时命中同一 SKU 做更新而非新增。
func upstreamAxisSpecCode(axes []upstreamAxisCandidate, combo map[string]upstreamAxisValue) string {
	parts := make([]string, 0, len(axes)+1)
	for _, axis := range axes {
		val, ok := combo[axis.Key]
		if !ok {
			continue
		}
		seg := upstreamAxisCodeSegment(axis.Key, val)
		if seg == "" {
			continue
		}
		parts = append(parts, seg)
	}
	if len(parts) == 0 {
		return "sku"
	}
	return strings.Join(parts, "-")
}

// upstreamAxisCodeSegment 单个轴的编码片段：cpu 用 8c、memory 用 16g，
// 其余参数用「参数名+取值」。口径与自营一致的行业写法。
func upstreamAxisCodeSegment(key string, val upstreamAxisValue) string {
	num := upstreamNumericValue(val.Raw)
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "cpu":
		if num > 0 {
			return strconv.Itoa(num) + "c"
		}
	case "memory", "mem":
		if num > 0 {
			// 上游内存按 MB 记（4096），编码里换算成 G。
			gb := num / 1024
			if gb <= 0 {
				gb = 1
			}
			return strconv.Itoa(gb) + "g"
		}
	case "system_disk_size", "disk", "disk_size":
		if num > 0 {
			return strconv.Itoa(num) + "g"
		}
	}
	// 无数字的取值（如 os 的镜像名）：用参数名 + 清洗后的标签。
	label := sanitizeCodeToken(val.Label)
	if label == "" {
		label = strconv.FormatInt(val.UpstreamSubID, 10)
	}
	return sanitizeCodeToken(key) + label
}

// upstreamNumericValue 取上游取值名里的数字（"4|4核" → 4，"4096|4G" → 4096）。
// 取第一个纯数字段 —— 上游取值名习惯是「值|展示名」，值本身即数字。
func upstreamNumericValue(raw string) int {
	head := strings.TrimSpace(raw)
	if idx := strings.Index(head, "|"); idx >= 0 {
		head = head[:idx]
	}
	if n, err := strconv.Atoi(strings.TrimSpace(head)); err == nil {
		return n
	}
	// 兜底：从展示名里挑第一段数字（"4核" → 4）。
	digits := strings.Builder{}
	for _, r := range head {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
			continue
		}
		if digits.Len() > 0 {
			break
		}
	}
	if digits.Len() == 0 {
		return 0
	}
	n, err := strconv.Atoi(digits.String())
	if err != nil {
		return 0
	}
	return n
}

// upstreamAxisSpecName 由轴组合派生规格名（"4核-8G"）。
func upstreamAxisSpecName(axes []upstreamAxisCandidate, combo map[string]upstreamAxisValue) string {
	parts := make([]string, 0, len(axes))
	for _, axis := range axes {
		val, ok := combo[axis.Key]
		if !ok {
			continue
		}
		label := strings.TrimSpace(val.Label)
		if label == "" {
			label = strings.TrimSpace(val.Raw)
		}
		if label == "" {
			continue
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}

// upstreamAxisPrice 组合的周期价：基准周期价 + 各轴取值在该周期上的加价，再乘倍率。
//
// 为什么是"基准 + 加价"而不是直接取取值价：上游的 option sub pricings 是**增量**
// （商品 112 基准 40，cpu 2核=0、4核=16、8核=48），矩阵存的却是绝对价。
// 基准取商品当前周期价（克隆时已按上游成本 × 倍率种好），保证与"不拆 SKU 时"一致。
func upstreamAxisPrice(base float64, axes []upstreamAxisCandidate, combo map[string]upstreamAxisValue, cycle string) float64 {
	amount := base
	for _, axis := range axes {
		val, ok := combo[axis.Key]
		if !ok {
			continue
		}
		amount += upstreamAxisValuePrice(val, cycle)
	}
	if amount < 0 {
		amount = 0
	}
	return round2Price(amount)
}

// upstreamAxisValuePrice 取某取值在某周期的加价（缺档时回落月价，与自营同口径）。
func upstreamAxisValuePrice(val upstreamAxisValue, cycle string) float64 {
	switch cycle {
	case "quarterly":
		if val.Quarterly > 0 {
			return val.Quarterly
		}
		return val.Monthly
	case "semiannually":
		// 上游多数商品不单列半年价：按季价 × 2 折算。
		if val.Quarterly > 0 {
			return round2Price(val.Quarterly * 2)
		}
		return val.Monthly
	case "annually":
		if val.Annually > 0 {
			return val.Annually
		}
		return val.Monthly
	case "onetime":
		if val.Onetime > 0 {
			return val.Onetime
		}
		return val.Monthly
	default:
		return val.Monthly
	}
}

// generateUpstreamAxisSpecs 按轴展开生成上游商品的 SKU（幂等）。
//
// 每一条 SKU：
//   - spec_code / name 由组合确定性派生（同组合重跑 = 更新，不新增）；
//   - platform_params 钉住上游选项 id → 取值 id（履约时直接下发该组合）；
//   - specs 写原子取值（供展示与校验；无原子映射的参数写进 Extra）；
//   - 周期价 = 商品当前周期价 + 该组合加价（写进该 SKU 的矩阵行）。
func (s *productService) generateUpstreamAxisSpecs(ctx context.Context, product *model.Product, plan *upstreamAxisPlan, operatorID uint64, operatorName string) ([]dto.ProductSpecInfo, error) {
	combos := upstreamAxisCombos(plan.Axes)
	out := make([]dto.ProductSpecInfo, 0, len(combos))
	// 基准周期价：读商品当前的矩阵（克隆时已按上游成本 × 倍率种好）。
	basePrices := s.currentCyclePrices(ctx, product.ID)
	for _, combo := range combos {
		code := upstreamAxisSpecCode(plan.Axes, combo)
		name := upstreamAxisSpecName(plan.Axes, combo)
		if name == "" {
			name = product.Name
		}
		specs, platParams := s.upstreamAxisSpecSnapshot(plan, combo)
		specCode := code
		item := &model.ProductSpec{
			ProductID:  product.ID,
			SpecCode:   specCode,
			Name:       name,
			Specs:      specs,
			PriceModel: firstNonEmptyStr(product.PriceModel, model.PriceModelFixed),
			Price:      upstreamAxisPrice(product.Price, plan.Axes, combo, "monthly"),
			Stock:      -1,
			Status:     model.ProductSpecEnabled,
		}
		// 幂等：同组合命中既有编码 → 更新；否则新增。
		if existing, ferr := s.repo.FindSpecByCode(ctx, product.ID, specCode); ferr == nil && existing != nil {
			item.ID = existing.ID
			item.SortOrder = existing.SortOrder
			if err := s.repo.UpdateSpec(ctx, item); err != nil {
				return out, fmt.Errorf("更新规格 %s 失败：%w", specCode, err)
			}
		} else if err := s.repo.CreateSpec(ctx, item); err != nil {
			return out, fmt.Errorf("生成规格 %s 失败：%w", specCode, err)
		}
		// 平台绑定：只写该组合的 pin（不复制上游基线的 area/os —— 那些仍由
		// 商品的配置项/mirror 决定，写进来反而会造成两处真源）。
		if s.bindingWriter != nil && len(platParams) > 0 {
			raw, err := json.Marshal(platParams)
			if err == nil {
				_ = s.bindingWriter.UpsertConfirmedProductSpecBinding(
					ctx, item.ID, 0, string(raw), operatorID, "按上游 SKU 维度生成："+name)
			}
		}
		// 该 SKU 的周期价矩阵：有基准价的周期各写一行（基准 + 加价）。
		s.saveUpstreamAxisPrices(ctx, product.ID, item.ID, basePrices, plan.Axes, combo)
		infos := []dto.ProductSpecInfo{buildProductSpecInfo(*item)}
		s.fillSpecBinding(ctx, infos)
		out = append(out, infos[0])
	}
	if len(out) > 0 {
		_ = s.repo.AddHistory(ctx, &model.ProductHistory{
			ProductID: product.ID, ChangeType: model.ChangeTypeUpdate,
			NewValue: fmt.Sprintf("%d", len(out)), OperatorID: operatorID, OperatorName: operatorName,
			Remark: fmt.Sprintf("按 SKU 维度生成 %d 个规格（上游商品）", len(out)),
		})
	}
	return out, nil
}

// upstreamAxisSpecSnapshot 组装某组合的原子取值与平台参数。
//
// specs 用原子 key（cpu → compute.cpu）：既有字典里映射得到就用原子 key（这样
// 上架门禁的原子校验与实例详情展示都能用），映射不到的参数（如 network_type、
// os 的镜像 id）原样留在 Extra —— ValidateMap 对未登记键会报错，故这类键
// 只在白名单/原子内写，其余交给 platform_params 承载。
func (s *productService) upstreamAxisSpecSnapshot(plan *upstreamAxisPlan, combo map[string]upstreamAxisValue) (string, map[string]interface{}) {
	specs := map[string]interface{}{}
	pins := upstreamAxisPins(plan.Axes, combo)
	for _, axis := range plan.Axes {
		val, ok := combo[axis.Key]
		if !ok {
			continue
		}
		// 只有能对应到原子字典的参数才写进 specs（cpu/memory/disk），
		// 其余（os/network_type 等）靠 platform_params 的 pin 表达。
		if atomKey, ok := upstreamAxisAtomKey(axis.Key); ok {
			num := upstreamNumericValue(val.Raw)
			if num > 0 {
				// 内存原子口径是 MB，上游也是 MB，直接落。
				specs[atomKey] = num
			}
		}
	}
	raw, _ := json.Marshal(specs)
	platParams := map[string]interface{}{}
	if len(pins) > 0 {
		platParams["configoption"] = pins
	}
	return string(raw), platParams
}

// upstreamAxisAtomKey 上游参数名 → 原子 key（只覆盖能明确对应的几个）。
func upstreamAxisAtomKey(key string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "cpu":
		return "compute.cpu", true
	case "memory", "mem":
		return "compute.memory", true
	case "system_disk_size", "disk", "disk_size":
		return "storage.system.size", true
	case "bw", "bandwidth":
		return "network.bandwidth", true
	}
	return "", false
}

// currentCyclePrices 读商品当前的周期价（商品级矩阵；无矩阵时回落商品展示价）。
func (s *productService) currentCyclePrices(ctx context.Context, productID uint64) map[string]float64 {
	out := map[string]float64{}
	rows, err := s.cycleMatrixReader.ListPrices(ctx, productID, 0)
	if err != nil {
		return out
	}
	for _, row := range rows {
		if row.Price <= 0 {
			continue
		}
		out[row.Cycle] = row.Price
	}
	return out
}

// saveUpstreamAxisPrices 写入该 SKU 的周期价矩阵（基准 + 该组合加价）。
func (s *productService) saveUpstreamAxisPrices(ctx context.Context, productID, specID uint64, base map[string]float64, axes []upstreamAxisCandidate, combo map[string]upstreamAxisValue) {
	if s.cycleMatrixWriter == nil || specID == 0 || len(base) == 0 {
		return
	}
	items := make([]CyclePriceRow, 0, len(base))
	cycles := make([]string, 0, len(base))
	for cycle := range base {
		cycles = append(cycles, cycle)
	}
	sort.Strings(cycles)
	for _, cycle := range cycles {
		items = append(items, CyclePriceRow{
			Cycle:     cycle,
			Price:     upstreamAxisPrice(base[cycle], axes, combo, cycle),
			CostPrice: 0,
			Source:    "manual",
		})
	}
	if err := s.cycleMatrixWriter.SaveSpecPrices(ctx, productID, specID, items); err != nil {
		// 价格写失败不阻断 SKU 生成：规格已建好，运营可在价格页补。
		_ = err
	}
}

// ============================================================================
// 对外服务方法（T4.8）
// ============================================================================

// UpstreamAxisOptions 列出可做 SKU 维度的配置项与当前选择。
func (s *productService) UpstreamAxisOptions(ctx context.Context, productID uint64) (*dto.UpstreamAxisOptionsInfo, error) {
	product, err := s.repo.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.SourceMode != model.SourceModeUpstream {
		return nil, errors.New("只有上游转售商品需要设置 SKU 维度；自营商品的规格维度在规格模板里配置")
	}
	candidates, err := s.upstreamAxisCandidates(ctx, productID)
	if err != nil {
		return nil, err
	}
	selected := parseUpstreamAxisSelection(product.SkuAxes)
	info := &dto.UpstreamAxisOptionsInfo{
		ProductID:       productID,
		Candidates:      make([]dto.UpstreamAxisOptionInfo, 0, len(candidates)),
		SelectedKeys:    make([]string, 0, len(selected)),
		MaxCombinations: upstreamAxisMaxCombinations,
	}
	for _, cand := range candidates {
		item := dto.UpstreamAxisOptionInfo{
			Key:        cand.Key,
			Label:      cand.Label,
			OptionName: cand.OptionName,
			Locked:     cand.Locked,
			Selected:   selected[cand.Key],
			Values:     make([]dto.UpstreamAxisValueInfo, 0, len(cand.Values)),
		}
		for _, v := range cand.Values {
			item.Values = append(item.Values, dto.UpstreamAxisValueInfo{
				UpstreamSubID: v.UpstreamSubID,
				Label:         v.Label,
				Raw:           v.Raw,
				Monthly:       v.Monthly,
				Quarterly:     v.Quarterly,
				Annually:      v.Annually,
				Onetime:       v.Onetime,
			})
		}
		info.Candidates = append(info.Candidates, item)
	}
	// 组合数：把每个候选依次"假如选上"算一遍，界面实时预览用。
	for i := range info.Candidates {
		keys := make([]string, 0, len(selected)+1)
		for k := range selected {
			keys = append(keys, k)
		}
		if !selected[info.Candidates[i].Key] {
			keys = append(keys, info.Candidates[i].Key)
		}
		info.Candidates[i].CombinationIfSelected = s.upstreamAxisCombinationFor(keys, candidates)
	}
	info.SelectedKeys = sortedAxisSelectedKeys(selected)
	info.CombinationCount = s.upstreamAxisCombinationFor(info.SelectedKeys, candidates)
	// 已保存的维度若已失效（上游改了配置项），给出可读提示而不是静默忽略。
	if plan, perr := s.buildUpstreamAxisPlan(ctx, product); perr != nil {
		info.Notice = perr.Error()
	} else {
		_ = plan
	}
	if specs, serr := s.repo.ListSpecs(ctx, productID); serr == nil {
		info.ExistingSpecCount = len(specs)
	}
	return info, nil
}

// sortedAxisSelectedKeys 取有序的已选维度键。
func sortedAxisSelectedKeys(selected map[string]bool) []string {
	out := make([]string, 0, len(selected))
	for k := range selected {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// upstreamAxisCombinationFor 算「这些 key 若都作为维度」的组合数。
func (s *productService) upstreamAxisCombinationFor(keys []string, candidates []upstreamAxisCandidate) int {
	byKey := map[string]upstreamAxisCandidate{}
	for _, c := range candidates {
		byKey[c.Key] = c
	}
	total := 1
	counted := 0
	for _, k := range keys {
		cand, ok := byKey[k]
		if !ok || len(cand.Values) == 0 {
			continue
		}
		total *= len(cand.Values)
		counted++
	}
	if counted == 0 {
		return 0
	}
	return total
}

// SaveUpstreamAxisOptions 保存 SKU 维度选择（只存选择，不立即生成）。
//
// 为什么与"生成"分开：运营往往先试几种维度组合看组合数，确认后再生成；
// 保存即生成会让每次试选都落一批规格数据。
func (s *productService) SaveUpstreamAxisOptions(ctx context.Context, productID uint64, req dto.UpstreamAxisSaveRequest, operatorID uint64, operatorName string) (*dto.UpstreamAxisOptionsInfo, error) {
	product, err := s.repo.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.SourceMode != model.SourceModeUpstream {
		return nil, errors.New("只有上游转售商品需要设置 SKU 维度")
	}
	candidates, err := s.upstreamAxisCandidates(ctx, productID)
	if err != nil {
		return nil, err
	}
	byKey := map[string]upstreamAxisCandidate{}
	for _, c := range candidates {
		byKey[c.Key] = c
	}
	// 校验：所选维度必须存在、且不是数量型（候选列表本身已排除数量型，
	// 这里兜住"传了不在候选里的 key"这种情况）。
	chosen := map[string]bool{}
	total := 1
	axisCount := 0
	for _, key := range req.Keys {
		key = strings.TrimSpace(key)
		if key == "" || chosen[key] {
			continue
		}
		cand, ok := byKey[key]
		if !ok {
			return nil, fmt.Errorf("「%s」不能作为 SKU 维度：它可能是数量型配置项（客户填连续值），或已被上游移除", key)
		}
		// 单值（上游锁定）项做维度只会产出 1 个规格 —— 那不是"维度"，是固定配置。
		// 界面已禁用这类勾选，这里兜住直接调接口的情形（避免悄悄建出一条无意义的规格）。
		if len(cand.Values) <= 1 {
			return nil, fmt.Errorf(
				"「%s」在上游只有 1 个取值（固定配置），作为 SKU 维度只会生成 1 个规格、没有意义；"+
					"它已经由商品本身表达，无需拆维度", cand.Label)
		}
		chosen[key] = true
		total *= len(cand.Values)
		axisCount++
	}
	if axisCount > 0 && total > upstreamAxisMaxCombinations {
		return nil, fmt.Errorf("所选维度的取值组合出 %d 个规格，超出上限 %d 个；请减少维度或取值（带宽/系统盘/数据盘这类连续量不应作为维度）",
			total, upstreamAxisMaxCombinations)
	}
	raw, err := json.Marshal(chosen)
	if err != nil {
		return nil, err
	}
	product.SkuAxes = string(raw)
	if err := s.repo.Update(ctx, product); err != nil {
		return nil, err
	}
	_ = s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: productID, ChangeType: model.ChangeTypeUpdate,
		NewValue:   strings.Join(sortedAxisSelectedKeys(chosen), ","),
		OperatorID: operatorID, OperatorName: operatorName,
		Remark: fmt.Sprintf("设置 SKU 维度：%s", strings.Join(sortedAxisSelectedKeys(chosen), "、")),
	})
	return s.UpstreamAxisOptions(ctx, productID)
}

// GenerateUpstreamAxisSpecs 按已保存的维度展开生成规格（幂等）。
func (s *productService) GenerateUpstreamAxisSpecs(ctx context.Context, productID uint64, operatorID uint64, operatorName string) (*dto.UpstreamAxisGenerateResult, error) {
	product, err := s.repo.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.SourceMode != model.SourceModeUpstream {
		return nil, errors.New("只有上游转售商品可以按 SKU 维度生成规格")
	}
	plan, err := s.buildUpstreamAxisPlan(ctx, product)
	if err != nil {
		return nil, err
	}
	if plan.Empty() {
		return nil, errors.New("尚未选择 SKU 维度（或所选维度都只有单值），没有可生成的规格")
	}
	// 先记下已有编码，便于区分本次是新增还是更新。
	before := map[string]bool{}
	if specs, lerr := s.repo.ListSpecs(ctx, productID); lerr == nil {
		for _, sp := range specs {
			before[sp.SpecCode] = true
		}
	}
	specs, err := s.generateUpstreamAxisSpecs(ctx, product, plan, operatorID, operatorName)
	if err != nil {
		return nil, err
	}
	result := &dto.UpstreamAxisGenerateResult{
		Specs:            specs,
		CombinationCount: plan.CombinationCount(),
	}
	for _, sp := range specs {
		if before[sp.SpecCode] {
			result.Updated++
		} else {
			result.Created++
		}
	}
	return result, nil
}
