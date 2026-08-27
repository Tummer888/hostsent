// Package model 提供上游提供商模块的数据模型。
package model

import (
	"time"

	"gorm.io/gorm"
)

// UpstreamProvider 上游提供商
type UpstreamProvider struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement"`
	Name         string         `gorm:"size:100;not null"`
	ProviderType string         `gorm:"column:provider_type;size:50;not null;index"`
	APIEndpoint  string         `gorm:"column:api_endpoint;size:255;not null"`
	APIKey       string         `gorm:"column:api_key;size:255;not null"`
	APISecret    string         `gorm:"column:api_secret;size:255;not null"`
	Region       string         `gorm:"size:50"`
	Status       int            `gorm:"default:1;index"`
	SyncEnabled  bool           `gorm:"column:sync_enabled;default:true"`
	SyncInterval int            `gorm:"column:sync_interval;default:3600"`
	LastSyncAt   *time.Time     `gorm:"column:last_sync_at"`
	TotalCPU     int            `gorm:"column:total_cpu;default:0"`
	TotalMemory  int            `gorm:"column:total_memory;default:0"`
	TotalDisk    int            `gorm:"column:total_disk;default:0"`
	UsedCPU      int            `gorm:"column:used_cpu;default:0"`
	UsedMemory   int            `gorm:"column:used_memory;default:0"`
	UsedDisk     int            `gorm:"column:used_disk;default:0"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (UpstreamProvider) TableName() string {
	return "upstream_providers"
}

// ResourcePool 资源池
type ResourcePool struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	ProviderID  uint64    `gorm:"column:provider_id;not null;uniqueIndex:idx_pool_provider_upstream"`
	UpstreamID  string    `gorm:"column:upstream_id;size:64;not null;uniqueIndex:idx_pool_provider_upstream"`
	Name        string    `gorm:"size:100;not null"`
	PoolType    string    `gorm:"column:pool_type;size:20"`
	TotalCPU    int       `gorm:"column:total_cpu;default:0"`
	TotalMemory int       `gorm:"column:total_memory;default:0"`
	TotalDisk   int       `gorm:"column:total_disk;default:0"`
	UsedCPU     int       `gorm:"column:used_cpu;default:0"`
	UsedMemory  int       `gorm:"column:used_memory;default:0"`
	UsedDisk    int       `gorm:"column:used_disk;default:0"`
	Status      int       `gorm:"default:1"`
	LastSyncAt  *time.Time `gorm:"column:last_sync_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ResourcePool) TableName() string {
	return "resource_pools"
}
