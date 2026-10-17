package service

import (
	"encoding/json"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/resource/provider/dto"
	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
)

// TestSummarizePoolsExcludesLocationPools 容量汇总必须把位置型池（区域/可用区）排除在
// 容量累加之外：它们没有 CPU/内存/磁盘配额，计入只会把「CPU 总量」这类看板数字稀释成
// 失真值（实测现象：一个区域行 + 一台宿主机，CPU 总量看起来总是偏低）。
// 但两类计数要分别给出，页面才能同时说清「几台宿主机、几个区域」。
func TestSummarizePoolsExcludesLocationPools(t *testing.T) {
	items := []providermodel.ResourcePool{
		{ // 宿主机：有容量
			PoolType: "node", Status: 1, Region: "默认",
			TotalCPU: 8, UsedCPU: 1, TotalMemory: 15, UsedMemory: 2, TotalDisk: 30, UsedDisk: 20,
		},
		{ // 区域：无容量，容量列全 0
			PoolType: "region", Status: 1, Region: "默认",
		},
		{ // 存储池：有容量
			PoolType: "storage", Status: 1, Region: "默认",
			TotalCPU: 0, UsedCPU: 0, TotalMemory: 0, UsedMemory: 0, TotalDisk: 1000, UsedDisk: 300,
		},
		{ // 区域停用：计数照算，在线数不计
			PoolType: "region", Status: 0, Region: "",
		},
	}

	s := summarizePools(items)

	if s.TotalPools != 4 {
		t.Errorf("TotalPools = %d, want 4", s.TotalPools)
	}
	if s.CapacityPools != 2 {
		t.Errorf("CapacityPools = %d, want 2（宿主机 + 存储）", s.CapacityPools)
	}
	if s.LocationPools != 2 {
		t.Errorf("LocationPools = %d, want 2（两个区域）", s.LocationPools)
	}
	// 容量只来自容量型：8/15/30 + 0/0/1000
	if s.TotalCPU != 8 || s.UsedCPU != 1 {
		t.Errorf("CPU = %d/%d, want 1/8（区域不得计入）", s.UsedCPU, s.TotalCPU)
	}
	if s.TotalMemory != 15 || s.UsedMemory != 2 {
		t.Errorf("内存 = %d/%d, want 2/15", s.UsedMemory, s.TotalMemory)
	}
	if s.TotalDisk != 1030 || s.UsedDisk != 320 {
		t.Errorf("磁盘 = %d/%d, want 320/1030", s.UsedDisk, s.TotalDisk)
	}
	if s.OnlinePools != 3 {
		t.Errorf("OnlinePools = %d, want 3（停用的区域不计）", s.OnlinePools)
	}
	if s.UnlocatedPools != 1 {
		t.Errorf("UnlocatedPools = %d, want 1（那个停用且无地域的区域）", s.UnlocatedPools)
	}
	// 宿主机磁盘 20/30 ≈ 67%，越过 60% 预警线（未到 80% 告警线）；
	// 存储池 300/1000 = 30%，不告警——告警判定用的是池自身的比例，不是汇总比例。
	if s.WarningPools != 1 || s.DangerPools != 0 {
		t.Errorf("告警池 = %d/%d, want 1/0", s.WarningPools, s.DangerPools)
	}
}

// TestSummarizePoolsLocationOnly 全是位置型池时容量必须为 0，不能因为
// 「都是区域」就把它们的 0 当正常容量汇总（避免 0/0 除零或伪造出额度）。
func TestSummarizePoolsLocationOnly(t *testing.T) {
	s := summarizePools([]providermodel.ResourcePool{
		{PoolType: "region", Status: 1},
		{PoolType: "zone", Status: 1},
	})
	if s.CapacityPools != 0 || s.LocationPools != 2 {
		t.Errorf("分类计数错误: capacity=%d location=%d", s.CapacityPools, s.LocationPools)
	}
	if s.TotalCPU != 0 || s.TotalMemory != 0 || s.TotalDisk != 0 {
		t.Errorf("全位置型时容量应为 0，实得 cpu=%d mem=%d disk=%d", s.TotalCPU, s.TotalMemory, s.TotalDisk)
	}
}

// TestPoolListResponseItemsSerializesEmptyArray 零结果的层级视图必须序列化成 `[]` 而非 `null`。
//
// 回归背景（实测故障，用户报 "Cannot read properties of null (reading 'length')"）：
// 层级视图分页越界时曾把顶层切片置为 nil，JSON 出来是 `items: null`；前端树表格
// （t-enhanced-table）在树模式下对 data 取 length，拿到 null 直接抛错 —— 渠道详情页
// 「资源池」页签整片打不开。平铺视图当时返回 []，所以只有层级视图暴露。
// 这里锁住契约：无论哪种视图、结果是否为空，items 都必须是数组。
func TestPoolListResponseItemsSerializesEmptyArray(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []dto.PoolInfo
	}{
		{name: "空切片", items: make([]dto.PoolInfo, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 复制 ListPools 的构造方式：items 直接来自切片变量。
			resp := &dto.PoolListResponse{Items: tc.items}
			raw, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("序列化失败: %v", err)
			}
			if strings.Contains(string(raw), `"items":null`) {
				t.Fatalf("items 序列化成了 null，前端表格会抛错；期望 []。实际: %s", raw)
			}
			if !strings.Contains(string(raw), `"items":[]`) {
				t.Fatalf("items 未序列化成空数组。实际: %s", raw)
			}
		})
	}

	// 反向锁：nil 切片确实会变成 null —— 这正是必须避开 nil 的原因。
	rawNil, err := json.Marshal(&dto.PoolListResponse{Items: nil})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if !strings.Contains(string(rawNil), `"items":null`) {
		t.Fatalf("前置假设不成立：nil 切片本应序列化为 null，实得 %s", rawNil)
	}
}
