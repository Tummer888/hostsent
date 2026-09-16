// Package handler 提供菜单模块的 HTTP 接口。
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/menu/service"
	"hostsent/backend/internal/pkg/middleware"
)

// MenuHandler 菜单接口处理器。
//
// 只读：菜单树是 seed（internal/pkg/db 的 SeedMenus）在 menus 表上的投影，
// 运营侧的新增/编辑/删除入口已下线（doc102 M0），改菜单要改 seed 并发版。
type MenuHandler struct {
	menuService service.MenuService
}

// NewMenuHandler 创建菜单接口处理器。
func NewMenuHandler(menuService service.MenuService) *MenuHandler {
	return &MenuHandler{menuService: menuService}
}

// Tree godoc
// @Summary 菜单树
// @Description 按平台获取管理员后台或用户中心菜单树
// @Tags 菜单管理
// @Produce json
// @Param platform query string false "平台：admin(默认) 或 user"
// @Success 200 {object} dto.APIResponse[[]dto.MenuNode]
// @Router /api/v1/admin/menus/tree [get]
func (h *MenuHandler) Tree(c *gin.Context) {
	platform := c.DefaultQuery("platform", "admin")
	tree, err := h.menuService.Tree(c.Request.Context(), platform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	// 后台菜单按当前管理员权限过滤后再返回（82 P1-13）：前端直接渲染，不做权限过滤。
	if platform == "admin" {
		if grant, ok := middleware.GetAdminGrant(c); ok {
			tree = service.FilterByPermissions(tree, grant.Perms)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": tree, "timestamp": time.Now().Unix()})
}
