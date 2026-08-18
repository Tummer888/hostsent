package model

import "time"

type User struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	Username     string    `gorm:"size:64;not null;uniqueIndex"`
	Email        string    `gorm:"size:128;not null;uniqueIndex"`
	Phone        string    `gorm:"size:32"`
	PasswordHash string    `gorm:"column:password_hash;size:255;not null"`
	Status       string    `gorm:"size:32;not null;default:active"`
	Role         string    `gorm:"-"`
	Roles        []string  `gorm:"-"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (User) TableName() string {
	return "users"
}
