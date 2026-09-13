// Package passwordpolicy 集中密码强度校验（doc91 §5.5）。
//
// 现状 6 个密码配置开关（password_min_length / password_require_*）在后台可见但
// 0 处读取；本包把它们收敛为一处，供注册 / 改密 / 重置三处共用。
//
// 语义边界：只作用于「设置新密码」时，不改存量密码（否则存量用户无法登录）。
package passwordpolicy

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// 配置键。
const (
	KeyMinLength     = "password_min_length"
	KeyRequireUpper  = "password_require_uppercase"
	KeyRequireLower  = "password_require_lowercase"
	KeyRequireDigit  = "password_require_digit"
	KeyRequireSymbol = "password_require_special"
	KeyMaxLength     = "password_max_length"
)

// 默认值：最小 6 位，不做字符种类要求（与现状行为一致，避免升级即失败）。
const (
	defaultMinLength = 6
	defaultMaxLength = 64
)

// Reader 配置读取函数（由 service 层的开关读取器注入，避免本包依赖配置仓储）。
type Reader func(ctx context.Context, key string) string

// Policy 密码强度策略。
type Policy struct {
	read Reader
}

// New 创建策略校验器。
func New(read Reader) *Policy { return &Policy{read: read} }

// Default 返回不读配置的默认策略（配置缺失/读取失败时的兜底）。
func Default() *Policy { return &Policy{} }

func (p *Policy) str(ctx context.Context, key string) string {
	if p == nil || p.read == nil {
		return ""
	}
	return strings.TrimSpace(p.read(ctx, key))
}

func (p *Policy) boolean(ctx context.Context, key string) bool {
	switch strings.ToLower(p.str(ctx, key)) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

func (p *Policy) integer(ctx context.Context, key string, def int) int {
	v := p.str(ctx, key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// MinLength 返回最小长度。
func (p *Policy) MinLength(ctx context.Context) int {
	return p.integer(ctx, KeyMinLength, defaultMinLength)
}

// MaxLength 返回最大长度。
func (p *Policy) MaxLength(ctx context.Context) int {
	return p.integer(ctx, KeyMaxLength, defaultMaxLength)
}

// Validate 校验新密码；不符合返回面向用户的中文错误。
func (p *Policy) Validate(ctx context.Context, password string) error {
	minLen := p.MinLength(ctx)
	maxLen := p.MaxLength(ctx)
	runes := []rune(password)
	if len(runes) < minLen {
		return fmt.Errorf("密码长度不能少于 %d 位", minLen)
	}
	if len(runes) > maxLen {
		return fmt.Errorf("密码长度不能超过 %d 位", maxLen)
	}
	if p.boolean(ctx, KeyRequireUpper) && !hasFunc(runes, unicode.IsUpper) {
		return fmt.Errorf("密码必须包含大写字母")
	}
	if p.boolean(ctx, KeyRequireLower) && !hasFunc(runes, unicode.IsLower) {
		return fmt.Errorf("密码必须包含小写字母")
	}
	if p.boolean(ctx, KeyRequireDigit) && !hasFunc(runes, unicode.IsDigit) {
		return fmt.Errorf("密码必须包含数字")
	}
	if p.boolean(ctx, KeyRequireSymbol) && !hasFunc(runes, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r)
	}) {
		return fmt.Errorf("密码必须包含特殊字符")
	}
	return nil
}

func hasFunc(rs []rune, f func(rune) bool) bool {
	for _, r := range rs {
		if f(r) {
			return true
		}
	}
	return false
}
