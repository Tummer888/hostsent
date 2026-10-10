package upstream

import "hostsent/backend/internal/pkg/integration"

// 平台配置项目录（T4.5 规格配置化）。
//
// 为什么要有它：适配器过去只以 FieldDictionary["write"] 暴露一串平台字段名，
// 没有标签、必选标记、默认值、可选值来源，后台无法据此渲染出"哪些参数可选、
// 每个参数能选哪些值"。这份声明把「平台支持哪些配置项」从口头/文档搬到代码里，
// 由适配器自己声明，再由配置档引用。
//
// 与 Field 的分工：
//   - Field 描述**渠道接入**表单（凭证/端点，含加密标记）；
//   - ConfigOptionSpec 描述**商品规格**参数（下单开通时下发到平台）。
const (
	// ConfigWidgetSelect 下拉（可带分组）。
	ConfigWidgetSelect = "select"
	// ConfigWidgetRadio 单选（按钮组）。
	ConfigWidgetRadio = "radio"
	// ConfigWidgetQty 数量（步进器），用 MinValue/MaxValue/Step 约束。
	ConfigWidgetQty = "qty"
	// ConfigWidgetBool 开关（是/否）。
	ConfigWidgetBool = "bool"
	// ConfigWidgetJSON 高级 JSON（多段值，如磁盘性能 a,b,c,d）。
	ConfigWidgetJSON = "json"
)

// 取值来源：决定后台从哪里拿"可选值"。
const (
	// ValueSourceStatic 静态枚举，取 Options。
	ValueSourceStatic = "static"
	// ValueSourceManual 平台面板上的 ID，需人工填写或批量入库。
	ValueSourceManual = "manual"
	// ValueSourceAreas 平台区域列表。
	ValueSourceAreas = "areas"
	// ValueSourceNodes 平台节点列表。
	ValueSourceNodes = "nodes"
	// ValueSourceStores 平台存储列表。
	ValueSourceStores = "stores"
	// ValueSourceImages 平台镜像列表（可下载的操作系统/应用镜像）。
	ValueSourceImages = "images"
)

// ConfigOptionSpec 平台单个可配置项的声明。
type ConfigOptionSpec struct {
	// Key 平台写参数名（与下发 /clouds 的字段名一致，如 cpu/memory/system_disk_size）。
	Key string `json:"key"`
	// Label 中文名，后台与用户侧展示。
	Label string `json:"label"`
	// Group 分组（基础配置 / 网络 / 高级），用于后台与用户侧分区展示。
	Group string `json:"group"`
	// Required 是否必填（平台不传会拒绝开通）。
	Required bool `json:"required"`
	// Default 未传递时平台的默认值（文档口径，原样展示，不代平台判断）。
	Default string `json:"default"`
	// Widget 控件类型，见 ConfigWidget*。
	Widget string `json:"widget"`
	// ValueSource 可选值来源，见 ValueSource*。
	ValueSource string `json:"value_source"`
	// Options 静态枚举取值（ValueSource=static 时使用）。
	Options []integration.FieldOption `json:"options,omitempty"`
	// MinValue/MaxValue/Step 数量型约束（Widget=qty）。
	MinValue *float64 `json:"min_value,omitempty"`
	MaxValue *float64 `json:"max_value,omitempty"`
	Step     *float64 `json:"step,omitempty"`
	// Unit 单位（核 / GB / Mbps / 个 / Mbps…）。
	Unit string `json:"unit,omitempty"`
	// Help 注释性标注：取值从哪来、怎么填、与其它参数的关系。
	Help string `json:"help,omitempty"`
	// OptionsHelp 取值含义表（JSON 对象：值 → 含义）。特殊值（-1/0/auto 等）从取值
	// 本身看不懂，必须把文档口径的说明挂到每个值上，后台与客户端才不用回头查文档。
	// 形如 {"-1":"不能创建快照","0":"不限量"}。
	OptionsHelp string `json:"options_help,omitempty"`
	// MultiValue 配置档里是否允许多选（如 CPU 2核/4核/8核 同时提供给客户选）。
	MultiValue bool `json:"multi_value"`
	// SortOrder 排序，0 表示按声明顺序。
	SortOrder int `json:"sort_order,omitempty"`
}

// OptionCatalogOf 返回某平台类型声明的配置项目录；未注册返回 nil（视为该平台不支持配置化规格）。
func OptionCatalogOf(platform string) []ConfigOptionSpec {
	d, ok := Descriptor(platform)
	if !ok {
		return nil
	}
	return d.OptionCatalog
}
