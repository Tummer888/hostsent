package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	distributiondto "hostsent/backend/internal/modules/distribution/dto"
	distributionmodel "hostsent/backend/internal/modules/distribution/model"
	distributionrepo "hostsent/backend/internal/modules/distribution/repository"
	userrepo "hostsent/backend/internal/modules/user/repository"
)

var (
	ErrInvalidCommissionStatus   = errors.New("invalid commission status")
	ErrCommissionStatusUnchanged = errors.New("commission status cannot be changed")
	commissionStatusTransitions  = map[string]map[string]struct{}{
		distributiondto.CommissionStatusPending: {
			distributiondto.CommissionStatusFrozen:    {},
			distributiondto.CommissionStatusAvailable: {},
			distributiondto.CommissionStatusCancelled: {},
		},
		distributiondto.CommissionStatusFrozen: {
			distributiondto.CommissionStatusAvailable: {},
			distributiondto.CommissionStatusCancelled: {},
		},
		distributiondto.CommissionStatusAvailable: {
			distributiondto.CommissionStatusSettled:   {},
			distributiondto.CommissionStatusCancelled: {},
		},
	}
)

type CommissionService interface {
	List(ctx context.Context, query distributiondto.CommissionListQuery) (*distributiondto.CommissionListResponse, error)
	FindByID(ctx context.Context, id uint64) (*distributiondto.CommissionInfo, error)
	Create(ctx context.Context, req distributiondto.CommissionCreateRequest) (*distributiondto.CommissionInfo, error)
	Update(ctx context.Context, id uint64, req distributiondto.CommissionUpdateRequest) (*distributiondto.CommissionInfo, error)
	Freeze(ctx context.Context, id uint64, req distributiondto.CommissionStatusChangeRequest) (*distributiondto.CommissionInfo, error)
	Unfreeze(ctx context.Context, id uint64, req distributiondto.CommissionStatusChangeRequest) (*distributiondto.CommissionInfo, error)
	Cancel(ctx context.Context, id uint64, req distributiondto.CommissionStatusChangeRequest) (*distributiondto.CommissionInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type commissionService struct {
	repo            distributionrepo.CommissionRepository
	agentRepo       distributionrepo.AgentRepository
	subordinateRepo distributionrepo.SubordinateRepository
	userRepo        userrepo.UserRepository
}

func NewCommissionService(repo distributionrepo.CommissionRepository, agentRepo distributionrepo.AgentRepository, subordinateRepo distributionrepo.SubordinateRepository, userRepo userrepo.UserRepository) CommissionService {
	return &commissionService{repo: repo, agentRepo: agentRepo, subordinateRepo: subordinateRepo, userRepo: userRepo}
}

func (s *commissionService) List(ctx context.Context, query distributiondto.CommissionListQuery) (*distributiondto.CommissionListResponse, error) {
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
	respItems := make([]distributiondto.CommissionInfo, 0, len(items))
	for _, item := range items {
		resp, err := s.buildCommissionInfo(ctx, item)
		if err != nil {
			return nil, err
		}
		respItems = append(respItems, resp)
	}
	return &distributiondto.CommissionListResponse{Items: respItems, Meta: distributiondto.CommissionListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *commissionService) FindByID(ctx context.Context, id uint64) (*distributiondto.CommissionInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := s.buildCommissionInfo(ctx, *item)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *commissionService) Create(ctx context.Context, req distributiondto.CommissionCreateRequest) (*distributiondto.CommissionInfo, error) {
	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = "order"
	}
	commissionType := req.CommissionType
	if commissionType == "" {
		commissionType = "direct"
	}
	status := req.Status
	if status == "" {
		status = distributiondto.CommissionStatusPending
	}
	if err := validateCommissionStatus(status); err != nil {
		return nil, err
	}
	freezeUntil, settledAt := normalizeCommissionTimes(status, req.FreezeUntil, req.SettledAt)
	item := &distributionmodel.Commission{
		AgentID:        req.AgentID,
		SubordinateID:  req.SubordinateID,
		SettlementID:   req.SettlementID,
		OrderNo:        req.OrderNo,
		SourceType:     sourceType,
		CommissionType: commissionType,
		BaseAmount:     req.BaseAmount,
		Rate:           req.Rate,
		Amount:         req.Amount,
		Status:         status,
		FreezeUntil:    freezeUntil,
		SettledAt:      settledAt,
		Remark:         req.Remark,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *commissionService) Update(ctx context.Context, id uint64, req distributiondto.CommissionUpdateRequest) (*distributiondto.CommissionInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validateCommissionStatus(req.Status); err != nil {
		return nil, err
	}
	if err := ensureCommissionStatusTransition(item.Status, req.Status); err != nil {
		return nil, err
	}
	freezeUntil, settledAt := normalizeCommissionTimes(req.Status, req.FreezeUntil, req.SettledAt)
	item.SubordinateID = req.SubordinateID
	item.SettlementID = req.SettlementID
	item.OrderNo = req.OrderNo
	item.SourceType = req.SourceType
	item.CommissionType = req.CommissionType
	item.BaseAmount = req.BaseAmount
	item.Rate = req.Rate
	item.Amount = req.Amount
	item.Status = req.Status
	item.FreezeUntil = freezeUntil
	item.SettledAt = settledAt
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *commissionService) Freeze(ctx context.Context, id uint64, req distributiondto.CommissionStatusChangeRequest) (*distributiondto.CommissionInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureCommissionStatusTransition(item.Status, distributiondto.CommissionStatusFrozen); err != nil {
		return nil, err
	}
	freezeUntil := req.FreezeUntil
	if freezeUntil == nil {
		now := time.Now().Add(7 * 24 * time.Hour)
		freezeUntil = &now
	}
	item.Status = distributiondto.CommissionStatusFrozen
	item.FreezeUntil = freezeUntil
	item.SettledAt = nil
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *commissionService) Unfreeze(ctx context.Context, id uint64, req distributiondto.CommissionStatusChangeRequest) (*distributiondto.CommissionInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureCommissionStatusTransition(item.Status, distributiondto.CommissionStatusAvailable); err != nil {
		return nil, err
	}
	item.Status = distributiondto.CommissionStatusAvailable
	item.FreezeUntil = nil
	item.SettledAt = nil
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *commissionService) Cancel(ctx context.Context, id uint64, req distributiondto.CommissionStatusChangeRequest) (*distributiondto.CommissionInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureCommissionStatusTransition(item.Status, distributiondto.CommissionStatusCancelled); err != nil {
		return nil, err
	}
	item.Status = distributiondto.CommissionStatusCancelled
	item.FreezeUntil = nil
	item.SettledAt = nil
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *commissionService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *commissionService) buildCommissionInfo(ctx context.Context, item distributionmodel.Commission) (distributiondto.CommissionInfo, error) {
	agent, err := s.agentRepo.FindByID(ctx, item.AgentID)
	if err != nil {
		return distributiondto.CommissionInfo{}, err
	}
	agentUser, err := s.userRepo.FindByID(ctx, agent.UserID)
	if err != nil {
		return distributiondto.CommissionInfo{}, err
	}
	agentName := agentUser.Username
	if agentUser.RealName != "" {
		agentName = agentUser.RealName
	}
	subordinateName := ""
	if item.SubordinateID != nil {
		subordinate, err := s.subordinateRepo.FindByID(ctx, *item.SubordinateID)
		if err != nil {
			return distributiondto.CommissionInfo{}, err
		}
		user, err := s.userRepo.FindByID(ctx, subordinate.UserID)
		if err != nil {
			return distributiondto.CommissionInfo{}, err
		}
		if user.RealName != "" {
			subordinateName = user.RealName
		} else {
			subordinateName = user.Username
		}
	}
	return distributiondto.CommissionInfo{
		ID:              item.ID,
		AgentID:         item.AgentID,
		AgentName:       agentName,
		SubordinateID:   item.SubordinateID,
		SubordinateName: subordinateName,
		SettlementID:    item.SettlementID,
		OrderNo:         item.OrderNo,
		SourceType:      item.SourceType,
		CommissionType:  item.CommissionType,
		BaseAmount:      item.BaseAmount,
		Rate:            item.Rate,
		Amount:          item.Amount,
		Status:          item.Status,
		FreezeUntil:     item.FreezeUntil,
		SettledAt:       item.SettledAt,
		Remark:          item.Remark,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}, nil
}

func validateCommissionStatus(status string) error {
	if _, ok := distributiondto.CommissionStatuses[status]; !ok {
		return fmt.Errorf("%w: %s", ErrInvalidCommissionStatus, status)
	}
	return nil
}

func ensureCommissionStatusTransition(from, to string) error {
	if from == to {
		return nil
	}
	allowedTargets, ok := commissionStatusTransitions[from]
	if !ok {
		return ErrCommissionStatusUnchanged
	}
	if _, ok := allowedTargets[to]; !ok {
		return fmt.Errorf("commission status cannot change from %s to %s", from, to)
	}
	return nil
}

func normalizeCommissionTimes(status string, freezeUntil, settledAt *time.Time) (*time.Time, *time.Time) {
	switch status {
	case distributiondto.CommissionStatusFrozen:
		if freezeUntil == nil {
			now := time.Now().Add(7 * 24 * time.Hour)
			freezeUntil = &now
		}
		return freezeUntil, nil
	case distributiondto.CommissionStatusSettled:
		if settledAt == nil {
			now := time.Now()
			settledAt = &now
		}
		return nil, settledAt
	default:
		return nil, nil
	}
}
