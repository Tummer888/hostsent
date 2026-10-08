package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/user/security/model"
	"hostsent/backend/internal/modules/admin/user/security/repository"
	"hostsent/backend/internal/pkg/security"
)

// 规则编码（落 risk_events.rule_code，是运营在页面上看到的「命中规则」）。
//
// 全部用大写常量而不是散落的字符串字面量：规则编码是事件聚合的键
// （同一主体同一规则的连续命中会合并成一条事件累加 occur_count），
// 拼错一个字母就会把本该聚合的事件拆成两条，页面上看不出异常。
const (
	// RuleLoginFailThreshold 账号在窗口内连续登录失败超阈值。
	RuleLoginFailThreshold = "LOGIN_FAIL_THRESHOLD"
	// RuleDeviceFingerprintChanged 登录设备指纹与历史常用设备不一致。
	RuleDeviceFingerprintChanged = "DEVICE_FINGERPRINT_CHANGED"
	// RuleNewIPLogin 登录 IP 不在该账号历史成功登录 IP 里。
	RuleNewIPLogin = "NEW_IP_LOGIN"
	// RuleIPMultiAccountFail 同一 IP 在窗口内对多个不同账号连续失败（撞库）。
	RuleIPMultiAccountFail = "IP_MULTI_ACCOUNT_FAIL"
	// RuleLoginBurst 同一 IP 在窗口内登录请求频率异常。
	RuleLoginBurst = "LOGIN_BURST"
)

// 风险类型（落 risk_events.risk_type，前端筛选与展示枚举）。
const (
	RiskTypeBruteForce    = "brute_force"
	RiskTypeSuspiciousIP  = "suspicious_ip"
	RiskTypeDeviceChange  = "device_change"
	RiskTypeHighFrequency = "high_frequency"
)

// 风险规则阈值（可由 system_configs 覆盖，见 RiskRuleConfig 与 risk_switches.go）。
//
// 默认值取「宁可漏报、不要误报」：默认开启的规则只有 device_change（改设备是
// 用户自己能感知的正常行为，误报代价只是运营看一眼），而 IP 类规则在运营商 NAT
// 出口下天然会大量命中，默认阈值放宽。
type RiskRuleConfig struct {
	// Enabled 风险规则引擎总开关（risk_rules_enabled，默认 true）。
	//
	// 默认开：这条链路只**写入事件**、不阻断任何登录（阻断只发生在黑名单命中时，
	// 而黑名单必须由运营显式添加），因此它不会改变任何既有行为，只是让
	// 「异常行为监控」页从空壳变成真有数据。
	Enabled bool
	// NotifyOnCritical 严重级事件是否同时写一条站内通知给管理员（默认 false）。
	NotifyOnCritical bool
	// FailThreshold 连续失败次数阈值（risk_fail_threshold，默认 5）。
	FailThreshold int
	// FailWindowMinutes 连续失败统计窗口（默认 10 分钟）。
	FailWindowMinutes int
	// MultiAccountFailAccounts 同 IP 失败的不同账号数阈值（默认 8）。
	MultiAccountFailAccounts int
	// MultiAccountFailWindowMinutes 撞库统计窗口（默认 10 分钟）。
	MultiAccountFailWindowMinutes int
	// LoginBurstCount 同 IP 登录尝试次数阈值（默认 30）。
	LoginBurstCount int
	// LoginBurstWindowMinutes 频率统计窗口（默认 5 分钟）。
	LoginBurstWindowMinutes int
	// DeviceChangeEnabled 是否启用设备变更规则（默认 true）。
	DeviceChangeEnabled bool
	// NewIPEnabled 是否启用新 IP 规则（默认 false）。
	//
	// 默认关是刻意的：移动网络、企业 NAT、CDN 回源都会让「新 IP」常态化，
	// 开着会淹没真正需要看的事件。运营在确认自己的用户分布后可以打开。
	NewIPEnabled bool
}

// DefaultRiskRuleConfig 默认规则配置（与迁移 065 写入的 system_configs 值一致）。
func DefaultRiskRuleConfig() RiskRuleConfig {
	return RiskRuleConfig{
		Enabled:                       true,
		FailThreshold:                 5,
		FailWindowMinutes:             10,
		MultiAccountFailAccounts:      8,
		MultiAccountFailWindowMinutes: 10,
		LoginBurstCount:               30,
		LoginBurstWindowMinutes:       5,
		DeviceChangeEnabled:           true,
		NewIPEnabled:                  false,
	}
}

// RiskConfigReader 读一条 system_configs 原文（键不存在返回 ok=false）。
type RiskConfigReader func(ctx context.Context, key string) (string, bool, error)

// RiskEngine 登录风险规则引擎（doc06 §4.3）。
//
// 职责边界：只**观察**登录，产出风险事件，绝不打断登录。唯一会阻断登录的是
// 黑名单（BlacklistGuard），两者刻意分开 —— 规则误报只该让运营多点一次「忽略」，
// 不该让用户登不进来。
//
// 同类事件的聚合（doc06 §4.3 关键规则 2）在这里做，而不是靠给表加唯一约束：
// 运营需要看到「这条规则最近命中了 5 次」这种信息，所以同一主体同一规则在
// 时间窗内命中要累加 occur_count 而不是新插一行；一旦处置（handled/ignored）
// 就重新开一条，否则新发生的异常会被挂到已结案的事件上、页面上看不到。
type RiskEngine struct {
	repo     repository.RiskRepository
	reader   RiskConfigReader
	logCache *riskConfigCache
}

// NewRiskEngine 创建规则引擎；reader 为 nil 时全部按默认阈值走。
func NewRiskEngine(repo repository.RiskRepository, reader RiskConfigReader) *RiskEngine {
	return &RiskEngine{repo: repo, reader: reader, logCache: newRiskConfigCache()}
}

// RuleConfig 读取当前生效的规则配置（带 30 秒内存缓存）。
func (e *RiskEngine) RuleConfig(ctx context.Context) RiskRuleConfig {
	cfg := DefaultRiskRuleConfig()
	if e == nil || e.reader == nil {
		return cfg
	}
	vals := e.logCache.load(ctx, e.reader)
	cfg.Enabled = boolOr(vals[ConfigRiskRulesEnabled], cfg.Enabled)
	cfg.NotifyOnCritical = boolOr(vals[ConfigRiskNotifyCritical], cfg.NotifyOnCritical)
	cfg.DeviceChangeEnabled = boolOr(vals[ConfigRiskDeviceChangeEnabled], cfg.DeviceChangeEnabled)
	cfg.NewIPEnabled = boolOr(vals[ConfigRiskNewIPEnabled], cfg.NewIPEnabled)
	cfg.FailThreshold = intOr(vals[ConfigRiskFailThreshold], cfg.FailThreshold)
	cfg.FailWindowMinutes = intOr(vals[ConfigRiskFailWindow], cfg.FailWindowMinutes)
	cfg.MultiAccountFailAccounts = intOr(vals[ConfigRiskMultiAccount], cfg.MultiAccountFailAccounts)
	cfg.MultiAccountFailWindowMinutes = intOr(vals[ConfigRiskMultiAccountWindow], cfg.MultiAccountFailWindowMinutes)
	cfg.LoginBurstCount = intOr(vals[ConfigRiskBurstCount], cfg.LoginBurstCount)
	cfg.LoginBurstWindowMinutes = intOr(vals[ConfigRiskBurstWindow], cfg.LoginBurstWindowMinutes)
	return cfg
}

// Observe 用一次登录尝试的信号评估全部规则并落库命中的事件。
//
// 任何一步失败都只记日志、不返回错误：风控是旁路能力，它挂了不能影响登录。
func (e *RiskEngine) Observe(ctx context.Context, sig security.LoginSignal) {
	if e == nil || e.repo == nil {
		return
	}
	cfg := e.RuleConfig(ctx)
	if !cfg.Enabled {
		return
	}
	subject := firstNonEmpty(sig.SubjectType, model.SubjectTypeUser)
	now := time.Now()
	for _, hit := range e.evaluate(ctx, cfg, sig, now) {
		e.record(ctx, subject, sig, hit, now)
	}
}

// ruleHit 一条命中的规则（已判定，待落库）。
type ruleHit struct {
	RuleCode  string
	RiskType  string
	RiskLevel string
	Summary   string
	Detail    map[string]any
}

// observeAt 归一化信号时刻：未显式给定时按「现在」处理。
//
// 历史设备/IP 的查询上界必须用信号里的时刻，而不是观察时的 now ——
// 见 security.LoginSignal.At 的说明：登录日志先落库，本次登录自己写的那行
// 必须被排除，否则「新设备」在写日志的瞬间就变成「常用设备」，规则永不触发。
func observeAt(sig security.LoginSignal, fallback time.Time) time.Time {
	if !sig.At.IsZero() {
		return sig.At
	}
	return fallback
}

// evaluate 逐条评估规则（顺序即优先级，同一次登录可同时命中多条）。
func (e *RiskEngine) evaluate(ctx context.Context, cfg RiskRuleConfig, sig security.LoginSignal, now time.Time) []ruleHit {
	var hits []ruleHit
	subject := firstNonEmpty(sig.SubjectType, model.SubjectTypeUser)
	at := observeAt(sig, now)

	// —— 规则 1：连续登录失败超阈值 ——
	// 只看失败尝试：成功登录不该被这条规则触发。
	if !sig.Success && sig.Username != "" && cfg.FailThreshold > 0 {
		since := now.Add(-time.Duration(cfg.FailWindowMinutes) * time.Minute)
		if n, err := e.repo.CountFailedLogins(ctx, subject, sig.Username, since); err == nil && n >= int64(cfg.FailThreshold) {
			hits = append(hits, ruleHit{
				RuleCode:  RuleLoginFailThreshold,
				RiskType:  RiskTypeBruteForce,
				RiskLevel: model.RiskLevelHigh,
				Summary:   fmt.Sprintf("账号 %s 在 %d 分钟内连续登录失败 %d 次", sig.Username, cfg.FailWindowMinutes, n),
				Detail: map[string]any{
					"fail_count": n, "window_minutes": cfg.FailWindowMinutes,
					"threshold": cfg.FailThreshold, "failure_reason": sig.FailureReason,
				},
			})
		}
	}

	// —— 规则 2：同 IP 撞库（多个不同账号失败）——
	if cfg.MultiAccountFailAccounts > 0 && sig.IP != "" {
		since := now.Add(-time.Duration(cfg.MultiAccountFailWindowMinutes) * time.Minute)
		if n, err := e.repo.CountFailedAccountsByIP(ctx, subject, sig.IP, since); err == nil && n >= int64(cfg.MultiAccountFailAccounts) {
			hits = append(hits, ruleHit{
				RuleCode:  RuleIPMultiAccountFail,
				RiskType:  RiskTypeBruteForce,
				RiskLevel: model.RiskLevelCritical,
				Summary:   fmt.Sprintf("IP %s 在 %d 分钟内对 %d 个不同账号登录失败，疑似撞库", sig.IP, cfg.MultiAccountFailWindowMinutes, n),
				Detail: map[string]any{
					"account_count": n, "window_minutes": cfg.MultiAccountFailWindowMinutes,
					"threshold": cfg.MultiAccountFailAccounts,
				},
			})
		}
	}

	// —— 规则 3：同 IP 登录请求频率异常 ——
	if cfg.LoginBurstCount > 0 && sig.IP != "" {
		since := now.Add(-time.Duration(cfg.LoginBurstWindowMinutes) * time.Minute)
		if n, err := e.repo.CountLoginsByIP(ctx, subject, sig.IP, since); err == nil && n >= int64(cfg.LoginBurstCount) {
			hits = append(hits, ruleHit{
				RuleCode:  RuleLoginBurst,
				RiskType:  RiskTypeHighFrequency,
				RiskLevel: model.RiskLevelMedium,
				Summary:   fmt.Sprintf("IP %s 在 %d 分钟内发起 %d 次登录，频率异常", sig.IP, cfg.LoginBurstWindowMinutes, n),
				Detail: map[string]any{
					"request_count": n, "window_minutes": cfg.LoginBurstWindowMinutes,
					"threshold": cfg.LoginBurstCount,
				},
			})
		}
	}

	// —— 规则 4 / 5：设备变更、新 IP ——
	// 两条都只对**成功登录**判定，且只对已存在且拿得到 ID 的账号判定。
	// 失败尝试里的设备/IP 是攻击者的，把它写进「历史常用」会让攻击者第一次
	// 试探就完成「养熟」，后续真正的异常登录反而不报警；账号不存在（user_id=0）
	// 也没有「常用」可言，所有登录都是新设备，纯噪声。
	if sig.Success && sig.UserID > 0 {
		if cfg.DeviceChangeEnabled && sig.DeviceFingerprint != "" {
			if known, err := e.repo.KnownDevices(ctx, subject, sig.UserID, at); err == nil {
				if len(known) > 0 && !containsString(known, sig.DeviceFingerprint) {
					hits = append(hits, ruleHit{
						RuleCode:  RuleDeviceFingerprintChanged,
						RiskType:  RiskTypeDeviceChange,
						RiskLevel: model.RiskLevelLow,
						Summary:   "登录设备与历史常用设备不一致",
						Detail: map[string]any{
							"new_device":  sig.DeviceFingerprint,
							"known_count": len(known),
							// 只带最近一台旧设备：全量历史设备可能几十个，写进 payload 既没用又撑大表。
							"previous_device": known[0],
						},
					})
				}
			}
		}
		if cfg.NewIPEnabled && sig.IP != "" {
			if known, err := e.repo.KnownLoginIPs(ctx, subject, sig.UserID, at); err == nil {
				if len(known) > 0 && !containsString(known, sig.IP) {
					hits = append(hits, ruleHit{
						RuleCode:  RuleNewIPLogin,
						RiskType:  RiskTypeSuspiciousIP,
						RiskLevel: model.RiskLevelMedium,
						Summary:   fmt.Sprintf("账号 %s 从新 IP 登录", sig.Username),
						Detail: map[string]any{
							"new_ip": sig.IP, "known_count": len(known), "previous_ip": known[0],
						},
					})
				}
			}
		}
	}
	return hits
}

// record 落库一条命中：时间窗内同一主体同一规则已有未处置事件时累加，否则新建。
func (e *RiskEngine) record(ctx context.Context, subject string, sig security.LoginSignal, hit ruleHit, now time.Time) {
	payload, err := json.Marshal(hit.Detail)
	if err != nil {
		payload = []byte("{}")
	}
	// 聚合窗口与统计窗口同量级：运营看到的「发生次数」应该对应规则口径，
	// 而不是「历史上所有命中」。
	aggregateSince := now.Add(-time.Duration(e.RuleConfig(ctx).MultiAccountFailWindowMinutes) * time.Minute)

	existing, err := e.repo.FindOpenEvent(ctx, subject, sig.UserID, sig.Username, hit.RuleCode, aggregateSince)
	if err != nil {
		return
	}
	if existing != nil {
		existing.OccurCount++
		existing.LastOccurredAt = now
		existing.DetailPayload = string(payload)
		// 等级只升不降：聚合期间如果又命中了更重的判据（比如单账号失败升级为
		// 多账号撞库），页面上的等级要跟上，不能让运营以为还是小事。
		if riskLevelWeight(hit.RiskLevel) > riskLevelWeight(existing.RiskLevel) {
			existing.RiskLevel = hit.RiskLevel
		}
		_ = e.repo.UpdateEvent(ctx, existing)
		return
	}
	event := &model.RiskEvent{
		RiskType:          hit.RiskType,
		RiskLevel:         hit.RiskLevel,
		UserID:            sig.UserID,
		Username:          sig.Username,
		SubjectType:       subject,
		IP:                sig.IP,
		DeviceFingerprint: sig.DeviceFingerprint,
		RuleCode:          hit.RuleCode,
		Summary:           truncateRunes(hit.Summary, 255),
		DetailPayload:     string(payload),
		OccurCount:        1,
		FirstOccurredAt:   now,
		LastOccurredAt:    now,
		Status:            model.RiskStatusPending,
	}
	_ = e.repo.CreateEvent(ctx, event)
}

// riskLevelWeight 等级权重（聚合时「只升不降」的比较基准）。
func riskLevelWeight(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case model.RiskLevelCritical:
		return 4
	case model.RiskLevelHigh:
		return 3
	case model.RiskLevelMedium:
		return 2
	case model.RiskLevelLow:
		return 1
	default:
		return 0
	}
}

// BlacklistGuard 登录前置的黑名单校验（doc06 §4.4）。
//
// 与风险引擎分开的两个理由：
//   - 它会**阻断**登录，必须逐条可解释（命中了哪一类、哪个值），
//     而风险引擎只写事件、永远放行；
//   - 它的成本必须极低（登录热路径上每次 4~5 次带索引的等值查询），
//     而风险引擎有多次聚合统计，本身就不该挂在每一次尝试上。
//
// 留痕不在这里做：命中的这次尝试会被登录链路自己记进 login_logs
// （failure_reason = 命中黑名单(...)），页面上的「命中记录」抽屉正是按
// 黑名单的 type/target_value 关联登录日志得来的。让守卫再写一条会变成两条。
type BlacklistGuard struct {
	repo repository.RiskRepository
}

// NewBlacklistGuard 创建黑名单守卫。
func NewBlacklistGuard(repo repository.RiskRepository) *BlacklistGuard {
	return &BlacklistGuard{repo: repo}
}

// Check 按 IP → 账号 → 设备 → 手机/邮箱 的顺序校验。
//
// 顺序即优先级：IP 是「这个来源」，最该先拦；账号名是「这次尝试用的身份」；
// 设备与手机/邮箱才是「这个人」。IP 命中只影响当前来源，账号/设备命中才是
// 真正把这个人挡在门外。
func (g *BlacklistGuard) Check(ctx context.Context, account, ip, deviceFingerprint string) error {
	if g == nil || g.repo == nil {
		return nil
	}
	candidates := []repository.BlacklistCandidate{
		{Type: security.BlacklistTypeIP, Value: ip},
		{Type: security.BlacklistTypeUser, Value: account},
		{Type: security.BlacklistTypeDevice, Value: deviceFingerprint},
	}
	// 账号名可能本身就是手机号/邮箱（短信、邮箱验证码登录就是这么传的），
	// 因此同值按 phone/email 再查一遍，让运营用「手机号」类型拉黑也能生效。
	if strings.Contains(account, "@") {
		candidates = append(candidates, repository.BlacklistCandidate{Type: security.BlacklistTypeEmail, Value: account})
	} else if isPhoneLike(account) {
		candidates = append(candidates, repository.BlacklistCandidate{Type: security.BlacklistTypePhone, Value: account})
	}
	found, err := g.repo.MatchBlacklist(ctx, candidates)
	if err != nil || found == nil {
		// 查询失败一律放行：风控表异常不该演变成「所有人都登不进来」。
		return nil
	}
	// 命中计数用于运营判断「这条黑名单到底有没有在拦人」；
	// 累加失败不影响本次拒绝（拦截本身已经由上面的判定决定了）。
	_ = g.repo.BumpBlacklistHit(ctx, found.ID)
	return &security.ErrBlacklisted{Scope: found.Type, Reason: found.Reason}
}

// isPhoneLike 粗判是否是手机号形态（11 位数字，允许 +86 前缀）。
func isPhoneLike(v string) bool {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "+86")
	v = strings.TrimPrefix(v, "86")
	if len(v) != 11 {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return v[0] == '1'
}

// containsString 判断切片是否包含给定值。
func containsString(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// —— 配置缓存 ——

// riskConfigCache system_configs 的 30 秒内存缓存。
//
// 登录热路径每条规则都要读阈值，每次都查库会让登录多出十几次查询。
// 30 秒与 captcha 的 Switches 同口径：运营在后台改完阈值，最多半分钟生效。
type riskConfigCache struct {
	vals map[string]string
	at   time.Time
}

func newRiskConfigCache() *riskConfigCache { return &riskConfigCache{} }

func (c *riskConfigCache) load(ctx context.Context, reader RiskConfigReader) map[string]string {
	if time.Since(c.at) < 30*time.Second && c.vals != nil {
		return c.vals
	}
	out := make(map[string]string, len(riskSwitchKeys))
	for _, key := range riskSwitchKeys {
		if v, ok, err := reader(ctx, key); err == nil && ok {
			out[key] = v
		}
	}
	// 读失败（全部为空）时不覆盖旧值，避免一次抖动把阈值打回默认。
	if len(out) == 0 && c.vals != nil {
		return c.vals
	}
	c.vals = out
	c.at = time.Now()
	return out
}

// truncateRunes 按字符（而非字节）截断，避免把多字节汉字切成乱码。
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
