package repository

import (
	"encoding/json"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/model"
)

// quantityGroupsJSON 上游数量型配置项的真实形态（取自实测：美国四区商品 config_groups）：
// 系统盘 80–1024（80 免费、81 起 0.8/GB）、数据盘 0–1024（0.8/GB）、带宽 75–500（75 免费、76 起 20/Mbps）、
// IP 1–10（1 免费、2 起 25/个）。
const quantityGroupsJSON = `[
  {
    "id": 32927, "name": "美国CN2 4核 4G 60M 不限流量配置项组",
    "options": [
      {
        "id": 241017, "option_name": "system_disk_size|系统盘", "option_type": 19,
        "qty_minimum": 80, "qty_maximum": 1024, "upstream_id": 0, "hidden": 0,
        "sub": [
          {"id": 1036029, "option_name": "系统盘", "qty_minimum": 80, "qty_maximum": 80, "hidden": 0,
           "pricings": [{"monthly": 0, "annually": 0, "quarterly": 0, "onetime": 0}]},
          {"id": 1036030, "option_name": "系统盘", "qty_minimum": 81, "qty_maximum": 1024, "hidden": 0,
           "pricings": [{"monthly": 0.8, "annually": 9.6, "quarterly": 2.4, "onetime": 0}]}
        ]
      },
      {
        "id": 241021, "option_name": "data_disk_size|数据盘", "option_type": 14,
        "qty_minimum": 0, "qty_maximum": 1024, "upstream_id": 0, "hidden": 0,
        "sub": [
          {"id": 1036036, "option_name": "数据盘", "qty_minimum": 0, "qty_maximum": 1024, "hidden": 0,
           "pricings": [{"monthly": 0.8, "annually": 9.6, "quarterly": 2.4, "onetime": 0}]}
        ]
      },
      {
        "id": 241019, "option_name": "bw|带宽", "option_type": 18,
        "qty_minimum": 75, "qty_maximum": 500, "upstream_id": 0, "hidden": 0,
        "sub": [
          {"id": 1036033, "option_name": "带宽", "qty_minimum": 75, "qty_maximum": 75, "hidden": 0,
           "pricings": [{"monthly": 0, "annually": 0, "quarterly": 0, "onetime": 0}]},
          {"id": 1036034, "option_name": "带宽", "qty_minimum": 76, "qty_maximum": 500, "hidden": 0,
           "pricings": [{"monthly": 20, "annually": 240, "quarterly": 60, "onetime": 0}]}
        ]
      },
      {
        "id": 241020, "option_name": "ip_num|IP数量", "option_type": 15,
        "qty_minimum": 1, "qty_maximum": 10, "upstream_id": 0, "hidden": 0,
        "sub": [
          {"id": 1036035, "option_name": "IP数量", "qty_minimum": 1, "qty_maximum": 1, "hidden": 0,
           "pricings": [{"monthly": 0, "annually": 0, "quarterly": 0, "onetime": 0}]},
          {"id": 1036037, "option_name": "IP数量", "qty_minimum": 2, "qty_maximum": 10, "hidden": 0,
           "pricings": [{"monthly": 25, "annually": 300, "quarterly": 75, "onetime": 0}]}
        ]
      }
    ]
  }
]`

func parseQuantityGroups(t *testing.T) []configGroup {
	t.Helper()
	var groups []interface{}
	if err := json.Unmarshal([]byte(quantityGroupsJSON), &groups); err != nil {
		t.Fatalf("解析测试 JSON 失败: %v", err)
	}
	parsed, err := parseConfigGroups(groups)
	if err != nil {
		t.Fatalf("parseConfigGroups 失败: %v", err)
	}
	return parsed
}

// TestBuildConfigOptionRowsQuantityRange 数量型选项必须落成带区间的 qty 控件。
//
// 实测故障：系统盘/数据盘/带宽/IP 这类"按量计价"的项，上游给的是可填数量区间，
// 早期导入只落了 option_type 与价档，区间与控件类型都没落库，客户端把它们
// 渲染成普通下拉（只有几个价档可选），客户根本填不出 100G 系统盘。
func TestBuildConfigOptionRowsQuantityRange(t *testing.T) {
	rows := buildConfigOptionRows(parseQuantityGroups(t), 212)
	if len(rows) != 4 {
		t.Fatalf("配置项数 = %d, want 4", len(rows))
	}
	byKey := map[int64]*model.ProductConfigOption{}
	for _, r := range rows {
		byKey[r.UpstreamKey] = r
	}

	cases := []struct {
		key      int64
		name     string
		wantMin  float64
		wantMax  float64
		wantDef  string
		wantUnit float64 // 每单位加价（取首个有价档）
	}{
		{241017, "系统盘", 80, 1024, "80", 0.8},
		{241021, "数据盘", 0, 1024, "0", 0.8},
		{241019, "带宽", 75, 500, "75", 20},
		{241020, "IP数量", 1, 10, "1", 25},
	}
	for _, c := range cases {
		opt := byKey[c.key]
		if opt == nil {
			t.Errorf("%s（key=%d）未落库", c.name, c.key)
			continue
		}
		if opt.Widget != model.ConfigWidgetQty {
			t.Errorf("%s 控件 = %q, want qty（否则客户端按下拉渲染，填不了具体数量）", c.name, opt.Widget)
		}
		if opt.MinValue == nil || *opt.MinValue != c.wantMin {
			t.Errorf("%s min = %v, want %v", c.name, deref(opt.MinValue), c.wantMin)
		}
		if opt.MaxValue == nil || *opt.MaxValue != c.wantMax {
			t.Errorf("%s max = %v, want %v", c.name, deref(opt.MaxValue), c.wantMax)
		}
		if opt.DefaultValue != c.wantDef {
			t.Errorf("%s 默认值 = %q, want %q（应为免费额度，按默认值开通不加价）", c.name, opt.DefaultValue, c.wantDef)
		}
		if len(opt.Subs) == 0 {
			t.Errorf("%s 应有价档子项", c.name)
			continue
		}
		var gotUnit float64
		for _, s := range opt.Subs {
			if !s.IsQuantityTier {
				t.Errorf("%s 子项应标记为数量价档（is_quantity_tier）", c.name)
			}
			if s.PriceMonthly > 0 && gotUnit == 0 {
				gotUnit = s.PriceMonthly
			}
		}
		if gotUnit != c.wantUnit {
			t.Errorf("%s 每单位加价 = %v, want %v", c.name, gotUnit, c.wantUnit)
		}
	}

	// 子项区间必须逐档落库：客户改数量时要靠它匹配正确的价档。
	sysDisk := byKey[241017]
	if sysDisk.Subs[0].QtyMaximum != 80 || sysDisk.Subs[1].QtyMinimum != 81 {
		t.Errorf("系统盘价档区间错误: %+v", sysDisk.Subs)
	}
}

// TestBuildConfigOptionRowsQuantityRangeRoundTrip 区间与价档标记要能经
// buildConfigGroups 回读（履约侧据此还原数量价档，丢失会让改动失败）。
func TestBuildConfigOptionRowsQuantityRangeRoundTrip(t *testing.T) {
	rows := buildConfigOptionRows(parseQuantityGroups(t), 212)
	groups := buildConfigGroups(rows)
	raw, err := json.Marshal(groups)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var back []configGroup
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	var found *configOption
	for i := range back[0].Options {
		if back[0].Options[i].ID == 241017 {
			found = &back[0].Options[i]
		}
	}
	if found == nil {
		t.Fatal("系统盘配置项未回读")
	}
	if found.QtyMinimum != 80 || found.QtyMaximum != 1024 {
		t.Errorf("回读区间 = %d..%d, want 80..1024", found.QtyMinimum, found.QtyMaximum)
	}
	if !found.Sub[1].IsQuantityTier {
		t.Error("价档标记未回读（履约侧将无法识别数量价档）")
	}
	if found.Sub[1].QtyMinimum != 81 || found.Sub[1].QtyMaximum != 1024 {
		t.Errorf("子项区间未回读: %d..%d", found.Sub[1].QtyMinimum, found.Sub[1].QtyMaximum)
	}
}

// TestQuantityOptionType 数量型判定必须与上游 judgeQuantity 一致。
func TestQuantityOptionType(t *testing.T) {
	// 上游 common.php::judgeQuantity 认定的数量型。
	for _, tp := range []int{4, 7, 9, 11, 14, 15, 16, 17, 18, 19} {
		if !quantityOptionType(tp) {
			t.Errorf("option_type=%d 应判为数量型", tp)
		}
	}
	// 枚举/开关/下拉不是数量型（5=操作系统、12=区域、1/2/3=普通）。
	for _, tp := range []int{1, 2, 3, 5, 6, 8, 10, 12, 13, 20} {
		if quantityOptionType(tp) {
			t.Errorf("option_type=%d 不应判为数量型", tp)
		}
	}
}

// TestQuantityUnitPriceAndFreeMax 免费额度取零价档覆盖的最大数量，价档自带单价。
func TestQuantityUnitPriceAndFreeMax(t *testing.T) {
	subs := []configSub{
		{QtyMinimum: 80, QtyMaximum: 80, Pricings: []configPrice{{Monthly: 0}}},
		{QtyMinimum: 81, QtyMaximum: 1024, Pricings: []configPrice{{Monthly: 0.8}}},
	}
	if got := quantityTierFreeMax(subs); got != 80 {
		t.Errorf("免费额度 = %d, want 80", got)
	}
	if got := firstPrice(subs[1].Pricings).Monthly; float64(got) != 0.8 {
		t.Errorf("单价 = %v, want 0.8", got)
	}
	// 单一有价档（数据盘）：免费额度为 0，客户按量付费。
	only := []configSub{{QtyMinimum: 0, QtyMaximum: 1024, Pricings: []configPrice{{Monthly: 0.8}}}}
	if got := quantityTierFreeMax(only); got != 0 {
		t.Errorf("无免费档时免费额度 = %d, want 0", got)
	}
	if got := firstPrice(only[0].Pricings).Monthly; float64(got) != 0.8 {
		t.Errorf("单价 = %v, want 0.8", got)
	}
	// 隐藏档不参与。
	hidden := append([]configSub{{QtyMinimum: 0, QtyMaximum: 50, Hidden: 1, Pricings: []configPrice{{Monthly: 0}}}}, subs...)
	if got := quantityTierFreeMax(hidden); got != 80 {
		t.Errorf("隐藏档不应计入免费额度, got %d", got)
	}
}

func deref(v *float64) string {
	if v == nil {
		return "nil"
	}
	return stringOfFloatLocal(*v)
}

func stringOfFloatLocal(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
