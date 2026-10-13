package service

import (
	"context"
	"encoding/json"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
)

// 对接商品（source_mode=upstream）的配置锁定与上游信息（2026-10 上游对接修复）。
//
// 背景：上游商品是固定规格（如四核8G/50Mbps），本地改配置必然与上游商品不符 → 开通报错。
// 因此锁定项（上游只有一个可见取值）在履约侧必须拒绝一切本地覆盖。

// upstreamGroupsFixture 上游 config_groups 形态：固定项（cpu/memory/bw）+ 可选项（os/network_type）。
const upstreamGroupsFixture = `[
  {"name":"绍兴电信16C-16G-50M配置项组","options":[
    {"id":217855,"option_name":"cpu|CPU","option_type":6,"sub":[
      {"id":838666,"option_name":"16|16核","hidden":0}]},
    {"id":217856,"option_name":"memory|内存","option_type":8,"sub":[
      {"id":838667,"option_name":"16384|16G","hidden":0}]},
    {"id":217857,"option_name":"bw|带宽","option_type":13,"sub":[
      {"id":838668,"option_name":"50|50Mbps","hidden":0}]},
    {"id":217859,"option_name":"os|操作系统","option_type":5,"sub":[
      {"id":838670,"option_name":"12|CentOS","hidden":0},
      {"id":838671,"option_name":"15|Debian","hidden":0}]},
    {"id":217860,"option_name":"network_type|网络类型","option_type":6,"sub":[
      {"id":838697,"option_name":"vpc|VPC网络","hidden":0},
      {"id":838698,"option_name":"normal|经典网络","hidden":0}]}
  ]}
]`

// TestUpstreamOptionLocksFromRaw 验证锁定判定：单一可见取值=锁定，多取值=可选，self 项不参与。
func TestUpstreamOptionLocksFromRaw(t *testing.T) {
	var groups []upstreamConfigGroup
	if err := json.Unmarshal([]byte(upstreamGroupsFixture), &groups); err != nil {
		t.Fatalf("解析 config_groups 失败: %v", err)
	}
	locks := upstreamOptionLocksFromRaw(groups)
	got := map[string]bool{}
	for _, l := range locks {
		got[l.Key] = l.Locked
	}
	want := map[string]bool{"cpu": true, "memory": true, "bw": true, "os": false, "network_type": false}
	for key, locked := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("配置项 %s 未出现在锁定清单", key)
			continue
		}
		if got[key] != locked {
			t.Errorf("配置项 %s locked=%v, want %v", key, got[key], locked)
		}
	}
	// 锁定项要能展示唯一取值（详情页只读展示用）。
	for _, l := range locks {
		if l.Key == "cpu" && l.Value != "16|16核" {
			t.Errorf("cpu 锁定取值 = %q, want 16|16核", l.Value)
		}
	}
}

// TestStripLockedUpstreamOptions 验证履约守卫：锁定配置项的本地覆盖被剔除，
// 可选配置项（os/network_type）与开通选项（hostname）保留。
func TestStripLockedUpstreamOptions(t *testing.T) {
	locks := []dto.ProductUpstreamOption{
		{Key: "cpu", Locked: true},
		{Key: "memory", Locked: true},
		{Key: "bw", Locked: true},
		{Key: "system_disk_size", Locked: true},
		{Key: "os", Locked: false},
	}
	opts := map[string]interface{}{
		"cpu":               2,          // 被本地改小 → 必须剔除
		"memory":            4096,       // 同上
		"bw":                100,        // 同上
		"disk":              20,         // 系统盘的上游别名（StandardProductSpec 口径）
		"os":                "debian",   // 可选值 → 保留
		"hostname":          "hs-abc12", // 开通选项 → 保留
		"config_groups":     "[...]",    // 上游配置组本体 → 保留
		"upstream_pid":      "468",      // 上游商品 ID → 保留
		"spec_code_unknown": 1,          // 未知键不属于任何锁定项 → 保留（不在守卫职责内）
	}
	stripped := stripLockedUpstreamOptions(opts, locks)
	for _, key := range []string{"cpu", "memory", "bw", "disk"} {
		if _, ok := opts[key]; ok {
			t.Errorf("锁定项别名 %s 未被剔除", key)
		}
	}
	for _, key := range []string{"os", "hostname", "config_groups", "upstream_pid", "spec_code_unknown"} {
		if _, ok := opts[key]; !ok {
			t.Errorf("非锁定项 %s 被误删", key)
		}
	}
	if len(stripped) != 4 {
		t.Errorf("剔除键数 = %d (%v), want 4", len(stripped), stripped)
	}
}

// TestUpstreamRenewable 验证可续费判定：仅周期性计费可续费。
func TestUpstreamRenewable(t *testing.T) {
	cases := map[string]bool{
		`{"pay_type":"recurring"}`: true,
		`{"pay_type":"onetime"}`:   false,
		`{"pay_type":"free"}`:      false,
		``:                         true, // 拿不到 pay_type 时不误禁（交上游回绝）
	}
	for raw, want := range cases {
		got, note := upstreamRenewable(raw)
		if got != want {
			t.Errorf("upstreamRenewable(%q) = %v, want %v", raw, got, want)
		}
		if !got && note == "" {
			t.Errorf("不可续费时应给出原因说明（%q）", raw)
		}
	}
}

// TestUpstreamCycles 验证双价周期解析：字符串/数字都能解，-1（未配置）与 0（免费）不进可售周期。
func TestUpstreamCycles(t *testing.T) {
	rows := []map[string]interface{}{
		{"monthly": "76.30", "quarterly": 228.9, "onetime": -1, "semiannually": "0.00", "monthly_list": "109.00"},
	}
	got := upstreamCycles(rows)
	if got["monthly"] != 76.30 || got["quarterly"] != 228.9 {
		t.Errorf("周期解析错误: %v", got)
	}
	if _, ok := got["onetime"]; ok {
		t.Errorf("onetime=-1（未配置）不应作为可售周期: %v", got)
	}
	if _, ok := got["semiannually"]; ok {
		t.Errorf("semiannually=0（免费）不应作为可售周期: %v", got)
	}
}

// TestUpstreamInfoForNonUpstream 自营商品不产出上游信息（详情页不展示上游页签）。
func TestUpstreamInfoForNonUpstream(t *testing.T) {
	svc := &productService{repo: &fakeProductRepo{}}
	info := svc.upstreamInfoFor(context.Background(), &model.Product{ID: 1, SourceMode: model.SourceModeSelf})
	if info != nil {
		t.Fatalf("自营商品不应有上游信息: %+v", info)
	}
}

// TestUpstreamInfoForUpstream 对接商品产出完整上游信息（分类/双价/折扣/锁定/可续费）。
func TestUpstreamInfoForUpstream(t *testing.T) {
	rp := &resourceproductmodel.ResourceProduct{
		ID: 8, ProviderID: 7, UpstreamID: "468", Name: "绍兴电信|特惠·标准 16-16 50Mbps",
		GroupID: 14, GroupName: "常驻 | 活动", CostPrice: 76.3, SalePrice: 109,
		RawSpecs: `{"gid":14,"group_name":"常驻 | 活动","description":"<div>16核16G</div>",
		  "pay_type":"{\"pay_type\":\"recurring\"}",
		  "product_pricings":[{"monthly":"76.30","quarterly":"228.90"}],
		  "product_pricings_list":[{"monthly":"109.00","quarterly":"327.00"}],
		  "upstream_discount":{"type":1,"bates":70,"desc":"代理折扣 70%（应付标价的 70%）"},
		  "config_groups":` + upstreamGroupsFixture + `}`,
	}
	repo := &fakeProductRepo{
		findByID: &model.Product{ID: 149, SourceMode: model.SourceModeUpstream, SourceProductID: 8, SourceProviderID: 7},
	}
	svc := &productService{repo: repo, resourceReader: fakeResourceReader{rp: rp}}
	info := svc.upstreamInfoFor(context.Background(), repo.findByID)
	if info == nil {
		t.Fatal("对接商品应产出上游信息")
	}
	if info.GroupName != "常驻 | 活动" || info.UpstreamID != "468" {
		t.Errorf("分类/上游商品 ID 错误: group=%q upstream=%q", info.GroupName, info.UpstreamID)
	}
	if info.ListPrice != 109 || info.CostPrice != 76.3 {
		t.Errorf("双价口径错误: list=%v cost=%v", info.ListPrice, info.CostPrice)
	}
	if info.DiscountDesc == "" {
		t.Error("折扣描述缺失（详情页无法说明代理价来源）")
	}
	if info.ListCycles["monthly"] != 109 || info.CostCycles["monthly"] != 76.3 {
		t.Errorf("周期双价错误: list=%v cost=%v", info.ListCycles, info.CostCycles)
	}
	if !info.Renewable || info.RenewNote != "" {
		t.Errorf("周期性商品应可续费: renewable=%v note=%q", info.Renewable, info.RenewNote)
	}
	if info.FixedConfig {
		t.Error("存在多取值配置项（os/network_type）时不应判定为整机固定配置")
	}
	if len(info.Options) != 5 {
		t.Errorf("配置项清单数量 = %d, want 5", len(info.Options))
	}
	if info.Description == "" {
		t.Error("上游描述未透传（详情页无内容可展示）")
	}
}

// fakeResourceReader 上游资源商品读取（测试替身）。
type fakeResourceReader struct {
	rp *resourceproductmodel.ResourceProduct
}

func (f fakeResourceReader) FindByID(context.Context, uint64) (*resourceproductmodel.ResourceProduct, error) {
	return f.rp, nil
}

// TestBuildProvisionRequestStripsLockedOverrides 端到端验证履约守卫：
// 商品级可配置项（config_options）里改了上游锁定项（cpu）→ 不下发；
// 未锁定项（os）与开通选项（hostname）照常下发；上游商品 ID 与配置组必须带上。
func TestBuildProvisionRequestStripsLockedOverrides(t *testing.T) {
	var groups []interface{}
	if err := json.Unmarshal([]byte(upstreamGroupsFixture), &groups); err != nil {
		t.Fatalf("解析 config_groups 失败: %v", err)
	}
	rp := &resourceproductmodel.ResourceProduct{
		ID: 8, ProviderID: 7, UpstreamID: "468", Name: "绍兴电信 16-16",
		CPU: 16, Memory: 16384, Disk: 50, Bandwidth: 50, OS: "centos",
		RawSpecs: `{"gid":14,"group_name":"常驻 | 活动","config_groups":` + upstreamGroupsFixture + `}`,
	}
	repo := &fakeProductRepo{
		findByID: &model.Product{
			ID: 149, Name: "绍兴电信 16-16", SourceMode: model.SourceModeUpstream,
			SourceProductID: 8, SourceProviderID: 7, PriceModel: "monthly",
			// 运营在商品编辑里把 CPU 改成了 2 核、系统换 Debian，并填了主机名：
			// 前两项属于上游固定配置（必须被守卫剔除），后两项应保留。
			ConfigOptions: `{"cpu":2,"os":"debian","hostname":"hs-myvm01"}`,
		},
		configGroups: groups,
	}
	svc := &productService{repo: repo, resourceReader: fakeResourceReader{rp: rp}}

	req, err := svc.BuildProvisionRequest(context.Background(), 149, "hs-myvm01", "", "", nil)
	if err != nil {
		t.Fatalf("BuildProvisionRequest 失败: %v", err)
	}
	if req.ProviderID != 7 {
		t.Errorf("ProviderID = %d, want 7", req.ProviderID)
	}
	if got := req.ConfigOptions["upstream_pid"]; got != "468" {
		t.Errorf("upstream_pid = %v, want 468（上游下单必需）", got)
	}
	if _, ok := req.ConfigOptions["config_groups"]; !ok {
		t.Error("config_groups 缺失（上游配置项无法下发）")
	}
	if _, ok := req.ConfigOptions["cpu"]; ok {
		t.Errorf("上游锁定的 cpu 被本地覆盖值带下去了: %v", req.ConfigOptions["cpu"])
	}
	if got := req.ConfigOptions["os"]; got != "debian" {
		t.Errorf("未锁定的 os 应保留本地取值, got %v", got)
	}
	if got := req.ConfigOptions["hostname"]; got != "hs-myvm01" {
		t.Errorf("开通选项 hostname 应保留, got %v", got)
	}
	if len(req.StrippedLockedKeys) == 0 {
		t.Error("被剔除的锁定键应留痕（StrippedLockedKeys）")
	}
}

// TestBuildProvisionRequestUpstreamSelectionOverride 端到端验证：对接商品的客户选配
// 会以 configoption[上游选项id]=上游取值id 的形式显式下发给适配器。
//
// 适配器按 config_groups 还原映射时靠本地词汇猜上游子项，网络类型这类项猜不到；
// 显式映射是「客户选了什么就一定开通什么」的保证。
func TestBuildProvisionRequestUpstreamSelectionOverride(t *testing.T) {
	// 上游配置组：cpu（单值固定）+ os/network_type（多值可选），id 即上游选项 id。
	groups := []interface{}{map[string]interface{}{
		"options": []interface{}{
			map[string]interface{}{
				"id": float64(163749), "option_name": "cpu|CPU", "source": "upstream",
				"sub": []interface{}{map[string]interface{}{"id": float64(565658), "option_name": "4|4核", "hidden": 0}},
			},
			map[string]interface{}{
				"id": float64(163751), "option_name": "os|操作系统", "source": "upstream",
				"sub": []interface{}{
					map[string]interface{}{"id": float64(565660), "option_name": "12|CentOS^CentOS-7.6.1810-x64", "hidden": 0},
					map[string]interface{}{"id": float64(565664), "option_name": "75|Debian^Debian-12.0_x64", "hidden": 0},
				},
			},
			map[string]interface{}{
				"id": float64(163744), "option_name": "network_type|网络类型", "source": "upstream",
				"sub": []interface{}{
					map[string]interface{}{"id": float64(565652), "option_name": "vpc|VPC网络", "hidden": 0},
					map[string]interface{}{"id": float64(565653), "option_name": "normal|经典网络", "hidden": 0},
				},
			},
		},
	}}
	repo := &fakeProductRepo{
		findByID: &model.Product{
			ID: 176, Name: "香港轻量", SourceMode: model.SourceModeUpstream,
			SourceProductID: 120, SourceProviderID: 7, PriceModel: "monthly",
		},
		configGroups: groups,
	}
	rp := &resourceproductmodel.ResourceProduct{
		ID: 120, ProviderID: 7, UpstreamID: "528", Name: "香港轻量",
		RawSpecs: `{"group_name":"香港六区"}`,
	}
	svc := &productService{repo: repo, resourceReader: fakeResourceReader{rp: rp}}

	req, err := svc.BuildProvisionRequest(context.Background(), 176, "hs-node1", "", "",
		map[string]string{"163751": "565664", "163744": "565653", "hostname": "hs-node1"})
	if err != nil {
		t.Fatalf("BuildProvisionRequest: %v", err)
	}
	override, ok := req.ConfigOptions["configoption"].(map[string]interface{})
	if !ok {
		t.Fatalf("应下发显式 configoption 映射，实际 %#v", req.ConfigOptions["configoption"])
	}
	if override["163751"] != "565664" || override["163744"] != "565653" {
		t.Errorf("选配未翻译为上游 id 映射：%+v", override)
	}
	if _, ok := override["163749"]; ok {
		t.Errorf("上游单值固定项不应出现在覆盖映射里：%+v", override)
	}
	// 主机名（source=self 的文本项）必须原样透传：上游按 extra.hostname 取客户自定名，
	// 丢了就退回 defaultHostname 按商品名生成 —— 界面预填的名字与实际开出的不一致。
	if got := req.ConfigOptions["hostname"]; got != "hs-node1" {
		t.Errorf("主机名应原样透传为 %q，实际 %#v", "hs-node1", got)
	}
}
