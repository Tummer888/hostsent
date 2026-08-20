package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/security/dto"
	"hostsent/backend/internal/modules/security/service"
)

type SecurityHandler struct {
	service service.SecurityService
}

func NewSecurityHandler(service service.SecurityService) *SecurityHandler {
	return &SecurityHandler{service: service}
}

// ListLoginLogs godoc
// @Summary 登录日志列表
// @Description 获取登录日志分页列表，支持用户名、用户ID、结果、类型、IP、风险标记筛选
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param result query string false "登录结果"
// @Param login_type query string false "登录类型"
// @Param ip query string false "IP地址"
// @Param risk_flag query string false "风险标记"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.LoginLogInfo]]
// @Router /api/v1/security/login-logs [get]
func (h *SecurityHandler) ListLoginLogs(c *gin.Context) {
	var query dto.LoginLogListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.ListLoginLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// GetLoginLog godoc
// @Summary 登录日志详情
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param id path int true "日志ID"
// @Success 200 {object} dto.APIResponse[dto.LoginLogInfo]
// @Router /api/v1/security/login-logs/{id} [get]
func (h *SecurityHandler) GetLoginLog(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.GetLoginLog(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// ExportLoginLogs godoc
// @Summary 导出登录日志
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/security/login-logs/export [post]
func (h *SecurityHandler) ExportLoginLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "security-login-logs-export-job", "timestamp": time.Now().Unix()})
}

// ListAuditLogs godoc
// @Summary 审计日志列表
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param operator query string false "操作人"
// @Param module query string false "模块"
// @Param action query string false "动作"
// @Param result query string false "结果 success/failed"
// @Param resource_type query string false "资源类型"
// @Param resource_id query string false "资源ID"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.AuditLogInfo]]
// @Router /api/v1/security/audit-logs [get]
func (h *SecurityHandler) ListAuditLogs(c *gin.Context) {
	var query dto.AuditLogListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.ListAuditLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// GetAuditLog godoc
// @Summary 审计日志详情
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param id path int true "日志ID"
// @Success 200 {object} dto.APIResponse[dto.AuditLogInfo]
// @Router /api/v1/security/audit-logs/{id} [get]
func (h *SecurityHandler) GetAuditLog(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.GetAuditLog(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// ExportAuditLogs godoc
// @Summary 导出审计日志
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/security/audit-logs/export [post]
func (h *SecurityHandler) ExportAuditLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "security-audit-logs-export-job", "timestamp": time.Now().Unix()})
}

// ListRiskEvents godoc
// @Summary 风险事件列表
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param risk_type query string false "风险类型"
// @Param risk_level query string false "风险等级"
// @Param status query string false "处置状态"
// @Param keyword query string false "关键词"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.RiskEventInfo]]
// @Router /api/v1/security/risk-events [get]
func (h *SecurityHandler) ListRiskEvents(c *gin.Context) {
	var query dto.RiskEventListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.ListRiskEvents(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// GetRiskEvent godoc
// @Summary 风险事件详情
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param id path int true "风险事件ID"
// @Success 200 {object} dto.APIResponse[dto.RiskEventInfo]
// @Router /api/v1/security/risk-events/{id} [get]
func (h *SecurityHandler) GetRiskEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.GetRiskEvent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// IgnoreRiskEvent godoc
// @Summary 忽略风险事件
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "风险事件ID"
// @Param request body dto.RiskEventHandleRequest false "处理备注"
// @Success 200 {object} dto.APIResponse[dto.RiskEventInfo]
// @Router /api/v1/security/risk-events/{id}/ignore [post]
func (h *SecurityHandler) IgnoreRiskEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.RiskEventHandleRequest
	_ = c.ShouldBindJSON(&req)
	data, err := h.service.IgnoreRiskEvent(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// HandleRiskEvent godoc
// @Summary 处置风险事件
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "风险事件ID"
// @Param request body dto.RiskEventHandleRequest false "处理备注"
// @Success 200 {object} dto.APIResponse[dto.RiskEventInfo]
// @Router /api/v1/security/risk-events/{id}/handle [post]
func (h *SecurityHandler) HandleRiskEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.RiskEventHandleRequest
	_ = c.ShouldBindJSON(&req)
	data, err := h.service.HandleRiskEvent(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// CreateBlacklistFromRisk godoc
// @Summary 风险事件加入黑名单
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "风险事件ID"
// @Param request body dto.RiskEventHandleRequest false "处理备注"
// @Success 200 {object} dto.APIResponse[dto.BlacklistInfo]
// @Router /api/v1/security/risk-events/{id}/blacklist [post]
func (h *SecurityHandler) CreateBlacklistFromRisk(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.RiskEventHandleRequest
	_ = c.ShouldBindJSON(&req)
	data, err := h.service.CreateBlacklistFromRisk(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// RevokeSessionsFromRisk godoc
// @Summary 风险事件失效会话
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "风险事件ID"
// @Param request body dto.RiskEventHandleRequest false "处理备注"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.SessionInfo]]
// @Router /api/v1/security/risk-events/{id}/revoke-sessions [post]
func (h *SecurityHandler) RevokeSessionsFromRisk(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.RiskEventHandleRequest
	_ = c.ShouldBindJSON(&req)
	data, err := h.service.RevokeSessionsFromRisk(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// ListBlacklists godoc
// @Summary 黑名单列表
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param type query string false "黑名单类型"
// @Param status query string false "状态"
// @Param source query string false "来源"
// @Param keyword query string false "命中值关键词"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.BlacklistInfo]]
// @Router /api/v1/security/blacklists [get]
func (h *SecurityHandler) ListBlacklists(c *gin.Context) {
	var query dto.BlacklistListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.ListBlacklists(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// CreateBlacklist godoc
// @Summary 创建黑名单
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.BlacklistCreateRequest true "黑名单参数"
// @Success 200 {object} dto.APIResponse[dto.BlacklistInfo]
// @Router /api/v1/security/blacklists [post]
func (h *SecurityHandler) CreateBlacklist(c *gin.Context) {
	var req dto.BlacklistCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.CreateBlacklist(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// GetBlacklist godoc
// @Summary 黑名单详情
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param id path int true "黑名单ID"
// @Success 200 {object} dto.APIResponse[dto.BlacklistInfo]
// @Router /api/v1/security/blacklists/{id} [get]
func (h *SecurityHandler) GetBlacklist(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.GetBlacklist(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// UpdateBlacklist godoc
// @Summary 更新黑名单
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "黑名单ID"
// @Param request body dto.BlacklistUpdateRequest true "黑名单更新参数"
// @Success 200 {object} dto.APIResponse[dto.BlacklistInfo]
// @Router /api/v1/security/blacklists/{id} [put]
func (h *SecurityHandler) UpdateBlacklist(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.BlacklistUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.UpdateBlacklist(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// UpdateBlacklistStatus godoc
// @Summary 更新黑名单状态
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "黑名单ID"
// @Param request body dto.BlacklistStatusRequest true "状态参数"
// @Success 200 {object} dto.APIResponse[dto.BlacklistInfo]
// @Router /api/v1/security/blacklists/{id}/status [patch]
func (h *SecurityHandler) UpdateBlacklistStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.BlacklistStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.UpdateBlacklistStatus(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// ReleaseBlacklist godoc
// @Summary 解除黑名单
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "黑名单ID"
// @Success 200 {object} dto.APIResponse[dto.BlacklistInfo]
// @Router /api/v1/security/blacklists/{id}/release [post]
func (h *SecurityHandler) ReleaseBlacklist(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.ReleaseBlacklist(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// ListBlacklistHits godoc
// @Summary 黑名单命中记录
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param id path int true "黑名单ID"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.LoginLogInfo]]
// @Router /api/v1/security/blacklists/{id}/hits [get]
func (h *SecurityHandler) ListBlacklistHits(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.ListBlacklistHits(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// ListSessions godoc
// @Summary 会话列表
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param username query string false "用户名"
// @Param status query string false "会话状态"
// @Param platform query string false "平台类型"
// @Param ip query string false "IP地址"
// @Param risk_flag query string false "风险标记"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.SessionInfo]]
// @Router /api/v1/security/sessions [get]
func (h *SecurityHandler) ListSessions(c *gin.Context) {
	var query dto.SessionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.ListSessions(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// GetSession godoc
// @Summary 会话详情
// @Tags 安全与风控
// @Produce json
// @Security BearerAuth
// @Param id path int true "会话ID"
// @Success 200 {object} dto.APIResponse[dto.SessionInfo]
// @Router /api/v1/security/sessions/{id} [get]
func (h *SecurityHandler) GetSession(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.GetSession(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// RevokeSession godoc
// @Summary 强制下线会话
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "会话ID"
// @Param request body dto.SessionRevokeRequest false "失效原因"
// @Success 200 {object} dto.APIResponse[dto.SessionInfo]
// @Router /api/v1/security/sessions/{id}/revoke [post]
func (h *SecurityHandler) RevokeSession(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.SessionRevokeRequest
	_ = c.ShouldBindJSON(&req)
	data, err := h.service.RevokeSession(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// BatchRevokeSessions godoc
// @Summary 批量失效会话
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.SessionBatchRevokeRequest true "批量失效参数"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.SessionInfo]]
// @Router /api/v1/security/sessions/batch-revoke [post]
func (h *SecurityHandler) BatchRevokeSessions(c *gin.Context) {
	var req dto.SessionBatchRevokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.BatchRevokeSessions(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// RevokeUserAllSessions godoc
// @Summary 失效用户全部会话
// @Tags 安全与风控
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.SessionRevokeUserAllRequest true "用户会话失效参数"
// @Success 200 {object} dto.APIResponse[dto.ListResponse[dto.SessionInfo]]
// @Router /api/v1/security/sessions/revoke-user-all [post]
func (h *SecurityHandler) RevokeUserAllSessions(c *gin.Context) {
	var req dto.SessionRevokeUserAllRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	data, err := h.service.RevokeUserAllSessions(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}
