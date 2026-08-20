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

func (h *SettlementHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

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
