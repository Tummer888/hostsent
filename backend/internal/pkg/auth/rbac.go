package auth

import "strings"

// SuperPermission 超级管理员通配权限：中间件遇到该码直接放行。
const SuperPermission = "*"

// SuperAdminRoleCode 唯一持有全部权限的内置角色。
const SuperAdminRoleCode = "super_admin"

type PermissionSet map[string]struct{}

func NewPermissionSet(items []string) PermissionSet {
	set := make(PermissionSet, len(items))
	for _, item := range items {
		set[strings.TrimSpace(item)] = struct{}{}
	}
	return set
}

func (s PermissionSet) Has(permission string) bool {
	_, ok := s[permission]
	return ok
}

// HasAny 判断是否持有任一权限码；集合含 "*" 时恒为 true。
func (s PermissionSet) HasAny(permissions ...string) bool {
	if _, ok := s[SuperPermission]; ok {
		return true
	}
	for _, permission := range permissions {
		if _, ok := s[permission]; ok {
			return true
		}
	}
	return false
}

// IsSuper 判断权限集合是否为超管通配集合。
func (s PermissionSet) IsSuper() bool {
	_, ok := s[SuperPermission]
	return ok
}

// Slice 返回权限码切片（顺序不保证），便于序列化给前端。
func (s PermissionSet) Slice() []string {
	items := make([]string, 0, len(s))
	for code := range s {
		items = append(items, code)
	}
	return items
}

// EffectivePermissions 计算生效权限码：super_admin 角色返回通配 ["*"]，否则原样返回。
func EffectivePermissions(roleCodes, permCodes []string) []string {
	for _, code := range roleCodes {
		if code == SuperAdminRoleCode {
			return []string{SuperPermission}
		}
	}
	return permCodes
}

// HasRoleCode 判断角色 code 是否在列表中。
func HasRoleCode(roleCodes []string, code string) bool {
	for _, item := range roleCodes {
		if item == code {
			return true
		}
	}
	return false
}
