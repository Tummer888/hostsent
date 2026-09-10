// Package repository 提供用户中心代理专区的数据访问实现（P6-03）。
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	distributionmodel "hostsent/backend/internal/modules/admin/user/distribution/model"
)

// TeamMemberRow 下级成员连表查询结果。
type TeamMemberRow struct {
	ID                 uint64
	UserID             uint64
	Username           string
	Name               string
	Email              string
	LevelDepth         int
	RelationType       string
	ContributionAmount float64
	CommissionAmount   float64
	Status             string
	JoinedAt           *time.Time
}

// StatsRow 代理后台看板汇总（P7）：团队规模与佣金/结算金额分布。
type StatsRow struct {
	DirectSubCount    int64
	TeamSubCount      int64
	PendingCommission float64
	SettledCommission float64
	PaidSettlement    float64
	PendingSettlement float64
}

// TeamFilter 下级成员筛选条件（P7 增加层级）。
type TeamFilter struct {
	Status     string
	LevelDepth int // 0 表示全部层级
	Page       int
	PageSize   int
}

// Repository 代理专区只读数据访问。
type Repository interface {
	// AgentByUserID 按用户查代理；非代理返回 gorm.ErrRecordNotFound。
	AgentByUserID(ctx context.Context, userID uint64) (*distributionmodel.Agent, error)
	// LevelNameByID 返回代理等级名称与编码。
	LevelNameByID(ctx context.Context, id uint64) (name, code string, err error)
	// ListTeam 下级成员分页。
	ListTeam(ctx context.Context, agentID uint64, filter TeamFilter) ([]TeamMemberRow, int64, error)
	// ListCommissions 佣金记录分页。
	ListCommissions(ctx context.Context, agentID uint64, status string, page, pageSize int) ([]distributionmodel.Commission, int64, error)
	// ListSettlements 结算单分页。
	ListSettlements(ctx context.Context, agentID uint64, status string, page, pageSize int) ([]distributionmodel.Settlement, int64, error)
	// Stats 代理看板汇总。
	Stats(ctx context.Context, agentID uint64) (*StatsRow, error)
}

type agentRepository struct {
	db *gorm.DB
}

// NewAgentRepository 创建代理专区仓储实现。
func NewAgentRepository(db *gorm.DB) Repository {
	return &agentRepository{db: db}
}

func (r *agentRepository) AgentByUserID(ctx context.Context, userID uint64) (*distributionmodel.Agent, error) {
	var agent distributionmodel.Agent
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&agent).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *agentRepository) LevelNameByID(ctx context.Context, id uint64) (string, string, error) {
	var level distributionmodel.AgentLevel
	if err := r.db.WithContext(ctx).First(&level, id).Error; err != nil {
		return "", "", err
	}
	return level.Name, level.Code, nil
}

func (r *agentRepository) ListTeam(ctx context.Context, agentID uint64, filter TeamFilter) ([]TeamMemberRow, int64, error) {
	base := r.db.WithContext(ctx).
		Table("distribution_subordinates").
		Joins("LEFT JOIN users ON users.id = distribution_subordinates.user_id").
		Where("distribution_subordinates.agent_id = ?", agentID)
	if s := strings.TrimSpace(filter.Status); s != "" {
		base = base.Where("distribution_subordinates.status = ?", s)
	}
	if filter.LevelDepth > 0 {
		base = base.Where("distribution_subordinates.level_depth = ?", filter.LevelDepth)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	var rows []TeamMemberRow
	if err := base.
		Select(`distribution_subordinates.id,
			distribution_subordinates.user_id,
			users.username,
			users.real_name AS name,
			users.email,
			distribution_subordinates.level_depth,
			distribution_subordinates.relation_type,
			distribution_subordinates.contribution_amount,
			distribution_subordinates.commission_amount,
			distribution_subordinates.status,
			distribution_subordinates.joined_at`).
		Order("distribution_subordinates.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Stats 汇总代理的团队规模与佣金/结算金额（P7 看板）。
func (r *agentRepository) Stats(ctx context.Context, agentID uint64) (*StatsRow, error) {
	var row StatsRow

	if err := r.db.WithContext(ctx).
		Model(&distributionmodel.Subordinate{}).
		Where("agent_id = ?", agentID).
		Count(&row.TeamSubCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).
		Model(&distributionmodel.Subordinate{}).
		Where("agent_id = ? AND level_depth <= 1", agentID).
		Count(&row.DirectSubCount).Error; err != nil {
		return nil, err
	}

	type sumRow struct {
		Status string
		Total  float64
	}
	var commissionSums []sumRow
	if err := r.db.WithContext(ctx).
		Model(&distributionmodel.Commission{}).
		Select("status, COALESCE(SUM(amount), 0) AS total").
		Where("agent_id = ?", agentID).
		Group("status").
		Scan(&commissionSums).Error; err != nil {
		return nil, err
	}
	for _, s := range commissionSums {
		switch s.Status {
		case "pending":
			row.PendingCommission = s.Total
		default:
			row.SettledCommission += s.Total
		}
	}

	var settlementSums []sumRow
	if err := r.db.WithContext(ctx).
		Model(&distributionmodel.Settlement{}).
		Select("status, COALESCE(SUM(payable_total), 0) AS total").
		Where("agent_id = ?", agentID).
		Group("status").
		Scan(&settlementSums).Error; err != nil {
		return nil, err
	}
	for _, s := range settlementSums {
		if s.Status == "paid" {
			row.PaidSettlement = s.Total
		} else {
			row.PendingSettlement += s.Total
		}
	}

	return &row, nil
}

func (r *agentRepository) ListCommissions(ctx context.Context, agentID uint64, status string, page, pageSize int) ([]distributionmodel.Commission, int64, error) {
	base := r.db.WithContext(ctx).Model(&distributionmodel.Commission{}).Where("agent_id = ?", agentID)
	if s := strings.TrimSpace(status); s != "" {
		base = base.Where("status = ?", s)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []distributionmodel.Commission
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *agentRepository) ListSettlements(ctx context.Context, agentID uint64, status string, page, pageSize int) ([]distributionmodel.Settlement, int64, error) {
	base := r.db.WithContext(ctx).Model(&distributionmodel.Settlement{}).Where("agent_id = ?", agentID)
	if s := strings.TrimSpace(status); s != "" {
		base = base.Where("status = ?", s)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []distributionmodel.Settlement
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
