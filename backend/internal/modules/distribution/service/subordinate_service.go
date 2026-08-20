package service

import (
	"context"
	"time"

	distributiondto "hostsent/backend/internal/modules/distribution/dto"
	distributionmodel "hostsent/backend/internal/modules/distribution/model"
	distributionrepo "hostsent/backend/internal/modules/distribution/repository"
	userrepo "hostsent/backend/internal/modules/user/repository"
)

type SubordinateService interface {
	List(ctx context.Context, query distributiondto.SubordinateListQuery) (*distributiondto.SubordinateListResponse, error)
	FindByID(ctx context.Context, id uint64) (*distributiondto.SubordinateInfo, error)
	Create(ctx context.Context, req distributiondto.SubordinateCreateRequest) (*distributiondto.SubordinateInfo, error)
	Update(ctx context.Context, id uint64, req distributiondto.SubordinateUpdateRequest) (*distributiondto.SubordinateInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type subordinateService struct {
	repo      distributionrepo.SubordinateRepository
	agentRepo distributionrepo.AgentRepository
	userRepo  userrepo.UserRepository
}

func NewSubordinateService(repo distributionrepo.SubordinateRepository, agentRepo distributionrepo.AgentRepository, userRepo userrepo.UserRepository) SubordinateService {
	return &subordinateService{repo: repo, agentRepo: agentRepo, userRepo: userRepo}
}

func (s *subordinateService) List(ctx context.Context, query distributiondto.SubordinateListQuery) (*distributiondto.SubordinateListResponse, error) {
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
	respItems := make([]distributiondto.SubordinateInfo, 0, len(items))
	for _, item := range items {
		resp, err := s.buildSubordinateInfo(ctx, item)
		if err != nil {
			return nil, err
		}
		respItems = append(respItems, resp)
	}
	return &distributiondto.SubordinateListResponse{Items: respItems, Meta: distributiondto.SubordinateListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *subordinateService) FindByID(ctx context.Context, id uint64) (*distributiondto.SubordinateInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := s.buildSubordinateInfo(ctx, *item)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *subordinateService) Create(ctx context.Context, req distributiondto.SubordinateCreateRequest) (*distributiondto.SubordinateInfo, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	relationType := req.RelationType
	if relationType == "" {
		relationType = "direct"
	}
	joinedAt := req.JoinedAt
	if joinedAt == nil {
		now := time.Now()
		joinedAt = &now
	}
	item := &distributionmodel.Subordinate{
		AgentID:            req.AgentID,
		UserID:             req.UserID,
		ParentAgentID:      req.ParentAgentID,
		LevelDepth:         req.LevelDepth,
		RelationType:       relationType,
		ContributionAmount: req.ContributionAmount,
		CommissionAmount:   req.CommissionAmount,
		Status:             status,
		JoinedAt:           joinedAt,
	}
	if item.LevelDepth <= 0 {
		item.LevelDepth = 1
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *subordinateService) Update(ctx context.Context, id uint64, req distributiondto.SubordinateUpdateRequest) (*distributiondto.SubordinateInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.ParentAgentID = req.ParentAgentID
	item.LevelDepth = req.LevelDepth
	if item.LevelDepth <= 0 {
		item.LevelDepth = 1
	}
	item.RelationType = req.RelationType
	item.ContributionAmount = req.ContributionAmount
	item.CommissionAmount = req.CommissionAmount
	item.Status = req.Status
	item.JoinedAt = req.JoinedAt
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *subordinateService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *subordinateService) buildSubordinateInfo(ctx context.Context, item distributionmodel.Subordinate) (distributiondto.SubordinateInfo, error) {
	agent, err := s.agentRepo.FindByID(ctx, item.AgentID)
	if err != nil {
		return distributiondto.SubordinateInfo{}, err
	}
	agentUser, err := s.userRepo.FindByID(ctx, agent.UserID)
	if err != nil {
		return distributiondto.SubordinateInfo{}, err
	}
	user, err := s.userRepo.FindByID(ctx, item.UserID)
	if err != nil {
		return distributiondto.SubordinateInfo{}, err
	}
	parentName := ""
	if item.ParentAgentID != nil {
		parentAgent, err := s.agentRepo.FindByID(ctx, *item.ParentAgentID)
		if err != nil {
			return distributiondto.SubordinateInfo{}, err
		}
		parentUser, err := s.userRepo.FindByID(ctx, parentAgent.UserID)
		if err != nil {
			return distributiondto.SubordinateInfo{}, err
		}
		if parentUser.RealName != "" {
			parentName = parentUser.RealName
		} else {
			parentName = parentUser.Username
		}
	}
	agentName := agentUser.Username
	if agentUser.RealName != "" {
		agentName = agentUser.RealName
	}
	return distributiondto.SubordinateInfo{
		ID:                 item.ID,
		AgentID:            item.AgentID,
		AgentName:          agentName,
		UserID:             item.UserID,
		Username:           user.Username,
		RealName:           user.RealName,
		Phone:              user.Phone,
		ParentAgentID:      item.ParentAgentID,
		ParentAgentName:    parentName,
		LevelDepth:         item.LevelDepth,
		RelationType:       item.RelationType,
		ContributionAmount: item.ContributionAmount,
		CommissionAmount:   item.CommissionAmount,
		Status:             item.Status,
		JoinedAt:           item.JoinedAt,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}, nil
}
