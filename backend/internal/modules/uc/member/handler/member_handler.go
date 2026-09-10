// Package handler 提供用户中心成员（子账号）模块 HTTP 接口。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/member/dto"
	"hostsent/backend/internal/modules/uc/member/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// MemberHandler 成员管理 HTTP 处理器。
type MemberHandler struct {
	memberService service.MemberService
}

// NewMemberHandler 创建成员管理处理器。
func NewMemberHandler(memberService service.MemberService) *MemberHandler {
	return &MemberHandler{memberService: memberService}
}

// List godoc
// @Summary 我的成员列表（仅主账号）
// @Tags 用户中心-成员
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/members [get]
func (h *MemberHandler) List(c *gin.Context) {
	ownerID, isSub, ok := actor(c)
	if !ok {
		return
	}
	var query dto.MemberListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.memberService.List(c.Request.Context(), ownerID, isSub, query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 新建成员
// @Tags 用户中心-成员
// @Security BearerAuth
// @Param request body dto.CreateRequest true "成员参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/members [post]
func (h *MemberHandler) Create(c *gin.Context) {
	ownerID, isSub, ok := actor(c)
	if !ok {
		return
	}
	var req dto.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.memberService.Create(c.Request.Context(), ownerID, isSub, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 修改成员备注/状态
// @Tags 用户中心-成员
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/members/{id} [put]
func (h *MemberHandler) Update(c *gin.Context) {
	ownerID, isSub, ok := actor(c)
	if !ok {
		return
	}
	id, valid := pathID(c, "id")
	if !valid {
		return
	}
	var req dto.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.memberService.Update(c.Request.Context(), ownerID, isSub, id, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// SetPermissions godoc
// @Summary 覆盖式设置成员权限
// @Tags 用户中心-成员
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/members/{id}/permissions [put]
func (h *MemberHandler) SetPermissions(c *gin.Context) {
	ownerID, isSub, ok := actor(c)
	if !ok {
		return
	}
	id, valid := pathID(c, "id")
	if !valid {
		return
	}
	var req dto.SetPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.memberService.SetPermissions(c.Request.Context(), ownerID, isSub, id, req.Permissions)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除（禁用）成员
// @Tags 用户中心-成员
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/members/{id} [delete]
func (h *MemberHandler) Delete(c *gin.Context) {
	ownerID, isSub, ok := actor(c)
	if !ok {
		return
	}
	id, valid := pathID(c, "id")
	if !valid {
		return
	}
	if err := h.memberService.Delete(c.Request.Context(), ownerID, isSub, id); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, "已删除")
}

// ListLogs godoc
// @Summary 成员操作日志
// @Tags 用户中心-成员
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/members/{id}/logs [get]
func (h *MemberHandler) ListLogs(c *gin.Context) {
	ownerID, isSub, ok := actor(c)
	if !ok {
		return
	}
	id, valid := pathID(c, "id")
	if !valid {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := h.memberService.ListLogs(c.Request.Context(), ownerID, isSub, id, page, pageSize)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// actor 取归属账号 ID 与是否子账号（P4-04）。
func actor(c *gin.Context) (uint64, bool, bool) {
	ownerID := middleware.EffectiveUserID(c)
	if ownerID == 0 {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return 0, false, false
	}
	return ownerID, middleware.IsSubAccount(c), true
}

func pathID(c *gin.Context, param string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid "+param))
		return 0, false
	}
	return id, true
}

func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrNotAccountOwner):
		return apperrors.New(40301, err.Error())
	case errors.Is(err, service.ErrMemberQuotaExceeded):
		return apperrors.New(30005, err.Error())
	case errors.Is(err, service.ErrMemberNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrUsernameTaken),
		errors.Is(err, service.ErrEmailTaken),
		errors.Is(err, service.ErrPhoneTaken),
		errors.Is(err, service.ErrInvalidStatus):
		return apperrors.New(30005, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
