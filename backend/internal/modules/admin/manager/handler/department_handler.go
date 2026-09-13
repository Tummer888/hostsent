package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/service"
)

// DepartmentHandler 组织部门管理（S1 员工体系，doc86 §2.1）。
type DepartmentHandler struct {
	svc service.DepartmentService
}

// NewDepartmentHandler 创建部门处理器。
func NewDepartmentHandler(svc service.DepartmentService) *DepartmentHandler {
	return &DepartmentHandler{svc: svc}
}

func (h *DepartmentHandler) List(c *gin.Context) {
	var query dto.DepartmentListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	resp, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

func (h *DepartmentHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": "invalid id", "timestamp": time.Now().Unix()})
		return
	}
	resp, err := h.svc.FindByID(c.Request.Context(), id)
	if err != nil {
		status, code := mapDepartmentErr(err)
		c.JSON(status, gin.H{"code": code, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

func (h *DepartmentHandler) Create(c *gin.Context) {
	var req dto.DepartmentSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		status, code := mapDepartmentErr(err)
		c.JSON(status, gin.H{"code": code, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

func (h *DepartmentHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": "invalid id", "timestamp": time.Now().Unix()})
		return
	}
	var req dto.DepartmentSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		status, code := mapDepartmentErr(err)
		c.JSON(status, gin.H{"code": code, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

func (h *DepartmentHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": "invalid id", "timestamp": time.Now().Unix()})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		status, code := mapDepartmentErr(err)
		c.JSON(status, gin.H{"code": code, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "timestamp": time.Now().Unix()})
}

// mapDepartmentErr 区分「资源不存在/占用冲突」与「参数错误」，避免全部 500。
func mapDepartmentErr(err error) (int, int) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound, 40401
	case errors.Is(err, service.ErrDepartmentInUse), errors.Is(err, service.ErrDepartmentHasChildren):
		return http.StatusConflict, 40901
	default:
		return http.StatusBadRequest, 40001
	}
}
