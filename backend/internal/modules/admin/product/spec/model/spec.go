// Package model 提供规格管理（spec）子域的数据模型。
package model

import "time"

// 规格族分类
const (
	SpecFamilyGeneral string = "general" // 通用型
	SpecFamilyCompute string = "compute" // 计算型（CPU密集）
	SpecFamilyMemory  string = "memory"  // 内存型（内存密集）
	SpecFamilyStorage string = "storage" // 存储型（IO密集）
	SpecFamilyGPU     string = "gpu"     // GPU型
)

// 规格模板状态
const (
	SpecTemplateDisabled int = 0 // 停用
	SpecTemplateEnabled  int = 1 // 启用
)

// 规格模板来源（product_spec_templates.source，T2.5）
const (
	SpecTemplateSourceSelf     = "self"     // 平台自建
	SpecTemplateSourceImported = "imported" // 由上游规格归一而来
)

// 机型（product_spec_templates.machine_kind，doc122）：决定档位的平台参数基线。
//
// 魔方云两条开通链路共用 POST /clouds，靠参数取值区分机型；这个字段把这个隐含
// 语义显式化，用于预置档位分组与保存时的互斥校验（NAT 必须 ip_num=0 且有转发能力，
// VPS 必须 ip_num>=1 且无转发能力）。
const (
	// MachineKindVPS 独立 IP 机型：分配独立公网 IP，nat_*_limit=-1。
	MachineKindVPS = "vps"
	// MachineKindNAT NAT 机型：不分配独立 IP，经节点转发地址访问。
	MachineKindNAT = "nat"
	// MachineKindUnset 未标注：存量模板的缺省值，校验跳过。
	MachineKindUnset = ""
)

// SpecMapping 状态
const (
	SpecMappingUnmapped int = 0 // 待映射
	SpecMappingMapped   int = 1 // 已映射
)

// SpecTemplate 配置档：面向某个对接平台的一组可选规格取值。
//
// 自营链路的用法：运营在「平台配置项」里选好平台（魔方云），勾选每个参数允许的取值
// （CPU 2核/4核/8核、内存 4G/8G、一组镜像…），存成一个配置档（如「2核4G」）；
// 新建商品时勾选档位即生成 1 个 SKU + N 个客户可选配置项。
//
// 兼容说明：
//   - CPU/Memory/Disk/Bandwidth/OS 五个结构化字段保留，作为档位的"基线值"
//     （生成 SKU 时取它作为默认下发值），并为老模板（无 option_selections）兜底；
//   - SpecFamily 保留列但不再写入（默认 general），UI 与 SKU 编码都不再使用；
//   - SpecValues/PlatformParams 原样保留：前者是原子取值 JSON，后者是平台写参数 JSON。
type SpecTemplate struct {
	ID          uint64  `gorm:"primaryKey;autoIncrement"`
	Name        string  `gorm:"size:100;not null"`                                  // 档位名，如"2核4G"
	SpecFamily  string  `gorm:"column:spec_family;size:20;default:'general';index"` // 已弃用：保留列，不再写入
	CPU         int     `gorm:"column:cpu;default:1"`                               // 基线 CPU 核数
	Memory      float64 `gorm:"column:memory;type:decimal(8,2);default:1"`          // 基线内存 GB
	Disk        int     `gorm:"column:disk;default:40"`                             // 基线系统盘 GB
	DiskType    string  `gorm:"column:disk_type;size:20;default:'ssd'"`             // 磁盘类型
	Bandwidth   int     `gorm:"column:bandwidth;default:0"`                         // 基线带宽 Mbps，0 表示不限
	OS          string  `gorm:"column:os;size:50"`                                  // 基线操作系统
	Description string  `gorm:"type:text"`                                          // 适用场景描述
	Price       float64 `gorm:"column:price;type:decimal(10,2)"`                    // 参考售价
	SortOrder   int     `gorm:"column:sort_order;default:0"`                        // 排序
	Status      int     `gorm:"default:1;index"`                                    // 状态
	// SpecValues 原子 key → 取值 JSON（如 {"compute.cpu":2,"placement.region":"1"}）；
	// 留空时由 CPU/Memory/Disk/Bandwidth/OS/DiskType 推导（specatom.ToMap 口径）。
	// 指针语义：nil 落 NULL，空串不是合法 jsonb（落库报 22P02）。
	SpecValues *string `gorm:"column:spec_values;type:jsonb"`
	// PlatformParams 平台写参数 JSON（如 {"area":"1","node":"2","os":"12","store":"2"}）；
	// 生成 SKU 时原样写入 spec_bindings.platform_params 并置 confirmed。nil 落 NULL。
	PlatformParams *string `gorm:"column:platform_params;type:jsonb"`
	// ProviderType 档位归属的平台（dcimcloud / zjmf / ...）；空表示未绑定平台（旧模板）。
	ProviderType string `gorm:"column:provider_type;size:64;not null;default:'';index"`
	// MachineKind 机型（doc122）：vps 独立 IP / nat NAT 机型 / '' 未标注（存量模板）。
	// 机型由 platform_params 的取值决定，此列只做归类与保存时互斥校验。
	// 索引名显式固定：GORM 默认命名与迁移里的 DDL 不一致会多建一条冗余索引。
	MachineKind string `gorm:"column:machine_kind;size:16;not null;default:'';index:idx_spec_templates_machine_kind"`
	// OptionSelections 每参数勾选的可选值（T4.5），形如：
	//   {"cpu":{"values":["2","4"]},"bw":{"range":[1,100],"default":"10"}}
	// 建品时据此生成客户可选配置项；nil 表示按基线值生成单一 SKU（旧行为）。
	OptionSelections *string `gorm:"column:option_selections;type:jsonb"`
	// NameTemplate / DescriptionTemplate 商品名与描述的渲染模板，
	// 如 "{cpu}核{memory}G {os}"，用档位展开后的取值填充。
	NameTemplate        string `gorm:"column:name_template;size:255;not null;default:''"`
	DescriptionTemplate string `gorm:"column:description_template;type:text;not null;default:''"`
	// Source 模板来源：self 自建 / imported 上游归一（T2.5）。
	Source    string    `gorm:"column:source;size:16;default:'self'"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SpecTemplate) TableName() string {
	return "product_spec_templates"
}

// SpecMapping 规格映射：将上游（魔方云/阿里云/腾讯云）规格映射为平台规格。
type SpecMapping struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	ProviderType   string    `gorm:"column:provider_type;size:20;index"`              // 上游类型
	UpstreamSpecID string    `gorm:"column:upstream_spec_id;size:100;not null;index"` // 上游规格 ID
	UpstreamName   string    `gorm:"column:upstream_name;size:200"`                   // 上游规格名称
	PlatformSpecID uint64    `gorm:"column:platform_spec_id;index"`                   // 映射的平台规格模板 ID
	PlatformName   string    `gorm:"column:platform_name;size:200"`                   // 映射的平台规格名称
	CPU            int       `gorm:"column:cpu;default:0"`                            // 上游 CPU
	Memory         float64   `gorm:"column:memory;type:decimal(8,2);default:0"`       // 上游内存
	Disk           int       `gorm:"column:disk;default:0"`                           // 上游磁盘
	Status         int       `gorm:"default:0;index"`                                 // 0 待映射 1 已映射
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SpecMapping) TableName() string {
	return "spec_mappings"
}
