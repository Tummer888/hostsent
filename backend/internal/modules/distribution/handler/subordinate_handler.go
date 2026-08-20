package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/service"
)

type SubordinateHandler struct {
	service service.SubordinateService
}

func NewSubordinateHandler(service service.SubordinateService) *SubordinateHandler {
	return &SubordinateHandler{service: service}
}

// List godoc
// @Summary 下级成员列表
// @Description 获取分销代理的下级成员列表
// @Tags 分销管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param agent_id query int false "所属代理ID"
// @Param level_depth query int false "层级(1:直接下级, 2:间接下级)"
// @Param keyword query string false "关键词(用户名/姓名/邮箱/手机号)"
// @Success 200 {object} dto.APIResponse[dto.SubordinateListResponse]
// @Router /api/v1/distribution/subordinates [get]
func (h *SubordinateHandler) List(c *gin.Context) {
	var query dto.SubordinateListQuery
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
// @Summary 下级成员详情
// @Description 获取单个下级成员绑定详情
// @Tags 分销管理
// @Produce json
// @Param id path int true "记录ID"
// @Success 200 {object} dto.APIResponse[dto.SubordinateInfo]
// @Router /api/v1/distribution/subordinates/{id} [get]
func (h *SubordinateHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Create godoc
// @Summary 绑定下级成员
// @Description 手动建立分销下级关系
// @Tags 分销管理
// @Accept json
// @Produce json
// @Param request body dto.SubordinateCreateRequest true "绑定参数"
// @Success 200 {object} dto.APIResponse[dto.SubordinateInfo]
// @Router /api/v1/distribution/subordinates [post]
func (h *SubordinateHandler) Create(c *gin.Context) {
	var req dto.SubordinateCreateRequest
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
// @Summary 更新下级绑定信息
// @Description 更新下级成员绑定记录
// @Tags 分销管理
// @Accept json
// @Produce json
// @Param id path int true "记录ID"
// @Param request body dto.SubordinateUpdateRequest true "绑定参数"
// @Success 200 {object} dto.APIResponse[dto.SubordinateInfo]
// @Router /api/v1/distribution/subordinates/{id} [put]
func (h *SubordinateHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.SubordinateUpdateRequest
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
// @Summary 解除下级绑定
// @Description 解除分销代理与该下级的绑定关系
// @Tags 分销管理
// @Produce json
// @Param id path int true "记录ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/distribution/subordinates/{id} [delete]
func (h *SubordinateHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}
