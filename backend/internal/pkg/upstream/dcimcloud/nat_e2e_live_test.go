package dcimcloud

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"hostsent/backend/internal/pkg/model"
)

// TestNatEndToEndLive NAT 链路端到端实机验证：开通一台 NAT 机器 → 建一条端口转发 →
// 读回访问地址 → 无条件销毁。
//
// 为什么必须实机：nat_host_ip 在响应外层 data 上（面板模板 {$nat_host_ip}:{$vo.ext_port}），
// 只有真机请求才能确认解析对得上；而"读不到地址"的后果是客户拿到一台连不上的机器。
// VPS 实例上这个端点返回 null（已验证），所以非空分支只能靠真机 NAT 机器覆盖。
//
// 需要 LIVE_MOFANGYUN=1 + 凭据。机器名带时间戳避免重名；无论成败都在 Cleanup 里销毁。
func TestNatEndToEndLive(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	res, err := p.ListPlatformResources(ctx)
	if err != nil {
		t.Fatalf("ListPlatformResources failed: %v", err)
	}
	if len(res.Areas) == 0 {
		t.Skip("测试节点没有可用区域")
	}
	var image, imageNode string
	for _, img := range res.Images {
		if img.Status == "active" && img.Value != "" {
			image, imageNode = img.Value, img.ParentID
			break
		}
	}
	if image == "" {
		t.Skip("测试节点没有已下载可用的镜像")
	}

	// NAT 机型基线：ip_num=0（不分配独立 IP）+ nat_acl_limit>=0（开转发）。
	// 这正是 doc122 的机型互斥规则要求的那组取值。
	extra := map[string]interface{}{
		"area":             res.Areas[0].Value,
		"os":               image,
		"cpu":              1,
		"memory":           1024,
		"system_disk_size": 20,
		"bw":               5,
		"network_type":     "normal",
		"ip_num":           0,
		"nat_acl_limit":    0,
		"nat_web_limit":    -1,
	}
	if imageNode != "" {
		extra["node"] = imageNode
	} else if len(res.Nodes) > 0 {
		extra["node"] = res.Nodes[0].Value
	}
	if len(res.Stores) > 0 {
		extra["store"] = res.Stores[0].Value
	}
	t.Logf("NAT provision params: %v", extra)

	inst, err := p.CreateInstance(ctx, &model.CreateInstanceRequest{
		Name:  fmt.Sprintf("hs-nat-%d", time.Now().Unix()%1000000),
		Extra: extra,
	})
	if err != nil {
		t.Fatalf("NAT 开通失败: %v", err)
	}
	if inst.UpstreamID == "" {
		t.Fatal("开通未返回云主机 ID")
	}
	t.Logf("created NAT instance upstream_id=%s status=%s", inst.UpstreamID, inst.Status)

	// 无条件清理：独立 ctx，避免测试超时后销毁不掉留下计费机器。
	t.Cleanup(func() {
		cc, cancelC := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancelC()
		// 先删转发规则（有的规则会阻塞实例销毁）。
		if fwds, ferr := p.ListNatForwards(cc, inst.UpstreamID); ferr == nil {
			for _, f := range fwds {
				if f.ID != "" {
					_ = p.deleteNatRule(cc, inst.UpstreamID, "nat_acl", f.ID)
				}
			}
		}
		if derr := p.DeleteInstance(cc, inst.UpstreamID); derr != nil {
			t.Errorf("清理失败，请在面板手动销毁 %s：%v", inst.UpstreamID, derr)
			return
		}
		t.Logf("cleaned up NAT instance %s", inst.UpstreamID)
	})

	// 1) 刚开通、还没建规则：应返回空列表而不是报错。
	fwds, err := p.ListNatForwards(ctx, inst.UpstreamID)
	if err != nil {
		t.Fatalf("ListNatForwards（空规则）失败: %v", err)
	}
	t.Logf("初始转发规则数=%d（预期 0，空列表不是错误）", len(fwds))

	// 2) 建一条端口转发：内网 22 → 自动分配外网端口。
	if err := p.createNatForward(ctx, inst.UpstreamID, "nat-live-test", 22); err != nil {
		t.Fatalf("创建 NAT 转发失败（nat_acl_limit=0 时应被允许）: %v", err)
	}

	// 3) 读回：这是本用例的核心断言 —— 客户必须能拿到可直接使用的地址。
	fwds, err = p.ListNatForwards(ctx, inst.UpstreamID)
	if err != nil {
		t.Fatalf("ListNatForwards 失败: %v", err)
	}
	if len(fwds) == 0 {
		t.Fatal("创建转发后仍读不到规则：NAT 机型的客户将拿不到连接地址")
	}
	for _, f := range fwds {
		addr := FormatNatForward(f.HostIP, f.ExternalPort)
		t.Logf("转发规则 id=%s name=%q addr=%s 内网=%d 协议=%s", f.ID, f.Name, addr, f.InternalPort, f.Protocol)
		if f.HostIP == "" {
			t.Error("HostIP 为空：nat_host_ip 漏解析（它在响应外层 data 上，不在列表项里）")
		}
		if f.ExternalPort <= 0 {
			t.Errorf("外部端口非法：%d", f.ExternalPort)
		}
		if addr == "" {
			t.Error("拼不出可用地址，客户无法连接")
		}
	}
}

// createNatForward 创建一条 NAT 端口转发（内网端口 → 面板自动分配外网端口）。
// 面板入参：name / int_port 必填，ext_port 省略表示自动分配，protocol 1=tcp。
func (p *Provider) createNatForward(ctx context.Context, providerInstanceID, name string, intPort int) error {
	if providerInstanceID == "" {
		return fmt.Errorf("实例号为空")
	}
	return p.call(ctx, "CreateNatForward", http.MethodPost,
		"/clouds/"+providerInstanceID+"/nat_acl",
		url.Values{
			"name":     {name},
			"int_port": {fmt.Sprintf("%d", intPort)},
			"protocol": {"1"},
		}, nil)
}

// deleteNatRule 删除一条 NAT 规则（acl 或 web）。
func (p *Provider) deleteNatRule(ctx context.Context, providerInstanceID, kind, ruleID string) error {
	return p.call(ctx, "DeleteNatRule", http.MethodDelete,
		"/"+kind+"/"+ruleID+"?hostid="+providerInstanceID, nil, nil)
}
