package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// SpecHandler 规格管理入口（规格模板 + 规格映射 + 规格契约）。
type SpecHandler struct {
	templateService service.SpecTemplateService
	mappingService  service.SpecMappingService
	contractService service.SpecContractService
}

// NewSpecHandler 创建规格管理入口。
func NewSpecHandler(templateService service.SpecTemplateService, mappingService service.SpecMappingService, contractService service.SpecContractService) *SpecHandler {
	return &SpecHandler{templateService: templateService, mappingService: mappingService, contractService: contractService}
}

// ===== 规格模板 =====

// ListTemplates godoc
// @Summary 查询规格模板列表
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param keyword query string false "关键字"
// @Param spec_family query string false "规格族"
// @Param status query int false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/templates [get]
func (h *SpecHandler) ListTemplates(c *gin.Context) {
	var query dto.SpecTemplateQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.templateService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// CreateTemplate godoc
// @Summary 创建规格模板
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.SpecTemplateRequest true "规格模板参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/templates [post]
func (h *SpecHandler) CreateTemplate(c *gin.Context) {
	var req dto.SpecTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.templateService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GetTemplate godoc
// @Summary 查询规格模板详情
// @Tags 产品管理-规格
// @Param id path int true "规格模板 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/templates/{id} [get]
func (h *SpecHandler) GetTemplate(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.templateService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdateTemplate godoc
// @Summary 更新规格模板
// @Tags 产品管理-规格
// @Param id path int true "规格模板 ID"
// @Param request body dto.SpecTemplateRequest true "规格模板参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/templates/{id} [put]
func (h *SpecHandler) UpdateTemplate(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.SpecTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.templateService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// DeleteTemplate godoc
// @Summary 删除规格模板
// @Tags 产品管理-规格
// @Param id path int true "规格模板 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/templates/{id} [delete]
func (h *SpecHandler) DeleteTemplate(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.templateService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}

// ===== 规格映射 =====

// ListMappings godoc
// @Summary 查询规格映射列表
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param provider_type query string false "上游类型"
// @Param keyword query string false "关键字"
// @Param status query int false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/mappings [get]
func (h *SpecHandler) ListMappings(c *gin.Context) {
	var query dto.SpecMappingQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.mappingService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// CreateMapping godoc
// @Summary 创建规格映射
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.SpecMappingRequest true "规格映射参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/mappings [post]
func (h *SpecHandler) CreateMapping(c *gin.Context) {
	var req dto.SpecMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.mappingService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GetMapping godoc
// @Summary 查询规格映射详情
// @Tags 产品管理-规格
// @Param id path int true "规格映射 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/mappings/{id} [get]
func (h *SpecHandler) GetMapping(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.mappingService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdateMapping godoc
// @Summary 更新规格映射
// @Tags 产品管理-规格
// @Param id path int true "规格映射 ID"
// @Param request body dto.SpecMappingRequest true "规格映射参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/mappings/{id} [put]
func (h *SpecHandler) UpdateMapping(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.SpecMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.mappingService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// BindMapping godoc
// @Summary 绑定规格映射（映射到平台规格）
// @Tags 产品管理-规格
// @Param id path int true "规格映射 ID"
// @Param request body dto.SpecMappingBindRequest true "绑定参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/mappings/{id}/bind [post]
func (h *SpecHandler) BindMapping(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.SpecMappingBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.mappingService.Bind(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// DeleteMapping godoc
// @Summary 删除规格映射
// @Tags 产品管理-规格
// @Param id path int true "规格映射 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/mappings/{id} [delete]
func (h *SpecHandler) DeleteMapping(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.mappingService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}

// ===== 规格契约（P2/T2.5、T2.6）=====

// ListAtoms godoc
// @Summary 查询规格原子字典
// @Tags 产品管理-规格
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/atoms [get]
func (h *SpecHandler) ListAtoms(c *gin.Context) {
	resp, err := h.contractService.ListAtoms(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ValidateSpec godoc
// @Summary 按原子字典校验规格
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.SpecValidateRequest true "规格取值"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/validate [post]
func (h *SpecHandler) ValidateSpec(c *gin.Context) {
	var req dto.SpecValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, h.contractService.ValidateSpec(c.Request.Context(), req))
}

// ListExternalSpecs godoc
// @Summary 查询外部规格快照
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param provider_type query string false "渠道类型"
// @Param status query string false "状态 active/offline"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/external-specs [get]
func (h *SpecHandler) ListExternalSpecs(c *gin.Context) {
	resp, err := h.contractService.ListExternalSpecs(c.Request.Context(), c.Query("provider_type"), c.Query("status"))
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpsertExternalSpec godoc
// @Summary 登记/刷新外部规格快照
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.ExternalSpecUpsertRequest true "外部规格"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/external-specs [post]
func (h *SpecHandler) UpsertExternalSpec(c *gin.Context) {
	var req dto.ExternalSpecUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.contractService.UpsertExternalSpec(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ListBindings godoc
// @Summary 查询规格绑定
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param external_spec_id query int false "外部规格 ID"
// @Param product_spec_id query int false "商品 SKU ID（自营链路，T4.2）"
// @Param status query string false "绑定状态"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/bindings [get]
func (h *SpecHandler) ListBindings(c *gin.Context) {
	externalSpecID := uint64(0)
	if raw := c.Query("external_spec_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			externalSpecID = id
		}
	}
	productSpecID := uint64(0)
	if raw := c.Query("product_spec_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			productSpecID = id
		}
	}
	resp, err := h.contractService.ListBindings(c.Request.Context(), externalSpecID, productSpecID, c.Query("status"))
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpsertBinding godoc
// @Summary 建立/更新规格绑定
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.SpecBindingUpsertRequest true "绑定参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/bindings [post]
func (h *SpecHandler) UpsertBinding(c *gin.Context) {
	var req dto.SpecBindingUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.contractService.UpsertBinding(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ConfirmBinding godoc
// @Summary 人工确认规格绑定
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param id path int true "绑定 ID"
// @Param request body dto.SpecBindingConfirmRequest true "确认参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/spec/bindings/{id}/confirm [post]
func (h *SpecHandler) ConfirmBinding(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.SpecBindingConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID := adminID(c)
	resp, err := h.contractService.ConfirmBinding(c.Request.Context(), id, req, operatorID)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
