package zjmf

import (
	"encoding/json"
	"testing"
)

// TestSaleFlagParse 验证折扣字段的四种下发形态：
// 上游无折扣时 flag 直接下发 false（不是对象），解析必须容忍。
func TestSaleFlagParse(t *testing.T) {
	cases := []struct {
		raw     string
		enabled bool
		typ     int
		bates   float64
	}{
		{`false`, false, 0, 0},
		{`null`, false, 0, 0},
		{`{}`, false, 0, 0},
		{`{"type":1,"bates":"70.00"}`, true, 1, 70},
		{`{"type":1,"bates":65}`, true, 1, 65},
		{`{"type":0,"bates":"5.00"}`, true, 0, 5},
	}
	for _, c := range cases {
		var f SaleFlag
		if err := json.Unmarshal([]byte(c.raw), &f); err != nil {
			t.Fatalf("unmarshal %s: %v", c.raw, err)
		}
		if f.Enabled() != c.enabled || f.Type != c.typ || float64(f.Bates) != c.bates {
			t.Errorf("flag %s => {enabled:%v type:%d bates:%v}, want {enabled:%v type:%d bates:%v}",
				c.raw, f.Enabled(), f.Type, f.Bates, c.enabled, c.typ, c.bates)
		}
	}
}

// TestSaleFlagApply 验证折算口径与上游 zjmf.php::upstreamBates 一致：
// type=1 按应付百分比折算，其它按固定额减免（不为负），无折扣原价返回。
func TestSaleFlagApply(t *testing.T) {
	cases := []struct {
		flag  SaleFlag
		price float64
		want  float64
	}{
		{SaleFlag{Type: 1, Bates: 70}, 109, 76.30}, // 绍兴电信 16-16：标价 109，代理七折
		{SaleFlag{Type: 1, Bates: 65}, 14.99, 9.74},
		{SaleFlag{Type: 1, Bates: 60}, 249, 149.40},
		{SaleFlag{Type: 0, Bates: 5}, 109, 104.00}, // 固定减免 5 元
		{SaleFlag{Type: 0, Bates: 200}, 109, 0},    // 减免超过标价时不为负
		{SaleFlag{}, 109, 109},                     // 无折扣
		{SaleFlag{Type: 1, Bates: 70}, -1, -1},     // 上游 -1 表示未配置该周期，不能折算
	}
	for _, c := range cases {
		if got := c.flag.Apply(c.price); got != c.want {
			t.Errorf("Apply(%+v, %v) = %v, want %v", c.flag, c.price, got, c.want)
		}
	}
}

// TestPricingRowScale 验证 pricing 行按倍率折算：价格与初装费同步折算，
// -1（未配置）保持原值，非价格字段（currency/code）不动。
func TestPricingRowScale(t *testing.T) {
	var row PricingRow
	raw := `{"id":"9","currency":"1","code":"CNY","monthly":"109.00","quarterly":"327.00",
	         "annually":"1090.00","onetime":-1,"msetupfee":"0.00","asetupfee":"10.00"}`
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		t.Fatalf("unmarshal pricing row: %v", err)
	}
	got := row.Scale(0.7)
	checks := map[string]float64{"monthly": 76.30, "quarterly": 228.90, "annually": 763.00, "asetupfee": 7.00, "msetupfee": 0}
	for key, want := range checks {
		v, _ := got.Num(key)
		if v != want {
			t.Errorf("scaled %s = %v, want %v", key, v, want)
		}
	}
	if v, _ := got.Num("onetime"); v != -1 {
		t.Errorf("scaled onetime = %v, want -1（未配置周期不应被折算）", v)
	}
	if got["code"] != "CNY" || got["currency"] != "1" {
		t.Errorf("非价格字段被改动: %v", got)
	}
	// 原行不可被修改（Scale 返回副本）。
	if v, _ := row.Num("monthly"); v != 109 {
		t.Errorf("原行被就地修改: monthly=%v", v)
	}
}

// TestPricingRowHeadline 验证代表价选取顺序：monthly > quarterly > semiannually > annually > onetime，
// -1 与 0 都跳过（-1=未配置，0=免费）。
func TestPricingRowHeadline(t *testing.T) {
	cases := []struct {
		raw  string
		want float64
	}{
		{`{"monthly":"14.99","quarterly":"47.97"}`, 14.99},
		{`{"monthly":-1,"quarterly":"47.97"}`, 47.97},
		{`{"monthly":0,"semiannually":"90.00","annually":"180.00"}`, 90.00},
		{`{"onetime":"200.00"}`, 200.00},
		{`{"monthly":-1}`, 0},
	}
	for _, c := range cases {
		var row PricingRow
		if err := json.Unmarshal([]byte(c.raw), &row); err != nil {
			t.Fatalf("unmarshal %s: %v", c.raw, err)
		}
		if got := row.HeadlinePrice(); got != c.want {
			t.Errorf("HeadlinePrice(%s) = %v, want %v", c.raw, got, c.want)
		}
	}
}

// TestBuildProductRawSpecs 验证 raw_specs 的下游契约字段：
// gid/group_name（分类）、product_pricings（折后=采购价）、product_pricings_list（标价）、
// description/pay_type（详情页）、config_groups（开通参数）。
func TestBuildProductRawSpecs(t *testing.T) {
	detail := UpstreamProductDetail{
		ID:              468,
		GID:             14,
		Type:            "dcimcloud",
		Name:            "绍兴电信|特惠·标准 16-16 50Mbps",
		Description:     "<div>16核16G</div>",
		PayType:         `{"pay_type":"recurring"}`,
		ProductPricings: []PricingRow{jsonMustRow(t, `{"monthly":"109.00","quarterly":"327.00"}`)},
	}
	flag := SaleFlag{Type: 1, Bates: 70}
	costRows := scalePricingRows(detail.ProductPricings, flag)
	raw := buildProductRawSpecs(detail, productMeta{
		GroupName: "常驻 | 活动", GroupSource: groupSourceUpstream,
		ListRows: detail.ProductPricings, CostRows: costRows, Flag: flag,
		PriceSource: priceSourcePricings, Visibility: visibilityVisible,
	})

	if raw["gid"] != int64(14) {
		t.Errorf("gid = %v, want 14（sync 引擎据此写 resource_products.group_id）", raw["gid"])
	}
	if raw["group_name"] != "常驻 | 活动" {
		t.Errorf("group_name = %v, want 常驻 | 活动（导入弹窗的分类标题来源）", raw["group_name"])
	}
	if raw["description"] != "<div>16核16G</div>" {
		t.Errorf("description 未透传: %v", raw["description"])
	}
	if raw["list_price"] != 109.0 {
		t.Errorf("list_price = %v, want 109（上游标价）", raw["list_price"])
	}
	if raw["cost_price_monthly"] != 76.3 {
		t.Errorf("cost_price_monthly = %v, want 76.3（代理价）", raw["cost_price_monthly"])
	}
	if raw["product_pricings"] == nil || raw["product_pricings_list"] == nil {
		t.Fatalf("pricing 双口径缺失: %v", raw)
	}
	disc, ok := raw["upstream_discount"].(map[string]interface{})
	if !ok || disc["bates"] != 70.0 {
		t.Errorf("upstream_discount = %v, want bates=70", raw["upstream_discount"])
	}
	if desc, _ := disc["desc"].(string); desc == "" {
		t.Error("折扣描述为空，详情页无法展示折扣来源")
	}
	if raw["group_name_source"] != groupSourceUpstream {
		t.Errorf("group_name_source = %v, want upstream（前端据此判定是否打「派生」标记）", raw["group_name_source"])
	}
	if raw["visibility"] != visibilityVisible {
		t.Errorf("visibility = %v, want visible", raw["visibility"])
	}
	if raw["price_source"] != priceSourcePricings {
		t.Errorf("price_source = %v, want pricings", raw["price_source"])
	}
	// 无分组名时不写入 group_name（避免把 nil 当分类名）。
	deleteNoGroup := buildProductRawSpecs(UpstreamProductDetail{GID: 6}, productMeta{})
	if _, ok := deleteNoGroup["group_name"]; ok {
		t.Error("未知分组的商品不应写入 group_name")
	}
}

// TestDeriveGroupLabel 隐藏分组的分类名派生：只认「地区|区域」惯例，不拿营销词当分类名。
func TestDeriveGroupLabel(t *testing.T) {
	cases := []struct{ name, want string }{
		{"香港|二区·轻量 8-8 30Mbps", "香港|二区"},
		{"宁波电信|一区·标准 16-16 30Mbps", "宁波电信|一区"},
		{"美国|五区·流量 A型 1Gbps", "美国|五区"},
		{"香港五区|裸金属 32核 32GB 50Mbps", "香港五区|裸金属"},
		{"2025·海外|年付 活动款-1010", ""}, // 首个分隔符前没有「|」，不臆造
		{"美国精品CN2 8核 16GB 1G带宽 F型", ""},
		{"内蒙古云服务器-A型", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := deriveGroupLabel(c.name); got != c.want {
			t.Errorf("deriveGroupLabel(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestClassifyVisibility 在售判定：分组隐藏/商品隐藏/已下架都不得显示为上架。
func TestClassifyVisibility(t *testing.T) {
	visible := map[int64]string{14: "香港|六区", 6: ""}
	cases := []struct {
		name     string
		detail   UpstreamProductDetail
		groups   map[int64]string
		groupsOK bool
		want     string
	}{
		{"正常在售", UpstreamProductDetail{GID: 14}, visible, true, visibilityVisible},
		{"分组被隐藏", UpstreamProductDetail{GID: 66}, visible, true, visibilityGroupHid},
		{"商品隐藏", UpstreamProductDetail{GID: 14, Hidden: 1}, visible, true, visibilityProductHi},
		{"商品下架", UpstreamProductDetail{GID: 14, Retired: 1}, visible, true, visibilityRetired},
		{"cart/all 取不到时不误判", UpstreamProductDetail{GID: 66}, nil, false, visibilityVisible},
	}
	for _, c := range cases {
		if got := classifyVisibility(c.detail, c.groups, c.groupsOK); got != c.want {
			t.Errorf("%s: classifyVisibility = %q, want %q", c.name, got, c.want)
		}
	}
	if got := statusForVisibility(visibilityGroupHid); got != "inactive" {
		t.Errorf("隐藏分组商品状态 = %q, want inactive（同步后显示未在售）", got)
	}
	if got := statusForVisibility(visibilityVisible); got != "active" {
		t.Errorf("在售商品状态 = %q, want active", got)
	}
}

// TestResolveListRows 活动款价格回填：product_pricings 全 0/-1 时用前台展示价补其售卖周期。
func TestResolveListRows(t *testing.T) {
	raw := []PricingRow{jsonMustRow(t, `{"monthly":"-1.00","annually":"0"}`)}
	cat := CartIndexProduct{ProductPrice: 259, SalePrice: 181.3, HasBates: 1, BillingCycle: "annually", SetupFee: 0}
	rows, source := resolveListRows(raw, cat, true)
	if source != priceSourceCatalog {
		t.Fatalf("source = %q, want catalog", source)
	}
	if v, _ := rows[0].Num("annually"); v != 259 {
		t.Errorf("annually = %v, want 259（上游前台标价）", v)
	}
	if v, _ := rows[0].Num("monthly"); v != -1 {
		t.Errorf("monthly = %v, want -1（不可售周期保持不变）", v)
	}
	// 折后成本按 flag 换算：259 × 70% = 181.3，与上游前台 sale_price 一致。
	flag := SaleFlag{Type: 1, Bates: 70}
	costs := scalePricingRows(rows, flag)
	if v, _ := costs[0].Num("annually"); v != 181.3 {
		t.Errorf("代理价 annually = %v, want 181.3", v)
	}
	// 有正价时不回填（保持周期价口径）。
	withPrice := []PricingRow{jsonMustRow(t, `{"monthly":"109.00"}`)}
	if _, source := resolveListRows(withPrice, cat, true); source != priceSourcePricings {
		t.Errorf("source = %q, want pricings", source)
	}
	// 前台目录也没有 → 上游未定价。
	if _, source := resolveListRows(raw, CartIndexProduct{}, false); source != priceSourceNone {
		t.Errorf("source = %q, want none", source)
	}
}

// TestScalePricingRowsNoDiscount 无折扣时不得克隆/改动原始行。
func TestScalePricingRowsNoDiscount(t *testing.T) {
	rows := []PricingRow{jsonMustRow(t, `{"monthly":"109.00"}`)}
	got := scalePricingRows(rows, SaleFlag{})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if v, _ := got[0].Num("monthly"); v != 109 {
		t.Errorf("monthly = %v, want 109", v)
	}
}

// jsonMustRow 解析一行 pricing 测试数据。
func jsonMustRow(t *testing.T, raw string) PricingRow {
	t.Helper()
	var row PricingRow
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		t.Fatalf("unmarshal pricing row %s: %v", raw, err)
	}
	return row
}

// TestCartResponsesEnvelopeShape 回归守卫：CartAllResp / CartProductConfigResp 描述的是
// 响应壳 data **内部**的结构（p.call 已解开 {status,msg,data}）。
// 曾经多包一层 Data 字段 → 解出零值但不报错，分类名与代理折扣静默丢失。
func TestCartResponsesEnvelopeShape(t *testing.T) {
	// cart/all 的 data（截取自上游实测响应）。
	var all CartAllResp
	inner := `{"count":240,"currency":"CNY","products":[
	  {"id":14,"name":"常驻 | 活动","products":[{"id":468,"type":"dcimcloud","name":"绍兴电信|特惠·标准 16-16 50Mbps"}]}
	]}`
	if err := json.Unmarshal([]byte(inner), &all); err != nil {
		t.Fatalf("cart/all 内层结构解析失败: %v", err)
	}
	if all.Count != 240 || all.Currency != "CNY" || len(all.Products) != 1 {
		t.Fatalf("cart/all 解析结果错误: %+v", all)
	}
	if all.Products[0].Name != "常驻 | 活动" || len(all.Products[0].Products) != 1 ||
		all.Products[0].Products[0].ID != 468 {
		t.Fatalf("分组/成员解析错误: %+v", all.Products[0])
	}

	// cart/get_product_config 的 data：flag 是客户折扣（本案例 70% 应付比例）。
	var cfg CartProductConfigResp
	cfgInner := `{"flag":{"type":1,"bates":"70.00"},
	  "products":{"id":468,"gid":14,"pay_type":"{\"pay_type\":\"recurring\"}"},
	  "product_pricings":[{"monthly":"109.00","quarterly":"327.00","onetime":"-1.00"}],
	  "config_groups":[{"id":34774,"name":"配置项组","options":[
	    {"id":217855,"option_name":"cpu|CPU","upstream_id":912,"sub":[
	      {"id":838666,"option_name":"16|16核","upstream_id":7841,"hidden":0}]}]}],
	  "config_links":[34774]}`
	if err := json.Unmarshal([]byte(cfgInner), &cfg); err != nil {
		t.Fatalf("get_product_config 内层结构解析失败: %v", err)
	}
	if !cfg.Flag.Enabled() || cfg.Flag.Type != 1 || float64(cfg.Flag.Bates) != 70 {
		t.Fatalf("flag 解析错误: %+v", cfg.Flag)
	}
	if len(cfg.ProductPricings) != 1 || len(cfg.ConfigGroups) != 1 || len(cfg.ConfigLinks) != 1 {
		t.Fatalf("配置解析错误: pricings=%d groups=%d links=%v",
			len(cfg.ProductPricings), len(cfg.ConfigGroups), cfg.ConfigLinks)
	}
	if cfg.Flag.Apply(109) != 76.30 {
		t.Errorf("折扣后价格 = %v, want 76.30", cfg.Flag.Apply(109))
	}
}
