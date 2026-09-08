package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// SpecHandler 规格管理入口（规格模板 + 规格映射）。
type SpecHandler struct {
	templateService service.SpecTemplateService
	mappingService  service.SpecMappingService
}

// NewSpecHandler 创建规格管理入口。
func NewSpecHandler(templateService service.SpecTemplateService, mappingService service.SpecMappingService) *SpecHandler {
	return &SpecHandler{templateService: templateService, mappingService: mappingService}
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
