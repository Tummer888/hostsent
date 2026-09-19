package oauth

import (
	"context"
	"strings"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"hostsent/backend/internal/pkg/integration"
)

// 描述符与工厂的注册表行为：AllDescriptors 的 Implemented 必须由工厂注册情况推导，
// 而不是由描述符自己声明——否则「界面显示已接入但调用报未实现」会静默发生。
func TestRegistry_DescriptorAndFactory(t *testing.T) {
	const probe = "registry_probe_oauth"

	RegisterDescriptor(probe, CapabilityDescriptor{
		Type: probe,
		Name: "探针",
		Mode: "api",
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "AppID", Type: integration.FieldTypeString, Required: true},
		},
	})
	t.Cleanup(func() {
		descriptorMu.Lock()
		delete(descriptors, probe)
		descriptorMu.Unlock()
	})

	if _, ok := Descriptor(probe); !ok {
		t.Fatal("注册描述符后应能查到")
	}
	if IsRegistered(probe) {
		t.Fatal("只注册描述符时工厂不应被认为已注册")
	}

	for _, d := range AllDescriptors() {
		if d.Type == probe && d.Implemented {
			t.Fatal("无工厂时 Implemented 必须为 false")
		}
	}

	RegisterFactory(probe, func(cfg ProviderConfig) Provider { return &probeProvider{} })
	t.Cleanup(func() {
		factoryMu.Lock()
		delete(factories, probe)
		factoryMu.Unlock()
	})

	if !IsRegistered(probe) {
		t.Fatal("注册工厂后 IsRegistered 应为 true")
	}
	found := false
	for _, d := range AllDescriptors() {
		if d.Type == probe {
			found = true
			if !d.Implemented {
				t.Fatal("有工厂时 Implemented 必须为 true")
			}
		}
	}
	if !found {
		t.Fatal("AllDescriptors 应包含已注册的描述符")
	}

	p, err := New(probe, ProviderConfig{Type: probe})
	if err != nil {
		t.Fatalf("New 应成功: %v", err)
	}
	if p.Type() != probe {
		t.Fatalf("Type 应为 %s，实际 %s", probe, p.Type())
	}
	if _, err := New("no_such_provider_type", ProviderConfig{}); err == nil {
		t.Fatal("未注册类型必须报错")
	}
}

// AllDescriptors 必须按 Type 升序，后台类型下拉的顺序不能随 map 遍历随机抖动。
func TestRegistry_AllDescriptorsSorted(t *testing.T) {
	list := AllDescriptors()
	for i := 1; i < len(list); i++ {
		if list[i-1].Type > list[i].Type {
			t.Fatalf("描述符未按 Type 升序：%s 在 %s 之前", list[i-1].Type, list[i].Type)
		}
	}
}

// 必填校验：缺字段要报错，且错误里带中文标签而不是裸 key（运营要看得懂）。
func TestValidateCredentials(t *testing.T) {
	d := CapabilityDescriptor{
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "AppID", Required: true},
			{Key: "app_secret", Label: "AppSecret", Required: true, Secret: true},
			{Key: "scope", Label: "授权范围", Required: false},
		},
	}

	if err := ValidateCredentials(d, map[string]string{"app_id": "x", "app_secret": "y"}); err != nil {
		t.Fatalf("必填齐全时不应报错: %v", err)
	}
	err := ValidateCredentials(d, map[string]string{"app_id": "x"})
	if err == nil {
		t.Fatal("缺 app_secret 必须报错")
	}
	if !strings.Contains(err.Error(), "AppSecret") {
		t.Fatalf("错误信息应包含中文字段标签，实际: %v", err)
	}
	if err := ValidateCredentials(d, map[string]string{"app_id": "", "app_secret": "y"}); err == nil {
		t.Fatal("空字符串等同缺失，必须报错")
	}
}

// state 令牌：签发→解析往返，且 provider / mode / userID / inviteCode 原样带出。
func TestStateSigner_RoundTrip(t *testing.T) {
	s := NewStateSigner("test-secret", "hostsent-test")

	token, err := s.IssueState("wechat", ModeBind, 42, "INV-9", "nonce-1")
	if err != nil {
		t.Fatalf("签发 state 失败: %v", err)
	}
	claims, err := s.ParseState(token)
	if err != nil {
		t.Fatalf("解析 state 失败: %v", err)
	}
	if claims.Provider != "wechat" || claims.Mode != ModeBind || claims.UserID != 42 || claims.InviteCode != "INV-9" {
		t.Fatalf("state 声明往返不一致: %+v", claims)
	}
}

// 篡改（换密钥签发）必须被拒——这是 CSRF 防护的全部意义。
func TestStateSigner_RejectsTampered(t *testing.T) {
	issuer := NewStateSigner("secret-a", "hostsent-test")
	other := NewStateSigner("secret-b", "hostsent-test")

	token, err := issuer.IssueState("qq", ModeLogin, 0, "", "n")
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if _, err := other.ParseState(token); err == nil {
		t.Fatal("换密钥后必须解析失败")
	}
	if _, err := issuer.ParseState(token + "x"); err == nil {
		t.Fatal("被追加字符的令牌必须解析失败")
	}
	if _, err := issuer.ParseState(""); err == nil {
		t.Fatal("空 state 必须被拒")
	}
}

// 过期必须被拒：state 的 TTL 是重放窗口的上界。
func TestStateSigner_RejectsExpired(t *testing.T) {
	s := NewStateSigner("secret", "hostsent-test")
	// 直接签一个已过期的 state：绕过 IssueState 的固定 TTL。
	now := time.Now().Add(-2 * StateTTL)
	claims := StateClaims{
		Provider: "alipay",
		Mode:     ModeLogin,
		Nonce:    "n",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "hostsent-test",
			Audience:  jwt.ClaimStrings{AudienceOAuthState},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(StateTTL)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("构造过期令牌失败: %v", err)
	}
	if _, err := s.ParseState(token); err == nil {
		t.Fatal("过期 state 必须被拒")
	}
}

// audience 隔离：访问令牌/ticket 都不能当 state 用（防越权复用）。
func TestStateSigner_AudienceIsolation(t *testing.T) {
	s := NewStateSigner("secret", "hostsent-test")

	ticket, err := s.IssueTicket(7, "wechat", false, "n")
	if err != nil {
		t.Fatalf("签发 ticket 失败: %v", err)
	}
	if _, err := s.ParseState(ticket); err == nil {
		t.Fatal("ticket 的 audience 与 state 不同，不能当 state 解析")
	}

	state, err := s.IssueState("wechat", ModeLogin, 0, "", "n")
	if err != nil {
		t.Fatalf("签发 state 失败: %v", err)
	}
	if _, err := s.ParseTicket(state); err == nil {
		t.Fatal("state 不能当 ticket 解析")
	}

	// 普通访问令牌（无 oauth audience）同样必须被拒。
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "hostsent-test",
		Audience:  jwt.ClaimStrings{"access"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	accessStr, err := access.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("构造访问令牌失败: %v", err)
	}
	if _, err := s.ParseState(accessStr); err == nil {
		t.Fatal("普通访问令牌不能当 state 使用")
	}
}

// 换签名算法（none / RS256）必须被 keyFunc 拦下，避免算法混淆攻击。
func TestStateSigner_RejectsWrongSigningMethod(t *testing.T) {
	s := NewStateSigner("secret", "hostsent-test")
	token := jwt.NewWithClaims(jwt.SigningMethodNone, StateClaims{
		Provider: "wechat",
		Mode:     ModeLogin,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{AudienceOAuthState},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("构造 none 令牌失败: %v", err)
	}
	if _, err := s.ParseState(unsigned); err == nil {
		t.Fatal("alg=none 的令牌必须被拒")
	}
}

// ticket 往返：needBind 标记必须原样带出，前端据此决定跳绑定页还是直接登录。
func TestStateSigner_TicketRoundTrip(t *testing.T) {
	s := NewStateSigner("secret", "hostsent-test")
	ticket, err := s.IssueTicket(99, "alipay", true, "n2")
	if err != nil {
		t.Fatalf("签发 ticket 失败: %v", err)
	}
	claims, err := s.ParseTicket(ticket)
	if err != nil {
		t.Fatalf("解析 ticket 失败: %v", err)
	}
	if claims.UserID != 99 || claims.Provider != "alipay" || !claims.NeedBind {
		t.Fatalf("ticket 声明往返不一致: %+v", claims)
	}
}

type probeProvider struct{}

func (p *probeProvider) Type() string { return "registry_probe_oauth" }
func (p *probeProvider) AuthorizeURL(cfg ProviderConfig, state, redirectURI string) string {
	return ""
}
func (p *probeProvider) Exchange(ctx context.Context, cfg ProviderConfig, code, redirectURI string) (*Token, error) {
	return nil, nil
}
func (p *probeProvider) UserInfo(ctx context.Context, cfg ProviderConfig, token *Token) (*ExternalUser, error) {
	return nil, nil
}
func (p *probeProvider) Test(ctx context.Context, cfg ProviderConfig) error { return nil }
