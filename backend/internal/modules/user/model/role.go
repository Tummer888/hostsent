package model

import "time"

type Role struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	Name      string    `gorm:"size:64;not null;uniqueIndex"`
	Code      string    `gorm:"size:64;not null;uniqueIndex"`
	Status    string    `gorm:"size:32;not null;default:active"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (Role) TableName() string {
	return "roles"
}
