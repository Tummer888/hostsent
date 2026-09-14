// Package service 提供官网门户（frontend-site）公开只读数据的业务编排。
//
// 与 uc / admin 的分界：
//   - uc 服务「已登录用户的控制台」，接口一律带用户态；
//   - admin 服务「运营管理台」；
//   - 本模块只服务「未登录的公网访客与搜索引擎」，输出必须可共享缓存、字段必须脱敏。
//
// 因此它不挂在 uc 之下：三端后端归属清晰，避免把公开接口混进用户态模块。
package service

import (
	"context"
	"sort"
	"strings"

	contentdto "hostsent/backend/internal/modules/admin/content/dto"
	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	systemmodel "hostsent/backend/internal/modules/admin/system/model"

	"hostsent/backend/internal/modules/site/dto"
)

const (
	// announcementDefaultLimit 未指定 limit 时返回的公告条数。
	announcementDefaultLimit = 5
	// announcementMaxLimit 单次请求上限，避免公开接口被当成翻全量数据的入口。
	announcementMaxLimit = 50
	// articleDefaultPageSize / articleMaxPageSize 文章列表分页边界。
	articleDefaultPageSize = 10
	articleMaxPageSize     = 50
)

// announcementReader 公告读取能力的最小暴露接口（由装配层注入，避免 site 依赖 admin service 实现）。
type announcementReader interface {
	ListPublished(ctx context.Context, platform string) ([]notifydto.AnnouncementInfo, error)
	// GetPublishedByID 单条已发布公告；不存在返回 (nil, nil)，门户据此判 404 而非把内容删除。
	GetPublishedByID(ctx context.Context, id uint64) (*notifydto.AnnouncementInfo, error)
}

// contentReader 内容中心读取能力（由装配层注入 admin/content 的服务实现）。
//
// 用最小接口而不是直接依赖 admin/content 的 service 包：site 只声明「我读什么」，
// 装配层负责把实现塞进来。这样公开只读域与管理写入域在类型层面就是分开的，
// 将来把内容中心换成别的存储或拆成独立服务，门户侧一行不用改。
type contentReader interface {
	// ListPublished 已发布内容分页列表（仅 published 且未下线）。
	ListPublished(ctx context.Context, kind string, categoryID uint64, page, pageSize int) (contentdto.ArticleListResponse, error)
	// GetPublished 单篇详情；不存在返回 (nil, nil)。
	GetPublished(ctx context.Context, kind, slug string) (*contentdto.ArticleInfo, error)
	// GetPublishedSingleton 该类型唯一一篇已发布内容（条款 / 隐私）。
	GetPublishedSingleton(ctx context.Context, kind string) (*contentdto.ArticleInfo, error)
	// ListActiveTree 启用中的分类树（新闻分栏 / 帮助目录）。
	ListActiveTree(ctx context.Context, kind string) ([]contentdto.CategoryInfo, error)
	// ListActive 启用中的友情链接。
	ListActive(ctx context.Context) ([]contentdto.LinkInfo, error)
}

// configReader 站点配置读取能力（system_configs 表，按分组取）。
type configReader interface {
	ListByGroup(ctx context.Context, group string) ([]systemmodel.SystemConfig, error)
}

// SiteService 官网门户公开数据能力。
type SiteService interface {
	// ListAnnouncements 已发布公告，置顶优先再按发布时间倒序，按 limit 截断。
	ListAnnouncements(ctx context.Context, limit int) ([]dto.AnnouncementItem, error)
	// GetAnnouncement 单条已发布公告；不存在返回 (nil, nil)。
	GetAnnouncement(ctx context.Context, id uint64) (*dto.AnnouncementItem, error)
	// ListArticles 已发布文章分页列表（不含正文）。
	ListArticles(ctx context.Context, q dto.ArticleListQuery) (dto.ArticleListResponse, error)
	// GetArticle 单篇已发布文章详情（含已净化正文）；不存在返回 (nil, nil)。
	GetArticle(ctx context.Context, kind, slug string) (*dto.ArticleDetail, error)
	// GetSingletonArticle 条款/隐私这类「每类型仅一篇」的详情；不存在返回 (nil, nil)。
	GetSingletonArticle(ctx context.Context, kind string) (*dto.ArticleDetail, error)
	// ListCategories 公开分类树（新闻分栏 / 帮助目录树）。
	ListCategories(ctx context.Context, kind string) (dto.CategoryListResponse, error)
	// ListFriendlyLinks 启用中的友情链接（按 sort_order）。
	ListFriendlyLinks(ctx context.Context) (dto.FriendlyLinkListResponse, error)
	// SiteContent 站点品牌配置（白名单键，扁平 map，仅返回已启用项且非空值）。
	SiteContent(ctx context.Context) (dto.SiteContentResponse, error)
}

type siteService struct {
	announcements announcementReader
	contents      contentReader
	configs       configReader
}

// NewSiteService 创建官网门户公开数据服务。
//
// contents 为 nil 时内容类接口返回空结果（不报错）—— 门户必须能在内容模块未装配的
// 环境里正常渲染首页，而不是整站 500。
func NewSiteService(announcements announcementReader, contents contentReader, configs configReader) SiteService {
	return &siteService{announcements: announcements, contents: contents, configs: configs}
}

// publicSiteKeys 站点配置对外白名单。
//
// 硬规则（doc80 §5.3）：公开接口逐键返回，禁止整表序列化；带 internal. 前缀的键永不外露。
// 同时覆盖两套键名：管理端系统配置页写入的是历史扁平键（site_name，group=base），
// doc80 规范键是点号命名（site.name，group=site）。前端 BFF 的 fromFlatConfig 只认点号键，
// 因此公开接口把两套键都原样返回，由前端按 schema 取用，服务端不做语义改名。
var publicSiteKeys = []string{
	// doc80 §5.3 规范键（site 分组，点号命名）
	"site.name",
	"site.slogan",
	"site.logo",
	"site.favicon",
	"site.icp",
	"site.contact_phone",
	"site.contact_email",
	"site.copyright",
	"theme.primary_color",
	"theme.radius",
	"home.hero_title",
	"home.hero_subtitle",
	"home.hero_image",
	"home.hero_primary_cta",
	"home.hero_primary_link",
	"home.hero_secondary_cta",
	"home.hero_secondary_link",
	"home.features_title",
	"home.features",
	"home.cta_title",
	"home.cta_desc",
	"home.featured_title",
	"home.featured_limit",
	"home.announce_title",
	"home.announce_limit",
	// 站点法务与联系方式（页脚使用，doc80 未列出但同属公开品牌信息）
	"site.license_no",      // 增值电信业务经营许可证号
	"site.license_org",     // 代理域名注册服务机构
	"site.public_security", // 公网安备号
	"site.contact_address", // 联系地址
	"site.wechat",          // 公众号名称
	// 页脚配置化（doc100 §8.2）：栏目 / 服务承诺 / 社交按钮 / 法律行。
	// 值均为 JSON 字符串，解析与默认值回落由门户 shared/schemas/siteContent.ts 负责。
	"site.footer_columns",
	"site.footer_promises",
	"site.footer_socials",
	"site.footer_legal_line",
	// 管理端系统配置页当前写入的历史扁平键（group=base）
	"site_name",
	"site_slogan",
	"site_logo",
	"site_favicon",
	"site_icp",
	"site_copyright",
	"site_license_no",
	"site_license_org",
	"site_public_security",
	"contact_phone",
	"contact_email",
	"contact_address",
	"contact_wechat",
}

// publicSiteKeySet 白名单集合，供 O(1) 判定。
var publicSiteKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(publicSiteKeys))
	for _, key := range publicSiteKeys {
		set[key] = struct{}{}
	}
	return set
}()

// SiteContent 返回站点品牌配置。
//
// 读取合并两个分组（config_group='site' 与历史 'base'），按白名单逐键过滤：
//   - 只返回 status=active 且值非空的项（前端对缺失键回落代码默认值）；
//   - 同义键（site.name 与 site_name）同时存在时都返回，由前端 fromFlatConfig 按 schema 取用；
//   - 任何读取失败都返回空 map 而不是报错，避免官网白屏。
func (s *siteService) SiteContent(ctx context.Context) (dto.SiteContentResponse, error) {
	out := dto.SiteContentResponse{}
	if s.configs == nil {
		return out, nil
	}

	// 分组不存在时 ListByGroup 返回空切片而非错误，两个分组都读以防只配了一处。
	configs := make([]systemmodel.SystemConfig, 0, 64)
	for _, group := range []string{systemmodel.ConfigGroupSite, systemmodel.ConfigGroupBase} {
		items, err := s.configs.ListByGroup(ctx, group)
		if err != nil {
			continue
		}
		configs = append(configs, items...)
	}

	// 规范化键名：去空白、去首尾点，便于兼容运营手工录入的 " site.name " 之类脏值。
	picked := make(map[string]string, len(configs))
	for _, cfg := range configs {
		if cfg.Status != "" && cfg.Status != systemmodel.StatusActive {
			continue
		}
		key := strings.Trim(strings.TrimSpace(cfg.ConfigKey), ".")
		if key == "" {
			continue
		}
		if _, ok := publicSiteKeySet[key]; !ok {
			continue
		}
		if strings.TrimSpace(cfg.ConfigValue) == "" {
			continue
		}
		picked[key] = cfg.ConfigValue
	}
	if len(picked) == 0 {
		return out, nil
	}

	// 按白名单顺序稳定输出，便于人工比对与缓存。
	keys := make([]string, 0, len(picked))
	for key := range picked {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	out.Items = picked
	return out, nil
}

func (s *siteService) ListAnnouncements(ctx context.Context, limit int) ([]dto.AnnouncementItem, error) {
	if limit <= 0 {
		limit = announcementDefaultLimit
	}
	if limit > announcementMaxLimit {
		limit = announcementMaxLimit
	}
	if s.announcements == nil {
		return []dto.AnnouncementItem{}, nil
	}

	items, err := s.announcements.ListPublished(ctx, notifymodel.AnnouncementPlatformUser)
	if err != nil {
		return nil, err
	}
	if len(items) > limit {
		items = items[:limit]
	}

	out := make([]dto.AnnouncementItem, 0, len(items))
	for _, it := range items {
		out = append(out, toAnnouncementItem(it))
	}
	return out, nil
}

// GetAnnouncement 单条已发布公告。返回 (nil, nil) 表示不存在/已下线 —— 门户据此 404，
// 与「服务故障」严格区分，避免搜索引擎把临时故障当成内容删除。
func (s *siteService) GetAnnouncement(ctx context.Context, id uint64) (*dto.AnnouncementItem, error) {
	if s.announcements == nil {
		return nil, nil
	}
	item, err := s.announcements.GetPublishedByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Platform == notifymodel.AnnouncementPlatformAdmin {
		// platform=admin 的公告是给管理端看的，不对外；只有 user/both 允许公开读取。
		return nil, nil
	}
	out := toAnnouncementItem(*item)
	return &out, nil
}

func toAnnouncementItem(it notifydto.AnnouncementInfo) dto.AnnouncementItem {
	format := it.BodyFormat
	if format == "" {
		format = notifymodel.AnnouncementFormatText
	}
	return dto.AnnouncementItem{
		ID:         it.ID,
		Title:      it.Title,
		Content:    it.Content,
		BodyFormat: format,
		Level:      it.Level,
		Popup:      it.Popup,
		Pinned:     it.Pinned,
		PublishAt:  derefString(it.PublishAt),
	}
}

/* ------------------------------ 内容中心 ------------------------------ */

func (s *siteService) ListArticles(ctx context.Context, q dto.ArticleListQuery) (dto.ArticleListResponse, error) {
	out := dto.ArticleListResponse{Items: []dto.ArticleItem{}, Page: 1, PageSize: articleDefaultPageSize}
	if s.contents == nil {
		return out, nil
	}
	kind := normalizeArticleKind(q.Kind)
	if kind == "" {
		return out, nil
	}
	page, pageSize := normalizeArticlePage(q.Page, q.PageSize)

	resp, err := s.contents.ListPublished(ctx, kind, q.CategoryID, page, pageSize)
	if err != nil {
		return out, err
	}
	// 分类名/分类 slug 需要另外一张表；一次取全部启用分类做映射，避免逐行查询。
	categoryByID := map[uint64]contentdto.CategoryInfo{}
	if cats, cerr := s.contents.ListActiveTree(ctx, kind); cerr == nil {
		flattenCategories(cats, categoryByID)
	}

	out.Items = make([]dto.ArticleItem, 0, len(resp.List))
	for _, it := range resp.List {
		if it == nil {
			continue
		}
		out.Items = append(out.Items, toArticleItem(*it, categoryByID))
	}
	out.Total = resp.Total
	if resp.Page > 0 {
		out.Page = resp.Page
	}
	if resp.PageSize > 0 {
		out.PageSize = resp.PageSize
	}
	return out, nil
}

func (s *siteService) GetArticle(ctx context.Context, kind, slug string) (*dto.ArticleDetail, error) {
	if s.contents == nil {
		return nil, nil
	}
	normalized := normalizeArticleKind(kind)
	if normalized == "" || strings.TrimSpace(slug) == "" {
		return nil, nil
	}
	item, err := s.contents.GetPublished(ctx, normalized, slug)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	categoryByID := map[uint64]contentdto.CategoryInfo{}
	if cats, cerr := s.contents.ListActiveTree(ctx, normalized); cerr == nil {
		flattenCategories(cats, categoryByID)
	}
	return toArticleDetail(*item, categoryByID), nil
}

// GetSingletonArticle 取条款/隐私这类「每类型仅一篇」的文档。
func (s *siteService) GetSingletonArticle(ctx context.Context, kind string) (*dto.ArticleDetail, error) {
	if s.contents == nil {
		return nil, nil
	}
	normalized := normalizeArticleKind(kind)
	if normalized == "" {
		return nil, nil
	}
	item, err := s.contents.GetPublishedSingleton(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return toArticleDetail(*item, map[uint64]contentdto.CategoryInfo{}), nil
}

func (s *siteService) ListCategories(ctx context.Context, kind string) (dto.CategoryListResponse, error) {
	out := dto.CategoryListResponse{Items: []dto.CategoryItem{}}
	if s.contents == nil {
		return out, nil
	}
	normalized := normalizeArticleKind(kind)
	if normalized == "" {
		return out, nil
	}
	items, err := s.contents.ListActiveTree(ctx, normalized)
	if err != nil {
		return out, err
	}
	out.Items = toCategoryItems(items)
	return out, nil
}

func (s *siteService) ListFriendlyLinks(ctx context.Context) (dto.FriendlyLinkListResponse, error) {
	out := dto.FriendlyLinkListResponse{Items: []dto.FriendlyLinkItem{}}
	if s.contents == nil {
		return out, nil
	}
	items, err := s.contents.ListActive(ctx)
	if err != nil {
		return out, err
	}
	out.Items = make([]dto.FriendlyLinkItem, 0, len(items))
	for _, it := range items {
		out.Items = append(out.Items, dto.FriendlyLinkItem{
			ID:          it.ID,
			Name:        it.Name,
			URL:         it.URL,
			Logo:        it.Logo,
			Description: it.Description,
			OpenInNew:   it.OpenInNew,
		})
	}
	return out, nil
}

// normalizeArticleKind 收敛公开接口的 kind 入参。
//
// 未知 kind 返回空串而不是原样透传：透传会让仓储拼出 WHERE kind='../etc' 这类查询，
// 虽然不会注入，但会静默返回空列表，掩盖前端拼错参数的问题。
func normalizeArticleKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "news":
		return "news"
	case "help":
		return "help"
	case "terms":
		return "terms"
	case "privacy":
		return "privacy"
	default:
		return ""
	}
}

func normalizeArticlePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > articleMaxPageSize {
		pageSize = articleDefaultPageSize
	}
	return page, pageSize
}

func toArticleItem(it contentdto.ArticleInfo, categories map[uint64]contentdto.CategoryInfo) dto.ArticleItem {
	item := dto.ArticleItem{
		ID:         it.ID,
		Kind:       it.Kind,
		Slug:       it.Slug,
		Title:      it.Title,
		Summary:    it.Summary,
		Cover:      it.Cover,
		Tags:       splitTags(it.Tags),
		CategoryID: it.CategoryID,
		Pinned:     it.Pinned,
		Version:    it.Version,
		PublishAt:  it.PublishAt,
		UpdatedAt:  it.UpdatedAt,
	}
	if c, ok := categories[it.CategoryID]; ok {
		item.CategorySlug = c.Slug
		item.CategoryName = c.Name
	}
	return item
}

func toArticleDetail(it contentdto.ArticleInfo, categories map[uint64]contentdto.CategoryInfo) *dto.ArticleDetail {
	return &dto.ArticleDetail{
		ArticleItem: toArticleItem(it, categories),
		// body_format=text 的存量内容不经过净化，前端按文本节点渲染；
		// html 已在写入时净化过一次，这里直接透出。
		Body: it.Body,
	}
}

// toCategoryItems 把平铺分类组装成公开树。
//
// 与 admin 侧 buildCategoryTree 的差别：这里只保留展示字段（不暴露 status/sort_order），
// 且父节点缺失时提升为根 —— 不丢弃内容入口。
func toCategoryItems(items []contentdto.CategoryInfo) []dto.CategoryItem {
	nodes := make(map[uint64]*dto.CategoryItem, len(items))
	for _, it := range items {
		node := &dto.CategoryItem{
			ID:          it.ID,
			Slug:        it.Slug,
			Name:        it.Name,
			Description: it.Description,
			Icon:        it.Icon,
		}
		nodes[it.ID] = node
	}
	roots := make([]dto.CategoryItem, 0, len(items))
	for _, it := range items {
		node := nodes[it.ID]
		if it.ParentID != 0 {
			if parent, ok := nodes[it.ParentID]; ok && parent != node {
				parent.Children = append(parent.Children, *node)
				continue
			}
		}
		roots = append(roots, *node)
	}
	return roots
}

// flattenCategories 把分类树摊平成 id → 分类 的映射（列表项要显示分类名）。
//
// 兼容两种入参：装配层注入平铺列表（只有 parent_id）或已组装的树，两种情况都能摊平。
func flattenCategories(items []contentdto.CategoryInfo, out map[uint64]contentdto.CategoryInfo) {
	for i := range items {
		out[items[i].ID] = items[i]
		flattenCategoryNodes(items[i].Children, out)
	}
}

func flattenCategoryNodes(items []*contentdto.CategoryInfo, out map[uint64]contentdto.CategoryInfo) {
	for _, it := range items {
		if it == nil {
			continue
		}
		out[it.ID] = *it
		flattenCategoryNodes(it.Children, out)
	}
}

// splitTags 把逗号分隔的标签串切成切片；空串返回空切片（不是 nil）。
//
// 返回空切片而非 nil 的原因：JSON 里 nil 切片会序列化成 null，
// 前端 schema 校验 `z.array()` 会因 null 失败，整条内容被丢弃。
func splitTags(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if tag := strings.TrimSpace(part); tag != "" {
			out = append(out, tag)
		}
	}
	return out
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
