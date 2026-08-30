package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/resource/provider/dto"
	"hostsent/backend/internal/modules/admin/resource/provider/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ProviderHandler 上游提供商处理入口
type ProviderHandler struct {
	providerService service.ProviderService
}

// NewProviderHandler 创建上游提供商处理入口
func NewProviderHandler(providerService service.ProviderService) *ProviderHandler {
	return &ProviderHandler{providerService: providerService}
}

// List godoc
// @Summary 查询上游提供商列表
// @Description 分页查询上游提供商
// @Tags 资源管理-上游提供商
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param keyword query string false "关键字"
// @Param provider_type query string false "提供商类型"
// @Param status query int false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/providers [get]
func (h *ProviderHandler) List(c *gin.Context) {
	var query dto.ProviderListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.providerService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 添加上游提供商
// @Tags 资源管理-上游提供商
// @Param request body dto.ProviderCreateRequest true "添加参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/providers [post]
func (h *ProviderHandler) Create(c *gin.Context) {
	var req dto.ProviderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.providerService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询上游提供商详情
// @Tags 资源管理-上游提供商
// @Param id path int true "提供商 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/providers/{id} [get]
func (h *ProviderHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	resp, err := h.providerService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新上游提供商
// @Tags 资源管理-上游提供商
// @Param id path int true "提供商 ID"
// @Param request body dto.ProviderUpdateRequest true "更新参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/providers/{id} [put]
func (h *ProviderHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.ProviderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.providerService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除上游提供商
// @Tags 资源管理-上游提供商
// @Param id path int true "提供商 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/providers/{id} [delete]
func (h *ProviderHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.providerService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}

// ListTypes godoc
// @Summary 获取支持的提供商类型
// @Tags 资源管理-上游提供商
// @Success 200 {object} response.Body
// @Router /api/v1/admin/resource/providers/types [get]
func (h *ProviderHandler) ListTypes(c *gin.Context) {
	response.Success(c, h.providerService.ListTypes(c.Request.Context()))
}

// TestConnection godoc
// @Summary 测试提供商连接
// @Tags 资源管理-上游提供商
// @Param id path int true "提供商 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/resource/providers/{id}/test [post]
func (h *ProviderHandler) TestConnection(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	resp, err := h.providerService.TestConnection(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ListPools godoc
// @Summary 查询资源池列表
// @Tags 资源管理-上游提供商
// @Security BearerAuth
// @Param provider_id query int false "提供商 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/pools [get]
func (h *ProviderHandler) ListPools(c *gin.Context) {
	var query dto.PoolListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.providerService.ListPools(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GetPool godoc
// @Summary 查询资源池详情
// @Tags 资源管理-上游提供商
// @Param id path int true "资源池 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/pools/{id} [get]
func (h *ProviderHandler) GetPool(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	resp, err := h.providerService.FindPool(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return 0, false
	}
	return id, true
}
