package service

import (
	"context"
	"testing"

	adminmenu "hostsent/backend/internal/modules/admin/menu/dto"
	"hostsent/backend/internal/modules/uc/menu/dto"
	appauth "hostsent/backend/internal/pkg/auth"
)

// fakeMenuReader 返回固定树，避免测试依赖数据库。
type fakeMenuReader struct{ nodes []adminmenu.MenuNode }

func (f *fakeMenuReader) Tree(context.Context, string) ([]adminmenu.MenuNode, error) {
	return f.nodes, nil
}

// userMenuTreeFixture 复刻线上 user 平台菜单结构（含目录与主账号专属项）。
func userMenuTreeFixture() []adminmenu.MenuNode {
	return []adminmenu.MenuNode{
		{ID: 87, Name: "控制台", Path: "/dashboard", Type: "menu"},
		{ID: 88, Name: "云产品", Type: "directory", Children: []adminmenu.MenuNode{
			{ID: 89, Name: "我的云主机", Path: "/cloud/instances", Type: "menu"},
			{ID: 132, Name: "续费管理", Path: "/cloud/renewals", Type: "menu"},
		}},
		{ID: 217, Name: "云主机选购", Path: "/shop", Type: "menu"},
		{ID: 91, Name: "我的订单", Path: "/order", Type: "menu"},
		{ID: 92, Name: "费用中心", Path: "/billing", Type: "menu"},
		{ID: 209, Name: "我的积分", Path: "/points", Type: "menu"},
		{ID: 93, Name: "工单中心", Type: "directory", Children: []adminmenu.MenuNode{
			{ID: 94, Name: "我的工单", Path: "/support/tickets", Type: "menu"},
		}},
		{ID: 218, Name: "成员管理", Path: "/member", Type: "menu"},
		{ID: 175, Name: "推广邀请", Type: "directory", Children: []adminmenu.MenuNode{
			{ID: 176, Name: "推广概览", Path: "/referral/overview", Type: "menu"},
			{ID: 179, Name: "提现与转出", Path: "/referral/withdrawals", Type: "menu"},
		}},
	}
}

func collectPaths(nodes []dto.MenuNode) []string {
	var out []string
	for _, n := range nodes {
		if n.Path != "" {
			out = append(out, n.Path)
		}
		out = append(out, collectPaths(n.Children)...)
	}
	return out
}

func hasPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

func newMenuService(reader MenuReader) MenuService { return NewMenuService(reader) }

// TestOwnerSeesFullMenu 主账号不受裁剪：一个入口都不该少。
func TestOwnerSeesFullMenu(t *testing.T) {
	svc := newMenuService(&fakeMenuReader{nodes: userMenuTreeFixture()})
	tree, err := svc.Tree(context.Background(), ViewerProfile{IsSub: false})
	if err != nil {
		t.Fatalf("Tree 失败: %v", err)
	}
	paths := collectPaths(tree)
	for _, want := range []string{"/member", "/referral/withdrawals", "/billing", "/shop"} {
		if !hasPath(paths, want) {
			t.Fatalf("主账号应看到 %s，实际菜单: %v", want, paths)
		}
	}
}

// TestSubAccountHidesOwnerOnlyAndUngranted 子账号：主账号专属入口与无权限页面都要隐藏，
// 且裁剪后为空的目录不再下发（避免侧边栏出现点不开的空目录）。
func TestSubAccountHidesOwnerOnlyAndUngranted(t *testing.T) {
	svc := newMenuService(&fakeMenuReader{nodes: userMenuTreeFixture()})
	// 只授予 instance:view：续费管理(order:view)、选购/订单(order:create)、
	// 费用中心/积分(billing:view)、工单(ticket:view) 都应不可见。
	tree, err := svc.Tree(context.Background(), ViewerProfile{
		IsSub:       true,
		Permissions: []string{appauth.PermInstanceView},
	})
	if err != nil {
		t.Fatalf("Tree 失败: %v", err)
	}
	paths := collectPaths(tree)

	// 主账号专属（与权限无关，永不授予子账号）
	for _, hidden := range []string{"/member", "/referral/overview", "/referral/withdrawals"} {
		if hasPath(paths, hidden) {
			t.Fatalf("子账号不该看到 %s，实际菜单: %v", hidden, paths)
		}
	}
	// 未授予权限的页面
	for _, hidden := range []string{"/cloud/renewals", "/shop", "/order", "/billing", "/points", "/support/tickets"} {
		if hasPath(paths, hidden) {
			t.Fatalf("子账号无对应权限，不该看到 %s，实际菜单: %v", hidden, paths)
		}
	}
	// 已授予的仍可见
	if !hasPath(paths, "/cloud/instances") {
		t.Fatalf("子账号持 instance:view，应看到 /cloud/instances，实际: %v", paths)
	}
	// 无权限门槛的页面保留（个人中心/控制台）
	if !hasPath(paths, "/dashboard") {
		t.Fatalf("无需权限的 /dashboard 不该被裁掉，实际: %v", paths)
	}
	// 裁剪后变空的目录节点本身也不下发
	for _, n := range tree {
		if n.Name == "工单中心" || n.Name == "推广邀请" {
			t.Fatalf("子节点被裁空的目录 %q 不该下发，实际: %+v", n.Name, tree)
		}
	}
}

// TestSubAccountWithAllPermissions 授予全部可授予权限时，仍看不到主账号专属入口。
func TestSubAccountWithAllPermissions(t *testing.T) {
	svc := newMenuService(&fakeMenuReader{nodes: userMenuTreeFixture()})
	tree, err := svc.Tree(context.Background(), ViewerProfile{
		IsSub:       true,
		Permissions: appauth.UserPermissionDefault,
	})
	if err != nil {
		t.Fatalf("Tree 失败: %v", err)
	}
	paths := collectPaths(tree)
	if hasPath(paths, "/member") {
		t.Fatalf("成员管理永不授予子账号，不该出现在菜单: %v", paths)
	}
	if hasPath(paths, "/referral/withdrawals") {
		t.Fatalf("推广提现对子账号硬拒绝，不该出现在菜单: %v", paths)
	}
	for _, want := range []string{"/cloud/instances", "/cloud/renewals", "/shop", "/order", "/billing", "/support/tickets"} {
		if !hasPath(paths, want) {
			t.Fatalf("默认权限集应能看见 %s，实际: %v", want, paths)
		}
	}
}

// TestSubAccountZeroPermissionsKeepsPublicPages 零权限时不 panic 且仍保留无需权限的页面。
func TestSubAccountZeroPermissionsKeepsPublicPages(t *testing.T) {
	svc := newMenuService(&fakeMenuReader{nodes: userMenuTreeFixture()})
	tree, err := svc.Tree(context.Background(), ViewerProfile{IsSub: true})
	if err != nil {
		t.Fatalf("Tree 失败: %v", err)
	}
	paths := collectPaths(tree)
	if !hasPath(paths, "/dashboard") {
		t.Fatalf("零权限也应保留无需权限的页面，实际: %v", paths)
	}
	if hasPath(paths, "/billing") {
		t.Fatalf("零权限不该看到 /billing，实际: %v", paths)
	}
}
