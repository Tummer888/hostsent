package service

import (
	"context"

	distributiondto "hostsent/backend/internal/modules/distribution/dto"
	distributionmodel "hostsent/backend/internal/modules/distribution/model"
	distributionrepo "hostsent/backend/internal/modules/distribution/repository"
	userrepo "hostsent/backend/internal/modules/user/repository"
)

type AgentService interface {
	List(ctx context.Context, query distributiondto.AgentListQuery) (*distributiondto.AgentListResponse, error)
	FindByID(ctx context.Context, id uint64) (*distributiondto.AgentInfo, error)
	Create(ctx context.Context, req distributiondto.AgentCreateRequest) (*distributiondto.AgentInfo, error)
	Update(ctx context.Context, id uint64, req distributiondto.AgentUpdateRequest) (*distributiondto.AgentInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type agentService struct {
	repo           distributionrepo.AgentRepository
	userRepo       userrepo.UserRepository
	agentLevelRepo distributionrepo.AgentLevelRepository
}

func NewAgentService(repo distributionrepo.AgentRepository, userRepo userrepo.UserRepository, agentLevelRepo distributionrepo.AgentLevelRepository) AgentService {
	return &agentService{repo: repo, userRepo: userRepo, agentLevelRepo: agentLevelRepo}
}

func (s *agentService) List(ctx context.Context, query distributiondto.AgentListQuery) (*distributiondto.AgentListResponse, error) {
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
	respItems := make([]distributiondto.AgentInfo, 0, len(items))
	for _, item := range items {
		resp, err := s.buildAgentInfo(ctx, item)
		if err != nil {
			return nil, err
		}
		respItems = append(respItems, resp)
	}
	return &distributiondto.AgentListResponse{Items: respItems, Meta: distributiondto.AgentListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *agentService) FindByID(ctx context.Context, id uint64) (*distributiondto.AgentInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := s.buildAgentInfo(ctx, *item)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *agentService) Create(ctx context.Context, req distributiondto.AgentCreateRequest) (*distributiondto.AgentInfo, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	item := &distributionmodel.Agent{
		UserID:           req.UserID,
		AgentLevelID:     req.AgentLevelID,
		InviterAgentID:   req.InviterAgentID,
		InviteCode:       req.InviteCode,
		Status:           status,
		DirectSubCount:   req.DirectSubCount,
		TeamSubCount:     req.TeamSubCount,
		TotalCommission:  req.TotalCommission,
		AvailableBalance: req.AvailableBalance,
		LastSettledAt:    req.LastSettledAt,
		Remark:           req.Remark,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *agentService) Update(ctx context.Context, id uint64, req distributiondto.AgentUpdateRequest) (*distributiondto.AgentInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.AgentLevelID = req.AgentLevelID
	item.InviterAgentID = req.InviterAgentID
	item.InviteCode = req.InviteCode
	item.Status = req.Status
	item.DirectSubCount = req.DirectSubCount
	item.TeamSubCount = req.TeamSubCount
	item.TotalCommission = req.TotalCommission
	item.AvailableBalance = req.AvailableBalance
	item.LastSettledAt = req.LastSettledAt
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *agentService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *agentService) buildAgentInfo(ctx context.Context, item distributionmodel.Agent) (distributiondto.AgentInfo, error) {
	user, err := s.userRepo.FindByID(ctx, item.UserID)
	if err != nil {
		return distributiondto.AgentInfo{}, err
	}
	level, err := s.agentLevelRepo.FindByID(ctx, item.AgentLevelID)
	if err != nil {
		return distributiondto.AgentInfo{}, err
	}
	inviterName := ""
	if item.InviterAgentID != nil {
		inviter, err := s.repo.FindByID(ctx, *item.InviterAgentID)
		if err != nil {
			return distributiondto.AgentInfo{}, err
		}
		inviterUser, err := s.userRepo.FindByID(ctx, inviter.UserID)
		if err != nil {
			return distributiondto.AgentInfo{}, err
		}
		if inviterUser.RealName != "" {
			inviterName = inviterUser.RealName
		} else {
			inviterName = inviterUser.Username
		}
	}
	name := user.Username
	if user.RealName != "" {
		name = user.RealName
	}
	return distributiondto.AgentInfo{
		ID:               item.ID,
		UserID:           item.UserID,
		Username:         user.Username,
		RealName:         name,
		Email:            user.Email,
		Phone:            user.Phone,
		AgentLevelID:     item.AgentLevelID,
		AgentLevelName:   level.Name,
		InviterAgentID:   item.InviterAgentID,
		InviterAgentName: inviterName,
		InviteCode:       item.InviteCode,
		Status:           item.Status,
		DirectSubCount:   item.DirectSubCount,
		TeamSubCount:     item.TeamSubCount,
		TotalCommission:  item.TotalCommission,
		AvailableBalance: item.AvailableBalance,
		LastSettledAt:    item.LastSettledAt,
		Remark:           item.Remark,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}, nil
}
