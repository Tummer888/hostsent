// Package sanitize 提供富文本正文的服务端净化（XSS 防线）。
//
// 为什么必须在服务端做（doc100 §6.1）：
//  1. 前端 DOMPurify 挡不住绕过 UI 直接调 API 的请求 —— 攻击者不需要经过我们的编辑器；
//  2. 门户（frontend-site）按 doc80 R10 不引入 TDesign，也不应再引入一套前端净化链路；
//  3. 净化一次落在库里就是安全数据，所有消费端（门户 / 用户中心 / 将来的 App / 邮件）自动安全。
//
// 唯一例外：通知模块的邮件正文（notification/service/render.go）按「管理员可编辑的完整
// HTML 邮件模板」处理，**有意不做转义也不走本包** —— 净化会让现有模板失效。不要顺手去改它。
package sanitize

import "github.com/microcosm-cc/bluemonday"

// richPolicy 富文本正文白名单策略。
//
// 基线取 bluemonday.UGCPolicy()：它已剔除 script/style/iframe/form、所有 on* 事件属性、
// 以及 javascript:/data: 等危险协议。下面只做「加法」——放行编辑器确实会产出的结构标签。
var richPolicy = newRichPolicy()

func newRichPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	// 语义结构：标题、表格、分隔线（UGC 基线不含 table 与 h4+）。
	p.AllowElements("h2", "h3", "h4", "h5", "h6", "hr", "figure", "figcaption")
	p.AllowElements("table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col")

	// 属性收敛：只保留渲染必需的。
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	p.AllowAttrs("id").OnElements("h2", "h3", "h4", "h5", "h6") // 帮助文档目录锚点
	p.AllowAttrs("class").Globally()                            // 代码块/引用等样式钩子，值仍受 bluemonday 校验
	p.AllowAttrs("alt", "title").OnElements("img")
	p.AllowAttrs("title").OnElements("a")

	// 链接安全：外链一律补 rel，避免 window.opener 与 Referer 泄漏。
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(false)

	return p
}

// HTML 净化富文本正文。空串原样返回（不产生 <p></p> 之类噪声）。
func HTML(dirty string) string {
	if dirty == "" {
		return ""
	}
	return richPolicy.Sanitize(dirty)
}

// PlainText 剥离全部标签取纯文本，用于列表摘要与搜索索引。
// 连续空白折叠为单个空格，避免 HTML 换行在摘要里留下大段空白。
func PlainText(html string) string {
	if html == "" {
		return ""
	}
	return collapseSpaces(stripTags(html))
}

// Excerpt 取纯文本摘要，超长截断并追加省略号。
func Excerpt(html string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	plain := []rune(PlainText(html))
	if len(plain) <= maxLen {
		return string(plain)
	}
	return string(plain[:maxLen]) + "…"
}
