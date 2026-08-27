package model

import "time"

type Permission struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	ParentID  uint64    `gorm:"not null;default:0"`
	Name      string    `gorm:"size:64;not null"`
	Code      string    `gorm:"size:128;not null;uniqueIndex"`
	Type      string    `gorm:"size:32;not null"`
	Path      string    `gorm:"size:255"`
	Component string    `gorm:"size:255"`
	Icon      string    `gorm:"size:128"`
	SortOrder int       `gorm:"not null;default:0"`
	Status    string    `gorm:"size:32;not null;default:active"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (Permission) TableName() string {
	return "permissions"
}
