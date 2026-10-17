// Package handler 提供用户中心菜单模块的 HTTP 接口。
//
// 模块形态（CONTRIBUTING §2）：uc/menu 为薄适配——service 委托 admin 域菜单
// 服务读取，本端只做请求/响应转换；不持有 repository 与 model。
package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/menu/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// MenuHandler 用户中心菜单 HTTP 处理器。
type MenuHandler struct {
	menuService service.MenuService
	// permissions 子账号权限解析器（与 RequireUserPermission 同一个实现）。
	// 可为 nil：未装配时子账号拿到空权限集，主账号不受影响。
	permissions middleware.SubAccountPermissionResolver
}

// NewMenuHandler 创建用户中心菜单处理器。
//
// 权限解析器不在这里传入：它依赖 uc/member 的仓储，而该仓储在装配序列中晚于
// 本处理器构造（buildUCAuth → buildWire）。构造期传入会拿到 nil，表现为
// 子账号菜单被过度裁剪（实测：持 instance:view 却看不到「我的云主机」）。
// 用 setter 在仓储就绪后接线。
func NewMenuHandler(menuService service.MenuService) *MenuHandler {
	return &MenuHandler{menuService: menuService}
}

// SetPermissionResolver 注入子账号权限解析器（装配层在仓储就绪后调用）。
func (h *MenuHandler) SetPermissionResolver(resolver middleware.SubAccountPermissionResolver) {
	h.permissions = resolver
}

// HasPermissionResolver 装配自检：解析器是否已接线。
//
// 存在意义：漏接线不会报错，只会让子账号菜单被静默裁剪（实测踩过 ——
// 持 instance:view 的成员看不到「我的云主机」）。启动时显式校验，
// 把「静默少显示」变成「启动即失败」。
func (h *MenuHandler) HasPermissionResolver() bool {
	return h.permissions != nil
}

// Tree 用户中心菜单树
// @Summary 用户中心菜单树
// @Description 获取用户控制台（固定 platform=user）的菜单树，供前端侧边栏渲染。platform 参数仅为兼容保留，不会切换到管理端菜单。
// @Tags 用户中心-菜单
// @Produce json
// @Security BearerAuth
// @Param platform query string false "兼容参数，无论传值一律返回 user 菜单"
// @Success 200 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/uc/menus/tree [get]
func (h *MenuHandler) Tree(c *gin.Context) {
	// 按登录者裁剪：主账号全量，子账号去掉主账号专属入口与自己没有权限的页面。
	// 权限集合从令牌携带的 claims + 权限解析器取（与 RequireUserPermission 同源），
	// 避免「菜单看得见、点进去 403」。
	tree, err := h.menuService.Tree(c.Request.Context(), h.viewerProfile(c))
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, tree)
}

// viewerProfile 组装菜单裁剪所需的登录者画像。
//
// 解析失败（查库异常）时按「子账号 + 零权限」处理：宁可少显示菜单，
// 也不能因为一次查询失败就把主账号专属入口下发给子账号。
func (h *MenuHandler) viewerProfile(c *gin.Context) service.ViewerProfile {
	if !middleware.IsSubAccount(c) {
		return service.ViewerProfile{}
	}
	profile := service.ViewerProfile{IsSub: true}
	if h.permissions == nil {
		return profile
	}
	codes, err := h.permissions.PermissionsOf(c.Request.Context(), middleware.ActorUserID(c))
	if err != nil {
		return profile
	}
	profile.Permissions = codes
	return profile
}
