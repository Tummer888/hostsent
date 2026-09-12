// Package handler 提供任务队列 HTTP 入口。
package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/resource/taskqueue/dto"
	"hostsent/backend/internal/modules/admin/resource/taskqueue/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// TaskQueueHandler 任务队列处理入口。
type TaskQueueHandler struct {
	svc service.TaskQueueService
}

// NewTaskQueueHandler 创建任务队列处理入口。
func NewTaskQueueHandler(svc service.TaskQueueService) *TaskQueueHandler {
	return &TaskQueueHandler{svc: svc}
}

// List godoc
// @Summary 查询平台动作任务队列（开通/实例动作/续费/同步）
// @Description 聚合四类平台动作，每行标注是否到达上游；支持类别/状态/到达状态/渠道/关键词/时间筛选。
// @Tags 资源管理-运维
// @Security BearerAuth
// @Param category query string false "任务类别 provision/instance_action/renewal/sync"
// @Param status query string false "原始状态"
// @Param upstream_state query string false "上游到达状态 reached/not_reached/pending/not_applicable/skipped"
// @Param provider_id query int false "渠道 ID"
// @Param keyword query string false "关键词"
// @Param created_from query string false "创建时间起"
// @Param created_to query string false "创建时间止"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/resource/task-queue [get]
func (h *TaskQueueHandler) List(c *gin.Context) {
	var query dto.TaskQueueListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCategory) {
			response.Error(c, apperrors.New(20001, err.Error()))
			return
		}
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Categories godoc
// @Summary 查询任务队列类别元信息（页签）
// @Tags 资源管理-运维
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/resource/task-queue/categories [get]
func (h *TaskQueueHandler) Categories(c *gin.Context) {
	resp, err := h.svc.Categories(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
