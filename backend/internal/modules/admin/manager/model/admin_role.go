package model

// AdminRole 管理员与角色的多对多关联（D2：一个员工可挂多个角色）。
// admins.role 字段降级为兼容/展示字段，鉴权一律以本表为准。
type AdminRole struct {
	AdminID uint64 `gorm:"column:admin_id;primaryKey"`
	RoleID  uint64 `gorm:"column:role_id;primaryKey"`
}

func (AdminRole) TableName() string {
	return "admin_roles"
}
