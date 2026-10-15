package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/product/category/dto"
	"hostsent/backend/internal/modules/admin/product/category/model"
)

// fakeCategoryRepo 覆盖 List / ExistsChildren / CountProducts* 三项，
// 其余方法在用例里不会被调到。
type fakeCategoryRepo struct {
	items          []model.ProductCategory
	hasChildren    bool
	counts         map[uint64]int64
	uncategorized  int64
	subtreeInUse   int64
	deleted        bool
	countErr       error
	subtreeQueryID uint64
}

func (f *fakeCategoryRepo) List(context.Context, *int) ([]model.ProductCategory, error) {
	return f.items, nil
}
func (f *fakeCategoryRepo) FindByID(context.Context, uint64) (*model.ProductCategory, error) {
	return nil, nil
}
func (f *fakeCategoryRepo) Create(context.Context, *model.ProductCategory) error { return nil }
func (f *fakeCategoryRepo) Update(context.Context, *model.ProductCategory) error { return nil }
func (f *fakeCategoryRepo) Delete(context.Context, uint64) error {
	f.deleted = true
	return nil
}
func (f *fakeCategoryRepo) ExistsChildren(context.Context, uint64) (bool, error) {
	return f.hasChildren, nil
}
func (f *fakeCategoryRepo) CountProductsByCategory(context.Context) (map[uint64]int64, error) {
	if f.countErr != nil {
		return nil, f.countErr
	}
	return f.counts, nil
}
func (f *fakeCategoryRepo) CountUncategorizedProducts(context.Context) (int64, error) {
	return f.uncategorized, nil
}
func (f *fakeCategoryRepo) CountProductsInSubtree(_ context.Context, rootID uint64) (int64, error) {
	f.subtreeQueryID = rootID
	return f.subtreeInUse, nil
}

// TestListAttachesProductCounts 分类列表要带出节点商品数与未分类数。
// 没有这个数，分类导航只是棵树，运营看不出哪个分类是空的、也没法判断
// 「未分类」里堆了多少要不要整理。
func TestListAttachesProductCounts(t *testing.T) {
	repo := &fakeCategoryRepo{
		items: []model.ProductCategory{
			{ID: 1, Name: "云主机", Status: model.CategoryStatusEnabled},
			{ID: 8, Name: "虚拟主机", ParentID: 5, Status: model.CategoryStatusEnabled},
			{ID: 5, Name: "建站服务", Status: model.CategoryStatusEnabled},
		},
		counts:        map[uint64]int64{1: 2, 5: 0, 8: 3},
		uncategorized: 4,
	}
	svc := NewCategoryService(repo)

	resp, err := svc.List(context.Background(), dto.CategoryListRequest{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.UncategorizedCount != 4 {
		t.Errorf("未分类数 = %d，期望 4", resp.UncategorizedCount)
	}
	// 树里逐个节点核对：父节点的数字必须是「自身直接挂的」，不能把子节点累加进来
	// —— 点父节点时列表按整棵子树展开，累加会让数字与看到的结果对不上。
	got := map[string]int64{}
	var walk func([]*dto.CategoryInfo)
	walk = func(nodes []*dto.CategoryInfo) {
		for _, n := range nodes {
			got[n.Name] = n.ProductCount
			walk(n.Children)
		}
	}
	walk(resp.Items)

	want := map[string]int64{"云主机": 2, "建站服务": 0, "虚拟主机": 3}
	for name, exp := range want {
		if got[name] != exp {
			t.Errorf("分类 %q 的商品数 = %d，期望 %d", name, got[name], exp)
		}
	}
}

// TestDeleteBlockedWhenProductsAttached 分类下还有商品时不允许删除。
//
// 这是「未分类」数据的根源：此前只挡子分类，分类删掉后商品的 category_id
// 变成指向不存在行的悬空值 —— 界面显示不出分类名、也统计不进任何分类。
func TestDeleteBlockedWhenProductsAttached(t *testing.T) {
	repo := &fakeCategoryRepo{subtreeInUse: 7}
	svc := NewCategoryService(repo)

	err := svc.Delete(context.Background(), 5)
	if err == nil {
		t.Fatal("分类下有商品时应当拒绝删除")
	}
	if !errors.Is(err, ErrCategoryInUse) {
		t.Errorf("错误类型不对：%v", err)
	}
	// 提示里要带上数量，运营才知道要移走多少个。
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("错误信息应包含商品数：%v", err)
	}
	if repo.deleted {
		t.Error("校验未通过却执行了删除")
	}
	// 子分类的存在与否不影响这次判断，但必须确认查的是**子树**数量：
	// 只看自身会让"父分类空、子分类有货"的情况漏过去，删完照样产生悬空值。
	if repo.subtreeQueryID != 5 {
		t.Errorf("子树统计的根节点 = %d，期望 5", repo.subtreeQueryID)
	}
}

// TestDeleteAllowedWhenEmpty 没有子分类也没有商品时正常删除。
func TestDeleteAllowedWhenEmpty(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)

	if err := svc.Delete(context.Background(), 9); err != nil {
		t.Fatalf("空分类应当可删：%v", err)
	}
	if !repo.deleted {
		t.Error("未执行删除")
	}
}

// TestDeleteStillBlockedByChildren 子分类优先拦截（原有行为不能因新校验而改变）。
func TestDeleteStillBlockedByChildren(t *testing.T) {
	repo := &fakeCategoryRepo{hasChildren: true, subtreeInUse: 3}
	svc := NewCategoryService(repo)

	err := svc.Delete(context.Background(), 5)
	if !errors.Is(err, ErrCategoryHasChildren) {
		t.Errorf("有子分类时应报 HasChildren，实际：%v", err)
	}
	if repo.deleted {
		t.Error("校验未通过却执行了删除")
	}
}
