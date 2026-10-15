package service

import (
	"context"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
)

// TestCreateRequiresCategory 建品分类必填。
//
// 分类为空时商品会落进「未分类」：列表那一列显示不出归属、分类导航也统计不到，
// 运营事后得逐个回头补。挡在服务端而不是只靠前端 required —— 接口是唯一入口，
// 绕过界面（脚本、旧客户端）一样会漏。
func TestCreateRequiresCategory(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := NewProductService(repo)

	_, err := svc.Create(context.Background(), dto.ProductCreateRequest{
		Code: "cat-required", Name: "无分类商品", CategoryID: 0,
	}, 1, "admin")

	if err == nil {
		t.Fatal("分类为空时应当拒绝建品")
	}
	if !strings.Contains(err.Error(), "分类") {
		t.Errorf("错误信息应指出是分类问题：%v", err)
	}
}

// TestCreateRejectsUnknownCategory 分类必须真实存在。
//
// 分类被删过之后，界面缓存里那个分类仍可能被选中提交，落库就成了
// 指向不存在行的悬空值（界面上表现为"未分类"，但谁也不知道是哪来的）。
func TestCreateRejectsUnknownCategory(t *testing.T) {
	notExists := false
	repo := &fakeProductRepo{categoryExists: &notExists}
	svc := NewProductService(repo)

	_, err := svc.Create(context.Background(), dto.ProductCreateRequest{
		Code: "cat-missing", Name: "悬空分类商品", CategoryID: 999,
	}, 1, "admin")

	if err == nil {
		t.Fatal("分类不存在时应当拒绝建品")
	}
	if !strings.Contains(err.Error(), "不存在") {
		t.Errorf("错误信息应说明分类不存在：%v", err)
	}
}

// TestCloneRequiresCategory 导入上游商品时分类必填（用户明确要求）。
//
// 导入动辄几十上百个商品，导入时留空会整批落进「未分类」，
// 事后逐个补分类的成本远高于导入时选一次。
func TestCloneRequiresCategory(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := NewProductService(repo)

	_, err := svc.CloneFromUpstream(context.Background(), dto.ProductCloneRequest{
		SourceProductID: 42, SourceProviderID: 7,
		Code: "clone-no-cat", CategoryID: 0,
	}, 1, "admin")

	if err == nil {
		t.Fatal("导入未选分类时应当拒绝")
	}
	if !strings.Contains(err.Error(), "分类") {
		t.Errorf("错误信息应指出是分类问题：%v", err)
	}
}

// TestBatchCloneRequiresCategory 批量导入同样必填，且在进入循环前就拦下。
func TestBatchCloneRequiresCategory(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := NewProductService(repo)

	_, err := svc.CloneFromUpstreamBatch(context.Background(), dto.ProductBatchCloneRequest{
		SourceProviderID: 7, SourceProductIDs: []uint64{1, 2, 3},
		CategoryID: 0, PricePercent: 100, Status: nil,
	}, 1, "admin")

	if err == nil {
		t.Fatal("批量导入未选分类时应当整批拒绝")
	}
	if !strings.Contains(err.Error(), "分类") {
		t.Errorf("错误信息应指出是分类问题：%v", err)
	}
	if len(repo.items) != 0 {
		t.Errorf("校验失败却建了 %d 个商品（应在循环前拦下）", len(repo.items))
	}
}

// TestValidateCategoryChecksExistence 校验器对合法/非法分类的判定。
func TestValidateCategoryChecksExistence(t *testing.T) {
	// 存在 → 放行
	if err := (&productService{repo: &fakeProductRepo{}}).validateCategory(context.Background(), 5); err != nil {
		t.Errorf("合法分类不应被拦：%v", err)
	}
	// 不存在 → 拦下
	notExists := false
	svc := &productService{repo: &fakeProductRepo{categoryExists: &notExists}}
	if err := svc.validateCategory(context.Background(), 5); err == nil {
		t.Error("不存在的分类应当被拦")
	}
	// 0 → 拦下（必填）
	if err := (&productService{repo: &fakeProductRepo{}}).validateCategory(context.Background(), 0); err == nil {
		t.Error("分类为空应当被拦")
	}
}
