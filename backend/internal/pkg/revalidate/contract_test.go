package revalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRevalidatePortalContract 钉住后端与门户（frontend-site）的内部接口契约
// （批次四 G3）：
//   - 后端请求路径 /api/internal/revalidate 必须对应门户 server/api/internal/revalidate.post.ts；
//   - 鉴权头 X-Internal-Token 与门户读取的头名一致；
//   - 密钥来源 NUXT_INTERNAL_TOKEN 与环境变量名一致。
//
// 门户是 Nuxt 项目、无测试框架（与 menu_align_test 同款取舍），这里做纯文本扫描：
// 契约任何一端漂移（改路径/改头名/改环境变量）本测试直接失败。
func TestRevalidatePortalContract(t *testing.T) {
	root := repoRootRevalidate(t)
	src := readRevalidateFile(t, filepath.Join(root, "frontend-site", "server", "api", "internal", "revalidate.post.ts"))

	for _, want := range []struct {
		name string
		frag string
	}{
		{"接口路径", "'x-internal-token'"},
		{"密钥环境变量", "NUXT_INTERNAL_TOKEN"},
	} {
		if !strings.Contains(src, want.frag) {
			t.Errorf("门户 revalidate 契约缺失 %s：需要 %q（后端 http.go 与之一致）", want.name, want.frag)
		}
	}
}

// repoRootRevalidate 与 db 包 menu_align_test 的 repoRoot 同款：找到同时含
// backend/go.mod 与 frontend-site/nuxt.config.ts 的那一层。
func repoRootRevalidate(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if fileExistsRevalidate(filepath.Join(dir, "backend", "go.mod")) &&
			fileExistsRevalidate(filepath.Join(dir, "frontend-site", "nuxt.config.ts")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("找不到仓库根（需同时含 backend/go.mod 与 frontend-site/nuxt.config.ts）")
		}
		dir = parent
	}
}

func readRevalidateFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s: %v", path, err)
	}
	return string(data)
}

func fileExistsRevalidate(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
