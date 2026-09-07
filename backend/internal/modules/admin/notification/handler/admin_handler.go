// Package handler 提供通知与消息中心模块的 HTTP 接口。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifyservice "hostsent/backend/internal/modules/admin/notification/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// pathID 解析路径参数指定的数字 ID。
func pathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return 0, false
	}
	return id, true
}

func adminIDFromContext(c *gin.Context) (uint64, bool) {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID, true
	}
	return 0, false
}

func writeNotifyError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, notifyservice.ErrAnnouncementNotFound),
		errors.Is(err, notifyservice.ErrNotificationNotFound),
		errors.Is(err, notifyservice.ErrTemplateNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, notifyservice.ErrTemplateExists):
		return apperrors.New(20004, err.Error())
	case errors.Is(err, notifyservice.ErrInvalidParams):
		return apperrors.New(20001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}

// AdminHandler 管理端通知与消息中心 HTTP 处理器。
type AdminHandler struct {
	notifySvc     notifyservice.NotificationService
	announceSvc   notifyservice.AnnouncementService
	templateSvc   notifyservice.TemplateService
	preferenceSvc notifyservice.PreferenceService
}

func NewAdminHandler(
	notifySvc notifyservice.NotificationService,
	announceSvc notifyservice.AnnouncementService,
	templateSvc notifyservice.TemplateService,
	preferenceSvc notifyservice.PreferenceService,
) *AdminHandler {
	return &AdminHandler{
		notifySvc:     notifySvc,
		announceSvc:   announceSvc,
		templateSvc:   templateSvc,
		preferenceSvc: preferenceSvc,
	}
}

// —— 公告管理 ——

func (h *AdminHandler) ListAnnouncements(c *gin.Context) {
	var query notifydto.AnnouncementListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.announceSvc.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

func (h *AdminHandler) CreateAnnouncement(c *gin.Context) {
	var req notifydto.AnnouncementCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	adminID, _ := adminIDFromContext(c)
	resp, err := h.announceSvc.Create(c.Request.Context(), &req, adminID)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

func (h *AdminHandler) UpdateAnnouncement(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req notifydto.AnnouncementUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.announceSvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

func (h *AdminHandler) PublishAnnouncement(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.announceSvc.Publish(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

func (h *AdminHandler) OfflineAnnouncement(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.announceSvc.Offline(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

func (h *AdminHandler) DeleteAnnouncement(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.announceSvc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.SuccessMessage(c, "deleted")
}

// —— 通知记录 ——

func (h *AdminHandler) ListRecords(c *gin.Context) {
	var query notifydto.NotificationListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.notifySvc.ListRecords(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

func (h *AdminHandler) Resend(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.notifySvc.Resend(c.Request.Context(), id); err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.SuccessMessage(c, "resent")
}

func (h *AdminHandler) AdminUnreadCount(c *gin.Context) {
	adminID, _ := adminIDFromContext(c)
	count, err := h.notifySvc.UnreadCount(c.Request.Context(), adminID, "admin")
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, notifydto.UnreadCountResponse{Count: count})
}

func (h *AdminHandler) SendMailTest(c *gin.Context) {
	var req notifydto.MailTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.notifySvc.SendMailTest(c.Request.Context(), req.To); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "test mail sent")
}

// —— 通知模板 ——

func (h *AdminHandler) ListTemplates(c *gin.Context) {
	items, err := h.templateSvc.List(c.Request.Context())
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, gin.H{"list": items})
}

func (h *AdminHandler) UpdateTemplate(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req notifydto.TemplateUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.templateSvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}
