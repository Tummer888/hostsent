package db

import (
	"fmt"

	config "hostsent/backend/internal/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func New(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Seed(db *gorm.DB) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return SeedDefaults(db, *cfg)
}
