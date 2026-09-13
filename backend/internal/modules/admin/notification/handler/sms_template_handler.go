package handler

import (
	"github.com/gin-gonic/gin"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifyservice "hostsent/backend/internal/modules/admin/notification/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// SmsTemplateHandler 短信模板与变量注册表入口。
type SmsTemplateHandler struct {
	svc    notifyservice.SmsTemplateService
	varSvc notifyservice.TemplateVarService
}

// NewSmsTemplateHandler 创建短信模板处理入口。
func NewSmsTemplateHandler(svc notifyservice.SmsTemplateService, varSvc notifyservice.TemplateVarService) *SmsTemplateHandler {
	return &SmsTemplateHandler{svc: svc, varSvc: varSvc}
}

// List 短信模板列表。
// @Router /api/v1/admin/notification/sms-templates [get]
func (h *SmsTemplateHandler) List(c *gin.Context) {
	var q notifydto.SmsTemplateListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Get 短信模板详情。
// @Router /api/v1/admin/notification/sms-templates/{id} [get]
func (h *SmsTemplateHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Create 新建短信模板（未注册变量直接拒存并回传变量名）。
// @Router /api/v1/admin/notification/sms-templates [post]
func (h *SmsTemplateHandler) Create(c *gin.Context) {
	var req notifydto.SmsTemplateSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Update 更新短信模板。
// @Router /api/v1/admin/notification/sms-templates/{id} [put]
func (h *SmsTemplateHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req notifydto.SmsTemplateSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Delete 删除短信模板（被通知模板引用时拒绝）。
// @Router /api/v1/admin/notification/sms-templates/{id} [delete]
func (h *SmsTemplateHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.SuccessMessage(c, "deleted")
}

// Preview 用注册变量样例值渲染预览。
// @Router /api/v1/admin/notification/sms-templates/preview [post]
func (h *SmsTemplateHandler) Preview(c *gin.Context) {
	var req notifydto.SmsTemplatePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Preview(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// ListVars 变量注册表列表。
// @Router /api/v1/admin/notification/template-vars [get]
func (h *SmsTemplateHandler) ListVars(c *gin.Context) {
	items, err := h.varSvc.List(c.Request.Context(), c.Query("category"))
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, gin.H{"items": items})
}

// CreateVar 新增变量。
// @Router /api/v1/admin/notification/template-vars [post]
func (h *SmsTemplateHandler) CreateVar(c *gin.Context) {
	var req notifydto.TemplateVarSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.varSvc.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// UpdateVar 修改变量（label/sample/状态）。
// @Router /api/v1/admin/notification/template-vars/{id} [put]
func (h *SmsTemplateHandler) UpdateVar(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req notifydto.TemplateVarSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.varSvc.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// DeleteVar 逻辑停用变量（不物理删）。
// @Router /api/v1/admin/notification/template-vars/{id} [delete]
func (h *SmsTemplateHandler) DeleteVar(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.varSvc.Disable(c.Request.Context(), id); err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.SuccessMessage(c, "disabled")
}
