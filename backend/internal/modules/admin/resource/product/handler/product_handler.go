package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/resource/product/dto"
	"hostsent/backend/internal/modules/admin/resource/product/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ProductHandler 上游商品处理入口
type ProductHandler struct {
	productService service.ProductService
}

// NewProductHandler 创建上游商品处理入口
func NewProductHandler(productService service.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

// List godoc
// @Summary 查询商品列表
// @Tags 资源管理-上游商品
// @Security BearerAuth
// @Param keyword query string false "关键字"
// @Param provider_id query int false "提供商 ID"
// @Param status query int false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/products [get]
func (h *ProductHandler) List(c *gin.Context) {
	var query dto.ProductListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.productService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询商品详情
// @Tags 资源管理-上游商品
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/products/{id} [get]
func (h *ProductHandler) Get(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	resp, err := h.productService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdatePrice godoc
// @Summary 更新商品价格
// @Tags 资源管理-上游商品
// @Param id path int true "商品 ID"
// @Param request body dto.ProductPriceRequest true "价格参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/products/{id}/price [put]
func (h *ProductHandler) UpdatePrice(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	var req dto.ProductPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.productService.UpdatePrice(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Sync godoc
// @Summary 手动同步商品
// @Tags 资源管理-上游商品
// @Param request body map[string]uint64 true "provider_id"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/products/sync [post]
func (h *ProductHandler) Sync(c *gin.Context) {
	var req struct {
		ProviderID uint64 `json:"provider_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.productService.Sync(c.Request.Context(), req.ProviderID)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

func productID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return 0, false
	}
	return id, true
}
