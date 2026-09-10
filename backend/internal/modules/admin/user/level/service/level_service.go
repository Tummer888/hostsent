// Package service 提供用户等级模块的业务编排。
package service

import (
	"context"

	"hostsent/backend/internal/modules/admin/user/level/dto"
	"hostsent/backend/internal/modules/admin/user/level/model"
	"hostsent/backend/internal/modules/admin/user/level/repository"
)

// UserLevelService 定义用户等级的业务能力。
type UserLevelService interface {
	List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.Info, error)
	Create(ctx context.Context, req dto.CreateRequest) (*dto.Info, error)
	Update(ctx context.Context, id uint64, req dto.UpdateRequest) (*dto.Info, error)
	Delete(ctx context.Context, id uint64) error
}

type userLevelService struct {
	repo repository.UserLevelRepository
}

// NewUserLevelService 创建用户等级业务服务。
func NewUserLevelService(repo repository.UserLevelRepository) UserLevelService {
	return &userLevelService{repo: repo}
}

func (s *userLevelService) List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.Info, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toInfo(item))
	}
	return &dto.ListResponse{
		Items: respItems,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *userLevelService) FindByID(ctx context.Context, id uint64) (*dto.Info, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toInfo(*item)
	return &resp, nil
}

func (s *userLevelService) Create(ctx context.Context, req dto.CreateRequest) (*dto.Info, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	item := &model.UserLevel{
		Name:             req.Name,
		Code:             req.Code,
		Weight:           req.Weight,
		Status:           status,
		FeatureFlags:     req.FeatureFlags,
		UpgradeCondition: req.UpgradeCondition,
		UpgradeThreshold: req.UpgradeThreshold,
		MaxSubAccounts:   req.MaxSubAccounts,
		Benefits:         req.Benefits,
		Description:      req.Description,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	resp := toInfo(*item)
	return &resp, nil
}

func (s *userLevelService) Update(ctx context.Context, id uint64, req dto.UpdateRequest) (*dto.Info, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = req.Name
	item.Code = req.Code
	item.Weight = req.Weight
	item.Status = req.Status
	item.FeatureFlags = req.FeatureFlags
	item.UpgradeCondition = req.UpgradeCondition
	item.UpgradeThreshold = req.UpgradeThreshold
	item.MaxSubAccounts = req.MaxSubAccounts
	item.Benefits = req.Benefits
	item.Description = req.Description
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	resp := toInfo(*item)
	return &resp, nil
}

func (s *userLevelService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func normalizeMeta(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func toInfo(item model.UserLevel) dto.Info {
	return dto.Info{
		ID:               item.ID,
		Name:             item.Name,
		Code:             item.Code,
		Weight:           item.Weight,
		Status:           item.Status,
		FeatureFlags:     item.FeatureFlags,
		UpgradeCondition: item.UpgradeCondition,
		UpgradeThreshold: item.UpgradeThreshold,
		MaxSubAccounts:   item.MaxSubAccounts,
		Benefits:         item.Benefits,
		Description:      item.Description,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}
