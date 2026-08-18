package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/service"
)

type RoleHandler struct {
	roleService service.RoleService
}

func NewRoleHandler(roleService service.RoleService) *RoleHandler {
	return &RoleHandler{roleService: roleService}
}

// ListRoles godoc
// @Summary 角色列表
// @Description 获取角色列表
// @Tags 角色管理
// @Produce json
// @Success 200 {object} dto.APIResponse[[]dto.RoleInfo]
// @Router /api/v1/roles [get]
func (h *RoleHandler) ListRoles(c *gin.Context) {
	roles, err := h.roleService.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": roles, "timestamp": time.Now().Unix()})
}

// CreateRole godoc
// @Summary 创建角色
// @Description 创建后台角色
// @Tags 角色管理
// @Accept json
// @Produce json
// @Param request body dto.RoleCreateRequest true "角色参数"
// @Success 200 {object} dto.APIResponse[dto.RoleInfo]
// @Router /api/v1/roles [post]
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var req dto.RoleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	role, err := h.roleService.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": role, "timestamp": time.Now().Unix()})
}

// GetRole godoc
// @Summary 角色详情
// @Description 获取单个角色详情
// @Tags 角色管理
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} dto.APIResponse[dto.RoleInfo]
// @Router /api/v1/roles/{id} [get]
func (h *RoleHandler) GetRole(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	role, err := h.roleService.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": role, "timestamp": time.Now().Unix()})
}

// UpdateRole godoc
// @Summary 更新角色
// @Description 更新角色信息
// @Tags 角色管理
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Param request body dto.RoleUpdateRequest true "角色参数"
// @Success 200 {object} dto.APIResponse[dto.RoleInfo]
// @Router /api/v1/roles/{id} [put]
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.RoleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	role, err := h.roleService.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": role, "timestamp": time.Now().Unix()})
}

// DeleteRole godoc
// @Summary 删除角色
// @Description 删除后台角色
// @Tags 角色管理
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/roles/{id} [delete]
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.roleService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}

// GetRolePermissions godoc
// @Summary 角色权限列表
// @Description 获取角色当前绑定的权限ID集合
// @Tags 角色管理
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} dto.APIResponse[[]uint64]
// @Router /api/v1/roles/{id}/permissions [get]
func (h *RoleHandler) GetRolePermissions(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	permissions, err := h.roleService.Permissions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": permissions, "timestamp": time.Now().Unix()})
}

// AssignPermissions godoc
// @Summary 分配角色权限
// @Description 覆盖角色权限关系
// @Tags 角色管理
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Param request body dto.AssignPermissionsRequest true "权限ID列表"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/roles/{id}/permissions [post]
func (h *RoleHandler) AssignPermissions(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.AssignPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	if err := h.roleService.AssignPermissions(c.Request.Context(), id, req.PermissionIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}
