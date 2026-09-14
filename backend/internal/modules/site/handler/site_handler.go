// Package handler 提供官网门户公开接口的 HTTP 处理。
package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	sitedto "hostsent/backend/internal/modules/site/dto"
	siteservice "hostsent/backend/internal/modules/site/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// SiteHandler 官网门户公开数据 HTTP 处理器。
type SiteHandler struct {
	siteSvc siteservice.SiteService
}

// NewSiteHandler 创建官网门户公开数据处理器。
func NewSiteHandler(siteSvc siteservice.SiteService) *SiteHandler {
	return &SiteHandler{siteSvc: siteSvc}
}

// Announcements 公开公告列表（仅已发布，无需登录）。
//
// 该接口供官网门户 SSR/BFF 消费，因此显式声明可共享缓存；
// 后台发布/下线公告后由主动失效接口清理，不必等 TTL。
//
// @Summary 官网公开公告列表
// @Tags 官网门户-公开
// @Produce json
// @Param limit query int false "返回条数（默认 5，上限 50）"
// @Success 200 {object} response.Body{data=dto.AnnouncementListResponse}
// @Router /api/v1/public/announcements [get]
func (h *SiteHandler) Announcements(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	items, err := h.siteSvc.ListAnnouncements(c.Request.Context(), limit)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	c.Header("Cache-Control", "public, max-age=300, stale-while-revalidate=600")
	response.Success(c, gin.H{"items": items})
}

// SiteContent 站点品牌配置（品牌名 / Logo / 版权 / 备案号 / 联系方式 / 首页文案）。
//
// 供官网门户 SSR/BFF 消费：品牌信息变更需尽快可见，故 TTL 取 60s（doc80 §4.3）；
// 后台保存后由主动失效接口清理，不必等 TTL。
// 字段经白名单过滤（internal.* 永不外露），无配置时返回空 items，由前端回落代码默认值。
//
// @Summary 官网品牌与站点配置
// @Tags 官网门户-公开
// @Produce json
// @Success 200 {object} response.Body{data=dto.SiteContentResponse}
// @Router /api/v1/public/site-content [get]
func (h *SiteHandler) SiteContent(c *gin.Context) {
	content, err := h.siteSvc.SiteContent(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	c.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	response.Success(c, content)
}

// AnnouncementDetail 公开公告详情（门户 /announcements/:id）。
//
// 区分两种「取不到」：不存在返回业务失败 40404（门户转 404，允许搜索引擎移除），
// 仓储报错返回 50001（门户转 503，临时故障不删索引）。
//
// @Summary 官网公开公告详情
// @Tags 官网门户-公开
// @Produce json
// @Param id path int true "公告 ID"
// @Success 200 {object} response.Body{data=dto.AnnouncementItem}
// @Router /api/v1/public/announcements/{id} [get]
func (h *SiteHandler) AnnouncementDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(40004, "公告不存在"))
		return
	}
	item, err := h.siteSvc.GetAnnouncement(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	if item == nil {
		response.Error(c, apperrors.New(40004, "公告不存在"))
		return
	}
	c.Header("Cache-Control", "public, max-age=300, stale-while-revalidate=600")
	response.Success(c, item)
}

// Articles 公开文章列表（新闻 / 帮助 / 条款 / 隐私共用）。
//
// @Summary 官网公开文章列表
// @Tags 官网门户-公开
// @Produce json
// @Param kind query string true "news/help/terms/privacy"
// @Param category_id query int false "分类筛选"
// @Param page query int false "页码，默认 1"
// @Param page_size query int false "每页条数，默认 10，上限 50"
// @Success 200 {object} response.Body{data=dto.ArticleListResponse}
// @Router /api/v1/public/articles [get]
func (h *SiteHandler) Articles(c *gin.Context) {
	var q sitedto.ArticleListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(40001, err.Error()))
		return
	}
	resp, err := h.siteSvc.ListArticles(c.Request.Context(), q)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	c.Header("Cache-Control", "public, max-age=300, stale-while-revalidate=600")
	response.Success(c, resp)
}

// ArticleDetail 公开文章详情（门户 /news/:slug、/help/:slug）。
//
// @Summary 官网公开文章详情
// @Tags 官网门户-公开
// @Produce json
// @Param kind path string true "news/help/terms/privacy"
// @Param slug path string true "文章标识"
// @Success 200 {object} response.Body{data=dto.ArticleDetail}
// @Router /api/v1/public/articles/{kind}/{slug} [get]
func (h *SiteHandler) ArticleDetail(c *gin.Context) {
	kind := strings.TrimSpace(c.Param("kind"))
	slug := strings.TrimSpace(c.Param("slug"))
	detail, err := h.siteSvc.GetArticle(c.Request.Context(), kind, slug)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	if detail == nil {
		response.Error(c, apperrors.New(40004, "内容不存在"))
		return
	}
	c.Header("Cache-Control", "public, max-age=600, stale-while-revalidate=1800")
	response.Success(c, detail)
}

// SingletonArticle 条款 / 隐私政策这类「每类型一篇」的文档详情。
//
// 与 ArticleDetail 分开的原因：这类内容没有 slug 语义（运营不该被要求给法律文本
// 起一个英文标识），门户直接用 /terms、/privacy 取「当前生效版本」。
//
// @Summary 官网公开单篇文档（条款 / 隐私政策）
// @Tags 官网门户-公开
// @Produce json
// @Param kind path string true "terms/privacy"
// @Success 200 {object} response.Body{data=dto.ArticleDetail}
// @Router /api/v1/public/article-singletons/{kind} [get]
func (h *SiteHandler) SingletonArticle(c *gin.Context) {
	kind := strings.TrimSpace(c.Param("kind"))
	detail, err := h.siteSvc.GetSingletonArticle(c.Request.Context(), kind)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	if detail == nil {
		// 不为 404：法律文本还没配置属于「尚未准备好」，门户应显示占位提示而不是报「页面不存在」。
		response.Error(c, apperrors.New(40004, "内容尚未发布"))
		return
	}
	c.Header("Cache-Control", "public, max-age=600, stale-while-revalidate=1800")
	response.Success(c, detail)
}

// ArticleCategories 公开分类树（新闻分栏 / 帮助目录）。
//
// @Summary 官网公开内容分类
// @Tags 官网门户-公开
// @Produce json
// @Param kind query string true "news/help"
// @Success 200 {object} response.Body{data=dto.CategoryListResponse}
// @Router /api/v1/public/article-categories [get]
func (h *SiteHandler) ArticleCategories(c *gin.Context) {
	resp, err := h.siteSvc.ListCategories(c.Request.Context(), c.Query("kind"))
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	c.Header("Cache-Control", "public, max-age=600, stale-while-revalidate=1800")
	response.Success(c, resp)
}

// FriendlyLinks 公开友情链接（页脚用）。
//
// @Summary 官网公开友情链接
// @Tags 官网门户-公开
// @Produce json
// @Success 200 {object} response.Body{data=dto.FriendlyLinkListResponse}
// @Router /api/v1/public/friendly-links [get]
func (h *SiteHandler) FriendlyLinks(c *gin.Context) {
	resp, err := h.siteSvc.ListFriendlyLinks(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	c.Header("Cache-Control", "public, max-age=300, stale-while-revalidate=900")
	response.Success(c, resp)
}
