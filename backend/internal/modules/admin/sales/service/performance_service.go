package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/sales/dto"
	"hostsent/backend/internal/modules/admin/sales/model"
	"hostsent/backend/internal/modules/admin/sales/repository"
)

// PerformanceService 业绩排行与目标。
type PerformanceService interface {
	// Ranking 某周期业绩排行（含目标与达成率）。
	Ranking(ctx context.Context, scope dto.Scope, q dto.RankingQuery) (*dto.RankingResponse, error)
	// ListTargets 目标列表（含实时达成值）。
	ListTargets(ctx context.Context, scope dto.Scope, period string, departmentID uint64) (*dto.TargetListResponse, error)
	// UpsertTargets 按行 upsert 目标。
	UpsertTargets(ctx context.Context, scope dto.Scope, operatorID uint64, reqs []dto.TargetUpsertRequest) error
	// MyPerformance 我的业绩与目标达成。
	MyPerformance(ctx context.Context, adminID uint64, period string) (*dto.MyPerformance, error)
}

type performanceService struct {
	db       *gorm.DB
	commRepo repository.CommissionRepository
	relRepo  repository.RelationRepository
}

// NewPerformanceService 创建业绩服务。
func NewPerformanceService(db *gorm.DB, commRepo repository.CommissionRepository, relRepo repository.RelationRepository) *performanceService {
	return &performanceService{db: db, commRepo: commRepo, relRepo: relRepo}
}

func (s *performanceService) Ranking(ctx context.Context, scope dto.Scope, q dto.RankingQuery) (*dto.RankingResponse, error) {
	period, err := normalizePeriod(q.Period)
	if err != nil {
		return nil, err
	}
	deptID := q.DepartmentID
	switch {
	case scope.All:
		// 超管可看任意部门；deptID 保持前端传参。
	case scope.DepartmentID > 0:
		deptID = scope.DepartmentID
	case scope.AdminID > 0:
		// 销售只看自己的名次：用部门待会儿再过滤自己那一行。
		if d, err := s.relRepo.DepartmentOfAdmin(ctx, scope.AdminID); err == nil {
			deptID = d
		}
	}
	rows, err := s.commRepo.PerformanceRanking(ctx, period, deptID, 200)
	if err != nil {
		return nil, err
	}
	targets := map[uint64]model.SalesTarget{}
	if list, err := s.commRepo.ListTargets(ctx, period, 0); err == nil {
		for _, t := range list {
			if t.Scope == model.TargetScopeAdmin {
				targets[t.AdminID] = t.SalesTarget
			}
		}
	}
	restrictToAdmin := !scope.All && scope.AdminID > 0
	resp := &dto.RankingResponse{Period: period, Items: make([]dto.RankingRow, 0, len(rows))}
	for _, row := range rows {
		if restrictToAdmin && row.AdminID != scope.AdminID {
			continue
		}
		item := dto.RankingRow{
			AdminID:        row.AdminID,
			AdminName:      row.AdminName,
			AdminRealName:  row.AdminRealName,
			DepartmentID:   row.DepartmentID,
			DepartmentName: row.DepartmentName,
			Amount:         row.Amount,
			Orders:         row.Orders,
		}
		if t, ok := targets[row.AdminID]; ok {
			item.TargetAmount = t.TargetAmount
			item.TargetOrders = t.TargetOrders
			if t.TargetAmount > 0 {
				item.Achievement = round4(row.Amount / t.TargetAmount)
			}
		}
		resp.Items = append(resp.Items, item)
	}
	// 排序：默认按销售额，可切换按单量；同值按 admin_id 稳定排序避免名次抖动。
	sortBy := strings.TrimSpace(q.SortBy)
	sort.SliceStable(resp.Items, func(i, j int) bool {
		if sortBy == "orders" {
			if resp.Items[i].Orders != resp.Items[j].Orders {
				return resp.Items[i].Orders > resp.Items[j].Orders
			}
		} else if resp.Items[i].Amount != resp.Items[j].Amount {
			return resp.Items[i].Amount > resp.Items[j].Amount
		}
		return resp.Items[i].AdminID < resp.Items[j].AdminID
	})
	for i := range resp.Items {
		resp.Items[i].Rank = i + 1
	}
	return resp, nil
}

func (s *performanceService) ListTargets(ctx context.Context, scope dto.Scope, period string, departmentID uint64) (*dto.TargetListResponse, error) {
	p, err := normalizePeriod(period)
	if err != nil {
		return nil, err
	}
	deptID := departmentID
	if !scope.All && scope.DepartmentID > 0 {
		deptID = scope.DepartmentID
	}
	rows, err := s.commRepo.ListTargets(ctx, p, deptID)
	if err != nil {
		return nil, err
	}
	// 实际达成值：一次性取该周期全量排行，按 admin_id/部门建索引，避免逐行查库。
	actualByAdmin := map[uint64]repository.PerformanceRow{}
	actualByDept := map[uint64]*repository.PerformanceRow{}
	if ranking, err := s.commRepo.PerformanceRanking(ctx, p, 0, 200); err == nil {
		for _, r := range ranking {
			actualByAdmin[r.AdminID] = r
			if r.DepartmentID > 0 {
				if cur, ok := actualByDept[r.DepartmentID]; ok {
					cur.Amount += r.Amount
					cur.Orders += r.Orders
				} else {
					copied := r
					actualByDept[r.DepartmentID] = &copied
				}
			}
		}
	}
	items := make([]dto.TargetInfo, 0, len(rows))
	for _, row := range rows {
		info := dto.TargetInfo{
			ID:             row.ID,
			Period:         row.Period,
			Scope:          row.Scope,
			AdminID:        row.AdminID,
			AdminName:      row.AdminName,
			AdminRealName:  row.AdminRealName,
			DepartmentID:   row.DepartmentID,
			DepartmentName: row.DepartmentName,
			TargetAmount:   row.TargetAmount,
			TargetOrders:   row.TargetOrders,
		}
		if row.Scope == model.TargetScopeDepartment {
			if a, ok := actualByDept[row.DepartmentID]; ok {
				info.ActualAmount = a.Amount
				info.ActualOrders = a.Orders
			}
		} else if a, ok := actualByAdmin[row.AdminID]; ok {
			info.ActualAmount = a.Amount
			info.ActualOrders = a.Orders
		}
		if row.TargetAmount > 0 {
			info.Achievement = round4(info.ActualAmount / row.TargetAmount)
		}
		items = append(items, info)
	}
	return &dto.TargetListResponse{Period: p, Items: items}, nil
}

// UpsertTargets 目标下发。销售本人不得下发目标；主管只能给自己部门（个人目标归属本部门员工、
// 部门目标为本部门），超管不受限。
func (s *performanceService) UpsertTargets(ctx context.Context, scope dto.Scope, operatorID uint64, reqs []dto.TargetUpsertRequest) error {
	if !scope.All && scope.AdminID > 0 {
		return ErrOutOfScope
	}
	if len(reqs) == 0 {
		return ErrInvalidTarget
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, req := range reqs {
			period, err := normalizePeriod(req.Period)
			if err != nil {
				return err
			}
			scopeKind := strings.TrimSpace(req.Scope)
			if scopeKind == "" {
				scopeKind = model.TargetScopeAdmin
			}
			if scopeKind != model.TargetScopeAdmin && scopeKind != model.TargetScopeDepartment {
				return ErrInvalidTarget
			}
			if req.TargetAmount < 0 || req.TargetOrders < 0 {
				return ErrInvalidTarget
			}
			target := &model.SalesTarget{
				Period:       period,
				Scope:        scopeKind,
				TargetAmount: req.TargetAmount,
				TargetOrders: req.TargetOrders,
				CreatedBy:    operatorID,
			}
			switch scopeKind {
			case model.TargetScopeAdmin:
				if req.AdminID == 0 {
					return ErrInvalidTarget
				}
				brief, err := s.relRepo.AdminBrief(ctx, req.AdminID)
				if err != nil {
					return err
				}
				if brief == nil {
					return ErrInvalidSales
				}
				target.AdminID = req.AdminID
				if !scope.All && scope.DepartmentID > 0 && brief.DepartmentID != scope.DepartmentID {
					return ErrOutOfScope
				}
			case model.TargetScopeDepartment:
				if req.DepartmentID == 0 {
					return ErrInvalidTarget
				}
				if !scope.All && scope.DepartmentID > 0 && req.DepartmentID != scope.DepartmentID {
					return ErrOutOfScope
				}
				target.DepartmentID = req.DepartmentID
			}
			if err := s.commRepo.UpsertTarget(tx, target); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *performanceService) MyPerformance(ctx context.Context, adminID uint64, period string) (*dto.MyPerformance, error) {
	p, err := normalizePeriod(period)
	if err != nil {
		return nil, err
	}
	row, err := s.commRepo.AdminPerformance(ctx, adminID, p)
	if err != nil {
		return nil, err
	}
	me := &dto.MyPerformance{
		Period:         p,
		AdminID:        adminID,
		AdminName:      row.AdminName,
		DepartmentName: row.DepartmentName,
		Amount:         row.Amount,
		Orders:         row.Orders,
	}
	if brief, err := s.relRepo.AdminBrief(ctx, adminID); err == nil && brief != nil {
		me.AdminName = brief.Username
		me.DepartmentName = brief.DepartmentName
	}
	if t, err := s.commRepo.FindTarget(ctx, p, model.TargetScopeAdmin, adminID, 0); err == nil && t != nil {
		me.TargetAmount = t.TargetAmount
		me.TargetOrders = t.TargetOrders
		if t.TargetAmount > 0 {
			me.Achievement = round4(row.Amount / t.TargetAmount)
		}
	}
	// 名次：本部门内按销售额排名（与排行页同源同口径）。
	deptID := row.DepartmentID
	if deptID == 0 {
		deptID, _ = s.relRepo.DepartmentOfAdmin(ctx, adminID)
	}
	if ranking, err := s.commRepo.PerformanceRanking(ctx, p, deptID, 200); err == nil {
		for i, r := range ranking {
			if r.AdminID == adminID {
				me.Rank = i + 1
				break
			}
		}
	}
	if deptID > 0 {
		if n, err := s.relRepo.CountSalesInDepartment(ctx, deptID); err == nil {
			me.DeptMembers = int(n)
		}
	}
	if n, err := s.relRepo.CountActiveByAdmin(ctx, adminID); err == nil {
		me.ActiveCustomers = n
	}
	if n, err := s.relRepo.CountNewByAdminInPeriod(ctx, adminID, p); err == nil {
		me.MonthNewCustomers = n
	}
	return me, nil
}

// —— 内部工具 ——

// normalizePeriod 归一化 YYYY-MM；空值取当月。
func normalizePeriod(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Now().Format("2006-01"), nil
	}
	t, err := time.ParseInLocation("2006-01", v, time.Local)
	if err != nil {
		return "", ErrInvalidPeriod
	}
	return t.Format("2006-01"), nil
}

// round4 达成率保留四位小数（前端按百分比展示）。
func round4(v float64) float64 {
	return float64(int64(v*10000+0.5)) / 10000
}
