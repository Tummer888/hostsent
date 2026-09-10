package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/discount/dto"
	"hostsent/backend/internal/modules/admin/product/discount/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// PolicyHandler 折扣策略处理入口（P5-06 管理端「折扣策略」页）。
type PolicyHandler struct {
	policyService service.PolicyService
}

// NewPolicyHandler 创建折扣策略处理入口。
func NewPolicyHandler(policyService service.PolicyService) *PolicyHandler {
	return &PolicyHandler{policyService: policyService}
}

// List godoc
// @Summary 查询折扣策略列表
// @Tags 产品管理-折扣策略
// @Security BearerAuth
// @Param keyword query string false "名称/编码关键词"
// @Param status query string false "状态 active/disabled"
// @Param scope query string false "作用域 all/category/product"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/discount-policies [get]
func (h *PolicyHandler) List(c *gin.Context) {
	var query dto.PolicyQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.policyService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询折扣策略详情
// @Tags 产品管理-折扣策略
// @Security BearerAuth
// @Param id path int true "策略 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/discount-policies/{id} [get]
func (h *PolicyHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	resp, err := h.policyService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 新增折扣策略
// @Tags 产品管理-折扣策略
// @Security BearerAuth
// @Param request body dto.PolicyRequest true "折扣策略参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/discount-policies [post]
func (h *PolicyHandler) Create(c *gin.Context) {
	var req dto.PolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.policyService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新折扣策略
// @Tags 产品管理-折扣策略
// @Security BearerAuth
// @Param id path int true "策略 ID"
// @Param request body dto.PolicyRequest true "折扣策略参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/discount-policies/{id} [put]
func (h *PolicyHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.PolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.policyService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除折扣策略
// @Tags 产品管理-折扣策略
// @Security BearerAuth
// @Param id path int true "策略 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/discount-policies/{id} [delete]
func (h *PolicyHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.policyService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, "deleted")
}
