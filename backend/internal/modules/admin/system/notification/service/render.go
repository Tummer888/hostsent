package service

import (
	"context"
	"html"
	"regexp"
	"strings"

	sysconfigrepo "hostsent/backend/internal/modules/admin/system/repository"
)

// varPattern 模板变量占位符：{var_name}。
// 只认小写字母开头的下划线命名，避免把 JSON/HTML 里的花括号误当变量。
var varPattern = regexp.MustCompile(`\{([a-z_][a-z0-9_]*)\}`)

// ExtractVarNames 提取模板正文中的变量名（去重、保持出现顺序）。
func ExtractVarNames(content string) []string {
	matches := varPattern.FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		key := m[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

// RenderTemplate 变量替换：{var} → value。变量缺失时保留 {var} 原文，
// 便于在发送日志里一眼看出「哪个变量没喂值」，而不是静默发出空字符串。
func RenderTemplate(tpl string, vars map[string]string) string {
	if len(vars) == 0 {
		return tpl
	}
	result := tpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

// SmsSegments 估算短信条数：70 字符/条，超 70 按 67 字符/条（长短信每条留 3 位序号）。
// 按 rune 计（中文一条算 1 字符，与国内计费口径一致）。
func SmsSegments(content string) (chars, segments int) {
	chars = len([]rune(content))
	if chars <= 70 {
		if chars == 0 {
			return 0, 0
		}
		return chars, 1
	}
	return chars, (chars + 66) / 67
}

// siteShellConfig 邮件外壳所需的站点信息。
type siteShellConfig struct {
	SiteName  string
	Copyright string
}

// loadSiteShell 从 system_configs 读站点名与版权（不硬编码）。
//
// 键名优先取 doc80 规范点号键（`site.name` / `site.copyright`），回落历史扁平键
// （`site_name` / `site_copyright`）—— 迁移 045 之前扁平键是管理端的唯一入口，
// 迁移执行前的库仍需读得到值。
//
// 读失败或未配置时给出中性兜底，保证邮件仍然可读。
func loadSiteShell(ctx context.Context, repo sysconfigrepo.ConfigRepository) siteShellConfig {
	cfg := siteShellConfig{SiteName: "HostSent", Copyright: ""}
	if repo == nil {
		return cfg
	}
	if v := firstConfigValue(ctx, repo, "site.name", "site_name"); strings.TrimSpace(v) != "" {
		cfg.SiteName = v
	}
	cfg.Copyright = firstConfigValue(ctx, repo, "site.copyright", "site_copyright")
	return cfg
}

// firstConfigValue 按顺序返回第一个非空配置值（原文，不裁剪）；全都没有时返回空串。
func firstConfigValue(ctx context.Context, repo sysconfigrepo.ConfigRepository, keys ...string) string {
	for _, key := range keys {
		if c, err := repo.FindByKey(ctx, key); err == nil && c != nil {
			if strings.TrimSpace(c.ConfigValue) != "" {
				return c.ConfigValue
			}
		}
	}
	return ""
}

// RenderMailShell 给 HTML 邮件正文包一层统一外壳（站点名 + 页脚 + 版权）。
//
// 正文按「完整 HTML 片段」处理，不做转义——内容来自本系统后台模板，
// 只有管理员可编辑；转义会让 HTML 模板失效。页脚里的站点名/版权做转义。
func RenderMailShell(siteName, copyright, body string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="UTF-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1"></head>`)
	b.WriteString(`<body style="margin:0;padding:0;background:#f5f6f8;">`)
	b.WriteString(`<div style="max-width:640px;margin:0 auto;padding:24px 16px;">`)
	b.WriteString(`<div style="background:#ffffff;border-radius:8px;padding:28px 24px;`)
	b.WriteString(`font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;`)
	b.WriteString(`font-size:14px;line-height:1.7;color:#1f2329;">`)
	b.WriteString(body)
	b.WriteString(`</div>`)
	b.WriteString(`<div style="text-align:center;color:#8f959e;font-size:12px;padding:16px 0;">`)
	b.WriteString(html.EscapeString(siteName))
	if strings.TrimSpace(copyright) != "" {
		b.WriteString(` · `)
		b.WriteString(html.EscapeString(copyright))
	}
	b.WriteString(`</div></div></body></html>`)
	return b.String()
}
