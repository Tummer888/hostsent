package model

type RolePermission struct {
	RoleID       uint64 `gorm:"primaryKey"`
	PermissionID uint64 `gorm:"primaryKey"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}
