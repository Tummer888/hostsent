// Package handler 提供系统配置模块的 HTTP 接口。
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/system/dto"
	"hostsent/backend/internal/modules/admin/system/service"
)

// ConfigHandler 系统配置接口处理器。
type ConfigHandler struct {
	configService service.ConfigService
}

// NewConfigHandler 创建系统配置接口处理器。
func NewConfigHandler(configService service.ConfigService) *ConfigHandler {
	return &ConfigHandler{configService: configService}
}

// List godoc
// @Summary 配置项列表
// @Description 分页查询系统配置，支持按分组过滤与关键字模糊搜索
// @Tags 系统配置
// @Produce json
// @Security BearerAuth
// @Param group query string false "配置分组"
// @Param keyword query string false "关键字（匹配配置键/描述）"
// @Param page query int false "页码，默认 1"
// @Param page_size query int false "每页条数，默认 10，最大 100"
// @Success 200 {object} dto.APIResponse[dto.ConfigListResponse]
// @Router /api/v1/admin/system/configs [get]
func (h *ConfigHandler) List(c *gin.Context) {
	var query dto.ConfigListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	items, total, err := h.configService.List(c.Request.Context(), query.Group, query.Keyword, query.Page, query.PageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	resp := dto.ConfigListResponse{
		Items: items,
		Meta: dto.ConfigMeta{
			Page:     query.Page,
			PageSize: query.PageSize,
			Total:    total,
		},
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

// GetByKey godoc
// @Summary 按配置键查询
// @Description 根据配置键获取单个配置项
// @Tags 系统配置
// @Produce json
// @Security BearerAuth
// @Param key path string true "配置键"
// @Success 200 {object} dto.APIResponse[dto.ConfigInfo]
// @Router /api/v1/admin/system/configs/{key} [get]
func (h *ConfigHandler) GetByKey(c *gin.Context) {
	config, err := h.configService.GetByKey(c.Request.Context(), c.Param("key"))
	if err != nil {
		h.replyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": config, "timestamp": time.Now().Unix()})
}

// Create godoc
// @Summary 创建配置项
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ConfigCreateRequest true "配置参数"
// @Success 200 {object} dto.APIResponse[dto.ConfigInfo]
// @Router /api/v1/admin/system/configs [post]
func (h *ConfigHandler) Create(c *gin.Context) {
	var req dto.ConfigCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	config, err := h.configService.Create(c.Request.Context(), req)
	if err != nil {
		h.replyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": config, "timestamp": time.Now().Unix()})
}

// Update godoc
// @Summary 更新配置项
// @Description 配置键创建后不可修改
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "配置ID"
// @Param request body dto.ConfigUpdateRequest true "配置参数"
// @Success 200 {object} dto.APIResponse[dto.ConfigInfo]
// @Router /api/v1/admin/system/configs/{id} [put]
func (h *ConfigHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	config, err := h.configService.Update(c.Request.Context(), id, req)
	if err != nil {
		h.replyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": config, "timestamp": time.Now().Unix()})
}

// Delete godoc
// @Summary 删除配置项
// @Tags 系统配置
// @Produce json
// @Security BearerAuth
// @Param id path int true "配置ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/system/configs/{id} [delete]
func (h *ConfigHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.configService.Delete(c.Request.Context(), id); err != nil {
		h.replyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

// replyError 统一映射业务错误：不存在→404/20002，配置键重复→400/20004，其余→500/50001。
func (h *ConfigHandler) replyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 20002, "message": "配置项不存在", "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrConfigKeyDuplicate):
		c.JSON(http.StatusBadRequest, gin.H{"code": 20004, "message": err.Error(), "timestamp": time.Now().Unix()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
	}
}
