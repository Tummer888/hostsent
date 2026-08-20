package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/verification/dto"
	"hostsent/backend/internal/modules/verification/service"
)

// VerificationHandler 处理实名认证列表请求。
type VerificationHandler struct{ service service.VerificationService }

// NewVerificationHandler 创建实名认证处理器。
func NewVerificationHandler(service service.VerificationService) *VerificationHandler {
	return &VerificationHandler{service: service}
}

// ListPending godoc
// @Summary 实名待审核列表
// @Description 获取实名认证待审核列表
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param verification_type query string false "认证类型"
// @Param reviewer_name query string false "审核人"
// @Param keyword query string false "关键词"
// @Param start_time query string false "开始时间(RFC3339)"
// @Param end_time query string false "结束时间(RFC3339)"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.VerificationInfo]]
// @Router /api/v1/verifications/pending [get]
func (h *VerificationHandler) ListPending(c *gin.Context) {
	var query dto.VerificationListQuery
	if err := c.ShouldBindQuery(&query); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.ListPending(c.Request.Context(), query)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// ListApproved godoc
// @Summary 实名审核通过列表
// @Description 获取实名认证审核通过列表
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param verification_type query string false "认证类型"
// @Param reviewer_name query string false "审核人"
// @Param keyword query string false "关键词"
// @Param start_time query string false "开始时间(RFC3339)"
// @Param end_time query string false "结束时间(RFC3339)"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.VerificationInfo]]
// @Router /api/v1/verifications/approved [get]
func (h *VerificationHandler) ListApproved(c *gin.Context) {
	var query dto.VerificationListQuery
	if err := c.ShouldBindQuery(&query); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.ListApproved(c.Request.Context(), query)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

// ListRejected godoc
// @Summary 实名审核拒绝列表
// @Description 获取实名认证审核拒绝列表
// @Tags 实名认证
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param verification_type query string false "认证类型"
// @Param reviewer_name query string false "审核人"
// @Param keyword query string false "关键词"
// @Param start_time query string false "开始时间(RFC3339)"
// @Param end_time query string false "结束时间(RFC3339)"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.VerificationInfo]]
// @Router /api/v1/verifications/rejected [get]
func (h *VerificationHandler) ListRejected(c *gin.Context) {
	var query dto.VerificationListQuery
	if err := c.ShouldBindQuery(&query); err != nil { badRequest(c, err.Error()); return }
	data, err := h.service.ListRejected(c.Request.Context(), query)
	if err != nil { serverError(c, err.Error()); return }
	success(c, data)
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": message, "timestamp": time.Now().Unix()})
}

func serverError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": message, "timestamp": time.Now().Unix()})
}

func success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}
