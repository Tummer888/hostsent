//go:build live

package dcimcloud

import (
	"context"
	"testing"
	"time"
)

// TestListPoolsLive 实机校验宿主机容量采集：/nodes + 逐节点 /nodes/{id}/status。
// 只读，不建任何资源。
func TestListPoolsLive(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pools, err := p.ListPools(ctx)
	if err != nil {
		t.Fatalf("ListPools failed: %v", err)
	}
	if len(pools) == 0 {
		t.Skip("测试面板没有节点")
	}
	probed := 0
	withParent := 0
	for _, pl := range pools {
		t.Logf("pool %s %q type=%s status=%s region=%s parent=%q cpu=%d/%d mem=%d/%dGB disk=%d/%dGB live=%+v",
			pl.ID, pl.Name, pl.Type, pl.Status, pl.Region, pl.ParentUpstreamID,
			pl.UsedCPU, pl.TotalCPU, pl.UsedMemory, pl.TotalMemory, pl.UsedDisk, pl.TotalDisk, pl.Live)
		// 宿主机必须挂到所属区域，否则页面渲染不出「区域 → 宿主机」层级。
		if pl.ID != "" && pl.ParentUpstreamID != "" {
			withParent++
		}
		if pl.Live == nil {
			continue
		}
		if pl.Live.Status == "healthy" {
			probed++
			if pl.TotalCPU == 0 || pl.TotalMemory == 0 || pl.TotalDisk == 0 {
				t.Errorf("节点 %s 探针正常但容量为 0（cpu=%d mem=%d disk=%d）",
					pl.ID, pl.TotalCPU, pl.TotalMemory, pl.TotalDisk)
			}
		}
	}
	if probed == 0 {
		t.Error("没有任何节点探针正常，容量口径未被验证")
	}
	if withParent == 0 {
		t.Error("没有节点带上所属区域（ParentUpstreamID 全空），层级视图挂不上父级")
	}

	regions, err := p.ListRegions(ctx)
	if err != nil {
		t.Fatalf("ListRegions failed: %v", err)
	}
	for _, r := range regions {
		t.Logf("region %s %q status=%s", r.ID, r.Name, r.Status)
	}
}
