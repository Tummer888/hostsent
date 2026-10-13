// Package service 用户中心菜单薄适配（CONTRIBUTING §2）：只做请求/响应转换，
// 委托 admin 域 menu service 读取菜单树，不持有任何表与仓储。
package service

import (
	"context"
	"strings"

	adminmenu "hostsent/backend/internal/modules/admin/menu/dto"
	"hostsent/backend/internal/modules/uc/menu/dto"
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
	Tree(ctx context.Context, platform string) ([]dto.MenuNode, error)
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
func (s *menuService) Tree(ctx context.Context, _ string) ([]dto.MenuNode, error) {
	nodes, err := s.reader.Tree(ctx, "user")
	if err != nil {
		return nil, err
	}
	return toUserNodes(nodes), nil
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
