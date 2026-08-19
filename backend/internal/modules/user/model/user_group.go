package model

import "time"

type UserGroup struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"size:64;not null;uniqueIndex"`
	Code        string    `gorm:"size:64;not null;uniqueIndex"`
	Description string    `gorm:"size:255"`
	Status      string    `gorm:"size:32;not null;default:active"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (UserGroup) TableName() string {
	return "user_groups"
}
