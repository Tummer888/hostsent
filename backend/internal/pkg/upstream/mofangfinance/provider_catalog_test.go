package mofangfinance

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
	raw := buildProductRawSpecs(detail, "常驻 | 活动", detail.ProductPricings, costRows, flag)

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
	// 无分组名时不写入 group_name（避免把 nil 当分类名）。
	deleteNoGroup := buildProductRawSpecs(UpstreamProductDetail{GID: 6}, "", nil, nil, SaleFlag{})
	if _, ok := deleteNoGroup["group_name"]; ok {
		t.Error("未知分组的商品不应写入 group_name")
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
