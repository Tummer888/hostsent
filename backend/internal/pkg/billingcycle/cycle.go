// Package billingcycle 定义平台统一的计费周期口径（周期价格矩阵，doc25 §2）。
//
// 背景：平台侧历史上用 products.price_model（fixed/hourly/monthly）同时表达
// "计费模式"与"计费周期"两件事，而上游按周期给价（月付/季付/半年付/年付/两年/三年），
// 周期信息在下单链路上完全丢失。本包把周期收拢成唯一常量表：
//
//   - 平台规范值（Canonical）用于 product_prices.cycle 与 orders.cycle；
//   - 上游别名（Normalize）用于把各适配器/上游返回的多种写法归一。
//
// 周期推进语义（Months/Hours）由续费服务消费，避免 nextExpireAt 里再写一套 switch。
package billingcycle

import (
	"strings"
	"time"
)

// 平台规范周期取值（product_prices.cycle / orders.cycle 的唯一合法集合）。
const (
	Hourly       = "hourly"       // 按小时
	Daily        = "daily"        // 按日
	Monthly      = "monthly"      // 月付
	Quarterly    = "quarterly"    // 季付
	Semiannually = "semiannually" // 半年付
	Annually     = "annually"     // 年付
	Biennially   = "biennially"   // 两年
	Triennially  = "triennially"  // 三年
	Onetime      = "onetime"      // 一次性
)

// Cycle 一个周期的元信息。
type Cycle struct {
	Code     string // 规范值
	Name     string // 中文名（后端错误信息/日志用）
	Months   int    // 周期月数；0 表示非月历周期（小时/日/一次性）
	Hours    int    // 周期小时数（仅 hourly 有值）
	Onetime  bool   // 是否一次性买断（不产到期时间）
	Sortable int    // 展示排序权重
}

// all 规范周期全集，顺序即展示顺序。
var all = []Cycle{
	{Code: Hourly, Name: "按小时", Hours: 1, Sortable: 10},
	{Code: Daily, Name: "按日", Sortable: 20},
	{Code: Monthly, Name: "月付", Months: 1, Sortable: 30},
	{Code: Quarterly, Name: "季付", Months: 3, Sortable: 40},
	{Code: Semiannually, Name: "半年付", Months: 6, Sortable: 50},
	{Code: Annually, Name: "年付", Months: 12, Sortable: 60},
	{Code: Biennially, Name: "两年", Months: 24, Sortable: 70},
	{Code: Triennially, Name: "三年", Months: 36, Sortable: 80},
	{Code: Onetime, Name: "一次性", Onetime: true, Sortable: 90},
}

var (
	byCode map[string]Cycle
	// aliases 上游/历史写法 → 规范值。
	aliases = map[string]string{
		"hour": Hourly, "hourly": Hourly,
		"day": Daily, "daily": Daily,
		"month": Monthly, "monthly": Monthly,
		"quarter": Quarterly, "quarterly": Quarterly,
		"semiannual": Semiannually, "semiannually": Semiannually,
		"half_year": Semiannually, "halfyear": Semiannually,
		"year": Annually, "yearly": Annually, "annual": Annually, "annually": Annually,
		"biennial": Biennially, "biennially": Biennially,
		"triennial": Triennially, "triennially": Triennially,
		"onetime": Onetime, "one-time": Onetime, "one_time": Onetime, "fixed": Onetime,
	}
)

func init() {
	byCode = make(map[string]Cycle, len(all))
	for _, c := range all {
		byCode[c.Code] = c
	}
}

// All 返回全部规范周期（按展示顺序）。
func All() []Cycle {
	out := make([]Cycle, len(all))
	copy(out, all)
	return out
}

// AllCodes 返回全部规范周期码。
func AllCodes() []string {
	out := make([]string, 0, len(all))
	for _, c := range all {
		out = append(out, c.Code)
	}
	return out
}

// Normalize 把任意写法（上游别名 / 存量 price_model）归一为规范周期；无法识别返回空串。
func Normalize(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return ""
	}
	if code, ok := aliases[key]; ok {
		return code
	}
	// 已规范但大小写不同（如 "Monthly"）。
	if _, ok := byCode[key]; ok {
		return key
	}
	return ""
}

// NormalizeOrDefault 归一失败时回落 monthly：下单/续费链路的兼容入口，
// 保证存量数据（price_model=monthly 等）行为不变。
func NormalizeOrDefault(raw string) string {
	if code := Normalize(raw); code != "" {
		return code
	}
	return Monthly
}

// Lookup 按规范值取元信息。
func Lookup(code string) (Cycle, bool) {
	c, ok := byCode[code]
	return c, ok
}

// Name 返回中文名；未知返回原值。
func Name(code string) string {
	if c, ok := byCode[code]; ok {
		return c.Name
	}
	return code
}

// IsValid 判断是否为合法规范周期。
func IsValid(code string) bool {
	_, ok := byCode[code]
	return ok
}

// NormalizeList 归一化一组周期并去重，保持规范展示顺序；忽略无法识别的项。
func NormalizeList(raw []string) []string {
	seen := map[string]struct{}{}
	for _, r := range raw {
		if code := Normalize(r); code != "" {
			seen[code] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for _, c := range all {
		if _, ok := seen[c.Code]; ok {
			out = append(out, c.Code)
		}
	}
	return out
}

// Advance 按周期推进时间：base 起算 periodCount 个周期。
//
//	hourly → +N 小时；daily → +N 天；月历周期按 AddDate(月数) 推进；
//	onetime 与未知周期不推进（返回 base）。
//
// 替换续费服务里只认 4 档、其余默认按月的旧实现。
func Advance(base time.Time, code string, periodCount int) time.Time {
	if periodCount <= 0 {
		periodCount = 1
	}
	c, ok := byCode[Normalize(code)]
	if !ok {
		// 未知周期回落按月，保持既有兜底语义。
		return base.AddDate(0, periodCount, 0)
	}
	switch {
	case c.Hours > 0:
		return base.Add(time.Duration(c.Hours*periodCount) * time.Hour)
	case c.Months > 0:
		return base.AddDate(0, c.Months*periodCount, 0)
	case c.Code == Daily:
		return base.AddDate(0, 0, periodCount)
	default: // onetime 等：不产到期时间
		return base
	}
}

// Multiplier 返回相对月付的价格倍数，用于无周期价时的兜底换算（如按年: 12）。
// 非月历周期返回 0（调用方自行处理或拒绝）。
func Multiplier(code string) int {
	if c, ok := byCode[Normalize(code)]; ok {
		return c.Months
	}
	return 0
}
