package mofangyun

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// liveProvider 返回魔方云测试节点配置（自营链路联调用）。
// 会真实在测试面板上创建/销毁一台云主机，必须显式设置 LIVE_MOFANGYUN=1 才执行，
// 避免误在正式环境跑出机器。
func liveProvider(t *testing.T) *MoFangYunProvider {
	t.Helper()
	if os.Getenv("LIVE_MOFANGYUN") == "" {
		t.Skip("set LIVE_MOFANGYUN=1 to run live mofangyun provisioning (creates a real VM on the test panel)")
	}
	return NewMoFangYunProvider(&upstream.ProviderConfig{
		ID:          19,
		Name:        "魔方云测试接口",
		Type:        ProviderType,
		APIEndpoint: os.Getenv("MOFANGYUN_ENDPOINT"),
		APIKey:      os.Getenv("MOFANGYUN_USER"),
		APISecret:   os.Getenv("MOFANGYUN_PASS"),
		AccountType: "admin",
		Timeout:     30,
	})
}

// TestListPlatformResourcesLive 校验平台资源目录抓取：自营规格模板要靠这份目录
// 把 area/node/os/store 映射到平台真实取值，映射错了开通就会失败。
func TestListPlatformResourcesLive(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := p.ListPlatformResources(ctx)
	if err != nil {
		t.Fatalf("ListPlatformResources failed: %v", err)
	}
	t.Logf("areas=%d nodes=%d stores=%d images=%d", len(res.Areas), len(res.Nodes), len(res.Stores), len(res.Images))
	if len(res.Areas) == 0 || len(res.Images) == 0 {
		t.Fatalf("区域/镜像为空，映射无从建立")
	}
	for _, a := range res.Areas {
		t.Logf("area %s %s", a.Value, a.Label)
	}
	for _, n := range res.Nodes {
		t.Logf("node %s %s (area=%s)", n.Value, n.Label, n.ParentID)
	}
	for _, s := range res.Stores {
		t.Logf("store %s %s (area=%s)", s.Value, s.Label, s.ParentID)
	}
	t.Logf("image[0] %s %s", res.Images[0].Value, res.Images[0].Label)
}

// TestCreateInstanceFromSpecMappingLive 用「规格模板映射出来的平台参数」真机开通一台云主机
// 并在结束后销毁：验证 area/node/os/cpu/memory/system_disk_size/store 这套参数被面板接受。
func TestCreateInstanceFromSpecMappingLive(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	res, err := p.ListPlatformResources(ctx)
	if err != nil {
		t.Fatalf("ListPlatformResources failed: %v", err)
	}
	if len(res.Areas) == 0 {
		t.Skip("测试节点没有可用区域")
	}
	// 只挑在节点上真正可用的镜像（status=active）：测试节点多数镜像未下载，直接下发会被面板拒绝。
	var image, imageNode string
	for _, img := range res.Images {
		if img.Status == "active" && img.Value != "" {
			image, imageNode = img.Value, img.ParentID
			break
		}
	}
	if image == "" {
		t.Skip("测试节点没有已下载可用的镜像（请在面板「镜像管理」中下载后再跑）")
	}
	// 规格模板的平台参数（与后台「规格模板 → 平台映射」产出的键一致）。
	extra := map[string]interface{}{
		"area":             res.Areas[0].Value,
		"os":               image,
		"cpu":              2,
		"memory":           2048,
		"system_disk_size": 30,
		"bw":               5,
		"network_type":     "normal",
		"ip_num":           0,
	}
	// 镜像可用性与节点强绑定：优先用镜像所属节点，否则测试节点会报"镜像不可用"。
	if imageNode != "" {
		extra["node"] = imageNode
	} else if len(res.Nodes) > 0 {
		extra["node"] = res.Nodes[0].Value
	}
	if len(res.Stores) > 0 {
		extra["store"] = res.Stores[0].Value
	}
	t.Logf("provision params: %v", extra)

	inst, err := p.CreateInstance(ctx, &model.CreateInstanceRequest{
		// 主机名在面板内唯一：带时间戳避免与历史测试机冲突（面板报"主机名已存在"）。
		Name:  fmt.Sprintf("hs-live-%d", time.Now().Unix()%1000000),
		Extra: extra,
	})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	t.Logf("created instance upstream_id=%s status=%s", inst.UpstreamID, inst.Status)
	if inst.UpstreamID == "" {
		t.Fatalf("开通未返回云主机 ID")
	}

	// 暂停/恢复/销毁闭环：履约链路（到期暂停→宽限→销毁）走的就是这三个接口。
	if err := p.SuspendInstance(ctx, inst.UpstreamID, "due"); err != nil {
		t.Errorf("SuspendInstance failed: %v", err)
	} else {
		t.Logf("suspend ok（type=due）")
	}
	if err := p.UnsuspendInstance(ctx, inst.UpstreamID); err != nil {
		t.Errorf("UnsuspendInstance failed: %v", err)
	} else {
		t.Logf("unsuspend ok")
	}

	t.Cleanup(func() {
		// 清理上下文独立于测试 ctx，避免超时后无法销毁。
		cleanupCtx, cc := context.WithTimeout(context.Background(), 90*time.Second)
		defer cc()
		if err := p.DeleteInstance(cleanupCtx, inst.UpstreamID); err != nil {
			t.Logf("cleanup DeleteInstance(%s) failed: %v", inst.UpstreamID, err)
			return
		}
		t.Logf("cleaned up instance %s", inst.UpstreamID)
	})
}
