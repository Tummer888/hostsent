package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/pricing/dto"
	"hostsent/backend/internal/modules/admin/product/pricing/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// PricingHandler 定价与计费处理入口。
type PricingHandler struct {
	pricingService service.PricingService
}

// NewPricingHandler 创建定价处理入口。
func NewPricingHandler(pricingService service.PricingService) *PricingHandler {
	return &PricingHandler{pricingService: pricingService}
}

// List godoc
// @Summary 查询价格策略列表
// @Tags 产品管理-定价
// @Security BearerAuth
// @Param product_id query int false "商品 ID"
// @Param billing_mode query string false "计费模式"
// @Param status query int false "状态"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/pricing [get]
func (h *PricingHandler) List(c *gin.Context) {
	var query dto.PricingQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.pricingService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 新增价格策略
// @Tags 产品管理-定价
// @Security BearerAuth
// @Param request body dto.PricingRequest true "价格策略参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/pricing [post]
func (h *PricingHandler) Create(c *gin.Context) {
	var req dto.PricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.pricingService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询价格策略详情
// @Tags 产品管理-定价
// @Param id path int true "价格策略 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/pricing/{id} [get]
func (h *PricingHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.pricingService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新价格策略
// @Tags 产品管理-定价
// @Param id path int true "价格策略 ID"
// @Param request body dto.PricingRequest true "价格策略参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/pricing/{id} [put]
func (h *PricingHandler) Update(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.PricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.pricingService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除价格策略
// @Tags 产品管理-定价
// @Param id path int true "价格策略 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/pricing/{id} [delete]
func (h *PricingHandler) Delete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.pricingService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}
