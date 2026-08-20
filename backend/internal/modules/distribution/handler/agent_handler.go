package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/service"
)

type AgentHandler struct {
	service service.AgentService
}

func NewAgentHandler(service service.AgentService) *AgentHandler {
	return &AgentHandler{service: service}
}

// List godoc
// @Summary 分销代理列表
// @Description 获取分销代理列表，支持分页、状态筛选与关键词搜索
// @Tags 分销管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "状态"
// @Param agent_level_id query int false "等级ID"
// @Param keyword query string false "关键词(用户名/姓名/邮箱/手机号)"
// @Success 200 {object} dto.APIResponse[dto.AgentListResponse]
// @Router /api/v1/distribution/agents [get]
func (h *AgentHandler) List(c *gin.Context) {
	var query dto.AgentListQuery
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
// @Summary 分销代理详情
// @Description 获取单个分销代理详情
// @Tags 分销管理
// @Produce json
// @Param id path int true "代理ID"
// @Success 200 {object} dto.APIResponse[dto.AgentInfo]
// @Router /api/v1/distribution/agents/{id} [get]
func (h *AgentHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Create godoc
// @Summary 创建分销代理
// @Description 手动创建或指定用户为分销代理
// @Tags 分销管理
// @Accept json
// @Produce json
// @Param request body dto.AgentCreateRequest true "代理参数"
// @Success 200 {object} dto.APIResponse[dto.AgentInfo]
// @Router /api/v1/distribution/agents [post]
func (h *AgentHandler) Create(c *gin.Context) {
	var req dto.AgentCreateRequest
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
// @Summary 更新分销代理
// @Description 更新分销代理信息
// @Tags 分销管理
// @Accept json
// @Produce json
// @Param id path int true "代理ID"
// @Param request body dto.AgentUpdateRequest true "代理参数"
// @Success 200 {object} dto.APIResponse[dto.AgentInfo]
// @Router /api/v1/distribution/agents/{id} [put]
func (h *AgentHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.AgentUpdateRequest
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
// @Summary 删除分销代理
// @Description 删除分销代理记录
// @Tags 分销管理
// @Produce json
// @Param id path int true "代理ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/distribution/agents/{id} [delete]
func (h *AgentHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}
