package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/model"
	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	"hostsent/backend/internal/pkg/specatom"
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

// fakeProviderReader 渠道读取替身：按 ID 返回固定 provider_type。
type fakeProviderReader struct{ typ string }

func (f *fakeProviderReader) FindByID(context.Context, uint64) (*providermodel.ResourceProvider, error) {
	if f.typ == "" {
		return nil, errors.New("not found")
	}
	return &providermodel.ResourceProvider{ProviderType: f.typ}, nil
}

// TestBuildProvisionRequestMergesSkuAtoms 自营商品：SKU 规格原子按平台字段字典翻译进开通参数，
// 且商品 ConfigOptions 的优先级高于 SKU 原子。
func TestBuildProvisionRequestMergesSkuAtoms(t *testing.T) {
	dict := specatom.NewDictionary()
	dict.Replace([]specatom.Atom{
		{Key: specatom.KeyCPU, ValueType: "int", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{"mofangyun": {Write: "cpu"}}},
		{Key: specatom.KeyMemory, ValueType: "int", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{"mofangyun": {Write: "memory"}}},
		{Key: specatom.KeyRegion, ValueType: "string", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{"mofangyun": {Write: "area"}}},
	})
	prev := specatom.Global()
	specatom.SetGlobal(dict)
	t.Cleanup(func() { specatom.SetGlobal(prev) })

	repo := &fakeProductRepo{}
	repo.findByID = &model.Product{
		ID: 7, SourceMode: model.SourceModeSelf, SourceProviderID: 1, PriceModel: model.PriceModelFixed,
		ConfigOptions: `{"cpu":8,"os":"centos7"}`,
	}
	svc := &productService{repo: repo, providerReader: &fakeProviderReader{typ: "mofangyun"}}
	spec := `{"compute.cpu":2,"compute.memory":4096,"placement.region":"hk"}`

	req, err := svc.BuildProvisionRequest(context.Background(), 7, "vm", spec, "")
	if err != nil {
		t.Fatalf("BuildProvisionRequest: %v", err)
	}
	if req.ConfigOptions["memory"] != 4096 {
		t.Errorf("SKU 原子应翻译为 memory，got %v", req.ConfigOptions["memory"])
	}
	if req.ConfigOptions["area"] != "hk" {
		t.Errorf("SKU 原子应翻译为 area，got %v", req.ConfigOptions["area"])
	}
	// ConfigOptions 覆盖 SKU 原子（cpu 8 而非 2）
	if req.ConfigOptions["cpu"] != float64(8) {
		t.Errorf("商品 ConfigOptions 应覆盖 SKU 原子 cpu，got %v", req.ConfigOptions["cpu"])
	}
}

// TestBuildProvisionRequestNoSpecKeepsLegacy 未选规格时行为与改造前一致（仅 ConfigOptions）。
func TestBuildProvisionRequestNoSpecKeepsLegacy(t *testing.T) {
	repo := &fakeProductRepo{}
	repo.findByID = &model.Product{ID: 8, SourceMode: model.SourceModeSelf, ConfigOptions: `{"cpu":4}`}
	svc := &productService{repo: repo, providerReader: &fakeProviderReader{typ: "mofangyun"}}
	req, err := svc.BuildProvisionRequest(context.Background(), 8, "vm", "", "")
	if err != nil {
		t.Fatalf("BuildProvisionRequest: %v", err)
	}
	if len(req.ConfigOptions) != 1 || req.ConfigOptions["cpu"] != float64(4) {
		t.Fatalf("无规格快照时不应注入额外参数，got %v", req.ConfigOptions)
	}
}

// TestValidateSpecJSON spec_atoms 校验：越界与未登记键被拒绝，字典为空时放行。
func TestValidateSpecJSON(t *testing.T) {
	dict := specatom.NewDictionary()
	dict.Replace([]specatom.Atom{
		{Key: specatom.KeyCPU, ValueType: "int", MinValue: ptrFloat(1), Status: 1},
	})
	prev := specatom.Global()
	specatom.SetGlobal(dict)
	t.Cleanup(func() { specatom.SetGlobal(prev) })

	if err := validateSpecJSON(`{"compute.cpu":2}`); err != nil {
		t.Errorf("合法规格被拒: %v", err)
	}
	if err := validateSpecJSON(`{"compute.cpu":0.5}`); err == nil {
		t.Error("低于下限的取值应被拒绝")
	}
	if err := validateSpecJSON(`{"unknown.key":1}`); err == nil {
		t.Error("未登记原子应被拒绝")
	}
	if err := validateSpecJSON(`not-json`); err == nil {
		t.Error("非法 JSON 应被拒绝")
	}
	if err := validateSpecJSON(""); err != nil {
		t.Errorf("空规格应放行: %v", err)
	}
}

func ptrFloat(v float64) *float64 { return &v }

// fakeBindingReader SKU 平台绑定替身（T4.2/T4.6）。
type fakeBindingReader struct {
	status map[uint64]string
	params map[uint64]string
}

func (f *fakeBindingReader) BindingStatusByProductSpecs(context.Context, []uint64) (map[uint64]string, error) {
	return f.status, nil
}

func (f *fakeBindingReader) ConfirmedPlatformParamsByProductSpec(_ context.Context, id uint64) (string, error) {
	return f.params[id], nil
}

// TestValidatePublishSelfRequiresSkuAndBinding 自营商品上架门禁（T4.6）：
// 无启用 SKU → 拒绝；SKU 绑定未确认 → 拒绝；全部 confirmed → 放行。
func TestValidatePublishSelfRequiresSkuAndBinding(t *testing.T) {
	// 字典：只要求 cpu，且 mofangyun 有写字段，避免平台字段过滤把用例打偏。
	dict := specatom.NewDictionary()
	dict.Replace([]specatom.Atom{
		{Key: specatom.KeyCPU, ValueType: "int", Required: true, AppliesTo: "both", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{"mofangyun": {Write: "cpu"}}},
	})
	prev := specatom.Global()
	specatom.SetGlobal(dict)
	t.Cleanup(func() { specatom.SetGlobal(prev) })

	item := &model.Product{ID: 40, SourceMode: model.SourceModeSelf, SourceProviderID: 1}

	// 1) 无 SKU
	repo := &fakeProductRepo{findByID: item}
	svc := &productService{repo: repo, providerReader: &fakeProviderReader{typ: "mofangyun"}, bindingReader: &fakeBindingReader{}}
	if err := svc.validatePublish(context.Background(), item); err == nil {
		t.Fatal("无启用 SKU 时应拒绝上架")
	}

	// 2) 有 SKU 但绑定未确认
	repo = &fakeProductRepo{findByID: item, specs: []model.ProductSpec{
		{ID: 501, ProductID: 40, SpecCode: "s1", Status: model.ProductSpecEnabled, Specs: `{"compute.cpu":2}`},
	}}
	svc = &productService{repo: repo, providerReader: &fakeProviderReader{typ: "mofangyun"},
		bindingReader: &fakeBindingReader{status: map[uint64]string{501: specbindingAutoMapped}}}
	if err := svc.validatePublish(context.Background(), item); err == nil {
		t.Fatal("SKU 绑定未确认时应拒绝上架")
	}

	// 3) 全部 confirmed → 放行
	svc = &productService{repo: repo, providerReader: &fakeProviderReader{typ: "mofangyun"},
		bindingReader: &fakeBindingReader{status: map[uint64]string{501: specbindingConfirmed}}}
	if err := svc.validatePublish(context.Background(), item); err != nil {
		t.Fatalf("绑定已确认应放行，got %v", err)
	}
}

// TestValidatePublishSelfRejectsMissingRequiredAtom 必填原子缺失时拒绝；
// 且写侧为 "-" 的原子不参与平台必填校验（billing.cycle 类）。
func TestValidatePublishSelfRejectsMissingRequiredAtom(t *testing.T) {
	dict := specatom.NewDictionary()
	dict.Replace([]specatom.Atom{
		{Key: specatom.KeyCPU, ValueType: "int", Required: true, AppliesTo: "both", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{"mofangyun": {Write: "cpu"}}},
		{Key: "billing.cycle", ValueType: "enum", Required: true, AppliesTo: "both", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{"mofangyun": {Write: "-"}}},
	})
	prev := specatom.Global()
	specatom.SetGlobal(dict)
	t.Cleanup(func() { specatom.SetGlobal(prev) })

	item := &model.Product{ID: 41, SourceMode: model.SourceModeSelf, SourceProviderID: 1}
	repo := &fakeProductRepo{findByID: item, specs: []model.ProductSpec{
		{ID: 502, ProductID: 41, SpecCode: "s-bad", Status: model.ProductSpecEnabled, Specs: `{"compute.memory":4096}`},
	}}
	svc := &productService{repo: repo, providerReader: &fakeProviderReader{typ: "mofangyun"},
		bindingReader: &fakeBindingReader{status: map[uint64]string{502: specbindingConfirmed}}}
	err := svc.validatePublish(context.Background(), item)
	if err == nil || !strings.Contains(err.Error(), specatom.KeyCPU) {
		t.Fatalf("缺 cpu 必填项时应拒绝并指明原子，got %v", err)
	}
	if strings.Contains(err.Error(), "billing.cycle") {
		t.Fatalf("写侧为 '-' 的原子不应参与必填校验，got %v", err)
	}
}

// TestValidatePublishUpstreamRequiresSourceProduct 代理商品未绑定上游商品时拒绝上架。
func TestValidatePublishUpstreamRequiresSourceProduct(t *testing.T) {
	item := &model.Product{ID: 42, SourceMode: model.SourceModeUpstream}
	svc := &productService{repo: &fakeProductRepo{findByID: item}}
	if err := svc.validatePublish(context.Background(), item); err == nil {
		t.Fatal("代理商品缺 source_product_id 时应拒绝上架")
	}
}

// TestValidatePublishUpstreamPassthroughBypass 「仅透传」标记可放行代理商品。
func TestValidatePublishUpstreamPassthroughBypass(t *testing.T) {
	item := &model.Product{ID: 43, SourceMode: model.SourceModeUpstream, SourceProductID: 9001, SpecPassthrough: true}
	svc := &productService{repo: &fakeProductRepo{findByID: item}}
	if err := svc.validatePublish(context.Background(), item); err != nil {
		t.Fatalf("仅透传应放行，got %v", err)
	}
}

// TestValidateConfigGroupSources 配置项来源校验（T4.4）。
func TestValidateConfigGroupSources(t *testing.T) {
	ok := []interface{}{
		map[string]any{"name": "g", "options": []any{
			map[string]any{"option_name": "node", "source": "self", "source_key": "node"},
			map[string]any{"option_name": "cpu", "upstream_id": 12},
		}},
	}
	if err := validateConfigGroupSources(ok); err != nil {
		t.Fatalf("合法配置项被拒: %v", err)
	}
	badSource := []interface{}{
		map[string]any{"name": "g", "options": []any{map[string]any{"option_name": "x", "source": "bogus"}}},
	}
	if err := validateConfigGroupSources(badSource); err == nil {
		t.Error("非法 source 应被拒绝")
	}
	selfNoKey := []interface{}{
		map[string]any{"name": "g", "options": []any{map[string]any{"source": "self"}}},
	}
	if err := validateConfigGroupSources(selfNoKey); err == nil {
		t.Error("自营配置项缺平台参数名应被拒绝")
	}
}

// TestNormalizeMarkupType 加价类型归一（T4.3）。
func TestNormalizeMarkupType(t *testing.T) {
	cases := map[string]string{"": "", "none": "", "percent": "percent", "PERCENT": "percent", "fixed": "fixed"}
	for in, want := range cases {
		got, err := normalizeMarkupType(in)
		if err != nil || got != want {
			t.Errorf("normalizeMarkupType(%q) = (%q,%v), want %q", in, got, err, want)
		}
	}
	if _, err := normalizeMarkupType("bogus"); err == nil {
		t.Error("未知加价类型应报错")
	}
}
