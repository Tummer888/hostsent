package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// CallbackHandler 回调日志入口。
type CallbackHandler struct {
	svc service.CallbackService
}

// NewCallbackHandler 创建回调日志处理入口。
func NewCallbackHandler(svc service.CallbackService) *CallbackHandler {
	return &CallbackHandler{svc: svc}
}

// List 回调日志列表。
// @Router /api/v1/admin/payment/callbacks [get]
func (h *CallbackHandler) List(c *gin.Context) {
	var q dto.CallbackListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
