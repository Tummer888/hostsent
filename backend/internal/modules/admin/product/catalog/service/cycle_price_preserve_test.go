package service

import (
	"context"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/model"
	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
)

// TestBuildCyclePriceRowPreservesWithoutMarkup 无加价规则时不得把售价写成成本价。
//
// 实测故障（doc120 §11.7）：上游调价确认走周期矩阵重建，未配置加价规则的商品
// 被写成 Price=cost —— 零毛利白卖，且周期价在算价里优先级高于商品价，
// 客户真的按成本价下单。5 个商品、20 档被这样改写。
func TestBuildCyclePriceRowPreservesWithoutMarkup(t *testing.T) {
	item := model.Product{ID: 1, Price: 30, CostPrice: 18}
	row := buildCyclePriceRow(item, "monthly", 18, 30)
	if !row.PreservePrice {
		t.Fatal("无加价规则时应标记 PreservePrice，交由写入方保留原售价")
	}
	if row.Price != 0 {
		t.Errorf("无加价规则时不应自带售价，got %v", row.Price)
	}
	if row.CostPrice != 18 {
		t.Errorf("成本应照常同步 = 18, got %v", row.CostPrice)
	}
	if row.Source != "upstream" {
		t.Errorf("来源 = %q, want upstream", row.Source)
	}
}

// TestBuildCyclePriceRowMarkupStillDerives 有加价规则时照常按规则推导（不能被上面的改动带偏）。
func TestBuildCyclePriceRowMarkupStillDerives(t *testing.T) {
	cases := []struct {
		name  string
		mtype string
		mval  float64
		want  float64
	}{
		{"percent", model.MarkupTypePercent, 150, 27},          // 18 × 150%
		{"fixed", model.MarkupTypeFixed, 5, 23},                // 18 + 5
		{"list_percent", model.MarkupTypeListPercent, 120, 36}, // 标价 30 × 120%
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			item := model.Product{ID: 1, UpstreamMarkupType: c.mtype, UpstreamMarkupValue: c.mval}
			row := buildCyclePriceRow(item, "monthly", 18, 30)
			if row.PreservePrice {
				t.Error("有加价规则时不应 PreservePrice（要按规则重算）")
			}
			if row.Price != c.want {
				t.Errorf("售价 = %v, want %v", row.Price, c.want)
			}
			if row.Source != "markup" {
				t.Errorf("来源 = %q, want markup", row.Source)
			}
		})
	}
}

// TestApplyConfirmedPriceKeepsCycleSellPriceWithoutMarkup 端到端：上游成本变更确认后，
// 无加价规则的商品各周期售价必须原样保留，只更新成本。
func TestApplyConfirmedPriceKeepsCycleSellPriceWithoutMarkup(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{{
		ID: 176, Price: 30, CostPrice: 18, SourceProductID: 120,
		SourceMode: model.SourceModeUpstream,
	}}}
	writer := &fakeCycleWriter{}
	svc := &productService{repo: repo, cycleWriter: writer}
	// 上游各周期成本带了新的值（月度 18→20），标价 30/90/180/300。
	svc.resourceReader = fakeResourceReader{rp: &resourceproductmodel.ResourceProduct{
		ProviderID: 7, UpstreamID: "528", CostPrice: 18, SalePrice: 30,
		RawSpecs: `{
			"product_pricings": [{"monthly":"20.00","quarterly":"60.00","semiannually":"120.00","annually":"200.00"}],
			"product_pricings_list": [{"monthly":"30.00","quarterly":"90.00","semiannually":"180.00","annually":"300.00"}]
		}`,
	}}

	if _, err := svc.ApplyConfirmedPrice(context.Background(), 120, 20, "admin", ""); err != nil {
		t.Fatalf("ApplyConfirmedPrice: %v", err)
	}
	if len(writer.rows) != 4 {
		t.Fatalf("应产出 4 档周期行，got %d", len(writer.rows))
	}
	for _, row := range writer.rows {
		if !row.PreservePrice {
			t.Errorf("%s 档应保留售价，got PreservePrice=false price=%v", row.Cycle, row.Price)
		}
		if row.Price != 0 {
			t.Errorf("%s 档不应自带售价，got %v", row.Cycle, row.Price)
		}
		if row.CostPrice <= 0 {
			t.Errorf("%s 档成本应同步，got %v", row.Cycle, row.CostPrice)
		}
	}
}

// fakeCycleWriter 记录写入的周期行，供"不得按成本改写售价"的回归断言使用。
type fakeCycleWriter struct {
	rows []CyclePriceRow
}

func (f *fakeCycleWriter) ApplyUpstreamPrices(_ context.Context, _ uint64, rows []CyclePriceRow) error {
	f.rows = append(f.rows, rows...)
	return nil
}
