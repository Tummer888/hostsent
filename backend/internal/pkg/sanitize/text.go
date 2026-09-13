package sanitize

import "strings"

// stripTags 去掉 HTML 标签。不追求完整 HTML 解析（净化已在第一步完成），
// 只处理两类情况：
//  1. 真实标签 <...>；
//  2. 标签被剥离后残留的实体（&nbsp; &amp; 等）—— 一并解成可读字符，
//     否则摘要里会出现 `&amp;` 这种给机器看的写法。
func stripTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return unescapeEntities(b.String())
}

// unescapeEntities 还原正文里最常见的几个 HTML 实体。
// 少量的白名单式替换，目的是让摘要读起来是「A & B」而不是「A &amp; B」。
//
// 安全性前提：PlainText 的产出**只作为纯文本使用**（列表摘要、搜索索引），
// 消费端再经文本节点渲染时会被二次转义，所以这里把实体解回字符是安全的。
// 绝不把 PlainText 的产出当 HTML 再插回页面 —— 那就等于用摘要绕过了一遍净化。
func unescapeEntities(s string) string {
	replacer := strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
	)
	return replacer.Replace(s)
}

// collapseSpaces 把连续空白（含换行/制表）折叠成单个空格并去除首尾空白。
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
