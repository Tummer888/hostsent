package model

// 角色作用域：隔离后台员工角色与客户侧角色（与 admin/user/account/model 同口径）。
// 在员工域内重复声明是为了避免 manager 包反向依赖用户域模型。
const (
	RoleScopeAdmin = "admin"
	RoleScopeUser  = "user"
)

// AdminRole 管理员与角色的多对多关联（D2：一个员工可挂多个角色）。
// admins.role 字段降级为兼容/展示字段，鉴权一律以本表为准。
type AdminRole struct {
	AdminID uint64 `gorm:"column:admin_id;primaryKey"`
	RoleID  uint64 `gorm:"column:role_id;primaryKey"`
}

func (AdminRole) TableName() string {
	return "admin_roles"
}
