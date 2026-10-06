// Package handler 提供限时活动折扣的 HTTP 接口（doc108 §8J）。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/flashdiscount/dto"
	"hostsent/backend/internal/modules/admin/product/flashdiscount/repository"
	"hostsent/backend/internal/modules/admin/product/flashdiscount/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// Handler 活动折扣处理器。
type Handler struct {
	svc service.Service
}

// NewHandler 创建活动折扣处理器。
func NewHandler(svc service.Service) *Handler {
	return &Handler{svc: svc}
}

// List godoc
// @Summary 限时活动折扣列表
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param keyword query string false "关键字"
// @Param status query string false "启停意图 active/disabled"
// @Param running query string false "只看此刻进行中（true）"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/promotion/flash-discounts [get]
func (h *Handler) List(c *gin.Context) {
	var query dto.Query
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	data, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, data)
}

// Create godoc
// @Summary 创建限时活动折扣
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param request body dto.Request true "活动参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/promotion/flash-discounts [post]
func (h *Handler) Create(c *gin.Context) {
	var req dto.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	data, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, data)
}

// Get godoc
// @Summary 活动折扣详情
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param id path int true "活动ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/promotion/flash-discounts/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.svc.FindByID(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, data)
}

// Update godoc
// @Summary 更新活动折扣（items 整体覆盖语义）
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param id path int true "活动ID"
// @Param request body dto.Request true "活动参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/promotion/flash-discounts/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	data, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, data)
}

// Delete godoc
// @Summary 删除活动折扣
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param id path int true "活动ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/promotion/flash-discounts/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, "ok")
}

// writeError 把业务错误映射成 404/409（可自解状态），其余回落 500。
//
// 「折扣低于成本」「时间窗反了」「范围没选」都是运营能自己改对的状态冲突，
// 回落 500 只会让前端提示「服务器错误」，运营不知道下一步做什么。
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		response.Error(c, apperrors.New(40401, err.Error()))
	case errors.Is(err, service.ErrInvalidValue),
		errors.Is(err, service.ErrInvalidWindow),
		errors.Is(err, service.ErrInvalidItem),
		errors.Is(err, service.ErrBelowCost),
		errors.Is(err, service.ErrNoTarget),
		errors.Is(err, repository.ErrCodeTaken):
		response.Error(c, apperrors.New(40901, err.Error()))
	default:
		response.Error(c, apperrors.New(50001, err.Error()))
	}
}
