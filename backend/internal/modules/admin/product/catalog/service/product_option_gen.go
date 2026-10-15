package service

// 建品时「配置档 → 客户选配项」的推导逻辑（T4.5）。
//
// 配置档（product_spec_templates.option_selections）里记录运营为每个平台参数勾选的
// 可选值，形如：
//
//	{
//	  "cpu":            {"values": ["2", "4", "8"]},
//	  "memory":         {"values": ["4096", "8192"]},
//	  "os":             {"values": ["12", "13"], "group_label": "Ubuntu"},
//	  "bw":             {"range": [1, 100], "default": "10"},
//	  "data_disk_size": {"range": [0, 500], "default": "0"}
//	}
//
// values 是离散多选（CPU/内存/镜像/区域）；range 是数量型区间（带宽/盘/IP 数），
// 用户侧渲染成步进器而不是一组单选。
//
// 生成规则：
//   - 取值多于一个，或本身是数量型 → 生成一件客户可选配置项；
//   - 只有一个取值且非数量型 → 不生成（客户没得选，作为 SKU 基线值直接下发）；
//   - 跨档位同参数取并集（多个档位都开放 2核/4核 时只出现一件配置项）。

import (
	"encoding/json"
	"sort"
	"strings"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
)

// 控件类型（与 model.ConfigWidget* / upstream.ConfigWidget* 同口径）。
const (
	SpecWidgetSelect = "select"
	SpecWidgetRadio  = "radio"
	SpecWidgetQty    = "qty"
	SpecWidgetBool   = "bool"
	// SpecWidgetGroupSelect 分组下拉（操作系统按 Ubuntu/Windows 分组）。
	SpecWidgetGroupSelect = "group_select"
)

// optionSelection 配置档里单个参数的勾选结果。
type optionSelection struct {
	// Values 离散多选取值。
	Values []string `json:"values"`
	// Range 数量型区间 [min, max]（长度 2 时生效）。
	Range []float64 `json:"range"`
	// Default 默认值（字符串形式，与平台写参数口径一致）。
	//
	// 类型是 flexString 而不是 string：这一列是 jsonb，写入方不止一个 ——
	// 编辑器写的是字符串 "10"，而 SQL 侧的修正（migrations/111 的 to_jsonb(numeric)）
	// 与手工 UPDATE 写出来的是数字 10。两种形态都必须能读，
	// 否则一次类型不符会让**整份 selections 反序列化失败**，
	// 表现为商品缺失全部配置项（实测故障）。
	Default flexString `json:"default"`
	// GroupLabel 分组标签（同一参数的取值默认归入哪个分组）。
	GroupLabel string `json:"group_label"`
}

// flexString 兼容 JSON 字符串与数字两种写法的字符串字段。
//
// 只用于读配置档这类"多来源写入"的 jsonb，避免为类型差异把整份数据丢掉。
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = ""
		return nil
	}
	// 字符串：去掉引号并解转义；数字/布尔等：取字面量。
	if s[0] == '"' {
		var out string
		if err := json.Unmarshal(b, &out); err != nil {
			return err
		}
		*f = flexString(out)
		return nil
	}
	*f = flexString(s)
	return nil
}

func (f flexString) String() string { return string(f) }

// parseOptionSelections 解析配置档的 option_selections JSON；非法/空返回 nil。
func parseOptionSelections(raw string) map[string]optionSelection {
	s := strings.TrimSpace(raw)
	if s == "" || s == "null" {
		return nil
	}
	var out map[string]optionSelection
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// sortedOptionKeys 稳定排序参数名，保证生成顺序可复现（配置项排序要稳定）。
func sortedOptionKeys(sel map[string]optionSelection) []string {
	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// optionValue 已确定可取的一个值（含展示名/分组/含义/加价）。
type optionValue struct {
	Value   string
	Label   string
	Group   string
	Help    string
	Default bool
	Hidden  bool
	Monthly float64
	Annual  float64
	Quarter float64
	Onetime float64
}

// optionAccumulator 跨档位累加同一参数的可选值与元数据。
type optionAccumulator struct {
	meta   SpecOptionMeta
	values []optionValue
	seen   map[string]int // value → values 下标（去重合并）
	// range/default 仅数量型使用
	minValue *float64
	maxValue *float64
	defRange string
	// isRange 该参数是数量型（区间），即使没有离散取值也要生成配置项。
	isRange bool
	// explicit 标记该参数是否被建品页显式覆盖过（覆盖后不再受目录默认约束）。
	explicit bool
}

// addValues 把某档位勾选的取值并入累加器。
// meta 提供取值标签（如镜像的中文名）与推荐控件，避免用户侧只看到一个数字 ID。
func (a *optionAccumulator) addValues(sel optionSelection, meta SpecOptionMeta) {
	// 数量型：把区间与默认值记下来，用户侧渲染步进器。
	if len(sel.Range) == 2 {
		a.isRange = true
		a.meta.Widget = SpecWidgetQty
		minV, maxV := sel.Range[0], sel.Range[1]
		if a.minValue == nil || minV < *a.minValue {
			a.minValue = &minV
		}
		if a.maxValue == nil || maxV > *a.maxValue {
			a.maxValue = &maxV
		}
		// 取最宽区间：不同档位给的区间不同时，客户应能选到任意档位允许的值。
		a.meta.MinValue = a.minValue
		a.meta.MaxValue = a.maxValue
		if d := strings.TrimSpace(sel.Default.String()); d != "" {
			// 数量型的默认值：目录没给默认时用档位声明，保证客户看到的是一个可开通的值。
			if a.defRange == "" || a.meta.Default == "" {
				a.meta.Default = d
			}
			if a.defRange == "" {
				a.defRange = d
			}
		}
		return
	}
	// 档位声明的默认取值（模板 option_selections 的 default）。
	declDefault := strings.TrimSpace(sel.Default.String())
	for _, v := range sel.Values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		a.seenEnsure()
		if idx, ok := a.seen[v]; ok {
			// 已在其它档位出现过：只补空标签，不动其它字段。
			if a.values[idx].Label == "" {
				a.values[idx].Label = labelForValue(meta, v)
			}
			if a.values[idx].Group == "" {
				a.values[idx].Group = groupForValue(meta, v, sel.GroupLabel)
			}
			if a.values[idx].Help == "" {
				a.values[idx].Help = optionValueHelp(meta, v)
			}
			continue
		}
		a.seen[v] = len(a.values)
		a.values = append(a.values, optionValue{
			Value: v,
			Label: labelForValue(meta, v),
			Group: groupForValue(meta, v, sel.GroupLabel),
			Help:  optionValueHelp(meta, v),
			// 档位声明的 default 即"基线值"，标记它让后端 pickEnumValue 有据可依。
			Default: declDefault != "" && v == declDefault,
		})
	}
	// 控件：目录推荐优先，多取值时按目录能力降级/升级。
	if a.meta.Widget == "" {
		a.meta.Widget = SpecWidgetSelect
	}
	// 枚举型默认值必须落在**已开放的取值**里（实测故障）：目录的默认值常是
	// 平台自身口径（snap_num 目录默认 2），而档位只开放了 -1 —— 不校正就会生成
	// default_value=2 而取值列表只有 -1，客户下单时预选不出任何一项，
	// 编辑器还显示"平台默认 2"这种客户永远选不到的值。
	// 口径：档位声明的 default 优先，其次目录默认（仅当它在取值内），最后首个取值。
	if len(a.values) > 0 {
		_, declOpen := a.seen[declDefault]
		_, metaOpen := a.seen[a.meta.Default]
		switch {
		case declDefault != "" && declOpen:
			a.meta.Default = declDefault
		case a.meta.Default != "" && !metaOpen:
			// 目录默认值不在开放取值里：清掉，否则客户预选不出任何一项。
			a.meta.Default = ""
		}
		if a.meta.Default == "" {
			a.meta.Default = a.values[0].Value
		}
		// 把最终默认同步到取值标记上：后端 pickEnumValue 与用户侧预选都读它。
		for i := range a.values {
			a.values[i].Default = a.values[i].Value == a.meta.Default
		}
	}
	if len(a.values) > 1 && (meta.Widget == SpecWidgetRadio || meta.Widget == SpecWidgetQty) {
		// 目录说"单选/数量"但档位开放了多个离散值：按按钮组呈现（CPU 2核/4核/8核）。
		a.meta.Widget = SpecWidgetRadio
	}
	if hasGroup(a.values) {
		a.meta.Widget = SpecWidgetGroupSelect
	}
}

// optionValueHelp 取某取值的一句话含义：来自目录项的 OptionsHelp（值 → 含义）。
// 特殊值（-1 不开通 / 0 不限量 / auto 随机端口）光看取值看不出含义，必须带给客户。
func optionValueHelp(meta SpecOptionMeta, value string) string {
	if strings.TrimSpace(meta.OptionsHelp) == "" {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(meta.OptionsHelp), &m); err != nil {
		return ""
	}
	return strings.TrimSpace(m[value])
}

// applyOverride 建品页就地覆盖：以请求为准（取值、加价、控件、必选、默认）。
func (a *optionAccumulator) applyOverride(ov dto.ProductOptionOverride) {
	a.explicit = true
	if strings.TrimSpace(ov.Label) != "" {
		a.meta.Label = ov.Label
	}
	if strings.TrimSpace(ov.Widget) != "" {
		a.meta.Widget = ov.Widget
	}
	a.meta.Required = ov.Required
	if strings.TrimSpace(ov.Unit) != "" {
		a.meta.Unit = ov.Unit
	}
	if strings.TrimSpace(ov.Help) != "" {
		a.meta.Help = ov.Help
	}
	if ov.SortOrder != 0 {
		a.meta.SortOrder = ov.SortOrder
	}
	if ov.MinValue != nil {
		a.meta.MinValue = ov.MinValue
	}
	if ov.MaxValue != nil {
		a.meta.MaxValue = ov.MaxValue
	}
	if d := strings.TrimSpace(ov.Default); d != "" {
		a.meta.Default = d
	}
	// 数量型（带宽/数据盘/系统盘/IP 数）：只有区间、没有离散取值。
	//
	// 这里必须把 isRange 标上（实测故障）：建品页把这类项放在 option_overrides 里传过来时
	// 不带 values，而 buildGroupsFromAccumulators 的保留条件是
	// 「有取值 或 isRange」—— 不标 isRange 就会被当成"没有取值的单选型"整项丢弃，
	// 于是生成的商品缺失全部范围型配置项，下单表单里也就没有这些可填项。
	// 判据用 min/max 是否存在：applyOverride 只在这两个字段有值时才设置它们。
	if a.meta.MinValue != nil || a.meta.MaxValue != nil {
		a.isRange = true
		a.meta.Widget = SpecWidgetQty
	}
	if len(ov.Values) == 0 {
		return
	}
	// 覆盖时整体替换取值列表（运营删掉的取值不应残留）。
	a.values = a.values[:0]
	a.seen = map[string]int{}
	for _, v := range ov.Values {
		val := strings.TrimSpace(v.Value)
		if val == "" {
			continue
		}
		if a.seen == nil {
			a.seen = map[string]int{}
		}
		if _, dup := a.seen[val]; dup {
			continue
		}
		a.seen[val] = len(a.values)
		a.values = append(a.values, optionValue{
			Value:   val,
			Label:   firstNonEmptyStr(strings.TrimSpace(v.Label), val),
			Group:   strings.TrimSpace(v.Group),
			Help:    strings.TrimSpace(v.Help),
			Default: v.Default,
			Hidden:  v.Hidden,
			Monthly: v.PriceMonthly,
			Annual:  v.PriceAnnually,
			Quarter: v.PriceQuarterly,
			Onetime: v.PriceOnetime,
		})
		if v.Default && a.meta.Default == "" {
			a.meta.Default = val
		}
	}
	if hasGroup(a.values) && a.meta.Widget != SpecWidgetQty {
		a.meta.Widget = SpecWidgetGroupSelect
	}
}

func (a *optionAccumulator) seenEnsure() {
	if a.seen == nil {
		a.seen = map[string]int{}
	}
}

// toConfigOption 转成 config_groups 里的一个 option（结构对齐上游 config_groups）。
func (a *optionAccumulator) toConfigOption() map[string]interface{} {
	optionType := 1 // 下拉
	widget := a.meta.Widget
	switch widget {
	case SpecWidgetRadio:
		optionType = 2 // 单选
	case SpecWidgetBool:
		optionType = 3 // 开关
	case SpecWidgetQty:
		optionType = 4 // 数量
	case SpecWidgetGroupSelect:
		optionType = 1
	}
	opt := map[string]interface{}{
		"option_name":   a.meta.Label,
		"option_type":   optionType,
		"source":        "self",
		"source_key":    a.meta.OptionKey,
		"provider_type": a.meta.ProviderType,
		"option_key":    a.meta.OptionKey,
		"widget":        widget,
		"required":      a.meta.Required,
		"default_value": a.meta.Default,
		"widget_group":  a.meta.GroupName,
		"min_value":     a.meta.MinValue,
		"max_value":     a.meta.MaxValue,
		"unit":          a.meta.Unit,
		"help":          a.meta.Help,
		"options_help":  a.meta.OptionsHelp,
		"sort_order":    a.meta.SortOrder,
		"hidden":        0,
	}
	subs := make([]map[string]interface{}, 0, len(a.values))
	for _, v := range a.values {
		hidden := 0
		if v.Hidden {
			hidden = 1
		}
		subs = append(subs, map[string]interface{}{
			"option_name": v.Label,
			"source":      "self",
			"source_key":  v.Value,
			"group_label": v.Group,
			"help":        v.Help,
			"is_default":  v.Default,
			"hidden":      hidden,
			"pricings": []map[string]interface{}{{
				"monthly":   v.Monthly,
				"annually":  v.Annual,
				"quarterly": v.Quarter,
				"onetime":   v.Onetime,
			}},
		})
	}
	// 数量型没有离散 sub：用一个占位 sub，取值范围由 min/max 表达。
	//
	// 占位 sub 的名字只用于后端回读，不该出现在客户界面（数量型靠区间渲染步进器）。
	// 取配置项本身的名称即可；客户端不会把它当可选取值展示。
	if widget == SpecWidgetQty && len(subs) == 0 {
		subs = append(subs, map[string]interface{}{
			"option_name": firstNonEmptyStr(a.meta.Label, a.meta.OptionKey),
			"source":      "self",
			"source_key":  "",
			"hidden":      0,
			"pricings": []map[string]interface{}{{
				"monthly": 0.0, "annually": 0.0, "quarterly": 0.0, "onetime": 0.0,
			}},
		})
	}
	opt["sub"] = subs
	return opt
}

// labelForValue 取某取值的中文展示名：优先目录取值库里的标签，其次原值。
func labelForValue(meta SpecOptionMeta, value string) string {
	for _, v := range meta.Values {
		if v.Value == value && strings.TrimSpace(v.Label) != "" {
			return v.Label
		}
	}
	return value
}

// groupForValue 取某取值所属分组：优先取值自带的 group_label，其次档位声明，再次目录取值。
func groupForValue(meta SpecOptionMeta, value, fallback string) string {
	for _, v := range meta.Values {
		if v.Value == value && strings.TrimSpace(v.GroupLabel) != "" {
			return v.GroupLabel
		}
	}
	return strings.TrimSpace(fallback)
}

// hasGroup 判断取值列表里是否存在分组标签（决定是否渲染分组下拉）。
func hasGroup(values []optionValue) bool {
	for _, v := range values {
		if strings.TrimSpace(v.Group) != "" {
			return true
		}
	}
	return false
}
