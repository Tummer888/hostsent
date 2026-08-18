package db

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/model"
	"hostsent/backend/internal/pkg/config"
)

func New(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func AutoMigrate(database *gorm.DB) error {
	return database.AutoMigrate(&model.User{})
}

func Seed(database *gorm.DB) error {
	var count int64
	if err := database.Model(&model.User{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return database.Create(&model.User{
		Username:     "admin",
		Email:        "admin@example.com",
		PasswordHash: string(passwordHash),
		Status:       "active",
	}).Error
}
