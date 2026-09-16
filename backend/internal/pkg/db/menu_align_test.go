package db_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	menuservice "hostsent/backend/internal/modules/admin/menu/service"
	"hostsent/backend/internal/pkg/db"
)

// 菜单四处对齐门禁（doc102 §6）。
//
// 四个对齐点：
//  ① 菜单种子     backend/internal/pkg/db/db.go 的 SeedMenus()
//  ② 菜单→权限映射 backend/internal/modules/admin/menu/service/permission_map.go
//  ③ 前端路由     frontend-admin/src/router/index.ts（含 meta.permission）
//  ④ 静态兜底菜单 frontend-admin/src/permission.ts 的 navMenu
//
// 断言只针对**启用**（Status=active）的 admin 叶子 —— 禁用行必须删除（doc102 R8），
// 所以它们既不在数据库里，也不该出现在种子中。

// ---------- 仓库根定位 ----------

// repoRoot 从测试文件所在目录逐级向上找仓库根：同时含 backend/go.mod（后端子仓）
// 与 frontend-admin/src/router/index.ts（前端子仓）的那一层。
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if fileExists(filepath.Join(dir, "backend", "go.mod")) &&
			fileExists(filepath.Join(dir, "frontend-admin/src/router/index.ts")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("前端不在当前检出中，跳过菜单对齐门禁")
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return string(raw)
}

// ---------- TypeScript 字面量解析（不引入 node/vitest：frontend-admin 无测试框架） ----------

type routeEntry struct {
	Path       string
	Component  string
	Redirect   string
	Permission string
}

var (
	tsPathRe      = regexp.MustCompile(`\bpath:\s*'([^']*)'`)
	tsComponentRe = regexp.MustCompile(`\bcomponent:\s*\(\)\s*=>\s*import\('([^']+)'\)`)
	tsRedirectRe  = regexp.MustCompile(`\bredirect:\s*'([^']*)'`)
	// permission 取单值（'x'）或数组（['x','y']）两种写法；\s* 后只吃到收尾引号/方括号，
	// 避免把同一行的 role / channelKind 等字段一并吞进来。
	tsPermissionRe = regexp.MustCompile(`\bpermission:\s*(\[[^\]]*\]|'[^']*')`)
	tsStringRe     = regexp.MustCompile(`'([^']*)'`)
)

// parseRouter 扫描 router/index.ts，拼出每条路由的完整路径。
//
// 这里是纯文本扫描（frontend-admin 未安装任何测试框架，不引入 node/vitest）：
//   - 用花括号/方括号累计深度，深度在「path 行」上的值即该路由对象的嵌套层级；
//   - 记录 (depth, fullPath) 栈：遇到 path 时取栈内深度更浅的最近一条作为前缀，
//     因此 `path: 'accounts/list'` 会被拼成 `/users/accounts/list`；
//   - 对象闭合（深度回退）时把更深的记录弹出，避免相邻兄弟块的路径互相污染。
//   - permission 可能是单值或数组写法，两种都取到全部权限码。
func parseRouter(t *testing.T, src string) []routeEntry {
	t.Helper()
	type frame struct {
		depth int
		path  string
	}
	var (
		out   []routeEntry
		stack []frame
		depth int
		cur   = -1 // 当前正在累积的 routeEntry 下标
	)
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
			continue
		}

		open := strings.Count(line, "{") + strings.Count(line, "[") -
			strings.Count(line, "}") - strings.Count(line, "]")

		// path 行按「本行之前」的深度入栈（对象自身的左括号在上一行）。
		if m := tsPathRe.FindStringSubmatch(line); m != nil {
			for len(stack) > 0 && stack[len(stack)-1].depth >= depth {
				stack = stack[:len(stack)-1]
			}
			prefix := ""
			if len(stack) > 0 {
				prefix = stack[len(stack)-1].path
			}
			full := joinRoutePath(prefix, m[1])
			stack = append(stack, frame{depth: depth, path: full})
			if strings.Contains(line, "{") && open <= 0 {
				// 单行对象（`{ path: 'x', redirect: 'y' },`）：本行即自闭合，不留在栈里。
				// 注意：普通的 `path: 'x',` 行同样 open==0，但它所在的对象在后面几行才闭合，
				// 必须留在栈里供子路由拼接。
				stack = stack[:len(stack)-1]
			}
			out = append(out, routeEntry{Path: full})
			cur = len(out) - 1
		}
		if cur >= 0 {
			if m := tsComponentRe.FindStringSubmatch(line); m != nil {
				out[cur].Component = m[1]
			}
			if m := tsRedirectRe.FindStringSubmatch(line); m != nil {
				out[cur].Redirect = m[1]
			}
			if m := tsPermissionRe.FindStringSubmatch(line); m != nil {
				var codes []string
				for _, s := range tsStringRe.FindAllStringSubmatch(m[1], -1) {
					codes = append(codes, s[1])
				}
				out[cur].Permission = strings.Join(codes, ",")
			}
		}

		depth += open
		if depth < 0 {
			depth = 0
		}
		for len(stack) > 0 && stack[len(stack)-1].depth > depth {
			stack = stack[:len(stack)-1]
		}
	}
	return out
}

// joinRoutePath 把子路由的 path 接到父路径上：绝对路径（以 / 开头）直接采用，
// 相对路径拼到父路径之后（`'accounts/list'` + `/users` → `/users/accounts/list`）。
func joinRoutePath(prefix, path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	if prefix == "" || prefix == "/" {
		return "/" + strings.Trim(path, "/")
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.Trim(path, "/")
}

// parseNavMenuPaths 从 permission.ts 的 navMenu 数组中取出全部 path 字面量。
func parseNavMenuPaths(t *testing.T, src string) []string {
	t.Helper()
	start := strings.Index(src, "export const navMenu = [")
	if start < 0 {
		t.Fatal("permission.ts 中找不到 navMenu 定义")
	}
	rest := src[start:]
	end := strings.Index(rest, "\n]\n")
	if end < 0 {
		t.Fatal("permission.ts 中 navMenu 数组未正常闭合")
	}
	var out []string
	for _, m := range tsPathRe.FindAllStringSubmatch(rest[:end], -1) {
		out = append(out, m[1])
	}
	return out
}

// ---------- 种子视图 ----------

type seedView struct {
	Platform string
	Name     string
	Type     string
	Path     string
	Status   string
	Parent   string // ParentKey，空表示一级
}

func seedViews() []seedView {
	items := db.SeedMenus()
	out := make([]seedView, 0, len(items))
	for _, item := range items {
		out = append(out, seedView{
			Platform: item.Platform,
			Name:     item.Name,
			Type:     item.Type,
			Path:     item.Path,
			Status:   item.Status,
			Parent:   item.ParentKey,
		})
	}
	return out
}

func activeLeaves(platform string) []seedView {
	var out []seedView
	for _, item := range seedViews() {
		if item.Platform == platform && item.Type == "menu" && item.Status == "active" {
			out = append(out, item)
		}
	}
	return out
}

// ---------- 断言 ----------

// 断言 1：每个启用 admin 叶子的 Path 在 router 中有组件路由或 redirect（菜单点了不能 404）。
func TestMenuAlign_ActiveLeavesResolveInRouter(t *testing.T) {
	root := repoRoot(t)
	src := readFile(t, filepath.Join(root, "frontend-admin", "src", "router", "index.ts"))
	routes := parseRouter(t, src)

	resolvable := map[string]bool{}
	for _, r := range routes {
		if r.Component != "" || r.Redirect != "" {
			resolvable[r.Path] = true
		}
	}
	var missing []string
	for _, leaf := range activeLeaves("admin") {
		if !resolvable[leaf.Path] {
			missing = append(missing, leaf.Path)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("以下启用菜单在 router 中既无组件也无 redirect（点了 404）：\n  %s", strings.Join(missing, "\n  "))
	}
}

// 断言 2：每个启用 admin 叶子的 Path 在 PermissionMap() 中有键（值可为空串）。
// 漏登记 = 菜单对所有角色可见（permission_map 未命中时按「无需权限」放行）。
func TestMenuAlign_ActiveLeavesRegisteredInPermissionMap(t *testing.T) {
	perms := menuservice.PermissionMap()
	var missing []string
	for _, leaf := range activeLeaves("admin") {
		if _, ok := perms[leaf.Path]; !ok {
			missing = append(missing, leaf.Path)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("以下启用菜单未登记到 permission_map（等于对所有角色可见）：\n  %s", strings.Join(missing, "\n  "))
	}
}

// 断言 3：router meta.permission 与 PermissionMap() 对同一路径必须给出相同权限码。
func TestMenuAlign_PermissionCodesAgree(t *testing.T) {
	root := repoRoot(t)
	src := readFile(t, filepath.Join(root, "frontend-admin", "src", "router", "index.ts"))
	routes := parseRouter(t, src)
	perms := menuservice.PermissionMap()

	var conflicts []string
	for _, r := range routes {
		if r.Permission == "" {
			continue
		}
		code, ok := perms[r.Path]
		if !ok {
			continue
		}
		if code == r.Permission {
			continue
		}
		// 数组写法：任一命中即视为一致。
		if strings.Contains(r.Permission, code) && code != "" {
			continue
		}
		conflicts = append(conflicts, fmt.Sprintf("%s: router=%q permission_map=%q", r.Path, r.Permission, code))
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		t.Errorf("router 与 permission_map 权限口径冲突：\n  %s", strings.Join(conflicts, "\n  "))
	}
}

// 断言 4：启用叶子的 Component（非空时）解析到对应前端仓库的 pages/<Component>.vue。
func TestMenuAlign_ComponentsResolve(t *testing.T) {
	root := repoRoot(t)
	var missing []string
	for _, item := range seedViews() {
		component := componentOf(item.Path, item.Platform)
		if component == "" || item.Status != "active" {
			continue
		}
		repo := "frontend-admin"
		if item.Platform == "user" {
			repo = "frontend-user"
		}
		path := filepath.Join(root, repo, "src", "pages", component+".vue")
		if !fileExists(path) {
			missing = append(missing, fmt.Sprintf("%s (%s) → %s/src/pages/%s.vue", item.Path, item.Platform, repo, component))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("以下菜单的 component 指向不存在的页面文件：\n  %s", strings.Join(missing, "\n  "))
	}
}

func componentOf(path, platform string) string {
	for _, item := range db.SeedMenus() {
		if item.Path == path && item.Platform == platform {
			return item.Component
		}
	}
	return ""
}

// 断言 5：同一 platform 内 Path 唯一；同一父级内 Name 唯一；admin 全域 Name 唯一。
func TestMenuAlign_NoDuplicates(t *testing.T) {
	items := seedViews()

	byPath := map[string][]seedView{}
	for _, item := range items {
		key := item.Platform + "|" + item.Path
		byPath[key] = append(byPath[key], item)
	}
	for key, group := range byPath {
		if len(group) > 1 {
			t.Errorf("(platform,path) 重复：%s（%d 条）", key, len(group))
		}
	}

	byParentName := map[string]int{}
	byName := map[string][]string{}
	for _, item := range items {
		parent := item.Parent
		if parent == "" {
			parent = item.Platform + ":/"
		}
		byParentName[parent+"|"+item.Name]++
		if item.Platform == "admin" {
			byName[item.Name] = append(byName[item.Name], item.Path)
		}
	}
	for key, n := range byParentName {
		if n > 1 {
			t.Errorf("同一父级下菜单名重复：%s（%d 条）", key, n)
		}
	}
	for name, paths := range byName {
		if len(paths) > 1 {
			sort.Strings(paths)
			t.Errorf("admin 全域菜单名重复：%q → %s", name, strings.Join(paths, ", "))
		}
	}
}

// 断言 6：navMenu 中每个 path 都是 seed 中的启用叶子或目录；且每个一级域的 path 都在 navMenu 里。
func TestMenuAlign_NavMenuMatchesSeed(t *testing.T) {
	root := repoRoot(t)
	src := readFile(t, filepath.Join(root, "frontend-admin", "src", "permission.ts"))
	paths := parseNavMenuPaths(t, src)

	known := map[string]bool{}
	domains := map[string]bool{}
	for _, item := range seedViews() {
		if item.Platform != "admin" || item.Status != "active" {
			continue
		}
		known[item.Path] = true
		if item.Parent == "" {
			domains[item.Path] = true
		}
	}

	// /login 是登录页，不在 menus 表里，属显式例外。
	exceptions := map[string]bool{"/login": true}

	var unknown []string
	seen := map[string]bool{}
	for _, p := range paths {
		if exceptions[p] || known[p] {
			seen[p] = true
			continue
		}
		unknown = append(unknown, p)
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		t.Errorf("navMenu 中存在 seed 未声明的路径（兜底菜单漂移）：\n  %s", strings.Join(unknown, "\n  "))
	}

	var missingDomains []string
	for domain := range domains {
		if !seen[domain] {
			missingDomains = append(missingDomains, domain)
		}
	}
	if len(missingDomains) > 0 {
		sort.Strings(missingDomains)
		t.Errorf("以下一级域未出现在 navMenu（后端不可达时侧边栏会缺整块）：\n  %s", strings.Join(missingDomains, "\n  "))
	}
}

// 断言 7：router 中每个 meta.permission 值都必须在 SeedPermissionCodes() 中声明过。
func TestMenuAlign_RouterPermissionsDeclared(t *testing.T) {
	root := repoRoot(t)
	src := readFile(t, filepath.Join(root, "frontend-admin", "src", "router", "index.ts"))
	routes := parseRouter(t, src)

	declared := map[string]bool{}
	for _, code := range db.SeedPermissionCodes() {
		declared[code] = true
	}

	var undeclared []string
	for _, r := range routes {
		if r.Permission == "" {
			continue
		}
		for _, code := range strings.Split(r.Permission, ",") {
			if code == "" || declared[code] {
				continue
			}
			undeclared = append(undeclared, fmt.Sprintf("%s → %s", r.Path, code))
		}
	}
	if len(undeclared) > 0 {
		sort.Strings(undeclared)
		t.Errorf("router 引用了未在 seedPermissions 声明的权限码：\n  %s", strings.Join(undeclared, "\n  "))
	}
}

// 断言 8（doc102 R1 的机械校验）：二级目录只在启用子叶子 ≥ 2 时保留；
// 一级域至少有一个启用子节点（侧边栏要求 currentGroup.children 非空）。
func TestMenuAlign_DirectoryArity(t *testing.T) {
	items := seedViews()

	type dirInfo struct {
		path   string
		parent string
	}
	children := map[string]int{}
	dirs := map[string]dirInfo{}
	for _, item := range items {
		if item.Platform != "admin" || item.Status != "active" {
			continue
		}
		if item.Parent != "" {
			children[item.Parent]++
		}
		if item.Type == "directory" {
			dirs[item.Platform+":"+item.Path] = dirInfo{path: item.Path, parent: item.Parent}
		}
	}

	var flat, childless []string
	for key, info := range dirs {
		if !strings.HasPrefix(key, "admin:") {
			continue
		}
		n := children[key]
		if info.parent == "" {
			// 一级域：至少一个子节点。
			if n == 0 {
				childless = append(childless, info.path)
			}
			continue
		}
		if n < 2 {
			flat = append(flat, fmt.Sprintf("%s（%d 个启用子节点）", info.path, n))
		}
	}
	sort.Strings(flat)
	sort.Strings(childless)
	if len(flat) > 0 {
		t.Errorf("以下二级目录的启用子节点少于 2 个，应压平（R1）：\n  %s", strings.Join(flat, "\n  "))
	}
	if len(childless) > 0 {
		t.Errorf("以下一级域没有启用子节点（侧边栏会是空目录）：\n  %s", strings.Join(childless, "\n  "))
	}
}
