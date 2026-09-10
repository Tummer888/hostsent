// Package service 提供用户中心代理专区的业务编排（P6-03）。
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	distributionmodel "hostsent/backend/internal/modules/admin/user/distribution/model"
	"hostsent/backend/internal/modules/uc/agent/dto"
	"hostsent/backend/internal/modules/uc/agent/repository"
)

// ErrNotAgent 当前账号不是代理，禁止访问代理专区。
var ErrNotAgent = errors.New("当前账号不是代理")

// AgentService 代理专区业务能力。
type AgentService interface {
	// Profile 代理概览；非代理返回 ErrNotAgent。
	Profile(ctx context.Context, userID uint64) (*dto.ProfileInfo, error)
	// Stats 代理看板汇总（P7）。
	Stats(ctx context.Context, userID uint64) (*dto.StatsInfo, error)
	// Team 我的下级成员。
	Team(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.TeamListResponse, error)
	// Commissions 我的佣金记录。
	Commissions(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.CommissionListResponse, error)
	// Settlements 我的结算单。
	Settlements(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.SettlementListResponse, error)
}

type agentService struct {
	repo repository.Repository
}

// NewAgentService 创建代理专区服务。
func NewAgentService(repo repository.Repository) AgentService {
	return &agentService{repo: repo}
}

// agent 加载代理主体，非代理统一返回 ErrNotAgent（门禁：agents.user_id 存在即放行）。
func (s *agentService) agent(ctx context.Context, userID uint64) (*distributionmodel.Agent, error) {
	item, err := s.repo.AgentByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotAgent
		}
		return nil, err
	}
	return item, nil
}

func (s *agentService) Profile(ctx context.Context, userID uint64) (*dto.ProfileInfo, error) {
	agent, err := s.agent(ctx, userID)
	if err != nil {
		return nil, err
	}
	name, code, err := s.repo.LevelNameByID(ctx, agent.AgentLevelID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &dto.ProfileInfo{
		AgentID:          agent.ID,
		LevelName:        name,
		LevelCode:        code,
		InviteCode:       agent.InviteCode,
		InviteLink:       fmt.Sprintf("/register?invite_code=%s", agent.InviteCode),
		Status:           agent.Status,
		DirectSubCount:   agent.DirectSubCount,
		TeamSubCount:     agent.TeamSubCount,
		TotalCommission:  agent.TotalCommission,
		AvailableBalance: agent.AvailableBalance,
	}, nil
}

func (s *agentService) Team(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.TeamListResponse, error) {
	agent, err := s.agent(ctx, userID)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizePage(query)
	rows, total, err := s.repo.ListTeam(ctx, agent.ID, repository.TeamFilter{
		Status:     query.Status,
		LevelDepth: query.LevelDepth,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]dto.TeamMemberInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.TeamMemberInfo{
			ID:                 row.ID,
			UserID:             row.UserID,
			Username:           row.Username,
			Name:               row.Name,
			Email:              row.Email,
			LevelDepth:         row.LevelDepth,
			RelationType:       row.RelationType,
			ContributionAmount: row.ContributionAmount,
			CommissionAmount:   row.CommissionAmount,
			Status:             row.Status,
			JoinedAt:           formatTime(row.JoinedAt),
		})
	}
	return &dto.TeamListResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// Stats 代理看板汇总（P7）。
func (s *agentService) Stats(ctx context.Context, userID uint64) (*dto.StatsInfo, error) {
	agent, err := s.agent(ctx, userID)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.Stats(ctx, agent.ID)
	if err != nil {
		return nil, err
	}
	return &dto.StatsInfo{
		DirectSubCount:    row.DirectSubCount,
		TeamSubCount:      row.TeamSubCount,
		PendingCommission: row.PendingCommission,
		SettledCommission: row.SettledCommission,
		PaidSettlement:    row.PaidSettlement,
		PendingSettlement: row.PendingSettlement,
	}, nil
}

func (s *agentService) Commissions(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.CommissionListResponse, error) {
	agent, err := s.agent(ctx, userID)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizePage(query)
	items, total, err := s.repo.ListCommissions(ctx, agent.ID, query.Status, page, pageSize)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.CommissionInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, dto.CommissionInfo{
			ID:             item.ID,
			OrderNo:        item.OrderNo,
			CommissionType: item.CommissionType,
			SourceType:     item.SourceType,
			BaseAmount:     item.BaseAmount,
			Rate:           item.Rate,
			Amount:         item.Amount,
			Status:         item.Status,
			Remark:         item.Remark,
			CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		})
	}
	return &dto.CommissionListResponse{Items: resp, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *agentService) Settlements(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.SettlementListResponse, error) {
	agent, err := s.agent(ctx, userID)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizePage(query)
	items, total, err := s.repo.ListSettlements(ctx, agent.ID, query.Status, page, pageSize)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.SettlementInfo, 0, len(items))
	for _, item := range items {
		info := dto.SettlementInfo{
			ID:              item.ID,
			SettlementNo:    item.SettlementNo,
			PeriodStart:     item.PeriodStart.Format(time.RFC3339),
			PeriodEnd:       item.PeriodEnd.Format(time.RFC3339),
			CommissionTotal: item.CommissionTotal,
			DeductionTotal:  item.DeductionTotal,
			PayableTotal:    item.PayableTotal,
			Status:          item.Status,
			CreatedAt:       item.CreatedAt.Format(time.RFC3339),
		}
		if item.PaidAt != nil {
			info.PaidAt = item.PaidAt.Format(time.RFC3339)
		}
		resp = append(resp, info)
	}
	return &dto.SettlementListResponse{Items: resp, Page: page, PageSize: pageSize, Total: total}, nil
}

func normalizePage(query dto.ListQuery) (int, int) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}
