package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ProductHandler 商品处理入口
type ProductHandler struct {
	productService service.ProductService
}

// NewProductHandler 创建商品处理入口
func NewProductHandler(productService service.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

// List godoc
// @Summary 查询商品列表
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param keyword query string false "关键字"
// @Param category_id query int false "分类 ID"
// @Param status query int false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products [get]
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

// Create godoc
// @Summary 创建商品
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param request body dto.ProductCreateRequest true "商品参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products [post]
func (h *ProductHandler) Create(c *gin.Context) {
	var req dto.ProductCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.Create(c.Request.Context(), req, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// CloneFromUpstream godoc
// @Summary 从上游商品克隆创建销售商品
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param request body dto.ProductCloneRequest true "克隆参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/clone [post]
func (h *ProductHandler) CloneFromUpstream(c *gin.Context) {
	var req dto.ProductCloneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.CloneFromUpstream(c.Request.Context(), req, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// BatchCloneFromUpstream godoc
// @Summary 批量从上游商品克隆创建销售商品（按百分比定价）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param request body dto.ProductBatchCloneRequest true "批量克隆参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/clone/batch [post]
func (h *ProductHandler) BatchCloneFromUpstream(c *gin.Context) {
	var req dto.ProductBatchCloneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.CloneFromUpstreamBatch(c.Request.Context(), req, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询商品详情
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id} [get]
func (h *ProductHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
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

// Update godoc
// @Summary 更新商品
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Param request body dto.ProductUpdateRequest true "商品参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id} [put]
func (h *ProductHandler) Update(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.ProductUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.Update(c.Request.Context(), id, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除商品
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id} [delete]
func (h *ProductHandler) Delete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	if err := h.productService.Delete(c.Request.Context(), id, operatorID, operatorName); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}

// Publish godoc
// @Summary 上架商品
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id}/publish [post]
func (h *ProductHandler) Publish(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.Publish(c.Request.Context(), id, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Unpublish godoc
// @Summary 下架商品
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id}/unpublish [post]
func (h *ProductHandler) Unpublish(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.Unpublish(c.Request.Context(), id, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdatePrice godoc
// @Summary 更新商品价格
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Param request body dto.ProductPriceRequest true "价格参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id}/price [put]
func (h *ProductHandler) UpdatePrice(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.ProductPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.UpdatePrice(c.Request.Context(), id, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// SetFeatured godoc
// @Summary 设置前台推荐位
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Param request body dto.ProductFeaturedRequest true "推荐参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id}/featured [post]
func (h *ProductHandler) SetFeatured(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.ProductFeaturedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.SetFeatured(c.Request.Context(), id, req.Featured, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ListHistory godoc
// @Summary 查询商品变更历史
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id}/history [get]
func (h *ProductHandler) ListHistory(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.productService.ListHistory(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ListSpecs godoc
// @Summary 查询商品规格列表
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/catalog/products/{id}/specs [get]
func (h *ProductHandler) ListSpecs(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.productService.ListSpecs(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
