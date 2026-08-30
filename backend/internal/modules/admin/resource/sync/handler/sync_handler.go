package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/resource/sync/dto"
	"hostsent/backend/internal/modules/admin/resource/sync/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// SyncHandler 资源同步处理入口
type SyncHandler struct {
	syncService service.SyncService
}

// NewSyncHandler 创建资源同步处理入口
func NewSyncHandler(syncService service.SyncService) *SyncHandler {
	return &SyncHandler{syncService: syncService}
}

// ListTasks godoc
// @Summary 查询同步任务列表
// @Tags 资源管理-同步
// @Security BearerAuth
// @Param provider_id query int false "提供商 ID"
// @Param task_type query string false "任务类型"
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/sync/tasks [get]
func (h *SyncHandler) ListTasks(c *gin.Context) {
	var query dto.SyncTaskListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.syncService.ListTasks(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// CreateTask godoc
// @Summary 触发同步任务
// @Tags 资源管理-同步
// @Param request body dto.CreateSyncTaskRequest true "同步参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/sync [post]
func (h *SyncHandler) CreateTask(c *gin.Context) {
	var req dto.CreateSyncTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.syncService.CreateTask(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GetTask godoc
// @Summary 查询同步任务详情
// @Tags 资源管理-同步
// @Param id path int true "任务 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/sync/tasks/{id} [get]
func (h *SyncHandler) GetTask(c *gin.Context) {
	id, ok := syncID(c)
	if !ok {
		return
	}
	resp, err := h.syncService.FindTask(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ListLogs godoc
// @Summary 查询同步日志
// @Tags 资源管理-同步
// @Security BearerAuth
// @Param provider_id query int false "提供商 ID"
// @Param task_id query int false "任务 ID"
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/sync/logs [get]
func (h *SyncHandler) ListLogs(c *gin.Context) {
	var query dto.SyncLogListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.syncService.ListLogs(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ListInstances godoc
// @Summary 查询实例列表
// @Tags 资源管理-同步
// @Security BearerAuth
// @Param provider_id query int false "提供商 ID"
// @Param user_id query int false "用户 ID"
// @Param status query string false "状态"
// @Param keyword query string false "关键字"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/instances [get]
func (h *SyncHandler) ListInstances(c *gin.Context) {
	var query dto.InstanceListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.syncService.ListInstances(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GetInstance godoc
// @Summary 查询实例详情
// @Tags 资源管理-同步
// @Param id path int true "实例 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/resource/instances/{id} [get]
func (h *SyncHandler) GetInstance(c *gin.Context) {
	id, ok := syncID(c)
	if !ok {
		return
	}
	resp, err := h.syncService.FindInstance(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

func syncID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return 0, false
	}
	return id, true
}
