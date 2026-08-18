package model

type UserRole struct {
	UserID uint64 `gorm:"primaryKey"`
	RoleID uint64 `gorm:"primaryKey"`
}

func (UserRole) TableName() string {
	return "user_roles"
}
