// Package dto 提供规格管理（spec）子域的数据传输结构。
package dto

import "encoding/json"

// ===== 平台配置项目录（T4.5 规格配置化）=====

// OptionValueItem 一个可选值（静态枚举 / 平台取值 / 手工取值共用）。
type OptionValueItem struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	GroupLabel  string `json:"group_label,omitempty"`  // 分组标签（Ubuntu/Windows/CentOS）
	ParentValue string `json:"parent_value,omitempty"` // 上级取值（节点/存储挂区域）
	Status      string `json:"status,omitempty"`       // active / offline
	Origin      string `json:"origin,omitempty"`       // platform / manual
}

// OptionSpecInfo 平台可配置项（含取值库中已入库的可选值）。
type OptionSpecInfo struct {
	ID           uint64          `json:"id"`
	ProviderType string          `json:"provider_type"`
	OptionKey    string          `json:"option_key"`
	Label        string          `json:"label"`
	GroupName    string          `json:"group_name"`
	Required     bool            `json:"required"`
	DefaultValue string          `json:"default_value"`
	Widget       string          `json:"widget"`
	ValueSource  string          `json:"value_source"`
	Options      json.RawMessage `json:"options"`
	MinValue     *float64        `json:"min_value"`
	MaxValue     *float64        `json:"max_value"`
	StepValue    *float64        `json:"step_value"`
	Unit         string          `json:"unit"`
	Help         string          `json:"help"`
	MultiValue   bool            `json:"multi_value"`
	Hidden       bool            `json:"hidden"`
	SortOrder    int             `json:"sort_order"`
	Source       string          `json:"source"`
	// Values 取值库中该配置项的可选值（仅当 ValueSource 不是 static 时返回）。
	Values []OptionValueItem `json:"values,omitempty"`
}

// OptionSpecRequest 新增/修改平台可配置项。
type OptionSpecRequest struct {
	ProviderType string          `json:"provider_type"`
	OptionKey    string          `json:"option_key" binding:"required"`
	Label        string          `json:"label"`
	GroupName    string          `json:"group_name"`
	Required     bool            `json:"required"`
	DefaultValue string          `json:"default_value"`
	Widget       string          `json:"widget"`
	ValueSource  string          `json:"value_source"`
	Options      json.RawMessage `json:"options"`
	MinValue     *float64        `json:"min_value"`
	MaxValue     *float64        `json:"max_value"`
	StepValue    *float64        `json:"step_value"`
	Unit         string          `json:"unit"`
	Help         string          `json:"help"`
	MultiValue   bool            `json:"multi_value"`
	Hidden       bool            `json:"hidden"`
	SortOrder    int             `json:"sort_order"`
}

// OptionCatalogQuery 目录查询参数：按平台类型或渠道 ID（渠道 ID 用于未知平台时反查类型）。
type OptionCatalogQuery struct {
	ProviderType string `form:"provider_type" json:"provider_type"`
	ProviderID   uint64 `form:"provider_id" json:"provider_id"`
}

// OptionCatalogSyncRequest 把适配器声明的目录幂等导入指定平台。
type OptionCatalogSyncRequest struct {
	ProviderType string `json:"provider_type" binding:"required"`
}

// OptionCatalogSyncResult 导入结果：新增/跳过条数。
type OptionCatalogSyncResult struct {
	ProviderType string `json:"provider_type"`
	Declared     int    `json:"declared"`
	Created      int    `json:"created"`
	Skipped      int    `json:"skipped"`
}

// OptionValueQuery 取值库查询参数。
type OptionValueQuery struct {
	ProviderType string `form:"provider_type" json:"provider_type"`
	ProviderID   uint64 `form:"provider_id" json:"provider_id"`
	OptionKey    string `form:"option_key" json:"option_key"`
	// IncludeOffline 是否包含已停用取值（默认不返回）。
	IncludeOffline bool `form:"include_offline" json:"include_offline"`
}

// OptionValueUpsertRequest 新增/更新一条平台取值。
type OptionValueUpsertRequest struct {
	ProviderType string `json:"provider_type"`
	OptionKey    string `json:"option_key" binding:"required"`
	Value        string `json:"value" binding:"required"`
	Label        string `json:"label"`
	ParentValue  string `json:"parent_value"`
	GroupLabel   string `json:"group_label"`
	Status       string `json:"status"`
	SortOrder    int    `json:"sort_order"`
}

// OptionValueImportRequest 批量导入取值（镜像类型直接入库的入口）。
type OptionValueImportRequest struct {
	ProviderType string `json:"provider_type"`
	OptionKey    string `json:"option_key" binding:"required"`
	// Replace 为 true 时先停用该配置项下来源为 platform 的旧取值（镜像刷新场景）。
	Replace bool              `json:"replace"`
	Items   []OptionValueItem `json:"items" binding:"required"`
}

// OptionValueImportResult 批量导入结果。
type OptionValueImportResult struct {
	OptionKey string `json:"option_key"`
	Created   int    `json:"created"`
	Updated   int    `json:"updated"`
	Offlined  int    `json:"offlined"`
}

// OptionValueRefreshRequest 从平台实时刷新取值库。
type OptionValueRefreshRequest struct {
	ProviderType string `json:"provider_type"`
	ProviderID   uint64 `json:"provider_id" binding:"required"`
	// OptionKeys 要刷新的配置项参数名（os/area/node/store）；留空则由平台页面按需指定。
	OptionKeys []string `json:"option_keys"`
}
