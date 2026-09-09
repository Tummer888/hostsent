// Package handler 提供用户中心主机模块 HTTP 接口。
package handler

import (
	"github.com/gin-gonic/gin"
	"strconv"

	"hostsent/backend/internal/modules/uc/instance/dto"
	"hostsent/backend/internal/modules/uc/instance/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// InstanceHandler 用户中心主机 HTTP 处理器。
type InstanceHandler struct {
	instanceService service.InstanceService
}

// NewInstanceHandler 创建用户中心主机处理器。
func NewInstanceHandler(instanceService service.InstanceService) *InstanceHandler {
	return &InstanceHandler{instanceService: instanceService}
}

func currentUserID(c *gin.Context) (uint64, bool) {
	claims, ok := middleware.GetClaims(c)
	if !ok || claims.UserID == 0 {
		return 0, false
	}
	return claims.UserID, true
}

// List godoc
// @Summary 我的主机列表
// @Tags 用户中心-主机
// @Security BearerAuth
// @Param live query bool false "是否刷新上游实时状态"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/instances [get]
func (h *InstanceHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var query dto.ListQuery
	_ = c.ShouldBindQuery(&query)
	resp, err := h.instanceService.List(c.Request.Context(), userID, query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Detail godoc
// @Summary 主机详情
// @Tags 用户中心-主机
// @Security BearerAuth
// @Param id path uint true "主机记录ID"
// @Param live query bool false "是否刷新上游实时状态"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/instances/{id} [get]
func (h *InstanceHandler) Detail(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	id := pathUint(c, "id")
	if id == 0 {
		response.Error(c, apperrors.New(50001, "参数错误"))
		return
	}
	var live bool
	if v := c.Query("live"); v == "1" || v == "true" {
		live = true
	}
	info, err := h.instanceService.Detail(c.Request.Context(), userID, id, live)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, info)
}

// Power godoc
// @Summary 电源操作（on/off/reboot/hard_off/hard_reboot）
// @Tags 用户中心-主机
// @Security BearerAuth
// @Param id path uint true "主机记录ID"
// @Param body body dto.PowerRequest true "操作"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/instances/{id}/power [post]
func (h *InstanceHandler) Power(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	id := pathUint(c, "id")
	if id == 0 {
		response.Error(c, apperrors.New(50001, "参数错误"))
		return
	}
	var req dto.PowerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	if err := h.instanceService.Power(c.Request.Context(), userID, id, req.Action); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "操作成功")
}

// VNC godoc
// @Summary 获取远程控制台
// @Tags 用户中心-主机
// @Security BearerAuth
// @Param id path uint true "主机记录ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/instances/{id}/vnc [post]
func (h *InstanceHandler) VNC(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	id := pathUint(c, "id")
	if id == 0 {
		response.Error(c, apperrors.New(50001, "参数错误"))
		return
	}
	res, err := h.instanceService.VNC(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, res)
}

func pathUint(c *gin.Context, key string) uint64 {
	v, _ := strconv.ParseUint(c.Param(key), 10, 64)
	return v
}
