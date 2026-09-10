package service

import (
	"context"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
	"hostsent/backend/internal/modules/admin/user/account/repository"
)

type UserGroupService interface {
	List(ctx context.Context, query dto.UserGroupListQuery) (*dto.UserGroupListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.UserGroupInfo, error)
	Create(ctx context.Context, req dto.UserGroupCreateRequest) (*dto.UserGroupInfo, error)
	Update(ctx context.Context, id uint64, req dto.UserGroupUpdateRequest) (*dto.UserGroupInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type userGroupService struct {
	repo repository.UserGroupRepository
}

func NewUserGroupService(repo repository.UserGroupRepository) UserGroupService {
	return &userGroupService{repo: repo}
}

func (s *userGroupService) List(ctx context.Context, query dto.UserGroupListQuery) (*dto.UserGroupListResponse, error) {
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
	respItems := make([]dto.UserGroupInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toUserGroupInfo(item))
	}
	return &dto.UserGroupListResponse{
		Items: respItems,
		Meta:  dto.UserGroupListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *userGroupService) FindByID(ctx context.Context, id uint64) (*dto.UserGroupInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toUserGroupInfo(*item)
	return &resp, nil
}

func (s *userGroupService) Create(ctx context.Context, req dto.UserGroupCreateRequest) (*dto.UserGroupInfo, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	item := &model.UserGroup{
		Name:          req.Name,
		Code:          req.Code,
		Description:   req.Description,
		Status:        status,
		SortOrder:     req.SortOrder,
		PricePolicyID: req.PricePolicyID,
		Priority:      req.Priority,
		IsDefault:     req.IsDefault,
		IsAgentGroup:  req.IsAgentGroup,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	resp := toUserGroupInfo(*item)
	return &resp, nil
}

func (s *userGroupService) Update(ctx context.Context, id uint64, req dto.UserGroupUpdateRequest) (*dto.UserGroupInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = req.Name
	item.Code = req.Code
	item.Description = req.Description
	item.Status = req.Status
	item.SortOrder = req.SortOrder
	item.PricePolicyID = req.PricePolicyID
	item.Priority = req.Priority
	item.IsDefault = req.IsDefault
	item.IsAgentGroup = req.IsAgentGroup
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	resp := toUserGroupInfo(*item)
	return &resp, nil
}

func (s *userGroupService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func toUserGroupInfo(item model.UserGroup) dto.UserGroupInfo {
	return dto.UserGroupInfo{
		ID:            item.ID,
		Name:          item.Name,
		Code:          item.Code,
		Description:   item.Description,
		Status:        item.Status,
		SortOrder:     item.SortOrder,
		PricePolicyID: item.PricePolicyID,
		Priority:      item.Priority,
		IsDefault:     item.IsDefault,
		IsAgentGroup:  item.IsAgentGroup,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}
