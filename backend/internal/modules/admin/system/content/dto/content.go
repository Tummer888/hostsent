// Package dto 定义内容中心的出入参结构（管理端）。
package dto

// ArticleListQuery 文章列表查询。
type ArticleListQuery struct {
	Kind       string `form:"kind"`        // news/help/terms/privacy；空为全部
	CategoryID uint64 `form:"category_id"` // 分类筛选；0 为不限
	Status     string `form:"status"`      // draft/published/offline；空为全部
	Keyword    string `form:"keyword"`     // 标题模糊搜索
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// ArticleSaveRequest 创建 / 更新文章。
//
// Body 走服务端净化（internal/pkg/sanitize），前端提交什么都会被白名单过滤一遍再入库。
type ArticleSaveRequest struct {
	Kind       string `json:"kind" binding:"required"`
	CategoryID uint64 `json:"category_id"`
	Slug       string `json:"slug"`
	Title      string `json:"title" binding:"required"`
	Summary    string `json:"summary"`
	Body       string `json:"body"`
	BodyFormat string `json:"body_format"`
	Cover      string `json:"cover"`
	Tags       string `json:"tags"`
	Pinned     *bool  `json:"pinned"`
	SortOrder  int    `json:"sort_order"`
	Version    string `json:"version"`
	PublishAt  string `json:"publish_at"` // 2006-01-02 15:04:05；空=立即发布
}

// ArticleInfo 文章信息。
type ArticleInfo struct {
	ID           uint64 `json:"id"`
	Kind         string `json:"kind"`
	CategoryID   uint64 `json:"category_id"`
	CategoryName string `json:"category_name"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	Body         string `json:"body"`
	BodyFormat   string `json:"body_format"`
	Cover        string `json:"cover"`
	Tags         string `json:"tags"`
	Status       string `json:"status"`
	Pinned       bool   `json:"pinned"`
	SortOrder    int    `json:"sort_order"`
	Version      string `json:"version"`
	PublishAt    string `json:"publish_at"`
	OfflineAt    string `json:"offline_at"`
	OperatorID   uint64 `json:"operator_id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ArticleListResponse 文章分页列表。
type ArticleListResponse struct {
	Total    int64          `json:"total"`
	List     []*ArticleInfo `json:"list"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// CategoryListQuery 分类查询。
type CategoryListQuery struct {
	Kind   string `form:"kind"`   // news/help；空为全部
	Status string `form:"status"` // active/disabled；空为全部（保证树完整）
}

// CategorySaveRequest 创建 / 更新分类。
type CategorySaveRequest struct {
	Kind        string `json:"kind" binding:"required"`
	ParentID    uint64 `json:"parent_id"`
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sort_order"`
	Status      string `json:"status"`
}

// CategoryInfo 分类信息（含子节点树）。
type CategoryInfo struct {
	ID          uint64          `json:"id"`
	Kind        string          `json:"kind"`
	ParentID    uint64          `json:"parent_id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	Icon        string          `json:"icon"`
	SortOrder   int             `json:"sort_order"`
	Status      string          `json:"status"`
	Children    []*CategoryInfo `json:"children,omitempty"`
}

// CategoryListResponse 分类树响应。
type CategoryListResponse struct {
	Items []*CategoryInfo `json:"items"`
}

// LinkListQuery 友情链接查询。
type LinkListQuery struct {
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// LinkSaveRequest 创建 / 更新友情链接。
type LinkSaveRequest struct {
	Name        string `json:"name" binding:"required"`
	URL         string `json:"url" binding:"required"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	OpenInNew   *bool  `json:"open_in_new"`
	Status      string `json:"status"`
}

// LinkInfo 友情链接信息。
type LinkInfo struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	OpenInNew   bool   `json:"open_in_new"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// LinkListResponse 友情链接分页列表。
type LinkListResponse struct {
	Total    int64       `json:"total"`
	List     []*LinkInfo `json:"list"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}
