package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/service"
)

type SettlementHandler struct {
	service service.SettlementService
}

func NewSettlementHandler(service service.SettlementService) *SettlementHandler {
	return &SettlementHandler{service: service}
}

// List godoc
// @Summary 佣金结算单列表
// @Description 获取分销代理结算单列表
// @Tags 结算管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "状态(pending/confirmed/paid/cancelled)"
// @Param agent_id query int false "代理ID"
// @Param keyword query string false "关键词(结算单号/备注)"
// @Success 200 {object} dto.APIResponse[dto.SettlementListResponse]
// @Router /api/v1/distribution/settlements [get]
func (h *SettlementHandler) List(c *gin.Context) {
	var query dto.SettlementListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	items, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": items, "timestamp": time.Now().Unix()})
}

// Get godoc
// @Summary 结算单详情
// @Description 获取单个结算单详情
// @Tags 结算管理
// @Produce json
// @Param id path int true "结算单ID"
// @Success 200 {object} dto.APIResponse[dto.SettlementInfo]
// @Router /api/v1/distribution/settlements/{id} [get]
func (h *SettlementHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Create godoc
// @Summary 手动创建结算单
// @Description 手动录入一笔结算记录
// @Tags 结算管理
// @Accept json
// @Produce json
// @Param request body dto.SettlementCreateRequest true "结算参数"
// @Success 200 {object} dto.APIResponse[dto.SettlementInfo]
// @Router /api/v1/distribution/settlements [post]
func (h *SettlementHandler) Create(c *gin.Context) {
	var req dto.SettlementCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Update godoc
// @Summary 更新结算单
// @Description 更新结算单信息
// @Tags 结算管理
// @Accept json
// @Produce json
// @Param id path int true "结算单ID"
// @Param request body dto.SettlementUpdateRequest true "结算参数"
// @Success 200 {object} dto.APIResponse[dto.SettlementInfo]
// @Router /api/v1/distribution/settlements/{id} [put]
func (h *SettlementHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.SettlementUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Confirm godoc
// @Summary 确认结算单
// @Description 将结算单状态设为已确认，锁定金额
// @Tags 结算管理
// @Accept json
// @Produce json
// @Param id path int true "结算单ID"
// @Param request body dto.SettlementStatusChangeRequest true "操作原因"
// @Success 200 {object} dto.APIResponse[dto.SettlementInfo]
// @Router /api/v1/distribution/settlements/{id}/confirm [post]
func (h *SettlementHandler) Confirm(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.SettlementStatusChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Confirm(c.Request.Context(), id, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Pay godoc
// @Summary 支付结算单
// @Description 完成结算单支付，释放佣金到可用余额
// @Tags 结算管理
// @Accept json
// @Produce json
// @Param id path int true "结算单ID"
// @Param request body dto.SettlementStatusChangeRequest true "操作原因"
// @Success 200 {object} dto.APIResponse[dto.SettlementInfo]
// @Router /api/v1/distribution/settlements/{id}/pay [post]
func (h *SettlementHandler) Pay(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.SettlementStatusChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Pay(c.Request.Context(), id, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Cancel godoc
// @Summary 取消结算单
// @Description 作废该笔结算单
// @Tags 结算管理
// @Accept json
// @Produce json
// @Param id path int true "结算单ID"
// @Param request body dto.SettlementStatusChangeRequest true "取消原因"
// @Success 200 {object} dto.APIResponse[dto.SettlementInfo]
// @Router /api/v1/distribution/settlements/{id}/cancel [post]
func (h *SettlementHandler) Cancel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.SettlementStatusChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Cancel(c.Request.Context(), id, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Delete godoc
// @Summary 删除结算单
// @Description 删除结算记录
// @Tags 结算管理
// @Produce json
// @Param id path int true "结算单ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/distribution/settlements/{id} [delete]
func (h *SettlementHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

func (h *SettlementHandler) handleError(c *gin.Context, err error) {
	statusCode := http.StatusInternalServerError
	code := 50001
	if errors.Is(err, gorm.ErrRecordNotFound) {
		statusCode = http.StatusNotFound
		code = 40404
	} else if errors.Is(err, service.ErrInvalidSettlementStatus) || errors.Is(err, service.ErrSettlementStatusChange) || strings.Contains(err.Error(), "settlement status") || strings.Contains(err.Error(), "period_end") {
		statusCode = http.StatusBadRequest
		code = 20001
	}
	c.JSON(statusCode, gin.H{"code": code, "message": err.Error(), "timestamp": time.Now().Unix()})
}
