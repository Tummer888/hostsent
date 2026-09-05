package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// CategoryHandler 工单分类处理入口（管理端）。
type CategoryHandler struct {
	categoryService service.CategoryService
}

// NewCategoryHandler 创建工单分类处理入口。
func NewCategoryHandler(categoryService service.CategoryService) *CategoryHandler {
	return &CategoryHandler{categoryService: categoryService}
}

// List godoc
// @Summary 查询工单分类列表（含禁用）
// @Tags 工单支持-分类
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/ticket-categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	resp, err := h.categoryService.List(c.Request.Context(), true)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 创建工单分类
// @Tags 工单支持-分类
// @Security BearerAuth
// @Param request body dto.CategorySaveRequest true "分类参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/ticket-categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req dto.CategorySaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.categoryService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新工单分类
// @Tags 工单支持-分类
// @Security BearerAuth
// @Param id path int true "分类 ID"
// @Param request body dto.CategorySaveRequest true "分类参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/ticket-categories/{id} [put]
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
	resp, err := h.categoryService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除工单分类（使用中禁止删除）
// @Tags 工单支持-分类
// @Security BearerAuth
// @Param id path int true "分类 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/ticket-categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.categoryService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}
