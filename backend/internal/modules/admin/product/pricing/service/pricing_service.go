package service

import (
	"context"
	"time"

	"hostsent/backend/internal/modules/admin/product/pricing/dto"
	"hostsent/backend/internal/modules/admin/product/pricing/model"
	"hostsent/backend/internal/modules/admin/product/pricing/repository"
)

// PricingService 定义价格策略业务能力。
type PricingService interface {
	List(ctx context.Context, query dto.PricingQuery) (*dto.PricingListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.PricingInfo, error)
	Create(ctx context.Context, req dto.PricingRequest) (*dto.PricingInfo, error)
	Update(ctx context.Context, id uint64, req dto.PricingRequest) (*dto.PricingInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type pricingService struct {
	repo repository.PricingRepository
}

func NewPricingService(repo repository.PricingRepository) PricingService {
	return &pricingService{repo: repo}
}

func (s *pricingService) List(ctx context.Context, query dto.PricingQuery) (*dto.PricingListResponse, error) {
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.PricingInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildPricingInfo(item))
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	return &dto.PricingListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *pricingService) FindByID(ctx context.Context, id uint64) (*dto.PricingInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildPricingInfo(*item)
	return &info, nil
}

func (s *pricingService) Create(ctx context.Context, req dto.PricingRequest) (*dto.PricingInfo, error) {
	item := &model.ProductPricing{
		ProductID:      req.ProductID,
		BillingMode:    req.BillingMode,
		UnitPrice:      req.UnitPrice,
		MinBillingUnit: req.MinBillingUnit,
		TierPricing:    req.TierPricing,
		TierDiscount:   req.TierDiscount,
		Status:         req.Status,
	}
	if item.MinBillingUnit < 1 {
		item.MinBillingUnit = 1
	}
	if item.Status == 0 {
		item.Status = model.PricingEnabled
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *pricingService) Update(ctx context.Context, id uint64, req dto.PricingRequest) (*dto.PricingInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.ProductID = req.ProductID
	item.BillingMode = req.BillingMode
	item.UnitPrice = req.UnitPrice
	item.MinBillingUnit = req.MinBillingUnit
	item.TierPricing = req.TierPricing
	item.TierDiscount = req.TierDiscount
	item.Status = req.Status
	if item.MinBillingUnit < 1 {
		item.MinBillingUnit = 1
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *pricingService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func buildPricingInfo(item model.ProductPricing) dto.PricingInfo {
	return dto.PricingInfo{
		ID:             item.ID,
		ProductID:      item.ProductID,
		BillingMode:    item.BillingMode,
		UnitPrice:      item.UnitPrice,
		MinBillingUnit: item.MinBillingUnit,
		TierPricing:    item.TierPricing,
		TierDiscount:   item.TierDiscount,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      item.UpdatedAt.Format(time.RFC3339),
	}
}
