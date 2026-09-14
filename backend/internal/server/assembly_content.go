package server

// 内容中心装配（doc100）：
//
//   - 管理端三类 CRUD：文章（新闻/帮助/条款/隐私）/ 分类 / 友情链接
//   - 公开只读适配：把三个管理服务收敛成 site 模块声明的 contentReader 最小接口
//
// 依赖方向：site（公开只读）不 import admin/content 的 service，admin/content
// 也不知道 site 的存在；唯一的交叉点在 contentReaderAdapter —— 装配层负责对接。
// 这样「公开域」与「写入域」在类型上是分开的，将来内容中心换实现，门户侧零改动。

import (
	"context"

	"gorm.io/gorm"

	contentdto "hostsent/backend/internal/modules/admin/content/dto"
	contenthandler "hostsent/backend/internal/modules/admin/content/handler"
	contentrepo "hostsent/backend/internal/modules/admin/content/repository"
	contentservice "hostsent/backend/internal/modules/admin/content/service"
)

// contentBundle 内容中心处理器与公开读取适配器集合。
type contentBundle struct {
	articleHandler  *contenthandler.ArticleHandler
	categoryHandler *contenthandler.CategoryHandler
	linkHandler     *contenthandler.LinkHandler
	// reader 供 site 模块读取公开内容（装配时注入 NewSiteService 的第二个参数）。
	reader *contentReaderAdapter
	// articleRepo 供定时发布调度器推进到点草稿（internal/pkg/publishsched）：
	// 调度器只需要「按 publish_at 批量置 published」这一条语句，故复用同一个仓储实例。
	articleRepo contentrepo.ArticleRepository
}

// buildContentBundle 装配内容中心：仓储 → 服务 → 处理器 + 公开读取适配器。
func buildContentBundle(db *gorm.DB) *contentBundle {
	articleRepo := contentrepo.NewArticleRepository(db)
	categoryRepo := contentrepo.NewCategoryRepository(db)
	linkRepo := contentrepo.NewLinkRepository(db)

	articleSvc := contentservice.NewArticleService(articleRepo, categoryRepo)
	categorySvc := contentservice.NewCategoryService(categoryRepo)
	linkSvc := contentservice.NewLinkService(linkRepo)

	return &contentBundle{
		articleHandler:  contenthandler.NewArticleHandler(articleSvc),
		categoryHandler: contenthandler.NewCategoryHandler(categorySvc),
		linkHandler:     contenthandler.NewLinkHandler(linkSvc),
		articleRepo:     articleRepo,
		reader: &contentReaderAdapter{
			articles:   articleSvc,
			categories: categorySvc,
			links:      linkSvc,
		},
	}
}

// contentReaderAdapter 把内容中心的管理服务适配成门户公开读取接口。
//
// 存在的原因只有一个：管理服务的入参出参带「后台语义」（分页结构体指针、状态筛选），
// 公开接口要的是「已发布内容」；两者形状相近但不能直接互换，隔一层适配比在两处
// 各写一遍可见性条件更不容易漂移。
type contentReaderAdapter struct {
	articles   contentservice.ArticleService
	categories contentservice.CategoryService
	links      contentservice.LinkService
}

func (a *contentReaderAdapter) ListPublished(ctx context.Context, kind string, categoryID uint64, page, pageSize int) (contentdto.ArticleListResponse, error) {
	if a == nil || a.articles == nil {
		return contentdto.ArticleListResponse{List: []*contentdto.ArticleInfo{}}, nil
	}
	resp, err := a.articles.ListPublished(ctx, kind, categoryID, page, pageSize)
	if err != nil {
		return contentdto.ArticleListResponse{List: []*contentdto.ArticleInfo{}}, err
	}
	if resp == nil {
		return contentdto.ArticleListResponse{List: []*contentdto.ArticleInfo{}}, nil
	}
	return *resp, nil
}

func (a *contentReaderAdapter) GetPublished(ctx context.Context, kind, slug string) (*contentdto.ArticleInfo, error) {
	if a == nil || a.articles == nil {
		return nil, nil
	}
	return a.articles.GetPublished(ctx, kind, slug)
}

func (a *contentReaderAdapter) GetPublishedSingleton(ctx context.Context, kind string) (*contentdto.ArticleInfo, error) {
	if a == nil || a.articles == nil {
		return nil, nil
	}
	return a.articles.GetPublishedSingleton(ctx, kind)
}

func (a *contentReaderAdapter) ListActiveTree(ctx context.Context, kind string) ([]contentdto.CategoryInfo, error) {
	if a == nil || a.categories == nil {
		return []contentdto.CategoryInfo{}, nil
	}
	return a.categories.ListActiveTree(ctx, kind)
}

func (a *contentReaderAdapter) ListActive(ctx context.Context) ([]contentdto.LinkInfo, error) {
	if a == nil || a.links == nil {
		return []contentdto.LinkInfo{}, nil
	}
	return a.links.ListActive(ctx)
}
