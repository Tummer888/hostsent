package model

import "time"

// 平台配置项目录与平台取值库（T4.5 规格配置化）。
//
// 两层分工：
//   - ProviderOptionSpec 声明「这个平台支持哪些配置项」——适配器在 Go 里声明，
//     首次同步进本表，运营可在后台改标签/默认值/必选，也可新增自定义项；
//   - PlatformOptionValue 存「某个配置项允许取哪些值」——镜像/区域/节点/存储来自
//     平台接口拉取，CPU 档/内存档/枚举等来自适配器声明或运营批量导入。
//
// 规格模板（配置档）通过 option_selections 引用这两层里的键与取值。

const (
	// OptionSpecSourceAdapter 适配器内置声明（同步导入）。
	OptionSpecSourceAdapter = "adapter"
	// OptionSpecSourceCustom 运营在后台新增的自定义配置项。
	OptionSpecSourceCustom = "custom"
)

const (
	// OptionValueOriginPlatform 由平台接口拉取入库。
	OptionValueOriginPlatform = "platform"
	// OptionValueOriginManual 运营手工新增或批量导入。
	OptionValueOriginManual = "manual"
)

const (
	// OptionValueActive 启用（可被配置档勾选、可下发给客户）。
	OptionValueActive = "active"
	// OptionValueOffline 停用（保留历史引用，不再出现在可选项里）。
	OptionValueOffline = "offline"
)

// ProviderOptionSpec 平台可配置项声明。
type ProviderOptionSpec struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	ProviderType string    `gorm:"column:provider_type;size:64;not null;uniqueIndex:uk_provider_option_specs"`
	OptionKey    string    `gorm:"column:option_key;size:64;not null;uniqueIndex:uk_provider_option_specs"`
	Label        string    `gorm:"size:128;not null;default:''"`
	GroupName    string    `gorm:"column:group_name;size:64;not null;default:''"`
	Required     bool      `gorm:"not null;default:false"`
	DefaultValue string    `gorm:"column:default_value;size:255;not null;default:''"`
	Widget       string    `gorm:"size:32;not null;default:'select'"`
	ValueSource  string    `gorm:"column:value_source;size:32;not null;default:'static'"`
	Options      *string   `gorm:"type:jsonb"` // 静态枚举 [{label,value}]；nil 落 NULL（空串不是合法 jsonb）
	MinValue     *float64  `gorm:"column:min_value;type:numeric(16,4)"`
	MaxValue     *float64  `gorm:"column:max_value;type:numeric(16,4)"`
	StepValue    *float64  `gorm:"column:step_value;type:numeric(16,4)"`
	Unit         string    `gorm:"size:16;not null;default:''"`
	Help         string    `gorm:"type:text;not null;default:''"`
	MultiValue   bool      `gorm:"column:multi_value;not null;default:false"`
	Hidden       bool      `gorm:"not null;default:false"`
	SortOrder    int       `gorm:"column:sort_order;not null;default:0"`
	Source       string    `gorm:"size:16;not null;default:'adapter'"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (ProviderOptionSpec) TableName() string { return "provider_option_specs" }

// PlatformOptionValue 平台取值库的一行：某平台某配置项的一个可选值。
type PlatformOptionValue struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	ProviderType string    `gorm:"column:provider_type;size:64;not null;uniqueIndex:uk_platform_option_values"`
	OptionKey    string    `gorm:"column:option_key;size:64;not null;uniqueIndex:uk_platform_option_values"`
	Value        string    `gorm:"size:255;not null;uniqueIndex:uk_platform_option_values"`
	Label        string    `gorm:"size:255;not null;default:''"`
	ParentValue  string    `gorm:"column:parent_value;size:255;not null;default:'';uniqueIndex:uk_platform_option_values"`
	GroupLabel   string    `gorm:"column:group_label;size:128;not null;default:''"`
	Status       string    `gorm:"size:16;not null;default:'active'"`
	Origin       string    `gorm:"size:16;not null;default:'manual'"`
	SortOrder    int       `gorm:"column:sort_order;not null;default:0"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (PlatformOptionValue) TableName() string { return "platform_option_values" }
