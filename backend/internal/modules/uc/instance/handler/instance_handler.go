// Package handler 提供用户中心主机模块 HTTP 接口。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

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

// currentUserID 返回数据归属账号 ID（P4-04）：子账号取主账号，实例一律归属主账号。
func currentUserID(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, false
	}
	return userID, true
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

// Destroy godoc
// @Summary 销毁实例（不可逆）
// @Description 用户自助销毁：归属校验 + 二次确认标识；不触发退款/余额返还（doc91 §6.4）
// @Tags 用户中心-主机
// @Security BearerAuth
// @Param id path uint true "主机记录ID"
// @Param body body dto.DestroyRequest true "销毁确认"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/instances/{id} [delete]
func (h *InstanceHandler) Destroy(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	id := pathUint(c, "id")
	if id == 0 {
		response.Error(c, apperrors.New(20001, "参数错误"))
		return
	}
	var req dto.DestroyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	// 操作人取真实登录者（子账号），归属仍用主账号 ID；路由已挂 rejectSub，
	// 这里的区分是为将来放开子账号时流水仍能回答「是谁点的」。
	actorID := middleware.ActorUserID(c)
	actorName := middleware.ActorUsername(c)
	if err := h.instanceService.Destroy(c.Request.Context(), userID, actorID, actorName, id, &req); err != nil {
		switch {
		case errors.Is(err, service.ErrInstanceNotFoundOrDenied):
			response.Error(c, apperrors.New(20002, err.Error()))
		case errors.Is(err, service.ErrDestroyUnsupported):
			response.Error(c, apperrors.New(20003, err.Error()))
		default:
			response.Error(c, apperrors.New(50001, err.Error()))
		}
		return
	}
	response.SuccessMessage(c, "销毁指令已提交")
}

func pathUint(c *gin.Context, key string) uint64 {
	v, _ := strconv.ParseUint(c.Param(key), 10, 64)
	return v
}
