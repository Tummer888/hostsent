// Package dto 定义用户中心菜单模块的返回结构。
package dto

// MenuNode 用户中心菜单树节点。
// 字段对齐前端 frontend-user/src/api/menu.ts 的 MenuNode 定义
// （注意 parentId 为驼峰命名，与后台管理模块的 parent_id 区分）。
type MenuNode struct {
	ID       uint64     `json:"id"`
	ParentID uint64     `json:"parentId"`
	Name     string     `json:"name"`
	Type     string     `json:"type,omitempty"`
	Path     string     `json:"path,omitempty"`
	Icon     string     `json:"icon,omitempty"`
	Children []MenuNode `json:"children,omitempty"`
}
