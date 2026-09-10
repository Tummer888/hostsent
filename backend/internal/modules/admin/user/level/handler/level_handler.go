// Package handler 提供用户等级模块的 HTTP 接口。
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/user/level/dto"
	"hostsent/backend/internal/modules/admin/user/level/service"
)

// UserLevelHandler 提供用户等级相关的 HTTP 接口。
type UserLevelHandler struct{ service service.UserLevelService }

// NewUserLevelHandler 创建用户等级处理器。
func NewUserLevelHandler(service service.UserLevelService) *UserLevelHandler {
	return &UserLevelHandler{service: service}
}

// List godoc
// @Summary 用户等级列表
// @Tags 用户等级
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "状态"
// @Param keyword query string false "关键词"
// @Success 200 {object} dto.APIResponse[dto.ListResponse]
// @Router /api/v1/admin/user-levels [get]
func (h *UserLevelHandler) List(c *gin.Context) {
	var query dto.ListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Get godoc
// @Summary 用户等级详情
// @Tags 用户等级
// @Produce json
// @Param id path int true "等级ID"
// @Success 200 {object} dto.APIResponse[dto.Info]
// @Router /api/v1/admin/user-levels/{id} [get]
func (h *UserLevelHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Create godoc
// @Summary 创建用户等级
// @Tags 用户等级
// @Accept json
// @Produce json
// @Param request body dto.CreateRequest true "等级参数"
// @Success 200 {object} dto.APIResponse[dto.Info]
// @Router /api/v1/admin/user-levels [post]
func (h *UserLevelHandler) Create(c *gin.Context) {
	var req dto.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Update godoc
// @Summary 更新用户等级
// @Tags 用户等级
// @Accept json
// @Produce json
// @Param id path int true "等级ID"
// @Param request body dto.UpdateRequest true "等级参数"
// @Success 200 {object} dto.APIResponse[dto.Info]
// @Router /api/v1/admin/user-levels/{id} [put]
func (h *UserLevelHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Delete godoc
// @Summary 删除用户等级
// @Tags 用户等级
// @Produce json
// @Param id path int true "等级ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/user-levels/{id} [delete]
func (h *UserLevelHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, "ok")
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
