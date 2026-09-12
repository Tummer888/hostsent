// Package handler 提供积分体系的管理端 HTTP 接口。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/point/dto"
	"hostsent/backend/internal/modules/admin/point/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// PointHandler 积分管理端处理器。
type PointHandler struct {
	svc service.PointService
}

// NewPointHandler 创建积分处理器。
func NewPointHandler(svc service.PointService) *PointHandler {
	return &PointHandler{svc: svc}
}

// Overview 积分概览（积分中心首页）。
// @Summary 积分概览
// @Tags 积分中心-概览
// @Security BearerAuth
// @Router /api/v1/admin/points/overview [get]
func (h *PointHandler) Overview(c *gin.Context) {
	resp, err := h.svc.Overview(c.Request.Context())
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Rules 积分规则列表。
// @Summary 积分规则列表
// @Tags 积分中心-规则
// @Security BearerAuth
// @Router /api/v1/admin/points/rules [get]
func (h *PointHandler) Rules(c *gin.Context) {
	var q dto.RuleListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Rules(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// CreateRule 新建积分规则。
// @Summary 新建积分规则
// @Tags 积分中心-规则
// @Security BearerAuth
// @Router /api/v1/admin/points/rules [post]
func (h *PointHandler) CreateRule(c *gin.Context) {
	var req dto.RuleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.CreateRule(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// UpdateRule 更新积分规则。
// @Summary 更新积分规则
// @Tags 积分中心-规则
// @Security BearerAuth
// @Router /api/v1/admin/points/rules/{id} [put]
func (h *PointHandler) UpdateRule(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RuleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.UpdateRule(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Accounts 积分账户列表。
// @Summary 积分账户列表
// @Tags 积分中心-账户
// @Security BearerAuth
// @Router /api/v1/admin/points/accounts [get]
func (h *PointHandler) Accounts(c *gin.Context) {
	var q dto.AccountListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Accounts(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Account 单个用户积分账户（按用户 ID）。
// @Summary 用户积分账户
// @Tags 积分中心-账户
// @Security BearerAuth
// @Router /api/v1/admin/points/accounts/{user_id} [get]
func (h *PointHandler) Account(c *gin.Context) {
	userID, ok := pathID(c, "user_id")
	if !ok {
		return
	}
	resp, err := h.svc.AccountOf(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Adjust 人工调整积分。
// @Summary 人工调整积分
// @Tags 积分中心-账户
// @Security BearerAuth
// @Router /api/v1/admin/points/accounts/adjust [post]
func (h *PointHandler) Adjust(c *gin.Context) {
	var req dto.AdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	info, err := h.svc.Adjust(c.Request.Context(), req, operatorID(c))
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, info)
}

// Transactions 积分流水列表。
// @Summary 积分流水列表
// @Tags 积分中心-流水
// @Security BearerAuth
// @Router /api/v1/admin/points/transactions [get]
func (h *PointHandler) Transactions(c *gin.Context) {
	var q dto.TransactionListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Transactions(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// —— 工具 ——

func pathID(c *gin.Context, param string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid "+param))
		return 0, false
	}
	return id, true
}

func operatorID(c *gin.Context) uint64 {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID
	}
	return 0
}

func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrRuleNotFound), errors.Is(err, service.ErrAccountNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrRuleCodeExists):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, service.ErrInvalidRule), errors.Is(err, service.ErrInvalidPoints):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, service.ErrInsufficientPoints):
		return apperrors.New(30001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
