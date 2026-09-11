package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
	specmodel "hostsent/backend/internal/modules/admin/product/spec/model"
	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	"hostsent/backend/internal/modules/open/dto"
	openrepo "hostsent/backend/internal/modules/open/repository"
	ucproductdto "hostsent/backend/internal/modules/uc/product/dto"
	ucproductservice "hostsent/backend/internal/modules/uc/product/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/pricing"
)

type stubAtomReader struct{ atoms []specmodel.SpecAtom }

func (s *stubAtomReader) ListAtoms(_ context.Context) ([]specmodel.SpecAtom, error) {
	return s.atoms, nil
}

type stubProductReader struct {
	list    *ucproductdto.ListResponse
	get     map[uint64]*ucproductdto.ProductInfo
	getErr  error
	lastIn  *ucproductdto.ListQuery
	lastUID uint64
}

func (s *stubProductReader) List(_ context.Context, q ucproductdto.ListQuery) (*ucproductdto.ListResponse, error) {
	s.lastIn = &q
	return s.list, nil
}

func (s *stubProductReader) Get(_ context.Context, id uint64) (*ucproductdto.ProductInfo, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if p, ok := s.get[id]; ok {
		return p, nil
	}
	return nil, errors.New("not found")
}

type stubCatalogRepo struct {
	pools []providermodel.ResourcePool
	specs []string
}

func (s *stubCatalogRepo) ListEnabledPools(_ context.Context) ([]providermodel.ResourcePool, error) {
	return s.pools, nil
}
func (s *stubCatalogRepo) ListOnSaleSkuSpecs(_ context.Context) ([]string, error) {
	return s.specs, nil
}

type stubQuoteResolver struct {
	quote     *pricing.Quote
	lastInput *pricing.ResolveInput
}

func (s *stubQuoteResolver) Resolve(_ context.Context, in pricing.ResolveInput) (*pricing.Quote, error) {
	s.lastInput = &in
	return s.quote, nil
}

func newCatalogFixture() (*CatalogService, *stubProductReader, *stubQuoteResolver) {
	products := &stubProductReader{
		list: &ucproductdto.ListResponse{
			Items: []ucproductdto.ProductInfo{{ID: 1, Name: "标准云主机", Price: 68, PriceModel: "monthly"}},
			Total: 1, Page: 1, PageSize: 20,
		},
		get: map[uint64]*ucproductdto.ProductInfo{
			1: {ID: 1, Name: "标准云主机", Price: 68, PriceModel: "monthly",
				Skus: []ucproductdto.SkuInfo{{SpecCode: "2c4g", Name: "2C4G", Specs: `{"os":"ubuntu-22.04"}`, Price: 68, Stock: -1}}},
		},
	}
	quotes := &stubQuoteResolver{quote: &pricing.Quote{
		OriginalAmount: 68, DiscountAmount: 13.6, FinalAmount: 54.4,
		Source: pricing.SourceGroup,
	}}
	svc := NewCatalogService(CatalogDeps{
		Atoms: &stubAtomReader{atoms: []specmodel.SpecAtom{
			{Key: "compute.cpu", Name: "CPU", ValueType: "int", Status: 1, PlatformFields: `{"mofangyun":"cpu"}`},
			{Key: "legacy.atom", Name: "停用", Status: 0},
		}},
		Products: products,
		Catalog: &stubCatalogRepo{
			pools: []providermodel.ResourcePool{{ID: 3, Name: "华东节点", PoolType: "compute", Status: 1}},
			specs: []string{`{"os":"ubuntu-22.04"}`, `{"os":"debian-12"}`, `{"os":"ubuntu-22.04"}`, `not-json`},
		},
		Pricing: quotes,
	})
	return svc, products, quotes
}

func TestListSpecAtomsFiltersDisabled(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	items, err := svc.ListSpecAtoms(context.Background())
	if err != nil {
		t.Fatalf("ListSpecAtoms: %v", err)
	}
	if len(items) != 1 || items[0].Key != "compute.cpu" {
		t.Fatalf("disabled atoms must be filtered: %+v", items)
	}
}

func TestListProductsPagination(t *testing.T) {
	svc, products, _ := newCatalogFixture()
	resp, err := svc.ListProducts(context.Background(), "", 0, 0, 500)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	// 超大 pageSize 被钳制到上限；page<1 归一为 1。
	if products.lastIn.PageSize != 100 {
		t.Fatalf("pageSize cap: %d", products.lastIn.PageSize)
	}
	if resp.Items[0].ID != 1 || resp.Items[0].Price != 68 {
		t.Fatalf("product mapping: %+v", resp.Items[0])
	}
}

func TestGetProductOfflineMapsTo40404(t *testing.T) {
	svc, products, _ := newCatalogFixture()
	products.getErr = ucproductservice.ErrProductOffline
	_, err := svc.GetProduct(context.Background(), 9)
	ae, ok := err.(*apperrors.AppError)
	if !ok || ae.Code != CodeOpenNotFound {
		t.Fatalf("offline product must map to 40404, got %v", err)
	}
	// gorm record not found 同样归一为 40404。
	products.getErr = gorm.ErrRecordNotFound
	_, err = svc.GetProduct(context.Background(), 9)
	ae, ok = err.(*apperrors.AppError)
	if !ok || ae.Code != CodeOpenNotFound {
		t.Fatalf("missing product must map to 40404, got %v", err)
	}
}

func TestListImagesDedup(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	items, err := svc.ListImages(context.Background())
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("dedup os values: %+v", items)
	}
}

func TestListRegions(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	items, err := svc.ListRegions(context.Background())
	if err != nil || len(items) != 1 || items[0].Name != "华东节点" {
		t.Fatalf("regions: %+v err=%v", items, err)
	}
}

func TestQuoteUsesOwnerUserIDAndHidesDiscount(t *testing.T) {
	svc, _, quotes := newCatalogFixture()
	info, err := svc.Quote(context.Background(), 42, dto.QuoteRequest{ProductID: 1, SpecCode: "2c4g", Quantity: 2})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if quotes.lastInput.UserID != 42 {
		t.Fatalf("quote must resolve with owner user id, got %d", quotes.lastInput.UserID)
	}
	// 对外只给原价与实付，不给折扣明细。
	if info.OriginalAmount != 68 || info.FinalAmount != 54.4 {
		t.Fatalf("quote amounts: %+v", info)
	}
	b, _ := json.Marshal(info)
	if strings.Contains(string(b), "discount") || strings.Contains(string(b), "policy") {
		t.Fatalf("quote response must not leak discount details: %s", b)
	}
}

func TestQuoteParamCheck(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	if _, err := svc.Quote(context.Background(), 1, dto.QuoteRequest{}); err == nil {
		t.Fatalf("product_id=0 must be rejected")
	}
}

var _ = openrepo.ErrAppNotFound // 保持仓储契约导入（文档化依赖方向）
