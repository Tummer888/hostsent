package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
)

type fakeProductRepo struct {
	items    []model.Product
	updates  []model.Product
	history  []model.ProductHistory
	findByID *model.Product
	// specs 供 ListSpecs 返回（T4.2 门禁用例）；specByCode 供 FindSpecByCode 命中。
	specs      []model.ProductSpec
	specByCode map[string]*model.ProductSpec
	// configOptions 供 SelfConfigOptionRows 返回（T4.5 选配项用例）。
	configOptions []*model.ProductConfigOption
	// configGroups 供 ConfigGroupsByProductID / AllConfigGroupsByProductID 返回（履约守卫用例）。
	configGroups []interface{}
}

func (f *fakeProductRepo) List(context.Context, dto.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (f *fakeProductRepo) FindByID(_ context.Context, id uint64) (*model.Product, error) {
	// 先按 ID 在 items 里找（贴近真实仓库：批量操作按 ID 回查名称）；
	// 未命中时退回 findByID 显式桩，最后才是 not found。
	for i := range f.items {
		if f.items[i].ID == id {
			item := f.items[i]
			return &item, nil
		}
	}
	if f.findByID == nil {
		return nil, errors.New("not found")
	}
	return f.findByID, nil
}
func (f *fakeProductRepo) Create(context.Context, *model.Product) error { return nil }
func (f *fakeProductRepo) Update(_ context.Context, item *model.Product) error {
	f.updates = append(f.updates, *item)
	// 与真实库一致：写入要能被后续读取看到（ApplyUpstreamMarkupRule 先写规则再重算售价）。
	for i := range f.items {
		if f.items[i].ID == item.ID {
			f.items[i] = *item
			break
		}
	}
	return nil
}
func (f *fakeProductRepo) Delete(context.Context, uint64) error { return nil }
func (f *fakeProductRepo) ListSpecs(context.Context, uint64) ([]model.ProductSpec, error) {
	return f.specs, nil
}
func (f *fakeProductRepo) AddHistory(_ context.Context, h *model.ProductHistory) error {
	f.history = append(f.history, *h)
	return nil
}
func (f *fakeProductRepo) ListHistory(context.Context, uint64) ([]model.ProductHistory, error) {
	return nil, nil
}
func (f *fakeProductRepo) ListBySourceProductID(context.Context, uint64) ([]model.Product, error) {
	return f.items, nil
}
func (f *fakeProductRepo) ListImportedRefs(_ context.Context, ids []uint64) (map[uint64]model.Product, error) {
	out := make(map[uint64]model.Product, len(ids))
	for _, item := range f.items {
		out[item.SourceProductID] = item
	}
	return out, nil
}
func (f *fakeProductRepo) SaveConfigOptions(context.Context, uint64, []interface{}) error { return nil }
func (f *fakeProductRepo) SaveSelfOptionRows(context.Context, uint64, []*model.ProductConfigOption) error {
	return nil
}
func (f *fakeProductRepo) ConfigGroupsByProductID(context.Context, uint64) ([]interface{}, error) {
	return f.configGroups, nil
}
func (f *fakeProductRepo) FindSpecByCode(_ context.Context, _ uint64, code string) (*model.ProductSpec, error) {
	if sp, ok := f.specByCode[code]; ok {
		return sp, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeProductRepo) FindSpecByID(context.Context, uint64) (*model.ProductSpec, error) {
	return nil, errors.New("not found")
}
func (f *fakeProductRepo) CreateSpec(context.Context, *model.ProductSpec) error { return nil }
func (f *fakeProductRepo) UpdateSpec(context.Context, *model.ProductSpec) error { return nil }
func (f *fakeProductRepo) DeleteSpec(context.Context, uint64) error             { return nil }
func (f *fakeProductRepo) DecrementSpecStock(context.Context, uint64, int) (int64, error) {
	return 1, nil
}
func (f *fakeProductRepo) IncrementSpecStock(context.Context, uint64, int) (int64, error) {
	return 1, nil
}
func (f *fakeProductRepo) SelfConfigParams(context.Context, uint64) (map[string]string, error) {
	return nil, nil
}
func (f *fakeProductRepo) SelfConfigOptionRows(context.Context, uint64) ([]*model.ProductConfigOption, error) {
	return f.configOptions, nil
}
func (f *fakeProductRepo) AllConfigGroupsByProductID(context.Context, uint64) ([]interface{}, error) {
	return f.configGroups, nil
}

// TestApplyConfirmedPriceKeepsSalePriceWithoutMarkup 无加价规则时只更新成本，售价不变。
func TestApplyConfirmedPriceKeepsSalePriceWithoutMarkup(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{{ID: 19, Price: 180, CostPrice: 100, SourceProductID: 28975}}}
	svc := &productService{repo: repo}
	n, err := svc.ApplyConfirmedPrice(context.Background(), 28975, 150, "admin", "")
	if err != nil {
		t.Fatalf("ApplyConfirmedPrice: %v", err)
	}
	if n != 1 {
		t.Fatalf("affected = %d, want 1", n)
	}
	if got := repo.updates[0].Price; math.Abs(got-180) > 0.0001 {
		t.Errorf("售价应保持不变 = %v, want 180", got)
	}
	if got := repo.updates[0].CostPrice; math.Abs(got-150) > 0.0001 {
		t.Errorf("成本价 = %v, want 150", got)
	}
	if len(repo.history) != 1 || repo.history[0].ChangeType != model.ChangeTypePrice {
		t.Fatalf("应写入一条调价历史，got %+v", repo.history)
	}
}

// TestApplyConfirmedPriceRecomputesWithMarkup 配置加价规则时按规则重算售价。
func TestApplyConfirmedPriceRecomputesWithMarkup(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{
		{ID: 19, Price: 180, CostPrice: 100, UpstreamMarkupType: "percent", UpstreamMarkupValue: 150},
	}}
	svc := &productService{repo: repo}
	if _, err := svc.ApplyConfirmedPrice(context.Background(), 28975, 150, "admin", ""); err != nil {
		t.Fatalf("ApplyConfirmedPrice: %v", err)
	}
	if got := repo.updates[0].Price; math.Abs(got-225) > 0.0001 {
		t.Errorf("percent 150%% 售价 = %v, want 225", got)
	}
}

// TestApplyConfirmedPriceNoBinding 无关联售出商品时空操作，不报错。
func TestApplyConfirmedPriceNoBinding(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := &productService{repo: repo}
	n, err := svc.ApplyConfirmedPrice(context.Background(), 999, 10, "", "")
	if err != nil || n != 0 {
		t.Fatalf("无关联商品应返回 (0,nil)，got (%d,%v)", n, err)
	}
}

// TestApplyMarkupWithListPercent 标价口径加价：售价 = 上游标价 × value/100。
func TestApplyMarkupWithListPercent(t *testing.T) {
	// 标价 180 / 代理价 126：100% 按上游原价卖，120% 加价 20%。
	cost, list := 126.0, 180.0
	cases := []struct {
		typ     string
		value   float64
		want    float64
		changed bool
	}{
		{model.MarkupTypeListPercent, 100, 180, true},
		{model.MarkupTypeListPercent, 120, 216, true},
		{model.MarkupTypeListPercent, 80, 144, true},
		{model.MarkupTypePercent, 100, 126, true},
		{model.MarkupTypeFixed, 30, 156, true},
		{"", 100, 0, false},
	}
	for _, c := range cases {
		got, changed := applyMarkupWithList(c.typ, c.value, cost, list)
		if changed != c.changed || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("applyMarkupWithList(%q,%v) = (%v,%v), want (%v,%v)", c.typ, c.value, got, changed, c.want, c.changed)
		}
	}
	// 标价缺失时退化为成本口径，避免算出 0 元售价。
	if got, _ := applyMarkupWithList(model.MarkupTypeListPercent, 120, cost, 0); math.Abs(got-151.2) > 1e-9 {
		t.Errorf("缺标价时 = %v, want 151.2 (按代理价 x120%%)", got)
	}
	if !IsValidMarkupType("list_percent") || !IsValidMarkupType("") || IsValidMarkupType("bogus") {
		t.Error("IsValidMarkupType 校验不符合预期")
	}
}

// TestParseUpstreamCyclePrices 双基线解析：成本取折后价，标价取未折价行。
func TestParseUpstreamCyclePrices(t *testing.T) {
	raw := `{
		"product_pricings": [{"monthly": "126.00", "annually": "-1.00", "onetime": "-1.00"}],
		"product_pricings_list": [{"monthly": "180.00", "annually": "-1.00", "onetime": "-1.00"}]
	}`
	got := parseUpstreamCyclePrices(raw)
	if cost := got.costs["monthly"]; cost != 126 {
		t.Errorf("月度成本 = %v, want 126", cost)
	}
	if list := got.lists["monthly"]; list != 180 {
		t.Errorf("月度标价 = %v, want 180", list)
	}
	if got.listHeadline() != 180 {
		t.Errorf("代表标价 = %v, want 180", got.listHeadline())
	}
	// 无标价行（历史数据）时不臆造，listHeadline 为 0。
	legacy := parseUpstreamCyclePrices(`{"product_pricings":[{"monthly":"126.00"}]}`)
	if legacy.listHeadline() != 0 {
		t.Errorf("无标价行时 listHeadline = %v, want 0", legacy.listHeadline())
	}
	if legacy.costs["monthly"] != 126 {
		t.Errorf("无标价行时成本 = %v, want 126", legacy.costs["monthly"])
	}
}

// TestApplyUpstreamMarkupRule 调价：规则写入已导入商品并按规则重算售价。
func TestApplyUpstreamMarkupRule(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{{ID: 21, Price: 126, CostPrice: 126, SourceProductID: 28975}}}
	svc := &productService{repo: repo}
	svc.resourceReader = fakeResourceReader{rp: &resourceproductmodel.ResourceProduct{
		ProviderID: 7, UpstreamID: "468", CostPrice: 126, SalePrice: 180,
		RawSpecs: `{"product_pricings":[{"monthly":"126.00"}],"product_pricings_list":[{"monthly":"180.00"}]}`,
	}}
	n, err := svc.ApplyUpstreamMarkupRule(context.Background(), 28975, model.MarkupTypeListPercent, 100, "admin", "")
	if err != nil {
		t.Fatalf("ApplyUpstreamMarkupRule: %v", err)
	}
	if n != 1 {
		t.Fatalf("affected = %d, want 1", n)
	}
	// 最后一次 Update 是加价规则落库，规则要写进商品。
	if len(repo.updates) == 0 {
		t.Fatal("未写入商品")
	}
	last := repo.updates[len(repo.updates)-1]
	if last.UpstreamMarkupType != model.MarkupTypeListPercent || last.UpstreamMarkupValue != 100 {
		t.Errorf("规则 = (%q,%v), want (list_percent,100)", last.UpstreamMarkupType, last.UpstreamMarkupValue)
	}
	if math.Abs(last.Price-180) > 0.001 {
		t.Errorf("售价 = %v, want 180 (按上游标价 100%%)", last.Price)
	}
	// 调价要留痕：既有成本口径历史，也有"售价旧值→新值"的记录（259→310.8 之类）。
	var priceHistory *model.ProductHistory
	for i := range repo.history {
		if repo.history[i].OldValue == "126.00" && repo.history[i].NewValue == "180.00" {
			priceHistory = &repo.history[i]
		}
	}
	if priceHistory == nil {
		t.Errorf("缺少售价变更历史（旧值→新值），history=%+v", repo.history)
	}
	// 非法规则要报错且不落库。
	if _, err := svc.ApplyUpstreamMarkupRule(context.Background(), 28975, "bogus", 100, "admin", ""); err == nil {
		t.Error("非法加价方式应报错")
	}
}

// TestBatchApplyReportsFailures 批量操作逐条回报：一条失败不拖垮整批，且失败原因要能看懂。
func TestBatchApplyReportsFailures(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}}}
	svc := &productService{repo: repo}
	calls := 0
	resp, err := svc.batchApply(context.Background(), []uint64{1, 2, 3}, 7, "admin", func(_ context.Context, id uint64, _ uint64, _ string) error {
		calls++
		if id == 2 {
			return errors.New("上架失败：上游规格未完成平台绑定")
		}
		return nil
	}, "上架")
	if err != nil {
		t.Fatalf("batchApply: %v", err)
	}
	if calls != 3 {
		t.Errorf("应逐条执行 3 次，实际 %d", calls)
	}
	if len(resp.Succeeded) != 2 || len(resp.Failed) != 1 {
		t.Fatalf("succeeded=%v failed=%v，期望 2 成功 1 失败", resp.Succeeded, resp.Failed)
	}
	if resp.Failed[0].Name != "B" || resp.Failed[0].Reason == "" {
		t.Errorf("失败项要带商品名与原因，got %+v", resp.Failed[0])
	}
	if !strings.Contains(resp.Message, "2") || !strings.Contains(resp.Message, "1 个失败") {
		t.Errorf("汇总文案应给出成功/失败数，got %q", resp.Message)
	}
	// 空选择与超上限都要给明确反馈（超上限报错，避免误点几千条）。
	empty, err := svc.batchApply(context.Background(), nil, 7, "admin", func(context.Context, uint64, uint64, string) error { return nil }, "删除")
	if err != nil || empty.Message != "未选择商品" {
		t.Errorf("空选择应返回提示，got %+v err=%v", empty, err)
	}
	big := make([]uint64, batchActionLimit+1)
	if _, err := svc.batchApply(context.Background(), big, 7, "admin", func(context.Context, uint64, uint64, string) error { return nil }, "删除"); err == nil {
		t.Error("超过单次上限应报错")
	}
}

// TestSetSpecPassthrough 上架门禁放行项：仅上游转售商品可设，幂等，且必须留痕。
func TestSetSpecPassthrough(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{
		{ID: 157, Name: "海外年付活动", SourceMode: model.SourceModeUpstream, SourceProductID: 35, Status: 0},
		{ID: 8, Name: "自营商品", SourceMode: model.SourceModeSelf},
	}}
	svc := &productService{repo: repo}
	info, err := svc.SetSpecPassthrough(context.Background(), 157, true, 1, "admin")
	if err != nil {
		t.Fatalf("SetSpecPassthrough: %v", err)
	}
	if !info.SpecPassthrough {
		t.Error("返回的商品应已开启仅透传")
	}
	if len(repo.history) == 0 || !strings.Contains(repo.history[len(repo.history)-1].Remark, "仅透传") {
		t.Errorf("开启仅透传要写操作历史（含原因说明），history=%+v", repo.history)
	}
	// 幂等：状态未变时不重复写历史。
	before := len(repo.history)
	if _, err := svc.SetSpecPassthrough(context.Background(), 157, true, 1, "admin"); err != nil {
		t.Fatalf("幂等调用失败: %v", err)
	}
	if len(repo.history) != before {
		t.Error("重复设置同值不应再写历史")
	}
	// 自营商品没有这个开关的语义，要明确报错而不是静默打开。
	if _, err := svc.SetSpecPassthrough(context.Background(), 8, true, 1, "admin"); err == nil {
		t.Error("自营商品设置仅透传应报错")
	}
	// 关闭也要留痕。
	if _, err := svc.SetSpecPassthrough(context.Background(), 157, false, 1, "admin"); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	if !strings.Contains(repo.history[len(repo.history)-1].Remark, "关闭") {
		t.Errorf("关闭仅透传要写历史，got %q", repo.history[len(repo.history)-1].Remark)
	}
}
