package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	"hostsent/backend/internal/modules/admin/finance/bill/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// BillHandler 账单处理入口。
type BillHandler struct {
	billService service.BillService
}

// NewBillHandler 创建账单处理入口。
func NewBillHandler(billService service.BillService) *BillHandler {
	return &BillHandler{billService: billService}
}

// List godoc
// @Summary 账单列表
// @Tags 财务管理-账单
// @Security BearerAuth
// @Param user_id query int false "用户 ID"
// @Param period query string false "账期"
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/bills [get]
func (h *BillHandler) List(c *gin.Context) {
	var query dto.BillListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.billService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Close godoc
// @Summary 关账
// @Tags 财务管理-账单
// @Security BearerAuth
// @Param id path int true "账单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/bills/{id}/close [post]
func (h *BillHandler) Close(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.billService.Close(c.Request.Context(), id); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}
