package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/service"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/netutil"
	"hostsent/backend/internal/pkg/security"
)

type AdminHandler struct {
	adminService service.AdminService
}

func NewAdminHandler(adminService service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

// Login godoc
// @Summary 管理员登录
// @Description 使用管理员用户名和密码登录并获取 JWT
// @Tags 管理员认证
// @Accept json
// @Produce json
// @Param request body dto.AdminLoginRequest true "登录参数"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/auth/login [post]
func (h *AdminHandler) Login(c *gin.Context) {
	var req dto.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}

	resp, err := h.adminService.Login(c.Request.Context(), req, netutil.ClientIP(c),
		c.GetHeader("User-Agent"), c.GetHeader("X-Device-Fingerprint"))
	if err != nil {
		writeSecurityError(c, err)
		return
	}

	response.Success(c, resp)
}

// VerifyLoginOTP godoc
// @Summary 管理端登录二次验证
// @Description 用登录返回的 otp_token 换取正式 JWT（doc91 §5.2）
// @Tags 管理员认证
// @Accept json
// @Produce json
// @Param request body dto.AdminVerifyOTPRequest true "验证参数"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Router /api/v1/admin/auth/login/verify-otp [post]
func (h *AdminHandler) VerifyLoginOTP(c *gin.Context) {
	var req dto.AdminVerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}

	resp, err := h.adminService.VerifyLoginOTP(c.Request.Context(), req, netutil.ClientIP(c),
		c.GetHeader("User-Agent"), c.GetHeader("X-Device-Fingerprint"))
	if err != nil {
		writeSecurityError(c, err)
		return
	}

	response.Success(c, resp)
}

// writeSecurityError 把安全域错误映射为业务错误码（doc91 §3.4/§4.1）。
//
// 走 200 业务码而非 500：验证码错误、账号锁定等是正常业务反馈，落 500 会让
// 前端与监控把「用户输错验证码」当成服务故障。
func writeSecurityError(c *gin.Context, err error) {
	if code := security.CodeOf(err); code != 0 {
		response.Error(c, apperrors.New(code, err.Error()))
		return
	}
	response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
}

// Me godoc
// @Summary 当前管理员信息
// @Description 通过 JWT 获取当前登录管理员信息
// @Tags 管理员认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/auth/me [get]
func (h *AdminHandler) Me(c *gin.Context) {
	adminClaims, ok := middleware.GetAdminClaims(c)
	if !ok {
		// 兼容旧 token：回退到统一 Claims
		claims, ok := middleware.GetClaims(c)
		if !ok {
			response.ErrorWithStatus(c, http.StatusUnauthorized, apperrors.New(10001, "unauthorized"))
			return
		}
		resp, err := h.adminService.Me(c.Request.Context(), claims.UserID)
		if err != nil {
			response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
			return
		}
		response.Success(c, resp)
		return
	}

	resp, err := h.adminService.Me(c.Request.Context(), adminClaims.AdminID)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// List godoc
// @Summary 管理员账号列表
// @Tags 管理端-认证
// @Security BearerAuth
// @Param request body dto.AdminCreateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/admins [get]
func (h *AdminHandler) List(c *gin.Context) {
	var query dto.AdminListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.adminService.List(c.Request.Context(), query)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 新建管理员账号
// @Tags 管理端-认证
// @Security BearerAuth
// @Param request body dto.AdminCreateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/admins [post]
func (h *AdminHandler) Create(c *gin.Context) {
	var req dto.AdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.adminService.Create(c.Request.Context(), req)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 管理员账号详情
// @Tags 管理端-认证
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.AdminUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/admins/{id} [get]
func (h *AdminHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	resp, err := h.adminService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新管理员账号
// @Tags 管理端-认证
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.AdminUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/admins/{id} [put]
func (h *AdminHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	var req dto.AdminUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.adminService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdateStatus godoc
// @Summary 管理员账号更新状态
// @Tags 管理端-认证
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.AdminStatusRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/admins/{id}/status [patch]
func (h *AdminHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	var req dto.AdminStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.adminService.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, nil)
}

// ResetPassword godoc
// @Summary 重置管理员账号
// @Tags 管理端-认证
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.AdminResetPasswordRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/admins/{id}/reset-password [post]
func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	var req dto.AdminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.adminService.ResetPassword(c.Request.Context(), id, req.Password); err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, nil)
}

// Delete godoc
// @Summary 删除管理员账号
// @Tags 管理端-认证
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.AdminResignRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/admins/{id} [delete]
func (h *AdminHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	if err := h.adminService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, nil)
}

// Resign godoc
// @Summary 离职员工
// @Tags 管理端-员工组织
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.AdminResignRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/staff/{id}/resign [post]
// Resign 员工离职（S1，doc86 §2.1）：置离职标记、禁用账号并交待在途客户。
func (h *AdminHandler) Resign(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	var req dto.AdminResignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.adminService.Resign(c.Request.Context(), id, req); err != nil {
		response.ErrorWithStatus(c, http.StatusOK, apperrors.New(40001, err.Error()))
		return
	}
	response.Success(c, nil)
}

// SetRoles godoc
// @Summary 管理员账号分配角色
// @Tags 管理端-认证
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.AdminAssignRolesRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/admins/{id}/roles [put]
// SetRoles 覆盖式设置员工角色（D2 多角色）。
func (h *AdminHandler) SetRoles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, "invalid id"))
		return
	}
	var req dto.AdminAssignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.adminService.SetRoles(c.Request.Context(), id, req.RoleIDs); err != nil {
		response.ErrorWithStatus(c, http.StatusOK, apperrors.New(40001, err.Error()))
		return
	}
	response.Success(c, nil)
}

// ChangePassword godoc
// @Summary 修改管理员密码
// @Tags 管理端-认证
// @Security BearerAuth
// @Param request body dto.AdminChangePasswordRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/auth/change-password [post]
// ChangePassword 管理员自助改密（首次登录强制改密走同一接口）。
func (h *AdminHandler) ChangePassword(c *gin.Context) {
	adminClaims, ok := middleware.GetAdminClaims(c)
	if !ok {
		response.ErrorWithStatus(c, http.StatusUnauthorized, apperrors.New(10001, "unauthorized"))
		return
	}
	var req dto.AdminChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.adminService.ChangePassword(c.Request.Context(), adminClaims.AdminID, req); err != nil {
		response.ErrorWithStatus(c, http.StatusOK, apperrors.New(40001, err.Error()))
		return
	}
	response.Success(c, nil)
}

// ListAuditLogs godoc
// @Summary 管理端操作审计日志
// @Description 查询后台写操作审计（POST/PUT/PATCH/DELETE），payload 已脱敏
// @Tags 管理员
// @Security BearerAuth
// @Success 200 {object} dto.AdminAuditLogResponse
// @Router /api/v1/admin/audit-logs [get]
func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	var query dto.AdminAuditLogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.adminService.ListAuditLogs(c.Request.Context(), query)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
