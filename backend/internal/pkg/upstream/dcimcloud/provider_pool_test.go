package dcimcloud

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// TestListPools_MapsNodeCapacityFromStatus 钉住资源池容量口径：
// 容量来自 GET /nodes/{id}/status（CPU 核数与负载率、内存、硬盘），
// 而不是 /nodes 列表里的 cpu_used（已分配量）——后者算不出总量。
func TestListPools_MapsNodeCapacityFromStatus(t *testing.T) {
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/login":
			_, _ = w.Write([]byte("token"))
		case "/v1/nodes":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": 2, "name": "默认", "area_id": 1, "area_name": "默认", "status": 1, "enable": 1},
					{"id": 3, "name": "宿主机B", "area_id": 1, "area_name": "默认", "status": 1, "enable": 1},
				},
			})
		case "/v1/nodes/2/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":   1,
				"msg":      "连接成功",
				"cpu_info": map[string]interface{}{"cpu": 8, "use_percent": 6.4},
				"memory":   map[string]interface{}{"total_memory": "15.46GB", "use_memory": "2.08GB", "use_percent": 13.47},
				"disk":     map[string]interface{}{"disk_total": "29.99GB", "disk_use": "19.71GB", "disk_percent": "66%"},
			})
		case "/v1/nodes/3/status":
			// B 节点状态接口不可达：仍要出现在池列表里并标 warning，不能凭空消失。
			http.Error(w, `{"error":"node offline"}`, http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	})

	pools, err := p.ListPools(context.Background())
	if err != nil {
		t.Fatalf("ListPools 失败: %v", err)
	}
	if len(pools) != 2 {
		t.Fatalf("得到 %d 个池, want 2", len(pools))
	}
	a := pools[0]
	if a.ID != "node:2" || a.Type != "node" || a.Region != "默认" {
		t.Errorf("池标识/类型/地域不对: %+v", a)
	}
	// CPU 6.4% × 8 核 ≈ 1 核（四舍五入，下限 1）。
	if a.TotalCPU != 8 || a.UsedCPU != 1 {
		t.Errorf("CPU = %d/%d, want 1/8", a.UsedCPU, a.TotalCPU)
	}
	if a.TotalMemory != 15 || a.UsedMemory != 2 {
		t.Errorf("内存 = %d/%d GB, want 2/15", a.UsedMemory, a.TotalMemory)
	}
	if a.TotalDisk != 30 || a.UsedDisk != 20 {
		t.Errorf("磁盘 = %d/%d GB, want 20/30", a.UsedDisk, a.TotalDisk)
	}
	if a.Live == nil || a.Live.Status != "healthy" {
		t.Fatalf("A 节点探针结论应 healthy: %+v", a.Live)
	}
	if a.Status != "active" {
		t.Errorf("A 节点状态 = %q, want active", a.Status)
	}

	b := pools[1]
	if b.Live == nil || b.Live.Status != "warning" {
		t.Fatalf("B 节点采样失败应标 warning 且仍注册: %+v", b.Live)
	}
	if b.TotalCPU != 0 {
		t.Errorf("B 节点无采样数据，容量应为 0，实得 %d", b.TotalCPU)
	}
}

// TestListRegions_KeepsBareAreaID 区域沿用 /areas 裸 ID（历史 pool scope 写入的就是它），
// 且不带容量——容量口径归宿主机节点。
func TestListRegions_KeepsBareAreaID(t *testing.T) {
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/login":
			_, _ = w.Write([]byte("token"))
		case "/v1/areas":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": 1, "name": "默认", "status": 1, "country_code": "CN"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	regions, err := p.ListRegions(context.Background())
	if err != nil {
		t.Fatalf("ListRegions 失败: %v", err)
	}
	if len(regions) != 1 {
		t.Fatalf("得到 %d 个区域, want 1", len(regions))
	}
	if regions[0].ID != "1" || regions[0].Type != "region" {
		t.Errorf("区域标识不对: %+v", regions[0])
	}
	if regions[0].TotalCPU != 0 || regions[0].TotalDisk != 0 {
		t.Errorf("区域不应带容量: %+v", regions[0])
	}
	if regions[0].Live != nil {
		t.Errorf("区域无实时通道，不应有 Live: %+v", regions[0].Live)
	}
}

// TestParseSizeGB 面板容量串（"15.46GB" / "1.50TB" / "512MB"）换算为 GB 整数。
func TestParseSizeGB(t *testing.T) {
	cases := map[string]int{
		"15.46GB": 15,
		"29.99GB": 30,
		"1.50TB":  1536,
		"512MB":   1,
		"0.5GB":   1,
		"":        0,
		"abc":     0,
		"10.25":   10,
	}
	for in, want := range cases {
		if got := parseSizeGB(in); got != want {
			t.Errorf("parseSizeGB(%q) = %d, want %d", in, got, want)
		}
	}
}
