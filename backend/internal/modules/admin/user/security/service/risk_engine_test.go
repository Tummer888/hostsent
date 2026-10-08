package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"hostsent/backend/internal/modules/admin/user/security/model"
	"hostsent/backend/internal/modules/admin/user/security/repository"
	"hostsent/backend/internal/pkg/security"
)

// fakeRiskRepo 内存版 RiskRepository，用于在不依赖数据库的情况下验证规则判定逻辑。
//
// 用假实现而不是 live 库：规则引擎的正确性体现在「什么条件触发哪条规则、事件怎么聚合」，
// 这些判定与 SQL 无关；把它们放到 live 用例里会既慢又难定位（一条断言失败时无法区分
// 是规则写错还是查询写错）。SQL 本身另由 live 用例覆盖。
type fakeRiskRepo struct {
	// 统计输入
	failedByAccount map[string]int64
	failedAccounts  map[string]int64
	loginsByIP      map[string]int64
	knownIPs        map[uint64][]string
	knownDevices    map[uint64][]string

	// 事件存储
	events  []*model.RiskEvent
	nextID  uint64
	updated int
	created int

	// 黑名单
	blacklist   []model.Blacklist
	bumpedIDs   []uint64
	matchErr    error
	lastQueried []repository.BlacklistCandidate
}

func newFakeRiskRepo() *fakeRiskRepo {
	return &fakeRiskRepo{
		failedByAccount: map[string]int64{},
		failedAccounts:  map[string]int64{},
		loginsByIP:      map[string]int64{},
		knownIPs:        map[uint64][]string{},
		knownDevices:    map[uint64][]string{},
	}
}

func (f *fakeRiskRepo) CountFailedLogins(_ context.Context, _, username string, _ time.Time) (int64, error) {
	return f.failedByAccount[username], nil
}

func (f *fakeRiskRepo) CountFailedAccountsByIP(_ context.Context, _, ip string, _ time.Time) (int64, error) {
	return f.failedAccounts[ip], nil
}

func (f *fakeRiskRepo) CountLoginsByIP(_ context.Context, _, ip string, _ time.Time) (int64, error) {
	return f.loginsByIP[ip], nil
}

func (f *fakeRiskRepo) KnownLoginIPs(_ context.Context, _ string, userID uint64, _ time.Time) ([]string, error) {
	return f.knownIPs[userID], nil
}

func (f *fakeRiskRepo) KnownDevices(_ context.Context, _ string, userID uint64, _ time.Time) ([]string, error) {
	return f.knownDevices[userID], nil
}

func (f *fakeRiskRepo) FindOpenEvent(_ context.Context, subjectType string, userID uint64, username, ruleCode string, since time.Time) (*model.RiskEvent, error) {
	for i := len(f.events) - 1; i >= 0; i-- {
		e := f.events[i]
		if e.RuleCode != ruleCode || e.UserID != userID || e.Username != username {
			continue
		}
		if e.SubjectType != subjectType || e.Status != model.RiskStatusPending {
			continue
		}
		if e.LastOccurredAt.Before(since) {
			continue
		}
		return e, nil
	}
	return nil, nil
}

func (f *fakeRiskRepo) CreateEvent(_ context.Context, event *model.RiskEvent) error {
	f.nextID++
	event.ID = f.nextID
	f.events = append(f.events, event)
	f.created++
	return nil
}

func (f *fakeRiskRepo) UpdateEvent(_ context.Context, event *model.RiskEvent) error {
	for i := range f.events {
		if f.events[i].ID == event.ID {
			f.events[i] = event
		}
	}
	f.updated++
	return nil
}

func (f *fakeRiskRepo) MatchBlacklist(_ context.Context, candidates []repository.BlacklistCandidate) (*model.Blacklist, error) {
	f.lastQueried = candidates
	if f.matchErr != nil {
		return nil, f.matchErr
	}
	now := time.Now()
	for _, c := range candidates {
		value := strings.TrimSpace(c.Value)
		if value == "" {
			continue
		}
		for i := range f.blacklist {
			item := f.blacklist[i]
			if item.Type != c.Type || item.TargetValue != value {
				continue
			}
			if item.Status != model.BlacklistStatusActive {
				continue
			}
			if item.EffectiveAt.After(now) {
				continue
			}
			if item.ExpiredAt != nil && !item.ExpiredAt.After(now) {
				continue
			}
			return &item, nil
		}
	}
	return nil, nil
}

func (f *fakeRiskRepo) BumpBlacklistHit(_ context.Context, id uint64) error {
	f.bumpedIDs = append(f.bumpedIDs, id)
	return nil
}

// fakeRiskConfig 可控的配置读取器。
func fakeRiskConfig(vals map[string]string) RiskConfigReader {
	return func(_ context.Context, key string) (string, bool, error) {
		v, ok := vals[key]
		return v, ok, nil
	}
}

// newTestEngine 构造一个绕过配置缓存（每次新建 cache）的引擎。
func newTestEngine(repo repository.RiskRepository, vals map[string]string) *RiskEngine {
	return NewRiskEngine(repo, fakeRiskConfig(vals))
}

func userSignal(username string, userID uint64, success bool, reason string) security.LoginSignal {
	return security.LoginSignal{
		Username: username, UserID: userID, IP: "203.0.113.10", DeviceFingerprint: "fp-a",
		SubjectType: security.SubjectTypeUser, LoginType: "password",
		Success: success, FailureReason: reason,
	}
}

// TestRiskEngineFailThresholdCreatesEvent 连续失败达到阈值即产生暴力破解事件。
func TestRiskEngineFailThresholdCreatesEvent(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.failedByAccount["alice"] = 5
	engine := newTestEngine(repo, nil)

	engine.Observe(context.Background(), userSignal("alice", 7, false, "bad_password"))

	if len(repo.events) != 1 {
		t.Fatalf("期望产生 1 条事件，实际 %d", len(repo.events))
	}
	e := repo.events[0]
	if e.RuleCode != RuleLoginFailThreshold || e.RiskType != RiskTypeBruteForce {
		t.Errorf("规则/类型不符: %s / %s", e.RuleCode, e.RiskType)
	}
	if e.RiskLevel != model.RiskLevelHigh {
		t.Errorf("等级 = %q，期望 high", e.RiskLevel)
	}
	if e.SubjectType != model.SubjectTypeUser {
		t.Errorf("主体域 = %q，期望 user", e.SubjectType)
	}
	if e.OccurCount != 1 || e.Status != model.RiskStatusPending {
		t.Errorf("新事件应 occur_count=1 / pending，实际 %d / %s", e.OccurCount, e.Status)
	}
}

// TestRiskEngineThresholdNotReached 未达阈值不产生事件（避免误报）。
func TestRiskEngineThresholdNotReached(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.failedByAccount["bob"] = 4 // 阈值 5
	engine := newTestEngine(repo, nil)

	engine.Observe(context.Background(), userSignal("bob", 8, false, "bad_password"))

	if len(repo.events) != 0 {
		t.Fatalf("未达阈值不应产生事件，实际 %d 条", len(repo.events))
	}
}

// TestRiskEngineAggregatesRepeatedHits 同类事件在窗口内聚合：累加次数而不是刷屏。
//
// doc06 §4.3 关键规则 2「同类重复事件应支持聚合，避免告警风暴」的核心断言。
func TestRiskEngineAggregatesRepeatedHits(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.failedByAccount["carol"] = 6
	engine := newTestEngine(repo, nil)

	engine.Observe(context.Background(), userSignal("carol", 9, false, "bad_password"))
	engine.Observe(context.Background(), userSignal("carol", 9, false, "bad_password"))
	engine.Observe(context.Background(), userSignal("carol", 9, false, "bad_password"))

	if len(repo.events) != 1 {
		t.Fatalf("同类命中应聚合成 1 条事件，实际 %d 条（告警风暴）", len(repo.events))
	}
	if repo.events[0].OccurCount != 3 {
		t.Errorf("occur_count = %d，期望 3", repo.events[0].OccurCount)
	}
	if repo.created != 1 || repo.updated != 2 {
		t.Errorf("写入分布不符：created=%d updated=%d，期望 1/2", repo.created, repo.updated)
	}
}

// TestRiskEngineUpgradesLevelOnAggregate 聚合期间命中更重判据时等级只升不降。
func TestRiskEngineUpgradesLevelOnAggregate(t *testing.T) {
	repo := newFakeRiskRepo()
	// 第一次：单账号失败（high）
	repo.failedByAccount["dave"] = 5
	repo.failedAccounts["203.0.113.10"] = 9 // 已超撞库阈值 8
	engine := newTestEngine(repo, nil)

	engine.Observe(context.Background(), userSignal("dave", 10, false, "bad_password"))

	// 应同时命中「连续失败」与「IP 撞库」两条规则，各自一条事件。
	if len(repo.events) != 2 {
		t.Fatalf("期望 2 条事件（失败阈值 + 撞库），实际 %d", len(repo.events))
	}
	var critical *model.RiskEvent
	for _, e := range repo.events {
		if e.RuleCode == RuleIPMultiAccountFail {
			critical = e
		}
	}
	if critical == nil {
		t.Fatal("未产生撞库事件")
	}
	if critical.RiskLevel != model.RiskLevelCritical {
		t.Errorf("撞库事件等级 = %q，期望 critical", critical.RiskLevel)
	}
}

// TestRiskEngineReopensAfterHandled 事件处置后再命中应新开一条，不挂到已结案事件上。
func TestRiskEngineReopensAfterHandled(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.failedByAccount["erin"] = 5
	engine := newTestEngine(repo, nil)

	engine.Observe(context.Background(), userSignal("erin", 11, false, "bad_password"))
	if len(repo.events) != 1 {
		t.Fatalf("首次应产生 1 条事件")
	}
	// 运营处置掉这条。
	repo.events[0].Status = model.RiskStatusHandled

	engine.Observe(context.Background(), userSignal("erin", 11, false, "bad_password"))

	if len(repo.events) != 2 {
		t.Fatalf("已处置后再次命中应新开一条，实际共 %d 条", len(repo.events))
	}
	if repo.events[1].OccurCount != 1 {
		t.Errorf("新事件 occur_count 应为 1，实际 %d", repo.events[1].OccurCount)
	}
}

// TestRiskEngineDeviceChangeOnlyOnSuccess 设备变更只在成功登录时判定。
//
// 失败尝试里的设备是攻击者的：把它计入「历史常用设备」会让攻击者第一次试探
// 就完成养熟，后续真正的异常登录反而不报警。
func TestRiskEngineDeviceChangeOnlyOnSuccess(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.knownDevices[12] = []string{"fp-known"}
	engine := newTestEngine(repo, nil)

	// 同设备：不报
	same := userSignal("frank", 12, true, "")
	same.DeviceFingerprint = "fp-known"
	engine.Observe(context.Background(), same)
	if len(repo.events) != 0 {
		t.Fatalf("同设备不应产生事件，实际 %+v", repo.events)
	}

	// 换设备（成功）：报
	sig := userSignal("frank", 12, true, "")
	sig.DeviceFingerprint = "fp-new"
	engine.Observe(context.Background(), sig)
	if len(repo.events) != 1 || repo.events[0].RuleCode != RuleDeviceFingerprintChanged {
		t.Fatalf("换设备应产生 DEVICE_FINGERPRINT_CHANGED，实际 %+v", repo.events)
	}
	if repo.events[0].RiskLevel != model.RiskLevelLow {
		t.Errorf("设备变更等级 = %q，期望 low", repo.events[0].RiskLevel)
	}

	// 换设备但登录失败：不报
	repo.events = nil
	sig = userSignal("frank", 12, false, "bad_password")
	sig.DeviceFingerprint = "fp-new-2"
	engine.Observe(context.Background(), sig)
	if len(repo.events) != 0 {
		t.Fatalf("登录失败时的设备变化不应产生设备变更事件")
	}
}

// TestRiskEngineNoFingerprintSkipsDeviceRule 前端未上报指纹时不判定设备变更。
//
// 空串当指纹会让所有无指纹的登录互相「设备变更」，是纯粹的误报来源。
func TestRiskEngineNoFingerprintSkipsDeviceRule(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.knownDevices[13] = []string{"fp-x"}
	engine := newTestEngine(repo, nil)

	sig := userSignal("grace", 13, true, "")
	sig.DeviceFingerprint = ""
	engine.Observe(context.Background(), sig)

	if len(repo.events) != 0 {
		t.Fatalf("无指纹时不应判定设备变更，实际 %+v", repo.events)
	}
}

// TestRiskEngineNewIPDisabledByDefault 新 IP 规则默认关闭（NAT 环境易误报）。
func TestRiskEngineNewIPDisabledByDefault(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.knownIPs[14] = []string{"10.0.0.1"}
	engine := newTestEngine(repo, nil)

	sig := userSignal("heidi", 14, true, "")
	sig.IP = "198.51.100.7"
	engine.Observe(context.Background(), sig)

	if len(repo.events) != 0 {
		t.Fatalf("默认应关闭新 IP 规则，实际产生 %d 条", len(repo.events))
	}
}

// TestRiskEngineNewIPWhenEnabled 显式开启后新 IP 规则生效。
func TestRiskEngineNewIPWhenEnabled(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.knownIPs[15] = []string{"10.0.0.1"}
	engine := newTestEngine(repo, map[string]string{ConfigRiskNewIPEnabled: "true"})

	sig := userSignal("ivan", 15, true, "")
	sig.IP = "198.51.100.8"
	engine.Observe(context.Background(), sig)

	if len(repo.events) != 1 || repo.events[0].RuleCode != RuleNewIPLogin {
		t.Fatalf("开启后应产生 NEW_IP_LOGIN 事件，实际 %+v", repo.events)
	}
	if repo.events[0].RiskType != RiskTypeSuspiciousIP {
		t.Errorf("风险类型 = %q，期望 suspicious_ip", repo.events[0].RiskType)
	}
}

// TestRiskEngineDisabledProducesNothing 总开关关闭时完全不产出事件。
func TestRiskEngineDisabledProducesNothing(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.failedByAccount["judy"] = 99
	repo.failedAccounts["203.0.113.10"] = 99
	repo.loginsByIP["203.0.113.10"] = 999
	engine := newTestEngine(repo, map[string]string{ConfigRiskRulesEnabled: "false"})

	engine.Observe(context.Background(), userSignal("judy", 16, false, "bad_password"))

	if len(repo.events) != 0 {
		t.Fatalf("总开关关闭时不应产生事件，实际 %d 条", len(repo.events))
	}
}

// TestRiskEngineBurstRule 登录频率异常（medium）。
func TestRiskEngineBurstRule(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.loginsByIP["203.0.113.10"] = 30
	engine := newTestEngine(repo, nil)

	sig := userSignal("karl", 17, true, "")
	sig.DeviceFingerprint = "fp-known"
	repo.knownDevices[17] = []string{"fp-known"} // 排除设备变更规则的干扰
	engine.Observe(context.Background(), sig)

	var found *model.RiskEvent
	for _, e := range repo.events {
		if e.RuleCode == RuleLoginBurst {
			found = e
		}
	}
	if found == nil {
		t.Fatalf("应产生 LOGIN_BURST 事件，实际 %+v", repo.events)
	}
	if found.RiskType != RiskTypeHighFrequency || found.RiskLevel != model.RiskLevelMedium {
		t.Errorf("频率事件类型/等级不符: %s / %s", found.RiskType, found.RiskLevel)
	}
}

// TestRiskEngineSubjectDomainIsolated 客户与员工的事件互不干扰（user_id 撞号）。
func TestRiskEngineSubjectDomainIsolated(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.failedByAccount["same_name"] = 5
	engine := newTestEngine(repo, nil)

	userSig := userSignal("same_name", 42, false, "bad_password")
	adminSig := userSignal("same_name", 42, false, "bad_password")
	adminSig.SubjectType = security.SubjectTypeAdmin

	engine.Observe(context.Background(), userSig)
	engine.Observe(context.Background(), adminSig)

	if len(repo.events) != 2 {
		t.Fatalf("两个主体域应各产生一条事件，实际 %d 条", len(repo.events))
	}
	if repo.events[0].SubjectType == repo.events[1].SubjectType {
		t.Fatalf("两条事件主体域相同（%s），说明撞号未被区分", repo.events[0].SubjectType)
	}
}

// TestRiskEngineAggregateIsPerAccount 聚合要按账号分开，不能把不同账号折叠成一条。
//
// 场景：撞库打的全是**不存在**的账号名，这些登录的 user_id 都是 0。若聚合只按
// (规则, user_id) 匹配，几百个被扫的账号名会塌成一条事件，行上还挂着第一个名字，
// 运营在页面上看到的是「某账号失败 5 次」，完全看不出「有人在扫一大批账号」。
func TestRiskEngineAggregateIsPerAccount(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.failedByAccount["scan_a"] = 5
	repo.failedByAccount["scan_b"] = 5
	engine := newTestEngine(repo, nil)

	// 两个账号都被扫（user_id 都是 0，模拟不存在的账号）。
	engine.Observe(context.Background(), userSignal("scan_a", 0, false, "user_not_found"))
	engine.Observe(context.Background(), userSignal("scan_b", 0, false, "user_not_found"))

	if len(repo.events) != 2 {
		t.Fatalf("两个不同账号应各产生一条事件，实际 %d 条（被折叠了）", len(repo.events))
	}
	if repo.events[0].Username == repo.events[1].Username {
		t.Fatalf("两条事件的账号名相同（%s），说明聚合没按账号分开", repo.events[0].Username)
	}

	// 同一账号再次命中：仍然聚合到它自己那条（不影响别的账号）。
	engine.Observe(context.Background(), userSignal("scan_a", 0, false, "user_not_found"))
	if len(repo.events) != 2 {
		t.Fatalf("同账号重复命中应聚合，实际 %d 条", len(repo.events))
	}
	if repo.events[0].OccurCount != 2 {
		t.Errorf("scan_a 的 occur_count = %d，期望 2", repo.events[0].OccurCount)
	}
	if repo.events[1].OccurCount != 1 {
		t.Errorf("scan_b 的 occur_count = %d，期望 1（不该被 scan_a 的命中带跑）", repo.events[1].OccurCount)
	}
}

// —— 黑名单守卫 ——

func TestBlacklistGuardBlocksByIP(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.blacklist = []model.Blacklist{{
		ID: 1, Type: security.BlacklistTypeIP, TargetValue: "203.0.113.10",
		Status: model.BlacklistStatusActive, EffectiveAt: time.Now().Add(-time.Hour),
	}}
	guard := NewBlacklistGuard(repo)

	err := guard.Check(context.Background(), "alice", "203.0.113.10", "fp-a")
	if err == nil {
		t.Fatal("命中 IP 黑名单应拒绝")
	}
	var blocked *security.ErrBlacklisted
	if !errors.As(err, &blocked) || blocked.Scope != security.BlacklistTypeIP {
		t.Fatalf("错误类型/维度不符: %v", err)
	}
	if len(repo.bumpedIDs) != 1 || repo.bumpedIDs[0] != 1 {
		t.Errorf("命中计数未累加: %v", repo.bumpedIDs)
	}
}

func TestBlacklistGuardBlocksByUserAndDevice(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.blacklist = []model.Blacklist{
		{ID: 2, Type: security.BlacklistTypeUser, TargetValue: "bob", Status: model.BlacklistStatusActive, EffectiveAt: time.Now().Add(-time.Hour)},
		{ID: 3, Type: security.BlacklistTypeDevice, TargetValue: "fp-b", Status: model.BlacklistStatusActive, EffectiveAt: time.Now().Add(-time.Hour)},
	}
	guard := NewBlacklistGuard(repo)

	if err := guard.Check(context.Background(), "bob", "10.0.0.1", "fp-other"); err == nil {
		t.Error("账号命中应拒绝")
	}
	if err := guard.Check(context.Background(), "other", "10.0.0.1", "fp-b"); err == nil {
		t.Error("设备命中应拒绝")
	}
}

// TestBlacklistGuardBlocksByPhoneAndEmail 手机号/邮箱类型的黑名单对验证码登录生效。
func TestBlacklistGuardBlocksByPhoneAndEmail(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.blacklist = []model.Blacklist{
		{ID: 4, Type: security.BlacklistTypePhone, TargetValue: "13800001111", Status: model.BlacklistStatusActive, EffectiveAt: time.Now().Add(-time.Hour)},
		{ID: 5, Type: security.BlacklistTypeEmail, TargetValue: "bad@example.com", Status: model.BlacklistStatusActive, EffectiveAt: time.Now().Add(-time.Hour)},
	}
	guard := NewBlacklistGuard(repo)

	// 短信登录时 account 传的就是手机号。
	if err := guard.Check(context.Background(), "13800001111", "10.0.0.2", ""); err == nil {
		t.Error("手机号命中应拒绝")
	}
	if err := guard.Check(context.Background(), "bad@example.com", "10.0.0.2", ""); err == nil {
		t.Error("邮箱命中应拒绝")
	}
}

// TestBlacklistGuardIgnoresInactiveAndExpired 停用与已过期的黑名单不再拦截。
//
// 这条最容易被忽略却最要紧：运营把一条黑名单「解除」或限时到期后，
// 对方必须真的能登录回来。
func TestBlacklistGuardIgnoresInactiveAndExpired(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	repo := newFakeRiskRepo()
	repo.blacklist = []model.Blacklist{
		{ID: 6, Type: security.BlacklistTypeIP, TargetValue: "10.1.1.1", Status: model.BlacklistStatusInactive, EffectiveAt: time.Now().Add(-time.Hour)},
		{ID: 7, Type: security.BlacklistTypeIP, TargetValue: "10.1.1.2", Status: model.BlacklistStatusActive, EffectiveAt: time.Now().Add(-time.Hour), ExpiredAt: &past},
		{ID: 8, Type: security.BlacklistTypeIP, TargetValue: "10.1.1.3", Status: model.BlacklistStatusActive, EffectiveAt: time.Now().Add(time.Hour)},
	}
	guard := NewBlacklistGuard(repo)

	for _, ip := range []string{"10.1.1.1", "10.1.1.2", "10.1.1.3"} {
		if err := guard.Check(context.Background(), "alice", ip, ""); err != nil {
			t.Errorf("IP %s 不应被拦截（停用/已过期/未生效）: %v", ip, err)
		}
	}
	if len(repo.bumpedIDs) != 0 {
		t.Errorf("未命中不应累加计数: %v", repo.bumpedIDs)
	}
}

// TestBlacklistGuardFailsOpen 查询出错时放行 —— 风控表异常不能让所有人登不进来。
func TestBlacklistGuardFailsOpen(t *testing.T) {
	repo := newFakeRiskRepo()
	repo.matchErr = errFakeDB
	guard := NewBlacklistGuard(repo)

	if err := guard.Check(context.Background(), "alice", "10.0.0.3", "fp"); err != nil {
		t.Fatalf("黑名单查询失败时应放行，实际 %v", err)
	}
}

// TestBlacklistGuardNilRepoFailsOpen 未装配时恒放行。
func TestBlacklistGuardNilRepoFailsOpen(t *testing.T) {
	guard := NewBlacklistGuard(nil)
	if err := guard.Check(context.Background(), "alice", "10.0.0.4", "fp"); err != nil {
		t.Fatalf("未装配时应放行，实际 %v", err)
	}
}

// TestBlacklistTypeValidation 类型白名单覆盖 doc06 要求的 5 类。
func TestBlacklistTypeValidation(t *testing.T) {
	for _, kind := range []string{"ip", "user", "device", "phone", "email"} {
		if !model.IsBlacklistType(kind) {
			t.Errorf("类型 %s 应被支持", kind)
		}
	}
	if model.IsBlacklistType("ua") {
		t.Error("ua 未落地实现，不应被接受")
	}
}

// TestRiskLevelValidation 等级白名单（升级接口的入参校验基准）。
func TestRiskLevelValidation(t *testing.T) {
	for _, level := range []string{"low", "medium", "high", "critical"} {
		if !isValidRiskLevel(level) {
			t.Errorf("等级 %s 应被支持", level)
		}
	}
	if isValidRiskLevel("banana") {
		t.Error("非法等级不应被接受")
	}
}

var errFakeDB = fakeError("db unavailable")

type fakeError string

func (e fakeError) Error() string { return string(e) }
