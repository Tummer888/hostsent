package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
// @Router /api/v1/admin/product/products [get]
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
// @Router /api/v1/admin/product/products [post]
func (h *ProductHandler) Create(c *gin.Context) {
	var req dto.ProductCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.Create(c.Request.Context(), req, operatorID, operatorName)
	if err != nil {
		// 编码重复是可纠正的输入问题（换个 SKU 编码即可），用 20001 与其它参数错误同档；
		// 落 50001 会让前端把它当成服务器故障，运营也只看到一行 SQL 原文。
		if errors.Is(err, service.ErrProductCodeExists) {
			response.Error(c, apperrors.New(20001, err.Error()))
			return
		}
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

// SetSpecPassthrough godoc
// @Summary 设置商品「仅透传上游参数」（上架门禁的显式放行项）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Param request body dto.ProductSpecPassthroughRequest true "开关"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/spec-passthrough [post]
func (h *ProductHandler) SetSpecPassthrough(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "商品 ID 非法"))
		return
	}
	var req dto.ProductSpecPassthroughRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	info, err := h.productService.SetSpecPassthrough(c.Request.Context(), id, req.Enabled, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, info)
}

// SyncRenewalPrices godoc
// @Summary 把商品当前周期价同步到已绑定续费价的实例（支持 dry_run 试算与 clear 清除绑定）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Param request body dto.ProductRenewalPriceSyncRequest true "同步参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/sync-renewal-price [post]
func (h *ProductHandler) SyncRenewalPrices(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "商品 ID 非法"))
		return
	}
	var req dto.ProductRenewalPriceSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	result, err := h.productService.SyncRenewalPrices(c.Request.Context(), id, req.Clear, req.DryRun, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, result)
}

// BatchPublishProducts godoc
// @Summary 批量上架商品（逐条过门禁，失败逐条回报）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param request body dto.ProductBatchActionRequest true "商品 ID 列表"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/batch-publish [post]
func (h *ProductHandler) BatchPublishProducts(c *gin.Context) {
	h.handleBatchAction(c, h.productService.BatchPublish)
}

// BatchUnpublishProducts godoc
// @Summary 批量下架商品
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param request body dto.ProductBatchActionRequest true "商品 ID 列表"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/batch-unpublish [post]
func (h *ProductHandler) BatchUnpublishProducts(c *gin.Context) {
	h.handleBatchAction(c, h.productService.BatchUnpublish)
}

// BatchDeleteProducts godoc
// @Summary 批量删除商品
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param request body dto.ProductBatchActionRequest true "商品 ID 列表"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/batch-delete [post]
func (h *ProductHandler) BatchDeleteProducts(c *gin.Context) {
	h.handleBatchAction(c, h.productService.BatchDelete)
}

// handleBatchAction 批量上架/下架/删除的公共入口：三个接口只是执行的动作不同。
func (h *ProductHandler) handleBatchAction(c *gin.Context, run func(context.Context, []uint64, uint64, string) (*dto.ProductBatchActionResponse, error)) {
	var req dto.ProductBatchActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := run(c.Request.Context(), req.IDs, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ApplyUpstreamMarkup godoc
// @Summary 设定上游商品加价规则并重算已导入商品售价（上游商品页调价）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param request body dto.ProductUpstreamMarkupRequest true "调价参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/upstream-markup [post]
func (h *ProductHandler) ApplyUpstreamMarkup(c *gin.Context) {
	var req dto.ProductUpstreamMarkupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	_, operatorName := operatorFromContext(c)
	affected, err := h.productService.ApplyUpstreamMarkupRule(c.Request.Context(), req.ResourceProductID, req.MarkupType, req.MarkupValue, operatorName, req.Remark)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	msg := "该上游商品还没有导入成商品，规则已记录，导入后按此定价"
	if affected > 0 {
		msg = fmt.Sprintf("已按新规则重算 %d 个商品售价", affected)
	}
	details := []string{}
	if strings.TrimSpace(req.Remark) != "" {
		details = append(details, req.Remark)
	}
	response.Success(c, dto.ProductUpstreamMarkupResponse{Affected: affected, Message: msg, Details: details})
}

// Get godoc
// @Summary 查询商品详情
// @Tags 产品管理-商品
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/products/{id} [get]
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
// @Router /api/v1/admin/product/products/{id} [put]
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
// @Router /api/v1/admin/product/products/{id} [delete]
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
// @Router /api/v1/admin/product/products/{id}/publish [post]
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
// @Router /api/v1/admin/product/products/{id}/unpublish [post]
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
// @Router /api/v1/admin/product/products/{id}/price [put]
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
// @Router /api/v1/admin/product/products/{id}/featured [post]
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
// @Router /api/v1/admin/product/products/{id}/history [get]
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
// @Router /api/v1/admin/product/products/{id}/specs [get]
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

// CreateSpec godoc
// @Summary 新增商品规格变体（SKU）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Param request body dto.ProductSpecRequest true "规格参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/specs [post]
func (h *ProductHandler) CreateSpec(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.ProductSpecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.CreateSpec(c.Request.Context(), id, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, specError(err))
		return
	}
	response.Success(c, resp)
}

// GenerateSpecFromTemplate godoc
// @Summary 按规格模板为自营商品生成 SKU 并建立平台绑定
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Param request body dto.SpecTemplateGenerateRequest true "生成参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/specs/generate [post]
func (h *ProductHandler) GenerateSpecFromTemplate(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.SpecTemplateGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.GenerateSpecFromTemplate(c.Request.Context(), id, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, specError(err))
		return
	}
	response.Success(c, resp)
}

// UpdateSpec godoc
// @Summary 更新商品规格变体（SKU）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Param specId path int true "规格 ID"
// @Param request body dto.ProductSpecRequest true "规格参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/specs/{specId} [put]
func (h *ProductHandler) UpdateSpec(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	specID, ok := pathID(c, "specId")
	if !ok {
		return
	}
	var req dto.ProductSpecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.UpdateSpec(c.Request.Context(), id, specID, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, specError(err))
		return
	}
	response.Success(c, resp)
}

// DeleteSpec godoc
// @Summary 删除商品规格变体（SKU）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Param specId path int true "规格 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/specs/{specId} [delete]
func (h *ProductHandler) DeleteSpec(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	specID, ok := pathID(c, "specId")
	if !ok {
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	if err := h.productService.DeleteSpec(c.Request.Context(), id, specID, operatorID, operatorName); err != nil {
		response.Error(c, specError(err))
		return
	}
	response.SuccessMessage(c, "success")
}

// ListConfigOptions godoc
// @Summary 查询商品可配置项（含自营项）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/config-options [get]
func (h *ProductHandler) ListConfigOptions(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.productService.ListConfigOptions(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// SaveConfigOptions godoc
// @Summary 保存商品可配置项（覆盖式，支持 source/source_key 标注）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Param request body dto.ProductConfigOptionsRequest true "配置组"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/config-options [put]
func (h *ProductHandler) SaveConfigOptions(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.ProductConfigOptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	groups := make([]interface{}, 0, len(req.Groups))
	for _, raw := range req.Groups {
		var item interface{}
		if err := json.Unmarshal(raw, &item); err != nil {
			response.Error(c, apperrors.New(20001, "配置组结构非法"))
			return
		}
		groups = append(groups, item)
	}
	operatorID, operatorName := operatorFromContext(c)
	if err := h.productService.SaveConfigOptions(c.Request.Context(), id, groups, operatorID, operatorName); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}

// specError 把 SKU 业务错误映射为明确错误码（否则前端只能看到 50001 通用错误）。
func specError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrSpecNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrSpecCodeExists), errors.Is(err, service.ErrSpecInvalid):
		return apperrors.New(20001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}

// UpstreamAxisOptions godoc
// @Summary 查询上游商品的 SKU 维度设置（可做维度的配置项 + 当前选择 + 组合数）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/upstream-axes [get]
func (h *ProductHandler) UpstreamAxisOptions(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.productService.UpstreamAxisOptions(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// SaveUpstreamAxisOptions godoc
// @Summary 保存上游商品的 SKU 维度选择（不立即生成规格）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Param request body dto.UpstreamAxisSaveRequest true "维度选择"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/upstream-axes [put]
func (h *ProductHandler) SaveUpstreamAxisOptions(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.UpstreamAxisSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(40001, "参数错误"))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.SaveUpstreamAxisOptions(c.Request.Context(), id, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GenerateUpstreamAxisSpecs godoc
// @Summary 按已保存的 SKU 维度展开生成规格（幂等，可重复执行）
// @Tags 产品管理-商品
// @Security BearerAuth
// @Param id path int true "商品 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/products/{id}/upstream-axes/generate [post]
func (h *ProductHandler) GenerateUpstreamAxisSpecs(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.productService.GenerateUpstreamAxisSpecs(c.Request.Context(), id, operatorID, operatorName)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
