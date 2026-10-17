// Package service 用户中心菜单薄适配（CONTRIBUTING §2）：只做请求/响应转换，
// 委托 admin 域 menu service 读取菜单树，不持有任何表与仓储。
package service

import (
	"context"
	"strings"

	adminmenu "hostsent/backend/internal/modules/admin/menu/dto"
	"hostsent/backend/internal/modules/uc/menu/dto"
	appauth "hostsent/backend/internal/pkg/auth"
)

// MenuReader admin 域菜单树的只读能力（装配层注入 admin/menu/service.MenuService）。
// 只声明本模块用到的方法，避免直接依赖 admin service 实现。
type MenuReader interface {
	Tree(ctx context.Context, platform string) ([]adminmenu.MenuNode, error)
}

// MenuService 用户中心菜单服务接口。
type MenuService interface {
	// Tree 构建用户端菜单树。platform 参数只作兼容保留：用户中心只服务 platform=user，
	// 传入 admin 等其它值时同样回落 user，避免普通用户借此读取管理端菜单结构。
	//
	// profile 描述当前登录者（是否子账号、持有哪些客户侧权限码），用于按权限裁剪：
	// 子账号不该看到成员管理、推广提现这类主账号专属入口，也不该看到自己没有权限的页面。
	Tree(ctx context.Context, profile ViewerProfile) ([]dto.MenuNode, error)
}

// ViewerProfile 菜单裁剪所需的登录者信息。
//
// 为什么由调用方（handler）传入而不是在 service 里读 token：
// service 在 uc/menu 是薄适配层，不接触 HTTP 上下文；把鉴权信息的读取留在
// handler 一侧，既能被单测直接覆盖，也避免本层偷偷依赖 gin。
type ViewerProfile struct {
	// IsSub 是否子账号（主账号拥有全部客户侧权限，不做裁剪）。
	IsSub bool
	// Permissions 子账号已授予的客户侧权限码（主账号忽略）。
	Permissions []string
}

type menuService struct {
	reader MenuReader
}

// NewMenuService 创建用户中心菜单服务实例（薄适配：委托 admin 域 service）。
func NewMenuService(reader MenuReader) MenuService {
	return &menuService{reader: reader}
}

// Tree 委托 admin 域读取 user 平台菜单并转换为用户端节点结构。
//
// platform 不参与选择：用户中心菜单接口固定 platform=user。历史上该参数由客户端传入，
// 普通用户可用 ?platform=admin 拿到管理端菜单结构（路径/图标/层级），属信息越权；
// 这里改为一律回落 user，参数仅作向后兼容保留。
func (s *menuService) Tree(ctx context.Context, viewer ViewerProfile) ([]dto.MenuNode, error) {
	nodes, err := s.reader.Tree(ctx, "user")
	if err != nil {
		return nil, err
	}
	converted := toUserNodes(nodes)
	if !viewer.IsSub {
		return converted, nil
	}
	return filterForSubAccount(converted, viewer.Permissions), nil
}

// userMenuPermissionMap 用户端菜单路径 → 所需客户侧权限码。
//
// 与 pkg/auth 的枚举一一对应，只列「有权限门槛」的路径：未登记的路径视为
// 无需权限（与后台 menuPermissionMap 同一约定）。菜单是导航，不是安全边界 ——
// 真正的拦截在 RequireUserPermission；这里只是让子账号看不到点进去必然 403 的入口。
var userMenuPermissionMap = map[string]string{
	"/cloud/instances": appauth.PermInstanceView,
	"/cloud/renewals":  appauth.PermOrderView,
	"/shop":            appauth.PermOrderCreate,
	"/order":           appauth.PermOrderView,
	"/billing":         appauth.PermBillingView,
	"/points":          appauth.PermBillingView,
	"/support/tickets": appauth.PermTicketView,
}

// ownerOnlyMenuPaths 仅主账号可见的菜单路径（与前端 OWNER_ONLY_MENU_PATHS 同口径）。
//
// 成员管理按设计只归主账号（subaccount:manage 永不授予子账号）；推广提现是资金
// 出口、域名下的所有接口都挂了 rejectSub，子账号点进去只会被拒。放在服务端裁剪，
// 前端就不必维护一份「从菜单树里认路径」的过滤清单 —— 两端各存一份必然漂移。
var ownerOnlyMenuPaths = map[string]bool{
	"/member":               true,
	"/referral":             true,
	"/referral/overview":    true,
	"/referral/invitees":    true,
	"/referral/cashbacks":   true,
	"/referral/withdrawals": true,
	"/referral/materials":   true,
}

// filterForSubAccount 子账号可见性裁剪：去掉主账号专属路径与未授予权限的页面。
//
// 目录节点的处理：裁剪后若子节点全空，则整个目录也不再下发（否则侧边栏会留下
// 一个点不开的空目录）。这是递归自底向上的原因。
func filterForSubAccount(nodes []dto.MenuNode, granted []string) []dto.MenuNode {
	allow := make(map[string]bool, len(granted))
	for _, code := range granted {
		allow[code] = true
	}
	out := make([]dto.MenuNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Path != "" && ownerOnlyMenuPaths[n.Path] {
			continue
		}
		if need, ok := userMenuPermissionMap[n.Path]; ok && !allow[need] {
			continue
		}
		if len(n.Children) > 0 {
			n.Children = filterForSubAccount(n.Children, granted)
			// 目录原本有子节点、裁完为空 → 整个目录隐藏；原本就没有子节点的
			// 目录节点保留（如纯分组占位），避免误删运营有意留下的结构。
			if len(n.Children) == 0 {
				continue
			}
		}
		out = append(out, n)
	}
	return out
}

// toUserNodes 管理端菜单节点 → 用户端节点（只投影用户侧边栏需要的字段）。
func toUserNodes(nodes []adminmenu.MenuNode) []dto.MenuNode {
	out := make([]dto.MenuNode, 0, len(nodes))
	for _, n := range nodes {
		if strings.TrimSpace(n.Path) == "" && strings.TrimSpace(n.Type) == "" {
			continue // 用户端不渲染无路径的纯分组占位节点
		}
		out = append(out, dto.MenuNode{
			ID:       n.ID,
			ParentID: n.ParentID,
			Name:     n.Name,
			Type:     n.Type,
			Path:     n.Path,
			Icon:     n.Icon,
			Children: toUserNodes(n.Children),
		})
	}
	return out
}
