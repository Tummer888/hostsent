package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"

	apperrors "hostsent/backend/internal/pkg/errors"

	specmodel "hostsent/backend/internal/modules/admin/product/spec/model"
	openrepo "hostsent/backend/internal/modules/open/repository"
	ucproductdto "hostsent/backend/internal/modules/uc/product/dto"
	ucproductservice "hostsent/backend/internal/modules/uc/product/service"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/pricing"

	"hostsent/backend/internal/modules/open/dto"
)

// mapProductErr 商品读取错误归一：不存在/下架 → 40404，其余透传。
func mapProductErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ucproductservice.ErrProductOffline) || errors.Is(err, gorm.ErrRecordNotFound) {
		return apperrors.New(CodeOpenNotFound, "商品不存在或未上架")
	}
	if ae, ok := err.(*apperrors.AppError); ok {
		return ae
	}
	return err
}

// SpecAtomReader 规格原子字典读取（由 spec 契约仓储实现）。
type SpecAtomReader interface {
	ListAtoms(ctx context.Context) ([]specmodel.SpecAtom, error)
}

// OpenProductReader 在售商品读取（复用 uc 商品服务，仅上架口径）。
type OpenProductReader interface {
	List(ctx context.Context, query ucproductdto.ListQuery) (*ucproductdto.ListResponse, error)
	Get(ctx context.Context, id uint64) (*ucproductdto.ProductInfo, error)
}

// QuoteResolver 统一算价管线（D3：UserID 传 open_apps.owner_user_id 即下游成本价）。
type QuoteResolver interface {
	Resolve(ctx context.Context, in pricing.ResolveInput) (*pricing.Quote, error)
}

// CatalogDeps 只读目录服务依赖。
type CatalogDeps struct {
	Atoms    SpecAtomReader
	Products OpenProductReader
	Catalog  openrepo.CatalogRepository
	Pricing  QuoteResolver
	// PageSizeDefault / PageSizeMax 分页保护。
	PageSizeDefault int
	PageSizeMax     int
}

// CatalogService 开放平台只读目录服务（T6.2）。
type CatalogService struct {
	deps CatalogDeps
}

// NewCatalogService 构造目录服务。
func NewCatalogService(deps CatalogDeps) *CatalogService {
	if deps.PageSizeDefault <= 0 {
		deps.PageSizeDefault = 20
	}
	if deps.PageSizeMax <= 0 {
		deps.PageSizeMax = 100
	}
	return &CatalogService{deps: deps}
}

// ListSpecAtoms 规格原子字典（status=1），供下游做自身规格映射。
func (s *CatalogService) ListSpecAtoms(ctx context.Context) ([]dto.SpecAtomItem, error) {
	atoms, err := s.deps.Atoms.ListAtoms(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dto.SpecAtomItem, 0, len(atoms))
	for _, a := range atoms {
		if a.Status != 1 {
			continue
		}
		item := dto.SpecAtomItem{
			Key:          a.Key,
			Name:         a.Name,
			Unit:         a.Unit,
			ValueType:    a.ValueType,
			MinValue:     a.MinValue,
			MaxValue:     a.MaxValue,
			StepValue:    a.StepValue,
			Required:     a.Required,
			Configurable: a.Configurable,
			AppliesTo:    a.AppliesTo,
			Description:  a.Description,
		}
		if a.EnumValues != "" {
			var enums []string
			if json.Unmarshal([]byte(a.EnumValues), &enums) == nil {
				item.EnumValues = enums
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// ListProducts 在售商品分页（keyword/category 过滤）。
func (s *CatalogService) ListProducts(ctx context.Context, keyword string, categoryID uint64, page, pageSize int) (*dto.ProductListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = s.deps.PageSizeDefault
	}
	if pageSize > s.deps.PageSizeMax {
		pageSize = s.deps.PageSizeMax
	}
	resp, err := s.deps.Products.List(ctx, ucproductdto.ListQuery{
		Keyword:  keyword,
		Category: categoryID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]dto.ProductItem, 0, len(resp.Items))
	for _, it := range resp.Items {
		items = append(items, toProductItem(it))
	}
	return &dto.ProductListResponse{Total: resp.Total, Page: resp.Page, PageSize: resp.PageSize, Items: items}, nil
}

// GetProduct 商品详情（含 SKU）；未上架/不存在按 40404 处理。
func (s *CatalogService) GetProduct(ctx context.Context, id uint64) (*dto.ProductDetail, error) {
	info, err := s.deps.Products.Get(ctx, id)
	if err != nil {
		return nil, mapProductErr(err)
	}
	detail := &dto.ProductDetail{ProductItem: toProductItem(*info), Skus: make([]dto.SkuItem, 0, len(info.Skus))}
	for _, sku := range info.Skus {
		cycles := sku.Cycles
		if cycles == nil {
			// 显式给空数组：下游据此判断"该 SKU 无周期价、按单价下单"。
			cycles = []string{}
		}
		detail.Skus = append(detail.Skus, dto.SkuItem{
			SpecCode: sku.SpecCode, Name: sku.Name, Specs: sku.Specs,
			Price: sku.Price, PriceModel: sku.PriceModel, Stock: sku.Stock,
			Cycles: cycles,
		})
	}
	return detail, nil
}

// ListRegions 标准区域（启用中的资源池）。
func (s *CatalogService) ListRegions(ctx context.Context) ([]dto.RegionItem, error) {
	pools, err := s.deps.Catalog.ListEnabledPools(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dto.RegionItem, 0, len(pools))
	for _, p := range pools {
		items = append(items, dto.RegionItem{ID: p.ID, Name: p.Name, Type: p.PoolType})
	}
	return items, nil
}

// ListImages 标准镜像（v1：聚合在售 SKU 的 os 原子取值，去重）。
// 平台镜像同步能力落地后可替换数据源，对外契约不变。
func (s *CatalogService) ListImages(ctx context.Context) ([]dto.ImageItem, error) {
	specs, err := s.deps.Catalog.ListOnSaleSkuSpecs(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	items := make([]dto.ImageItem, 0)
	for _, raw := range specs {
		os := extractOSValue(raw)
		if os == "" {
			continue
		}
		if _, ok := seen[os]; ok {
			continue
		}
		seen[os] = struct{}{}
		items = append(items, dto.ImageItem{ID: os, Name: os})
	}
	return items, nil
}

// Quote 询价：走统一算价管线，UserID 用 owner_user_id（D3），
// 只返回原价与下游实付，折扣明细不外暴露。
func (s *CatalogService) Quote(ctx context.Context, ownerUserID uint64, req dto.QuoteRequest) (*dto.QuoteInfo, error) {
	if req.ProductID == 0 {
		return nil, apperrors.New(CodeOpenParam, "product_id 必填")
	}
	product, err := s.deps.Products.Get(ctx, req.ProductID)
	if err != nil {
		return nil, mapProductErr(err)
	}
	// 周期口径与下单一致（doc25）：显式周期须为规范值/上游别名，留空回落商品 price_model。
	cycle, ok := resolveOpenCycle(product.PriceModel, req.Cycle)
	if !ok {
		return nil, apperrors.New(CodeOpenParam, "不支持的计费周期："+req.Cycle)
	}
	quote, err := s.deps.Pricing.Resolve(ctx, pricing.ResolveInput{
		UserID:    ownerUserID,
		ProductID: req.ProductID,
		SpecCode:  req.SpecCode,
		Quantity:  req.Quantity,
		Cycle:     cycle,
	})
	if err != nil {
		return nil, err
	}
	return &dto.QuoteInfo{
		ProductID:      req.ProductID,
		SpecCode:       req.SpecCode,
		Quantity:       req.Quantity,
		Cycle:          cycle,
		OriginalAmount: quote.OriginalAmount,
		FinalAmount:    quote.FinalAmount,
	}, nil
}

// resolveOpenCycle 归一化开放平台询价/下单周期：显式周期非法返回 false（调用方报参数错误），
// 留空回落商品 price_model 对应周期（存量行为不变）。
func resolveOpenCycle(priceModel, raw string) (string, bool) {
	cycle := strings.TrimSpace(raw)
	if cycle == "" {
		return billingcycle.NormalizeOrDefault(priceModel), true
	}
	if billingcycle.IsValid(cycle) {
		return cycle, true
	}
	if normalized := billingcycle.Normalize(cycle); normalized != "" {
		return normalized, true
	}
	return "", false
}

// extractOSValue 从 SKU 原子取值 JSON 提取 os 值；解析失败返回空。
func extractOSValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	v, ok := m["os"]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case float64:
		return strings.TrimSuffix(strings.TrimRight(jsonNumber(val), "0"), ".")
	default:
		return ""
	}
}

func jsonNumber(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func toProductItem(it ucproductdto.ProductInfo) dto.ProductItem {
	return dto.ProductItem{
		ID:          it.ID,
		Name:        it.Name,
		Description: it.Description,
		CategoryID:  it.CategoryID,
		Price:       it.Price,
		PriceModel:  it.PriceModel,
		Featured:    it.Featured,
		CreatedAt:   it.CreatedAt,
	}
}
