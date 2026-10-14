package service

import (
	"context"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
)

// upstreamProduct 上游转售商品的最小样本（渠道 7）。
func upstreamProduct() model.Product {
	return model.Product{
		ID: 176, Code: "cpy-7-120", Name: "香港|六区·轻量 4-4 15Mbps",
		SourceMode: model.SourceModeUpstream, SourceProductID: 120, SourceProviderID: 7,
		Specs: `{"cpu":4}`, ConfigOptions: `{"os":"Debian"}`, Stock: 5,
	}
}

func ptrU64(v uint64) *uint64 { return &v }
func ptrStr(v string) *string { return &v }

// TestUpdateUpstreamAllowsIdempotentProviderEcho 上游商品回传**原值**渠道必须放行。
//
// 实测故障：编辑页把读到的 source_provider_id 原样回传，后端对上游商品无条件拒绝该字段，
// 导致"改个商品名/描述"都保存失败（报「代理商品的上游渠道由导入流程决定」）。
// 回传原值是幂等 no-op，不能当成改绑。
func TestUpdateUpstreamAllowsIdempotentProviderEcho(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{upstreamProduct()}}
	svc := &productService{repo: repo}

	e := "改过的描述"
	_, err := svc.Update(context.Background(), 176, dto.ProductUpdateRequest{
		Name:             "香港|六区·轻量 4-4 15Mbps",
		SourceProviderID: ptrU64(7), // 与库中一致
		SourceMode:       ptrStr("upstream"),
		Description:      e,
		Specs:            `{"cpu":4}`,
		ConfigOptions:    `{"os":"Debian"}`,
		Stock:            5,
	}, 1, "admin")
	if err != nil {
		t.Fatalf("回传原值渠道应放行，got %v", err)
	}
	if len(repo.updates) != 1 {
		t.Fatalf("应写入一次，got %d", len(repo.updates))
	}
	got := repo.updates[0]
	if got.Description != e {
		t.Fatalf("描述应更新，got %q", got.Description)
	}
	// 关键：规格/配置项不能被写成空（前端曾漏传这两个非指针字段）。
	if got.Specs != `{"cpu":4}` || got.ConfigOptions != `{"os":"Debian"}` {
		t.Fatalf("规格与配置项应保持原值，got specs=%q config=%q", got.Specs, got.ConfigOptions)
	}
	if got.SourceProviderID != 7 {
		t.Fatalf("渠道不应被改动，got %d", got.SourceProviderID)
	}
}

// TestUpdateUpstreamRejectsProviderChange 真正改绑仍必须被拒。
func TestUpdateUpstreamRejectsProviderChange(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{upstreamProduct()}}
	svc := &productService{repo: repo}

	_, err := svc.Update(context.Background(), 176, dto.ProductUpdateRequest{
		Name:             "香港|六区·轻量 4-4 15Mbps",
		SourceProviderID: ptrU64(9), // 换成别家渠道
		Specs:            `{"cpu":4}`,
		ConfigOptions:    `{"os":"Debian"}`,
		Stock:            5,
	}, 1, "admin")
	if err == nil {
		t.Fatal("改绑上游渠道应被拒绝")
	}
	if !strings.Contains(err.Error(), "上游渠道由导入流程决定") {
		t.Fatalf("错误信息应说明渠道不可改，got %v", err)
	}
	if len(repo.updates) != 0 {
		t.Fatalf("被拒时不应写库，got %d 次", len(repo.updates))
	}
}

// TestUpdateUpstreamAllowsOmittingProvider 完全不传渠道字段同样放行（指针 nil = 保持原值）。
func TestUpdateUpstreamAllowsOmittingProvider(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{upstreamProduct()}}
	svc := &productService{repo: repo}

	_, err := svc.Update(context.Background(), 176, dto.ProductUpdateRequest{
		Name:          "改名后的商品",
		Specs:         `{"cpu":4}`,
		ConfigOptions: `{"os":"Debian"}`,
		Stock:         5,
	}, 1, "admin")
	if err != nil {
		t.Fatalf("不传渠道字段应放行，got %v", err)
	}
	if repo.updates[0].SourceProviderID != 7 {
		t.Fatalf("渠道应保持 7，got %d", repo.updates[0].SourceProviderID)
	}
}
