package service

import "errors"

var (
	ErrRoleScopeNotStaff     = errors.New("选中的角色是客户角色，不能分配给员工")
	ErrDepartmentInUse       = errors.New("部门下仍有在职员工或被工单分类引用，请先转移后再删除")
	ErrDepartmentHasChildren = errors.New("部门下仍有子部门，请先处理子部门")
)
