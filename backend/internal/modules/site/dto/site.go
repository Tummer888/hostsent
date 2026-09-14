// Package dto 定义官网门户公开接口的出入参（脱敏，仅暴露展示所需字段）。
package dto

// AnnouncementItem 公开公告条目。
//
// 脱敏说明：不返回 operator_id、status、platform、offline_at 等内部字段，
// 仅保留展示所需内容。查询侧已强制 status=published 且 offline_at IS NULL，
// 不会泄漏草稿与已下线公告。
type AnnouncementItem struct {
	ID      uint64 `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	// BodyFormat text/html：text 是存量纯文本（渲染时按文本节点输出），
	// html 是服务端已净化的富文本（可直接 v-html）。前端必须按此字段分支，
	// 一律当 HTML 处理会把存量纯文本里的 < > 吃掉。
	BodyFormat string `json:"body_format"`
	Level      string `json:"level"`      // info / warning / critical
	Popup      bool   `json:"popup"`      // 是否弹窗强提醒
	Pinned     bool   `json:"pinned"`     // 置顶
	PublishAt  string `json:"publish_at"` // 2006-01-02 15:04:05，无则为空串
}

// AnnouncementListResponse 公开公告列表响应。
type AnnouncementListResponse struct {
	Items []AnnouncementItem `json:"items"`
}

// ArticleListQuery 公开文章列表查询。
type ArticleListQuery struct {
	Kind       string `form:"kind"`        // news / help / terms / privacy
	CategoryID uint64 `form:"category_id"` // 分类筛选；0 为不限
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// ArticleItem 公开文章条目（列表用，不含正文）。
//
// 列表刻意不带 body：一屏 20 篇文章各带几 KB 富文本会显著放大响应体，
// 而列表只需要标题与摘要。要正文必须走详情接口。
type ArticleItem struct {
	ID           uint64   `json:"id"`
	Kind         string   `json:"kind"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Cover        string   `json:"cover"`
	Tags         []string `json:"tags"`
	CategoryID   uint64   `json:"category_id"`
	CategorySlug string   `json:"category_slug"`
	CategoryName string   `json:"category_name"`
	Pinned       bool     `json:"pinned"`
	Version      string   `json:"version"`
	PublishAt    string   `json:"publish_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// ArticleDetail 公开文章详情（含已净化正文）。
type ArticleDetail struct {
	ArticleItem
	Body string `json:"body"`
}

// ArticleListResponse 公开文章列表响应（分页）。
type ArticleListResponse struct {
	Items    []ArticleItem `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// CategoryItem 公开分类条目（含子节点，帮助中心据此渲染目录树）。
type CategoryItem struct {
	ID          uint64         `json:"id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Children    []CategoryItem `json:"children,omitempty"`
}

// CategoryListResponse 公开分类树响应。
type CategoryListResponse struct {
	Items []CategoryItem `json:"items"`
}

// FriendlyLinkItem 公开友情链接条目。
type FriendlyLinkItem struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
	OpenInNew   bool   `json:"open_in_new"`
}

// FriendlyLinkListResponse 公开友情链接列表响应。
type FriendlyLinkListResponse struct {
	Items []FriendlyLinkItem `json:"items"`
}

// SiteContentResponse 站点品牌配置响应。
//
// Items 是「扁平键 → 值」的白名单映射，键形如 `site.name` / `home.hero_title`（点号命名）
// 或管理端历史扁平键 `site_name`。刻意不做结构化成嵌套对象：键的 schema、默认值与校验
// 是前端 frontend-site/shared/schemas/siteContent.ts 的单一真相，服务端只负责按白名单取值，
// 避免同一份配置定义在前后端各维护一遍而漂移。
type SiteContentResponse struct {
	Items map[string]string `json:"items"`
}
