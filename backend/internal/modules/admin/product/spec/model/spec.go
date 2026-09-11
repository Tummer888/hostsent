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

// SpecMapping 状态
const (
	SpecMappingUnmapped int = 0 // 待映射
	SpecMappingMapped   int = 1 // 已映射
)

// SpecTemplate 规格模板：面向不同场景（计算/内存/存储等）的规格预设。
type SpecTemplate struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"size:100;not null"`                                  // 规格名称，如"通用型-2核4G"
	SpecFamily  string    `gorm:"column:spec_family;size:20;default:'general';index"` // 规格族
	CPU         int       `gorm:"column:cpu;default:1"`                               // CPU 核数
	Memory      float64   `gorm:"column:memory;type:decimal(8,2);default:1"`          // 内存 GB
	Disk        int       `gorm:"column:disk;default:40"`                             // 系统盘 GB
	DiskType    string    `gorm:"column:disk_type;size:20;default:'ssd'"`             // 磁盘类型
	Bandwidth   int       `gorm:"column:bandwidth;default:0"`                         // 带宽 Mbps，0 表示不限
	OS          string    `gorm:"column:os;size:50"`                                  // 操作系统
	Description string    `gorm:"type:text"`                                          // 适用场景描述
	Price       float64   `gorm:"column:price;type:decimal(10,2)"`                    // 参考售价
	SortOrder   int       `gorm:"column:sort_order;default:0"`                        // 排序
	Status      int       `gorm:"default:1;index"`                                    // 状态
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
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
