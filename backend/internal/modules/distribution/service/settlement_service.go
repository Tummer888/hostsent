package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	distributiondto "hostsent/backend/internal/modules/distribution/dto"
	distributionmodel "hostsent/backend/internal/modules/distribution/model"
	distributionrepo "hostsent/backend/internal/modules/distribution/repository"
	userrepo "hostsent/backend/internal/modules/user/repository"
)

var (
	ErrInvalidSettlementStatus = errors.New("invalid settlement status")
	ErrSettlementStatusChange  = errors.New("settlement status cannot be changed")
)

type SettlementService interface {
	List(ctx context.Context, query distributiondto.SettlementListQuery) (*distributiondto.SettlementListResponse, error)
	FindByID(ctx context.Context, id uint64) (*distributiondto.SettlementInfo, error)
	Create(ctx context.Context, req distributiondto.SettlementCreateRequest) (*distributiondto.SettlementInfo, error)
	Update(ctx context.Context, id uint64, req distributiondto.SettlementUpdateRequest) (*distributiondto.SettlementInfo, error)
	Confirm(ctx context.Context, id uint64, req distributiondto.SettlementStatusChangeRequest) (*distributiondto.SettlementInfo, error)
	Pay(ctx context.Context, id uint64, req distributiondto.SettlementStatusChangeRequest) (*distributiondto.SettlementInfo, error)
	Cancel(ctx context.Context, id uint64, req distributiondto.SettlementStatusChangeRequest) (*distributiondto.SettlementInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type settlementService struct {
	repo       distributionrepo.SettlementRepository
	agentRepo  distributionrepo.AgentRepository
	userRepo   userrepo.UserRepository
	commission distributionrepo.CommissionRepository
}

func NewSettlementService(repo distributionrepo.SettlementRepository, agentRepo distributionrepo.AgentRepository, userRepo userrepo.UserRepository, commissionRepo distributionrepo.CommissionRepository) SettlementService {
	return &settlementService{repo: repo, agentRepo: agentRepo, userRepo: userRepo, commission: commissionRepo}
}

func (s *settlementService) List(ctx context.Context, query distributiondto.SettlementListQuery) (*distributiondto.SettlementListResponse, error) {
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
	respItems := make([]distributiondto.SettlementInfo, 0, len(items))
	for _, item := range items {
		resp, err := s.buildSettlementInfo(ctx, item)
		if err != nil {
			return nil, err
		}
		respItems = append(respItems, resp)
	}
	return &distributiondto.SettlementListResponse{Items: respItems, Meta: distributiondto.SettlementListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *settlementService) FindByID(ctx context.Context, id uint64) (*distributiondto.SettlementInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := s.buildSettlementInfo(ctx, *item)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *settlementService) Create(ctx context.Context, req distributiondto.SettlementCreateRequest) (*distributiondto.SettlementInfo, error) {
	if req.PeriodEnd.Before(req.PeriodStart) {
		return nil, fmt.Errorf("period_end cannot be earlier than period_start")
	}
	settlementNo := strings.TrimSpace(req.SettlementNo)
	if settlementNo == "" {
		settlementNo = fmt.Sprintf("SET-%d", time.Now().UnixNano())
	}
	if _, err := s.repo.FindBySettlementNo(ctx, settlementNo); err == nil {
		return nil, fmt.Errorf("settlement_no already exists: %s", settlementNo)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	commissionItems, totalAmount, err := s.collectCommissions(ctx, req.AgentID, req.CommissionIDs, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return nil, err
	}
	payable := totalAmount - req.DeductionTotal
	if payable < 0 {
		payable = 0
	}
	item := &distributionmodel.Settlement{
		AgentID:         req.AgentID,
		SettlementNo:    settlementNo,
		PeriodStart:     req.PeriodStart,
		PeriodEnd:       req.PeriodEnd,
		CommissionTotal: totalAmount,
		DeductionTotal:  req.DeductionTotal,
		PayableTotal:    payable,
		Status:          distributiondto.SettlementStatusDraft,
		Remark:          req.Remark,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	for _, commissionItem := range commissionItems {
		commissionItem.Status = distributiondto.CommissionStatusSettled
		commissionItem.SettlementID = &item.ID
		now := time.Now()
		commissionItem.SettledAt = &now
		if err := s.commission.Update(ctx, &commissionItem); err != nil {
			return nil, err
		}
	}
	return s.FindByID(ctx, item.ID)
}

func (s *settlementService) Update(ctx context.Context, id uint64, req distributiondto.SettlementUpdateRequest) (*distributiondto.SettlementInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Status != distributiondto.SettlementStatusDraft {
		return nil, ErrSettlementStatusChange
	}
	if req.PeriodEnd.Before(req.PeriodStart) {
		return nil, fmt.Errorf("period_end cannot be earlier than period_start")
	}
	item.PeriodStart = req.PeriodStart
	item.PeriodEnd = req.PeriodEnd
	item.DeductionTotal = req.DeductionTotal
	item.PayableTotal = item.CommissionTotal - req.DeductionTotal
	if item.PayableTotal < 0 {
		item.PayableTotal = 0
	}
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *settlementService) Confirm(ctx context.Context, id uint64, req distributiondto.SettlementStatusChangeRequest) (*distributiondto.SettlementInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureSettlementStatusTransition(item.Status, distributiondto.SettlementStatusConfirmed); err != nil {
		return nil, err
	}
	now := time.Now()
	item.Status = distributiondto.SettlementStatusConfirmed
	item.ConfirmedBy = req.ConfirmedBy
	item.ConfirmedAt = &now
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *settlementService) Pay(ctx context.Context, id uint64, req distributiondto.SettlementStatusChangeRequest) (*distributiondto.SettlementInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureSettlementStatusTransition(item.Status, distributiondto.SettlementStatusPaid); err != nil {
		return nil, err
	}
	now := time.Now()
	if item.ConfirmedAt == nil {
		item.ConfirmedAt = &now
	}
	if item.ConfirmedBy == nil {
		item.ConfirmedBy = req.ConfirmedBy
	}
	item.Status = distributiondto.SettlementStatusPaid
	item.PaidAt = &now
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *settlementService) Cancel(ctx context.Context, id uint64, req distributiondto.SettlementStatusChangeRequest) (*distributiondto.SettlementInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureSettlementStatusTransition(item.Status, distributiondto.SettlementStatusCancelled); err != nil {
		return nil, err
	}
	item.Status = distributiondto.SettlementStatusCancelled
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *settlementService) Delete(ctx context.Context, id uint64) error {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if item.Status != distributiondto.SettlementStatusDraft && item.Status != distributiondto.SettlementStatusCancelled {
		return ErrSettlementStatusChange
	}
	return s.repo.Delete(ctx, id)
}

func (s *settlementService) buildSettlementInfo(ctx context.Context, item distributionmodel.Settlement) (distributiondto.SettlementInfo, error) {
	agent, err := s.agentRepo.FindByID(ctx, item.AgentID)
	if err != nil {
		return distributiondto.SettlementInfo{}, err
	}
	agentUser, err := s.userRepo.FindByID(ctx, agent.UserID)
	if err != nil {
		return distributiondto.SettlementInfo{}, err
	}
	agentName := agentUser.Username
	if agentUser.RealName != "" {
		agentName = agentUser.RealName
	}
	confirmedByName := ""
	if item.ConfirmedBy != nil {
		user, err := s.userRepo.FindByID(ctx, *item.ConfirmedBy)
		if err == nil {
			confirmedByName = user.Username
			if user.RealName != "" {
				confirmedByName = user.RealName
			}
		}
	}
	commissionCount, err := s.countSettlementCommissions(ctx, item.ID)
	if err != nil {
		return distributiondto.SettlementInfo{}, err
	}
	return distributiondto.SettlementInfo{
		ID:              item.ID,
		AgentID:         item.AgentID,
		AgentName:       agentName,
		SettlementNo:    item.SettlementNo,
		PeriodStart:     item.PeriodStart,
		PeriodEnd:       item.PeriodEnd,
		CommissionTotal: item.CommissionTotal,
		DeductionTotal:  item.DeductionTotal,
		PayableTotal:    item.PayableTotal,
		CommissionCount: commissionCount,
		Status:          item.Status,
		ConfirmedBy:     item.ConfirmedBy,
		ConfirmedByName: confirmedByName,
		ConfirmedAt:     item.ConfirmedAt,
		PaidAt:          item.PaidAt,
		Remark:          item.Remark,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}, nil
}

func (s *settlementService) collectCommissions(ctx context.Context, agentID uint64, commissionIDs []uint64, periodStart, periodEnd time.Time) ([]distributionmodel.Commission, float64, error) {
	query := distributiondto.CommissionListQuery{
		Page:     1,
		PageSize: 100,
		AgentID:  agentID,
		Status:   distributiondto.CommissionStatusAvailable,
	}
	items, _, err := s.commission.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	selected := make([]distributionmodel.Commission, 0)
	selectedMap := make(map[uint64]struct{}, len(commissionIDs))
	for _, id := range commissionIDs {
		selectedMap[id] = struct{}{}
	}
	total := 0.0
	for _, item := range items {
		if item.CreatedAt.Before(periodStart) || item.CreatedAt.After(periodEnd.Add(24*time.Hour-time.Nanosecond)) {
			continue
		}
		if item.SettlementID != nil {
			continue
		}
		if len(selectedMap) > 0 {
			if _, ok := selectedMap[item.ID]; !ok {
				continue
			}
		}
		selected = append(selected, item)
		total += item.Amount
	}
	return selected, total, nil
}

func (s *settlementService) countSettlementCommissions(ctx context.Context, settlementID uint64) (int, error) {
	items, _, err := s.commission.List(ctx, distributiondto.CommissionListQuery{Page: 1, PageSize: 100})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		if item.SettlementID != nil && *item.SettlementID == settlementID {
			count++
		}
	}
	return count, nil
}

func ensureSettlementStatusTransition(from, to string) error {
	if _, ok := distributiondto.SettlementStatuses[from]; !ok {
		return ErrInvalidSettlementStatus
	}
	switch from {
	case distributiondto.SettlementStatusDraft:
		if to == distributiondto.SettlementStatusConfirmed || to == distributiondto.SettlementStatusCancelled {
			return nil
		}
	case distributiondto.SettlementStatusConfirmed:
		if to == distributiondto.SettlementStatusPaid || to == distributiondto.SettlementStatusCancelled {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrSettlementStatusChange, from, to)
}
