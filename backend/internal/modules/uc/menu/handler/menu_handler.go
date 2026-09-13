// Package handler 提供用户中心菜单模块的 HTTP 接口。
package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/menu/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// MenuHandler 用户中心菜单 HTTP 处理器。
type MenuHandler struct {
	menuService service.MenuService
}

// NewMenuHandler 创建用户中心菜单处理器。
func NewMenuHandler(menuService service.MenuService) *MenuHandler {
	return &MenuHandler{menuService: menuService}
}

// Tree 用户中心菜单树
// @Summary 用户中心菜单树
// @Description 获取用户控制台（固定 platform=user）的菜单树，供前端侧边栏渲染。platform 参数仅为兼容保留，不会切换到管理端菜单。
// @Tags 用户中心-菜单
// @Produce json
// @Security BearerAuth
// @Param platform query string false "兼容参数，无论传值一律返回 user 菜单"
// @Success 200 {object} response.Body{data=[]dto.MenuNode}
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/uc/menus/tree [get]
func (h *MenuHandler) Tree(c *gin.Context) {
	tree, err := h.menuService.Tree(c.Request.Context(), c.Query("platform"))
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, tree)
}
