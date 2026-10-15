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
	// CostRate 分类拿货折扣率（doc108，如 0.6=六折）；0=未配置。
	CostRate float64 `json:"cost_rate"`
}

// CategoryUpdateRequest 更新分类
type CategoryUpdateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Name      string `json:"name" binding:"required"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    int    `json:"status"`
	// CostRate 分类拿货折扣率（doc108，如 0.6=六折）；0=未配置。
	CostRate float64 `json:"cost_rate"`
}

// CategoryInfo 分类信息（含子分类树）
type CategoryInfo struct {
	ID        uint64 `json:"id"`
	ParentID  uint64 `json:"parent_id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    int    `json:"status"`
	// CostRate 分类拿货折扣率（0=未配置）；代理折扣的毛利校验基准。
	CostRate float64 `json:"cost_rate"`
	// ProductCount 该分类**自身的直接商品数**（不含子分类）。
	// 分类导航要显示每个节点有多少商品；只统计自身，避免父子重复计数。
	ProductCount int64           `json:"product_count"`
	Children     []*CategoryInfo `json:"children,omitempty"`
}

// CategoryListResponse 分类树响应
type CategoryListResponse struct {
	Items []*CategoryInfo `json:"items"`
	// UncategorizedCount 未分类商品数（category_id 为空，或指向已删除的分类）。
	// 单独给一个桶：这类商品不属于任何节点，遍历分类树算不出来；
	// 而"未分类有多少个"正是运营决定要不要整理分类的依据。
	UncategorizedCount int64 `json:"uncategorized_count"`
}
