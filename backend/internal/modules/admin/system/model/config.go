// Package model 定义系统配置模块的数据模型。
package model

import "time"

// 值类型常量：配置值统一以字符串存储，value_type 描述其语义类型。
const (
	ValueTypeString = "string" // 字符串
	ValueTypeBool   = "bool"   // 布尔（true/false）
	ValueTypeInt    = "int"    // 整数
	ValueTypeJSON   = "json"   // JSON 文本
)

// 状态常量：disabled 的配置项不参与业务读取。
const (
	StatusActive   = "active"   // 启用
	StatusDisabled = "disabled" // 禁用
)

// 配置分组常量。
const (
	ConfigGroupBase     = "base"     // 基础配置
	ConfigGroupSecurity = "security" // 安全配置
	ConfigGroupRegister = "register" // 注册配置
	ConfigGroupNotify   = "notify"   // 消息模板

	// 兼容历史分组
	ConfigGroupSite    = "site"    // 站点配置
	ConfigGroupBilling = "billing" // 计费配置
	ConfigGroupFeature = "feature" // 功能开关
	ConfigGroupOrder   = "order"   // 订单
)

// SystemConfig 系统配置项。
type SystemConfig struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`                        // 主键 ID
	ConfigKey   string    `gorm:"column:config_key;size:128;not null;uniqueIndex"` // 配置键（全局唯一）
	ConfigValue string    `gorm:"column:config_value;type:text"`                   // 配置值（统一以字符串存储）
	ValueType   string    `gorm:"column:value_type;size:20;default:string"`        // 值类型：string/bool/int/json
	Group       string    `gorm:"column:config_group;size:64;index"`               // 配置分组
	Description string    `gorm:"column:description;size:255"`                     // 配置描述
	SortOrder   int       `gorm:"column:sort_order;default:0"`                     // 排序权重
	Status      string    `gorm:"size:32;not null;default:active"`                 // 状态：active/disabled
	CreatedAt   time.Time `gorm:"autoCreateTime"`                                  // 创建时间
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`                                  // 更新时间
}

// TableName 指定表名
func (SystemConfig) TableName() string {
	return "system_configs"
}
