package service

import (
	"testing"

	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
)

// TestBuildCloneBaseOptions 验证克隆商品开通参数映射正确：
// 上游资源商品规格应转换为魔方云 /clouds 可消费的配置键（cpu/memory/system_disk_size/os/area/node/bw）。
func TestBuildCloneBaseOptions(t *testing.T) {
	rp := &resourceproductmodel.ResourceProduct{
		CPU:       4,
		Memory:    8192,
		Disk:      100,
		DiskType:  "ssd",
		Bandwidth: 20,
		OS:        "centos7",
		Region:    "香港",
		Zone:      "一区",
		Specs:     `{"gpu_num":1,"ip_num":2,"extra_flag":"x"}`,
	}
	opts := buildCloneBaseOptions(rp)

	cases := []struct {
		key  string
		want interface{}
	}{
		{"cpu", 4},
		{"memory", 8192},
		{"system_disk_size", 100},
		{"disk_type", "ssd"},
		{"bw", 20},
		{"os", "centos7"},
		{"area", "香港"},
		{"node", "一区"},
	}
	for _, c := range cases {
		if got, ok := opts[c.key]; !ok || got != c.want {
			t.Errorf("buildCloneBaseOptions[%q] = %v (ok=%v), want %v", c.key, got, ok, c.want)
		}
	}

	// mergeSpecJSON 应补齐 opts 中缺失的键，但不覆盖显式键
	if _, exists := opts["gpu_num"]; !exists {
		t.Errorf("buildCloneBaseOptions 应从 Specs 合并 gpu_num，但缺失")
	}
	if opts["cpu"] != 4 {
		t.Errorf("mergeSpecJSON 不应覆盖显式键 cpu")
	}
}

// TestBuildCloneBaseOptionsNoSpecs 验证规格全空时返回空映射（不 panic，由适配器兜底）。
func TestBuildCloneBaseOptionsNoSpecs(t *testing.T) {
	rp := &resourceproductmodel.ResourceProduct{}
	opts := buildCloneBaseOptions(rp)
	if len(opts) != 0 {
		t.Errorf("全空规格应返回空映射，得到 %v", opts)
	}
}

// TestMergeSpecJSON 验证规格 JSON 合并与优先级。
func TestMergeSpecJSON(t *testing.T) {
	opts := map[string]interface{}{"cpu": 4}
	mergeSpecJSON(opts, `{"cpu":8,"memory":4096,"os":"ubuntu"}`)
	if opts["cpu"] != 4 {
		t.Errorf("已存在的键不应被覆盖，cpu=%v", opts["cpu"])
	}
	if opts["memory"] != float64(4096) {
		t.Errorf("缺失键应合并，memory=%v", opts["memory"])
	}
	if opts["os"] != "ubuntu" {
		t.Errorf("缺失键应合并，os=%v", opts["os"])
	}
}
