// Package dto 定义系统配置模块的数据传输结构。
package dto

import "time"

// ConfigListQuery 配置项分页查询参数
type ConfigListQuery struct {
	Group    string `form:"group"`     // 配置分组（精确过滤，空为全部）
	Keyword  string `form:"keyword"`   // 关键字（匹配配置键/描述，模糊查询）
	Page     int    `form:"page"`      // 页码（从 1 开始）
	PageSize int    `form:"page_size"` // 每页条数
}

// ConfigCreateRequest 创建配置项请求
type ConfigCreateRequest struct {
	ConfigKey   string `json:"config_key" binding:"required"`                             // 配置键（全局唯一）
	ConfigValue string `json:"config_value" binding:"required"`                           // 配置值
	ValueType   string `json:"value_type" binding:"omitempty,oneof=string bool int json"` // 值类型，空为 string
	Group       string `json:"config_group"`                                              // 配置分组
	Description string `json:"description"`                                               // 配置描述
	SortOrder   int    `json:"sort_order"`                                                // 排序权重
	Status      string `json:"status" binding:"omitempty,oneof=active disabled"`          // 状态，空为 active
}

// ConfigUpdateRequest 更新配置项请求（配置键创建后不可修改）
type ConfigUpdateRequest struct {
	ConfigValue string `json:"config_value"`                                              // 配置值
	ValueType   string `json:"value_type" binding:"omitempty,oneof=string bool int json"` // 值类型
	Group       string `json:"config_group"`                                              // 配置分组
	Description string `json:"description"`                                               // 配置描述
	SortOrder   int    `json:"sort_order"`                                                // 排序权重
	Status      string `json:"status" binding:"omitempty,oneof=active disabled"`          // 状态
}

// ConfigSaveItem 分组批量保存的单项配置。
type ConfigSaveItem struct {
	ConfigKey   string `json:"config_key" binding:"required"`                             // 配置键（全局唯一）
	ConfigValue string `json:"config_value"`                                              // 配置值
	ValueType   string `json:"value_type" binding:"omitempty,oneof=string bool int json"` // 值类型，空为 string
	Group       string `json:"config_group"`                                              // 配置分组
	Description string `json:"description"`                                               // 配置描述
	SortOrder   int    `json:"sort_order"`                                                // 排序权重
	Status      string `json:"status" binding:"omitempty,oneof=active disabled"`          // 状态，空为 active
}

// ConfigBatchUpsertRequest 分组批量保存配置请求（按 config_key 幂等 upsert）。
type ConfigBatchUpsertRequest struct {
	Group string           `json:"config_group" binding:"required"`             // 目标分组
	Items []ConfigSaveItem `json:"items" binding:"required,min=1,dive"`         // 待保存的配置项
}

// ConfigInfo 配置项信息（响应体，字段与前端 snake_case 保持一致）
type ConfigInfo struct {
	ID          uint64    `json:"id"`           // 主键 ID
	ConfigKey   string    `json:"config_key"`   // 配置键
	ConfigValue string    `json:"config_value"` // 配置值
	ValueType   string    `json:"value_type"`   // 值类型：string/bool/int/json
	Group       string    `json:"config_group"` // 配置分组
	Description string    `json:"description"`  // 配置描述
	SortOrder   int       `json:"sort_order"`   // 排序权重
	Status      string    `json:"status"`       // 状态：active/disabled
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// ConfigMeta 配置列表分页元信息
type ConfigMeta struct {
	Page     int   `json:"page"`      // 当前页码
	PageSize int   `json:"page_size"` // 每页条数
	Total    int64 `json:"total"`     // 总记录数
}

// ConfigListResponse 配置项分页列表响应（items + meta 结构，与安全模块列表风格一致）
type ConfigListResponse struct {
	Items []ConfigInfo `json:"items"` // 当前页数据
	Meta  ConfigMeta   `json:"meta"`  // 分页元信息
}
