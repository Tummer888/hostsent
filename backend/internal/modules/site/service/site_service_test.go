package service

import (
	"context"
	"errors"
	"testing"

	systemmodel "hostsent/backend/internal/modules/admin/system/model"
)

// fakeConfigReader 站点配置写入测试替身：按分组返回预置行。
type fakeConfigReader struct {
	byGroup map[string][]systemmodel.SystemConfig
	err     error
}

func (f *fakeConfigReader) ListByGroup(_ context.Context, group string) ([]systemmodel.SystemConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byGroup[group], nil
}

func cfg(key, value, status string) systemmodel.SystemConfig {
	return systemmodel.SystemConfig{ConfigKey: key, ConfigValue: value, Status: status, Group: systemmodel.ConfigGroupSite}
}

// TestSiteContentWhitelist 公开接口白名单：internal.* 与未登记键永不外露。
func TestSiteContentWhitelist(t *testing.T) {
	reader := &fakeConfigReader{byGroup: map[string][]systemmodel.SystemConfig{
		systemmodel.ConfigGroupSite: {
			cfg("site.name", "宿派云控", systemmodel.StatusActive),
			cfg("internal.mail_password", "smtp-secret", systemmodel.StatusActive),
			cfg("internal.api_key", "sk-live", systemmodel.StatusActive),
			cfg("unknown.key", "nope", systemmodel.StatusActive),
		},
	}}
	svc := NewSiteService(nil, reader)

	got, err := svc.SiteContent(context.Background())
	if err != nil {
		t.Fatalf("SiteContent 不应报错: %v", err)
	}
	if got.Items["site.name"] != "宿派云控" {
		t.Fatalf("白名单键应返回, got %+v", got.Items)
	}
	for _, leaked := range []string{"internal.mail_password", "internal.api_key", "unknown.key"} {
		if _, ok := got.Items[leaked]; ok {
			t.Fatalf("非白名单键泄漏: %s", leaked)
		}
	}
}

// TestSiteContentSkipsEmptyAndDisabled 空值与 disabled 项不返回，由前端回落默认值。
func TestSiteContentSkipsEmptyAndDisabled(t *testing.T) {
	reader := &fakeConfigReader{byGroup: map[string][]systemmodel.SystemConfig{
		systemmodel.ConfigGroupSite: {
			cfg("site.icp", "", systemmodel.StatusActive),
			cfg("site.slogan", "   ", systemmodel.StatusActive),
			cfg("site.copyright", "© 2026", systemmodel.StatusDisabled),
			cfg("site.name", "宿派云控", ""), // 状态为空视为可用（历史行）
		},
	}}
	svc := NewSiteService(nil, reader)

	got, err := svc.SiteContent(context.Background())
	if err != nil {
		t.Fatalf("SiteContent 不应报错: %v", err)
	}
	if _, ok := got.Items["site.icp"]; ok {
		t.Fatal("空值不应返回")
	}
	if _, ok := got.Items["site.slogan"]; ok {
		t.Fatal("纯空白不应返回")
	}
	if _, ok := got.Items["site.copyright"]; ok {
		t.Fatal("disabled 项不应返回")
	}
	if got.Items["site.name"] != "宿派云控" {
		t.Fatalf("空状态应视为可用, got %+v", got.Items)
	}
}

// TestSiteContentMergesGroups 管理端历史扁平键写在 base 分组，需与 site 分组一并读取。
func TestSiteContentMergesGroups(t *testing.T) {
	reader := &fakeConfigReader{byGroup: map[string][]systemmodel.SystemConfig{
		systemmodel.ConfigGroupSite: {{ConfigKey: "site.name", ConfigValue: "规范键", Status: systemmodel.StatusActive}},
		systemmodel.ConfigGroupBase: {
			{ConfigKey: "site_name", ConfigValue: "扁平键", Status: systemmodel.StatusActive},
			{ConfigKey: "contact_phone", ConfigValue: "400-000-0000", Status: systemmodel.StatusActive},
		},
	}}
	svc := NewSiteService(nil, reader)

	got, err := svc.SiteContent(context.Background())
	if err != nil {
		t.Fatalf("SiteContent 不应报错: %v", err)
	}
	for key, want := range map[string]string{
		"site.name":     "规范键",
		"site_name":     "扁平键",
		"contact_phone": "400-000-0000",
	} {
		if got.Items[key] != want {
			t.Fatalf("%s = %q, want %q", key, got.Items[key], want)
		}
	}
}

// TestSiteContentDegrades 读取失败与未装配 reader 时返回空 items 而非报错，官网回落默认值。
func TestSiteContentDegrades(t *testing.T) {
	failing := NewSiteService(nil, &fakeConfigReader{err: errors.New("db down")})
	got, err := failing.SiteContent(context.Background())
	if err != nil {
		t.Fatalf("读取失败不应报错: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("读取失败应返回空 items, got %+v", got.Items)
	}

	unwired := NewSiteService(nil, nil)
	got, err = unwired.SiteContent(context.Background())
	if err != nil || len(got.Items) != 0 {
		t.Fatalf("未装配 reader 应返回空 items, got (%+v,%v)", got.Items, err)
	}
}
