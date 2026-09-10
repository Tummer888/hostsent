package service

import (
	"context"
	"errors"

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
	// DefaultGroupID 返回默认用户组 ID（0 表示无可用默认组）。
	// 注册链路与后台建号共用此兜底归组能力（装配层注入）。
	DefaultGroupID(ctx context.Context) (uint64, error)
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
		IsDefault:     req.IsDefault,
		IsAgentGroup:  req.IsAgentGroup,
	}
	// 默认组唯一性由仓储在同一事务内「先清后写」维护，此处无需再单独清理。
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
	// 默认组是系统不变量（有且仅有一个），只能「转移」不能「清空」：
	// 直接把当前默认组的标记取消会让注册/建号失去兜底目标，要求先指定另一个组。
	if item.IsDefault && !req.IsDefault {
		return nil, errors.New("必须保留一个默认用户组，请先将其他用户组设为默认再取消当前默认标记")
	}
	item.Name = req.Name
	item.Code = req.Code
	item.Description = req.Description
	item.Status = req.Status
	item.SortOrder = req.SortOrder
	item.PricePolicyID = req.PricePolicyID
	item.IsDefault = req.IsDefault
	item.IsAgentGroup = req.IsAgentGroup
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	resp := toUserGroupInfo(*item)
	return &resp, nil
}

func (s *userGroupService) Delete(ctx context.Context, id uint64) error {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	// 默认组是注册/建号的兜底目标，删除会让新用户重新散落在 NULL，要求先转移默认标记。
	if item.IsDefault {
		return errors.New("该用户组是默认用户组，请先将其他用户组设为默认后再删除")
	}
	return s.repo.Delete(ctx, id)
}

func (s *userGroupService) DefaultGroupID(ctx context.Context) (uint64, error) {
	return s.repo.DefaultGroupID(ctx)
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
		IsDefault:     item.IsDefault,
		IsAgentGroup:  item.IsAgentGroup,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}
