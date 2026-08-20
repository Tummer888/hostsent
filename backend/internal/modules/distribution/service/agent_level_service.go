// Package service 提供分销模块的业务编排与规则处理。
package service

import (
	"context"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/model"
	"hostsent/backend/internal/modules/distribution/repository"
)

// AgentLevelService 定义分销等级管理所需的业务能力。
type AgentLevelService interface {
	List(ctx context.Context, query dto.AgentLevelListQuery) (*dto.AgentLevelListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.AgentLevelInfo, error)
	Create(ctx context.Context, req dto.AgentLevelCreateRequest) (*dto.AgentLevelInfo, error)
	Update(ctx context.Context, id uint64, req dto.AgentLevelUpdateRequest) (*dto.AgentLevelInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type agentLevelService struct {
	repo repository.AgentLevelRepository
}

// NewAgentLevelService 创建分销等级业务服务。
func NewAgentLevelService(repo repository.AgentLevelRepository) AgentLevelService {
	return &agentLevelService{repo: repo}
}

func (s *agentLevelService) List(ctx context.Context, query dto.AgentLevelListQuery) (*dto.AgentLevelListResponse, error) {
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
	respItems := make([]dto.AgentLevelInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toAgentLevelInfo(item))
	}
	return &dto.AgentLevelListResponse{
		Items: respItems,
		Meta:  dto.AgentLevelListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *agentLevelService) FindByID(ctx context.Context, id uint64) (*dto.AgentLevelInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toAgentLevelInfo(*item)
	return &resp, nil
}

func (s *agentLevelService) Create(ctx context.Context, req dto.AgentLevelCreateRequest) (*dto.AgentLevelInfo, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	item := &model.AgentLevel{
		Name:                   req.Name,
		Code:                   req.Code,
		Weight:                 req.Weight,
		DirectCommissionRate:   req.DirectCommissionRate,
		IndirectCommissionRate: req.IndirectCommissionRate,
		RenewalCommissionRate:  req.RenewalCommissionRate,
		UpgradeRewardAmount:    req.UpgradeRewardAmount,
		SelfPurchaseRebateRate: req.SelfPurchaseRebateRate,
		AllowManualPrice:       req.AllowManualPrice,
		AllowSubAgent:          req.AllowSubAgent,
		MaxSubAgentDepth:       req.MaxSubAgentDepth,
		Status:                 status,
		Description:            req.Description,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	resp := toAgentLevelInfo(*item)
	return &resp, nil
}

func (s *agentLevelService) Update(ctx context.Context, id uint64, req dto.AgentLevelUpdateRequest) (*dto.AgentLevelInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = req.Name
	item.Code = req.Code
	item.Weight = req.Weight
	item.DirectCommissionRate = req.DirectCommissionRate
	item.IndirectCommissionRate = req.IndirectCommissionRate
	item.RenewalCommissionRate = req.RenewalCommissionRate
	item.UpgradeRewardAmount = req.UpgradeRewardAmount
	item.SelfPurchaseRebateRate = req.SelfPurchaseRebateRate
	item.AllowManualPrice = req.AllowManualPrice
	item.AllowSubAgent = req.AllowSubAgent
	item.MaxSubAgentDepth = req.MaxSubAgentDepth
	item.Status = req.Status
	item.Description = req.Description
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	resp := toAgentLevelInfo(*item)
	return &resp, nil
}

func (s *agentLevelService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func toAgentLevelInfo(item model.AgentLevel) dto.AgentLevelInfo {
	return dto.AgentLevelInfo{
		ID:                     item.ID,
		Name:                   item.Name,
		Code:                   item.Code,
		Weight:                 item.Weight,
		DirectCommissionRate:   item.DirectCommissionRate,
		IndirectCommissionRate: item.IndirectCommissionRate,
		RenewalCommissionRate:  item.RenewalCommissionRate,
		UpgradeRewardAmount:    item.UpgradeRewardAmount,
		SelfPurchaseRebateRate: item.SelfPurchaseRebateRate,
		AllowManualPrice:       item.AllowManualPrice,
		AllowSubAgent:          item.AllowSubAgent,
		MaxSubAgentDepth:       item.MaxSubAgentDepth,
		Status:                 item.Status,
		Description:            item.Description,
		CreatedAt:              item.CreatedAt,
		UpdatedAt:              item.UpdatedAt,
	}
}
