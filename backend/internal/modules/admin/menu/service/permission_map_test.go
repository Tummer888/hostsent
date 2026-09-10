package service

import (
	"testing"

	"hostsent/backend/internal/modules/admin/menu/dto"
	appauth "hostsent/backend/internal/pkg/auth"
)

// 构造一棵「用户管理 + 推广返现」的菜单树，覆盖 permission_map 中新增登记的路径。
func buildTestTree() []dto.MenuNode {
	return []dto.MenuNode{
		{
			Name: "仪表盘", Path: "/dashboard", Type: "directory",
			Children: []dto.MenuNode{{Name: "概览", Path: "/dashboard/base", Type: "menu"}},
		},
		{
			Name: "用户管理", Path: "/users", Type: "directory",
			Children: []dto.MenuNode{
				{Name: "用户总览", Path: "/users/overview", Type: "menu"},
				{Name: "账户管理", Path: "/users/accounts", Type: "directory", Children: []dto.MenuNode{
					{Name: "用户列表", Path: "/users/accounts/list", Type: "menu"},
					{Name: "用户组", Path: "/users/accounts/groups", Type: "menu"},
				}},
				{Name: "安全与风控", Path: "/users/security", Type: "directory", Children: []dto.MenuNode{
					{Name: "登录日志", Path: "/users/security/login-logs", Type: "menu"},
					{Name: "会话管理", Path: "/users/security/sessions", Type: "menu"},
				}},
				{Name: "用户等级", Path: "/users/levels", Type: "menu"},
				{Name: "实名认证", Path: "/users/verification", Type: "directory", Children: []dto.MenuNode{
					{Name: "待审核", Path: "/users/verification/pending", Type: "menu"},
				}},
			},
		},
		{
			Name: "推广返现", Path: "/referral", Type: "directory",
			Children: []dto.MenuNode{
				{Name: "返现台账", Path: "/referral/cashbacks", Type: "menu"},
				{Name: "提现审核", Path: "/referral/withdrawals", Type: "menu"},
				{Name: "邀请关系", Path: "/referral/invitees", Type: "menu"},
			},
		},
	}
}

// flattenPaths 收集过滤后树中的全部叶子路径。
func flattenPaths(nodes []dto.MenuNode) map[string]bool {
	out := map[string]bool{}
	var walk func([]dto.MenuNode)
	walk = func(items []dto.MenuNode) {
		for _, n := range items {
			if len(n.Children) == 0 {
				out[n.Path] = true
				continue
			}
			walk(n.Children)
		}
	}
	walk(nodes)
	return out
}

func hasPath(nodes []dto.MenuNode, path string) bool {
	return flattenPaths(nodes)[path]
}

// 只持有基础用户查看权限：能看总览/列表/用户组，看不到等级、安全、实名、返现。
func TestFilterByPermissions_NarrowUserPerms(t *testing.T) {
	perms := appauth.NewPermissionSet([]string{"system:user:list", "user:group:list"})
	got := FilterByPermissions(buildTestTree(), perms)

	for _, want := range []string{"/dashboard/base", "/users/overview", "/users/accounts/list", "/users/accounts/groups"} {
		if !hasPath(got, want) {
			t.Errorf("应保留 %s，但被过滤掉了", want)
		}
	}
	for _, unexpected := range []string{
		"/users/security/login-logs", "/users/security/sessions",
		"/users/levels", "/users/verification/pending", "/referral/cashbacks",
	} {
		if hasPath(got, unexpected) {
			t.Errorf("无权限却保留了 %s", unexpected)
		}
	}
	if hasPath(got, "/referral") {
		t.Error("推广返现下所有子菜单均无权限，整个目录应被移除")
	}
}

// 持有等级权限：目录保留，等级菜单可见。
func TestFilterByPermissions_LevelOnly(t *testing.T) {
	perms := appauth.NewPermissionSet([]string{"level:list"})
	got := FilterByPermissions(buildTestTree(), perms)

	if !hasPath(got, "/users/levels") {
		t.Fatal("持有 level:list 时应保留 /users/levels")
	}
	if hasPath(got, "/users/overview") {
		t.Error("未持有 system:user:list，不应看到用户总览")
	}
	if hasPath(got, "/users/security/login-logs") {
		t.Error("未持有 security 权限，不应看到安全与风控")
	}
}

// 返现两条权限相互独立：只持台账权限时看不到提现审核。
func TestFilterByPermissions_ReferralSplit(t *testing.T) {
	perms := appauth.NewPermissionSet([]string{"referral:cashback:list"})
	got := FilterByPermissions(buildTestTree(), perms)

	for _, want := range []string{"/referral/cashbacks", "/referral/invitees"} {
		if !hasPath(got, want) {
			t.Errorf("应保留 %s", want)
		}
	}
	if hasPath(got, "/referral/withdrawals") {
		t.Error("未持有 referral:withdraw:list，不应看到提现审核")
	}
}

// 超管通配集合原样返回。
func TestFilterByPermissions_Super(t *testing.T) {
	perms := appauth.NewPermissionSet([]string{appauth.SuperPermission})
	got := FilterByPermissions(buildTestTree(), perms)
	if len(got) != 3 {
		t.Fatalf("超管应看到全部 3 个一级目录，实际 %d", len(got))
	}
}
