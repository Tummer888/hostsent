package dto

import "time"

// DepartmentListQuery 部门列表查询（doc86 §2.1）。
// Kind/Status 为空表示不过滤；Flat=1 时返回平铺列表，否则由服务层组装成树。
type DepartmentListQuery struct {
	Kind   string `form:"kind"`
	Status string `form:"status"`
	Flat   int    `form:"flat"`
	// Keyword 按名称/编码模糊匹配，用于部门选择器搜索。
	Keyword string `form:"keyword"`
}

// DepartmentSaveRequest 新建/编辑部门入参。
type DepartmentSaveRequest struct {
	Name string `json:"name" binding:"required"`
	Code string `json:"code" binding:"required"`
	Kind string `json:"kind"`
	// ParentID 上级部门，0 表示根部门（仅两级，前端已限制）。
	ParentID uint64 `json:"parent_id"`
	// LeaderAdminID 部门主管（admins.id），0 表示未设置。
	LeaderAdminID uint64 `json:"leader_admin_id"`
	Remark        string `json:"remark"`
	SortOrder     int    `json:"sort_order"`
	Status        string `json:"status"`
}

// DepartmentInfo 部门信息。Children 仅在树形返回时填充。
type DepartmentInfo struct {
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	Code          string `json:"code"`
	Kind          string `json:"kind"`
	ParentID      uint64 `json:"parent_id"`
	LeaderAdminID uint64 `json:"leader_admin_id"`
	// LeaderName 只读：主管姓名（未设置时为空）。
	LeaderName string `json:"leader_name"`
	Remark     string `json:"remark"`
	SortOrder  int    `json:"sort_order"`
	Status     string `json:"status"`
	// AdminCount 只读：在职员工数（resigned_at 为空且 status=active）。
	AdminCount int64 `json:"admin_count"`
	// CategoryCount 只读：引用该部门的工单分类数（删除前置校验依据）。
	CategoryCount int64            `json:"category_count"`
	Children      []DepartmentInfo `json:"children,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// DepartmentListResponse 部门列表响应。
type DepartmentListResponse struct {
	Items []DepartmentInfo `json:"items"`
	// Tree 为 true 时 Items 为根节点（含 Children），前端据此决定渲染方式。
	Tree bool `json:"tree"`
}
