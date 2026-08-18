package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/service"
)

type PermissionHandler struct {
	permissionService service.PermissionService
}

func NewPermissionHandler(permissionService service.PermissionService) *PermissionHandler {
	return &PermissionHandler{permissionService: permissionService}
}

// Tree godoc
// @Summary 权限树
// @Description 获取目录、菜单、按钮树
// @Tags 权限管理
// @Produce json
// @Success 200 {object} dto.APIResponse[[]dto.PermissionNode]
// @Router /api/v1/permissions/tree [get]
func (h *PermissionHandler) Tree(c *gin.Context) {
	tree, err := h.permissionService.Tree(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": tree, "timestamp": time.Now().Unix()})
}

// CreatePermission godoc
// @Summary 创建权限节点
// @Description 创建目录、菜单或按钮节点
// @Tags 权限管理
// @Accept json
// @Produce json
// @Param request body dto.PermissionCreateRequest true "权限参数"
// @Success 200 {object} dto.APIResponse[dto.PermissionNode]
// @Router /api/v1/permissions [post]
func (h *PermissionHandler) CreatePermission(c *gin.Context) {
	var req dto.PermissionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	permission, err := h.permissionService.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": permission, "timestamp": time.Now().Unix()})
}

// UpdatePermission godoc
// @Summary 更新权限节点
// @Description 更新目录、菜单或按钮节点
// @Tags 权限管理
// @Accept json
// @Produce json
// @Param id path int true "权限ID"
// @Param request body dto.PermissionUpdateRequest true "权限参数"
// @Success 200 {object} dto.APIResponse[dto.PermissionNode]
// @Router /api/v1/permissions/{id} [put]
func (h *PermissionHandler) UpdatePermission(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.PermissionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	permission, err := h.permissionService.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": permission, "timestamp": time.Now().Unix()})
}

// DeletePermission godoc
// @Summary 删除权限节点
// @Description 删除目录、菜单或按钮节点
// @Tags 权限管理
// @Produce json
// @Param id path int true "权限ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/permissions/{id} [delete]
func (h *PermissionHandler) DeletePermission(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.permissionService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}
