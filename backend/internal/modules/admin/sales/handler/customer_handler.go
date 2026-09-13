package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/sales/dto"
	"hostsent/backend/internal/modules/admin/sales/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// CustomerHandler 客户归属管理端处理器。
type CustomerHandler struct {
	customer service.CustomerService
}

// NewCustomerHandler 创建客户归属处理器。
func NewCustomerHandler(customer service.CustomerService) *CustomerHandler {
	return &CustomerHandler{customer: customer}
}

// List 客户归属列表（数据范围由服务端推导）。
// @Summary 客户归属列表
// @Tags 销售-客户归属
// @Security BearerAuth
// @Param keyword query string false "用户名/邮箱/手机"
// @Param admin_id query int false "指定销售（主管/超管）"
// @Param department_id query int false "指定部门（主管/超管）"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/sales/customers [get]
func (h *CustomerHandler) List(c *gin.Context) {
	var query dto.CustomerQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.customer.List(c.Request.Context(), resolveScope(c), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Unassigned 未归属客户池。
// @Summary 未归属客户池
// @Tags 销售-客户归属
// @Security BearerAuth
// @Router /api/v1/admin/sales/customers/unassigned [get]
func (h *CustomerHandler) Unassigned(c *gin.Context) {
	var query dto.CustomerQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.customer.Unassigned(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Assign 分配/变更客户归属。
// @Summary 分配客户归属
// @Tags 销售-客户归属
// @Security BearerAuth
// @Router /api/v1/admin/sales/customers/assign [post]
func (h *CustomerHandler) Assign(c *gin.Context) {
	var req dto.AssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	if err := h.customer.Assign(c.Request.Context(), resolveScope(c), operatorID, operatorName, req); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// Release 释放客户归属。
// @Summary 释放客户归属
// @Tags 销售-客户归属
// @Security BearerAuth
// @Router /api/v1/admin/sales/customers/release [post]
func (h *CustomerHandler) Release(c *gin.Context) {
	var req dto.ReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	if err := h.customer.Release(c.Request.Context(), resolveScope(c), operatorID, operatorName, req); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// Relations 客户归属变更历史。
// @Summary 客户归属变更历史
// @Tags 销售-客户归属
// @Security BearerAuth
// @Param userID path int true "客户用户 ID"
// @Router /api/v1/admin/sales/customers/{userID}/relations [get]
func (h *CustomerHandler) Relations(c *gin.Context) {
	userID, ok := pathID(c, "userID")
	if !ok {
		return
	}
	resp, err := h.customer.Relations(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Candidates 可分配销售下拉。
// @Summary 可分配销售下拉
// @Tags 销售-客户归属
// @Security BearerAuth
// @Param department_id query int false "按部门过滤"
// @Router /api/v1/admin/sales/sales-candidates [get]
func (h *CustomerHandler) Candidates(c *gin.Context) {
	deptID, _ := strconvParseUint(c.Query("department_id"))
	resp, err := h.customer.Candidates(c.Request.Context(), deptID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

func strconvParseUint(v string) (uint64, error) {
	if v == "" {
		return 0, nil
	}
	return strconv.ParseUint(v, 10, 64)
}
