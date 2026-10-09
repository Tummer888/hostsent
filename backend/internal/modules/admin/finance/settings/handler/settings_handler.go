package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/settings/dto"
	"hostsent/backend/internal/modules/admin/finance/settings/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// SettingsHandler 财务参数处理入口。
type SettingsHandler struct {
	settingsService service.SettingsService
}

// NewSettingsHandler 创建财务参数处理入口。
func NewSettingsHandler(settingsService service.SettingsService) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsService}
}

// List godoc
// @Summary 财务参数列表（可写项 + 只读速查项）
// @Tags 财务管理-参数
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/settings [get]
func (h *SettingsHandler) List(c *gin.Context) {
	resp, err := h.settingsService.List(c.Request.Context())
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 保存财务参数（白名单键）
// @Tags 财务管理-参数
// @Security BearerAuth
// @Param request body dto.SettingsUpdateRequest true "参数键值"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/settings [put]
func (h *SettingsHandler) Update(c *gin.Context) {
	var req dto.SettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.settingsService.Update(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// writeError 将财务参数子域错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	if errors.Is(err, service.ErrInvalidSetting) {
		return apperrors.New(20001, err.Error())
	}
	return apperrors.New(50001, err.Error())
}
