package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

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
