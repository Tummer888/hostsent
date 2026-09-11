// Package handler 提供实例运维管理台的 HTTP 接口。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/instance/dto"
	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// InstanceHandler 实例运维台 HTTP 处理器。
type InstanceHandler struct {
	svc instanceservice.InstanceService
}

// NewInstanceHandler 创建实例运维台处理器。
func NewInstanceHandler(svc instanceservice.InstanceService) *InstanceHandler {
	return &InstanceHandler{svc: svc}
}

// pathID 解析路径参数 id。
func pathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return 0, false
	}
	return id, true
}

// operator 从鉴权上下文提取操作人（写入操作流水）。
func operator(c *gin.Context) instanceservice.Operator {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return instanceservice.Operator{Type: "admin", ID: claims.AdminID, Name: claims.Username}
	}
	return instanceservice.Operator{Type: "admin"}
}

// writeError 将实例运维域业务错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, instanceservice.ErrInstanceNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, instanceservice.ErrInvalidAction):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, instanceservice.ErrStatusConflict):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, instanceservice.ErrCapabilityUnsupported):
		return apperrors.New(20004, err.Error())
	case errors.Is(err, instanceservice.ErrProviderUnavailable):
		return apperrors.New(20005, err.Error())
	case errors.Is(err, instanceservice.ErrPermissionDenied):
		return apperrors.New(10003, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}

// List 跨用户实例分页列表。
// @Summary 实例运维列表（跨用户）
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param keyword query string false "实例标识/实例名"
// @Param user_keyword query string false "用户名/邮箱/手机号"
// @Param user_id query int false "用户 ID"
// @Param provider_id query int false "服务商 ID"
// @Param status query string false "服务状态"
// @Param expire_state query string false "到期筛选：all/expiring/expired/none"
// @Param expire_within_days query int false "临期天数，默认 7"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances [get]
func (h *InstanceHandler) List(c *gin.Context) {
	var query dto.ListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), &query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Stats 概览统计。
// @Summary 实例运维概览统计
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/stats [get]
func (h *InstanceHandler) Stats(c *gin.Context) {
	var query dto.StatsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Stats(c.Request.Context(), &query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Detail 实例详情。
// @Summary 实例详情（含归属用户/服务商/能力位）
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Param live query bool false "是否回源刷新"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id} [get]
func (h *InstanceHandler) Detail(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	live := c.Query("live") == "1" || c.Query("live") == "true"
	resp, err := h.svc.Detail(c.Request.Context(), id, live)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Operations 操作流水分页。
// @Summary 实例操作流水
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id}/operations [get]
func (h *InstanceHandler) Operations(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var query dto.OperationListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Operations(c.Request.Context(), id, &query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Related 关联订单/续费/工单。
// @Summary 实例关联业务记录
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id}/related [get]
func (h *InstanceHandler) Related(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Related(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Sync 单实例回源刷新。
// @Summary 回源刷新实例状态
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id}/sync [post]
func (h *InstanceHandler) Sync(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Sync(c.Request.Context(), operator(c), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Power 电源操作。
// @Summary 实例电源操作（on/off/hard_off/reboot/hard_reboot）
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Param body body dto.PowerRequest true "操作"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id}/power [post]
func (h *InstanceHandler) Power(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.PowerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.svc.Power(c.Request.Context(), operator(c), id, req.Action); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "操作成功")
}

// VNC 获取远程控制台。
// @Summary 实例远程控制台
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id}/vnc [post]
func (h *InstanceHandler) VNC(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.VNC(c.Request.Context(), operator(c), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Resize 变配。
// @Summary 实例变配
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Param body body dto.ResizeRequest true "目标规格"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id}/resize [post]
func (h *InstanceHandler) Resize(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.ResizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.svc.Resize(c.Request.Context(), operator(c), id, &req); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "变配指令已提交")
}

// SetRemark 写管理员备注。
// @Summary 设置实例管理员备注
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Param body body dto.RemarkRequest true "备注"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id}/remark [put]
func (h *InstanceHandler) SetRemark(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.RemarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.svc.SetRemark(c.Request.Context(), id, req.Remark); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "备注已保存")
}

// Destroy 销毁实例。
// @Summary 销毁实例（需二次确认实例标识）
// @Tags 管理端-实例运维
// @Security BearerAuth
// @Param id path int true "实例记录 ID"
// @Param body body dto.DestroyRequest true "销毁确认"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id} [delete]
func (h *InstanceHandler) Destroy(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.DestroyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.svc.Destroy(c.Request.Context(), operator(c), id, &req); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "实例已销毁")
}
