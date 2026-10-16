package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	notifydto "hostsent/backend/internal/modules/admin/system/notification/dto"
	notifyservice "hostsent/backend/internal/modules/admin/system/notification/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// UserHandler 用户端通知 HTTP 处理器。
type UserHandler struct {
	notifySvc     notifyservice.NotificationService
	announceSvc   notifyservice.AnnouncementService
	preferenceSvc notifyservice.PreferenceService
}

func NewUserHandler(
	notifySvc notifyservice.NotificationService,
	announceSvc notifyservice.AnnouncementService,
	preferenceSvc notifyservice.PreferenceService,
) *UserHandler {
	return &UserHandler{
		notifySvc:     notifySvc,
		announceSvc:   announceSvc,
		preferenceSvc: preferenceSvc,
	}
}

// userIDFromContext 返回数据归属账号 ID（P4-04）：子账号能看到主账号的站内信。
func userIDFromContext(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, false
	}
	return userID, true
}

// UserUnreadCount godoc
// @Summary 站内通知未读数
// @Tags 用户端-消息中心
// @Security BearerAuth
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/notifications/unread-count [get]
func (h *UserHandler) UserUnreadCount(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		response.Error(c, apperrors.New(10003, "user not authenticated"))
		return
	}
	count, err := h.notifySvc.UnreadCount(c.Request.Context(), userID, "user")
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, notifydto.UnreadCountResponse{Count: count})
}

// UserList godoc
// @Summary 站内通知列表
// @Tags 用户端-消息中心
// @Security BearerAuth
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/notifications [get]
func (h *UserHandler) UserList(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		response.Error(c, apperrors.New(10003, "user not authenticated"))
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := h.notifySvc.ListByUser(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// UserDetail godoc
// @Summary 站内通知详情
// @Tags 用户端-消息中心
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body notifydto.PreferenceUpdateRequest true "请求参数"
// @Success 200 {object} notifydto.NotificationInfo "成功"
// @Router /api/v1/uc/notifications/{id} [get]
func (h *UserHandler) UserDetail(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		response.Error(c, apperrors.New(10003, "user not authenticated"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return
	}
	resp, err := h.notifySvc.GetDetail(c.Request.Context(), id, userID)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// UserReadAll godoc
// @Summary 站内通知全部标记已读
// @Tags 用户端-消息中心
// @Security BearerAuth
// @Param request body notifydto.PreferenceUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/notifications/read-all [post]
func (h *UserHandler) UserReadAll(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		response.Error(c, apperrors.New(10003, "user not authenticated"))
		return
	}
	if err := h.notifySvc.ReadAll(c.Request.Context(), userID); err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.SuccessMessage(c, "all read")
}

// UserAnnouncements godoc
// @Summary 我的公告
// @Tags 用户端-消息中心
// @Security BearerAuth
// @Param request body notifydto.PreferenceUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/announcements [get]
func (h *UserHandler) UserAnnouncements(c *gin.Context) {
	items, err := h.announceSvc.ListPublished(c.Request.Context(), "user")
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, gin.H{"list": items})
}

// UserGetPrefs godoc
// @Summary 通知偏好
// @Tags 用户端-消息中心
// @Security BearerAuth
// @Param request body notifydto.PreferenceUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/notification-preferences [get]
func (h *UserHandler) UserGetPrefs(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		response.Error(c, apperrors.New(10003, "user not authenticated"))
		return
	}
	items, err := h.preferenceSvc.ListByUser(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, gin.H{"list": items})
}

// UserUpdatePrefs godoc
// @Summary 更新通知偏好
// @Tags 用户端-消息中心
// @Security BearerAuth
// @Param request body notifydto.PreferenceUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/notification-preferences [put]
func (h *UserHandler) UserUpdatePrefs(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		response.Error(c, apperrors.New(10003, "user not authenticated"))
		return
	}
	var req notifydto.PreferenceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	// bug ① 修复：兼容 {items:[...]} 与 {list:[...]} 两种请求体，
	// 空列表在归一化后校验（不能靠 binding tag，否则发 list 时直接 400）。
	items := req.Normalize()
	if len(items) == 0 {
		response.Error(c, apperrors.New(20001, "偏好列表不能为空"))
		return
	}
	if err := h.preferenceSvc.BatchUpsert(c.Request.Context(), userID, items); err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.SuccessMessage(c, "preferences updated")
}
