package db_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 前端「setup 期暂时性死区（TDZ）」门禁。
//
// 背景（实测故障）：新建产品页整个白屏。根因是 <script setup> 里
// `watch(() => form.price, ...)` 被放在 `const form = reactive({...})` **之前** ——
// watch 会在 setup 执行期立刻求值一次 getter（注册依赖），此时 form 还在 TDZ 里，
// 抛 ReferenceError，setup 中断，页面渲染为空白。
//
// 为什么既有工具发现不了：
//   - vue-tsc / tsc 只做类型检查，闭包内对后声明 const 的访问不报错；
//   - vite build 只做语法与模块打包，不做运行时求值；
//   - ESLint 默认规则集里没有 no-use-before-define 的 TDZ 语义检查。
//   结果：编译全绿，运行时白屏。
//
// 本门禁静态复现该判定：对每个 .vue 的 <script setup>，
// 取所有顶层 `watch(<首个参数>, ...)` 的求值源表达式（watch 的唯一"立即求值"部位），
// 断言其中引用的顶层 const/let 声明位置不晚于该 watch。
// computed 的 getter 是惰性的、不在此列（声明顺序对它是安全的）。

// topLevelDeclRe 顶层声明（行首无缩进，排除嵌套作用域）。
var topLevelDeclRe = regexp.MustCompile(`^(?:const|let|var)\s+([A-Za-z_$][\w$]*)`)

// watchCallRe 匹配 watch( 调用起点。
var watchCallRe = regexp.MustCompile(`\bwatch\s*\(`)

// TestFrontendSetupTDZ 检查前端组件 setup 期无"先使用后声明"。
func TestFrontendSetupTDZ(t *testing.T) {
	root := repoRoot(t)
	srcRoot := filepath.Join(root, "frontend-admin", "src")

	var files []string
	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// node_modules 不会出现在 src 下，但 dist 可能；跳过构建产物
			if d.Name() == "dist" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".vue") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历前端组件失败: %v", err)
	}
	if len(files) == 0 {
		t.Skip("未找到 .vue 组件，跳过 setup TDZ 门禁")
	}

	var violations []string
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", file, err)
		}
		for _, v := range findSetupTDZ(string(raw)) {
			rel, _ := filepath.Rel(root, file)
			violations = append(violations, rel+": "+v)
		}
	}
	if len(violations) > 0 {
		t.Errorf("setup 期存在先使用后声明（会导致页面白屏）：\n  %s\n\n"+
			"watch 的求值源会在注册时立刻取值，引用的 const/let 必须先声明；"+
			"把该 watch 移到被引用变量声明之后。", strings.Join(violations, "\n  "))
	}
}

// findSetupTDZ 返回某组件 setup 内的 TDZ 违规描述（无则空）。
func findSetupTDZ(src string) []string {
	const openTag = "<script setup"
	start := strings.Index(src, openTag)
	if start < 0 {
		return nil
	}
	bodyStart := strings.Index(src[start:], ">")
	if bodyStart < 0 {
		return nil
	}
	bodyStart += start + 1
	endRel := strings.Index(src[bodyStart:], "</script>")
	if endRel < 0 {
		return nil
	}
	body := src[bodyStart : bodyStart+endRel]
	lines := strings.Split(body, "\n")

	// 收集顶层声明：变量名 → 行号（1-based）
	declLine := map[string]int{}
	for i, ln := range lines {
		if m := topLevelDeclRe.FindStringSubmatch(ln); m != nil {
			if _, seen := declLine[m[1]]; !seen {
				declLine[m[1]] = i + 1
			}
		}
	}

	var out []string
	for i, ln := range lines {
		loc := watchCallRe.FindStringIndex(ln)
		if loc == nil {
			continue
		}
		// 括号配对取出整个 watch(...) 调用文本
		call, ok := balancedCall(lines, i, loc[1]-1)
		if !ok {
			continue
		}
		firstArg := firstArgument(call)
		if strings.TrimSpace(firstArg) == "" {
			continue
		}
		for name, dline := range declLine {
			if dline <= i+1 {
				continue
			}
			if regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(firstArg) {
				out = append(out, "第 "+itoa(i+1)+" 行的 watch 读取了第 "+itoa(dline)+
					" 行才声明的「"+name+"」")
			}
		}
	}
	return out
}

// balancedCall 从 lines[startLine] 的 openIdx（'(' 的下标）取到配对右括号，返回整段文本。
func balancedCall(lines []string, startLine, openIdx int) (string, bool) {
	depth := 0
	var sb strings.Builder
	for i := startLine; i < len(lines); i++ {
		seg := lines[i]
		from := 0
		if i == startLine {
			from = openIdx
		}
		for k := from; k < len(seg); k++ {
			switch seg[k] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					sb.WriteString(seg[from : k+1])
					return sb.String(), true
				}
			}
		}
		sb.WriteString(seg[from:])
		sb.WriteString("\n")
	}
	return "", false
}

// firstArgument 取调用文本里第一个顶层逗号之前的部分（跳过字符串与括号）。
func firstArgument(call string) string {
	open := strings.Index(call, "(")
	if open < 0 {
		return ""
	}
	depth := 1
	inStr := byte(0)
	for i := open + 1; i < len(call); i++ {
		c := call[i]
		switch {
		case inStr != 0:
			if c == inStr && call[i-1] != '\\' {
				inStr = 0
			}
		case c == '\'' || c == '"' || c == '`':
			inStr = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth == 0 {
				return call[open+1 : i]
			}
		case c == ',' && depth == 1:
			return call[open+1 : i]
		}
	}
	return call[open+1:]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
