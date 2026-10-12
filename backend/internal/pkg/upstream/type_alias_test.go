package upstream

import "testing"

// TestResolveTypeAlias 类型码别名：历史码必须解析到当前码。
//
// 为什么需要这条守卫：类型码从 mofangyun/mofangfinance 改名成 dcimcloud/zjmf 后，
// 存量渠道行在数据迁移跑完前仍是旧码。若 Build/Descriptor 只认新码，这些渠道
// 会立刻变成「适配器未注册」并被熔断——参考 provider 19 的历史故障。
func TestResolveTypeAlias(t *testing.T) {
	// 用测试专用的别名对，避免依赖真实适配器包的 init() 顺序。
	RegisterTypeAlias("legacy_probe", "current_probe")

	if got := ResolveType("legacy_probe"); got != "current_probe" {
		t.Errorf("ResolveType(legacy_probe) = %q, want current_probe", got)
	}
	if got := ResolveType("current_probe"); got != "current_probe" {
		t.Errorf("当前码应原样返回，实得 %q", got)
	}
	if got := ResolveType("no_such_type"); got != "no_such_type" {
		t.Errorf("无别名应原样返回，实得 %q", got)
	}
}

// TestBuildResolvesLegacyAlias 别名渠道能通过 Build 构建出适配器。
func TestBuildResolvesLegacyAlias(t *testing.T) {
	mgr := &ProviderManager{
		providers: map[string]Provider{},
		factories: map[string]ProviderFactory{},
		byID:      map[uint]Provider{},
		configs:   map[uint]*ProviderConfig{},
	}
	called := ""
	mgr.RegisterFactory("current_probe", func(cfg *ProviderConfig) Provider {
		called = cfg.Type
		return nil
	})
	RegisterTypeAlias("legacy_probe2", "current_probe")

	if _, err := mgr.Build("legacy_probe2", &ProviderConfig{Type: "legacy_probe2"}); err != nil {
		t.Fatalf("旧码应能构建（别名解析），实得错误: %v", err)
	}
	if called == "" {
		t.Fatal("工厂未被调用")
	}
}
