package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/service"
)

type AgentLevelHandler struct {
	service service.AgentLevelService
}

func NewAgentLevelHandler(service service.AgentLevelService) *AgentLevelHandler {
	return &AgentLevelHandler{service: service}
}

// List godoc
// @Summary 分销等级列表
// @Description 获取分销等级列表
// @Tags 分销管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "状态"
// @Param keyword query string false "关键词(名称/编码)"
// @Success 200 {object} dto.APIResponse[dto.AgentLevelListResponse]
// @Router /api/v1/distribution/agent-levels [get]
func (h *AgentLevelHandler) List(c *gin.Context) {
	var query dto.AgentLevelListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	items, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": items, "timestamp": time.Now().Unix()})
}

// Get godoc
// @Summary 分销等级详情
// @Description 获取单个分销等级详情
// @Tags 分销管理
// @Produce json
// @Param id path int true "等级ID"
// @Success 200 {object} dto.APIResponse[dto.AgentLevelInfo]
// @Router /api/v1/distribution/agent-levels/{id} [get]
func (h *AgentLevelHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Create godoc
// @Summary 创建分销等级
// @Description 创建新的分销等级
// @Tags 分销管理
// @Accept json
// @Produce json
// @Param request body dto.AgentLevelCreateRequest true "等级参数"
// @Success 200 {object} dto.APIResponse[dto.AgentLevelInfo]
// @Router /api/v1/distribution/agent-levels [post]
func (h *AgentLevelHandler) Create(c *gin.Context) {
	var req dto.AgentLevelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Update godoc
// @Summary 更新分销等级
// @Description 更新分销等级信息
// @Tags 分销管理
// @Accept json
// @Produce json
// @Param id path int true "等级ID"
// @Param request body dto.AgentLevelUpdateRequest true "等级参数"
// @Success 200 {object} dto.APIResponse[dto.AgentLevelInfo]
// @Router /api/v1/distribution/agent-levels/{id} [put]
func (h *AgentLevelHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.AgentLevelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Delete godoc
// @Summary 删除分销等级
// @Description 删除分销等级记录
// @Tags 分销管理
// @Produce json
// @Param id path int true "等级ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/distribution/agent-levels/{id} [delete]
func (h *AgentLevelHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}
