// Package service 提供上游商品管理模块的业务编排。
package service

import (
	"context"
	"time"

	"hostsent/backend/internal/modules/admin/resource/product/dto"
	"hostsent/backend/internal/modules/admin/resource/product/model"
	"hostsent/backend/internal/modules/admin/resource/product/repository"
)

// ProductService 上游商品业务能力
type ProductService interface {
	List(ctx context.Context, query dto.ProductListQuery) (*dto.ProductListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.ProductInfo, error)
	UpdatePrice(ctx context.Context, id uint64, req dto.ProductPriceRequest) (*dto.ProductInfo, error)
	// Sync 拉取上游商品（骨架占位，具体同步逻辑在同步引擎阶段实现）
	Sync(ctx context.Context, providerID uint64) (*dto.SyncResult, error)
}

type productService struct {
	repo repository.ProductRepository
}

// NewProductService 创建上游商品业务服务
func NewProductService(repo repository.ProductRepository) ProductService {
	return &productService{repo: repo}
}

func (s *productService) List(ctx context.Context, query dto.ProductListQuery) (*dto.ProductListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.ProductInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildProductInfo(item))
	}
	return &dto.ProductListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *productService) FindByID(ctx context.Context, id uint64) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildProductInfo(*item)
	return &info, nil
}

func (s *productService) UpdatePrice(ctx context.Context, id uint64, req dto.ProductPriceRequest) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.CostPrice = req.CostPrice
	item.SalePrice = req.SalePrice
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *productService) Sync(ctx context.Context, providerID uint64) (*dto.SyncResult, error) {
	return &dto.SyncResult{TaskID: 0, Status: "skipped", Message: "商品同步将在同步引擎阶段实现"}, nil
}

func buildProductInfo(item model.ResourceProduct) dto.ProductInfo {
	return dto.ProductInfo{
		ID:         item.ID,
		ProviderID: item.ProviderID,
		UpstreamID: item.UpstreamID,
		Name:       item.Name,
		CPU:        item.CPU,
		Memory:     item.Memory,
		Disk:       item.Disk,
		DiskType:   item.DiskType,
		Bandwidth:  item.Bandwidth,
		OS:         item.OS,
		Region:     item.Region,
		Zone:       item.Zone,
		Specs:      item.Specs,
		RawSpecs:   item.RawSpecs,
		CostPrice:  item.CostPrice,
		SalePrice:  item.SalePrice,
		Status:     item.Status,
		CreatedAt:  item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  item.UpdatedAt.Format(time.RFC3339),
	}
}
