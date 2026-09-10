package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/service"
	"hostsent/backend/internal/pkg/middleware"
)

type UserHandler struct {
	userService service.UserService
	// createOrder 为指定用户创建订单（余额支付/仅创建），由装配层注入。
	createOrder func(ctx context.Context, userID uint64, req dto.AdminCreateOrderRequest) (*dto.AdminOrderBrief, error)
}

func NewUserHandler(userService service.UserService, createOrder func(ctx context.Context, userID uint64, req dto.AdminCreateOrderRequest) (*dto.AdminOrderBrief, error)) *UserHandler {
	return &UserHandler{userService: userService, createOrder: createOrder}
}

// GetStats godoc
// @Summary 用户统计
// @Description 获取用户总览统计数据（总用户/今日新增/活跃/冻结/待实名/待审核/用户总余额/已购用户数）
// @Tags 用户管理
// @Produce json
// @Success 200 {object} dto.APIResponse[dto.UserStatsResponse]
// @Router /api/v1/admin/users/stats [get]
func (h *UserHandler) GetStats(c *gin.Context) {
	stats, err := h.userService.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": stats, "timestamp": time.Now().Unix()})
}

// GetRegionStats godoc
// @Summary 用户登录 IP 归属地分布
// @Description 获取用户按登录 IP 归属地分布的统计
// @Tags 用户管理
// @Produce json
// @Success 200 {object} dto.APIResponse[dto.RegionStatsResponse]
// @Router /api/v1/admin/users/region-stats [get]
func (h *UserHandler) GetRegionStats(c *gin.Context) {
	stats, err := h.userService.GetRegionStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": stats, "timestamp": time.Now().Unix()})
}

// ListUsers godoc
// @Summary 用户列表
// @Description 获取后台用户列表，支持分页、状态筛选、地域筛选、关键词搜索与快捷过滤
// @Tags 用户管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "用户状态，如 active/disabled/pending/cancelled"
// @Param filter query string false "快捷筛选，如 today/pending_real_name"
// @Param region query string false "地域筛选"
// @Param keyword query string false "关键词，支持用户名/姓名/邮箱/手机号模糊搜索"
// @Success 200 {object} dto.APIResponse[dto.UserListResponse]
// @Router /api/v1/admin/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	var query dto.UserListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}

	users, err := h.userService.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": users, "timestamp": time.Now().Unix()})
}

// CreateUser godoc
// @Summary 创建用户
// @Description 创建后台用户并可分配角色
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param request body dto.UserCreateRequest true "用户参数"
// @Success 200 {object} dto.APIResponse[dto.UserInfo]
// @Router /api/v1/admin/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req dto.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	user, err := h.userService.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": user, "timestamp": time.Now().Unix()})
}

// GetUser godoc
// @Summary 用户详情
// @Description 获取单个后台用户详情
// @Tags 用户管理
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} dto.APIResponse[dto.UserInfo]
// @Router /api/v1/admin/users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	user, err := h.userService.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": user, "timestamp": time.Now().Unix()})
}

// ListMembers godoc
// @Summary 用户成员（子账号）列表
// @Description 列出指定主账号名下的全部子账号及各自权限（P4-10 用户详情「成员」Tab）
// @Tags 用户管理
// @Produce json
// @Param id path int true "主账号ID"
// @Success 200 {object} dto.APIResponse[dto.SubAccountMemberListResponse]
// @Router /api/v1/admin/users/{id}/members [get]
func (h *UserHandler) ListMembers(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	result, err := h.userService.ListMembers(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result, "timestamp": time.Now().Unix()})
}

// UpdateUser godoc
// @Summary 更新用户
// @Description 更新后台用户基础信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body dto.UserUpdateRequest true "用户参数"
// @Success 200 {object} dto.APIResponse[dto.UserInfo]
// @Router /api/v1/admin/users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	user, err := h.userService.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": user, "timestamp": time.Now().Unix()})
}

// UpdateUserStatus godoc
// @Summary 更新用户状态
// @Description 启用或禁用后台用户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body dto.UserStatusRequest true "状态参数"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/users/{id}/status [patch]
func (h *UserHandler) UpdateUserStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.UserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	if err := h.userService.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

// ResetPassword godoc
// @Summary 重置用户密码
// @Description 重置后台用户密码
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body dto.ResetPasswordRequest true "密码参数"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/users/{id}/reset-password [post]
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	if err := h.userService.ResetPassword(c.Request.Context(), id, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

// AssignRoles godoc
// @Summary 分配用户角色
// @Description 覆盖用户角色关系
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body dto.AssignRolesRequest true "角色ID列表"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/users/{id}/roles [post]
func (h *UserHandler) AssignRoles(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.AssignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	if err := h.userService.AssignRoles(c.Request.Context(), id, req.RoleIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

// Impersonate godoc
// @Summary 代登录用户
// @Description 以指定用户身份登录，返回用户端 token
// @Tags 用户管理
// @Param id path int true "用户ID"
// @Success 200 {object} dto.APIResponse[dto.ImpersonateResponse]
// @Router /api/v1/admin/users/{id}/impersonate [post]
func (h *UserHandler) Impersonate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	resp, err := h.userService.Impersonate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

// Recharge godoc
// @Summary 用户充值
// @Description 给指定用户钱包人工充值（收入）
// @Tags 用户管理
// @Param id path int true "用户ID"
// @Param request body dto.RechargeRequest true "充值参数"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/users/{id}/recharge [post]
func (h *UserHandler) Recharge(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.RechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	operatorID := uint64(0)
	if claims, ok := middleware.GetAdminClaims(c); ok {
		operatorID = claims.AdminID
	}
	if err := h.userService.Recharge(c.Request.Context(), id, req.Amount, req.Remark, operatorID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

// CreateOrder godoc
// @Summary 为用户创建订单
// @Description 给指定用户添加订单：余额支付并开通，或仅创建待支付
// @Tags 用户管理
// @Param id path int true "用户ID"
// @Param request body dto.AdminCreateOrderRequest true "订单参数"
// @Success 200 {object} dto.APIResponse[dto.AdminOrderBrief]
// @Router /api/v1/admin/users/{id}/orders [post]
func (h *UserHandler) CreateOrder(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.AdminCreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	operatorID := uint64(0)
	if claims, ok := middleware.GetAdminClaims(c); ok {
		operatorID = claims.AdminID
	}
	_ = operatorID
	if h.createOrder == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": "添加订单能力未配置", "timestamp": time.Now().Unix()})
		return
	}
	brief, err := h.createOrder(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": brief, "timestamp": time.Now().Unix()})
}
