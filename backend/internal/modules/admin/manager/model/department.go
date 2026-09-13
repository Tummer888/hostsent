package model

import "time"

// 部门类型（kind）：决定该部门默认承接的业务线，用于筛选、派单与权限推荐。
const (
	DepartmentKindGeneral = "general" // 综合/其他
	DepartmentKindSales   = "sales"   // 销售
	DepartmentKindSupport = "support" // 客服
	DepartmentKindTech    = "tech"    // 技术
	DepartmentKindOps     = "ops"     // 运维
	DepartmentKindFinance = "finance" // 财务
)

// DepartmentKinds 返回全部合法部门类型（供校验与前端选项）。
func DepartmentKinds() []string {
	return []string{
		DepartmentKindGeneral, DepartmentKindSales, DepartmentKindSupport,
		DepartmentKindTech, DepartmentKindOps, DepartmentKindFinance,
	}
}

// IsValidDepartmentKind 判断部门类型是否合法。
func IsValidDepartmentKind(kind string) bool {
	for _, k := range DepartmentKinds() {
		if k == kind {
			return true
		}
	}
	return false
}

// Department 组织部门（替代 admins.department 自由文本与裸客服组 ID）。
type Department struct {
	ID   uint64 `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"size:64;not null"`
	Code string `gorm:"size:64;not null;uniqueIndex:uk_departments_code"`
	// Kind 部门类型：sales/support/tech/ops/finance/general。
	Kind     string `gorm:"size:32;not null;default:general;index:idx_departments_kind"`
	ParentID uint64 `gorm:"column:parent_id;not null;default:0;index:idx_departments_parent"`
	// LeaderAdminID 部门主管（admins.id），用于归属审批与工单复核兜底。
	LeaderAdminID uint64    `gorm:"column:leader_admin_id;not null;default:0"`
	Remark        string    `gorm:"size:255;not null;default:''"`
	SortOrder     int       `gorm:"column:sort_order;not null;default:0"`
	Status        string    `gorm:"size:32;not null;default:active"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (Department) TableName() string { return "departments" }
