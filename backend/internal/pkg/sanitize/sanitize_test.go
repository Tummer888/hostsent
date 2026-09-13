package sanitize

import (
	"strings"
	"testing"
)

// TestHTMLStripsXSS 净化必须挡住三类最常见的存储型 XSS 注入。
func TestHTMLStripsXSS(t *testing.T) {
	cases := []struct {
		name  string
		dirty string
		gone  []string
		kept  []string
	}{
		{
			name:  "script 标签",
			dirty: `<p>正常</p><script>alert(1)</script>`,
			gone:  []string{"<script", "alert(1)"},
			kept:  []string{"<p>正常</p>"},
		},
		{
			name:  "事件属性",
			dirty: `<img src="x" onerror="alert(1)">`,
			gone:  []string{"onerror", "alert(1)"},
			kept:  []string{"<img"},
		},
		{
			name:  "javascript 协议",
			dirty: `<a href="javascript:alert(1)">点我</a>`,
			gone:  []string{"javascript:"},
			kept:  []string{"点我"},
		},
		{
			name:  "iframe 与 style 标签",
			dirty: `<iframe src="//evil"></iframe><style>body{display:none}</style>`,
			gone:  []string{"<iframe", "<style"},
			kept:  []string{},
		},
		{
			name:  "svg onload",
			dirty: `<svg onload="alert(1)"></svg>`,
			gone:  []string{"onload", "alert(1)"},
			kept:  []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HTML(tc.dirty)
			for _, bad := range tc.gone {
				if strings.Contains(got, bad) {
					t.Fatalf("危险片段未剔除 %q: %s", bad, got)
				}
			}
			for _, want := range tc.kept {
				if !strings.Contains(got, want) {
					t.Fatalf("正常内容被误删 %q: %s", want, got)
				}
			}
		})
	}
}

// TestHTMLKeepsEditorOutput 编辑器真正会产出的结构必须保留，否则正文会被净化毁掉。
func TestHTMLKeepsEditorOutput(t *testing.T) {
	dirty := `<h2 id="install">安装</h2>` +
		`<p>请看 <strong>加粗</strong> 与 <em>斜体</em>。</p>` +
		`<ul><li>一</li><li>二</li></ul>` +
		`<ol><li>甲</li></ol>` +
		`<blockquote>引用</blockquote>` +
		`<pre><code>go build ./...</code></pre>` +
		`<table><thead><tr><th>列</th></tr></thead><tbody><tr><td colspan="1">值</td></tr></tbody></table>` +
		`<hr>` +
		`<a href="https://example.com/doc">外链</a>`

	got := HTML(dirty)
	for _, want := range []string{
		`id="install"`, "<strong>", "<em>", "<ul>", "<ol>", "<blockquote>",
		"<pre>", "<table>", "<th>", "colspan", "<hr", "https://example.com/doc",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("编辑器结构被误删 %q: %s", want, got)
		}
	}
}

// TestHTMLAddsLinkRel 外链必须补 nofollow/noopener，防止 window.opener 与权重外泄。
func TestHTMLAddsLinkRel(t *testing.T) {
	got := HTML(`<a href="https://external.example/x">x</a>`)
	if !strings.Contains(got, "nofollow") {
		t.Fatalf("外链缺少 rel=nofollow: %s", got)
	}
	if !strings.Contains(got, "noopener") && !strings.Contains(got, "noreferrer") {
		t.Fatalf("外链缺少 noopener/noreferrer: %s", got)
	}
}

// TestHTMLEmpty 空输入不产生噪声标签。
func TestHTMLEmpty(t *testing.T) {
	if got := HTML(""); got != "" {
		t.Fatalf("空串应原样返回，got %q", got)
	}
}

// TestPlainTextAndExcerpt 摘要取纯文本并折叠空白，且不把实体留给读者。
func TestPlainTextAndExcerpt(t *testing.T) {
	got := PlainText("<p>你好&nbsp;&amp; 欢迎</p>\n<p>第二段</p>")
	want := "你好 & 欢迎 第二段"
	if got != want {
		t.Fatalf("PlainText = %q, want %q", got, want)
	}

	long := Excerpt("<p>"+strings.Repeat("云", 30)+"</p>", 10)
	if !strings.HasPrefix(long, strings.Repeat("云", 10)) || !strings.HasSuffix(long, "…") {
		t.Fatalf("Excerpt 截断不正确: %q", long)
	}

	if Excerpt("<p>短</p>", 10) != "短" {
		t.Fatalf("未超长不应追加省略号")
	}
	if PlainText("") != "" || Excerpt("", 10) != "" {
		t.Fatalf("空输入应返回空串")
	}
}
