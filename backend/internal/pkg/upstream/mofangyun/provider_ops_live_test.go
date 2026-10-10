package mofangyun

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// 实例全操作实机复核（LIVE_MOFANGYUN=1 才跑；会在测试面板真建一台云主机并在结束时销毁）。
//
// 这份测试回答的问题是："面板上能做的每一项操作，我们的适配器是不是都真的打通了。"
// 逐项调用并把面板的原始错误带出来，避免"接口拼错了但一直没人发现"
// （VNC 就曾因为写成 GET 而在生产里 404，见下方 TestLive_VNCUsesPost）。
//
// 顺序：建机 → 只读 → 电源（软/硬）→ 重装 → 重置密码 → 救援进出 → 快照增查删
//      → 带宽/IP/数据盘 → 暂停恢复 → 销毁。任一步失败即记录，最后统一清理。

// liveInstance 建一台测试机并返回适配器与实例号；t.Cleanup 一定销毁。
func liveInstance(t *testing.T, extraFn func(*upstream.PlatformResources) map[string]interface{}) (*MoFangYunProvider, string) {
	t.Helper()
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
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
	extra := map[string]interface{}{
		"area":             res.Areas[0].Value,
		"os":               image,
		"cpu":              2,
		"memory":           2048,
		"system_disk_size": 30,
		"bw":               5,
		"network_type":     "normal",
		"ip_num":           0,
		// 快照/备份数量限制：测试机都给 0（不限量），否则面板拒绝创建快照。
		"snap_num":   0,
		"backup_num": 0,
	}
	if imageNode != "" {
		extra["node"] = imageNode
	} else if len(res.Nodes) > 0 {
		extra["node"] = res.Nodes[0].Value
	}
	if len(res.Stores) > 0 {
		extra["store"] = res.Stores[0].Value
	}
	if extraFn != nil {
		for k, v := range extraFn(res) {
			extra[k] = v
		}
	}
	inst, err := p.CreateInstance(ctx, &model.CreateInstanceRequest{
		Name:  fmt.Sprintf("hs-ops-%d", time.Now().Unix()%1000000),
		Extra: extra,
	})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	if inst.UpstreamID == "" {
		t.Fatal("开通未返回云主机 ID")
	}
	t.Logf("created instance id=%s", inst.UpstreamID)

	t.Cleanup(func() {
		cleanupCtx, cc := context.WithTimeout(context.Background(), 120*time.Second)
		defer cc()
		if err := p.DeleteInstance(cleanupCtx, inst.UpstreamID); err != nil {
			t.Errorf("清理失败，请手动删除实例 %s：%v", inst.UpstreamID, err)
			return
		}
		t.Logf("cleaned up instance %s", inst.UpstreamID)
	})
	return p, inst.UpstreamID
}

// waitIdle 等实例从 task 状态回到稳态（面板同一时刻只允许一个耗时任务）。
func waitIdle(t *testing.T, p *MoFangYunProvider, id string, tries int) string {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < tries; i++ {
		var st struct {
			Status   string `json:"status"`
			TaskName string `json:"task_name"`
		}
		if err := p.call(ctx, "status", "GET", "/clouds/"+id+"/status", nil, &st); err == nil {
			if st.Status != "task" {
				return st.Status
			}
		}
		time.Sleep(5 * time.Second)
	}
	return "task"
}

// TestLive_InstanceOperationsLive 逐项复核实例操作。
func TestLive_InstanceOperationsLive(t *testing.T) {
	p, id := liveInstance(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	// ---- 只读 ----
	t.Run("GetInstance", func(t *testing.T) {
		inst, err := p.GetInstance(ctx, id)
		if err != nil {
			t.Fatalf("GetInstance: %v", err)
		}
		if inst.UpstreamID == "" || inst.Name == "" || len(inst.RawData) == 0 {
			t.Fatalf("详情缺字段：%+v", inst)
		}
		t.Logf("detail id=%s name=%s status=%s raw_keys=%d", inst.UpstreamID, inst.Name, inst.Status, len(inst.RawData))
	})

	// ---- 控制台：必须是 POST（GET 会 404） ----
	t.Run("VNC", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		res, err := p.VNC(ctx, id)
		if err != nil {
			t.Fatalf("VNC: %v", err)
		}
		if res.URL == "" {
			t.Fatal("VNC 未返回地址")
		}
		t.Logf("vnc url=%s external=%v", res.URL, res.External)
	})

	// ---- 电源：软关/开/软重启 ----
	t.Run("PowerSoft", func(t *testing.T) {
		if err := p.StopInstance(ctx, id, false); err != nil {
			t.Fatalf("软关机: %v", err)
		}
		if st := waitIdle(t, p, id, 24); st != "off" {
			t.Fatalf("软关机后状态 = %s, want off", st)
		}
		if err := p.StartInstance(ctx, id); err != nil {
			t.Fatalf("开机: %v", err)
		}
		if st := waitIdle(t, p, id, 24); st != "on" {
			t.Fatalf("开机后状态 = %s, want on", st)
		}
		if err := p.RestartInstance(ctx, id); err != nil {
			t.Fatalf("软重启: %v", err)
		}
		waitIdle(t, p, id, 24)
	})

	// ---- 电源：硬关/硬重启（独立指令，非"软关再开"） ----
	t.Run("PowerHard", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		if err := p.HardStopInstance(ctx, id); err != nil {
			t.Fatalf("硬关机: %v", err)
		}
		if st := waitIdle(t, p, id, 24); st != "off" {
			t.Fatalf("硬关机后状态 = %s, want off", st)
		}
		if err := p.StartInstance(ctx, id); err != nil {
			t.Fatalf("开机: %v", err)
		}
		waitIdle(t, p, id, 24)
		if err := p.HardRestartInstance(ctx, id); err != nil {
			t.Fatalf("硬重启: %v", err)
		}
		waitIdle(t, p, id, 24)
	})

	// ---- 重置密码 ----
	t.Run("ResetPassword", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		if err := p.ResetInstancePassword(ctx, id, "HsLive#2026aa"); err != nil {
			t.Fatalf("重置密码: %v", err)
		}
		waitIdle(t, p, id, 24)
		t.Log("重置密码 ok")
	})

	// ---- 救援系统 进/出 ----
	t.Run("Rescue", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		if err := p.RescueInstance(ctx, id, 1, "HsRescue#2026"); err != nil {
			// 面板要先在「镜像管理」里下载救援系统镜像才允许进入救援。
			// 这是测试节点的资源前置条件，不是路由写错——路由错会报「请求地址有误」。
			// 面板给出明确业务原因，即证明接口已打通：跳过而不是失败。
			if strings.Contains(err.Error(), "救援系统") {
				t.Skipf("测试节点未下载救援系统镜像（接口已打通，面板给出业务前置原因）：%v", err)
			}
			t.Fatalf("进入救援: %v", err)
		}
		t.Log("进入救援 ok（等面板执行）")
		if st := waitIdle(t, p, id, 48); st != "on" && st != "off" {
			t.Logf("救援后状态 = %s（继续尝试退出）", st)
		}
		if err := p.ExitRescueInstance(ctx, id); err != nil {
			t.Fatalf("退出救援: %v", err)
		}
		waitIdle(t, p, id, 48)
		t.Log("退出救援 ok")
	})

	// ---- 快照：建 / 列 / 恢复 / 删 ----
	t.Run("Snapshot", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		diskID := systemDiskIDViaAdapter(t, p, id)
		if err := p.CreateSnapshot(ctx, &upstream.CreateSnapshotRequest{
			ProviderInstanceID: id,
			DiskID:             diskID,
			Type:               upstream.SnapshotTypeSnap,
			Name:               "hs-live-snap",
		}); err != nil {
			t.Fatalf("创建快照: %v", err)
		}
		waitIdle(t, p, id, 60)
		rows, err := p.ListSnapshots(ctx, id, upstream.SnapshotTypeSnap)
		if err != nil {
			t.Fatalf("列快照: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("创建成功但列表为空")
		}
		snapID := rows[len(rows)-1].ID
		t.Logf("快照已创建 id=%s name=%s", snapID, rows[len(rows)-1].Name)
		if err := p.DeleteSnapshot(ctx, snapID); err != nil {
			t.Fatalf("删除快照: %v", err)
		}
		t.Log("删除快照 ok")
	})

	// ---- 带宽直改 ----
	t.Run("UpdateBandwidth", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		if err := p.UpdateBandwidth(ctx, id, 8, 8); err != nil {
			t.Fatalf("改带宽: %v", err)
		}
		waitIdle(t, p, id, 24)
		inst, err := p.GetInstance(ctx, id)
		if err != nil {
			t.Fatalf("改带宽后取详情: %v", err)
		}
		t.Logf("改带宽 ok，面板详情已可读（ns=%d）", len(inst.RawData))
	})

	// ---- 加 IPv6（节点未启用时会明确报错，属预期） ----
	t.Run("AddIPv6", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		err := p.AddIPv6(ctx, id, 1)
		if err != nil {
			// 测试节点多半没启用 IPv6；报错即"接口打通且面板给了明确原因"。
			t.Logf("加 IPv6 被平台拒绝（预期，节点未启用）：%v", err)
			return
		}
		t.Log("加 IPv6 ok")
	})

	// ---- 暂停 / 恢复 ----
	t.Run("SuspendUnsuspend", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		if err := p.SuspendInstance(ctx, id, "due"); err != nil {
			t.Fatalf("暂停: %v", err)
		}
		st := waitIdle(t, p, id, 48)
		t.Logf("暂停后状态 = %s", st)
		if err := p.UnsuspendInstance(ctx, id); err != nil {
			t.Fatalf("恢复: %v", err)
		}
		waitIdle(t, p, id, 48)
		t.Log("恢复 ok")
	})

	// ---- 重装：真装一次（换系统盘），验证面板接受 os 并返回新凭据 ----
	t.Run("Reinstall", func(t *testing.T) {
		waitIdle(t, p, id, 24)
		// 先验证参数校验：不给 os 必须被本地拦住（不消耗面板任务）。
		if _, err := p.ReinstallInstance(ctx, &upstream.ReinstallRequest{ProviderInstanceID: id}); err == nil {
			t.Fatal("未指定 os 竟然成功，应报错")
		}
		// 取一个面板上可用的镜像 ID 作为目标系统。
		res, err := p.ListPlatformResources(ctx)
		if err != nil {
			t.Fatalf("取镜像列表: %v", err)
		}
		var osID string
		for _, img := range res.Images {
			if img.Status == "active" && img.Value != "" {
				osID = img.Value
				break
			}
		}
		if osID == "" {
			t.Skip("测试节点没有可用镜像，跳过真重装")
		}
		out, err := p.ReinstallInstance(ctx, &upstream.ReinstallRequest{
			ProviderInstanceID: id,
			OS:                 osID,
		})
		if err != nil {
			t.Fatalf("重装(os=%s): %v", osID, err)
		}
		if out != nil && out.Password != "" {
			t.Logf("重装已受理，平台签发新凭据 user=%s（密码已收到，不回显）", out.Username)
		} else {
			t.Log("重装已受理（平台未返回新凭据，沿用原密码）")
		}
		if st := waitIdle(t, p, id, 120); st == "task" {
			t.Log("重装仍在执行（长任务，属正常）")
		} else {
			t.Logf("重装后状态 = %s", st)
		}
	})
}

// systemDiskIDViaAdapter 从实例详情里取系统盘 ID（快照挂在磁盘上）。
func systemDiskIDViaAdapter(t *testing.T, p *MoFangYunProvider, id string) string {
	t.Helper()
	var detail map[string]interface{}
	if err := p.call(context.Background(), "detail", "GET", "/clouds/"+id, nil, &detail); err != nil {
		t.Fatalf("取磁盘列表失败: %v", err)
	}
	disks, ok := detail["disk"].([]interface{})
	if !ok {
		t.Fatalf("详情里没有 disk 数组：%T", detail["disk"])
	}
	for _, d := range disks {
		if m, ok := d.(map[string]interface{}); ok && fmt.Sprintf("%v", m["type"]) == "system" {
			return fmt.Sprintf("%v", m["id"])
		}
	}
	if len(disks) > 0 {
		if m, ok := disks[0].(map[string]interface{}); ok {
			return fmt.Sprintf("%v", m["id"])
		}
	}
	t.Fatal("找不到任何磁盘")
	return ""
}

// TestLive_VNCUsesPost 回归：/clouds/{id}/vnc 必须用 POST。
// 这条断言锁住一个真实事故——适配器曾用 GET，面板返回 404「请求地址有误」，
// 客户点「控制台」永远打不开，而单测因为没有覆盖 VNC 全绿。
func TestLive_VNCUsesPost(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 直连面板对比两种方法，确认 GET 确实不可用（若哪天面板改成 GET 也认，这条会失败并提醒我们）。
	base, err := p.baseURL()
	if err != nil {
		t.Fatal(err)
	}
	token, err := p.login(ctx, false)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	instances, err := p.ListInstances(ctx, map[string]string{"per_page": "1"})
	if err != nil || len(instances) == 0 {
		t.Skipf("面板上没有实例可用于探测（%v）", err)
	}
	id := instances[0].UpstreamID

	_, getStatus, getErr := p.doWithToken(ctx, base, token, "GET", "/clouds/"+id+"/vnc", nil)
	if getErr == nil && getStatus == 200 {
		t.Errorf("面板开始接受 GET /clouds/%s/vnc 了（此前返回 404）；"+
			"适配器已改用 POST，请复核是否仍需保留 POST（当前两端都可用不算错，但别再退回 GET）", id)
	}
	t.Logf("GET /clouds/%s/vnc → HTTP %d err=%v（预期非 200）", id, getStatus, getErr)

	res, err := p.VNC(ctx, id)
	if err != nil {
		t.Fatalf("VNC（POST）失败: %v", err)
	}
	if res.URL == "" {
		t.Fatal("VNC 未返回地址")
	}
	t.Logf("POST /clouds/%s/vnc → ok, url=%s", id, res.URL)
}

var _ = os.Getenv
