// Package handler 提供用户中心商品模块 HTTP 接口。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/product/dto"
	"hostsent/backend/internal/modules/uc/product/service"
	"hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ProductHandler 用户中心商品 HTTP 处理器。
type ProductHandler struct {
	productService service.ProductService
}

// NewProductHandler 创建用户中心商品处理器。
func NewProductHandler(productService service.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

// List godoc
// @Summary 商品列表（上架）
// @Tags 用户中心-商品
// @Param keyword query string false "关键字"
// @Param category_id query int false "分类"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/products [get]
func (h *ProductHandler) List(c *gin.Context) {
	var query dto.ListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, errors.New(50001, err.Error()))
		return
	}
	resp, err := h.productService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, errors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 商品详情
// @Tags 用户中心-商品
// @Param id path int true "商品ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/products/{id} [get]
func (h *ProductHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errors.New(50001, err.Error()))
		return
	}
	resp, err := h.productService.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, errors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
