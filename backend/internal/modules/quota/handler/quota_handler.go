// Package handler 提供配额模块的 HTTP 接口。
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/quota/dto"
	"hostsent/backend/internal/modules/quota/service"
)

type ResourceQuotaHandler struct{ service service.ResourceQuotaService }
type QuotaTemplateHandler struct{ service service.QuotaTemplateService }
type UserLevelHandler struct{ service service.UserLevelService }
// QuotaAdjustmentHandler 提供配额调整记录相关的 HTTP 接口。
type QuotaAdjustmentHandler struct{ service service.QuotaAdjustmentService }

func NewResourceQuotaHandler(service service.ResourceQuotaService) *ResourceQuotaHandler { return &ResourceQuotaHandler{service: service} }
// NewQuotaTemplateHandler 创建配额模板处理器。
func NewQuotaTemplateHandler(service service.QuotaTemplateService) *QuotaTemplateHandler { return &QuotaTemplateHandler{service: service} }
func NewUserLevelHandler(service service.UserLevelService) *UserLevelHandler { return &UserLevelHandler{service: service} }
func NewQuotaAdjustmentHandler(service service.QuotaAdjustmentService) *QuotaAdjustmentHandler { return &QuotaAdjustmentHandler{service: service} }

// List godoc
// @Summary 资源配额列表
// @Description 获取用户资源配额列表
// @Tags 资源配额与等级
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param quota_type query string false "配额类型"
// @Param source query string false "来源"
// @Param status query string false "状态"
// @Param is_overallocated query string false "是否超配"
// @Param keyword query string false "关键词"
// @Success 200 {object} dto.APIResponse[dto.QuotaListResponse]
// @Router /api/v1/quotas [get]
func (h *ResourceQuotaHandler) List(c *gin.Context) {
	var query dto.QuotaListQuery
	if err := c.ShouldBindQuery(&query); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.List(c.Request.Context(), query)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Get godoc
// @Summary 资源配额详情
// @Description 获取单个资源配额详情
// @Tags 资源配额与等级
// @Produce json
// @Param id path int true "配额ID"
// @Success 200 {object} dto.APIResponse[dto.QuotaInfo]
// @Router /api/v1/quotas/{id} [get]
func (h *ResourceQuotaHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// GetByUser godoc
// @Summary 用户资源配额列表
// @Description 根据用户ID获取资源配额列表
// @Tags 资源配额与等级
// @Produce json
// @Param user_id path int true "用户ID"
// @Success 200 {object} dto.APIResponse[[]dto.QuotaInfo]
// @Router /api/v1/quotas/users/{user_id} [get]
func (h *ResourceQuotaHandler) GetByUser(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("user_id"), 10, 64)
	data, err := h.service.FindByUserID(c.Request.Context(), userID)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Adjust godoc
// @Summary 调整资源配额
// @Description 手工调整单个资源配额上限
// @Tags 资源配额与等级
// @Accept json
// @Produce json
// @Param id path int true "配额ID"
// @Param request body dto.QuotaAdjustRequest true "调整参数"
// @Success 200 {object} dto.APIResponse[dto.QuotaInfo]
// @Router /api/v1/quotas/{id}/adjust [post]
func (h *ResourceQuotaHandler) Adjust(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.QuotaAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.Adjust(c.Request.Context(), id, req)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// List godoc
// @Summary 配额模板列表
// @Description 获取配额模板列表
// @Tags 资源配额与等级
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "状态"
// @Param scope query string false "范围"
// @Param keyword query string false "关键词"
// @Success 200 {object} dto.APIResponse[dto.QuotaTemplateListResponse]
// @Router /api/v1/quota-templates [get]
func (h *QuotaTemplateHandler) List(c *gin.Context) {
	var query dto.QuotaTemplateListQuery
	if err := c.ShouldBindQuery(&query); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.List(c.Request.Context(), query)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Get godoc
// @Summary 配额模板详情
// @Description 获取单个配额模板详情
// @Tags 资源配额与等级
// @Produce json
// @Param id path int true "模板ID"
// @Success 200 {object} dto.APIResponse[dto.QuotaTemplateInfo]
// @Router /api/v1/quota-templates/{id} [get]
func (h *QuotaTemplateHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Create godoc
// @Summary 创建配额模板
// @Description 创建新的配额模板
// @Tags 资源配额与等级
// @Accept json
// @Produce json
// @Param request body dto.QuotaTemplateCreateRequest true "模板参数"
// @Success 200 {object} dto.APIResponse[dto.QuotaTemplateInfo]
// @Router /api/v1/quota-templates [post]
func (h *QuotaTemplateHandler) Create(c *gin.Context) {
	var req dto.QuotaTemplateCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.Create(c.Request.Context(), req)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Update godoc
// @Summary 更新配额模板
// @Description 更新配额模板信息
// @Tags 资源配额与等级
// @Accept json
// @Produce json
// @Param id path int true "模板ID"
// @Param request body dto.QuotaTemplateUpdateRequest true "模板参数"
// @Success 200 {object} dto.APIResponse[dto.QuotaTemplateInfo]
// @Router /api/v1/quota-templates/{id} [put]
func (h *QuotaTemplateHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.QuotaTemplateUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Delete godoc
// @Summary 删除配额模板
// @Description 删除配额模板记录
// @Tags 资源配额与等级
// @Produce json
// @Param id path int true "模板ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/quota-templates/{id} [delete]
func (h *QuotaTemplateHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil { serverError(c, err.Error()); return }
	success(c, "ok")
}

// List godoc
// @Summary 用户等级列表
// @Description 获取用户等级列表
// @Tags 资源配额与等级
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "状态"
// @Param default_template_id query int false "默认模板ID"
// @Param keyword query string false "关键词"
// @Success 200 {object} dto.APIResponse[dto.UserLevelListResponse]
// @Router /api/v1/user-levels [get]
func (h *UserLevelHandler) List(c *gin.Context) {
	var query dto.UserLevelListQuery
	if err := c.ShouldBindQuery(&query); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.List(c.Request.Context(), query)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Get godoc
// @Summary 用户等级详情
// @Description 获取单个用户等级详情
// @Tags 资源配额与等级
// @Produce json
// @Param id path int true "等级ID"
// @Success 200 {object} dto.APIResponse[dto.UserLevelInfo]
// @Router /api/v1/user-levels/{id} [get]
func (h *UserLevelHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Create godoc
// @Summary 创建用户等级
// @Description 创建新的用户等级
// @Tags 资源配额与等级
// @Accept json
// @Produce json
// @Param request body dto.UserLevelCreateRequest true "等级参数"
// @Success 200 {object} dto.APIResponse[dto.UserLevelInfo]
// @Router /api/v1/user-levels [post]
func (h *UserLevelHandler) Create(c *gin.Context) {
	var req dto.UserLevelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.Create(c.Request.Context(), req)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Update godoc
// @Summary 更新用户等级
// @Description 更新用户等级信息
// @Tags 资源配额与等级
// @Accept json
// @Produce json
// @Param id path int true "等级ID"
// @Param request body dto.UserLevelUpdateRequest true "等级参数"
// @Success 200 {object} dto.APIResponse[dto.UserLevelInfo]
// @Router /api/v1/user-levels/{id} [put]
func (h *UserLevelHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.UserLevelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Delete godoc
// @Summary 删除用户等级
// @Description 删除用户等级记录
// @Tags 资源配额与等级
// @Produce json
// @Param id path int true "等级ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/user-levels/{id} [delete]
func (h *UserLevelHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil { serverError(c, err.Error()); return }
	success(c, "ok")
}

// BindTemplate godoc
// @Summary 绑定默认模板
// @Description 为用户等级绑定默认配额模板
// @Tags 资源配额与等级
// @Accept json
// @Produce json
// @Param id path int true "等级ID"
// @Param request body dto.UserLevelBindTemplateRequest true "绑定参数"
// @Success 200 {object} dto.APIResponse[dto.UserLevelInfo]
// @Router /api/v1/user-levels/{id}/bind-template [post]
func (h *UserLevelHandler) BindTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.UserLevelBindTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.BindTemplate(c.Request.Context(), id, req)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// List godoc
// @Summary 配额调整记录列表
// @Description 获取配额调整记录列表
// @Tags 资源配额与等级
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param quota_code query string false "配额编码"
// @Param adjustment_type query string false "调整类型"
// @Param source query string false "来源"
// @Param operator_name query string false "操作人"
// @Success 200 {object} dto.APIResponse[dto.QuotaAdjustmentListResponse]
// @Router /api/v1/quota-adjustments [get]
func (h *QuotaAdjustmentHandler) List(c *gin.Context) {
	var query dto.QuotaAdjustmentListQuery
	if err := c.ShouldBindQuery(&query); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.List(c.Request.Context(), query)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// Get godoc
// @Summary 配额调整记录详情
// @Description 获取单个配额调整记录详情
// @Tags 资源配额与等级
// @Produce json
// @Param id path int true "记录ID"
// @Success 200 {object} dto.APIResponse[dto.QuotaAdjustmentInfo]
// @Router /api/v1/quota-adjustments/{id} [get]
func (h *QuotaAdjustmentHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": message, "timestamp": time.Now().Unix()})
}

func serverError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": message, "timestamp": time.Now().Unix()})
}

func success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}
