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

type CommissionHandler struct {
	service service.CommissionService
}

func NewCommissionHandler(service service.CommissionService) *CommissionHandler {
	return &CommissionHandler{service: service}
}

// List godoc
// @Summary 佣金记录列表
// @Description 获取佣金结算记录列表
// @Tags 佣金管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "状态(pending/frozen/settled/cancelled)"
// @Param agent_id query int false "代理ID"
// @Param keyword query string false "关键词(订单号/备注)"
// @Success 200 {object} dto.APIResponse[dto.CommissionListResponse]
// @Router /api/v1/distribution/commissions [get]
func (h *CommissionHandler) List(c *gin.Context) {
	var query dto.CommissionListQuery
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
// @Summary 佣金记录详情
// @Description 获取单个佣金记录详情
// @Tags 佣金管理
// @Produce json
// @Param id path int true "记录ID"
// @Success 200 {object} dto.APIResponse[dto.CommissionInfo]
// @Router /api/v1/distribution/commissions/{id} [get]
func (h *CommissionHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Create godoc
// @Summary 手动创建佣金记录
// @Description 手动录入一笔佣金记录
// @Tags 佣金管理
// @Accept json
// @Produce json
// @Param request body dto.CommissionCreateRequest true "佣金参数"
// @Success 200 {object} dto.APIResponse[dto.CommissionInfo]
// @Router /api/v1/distribution/commissions [post]
func (h *CommissionHandler) Create(c *gin.Context) {
	var req dto.CommissionCreateRequest
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
// @Summary 更新佣金记录
// @Description 更新佣金记录信息
// @Tags 佣金管理
// @Accept json
// @Produce json
// @Param id path int true "记录ID"
// @Param request body dto.CommissionUpdateRequest true "佣金参数"
// @Success 200 {object} dto.APIResponse[dto.CommissionInfo]
// @Router /api/v1/distribution/commissions/{id} [put]
func (h *CommissionHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.CommissionUpdateRequest
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

// Freeze godoc
// @Summary 冻结佣金
// @Description 将佣金状态设为冻结
// @Tags 佣金管理
// @Accept json
// @Produce json
// @Param id path int true "记录ID"
// @Param request body dto.CommissionStatusChangeRequest true "操作原因"
// @Success 200 {object} dto.APIResponse[dto.CommissionInfo]
// @Router /api/v1/distribution/commissions/{id}/freeze [post]
func (h *CommissionHandler) Freeze(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.CommissionStatusChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Freeze(c.Request.Context(), id, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Unfreeze godoc
// @Summary 解冻佣金
// @Description 将佣金从冻结状态恢复
// @Tags 佣金管理
// @Accept json
// @Produce json
// @Param id path int true "记录ID"
// @Param request body dto.CommissionStatusChangeRequest true "操作原因"
// @Success 200 {object} dto.APIResponse[dto.CommissionInfo]
// @Router /api/v1/distribution/commissions/{id}/unfreeze [post]
func (h *CommissionHandler) Unfreeze(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.CommissionStatusChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	item, err := h.service.Unfreeze(c.Request.Context(), id, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

// Cancel godoc
// @Summary 取消佣金
// @Description 作废该笔佣金记录
// @Tags 佣金管理
// @Accept json
// @Produce json
// @Param id path int true "记录ID"
// @Param request body dto.CommissionStatusChangeRequest true "取消原因"
// @Success 200 {object} dto.APIResponse[dto.CommissionInfo]
// @Router /api/v1/distribution/commissions/{id}/cancel [post]
func (h *CommissionHandler) Cancel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.CommissionStatusChangeRequest
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
// @Summary 删除佣金记录
// @Description 删除佣金记录
// @Tags 佣金管理
// @Produce json
// @Param id path int true "记录ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/distribution/commissions/{id} [delete]
func (h *CommissionHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

func (h *CommissionHandler) handleError(c *gin.Context, err error) {
	statusCode := http.StatusInternalServerError
	code := 50001
	if errors.Is(err, gorm.ErrRecordNotFound) {
		statusCode = http.StatusNotFound
		code = 40404
	} else if errors.Is(err, service.ErrInvalidCommissionStatus) || errors.Is(err, service.ErrCommissionStatusUnchanged) || strings.Contains(err.Error(), "commission status cannot change") {
		statusCode = http.StatusBadRequest
		code = 20001
	}
	c.JSON(statusCode, gin.H{"code": code, "message": err.Error(), "timestamp": time.Now().Unix()})
}
