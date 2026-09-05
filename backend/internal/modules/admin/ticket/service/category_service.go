package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/model"
	"hostsent/backend/internal/modules/admin/ticket/repository"
)

// CategoryService 定义工单分类管理业务能力。
type CategoryService interface {
	// List 分类列表（管理端含禁用，用户端仅启用）
	List(ctx context.Context, includeDisabled bool) ([]dto.CategoryInfo, error)
	Create(ctx context.Context, req dto.CategorySaveRequest) (*dto.CategoryInfo, error)
	Update(ctx context.Context, id uint64, req dto.CategorySaveRequest) (*dto.CategoryInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type categoryService struct {
	categoryRepo repository.CategoryRepository
	ticketRepo   repository.TicketRepository
}

// NewCategoryService 创建工单分类业务服务。
func NewCategoryService(categoryRepo repository.CategoryRepository, ticketRepo repository.TicketRepository) CategoryService {
	return &categoryService{categoryRepo: categoryRepo, ticketRepo: ticketRepo}
}

func (s *categoryService) List(ctx context.Context, includeDisabled bool) ([]dto.CategoryInfo, error) {
	items, err := s.categoryRepo.List(ctx, includeDisabled)
	if err != nil {
		return nil, err
	}
	infos := make([]dto.CategoryInfo, 0, len(items))
	for _, item := range items {
		infos = append(infos, buildCategoryInfo(item))
	}
	return infos, nil
}

// Create 创建分类：编码唯一性校验。
func (s *categoryService) Create(ctx context.Context, req dto.CategorySaveRequest) (*dto.CategoryInfo, error) {
	code := strings.TrimSpace(req.Code)
	if existing, err := s.categoryRepo.FindByCode(ctx, code); err == nil && existing != nil {
		return nil, ErrCategoryCodeExists
	}
	status := req.Status
	if status == "" {
		status = model.CategoryStatusActive
	}
	item := &model.TicketCategory{
		Name:        req.Name,
		Code:        code,
		Description: req.Description,
		SortOrder:   req.SortOrder,
		Status:      status,
	}
	if err := s.categoryRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	info := buildCategoryInfo(*item)
	return &info, nil
}

// Update 更新分类：编码唯一性校验（排除自身）。
func (s *categoryService) Update(ctx context.Context, id uint64, req dto.CategorySaveRequest) (*dto.CategoryInfo, error) {
	item, err := s.categoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapCategoryErr(err)
	}
	code := strings.TrimSpace(req.Code)
	if existing, err := s.categoryRepo.FindByCode(ctx, code); err == nil && existing != nil && existing.ID != id {
		return nil, ErrCategoryCodeExists
	}
	item.Name = req.Name
	item.Code = code
	item.Description = req.Description
	item.SortOrder = req.SortOrder
	if req.Status != "" {
		item.Status = req.Status
	}
	if err := s.categoryRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	info := buildCategoryInfo(*item)
	return &info, nil
}

// Delete 删除分类：使用中的分类禁止删除。
func (s *categoryService) Delete(ctx context.Context, id uint64) error {
	category, err := s.categoryRepo.FindByID(ctx, id)
	if err != nil {
		return mapCategoryErr(err)
	}
	// 引用校验：分类下存在工单则禁止删除
	stats, err := s.ticketRepo.CountByCategory(ctx)
	if err != nil {
		return err
	}
	for _, stat := range stats {
		if stat.Category == category.Code && stat.Count > 0 {
			return ErrCategoryInUse
		}
	}
	return s.categoryRepo.Delete(ctx, id)
}

func buildCategoryInfo(item model.TicketCategory) dto.CategoryInfo {
	return dto.CategoryInfo{
		ID:          item.ID,
		Name:        item.Name,
		Code:        item.Code,
		Description: item.Description,
		SortOrder:   item.SortOrder,
		Status:      item.Status,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.Format(time.RFC3339),
	}
}

func mapCategoryErr(err error) error {
	if err == nil {
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return ErrCategoryNotFound
	}
	return err
}
