package service

import (
	"context"
	"testing"

	"hostsent/backend/internal/modules/admin/resource/product/dto"
	"hostsent/backend/internal/modules/admin/resource/product/model"
)

// fakeResourceProductRepo 列表/详情读取替身（只需满足 List 与 FindByID）。
type fakeResourceProductRepo struct {
	items []model.ResourceProduct
	total int64
}

func (f *fakeResourceProductRepo) List(context.Context, dto.ProductListQuery) ([]model.ResourceProduct, int64, error) {
	return f.items, f.total, nil
}
func (f *fakeResourceProductRepo) FindByID(context.Context, uint64) (*model.ResourceProduct, error) {
	if len(f.items) == 0 {
		return nil, nil
	}
	item := f.items[0]
	return &item, nil
}
func (f *fakeResourceProductRepo) Update(context.Context, *model.ResourceProduct) error { return nil }
func (f *fakeResourceProductRepo) UpsertMany(context.Context, uint64, []model.ResourceProduct) error {
	return nil
}

// fakeImportedReader 已导入引用替身。
type fakeImportedReader struct {
	refs map[uint64]ImportedProductRef
}

func (f fakeImportedReader) ListImportedRefs(context.Context, []uint64) (map[uint64]ImportedProductRef, error) {
	return f.refs, nil
}

// TestListMarksImportedProducts 上游商品列表要能标出「哪些已导入」：
// 导入弹窗据此禁止重复勾选，上游商品页据此区分已导入/未导入。
func TestListMarksImportedProducts(t *testing.T) {
	repo := &fakeResourceProductRepo{
		items: []model.ResourceProduct{
			{ID: 35, UpstreamID: "589", Name: "海外年付活动——轻量型", Status: 1, RawSpecs: `{"group_name_source":"upstream","visibility":"visible","price_source":"catalog"}`},
			{ID: 36, UpstreamID: "590", Name: "国内年付活动——标准型", Status: 1},
		},
		total: 2,
	}
	svc := NewProductService(repo)
	svc.SetImportedReader(fakeImportedReader{refs: map[uint64]ImportedProductRef{
		35: {ProductID: 157, Status: 0},
	}})

	resp, err := svc.List(context.Background(), dto.ProductListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(resp.Items))
	}
	first := resp.Items[0]
	if !first.Imported || first.ImportedProductID != 157 {
		t.Errorf("第一条应标记已导入（商品 157），got imported=%v id=%d", first.Imported, first.ImportedProductID)
	}
	if first.GroupNameDerived {
		t.Error("来源为 upstream 的分类不应标为派生")
	}
	if first.PriceSource != "catalog" || first.Visibility != "visible" {
		t.Errorf("元信息未从 raw_specs 摘出: price_source=%q visibility=%q", first.PriceSource, first.Visibility)
	}
	if second := resp.Items[1]; second.Imported || second.ImportedProductID != 0 {
		t.Errorf("未导入商品不应标记已导入: %+v", second)
	}
	// 列表不带 raw_specs（大字段），但要保留派生标记。
	if first.RawSpecs != "" {
		t.Error("列表不应返回 raw_specs")
	}
}

// TestListWithoutImportedReader 未装配端口时不报错、按未导入展示。
func TestListWithoutImportedReader(t *testing.T) {
	repo := &fakeResourceProductRepo{items: []model.ResourceProduct{{ID: 1, Name: "x"}}, total: 1}
	svc := NewProductService(repo)
	resp, err := svc.List(context.Background(), dto.ProductListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.Items[0].Imported {
		t.Error("未装配 importedReader 时不应标记已导入")
	}
}
