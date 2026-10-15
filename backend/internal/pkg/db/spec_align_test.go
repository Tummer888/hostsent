package db

// SKU 编码/名称对齐回填的单测（T4.7 配套）。
//
// 守的是「编码、名称、取值三者各说各话」这类数据不一致：
// 实测故障是商品 113 名下一条 general-2c4g-30g / 「通用型2核4G 30G」的规格，
// 实际取值却是 4核8G —— 商品名与客户买到的机器不是一回事。
//
// 这些用例同时钉住"不该改什么"：营销文案型名称、格式差异类编码都必须原样保留，
// 否则回填会在真实库上误改运营有意维护的商品名。

import "testing"

func TestSpecAlignTargetFixesContradiction(t *testing.T) {
	// 实测故障行：编码与名称都说 2核4G，取值是 4核8G。
	name, code := specAlignTarget("general-2c4g-30g", "通用型2核4G 30G", map[string]any{
		"compute.cpu": 4, "compute.memory": 8192,
	})
	if name != "4核8G" {
		t.Fatalf("名称应对齐为 4核8G，实际 %q", name)
	}
	if code != "general-4c8g" {
		t.Fatalf("编码应对齐为 general-4c8g，实际 %q", code)
	}
}

func TestSpecAlignTargetKeepsMarketingName(t *testing.T) {
	// 带营销文案的名称是运营有意起的商品名（含区域与机型信息），绝不能按取值覆盖。
	// 这里的取值恰好与名称里的 4核6G 一致，两种口径都要求不改动。
	name, code := specAlignTarget("dcimcloud-4c6g-100g", "香港|四区·轻量 8-8 9Mbps（4核6G）", map[string]any{
		"compute.cpu": 4, "compute.memory": 6144,
	})
	if name != "" {
		t.Fatalf("营销文案型名称不该被改写，实际得到 %q", name)
	}
	if code != "" {
		t.Fatalf("编码数字已对，不该被改写，实际得到 %q", code)
	}
}

func TestSpecAlignTargetKeepsConsistentNumbers(t *testing.T) {
	// 数字都对，只是编码少了磁盘段（历史格式差异）——不动，避免打断运营对编码的记忆。
	name, code := specAlignTarget("mfy-2c4g", "2核4G", map[string]any{
		"compute.cpu": 2, "compute.memory": 4096,
	})
	if name != "" || code != "" {
		t.Fatalf("数字一致的格式差异不该被改写，实际 name=%q code=%q", name, code)
	}
}

func TestSpecAlignTargetSkipsIncompleteSpecs(t *testing.T) {
	// 取值里没有 CPU/内存：推不出正确名称，宁可不动。
	if name, code := specAlignTarget("general-2c4g", "2核4G", map[string]any{
		"storage.system.size": 40,
	}); name != "" || code != "" {
		t.Fatalf("取值不完整时应跳过，实际 name=%q code=%q", name, code)
	}
	if name, code := specAlignTarget("general-2c4g", "2核4G", nil); name != "" || code != "" {
		t.Fatalf("取值为空时应跳过，实际 name=%q code=%q", name, code)
	}
}

func TestSpecAlignTargetHandlesStringValues(t *testing.T) {
	// 取值可能是字符串（flexString 那条链路写进来的），必须同样识别。
	name, code := specAlignTarget("general-2c4g-30g", "2核4G", map[string]any{
		"compute.cpu": "8", "compute.memory": "16384",
	})
	if name != "8核16G" || code != "general-8c16g" {
		t.Fatalf("字符串取值也应识别，实际 name=%q code=%q", name, code)
	}
}

func TestSpecAlignTargetSubGBMemoryClampsToOne(t *testing.T) {
	// 内存小于 1G（如 512MB）：按 1G 记，不能让 memGB 归零产生「4核0G」。
	name, _ := specAlignTarget("general-4c1g", "4核9G", map[string]any{
		"compute.cpu": 4, "compute.memory": 512,
	})
	if name != "4核1G" {
		t.Fatalf("不足 1G 应夹到 1G，实际 %q", name)
	}
}

func TestSpecAlignTargetNoFamilyPrefix(t *testing.T) {
	// 编码没有族前缀（整段都是配置）：仍要能改，且不能拼出前导横线。
	_, code := specAlignTarget("2c4g", "2核9G", map[string]any{
		"compute.cpu": 2, "compute.memory": 8192,
	})
	if code != "2c8g" {
		t.Fatalf("无族前缀时编码应为 2c8g，实际 %q", code)
	}
}

func TestSpecAlignTargetIsIdempotent(t *testing.T) {
	// 幂等：对已对齐的结果再跑一次，必须不再产生改动（回填每次启动都执行）。
	specs := map[string]any{"compute.cpu": 4, "compute.memory": 8192}
	name, code := specAlignTarget("general-2c4g-30g", "通用型2核4G 30G", specs)
	if name == "" || code == "" {
		t.Fatal("首轮应当产生修正")
	}
	name2, code2 := specAlignTarget(code, name, specs)
	if name2 != "" || code2 != "" {
		t.Fatalf("第二轮不该再改，实际 name=%q code=%q", name2, code2)
	}
}
