package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/product/dto"
	"hostsent/backend/internal/modules/admin/product/model"
	"hostsent/backend/internal/modules/admin/product/repository"
)

// ProductService 定义产品业务能力。
type ProductService interface {
	List(ctx context.Context, query dto.ProductListQuery) (*dto.ProductListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.ProductInfo, error)
	Create(ctx context.Context, req dto.ProductCreateRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	Update(ctx context.Context, id uint64, req dto.ProductUpdateRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	Delete(ctx context.Context, id uint64, operatorID uint64, operatorName string) error
	Publish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	Unpublish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	UpdatePrice(ctx context.Context, id uint64, req dto.ProductPriceRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	ListHistory(ctx context.Context, id uint64) ([]dto.ProductHistoryInfo, error)
	ListSpecs(ctx context.Context, id uint64) ([]dto.ProductSpecInfo, error)
}

type productService struct {
	repo repository.ProductRepository
}

// NewProductService 创建产品业务服务。
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

func (s *productService) Create(ctx context.Context, req dto.ProductCreateRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item := &model.Product{
		Code:             strings.TrimSpace(req.Code),
		Name:             strings.TrimSpace(req.Name),
		CategoryID:       req.CategoryID,
		ProductType:      req.ProductType,
		Description:      req.Description,
		Specs:            req.Specs,
		PriceModel:       req.PriceModel,
		Price:            req.Price,
		CostPrice:        req.CostPrice,
		SourceProductID:  req.SourceProductID,
		SourceProviderID: req.SourceProviderID,
		Stock:            req.Stock,
		SortOrder:        req.SortOrder,
		Status:           req.Status,
	}
	if item.Stock == 0 {
		item.Stock = -1
	}
	if item.Status == 0 {
		item.Status = model.ProductStatusDraft
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	// 记录创建历史
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID:    item.ID,
		ChangeType:   model.ChangeTypeCreate,
		NewValue:     item.Name,
		OperatorID:   operatorID,
		OperatorName: operatorName,
	})
	return s.FindByID(ctx, item.ID)
}

func (s *productService) Update(ctx context.Context, id uint64, req dto.ProductUpdateRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = strings.TrimSpace(req.Name)
	item.CategoryID = req.CategoryID
	item.ProductType = req.ProductType
	item.Description = req.Description
	item.Specs = req.Specs
	item.PriceModel = req.PriceModel
	item.Price = req.Price
	item.CostPrice = req.CostPrice
	item.Stock = req.Stock
	item.SortOrder = req.SortOrder
	item.Status = req.Status
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID:    id,
		ChangeType:   model.ChangeTypeUpdate,
		NewValue:     item.Name,
		OperatorID:   operatorID,
		OperatorName: operatorName,
	})
	return s.FindByID(ctx, id)
}

func (s *productService) Delete(ctx context.Context, id uint64, operatorID uint64, operatorName string) error {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID:    id,
		ChangeType:   model.ChangeTypeDelete,
		OldValue:     item.Name,
		OperatorID:   operatorID,
		OperatorName: operatorName,
	})
}

// Publish 上架产品：草稿/下架 → 上架，并记录操作历史。
func (s *productService) Publish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Status == model.ProductStatusPublished {
		return s.FindByID(ctx, id)
	}
	oldStatus := item.Status
	item.Status = model.ProductStatusPublished
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: id, ChangeType: model.ChangeTypePublish,
		OldValue: strconv.Itoa(oldStatus), NewValue: strconv.Itoa(item.Status),
		OperatorID: operatorID, OperatorName: operatorName,
	})
	return s.FindByID(ctx, id)
}

// Unpublish 下架产品：→ 下架，并记录操作历史。
func (s *productService) Unpublish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Status == model.ProductStatusOffline {
		return s.FindByID(ctx, id)
	}
	oldStatus := item.Status
	item.Status = model.ProductStatusOffline
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: id, ChangeType: model.ChangeTypeUnpublish,
		OldValue: strconv.Itoa(oldStatus), NewValue: strconv.Itoa(item.Status),
		OperatorID: operatorID, OperatorName: operatorName,
	})
	return s.FindByID(ctx, id)
}

func (s *productService) UpdatePrice(ctx context.Context, id uint64, req dto.ProductPriceRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	oldPrice := strconv.FormatFloat(item.Price, 'f', -1, 64)
	item.Price = req.Price
	item.CostPrice = req.CostPrice
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: id, ChangeType: model.ChangeTypePrice,
		OldValue: oldPrice, NewValue: strconv.FormatFloat(item.Price, 'f', -1, 64),
		OperatorID: operatorID, OperatorName: operatorName, Remark: req.Remark,
	})
	return s.FindByID(ctx, id)
}

func (s *productService) ListHistory(ctx context.Context, id uint64) ([]dto.ProductHistoryInfo, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.ProductHistoryInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, dto.ProductHistoryInfo{
			ID:           item.ID,
			ProductID:    item.ProductID,
			ChangeType:   item.ChangeType,
			OldValue:     item.OldValue,
			NewValue:     item.NewValue,
			OperatorName: item.OperatorName,
			Remark:       item.Remark,
			CreatedAt:    item.CreatedAt.Format(time.RFC3339),
		})
	}
	return resp, nil
}

func (s *productService) ListSpecs(ctx context.Context, id uint64) ([]dto.ProductSpecInfo, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListSpecs(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.ProductSpecInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, dto.ProductSpecInfo{
			ID:         item.ID,
			ProductID:  item.ProductID,
			SpecCode:   item.SpecCode,
			Name:       item.Name,
			Specs:      item.Specs,
			PriceModel: item.PriceModel,
			Price:      item.Price,
			CostPrice:  item.CostPrice,
			Stock:      item.Stock,
			SortOrder:  item.SortOrder,
			Status:     item.Status,
		})
	}
	return resp, nil
}

func buildProductInfo(item model.Product) dto.ProductInfo {
	return dto.ProductInfo{
		ID:               item.ID,
		Code:             item.Code,
		Name:             item.Name,
		CategoryID:       item.CategoryID,
		ProductType:      item.ProductType,
		Description:      item.Description,
		Specs:            item.Specs,
		PriceModel:       item.PriceModel,
		Price:            item.Price,
		CostPrice:        item.CostPrice,
		SourceProductID:  item.SourceProductID,
		SourceProviderID: item.SourceProviderID,
		Stock:            item.Stock,
		SortOrder:        item.SortOrder,
		Status:           item.Status,
		CreatedAt:        item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.Format(time.RFC3339),
	}
}
