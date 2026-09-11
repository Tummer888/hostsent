// Package service 提供用户中心商品模块业务编排（复用管理端商品目录，仅暴露上架商品）。
package service

import (
	"context"
	"errors"

	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"

	"hostsent/backend/internal/modules/uc/product/dto"
)

// ErrProductOffline 商品未上架，不可购买。
var ErrProductOffline = errors.New("商品已下架，不可购买")

// catalogReader 管理端商品目录读取能力的最小暴露接口（由装配层注入，避免 uc 依赖 admin service 层）。
type catalogReader interface {
	List(ctx context.Context, query catalogdto.ProductListQuery) (*catalogdto.ProductListResponse, error)
	FindByID(ctx context.Context, id uint64) (*catalogdto.ProductInfo, error)
	// ListSpecs 商品下的 SKU 列表（T4.1）：详情页展示可售规格。
	ListSpecs(ctx context.Context, productID uint64) ([]catalogdto.ProductSpecInfo, error)
}

// ProductService 用户中心商品业务能力。
type ProductService interface {
	// List 上架商品列表（status=published），仅返回用户可见字段。
	List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error)
	// Get 商品详情（仅上架商品可购）。
	Get(ctx context.Context, id uint64) (*dto.ProductInfo, error)
}

type productService struct {
	catalog catalogReader
}

// NewProductService 创建用户中心商品服务。
func NewProductService(catalog catalogReader) ProductService {
	return &productService{catalog: catalog}
}

func (s *productService) List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 12
	}
	resp, err := s.catalog.List(ctx, catalogdto.ProductListQuery{
		Keyword:    query.Keyword,
		CategoryID: query.Category,
		Status:     1, // 仅上架
		Featured:   query.Featured,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]dto.ProductInfo, 0, len(resp.Items))
	for _, it := range resp.Items {
		items = append(items, fromAdmin(it))
	}
	return &dto.ListResponse{Items: items, Page: page, PageSize: pageSize, Total: resp.Meta.Total}, nil
}

func (s *productService) Get(ctx context.Context, id uint64) (*dto.ProductInfo, error) {
	item, err := s.catalog.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 仅上架商品可购
	if item.Status != 1 {
		return nil, ErrProductOffline
	}
	info := fromAdmin(*item)
	// SKU 矩阵（T4.1）：详情页展示可售规格，供用户选择后按 spec_code 下单。
	if specs, err := s.catalog.ListSpecs(ctx, id); err == nil {
		for _, sp := range specs {
			if sp.Status != 1 {
				continue
			}
			info.Skus = append(info.Skus, dto.SkuInfo{
				SpecCode:   sp.SpecCode,
				Name:       sp.Name,
				Specs:      sp.Specs,
				Price:      sp.Price,
				PriceModel: sp.PriceModel,
				Stock:      sp.Stock,
			})
		}
	}
	return &info, nil
}

// fromAdmin 将管理端商品信息映射为用户可见字段（剔除成本价等）。
func fromAdmin(it catalogdto.ProductInfo) dto.ProductInfo {
	return dto.ProductInfo{
		ID:            it.ID,
		Name:          it.Name,
		Description:   it.Description,
		CoverImage:    it.CoverImage,
		CategoryID:    it.CategoryID,
		ProductType:   it.ProductType,
		Price:         it.Price,
		PriceModel:    it.PriceModel,
		Specs:         it.Specs,
		ProvisionMode: it.ProvisionMode,
		ConfigOptions: it.ConfigOptions,
		Featured:      it.Featured,
		CreatedAt:     it.CreatedAt,
	}
}
