package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"

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

// List godoc
// @Summary 部门列表
// @Tags 管理端-员工组织
// @Security BearerAuth
// @Param request body dto.DepartmentSaveRequest true "请求参数"
// @Success 200 {object} dto.DepartmentListResponse "成功"
// @Router /api/v1/admin/departments [get]
func (h *DepartmentHandler) List(c *gin.Context) {
	var query dto.DepartmentListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 部门详情
// @Tags 管理端-员工组织
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.DepartmentSaveRequest true "请求参数"
// @Success 200 {object} dto.DepartmentInfo "成功"
// @Router /api/v1/admin/departments/{id} [get]
func (h *DepartmentHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	resp, err := h.svc.FindByID(c.Request.Context(), id)
	if err != nil {
		status, code := mapDepartmentErr(err)
		response.ErrorWithStatus(c, status, apperrors.New(code, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 新建部门
// @Tags 管理端-员工组织
// @Security BearerAuth
// @Param request body dto.DepartmentSaveRequest true "请求参数"
// @Success 200 {object} dto.DepartmentInfo "成功"
// @Router /api/v1/admin/departments [post]
func (h *DepartmentHandler) Create(c *gin.Context) {
	var req dto.DepartmentSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		status, code := mapDepartmentErr(err)
		response.ErrorWithStatus(c, status, apperrors.New(code, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新部门
// @Tags 管理端-员工组织
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.DepartmentSaveRequest true "请求参数"
// @Success 200 {object} dto.DepartmentInfo "成功"
// @Router /api/v1/admin/departments/{id} [put]
func (h *DepartmentHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	var req dto.DepartmentSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		status, code := mapDepartmentErr(err)
		response.ErrorWithStatus(c, status, apperrors.New(code, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除部门
// @Tags 管理端-员工组织
// @Security BearerAuth
// @Param id path int true "id"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/departments/{id} [delete]
func (h *DepartmentHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		status, code := mapDepartmentErr(err)
		response.ErrorWithStatus(c, status, apperrors.New(code, err.Error()))
		return
	}
	response.Success(c, nil)
}

// mapDepartmentErr 区分「资源不存在/占用冲突」与「参数错误」，避免全部 500。
func mapDepartmentErr(err error) (int, int) {
	switch {
	case errors.Is(err, service.ErrDepartmentNotFound):
		return http.StatusNotFound, 40401
	case errors.Is(err, service.ErrDepartmentInUse), errors.Is(err, service.ErrDepartmentHasChildren):
		return http.StatusConflict, 40901
	default:
		return http.StatusBadRequest, 40001
	}
}
