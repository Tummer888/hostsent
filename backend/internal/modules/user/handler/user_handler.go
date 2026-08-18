package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/service"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// ListUsers godoc
// @Summary 用户列表
// @Description 获取后台用户列表
// @Tags 用户管理
// @Produce json
// @Success 200 {object} dto.APIResponse[[]dto.UserInfo]
// @Router /api/v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	users, err := h.userService.List(c.Request.Context())
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
// @Router /api/v1/users [post]
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

// AssignRoles godoc
// @Summary 分配用户角色
// @Description 覆盖用户角色关系
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body dto.AssignRolesRequest true "角色ID列表"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/users/{id}/roles [post]
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
