package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	"hostsent/backend/internal/modules/admin/finance/bill/service"
	"hostsent/backend/internal/pkg/response"
)

// ReconHandler 对账处理入口。
type ReconHandler struct {
	reconService service.ReconService
}

// NewReconHandler 创建对账处理入口。
func NewReconHandler(reconService service.ReconService) *ReconHandler {
	return &ReconHandler{reconService: reconService}
}

// Reconcile godoc
// @Summary 触发对账
// @Tags 财务管理-对账
// @Security BearerAuth
// @Param request body dto.ReconcileRequest false "账期（空为全量）"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/bills/recon [post]
func (h *ReconHandler) Reconcile(c *gin.Context) {
	var req dto.ReconcileRequest
	_ = c.ShouldBindJSON(&req)
	resp, err := h.reconService.Reconcile(c.Request.Context(), req.Period)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
