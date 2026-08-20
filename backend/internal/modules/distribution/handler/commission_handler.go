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

func (h *CommissionHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item, "timestamp": time.Now().Unix()})
}

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
