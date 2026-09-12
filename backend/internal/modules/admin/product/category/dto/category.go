// Package dto 提供商品分类（category）子域的数据传输结构。
package dto

// CategoryListRequest 分类树查询
type CategoryListRequest struct {
	// Status 指针区分「未传」与「显式筛 status=0（停用）」
	Status *int `form:"status" json:"status"`
}

// CategoryCreateRequest 创建分类
type CategoryCreateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Name      string `json:"name" binding:"required"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    int    `json:"status"`
}

// CategoryUpdateRequest 更新分类
type CategoryUpdateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Name      string `json:"name" binding:"required"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    int    `json:"status"`
}

// CategoryInfo 分类信息（含子分类树）
type CategoryInfo struct {
	ID        uint64          `json:"id"`
	ParentID  uint64          `json:"parent_id"`
	Name      string          `json:"name"`
	Icon      string          `json:"icon"`
	SortOrder int             `json:"sort_order"`
	Status    int             `json:"status"`
	Children  []*CategoryInfo `json:"children,omitempty"`
}

// CategoryListResponse 分类树响应
type CategoryListResponse struct {
	Items []*CategoryInfo `json:"items"`
}
