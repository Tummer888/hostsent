package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/pricing/dto"
	"hostsent/backend/internal/modules/admin/product/pricing/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// PriceMatrixHandler 周期价格矩阵处理入口（doc25）。
type PriceMatrixHandler struct {
	matrixService service.PriceMatrixService
}

// NewPriceMatrixHandler 创建周期价格矩阵处理入口。
func NewPriceMatrixHandler(matrixService service.PriceMatrixService) *PriceMatrixHandler {
	return &PriceMatrixHandler{matrixService: matrixService}
}

// Get godoc
// @Summary 查询商品周期价格矩阵
// @Tags 产品管理-定价
// @Security BearerAuth
// @Param product_id query int true "商品 ID"
// @Param spec_id query int false "SKU ID，0=商品级"
// @Param currency query string false "币种，缺省 CNY"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/prices [get]
func (h *PriceMatrixHandler) Get(c *gin.Context) {
	var query dto.PriceMatrixQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.matrixService.Matrix(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Save godoc
// @Summary 整表保存商品周期价格矩阵
// @Tags 产品管理-定价
// @Security BearerAuth
// @Param request body dto.PriceMatrixSaveRequest true "周期价格矩阵"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/prices [put]
func (h *PriceMatrixHandler) Save(c *gin.Context) {
	var req dto.PriceMatrixSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.matrixService.SaveMatrix(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
