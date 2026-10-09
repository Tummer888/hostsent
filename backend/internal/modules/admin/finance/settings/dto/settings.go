// Package dto 提供财务参数子域的数据传输结构。
//
// 财务参数只列「运行时真正被读取」的键，按消费域分组展示；每个参数都带上
// usage（生效点），改完能在页面上直接核对它到底影响哪个接口 —— 历史「财务配置」
// 页曾写 5 个 finance_* 键但整仓零引用，是本轮要杜绝的反模式。
package dto

// SettingItem 单个财务参数项。
type SettingItem struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	ValueType    string `json:"value_type"` // bool/number
	Value        string `json:"value"`
	DefaultValue string `json:"default_value"`
	// Usage 该参数在哪里生效（写清消费点，便于核对不是装饰表单）。
	Usage string  `json:"usage"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

// SettingGroup 参数分组：同一消费域的键放在一起（财务口径 / 返现参数）。
type SettingGroup struct {
	Key   string        `json:"key"`   // 落库的 config_group
	Label string        `json:"label"` // 分组标题
	Hint  string        `json:"hint"`  // 分组说明
	Items []SettingItem `json:"items"`
}

// SettingsListResponse 财务参数列表（按分组）。
type SettingsListResponse struct {
	Groups []SettingGroup `json:"groups"`
}

// SettingsUpdateRequest 财务参数保存请求：key → 原文值。
type SettingsUpdateRequest struct {
	Items map[string]string `json:"items"`
}
