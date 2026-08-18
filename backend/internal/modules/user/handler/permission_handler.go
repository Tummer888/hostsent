package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

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
