// Package model 提供规格契约（spec contract）子域的数据模型（P2/T2.5）。
//
// 与既有 product/spec（规格模板/映射）区分：本文件承载"规格原子字典 + 外部规格快照 +
// 双向绑定"，是对接 N 家异构上游/平台的基础设施（docs/实施计划/16 §3）。
package model

import "time"

// 规格原子适用链路。
const (
	AtomAppliesSelf     = "self"
	AtomAppliesUpstream = "upstream"
	AtomAppliesBoth     = "both"
)

// SpecAtom 规格原子：平台无关的最小规格维度。
type SpecAtom struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	Key            string    `gorm:"column:key;size:64;not null;uniqueIndex:uk_spec_atoms_key"`
	Name           string    `gorm:"size:64;not null"`
	Unit           string    `gorm:"size:16"`
	ValueType      string    `gorm:"column:value_type;size:16;not null;default:int"`
	EnumValues     string    `gorm:"column:enum_values;type:jsonb"`
	MinValue       *float64  `gorm:"column:min_value;type:numeric(16,4)"`
	MaxValue       *float64  `gorm:"column:max_value;type:numeric(16,4)"`
	StepValue      *float64  `gorm:"column:step_value;type:numeric(16,4)"`
	Required       bool      `gorm:"not null;default:false"`
	Configurable   bool      `gorm:"not null;default:false"`
	AppliesTo      string    `gorm:"column:applies_to;size:16;not null;default:both"`
	PlatformFields string    `gorm:"column:platform_fields;type:jsonb"` // 各平台读写字段名映射
	Description    string    `gorm:"type:text"`
	SortOrder      int       `gorm:"column:sort_order;not null;default:0"`
	Status         int       `gorm:"not null;default:1"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SpecAtom) TableName() string { return "spec_atoms" }

// 外部规格类型与状态。
const (
	ExternalKindFlavor   = "flavor"
	ExternalKindPlan     = "plan"
	ExternalKindTemplate = "template"
	ExternalKindPoolSpec = "pool_spec"

	ExternalStatusActive  = "active"
	ExternalStatusOffline = "offline" // 上游下架，软删除
)

// ExternalSpec 外部规格快照：上游/平台原始规格登记，与内部标准规格解耦。
type ExternalSpec struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	ProviderID   uint64     `gorm:"column:provider_id;index"`
	ProviderType string     `gorm:"column:provider_type;size:50;not null"`
	ExternalID   string     `gorm:"column:external_id;size:128;not null"`
	ExternalName string     `gorm:"column:external_name;size:200"`
	ExternalKind string     `gorm:"column:external_kind;size:24;not null;default:flavor"`
	Raw          string     `gorm:"type:jsonb"`    // 原始字段快照（可据此重算映射）
	Normalized   string     `gorm:"type:jsonb"`    // 归一后的原子取值（允许残缺）
	Fingerprint  string     `gorm:"size:64;index"` // 归一特征哈希
	Status       string     `gorm:"size:16;not null;default:active"`
	SyncedAt     *time.Time `gorm:"column:synced_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ExternalSpec) TableName() string { return "external_specs" }

// 绑定方向、匹配方式与状态机。
const (
	BindingDirectionInbound  = "inbound"  // 用于归一展示/对外输出
	BindingDirectionOutbound = "outbound" // 用于实际开通参数

	BindingMatchExact  = "auto_exact"
	BindingMatchRange  = "auto_range"
	BindingMatchManual = "manual"

	BindingStatusUnmapped   = "unmapped"
	BindingStatusAutoMapped = "auto_mapped"
	BindingStatusConfirmed  = "confirmed"
	BindingStatusStale      = "stale"
)

// SpecBinding 外部规格 ↔ 内部标准规格的双向绑定（由 spec_mappings 演进）。
// 两条链路共用一张表：
//   - 代理链路：ExternalSpecID 指向 external_specs（上游规格镜像，§7.3）；
//   - 自营链路：ProductSpecID 指向 product_specs（我方 SKU，§7.2）。
//
// 二者至少有一个非空，方向均为 outbound（实际开通参数）时生效。
type SpecBinding struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement"`
	ExternalSpecID *uint64    `gorm:"column:external_spec_id;index"` // 外部规格（代理链路）
	ProductSpecID  *uint64    `gorm:"column:product_spec_id;index"`  // 商品 SKU（自营链路，T4.2）
	SpecTemplateID uint64     `gorm:"column:spec_template_id;index"`
	Direction      string     `gorm:"size:16;not null;default:outbound"`
	PlatformParams string     `gorm:"column:platform_params;type:jsonb"`
	MatchType      string     `gorm:"size:16;not null;default:manual"`
	Status         string     `gorm:"size:16;not null;default:unmapped"`
	Confidence     int        `gorm:"not null;default:0"`
	ConfirmedBy    uint64     `gorm:"column:confirmed_by"`
	ConfirmedAt    *time.Time `gorm:"column:confirmed_at"`
	Remark         string     `gorm:"size:255"`
	Priority       int        `gorm:"not null;default:0"`
	CreatedAt      time.Time  `gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SpecBinding) TableName() string { return "spec_bindings" }
