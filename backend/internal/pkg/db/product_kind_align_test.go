package db_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	pkgmodel "hostsent/backend/internal/pkg/model"
)

// 产品类型两处对齐门禁。
//
// 产品类型（cloud_host / cdn / physical / …）有两份取值来源：
//  ① 后端登记表   backend/internal/pkg/model/product_kind.go 的 productKindRegistry
//     —— 实例导航（GET /instances/types）、列表筛选、开通落库的唯一权威；
//  ② 前端选项     frontend-admin/src/pages/product/constants.ts 的 productTypeOptions
//     —— 商品表单让运营选的那一列。
//
// 两处漂移的后果是"看得见但点不到"：运营在前端选了一个后端不认的类型，
// 商品能存下去，但开通出来的实例 instance_type 会被归一成 other，
// 于是它在「云主机/CDN/…」任何标签下都找不到，只能去「其它」里翻。
//
// 因此本门禁要求两边**完全一致**（集合相等，不只是包含）：多一个或少一个都报错。
// 新增产品线时同步改两处即可通过。

// productKindFrontendOptions 解析 constants.ts 里 productTypeOptions 的 value 列表。
func productKindFrontendOptions(t *testing.T, root string) []string {
	t.Helper()
	src := readFile(t, filepath.Join(root, "frontend-admin", "src", "pages", "product", "constants.ts"))
	// 只截取 productTypeOptions 数组本体，避免把文件里其它 value 字段一起吃进来。
	start := strings.Index(src, "export const productTypeOptions")
	if start < 0 {
		t.Fatal("constants.ts 里找不到 productTypeOptions（门禁无法校验，请确认是否被重命名）")
	}
	rest := src[start:]
	end := strings.Index(rest, "]")
	if end < 0 {
		t.Fatal("productTypeOptions 数组未闭合")
	}
	body := rest[:end]

	re := regexp.MustCompile(`value:\s*'([^']*)'`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		if m[1] != "" {
			out = append(out, m[1])
		}
	}
	return out
}

// TestProductKindAlign_FrontendOptionsMatchRegistry 前端选项与后端登记表逐一对应。
func TestProductKindAlign_FrontendOptionsMatchRegistry(t *testing.T) {
	root := repoRoot(t)
	frontend := productKindFrontendOptions(t, root)

	registry := map[string]string{} // value → label
	for _, kind := range pkgmodel.ProductKinds() {
		registry[kind.Value] = kind.Label
	}

	var onlyBackend, onlyFrontend []string
	for _, v := range frontend {
		if _, ok := registry[v]; !ok {
			onlyFrontend = append(onlyFrontend, v)
		}
	}
	seen := map[string]bool{}
	for v := range registry {
		if seen[v] {
			continue
		}
		seen[v] = true
		found := false
		for _, fv := range frontend {
			if fv == v {
				found = true
				break
			}
		}
		if !found {
			onlyBackend = append(onlyBackend, v)
		}
	}

	if len(onlyFrontend) > 0 {
		sort.Strings(onlyFrontend)
		t.Errorf("前端商品表单声明了后端未登记的产品类型（商品能存但实例会归 other，导航里找不到）：\n  %s\n"+
			"  修法：在 pkg/model/product_kind.go 的 productKindRegistry 里补上，或从前端移除。",
			strings.Join(onlyFrontend, "\n  "))
	}
	if len(onlyBackend) > 0 {
		sort.Strings(onlyBackend)
		t.Errorf("后端已登记但前端商品表单没有的产品类型（该产品线无法建商品）：\n  %s",
			strings.Join(onlyBackend, "\n  "))
	}
}

// TestProductKindAlign_RegistryInvariants 登记表自身的不变量。
func TestProductKindAlign_RegistryInvariants(t *testing.T) {
	kinds := pkgmodel.ProductKinds()
	if len(kinds) == 0 {
		t.Fatal("产品类型登记表为空")
	}

	seenValue := map[string]bool{}
	seenSort := map[int]string{}
	for _, kind := range kinds {
		if strings.TrimSpace(kind.Value) == "" {
			t.Error("存在空的产品类型值")
		}
		if strings.TrimSpace(kind.Label) == "" {
			t.Errorf("产品类型 %q 缺中文展示名（导航标签会显示空白）", kind.Value)
		}
		if seenValue[kind.Value] {
			t.Errorf("产品类型值重复：%q", kind.Value)
		}
		seenValue[kind.Value] = true
		if prev, ok := seenSort[kind.SortOrder]; ok {
			t.Errorf("排序值重复：%d（%q 与 %q）", kind.SortOrder, prev, kind.Value)
		}
		seenSort[kind.SortOrder] = kind.Value
	}

	// other 必须登记：列表筛选与开通落库都靠它兜住未分类行。
	if !pkgmodel.IsKnownProductKind(pkgmodel.ProductKindOther) {
		t.Error("other 必须是已登记类型（未分类实例的兜底档，缺了会让这些实例从界面消失）")
	}
	// 归一化兜底：空值与未知值都必须落到 other，而不是原样返回或置空。
	if got := pkgmodel.NormalizeProductKind(""); got != pkgmodel.ProductKindOther {
		t.Errorf("NormalizeProductKind(\"\") = %q, want %q", got, pkgmodel.ProductKindOther)
	}
	if got := pkgmodel.NormalizeProductKind("no_such_kind"); got != pkgmodel.ProductKindOther {
		t.Errorf("NormalizeProductKind(未知值) = %q, want %q", got, pkgmodel.ProductKindOther)
	}
	// 已登记值必须原样返回（不能被兜底改写）。
	if got := pkgmodel.NormalizeProductKind(pkgmodel.ProductKindCDN); got != pkgmodel.ProductKindCDN {
		t.Errorf("NormalizeProductKind(cdn) = %q, want cdn（已登记值不得被改写）", got)
	}
}

// TestProductKindAlign_InstanceTypeColumnMigrated 迁移文件必须真实定义该列与回填，
// 且 db.go 里有等价的 Go 回填（迁移目录不被 runner 执行，两者互为冗余）。
func TestProductKindAlign_InstanceTypeColumnMigrated(t *testing.T) {
	root := repoRoot(t)

	migration := readFile(t, filepath.Join(root, "backend", "migrations", "108_instance_product_type.sql"))
	for _, want := range []string{
		"instance_type",
		"ADD COLUMN IF NOT EXISTS",
		"UPDATE instances",
		"/instances/inventory", // 下线重复页的菜单行
	} {
		if !strings.Contains(migration, want) {
			t.Errorf("迁移 108 缺少 %q（列/回填/菜单下线的任一步缺失都会让改动不完整）", want)
		}
	}

	dbSrc := readFile(t, filepath.Join(root, "backend", "internal", "pkg", "db", "db.go"))
	if !strings.Contains(dbSrc, "func backfillInstanceType") {
		t.Error("db.go 缺少 backfillInstanceType（迁移目录不被 runner 执行，存量回填必须同时有 Go 实现）")
	}
	if !strings.Contains(dbSrc, "backfillInstanceType(db)") {
		t.Error("backfillInstanceType 未被 AutoMigrate 调用（写了不跑等于没写）")
	}

	// 页面文件必须真的删掉：菜单/路由都指向它，留着会变成死代码。
	page := filepath.Join(root, "frontend-admin", "src", "pages", "resource", "instances", "index.vue")
	if _, err := os.Stat(page); err == nil {
		t.Error("云主机实例页（resource/instances/index.vue）应已删除，但文件仍存在")
	}
}
