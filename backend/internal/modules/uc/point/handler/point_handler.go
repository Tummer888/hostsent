// Package handler 提供用户中心积分模块的 HTTP 接口。
//
// 用户自助只能访问自身积分数据，user_id 一律取自鉴权上下文，不能由前端指定。
// 本处理器不直接依赖 admin 的积分 service 层：能力由装配层以最小接口注入，
// 保持 uc 与 admin 服务层解耦（与 uc/finance 同一策略）。
package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	pointdto "hostsent/backend/internal/modules/admin/point/dto"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// pointPort 装配层注入的最小积分能力接口。
type pointPort interface {
	MyOverview(ctx context.Context, userID uint64) (*pointdto.MyOverview, error)
	MyTransactions(ctx context.Context, userID uint64, q pointdto.TransactionListQuery) (*pointdto.TransactionListResponse, error)
}

// PointHandler 用户中心积分处理器。
type PointHandler struct {
	points pointPort
	mapErr func(error) *apperrors.AppError
}

// NewPointHandler 创建用户中心积分处理器。
func NewPointHandler(points pointPort, mapErr func(error) *apperrors.AppError) *PointHandler {
	return &PointHandler{points: points, mapErr: mapErr}
}

// currentUserID 返回数据归属账号 ID（积分归主账号，子账号只读）。
func currentUserID(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, false
	}
	return userID, true
}

// Overview 我的积分概览（余额 / 规则说明 / 最近流水）。
// @Summary 我的积分
// @Tags 用户中心-积分
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/points [get]
func (h *PointHandler) Overview(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	resp, err := h.points.MyOverview(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// Transactions 我的积分流水。
// @Summary 我的积分流水
// @Tags 用户中心-积分
// @Security BearerAuth
// @Param type query string false "流水类型"
// @Param direction query int false "方向：1获得 -1消耗"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/points/transactions [get]
func (h *PointHandler) Transactions(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var query pointdto.TransactionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.points.MyTransactions(c.Request.Context(), userID, query)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}
