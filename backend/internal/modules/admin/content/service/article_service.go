// Package service 提供内容中心的业务编排。
//
// 职责边界：
//   - 富文本净化：所有写路径的 body 必须过 internal/pkg/sanitize（doc100 §6.1）。
//     净化放在服务层而不是 handler，保证「绕过 HTTP 层直接调服务」也拿不到未净化数据。
//   - slug 唯一性：同类型内唯一，冲突时自动追加序号，不让运营看到 500。
//   - 状态推进：draft → published → offline 与公告同一套语义。
//
// 不负责：门户缓存失效（装配层触发）、markdown 渲染（body_format 预留但当前只走 html）。
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/content/dto"
	"hostsent/backend/internal/modules/admin/content/model"
	"hostsent/backend/internal/modules/admin/content/repository"
	"hostsent/backend/internal/pkg/revalidate"
	"hostsent/backend/internal/pkg/sanitize"
)

// 业务错误。
var (
	ErrArticleNotFound   = errors.New("文章不存在")
	ErrInvalidKind       = errors.New("不支持的内容类型")
	ErrSlugRequired      = errors.New("无法生成有效标识，请手动填写 slug")
	ErrCategoryNotFound  = errors.New("分类不存在")
	ErrCategoryHasChild  = errors.New("分类下存在子分类，无法删除")
	ErrCategoryKindMix   = errors.New("分类类型与文章类型不一致")
	ErrLinkNotFound      = errors.New("友情链接不存在")
	ErrLinkURLRequired   = errors.New("链接地址不能为空")
	ErrTermsBodyRequired = errors.New("条款/隐私政策的正文不能为空")
)

// 摘要自动生成的截断长度（未手填 summary 时从正文剥标签取）。
const summaryMaxLen = 140

// ArticleService 内容文章业务能力。
type ArticleService interface {
	List(ctx context.Context, req dto.ArticleListQuery) (*dto.ArticleListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.ArticleInfo, error)
	Create(ctx context.Context, req *dto.ArticleSaveRequest, operatorID uint64) (*dto.ArticleInfo, error)
	Update(ctx context.Context, id uint64, req *dto.ArticleSaveRequest, operatorID uint64) (*dto.ArticleInfo, error)
	Publish(ctx context.Context, id uint64) (*dto.ArticleInfo, error)
	Offline(ctx context.Context, id uint64) (*dto.ArticleInfo, error)
	Delete(ctx context.Context, id uint64) error
	// ListPublished 门户公开列表：仅 published 且未下线，带分页与总数。
	ListPublished(ctx context.Context, kind string, categoryID uint64, page, pageSize int) (*dto.ArticleListResponse, error)
	// GetPublished 门户详情；不存在/未发布返回 (nil, nil)，门户据此判 404 而非 503。
	GetPublished(ctx context.Context, kind, slug string) (*dto.ArticleInfo, error)
	// GetPublishedSingleton 类型唯一一篇已发布内容（条款 / 隐私政策）。
	GetPublishedSingleton(ctx context.Context, kind string) (*dto.ArticleInfo, error)
}

type articleService struct {
	repo         repository.ArticleRepository
	categoryRepo repository.CategoryRepository
}

// NewArticleService 创建文章业务服务。
func NewArticleService(repo repository.ArticleRepository, categoryRepo repository.CategoryRepository) ArticleService {
	return &articleService{repo: repo, categoryRepo: categoryRepo}
}

func (s *articleService) List(ctx context.Context, req dto.ArticleListQuery) (*dto.ArticleListResponse, error) {
	items, total, err := s.repo.List(ctx, repository.ArticleQuery{
		Kind:       strings.TrimSpace(req.Kind),
		CategoryID: req.CategoryID,
		Status:     strings.TrimSpace(req.Status),
		Keyword:    req.Keyword,
		Page:       req.Page,
		PageSize:   req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	list, err := s.decorate(ctx, items)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizePage(req.Page, req.PageSize)
	return &dto.ArticleListResponse{Total: total, List: list, Page: page, PageSize: pageSize}, nil
}

// decorate 给列表补上分类名（批量取一次，避免 N+1）。
func (s *articleService) decorate(ctx context.Context, items []model.Article) ([]*dto.ArticleInfo, error) {
	ids := make([]uint64, 0, len(items))
	for _, it := range items {
		if it.CategoryID > 0 {
			ids = append(ids, it.CategoryID)
		}
	}
	names := map[uint64]string{}
	if len(ids) > 0 {
		found, err := s.categoryRepo.FindByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		for id, c := range found {
			names[id] = c.Name
		}
	}
	list := make([]*dto.ArticleInfo, 0, len(items))
	for i := range items {
		info := toArticleInfo(items[i])
		info.CategoryName = names[items[i].CategoryID]
		list = append(list, info)
	}
	return list, nil
}

/* --------------------------- 门户公开读取 --------------------------- */

func (s *articleService) ListPublished(ctx context.Context, kind string, categoryID uint64, page, pageSize int) (*dto.ArticleListResponse, error) {
	out := &dto.ArticleListResponse{List: []*dto.ArticleInfo{}}
	normalized := strings.TrimSpace(kind)
	if !model.ValidKind(normalized) {
		// 未知 kind 返回空列表而不是报错：门户拼错参数时展示「暂无内容」，
		// 比抛 500 更接近真实语义（确实没有这类内容）。
		return out, nil
	}
	items, total, err := s.repo.ListPublishedPage(ctx, normalized, categoryID, page, pageSize)
	if err != nil {
		return nil, err
	}
	list, err := s.decorate(ctx, items)
	if err != nil {
		return nil, err
	}
	out.List = list
	out.Total = total
	out.Page, out.PageSize = normalizePage(page, pageSize)
	return out, nil
}

func (s *articleService) GetPublished(ctx context.Context, kind, slug string) (*dto.ArticleInfo, error) {
	normalized := strings.TrimSpace(kind)
	if !model.ValidKind(normalized) || strings.TrimSpace(slug) == "" {
		return nil, nil
	}
	item, err := s.repo.FindPublishedBySlug(ctx, normalized, strings.TrimSpace(slug))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return s.decorateOne(ctx, *item), nil
}

func (s *articleService) GetPublishedSingleton(ctx context.Context, kind string) (*dto.ArticleInfo, error) {
	normalized := strings.TrimSpace(kind)
	if !model.ValidKind(normalized) {
		return nil, nil
	}
	item, err := s.repo.FindPublishedSingleton(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return s.decorateOne(ctx, *item), nil
}

func (s *articleService) decorateOne(ctx context.Context, item model.Article) *dto.ArticleInfo {
	list, err := s.decorate(ctx, []model.Article{item})
	if err != nil || len(list) == 0 {
		info := toArticleInfo(item)
		return info
	}
	return list[0]
}

func (s *articleService) FindByID(ctx context.Context, id uint64) (*dto.ArticleInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrArticleNotFound
	}
	info := toArticleInfo(*item)
	if item.CategoryID > 0 {
		if c, cerr := s.categoryRepo.FindByID(ctx, item.CategoryID); cerr == nil {
			info.CategoryName = c.Name
		}
	}
	return info, nil
}

func (s *articleService) Create(ctx context.Context, req *dto.ArticleSaveRequest, operatorID uint64) (*dto.ArticleInfo, error) {
	kind := strings.TrimSpace(req.Kind)
	if !model.ValidKind(kind) {
		return nil, ErrInvalidKind
	}
	if err := s.validateCategory(ctx, kind, req.CategoryID); err != nil {
		return nil, err
	}

	body := s.prepareBody(kind, req)
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		summary = sanitize.Excerpt(body, summaryMaxLen)
	}
	if body == "" && kindNeedsBody(kind) {
		return nil, ErrTermsBodyRequired
	}

	slug, err := s.resolveSlug(ctx, kind, req.Slug, req.Title, 0)
	if err != nil {
		return nil, err
	}

	status := model.StatusDraft
	var publishAt *time.Time
	if kindRequiresImmediatePublish(kind) {
		now := time.Now()
		status = model.StatusPublished
		publishAt = &now
	} else if t, ok := parsePublishAt(req.PublishAt); ok {
		publishAt = &t
		if !t.After(time.Now()) {
			status = model.StatusPublished
		}
	} else {
		// 未指定发布时间 = 立即发布（与公告的既有语义一致）
		now := time.Now()
		status = model.StatusPublished
		publishAt = &now
	}

	pinned := req.Pinned != nil && *req.Pinned
	item := &model.Article{
		Kind:       kind,
		CategoryID: req.CategoryID,
		Slug:       slug,
		Title:      strings.TrimSpace(req.Title),
		Summary:    summary,
		Body:       body,
		BodyFormat: model.FormatHTML,
		Cover:      strings.TrimSpace(req.Cover),
		Tags:       strings.TrimSpace(req.Tags),
		Status:     status,
		Pinned:     pinned,
		SortOrder:  req.SortOrder,
		Version:    strings.TrimSpace(req.Version),
		PublishAt:  publishAt,
		OperatorID: operatorID,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	// 门户内容页（/news、/help、/terms、/privacy）走 SWR，已发布内容要立刻可见。
	if item.Status == model.StatusPublished {
		revalidate.Notify(ctx, revalidate.KeyArticle)
	}
	return s.FindByID(ctx, item.ID)
}

func (s *articleService) Update(ctx context.Context, id uint64, req *dto.ArticleSaveRequest, operatorID uint64) (*dto.ArticleInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrArticleNotFound
	}
	kind := strings.TrimSpace(req.Kind)
	if !model.ValidKind(kind) {
		return nil, ErrInvalidKind
	}
	if err := s.validateCategory(ctx, kind, req.CategoryID); err != nil {
		return nil, err
	}

	body := s.prepareBody(kind, req)
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		summary = sanitize.Excerpt(body, summaryMaxLen)
	}

	slug, serr := s.resolveSlug(ctx, kind, req.Slug, req.Title, item.ID)
	if serr != nil {
		return nil, serr
	}

	item.Kind = kind
	item.CategoryID = req.CategoryID
	item.Slug = slug
	item.Title = strings.TrimSpace(req.Title)
	item.Summary = summary
	item.Body = body
	item.BodyFormat = model.FormatHTML
	item.Cover = strings.TrimSpace(req.Cover)
	item.Tags = strings.TrimSpace(req.Tags)
	item.SortOrder = req.SortOrder
	item.Version = strings.TrimSpace(req.Version)
	item.OperatorID = operatorID
	if req.Pinned != nil {
		item.Pinned = *req.Pinned
	}
	// 已发布内容的 publish_at 不被编辑覆盖：改标题不应把「发布时间」改成现在，
	// 否则门户上的时间会随每次编辑跳变，读者无法判断哪些是新内容。
	if item.Status != model.StatusPublished {
		if t, ok := parsePublishAt(req.PublishAt); ok {
			item.PublishAt = &t
			if !t.After(time.Now()) {
				item.Status = model.StatusPublished
				item.OfflineAt = nil
			}
		} else if item.PublishAt == nil {
			now := time.Now()
			item.PublishAt = &now
		}
	}

	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	// 编辑可能改变可见性（草稿→已发布、或正文变更），一律失效内容页缓存。
	revalidate.Notify(ctx, revalidate.KeyArticle)
	return s.FindByID(ctx, item.ID)
}

func (s *articleService) Publish(ctx context.Context, id uint64) (*dto.ArticleInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrArticleNotFound
	}
	if kindNeedsBody(item.Kind) && sanitize.PlainText(item.Body) == "" {
		return nil, ErrTermsBodyRequired
	}
	if item.Status != model.StatusPublished {
		now := time.Now()
		item.Status = model.StatusPublished
		if item.PublishAt == nil || item.PublishAt.After(now) {
			item.PublishAt = &now
		}
	}
	item.OfflineAt = nil // 重新发布必须清掉下线标记，否则内容依然不可见
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	revalidate.Notify(ctx, revalidate.KeyArticle)
	return s.FindByID(ctx, item.ID)
}

func (s *articleService) Offline(ctx context.Context, id uint64) (*dto.ArticleInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrArticleNotFound
	}
	now := time.Now()
	item.Status = model.StatusOffline
	item.OfflineAt = &now
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	// 下线必须让门户立刻撤下：缓存里留着已下线内容是最难解释的一类事故。
	revalidate.Notify(ctx, revalidate.KeyArticle)
	return s.FindByID(ctx, item.ID)
}

func (s *articleService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return ErrArticleNotFound
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	revalidate.Notify(ctx, revalidate.KeyArticle)
	return nil
}

/* --------------------------- 内部辅助 --------------------------- */

// prepareBody 净化正文。
//
// 条款/隐私政策正文为空时直接返回空串，由调用方报错 —— 页面存在但没有任何条文，
// 比页面不存在更容易引发合规质疑。
func (s *articleService) prepareBody(kind string, req *dto.ArticleSaveRequest) string {
	if strings.TrimSpace(req.Body) == "" {
		return ""
	}
	return strings.TrimSpace(sanitize.HTML(req.Body))
}

func (s *articleService) validateCategory(ctx context.Context, kind string, categoryID uint64) error {
	if categoryID == 0 {
		return nil
	}
	c, err := s.categoryRepo.FindByID(ctx, categoryID)
	if err != nil {
		return ErrCategoryNotFound
	}
	if c.Kind != kind {
		return ErrCategoryKindMix
	}
	return nil
}

// resolveSlug 解析并保证 slug 唯一。
//
// 生成顺序：请求显式值 → 标题里的 ASCII 片段 → 类型+时间戳兜底。
// 冲突时追加 -2 / -3…（而不是报错），运营不需要关心 slug 撞车。
func (s *articleService) resolveSlug(ctx context.Context, kind, raw, title string, excludeID uint64) (string, error) {
	base := slugify(raw)
	if base == "" {
		base = slugify(title)
	}
	if base == "" {
		base = fmt.Sprintf("%s-%s", kind, time.Now().Format("20060102150405"))
	}
	base = truncateRunes(base, 100)

	candidate := base
	for i := 2; i <= 50; i++ {
		exists, err := s.repo.ExistsSlug(ctx, kind, candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", ErrSlugRequired
}

// validSlugChars 只保留小写字母、数字与连字符 —— 这是 URL 里不需要转义的字符集。
var validSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

// slugify 把任意输入压成 URL 安全标识。
//
// 中文标题会得到空串（ASCII 正则过滤掉了），此时由调用方走时间戳兜底；
// 不引入拼音库是为了避免为「给中文生成好看 slug」增加一个字典依赖，
// 运营确实需要语义化 URL 时可在表单里手填。
func slugify(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = validSlugChars.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// kindNeedsBody 该类型是否必须有正文。
func kindNeedsBody(kind string) bool {
	return kind == model.KindTerms || kind == model.KindPrivacy
}

// kindRequiresImmediatePublish 固定为「保存即发布」的类型。
//
// 条款与隐私政策是随时可被注册页链接到的法律文本：留成草稿意味着注册页的链接
// 会指向 404。这类内容不做草稿态，保存即可见。
func kindRequiresImmediatePublish(kind string) bool {
	return kind == model.KindTerms || kind == model.KindPrivacy
}

// parsePublishAt 解析管理端传来的发布时间（本地时间，与公告 DTO 同格式）。
func parsePublishAt(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func toArticleInfo(a model.Article) *dto.ArticleInfo {
	return &dto.ArticleInfo{
		ID:         a.ID,
		Kind:       a.Kind,
		CategoryID: a.CategoryID,
		Slug:       a.Slug,
		Title:      a.Title,
		Summary:    a.Summary,
		Body:       a.Body,
		BodyFormat: a.BodyFormat,
		Cover:      a.Cover,
		Tags:       a.Tags,
		Status:     a.Status,
		Pinned:     a.Pinned,
		SortOrder:  a.SortOrder,
		Version:    a.Version,
		PublishAt:  formatTimePtr(a.PublishAt),
		OfflineAt:  formatTimePtr(a.OfflineAt),
		OperatorID: a.OperatorID,
		CreatedAt:  a.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:  a.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
