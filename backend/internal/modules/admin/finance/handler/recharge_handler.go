package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/dto"
	"hostsent/backend/internal/modules/admin/finance/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// RechargeHandler 充值处理入口。
type RechargeHandler struct {
	rechargeService service.RechargeService
}

// NewRechargeHandler 创建充值处理入口。
func NewRechargeHandler(rechargeService service.RechargeService) *RechargeHandler {
	return &RechargeHandler{rechargeService: rechargeService}
}

// Create godoc
// @Summary 线下充值登记
// @Tags 财务管理-充值
// @Security BearerAuth
// @Param request body dto.RechargeCreateRequest true "充值登记参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/recharges [post]
func (h *RechargeHandler) Create(c *gin.Context) {
	var req dto.RechargeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.rechargeService.Create(c.Request.Context(), req, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Approve godoc
// @Summary 手工确认到账（幂等）
// @Tags 财务管理-充值
// @Security BearerAuth
// @Param id path int true "充值单 ID"
// @Param request body dto.RechargeApproveRequest true "确认参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/recharges/{id}/approve [post]
func (h *RechargeHandler) Approve(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RechargeApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.rechargeService.Approve(c.Request.Context(), id, req, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// List godoc
// @Summary 充值单列表
// @Tags 财务管理-充值
// @Security BearerAuth
// @Param user_id query int false "用户 ID"
// @Param status query string false "状态"
// @Param method query string false "方式"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/recharges [get]
func (h *RechargeHandler) List(c *gin.Context) {
	var query dto.RechargeListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.rechargeService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
