// Package model 定义内容中心的数据模型（新闻 / 帮助 / 条款 / 隐私 / 分类 / 友情链接）。
//
// 四类文章共用一张表的原因（doc100 §5.1）：字段集完全一致，分表会复制四套
// repo/service/handler/管理页面；而公告带「平台定向 / 等级 / 弹窗」的站内消息语义，
// 合并进来会污染消息中心的查询口径，因此公告保持独立表。
package model

import "time"

// 内容类型：一篇文章的归属，决定它在哪个前端、哪条路由下出现。
const (
	KindNews    = "news"    // 新闻 → 门户 /news
	KindHelp    = "help"    // 帮助文档 → 门户 /help
	KindTerms   = "terms"   // 用户条款 → 门户 /terms + 用户中心
	KindPrivacy = "privacy" // 隐私政策 → 门户 /privacy + 用户中心
)

// 发布状态。与公告（notifymodel.Announcement）保持同一套语义，管理员心智一致。
const (
	StatusDraft     = "draft"     // 草稿，不对外
	StatusPublished = "published" // 已发布
	StatusOffline   = "offline"   // 已下线（保留数据，不再展示）
)

// 正文格式。html = 服务端已净化的富文本；text = 纯文本（存量公告）。
const (
	FormatHTML = "html"
	FormatText = "text"
)

// 分类状态。
const (
	CategoryStatusActive   = "active"
	CategoryStatusDisabled = "disabled"
)

// ValidKind 判断类型是否受支持。未知 kind 一律拒绝写入，避免脏数据进库后
// 在门户侧表现为「查不到也不报错」的静默失败。
func ValidKind(kind string) bool {
	switch kind {
	case KindNews, KindHelp, KindTerms, KindPrivacy:
		return true
	default:
		return false
	}
}

// KindNeedsCategory 该类型是否使用分类（新闻分栏 / 帮助目录树）。
// 条款与隐私是单篇文档，不参与分类。
func KindNeedsCategory(kind string) bool {
	return kind == KindNews || kind == KindHelp
}

// Article 内容文章。
type Article struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Kind       string     `gorm:"column:kind;size:20;not null;index" json:"kind"`
	CategoryID uint64     `gorm:"column:category_id;not null;default:0" json:"category_id"`
	Slug       string     `gorm:"column:slug;size:120;not null" json:"slug"`
	Title      string     `gorm:"column:title;size:255;not null" json:"title"`
	Summary    string     `gorm:"column:summary;size:500;not null;default:''" json:"summary"`
	Body       string     `gorm:"column:body;type:text;not null" json:"body"`
	BodyFormat string     `gorm:"column:body_format;size:10;not null;default:html" json:"body_format"`
	Cover      string     `gorm:"column:cover;size:255;not null;default:''" json:"cover"`
	Tags       string     `gorm:"column:tags;size:255;not null;default:''" json:"tags"`
	Status     string     `gorm:"column:status;size:20;not null;default:draft;index" json:"status"`
	Pinned     bool       `gorm:"column:pinned;not null;default:false" json:"pinned"`
	SortOrder  int        `gorm:"column:sort_order;not null;default:0" json:"sort_order"`
	Version    string     `gorm:"column:version;size:20;not null;default:''" json:"version"`
	PublishAt  *time.Time `gorm:"column:publish_at" json:"publish_at"`
	OfflineAt  *time.Time `gorm:"column:offline_at" json:"offline_at"`
	OperatorID uint64     `gorm:"column:operator_id;not null;default:0" json:"operator_id"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名。
func (Article) TableName() string { return "content_articles" }

// Category 内容分类（新闻分栏 / 帮助目录树）。
type Category struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Kind        string    `gorm:"column:kind;size:20;not null;index" json:"kind"`
	ParentID    uint64    `gorm:"column:parent_id;not null;default:0" json:"parent_id"`
	Name        string    `gorm:"column:name;size:64;not null" json:"name"`
	Slug        string    `gorm:"column:slug;size:64;not null" json:"slug"`
	Description string    `gorm:"column:description;size:255;not null;default:''" json:"description"`
	Icon        string    `gorm:"column:icon;size:64;not null;default:''" json:"icon"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0" json:"sort_order"`
	Status      string    `gorm:"column:status;size:20;not null;default:active" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名。
func (Category) TableName() string { return "content_categories" }

// FriendlyLink 友情链接。
//
// 独立成表而非塞进 system_configs 的 JSON 键（doc100 §5.3）：需要 logo、排序、
// 上下架与将来的点击统计，用配置键会让「改一条链接」变成「改一整坨 JSON」。
type FriendlyLink struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;size:64;not null" json:"name"`
	URL         string    `gorm:"column:url;size:255;not null" json:"url"`
	Logo        string    `gorm:"column:logo;size:255;not null;default:''" json:"logo"`
	Description string    `gorm:"column:description;size:255;not null;default:''" json:"description"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0" json:"sort_order"`
	OpenInNew   bool      `gorm:"column:open_in_new;not null;default:true" json:"open_in_new"`
	Status      string    `gorm:"column:status;size:20;not null;default:active" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名。
func (FriendlyLink) TableName() string { return "friendly_links" }
