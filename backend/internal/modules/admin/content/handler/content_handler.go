// Package handler 提供内容中心管理端的 HTTP 处理。
package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/content/dto"
	contentservice "hostsent/backend/internal/modules/admin/content/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ArticleHandler 内容文章管理端点。
type ArticleHandler struct {
	svc contentservice.ArticleService
}

// NewArticleHandler 创建文章处理器。
func NewArticleHandler(svc contentservice.ArticleService) *ArticleHandler {
	return &ArticleHandler{svc: svc}
}

// List godoc
// @Summary 内容文章列表
// @Tags 内容管理-文章
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/content/articles [get]
func (h *ArticleHandler) List(c *gin.Context) {
	var req dto.ArticleListQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 内容文章详情
// @Tags 内容管理-文章
// @Security BearerAuth
// @Param id path int true "文章 ID"
// @Router /api/v1/admin/content/articles/{id} [get]
func (h *ArticleHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 创建内容文章
// @Tags 内容管理-文章
// @Security BearerAuth
// @Router /api/v1/admin/content/articles [post]
func (h *ArticleHandler) Create(c *gin.Context) {
	var req dto.ArticleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), &req, operatorID(c))
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新内容文章
// @Tags 内容管理-文章
// @Security BearerAuth
// @Param id path int true "文章 ID"
// @Router /api/v1/admin/content/articles/{id} [put]
func (h *ArticleHandler) Update(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.ArticleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, &req, operatorID(c))
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Publish godoc
// @Summary 发布内容文章
// @Tags 内容管理-文章
// @Security BearerAuth
// @Param id path int true "文章 ID"
// @Router /api/v1/admin/content/articles/{id}/publish [post]
func (h *ArticleHandler) Publish(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.Publish(c.Request.Context(), id)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Offline godoc
// @Summary 下线内容文章
// @Tags 内容管理-文章
// @Security BearerAuth
// @Param id path int true "文章 ID"
// @Router /api/v1/admin/content/articles/{id}/offline [post]
func (h *ArticleHandler) Offline(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.Offline(c.Request.Context(), id)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除内容文章
// @Tags 内容管理-文章
// @Security BearerAuth
// @Param id path int true "文章 ID"
// @Router /api/v1/admin/content/articles/{id} [delete]
func (h *ArticleHandler) Delete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.SuccessMessage(c, "deleted")
}

// CategoryHandler 内容分类管理端点。
type CategoryHandler struct {
	svc contentservice.CategoryService
}

// NewCategoryHandler 创建分类处理器。
func NewCategoryHandler(svc contentservice.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

// List godoc
// @Summary 内容分类树
// @Tags 内容管理-分类
// @Security BearerAuth
// @Router /api/v1/admin/content/categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	var req dto.CategoryListQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 创建内容分类
// @Tags 内容管理-分类
// @Security BearerAuth
// @Router /api/v1/admin/content/categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req dto.CategorySaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新内容分类
// @Tags 内容管理-分类
// @Security BearerAuth
// @Param id path int true "分类 ID"
// @Router /api/v1/admin/content/categories/{id} [put]
func (h *CategoryHandler) Update(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.CategorySaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除内容分类
// @Tags 内容管理-分类
// @Security BearerAuth
// @Param id path int true "分类 ID"
// @Router /api/v1/admin/content/categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.SuccessMessage(c, "deleted")
}

// LinkHandler 友情链接管理端点。
type LinkHandler struct {
	svc contentservice.LinkService
}

// NewLinkHandler 创建友情链接处理器。
func NewLinkHandler(svc contentservice.LinkService) *LinkHandler {
	return &LinkHandler{svc: svc}
}

// List godoc
// @Summary 友情链接列表
// @Tags 内容管理-友情链接
// @Security BearerAuth
// @Router /api/v1/admin/content/links [get]
func (h *LinkHandler) List(c *gin.Context) {
	var req dto.LinkListQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 创建友情链接
// @Tags 内容管理-友情链接
// @Security BearerAuth
// @Router /api/v1/admin/content/links [post]
func (h *LinkHandler) Create(c *gin.Context) {
	var req dto.LinkSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新友情链接
// @Tags 内容管理-友情链接
// @Security BearerAuth
// @Param id path int true "链接 ID"
// @Router /api/v1/admin/content/links/{id} [put]
func (h *LinkHandler) Update(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.LinkSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除友情链接
// @Tags 内容管理-友情链接
// @Security BearerAuth
// @Param id path int true "链接 ID"
// @Router /api/v1/admin/content/links/{id} [delete]
func (h *LinkHandler) Delete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, mapContentError(err))
		return
	}
	response.SuccessMessage(c, "deleted")
}

/* ----------------------------- 错误映射 ----------------------------- */

// mapContentError 业务错误 → 统一错误码。
// 40001 段用于「参数/前置条件不满足」（20001 保留给绑定校验失败），50001 为兜底。
func mapContentError(err error) *apperrors.AppError {
	switch {
	case err == nil:
		return apperrors.New(50001, "unknown error")
	case errors.Is(err, contentservice.ErrArticleNotFound),
		errors.Is(err, contentservice.ErrCategoryNotFound),
		errors.Is(err, contentservice.ErrLinkNotFound):
		return apperrors.New(40004, err.Error())
	case errors.Is(err, contentservice.ErrInvalidKind),
		errors.Is(err, contentservice.ErrSlugRequired),
		errors.Is(err, contentservice.ErrCategoryHasChild),
		errors.Is(err, contentservice.ErrCategoryKindMix),
		errors.Is(err, contentservice.ErrLinkURLRequired),
		errors.Is(err, contentservice.ErrTermsBodyRequired):
		return apperrors.New(40001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
