package db

import (
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/model"
)

func AutoMigrateModels(database *gorm.DB) error {
	return database.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.Permission{},
		&model.RolePermission{},
		&model.UserRole{},
	)
}
