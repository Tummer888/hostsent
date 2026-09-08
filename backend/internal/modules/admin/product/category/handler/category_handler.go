package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/category/dto"
	"hostsent/backend/internal/modules/admin/product/category/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// CategoryHandler 商品分类处理入口
type CategoryHandler struct {
	categoryService service.CategoryService
}

// NewCategoryHandler 创建分类处理入口
func NewCategoryHandler(categoryService service.CategoryService) *CategoryHandler {
	return &CategoryHandler{categoryService: categoryService}
}

// List godoc
// @Summary 查询分类树
// @Tags 产品管理-分类
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	var req dto.CategoryListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.categoryService.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 创建分类
// @Tags 产品管理-分类
// @Security BearerAuth
// @Param request body dto.CategoryCreateRequest true "分类参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req dto.CategoryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.categoryService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询分类详情
// @Tags 产品管理-分类
// @Param id path int true "分类 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/categories/{id} [get]
func (h *CategoryHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.categoryService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新分类
// @Tags 产品管理-分类
// @Param id path int true "分类 ID"
// @Param request body dto.CategoryUpdateRequest true "分类参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/categories/{id} [put]
func (h *CategoryHandler) Update(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.CategoryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.categoryService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除分类
// @Tags 产品管理-分类
// @Param id path int true "分类 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.categoryService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}
