package dcimcloud

import (
	"context"
	"os"
	"testing"
	"time"

	"hostsent/backend/internal/pkg/upstream"
)

// TestNatReaderLive NAT 访问地址读取的实机验证（只读，不改面板任何状态）。
//
// 为什么要跑实机：nat_host_ip 在响应**外层 data** 上而不是列表项里，这个结构是从面板
// 模板（{$nat_host_ip}:{$vo.ext_port}）读出来的，只有真机请求才能确认解析对得上。
// 解析错了的后果是客户拿到空地址却以为机器可用。
//
// 需要 LIVE_MOFANGYUN=1 + 凭据；用 MOFANGYUN_NAT_INSTANCE 指定要读的实例号
// （默认取一台现网已开通实例；非 NAT 机型也应正常返回空列表而不是报错）。
func TestNatReaderLive(t *testing.T) {
	if os.Getenv("LIVE_MOFANGYUN") == "" {
		t.Skip("set LIVE_MOFANGYUN=1 to run live NAT reader test (read-only)")
	}
	p := NewProvider(&upstream.ProviderConfig{
		ID:          19,
		Name:        "魔方云测试接口",
		Type:        ProviderType,
		APIEndpoint: os.Getenv("MOFANGYUN_ENDPOINT"),
		APIKey:      os.Getenv("MOFANGYUN_USER"),
		APISecret:   os.Getenv("MOFANGYUN_PASS"),
		AccountType: "admin",
		Timeout:     30,
	})
	instanceID := os.Getenv("MOFANGYUN_NAT_INSTANCE")
	if instanceID == "" {
		instanceID = "13"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	forwards, err := p.ListNatForwards(ctx, instanceID)
	if err != nil {
		t.Fatalf("ListNatForwards(instance=%s) failed: %v", instanceID, err)
	}
	t.Logf("nat_acl: %d 条转发规则", len(forwards))
	for _, f := range forwards {
		addr := FormatNatForward(f.HostIP, f.ExternalPort)
		t.Logf("  id=%s name=%q %s -> 内网:%d 协议:%s", f.ID, f.Name, addr, f.InternalPort, f.Protocol)
		if addr == "" {
			t.Errorf("转发规则 %s 拼不出可用地址（HostIP=%q ExternalPort=%d）：客户将拿不到连接地址", f.ID, f.HostIP, f.ExternalPort)
		}
		if f.Protocol == "" {
			t.Errorf("转发规则 %s 协议为空", f.ID)
		}
	}

	webs, err := p.ListNatWebs(ctx, instanceID)
	if err != nil {
		t.Fatalf("ListNatWebs(instance=%s) failed: %v", instanceID, err)
	}
	t.Logf("nat_web: %d 条共享建站规则", len(webs))
	for _, w := range webs {
		t.Logf("  id=%s domain=%q %s", w.ID, w.Domain, FormatNatForward(w.HostIP, w.ExternalPort))
	}
}

// TestNatReaderEmptyInstanceIDLive 空实例号应直接返回空，不发请求（防调用方传空值打出无意义请求）。
func TestNatReaderEmptyInstanceIDLive(t *testing.T) {
	if os.Getenv("LIVE_MOFANGYUN") == "" {
		t.Skip("set LIVE_MOFANGYUN=1 to run live NAT reader test (read-only)")
	}
	p := NewProvider(&upstream.ProviderConfig{
		ID: 19, Type: ProviderType,
		APIEndpoint: os.Getenv("MOFANGYUN_ENDPOINT"),
		APIKey:      os.Getenv("MOFANGYUN_USER"), APISecret: os.Getenv("MOFANGYUN_PASS"),
		AccountType: "admin", Timeout: 10,
	})
	ctx := context.Background()
	if f, err := p.ListNatForwards(ctx, ""); err != nil || f != nil {
		t.Fatalf("空实例号应返回 (nil, nil)，实际 (%v, %v)", f, err)
	}
	if w, err := p.ListNatWebs(ctx, ""); err != nil || w != nil {
		t.Fatalf("空实例号应返回 (nil, nil)，实际 (%v, %v)", w, err)
	}
}
