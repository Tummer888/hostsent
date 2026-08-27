// Package dto 定义菜单模块的请求入参。
package dto

type MenuCreateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Platform  string `json:"platform" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status"`
}

// MenuUpdateRequest 定义更新菜单节点时的请求参数。
type MenuUpdateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Platform  string `json:"platform" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status" binding:"required"`
}
