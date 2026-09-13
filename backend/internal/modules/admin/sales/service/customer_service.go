// Package service 提供销售归属与提成的业务编排（doc86 §3.4–3.6）。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/sales/dto"
	"hostsent/backend/internal/modules/admin/sales/model"
	"hostsent/backend/internal/modules/admin/sales/repository"
)

// CustomerService 客户归属：分配、释放、列表与订单归属解析。
//
// 订单归属解析（ResolveForNewOrder/ResolveForRenewal）会被 uc 订单服务与续费服务
// 通过装配层注入的 port 调用，因此本服务的方法必须保证「查不到就返回 0，不阻断下单」。
type CustomerService interface {
	// ResolveForNewOrder 新单归属：取客户现役归属销售；无归属返回 0。
	ResolveForNewOrder(ctx context.Context, userID uint64) uint64
	// ResolveForRenewal 续费归属：默认跟源订单快照，配置切 follow_owner 时跟现役归属。
	ResolveForRenewal(ctx context.Context, userID, srcOrderID uint64) uint64

	// Assign 分配/变更归属（保护期校验 + 旧关系释放 + 新关系生效 + users 快照回写）。
	Assign(ctx context.Context, scope dto.Scope, operatorID uint64, operatorName string, req dto.AssignRequest) error
	// Release 释放归属（保护期校验）。
	Release(ctx context.Context, scope dto.Scope, operatorID uint64, operatorName string, req dto.ReleaseRequest) error

	List(ctx context.Context, scope dto.Scope, q dto.CustomerQuery) (*dto.CustomerListResponse, error)
	Unassigned(ctx context.Context, q dto.CustomerQuery) (*dto.UnassignedListResponse, error)
	Relations(ctx context.Context, userID uint64) (*dto.RelationListResponse, error)
	// Candidates 可分配销售下拉（在职且开启销售能力）。
	Candidates(ctx context.Context, departmentID uint64) (*dto.SalesCandidateListResponse, error)

	// EnsureAutoClaim 新客户注册时若无归属，按部门兜底分配给「客户数最少的在职销售」。
	// 未配置销售时静默跳过，不阻断注册。
	EnsureAutoClaim(ctx context.Context, userID uint64) error
	// TransferOnResign 员工离职：把名下现役客户转给同部门其他在职销售（无候选则释放）。
	TransferOnResign(ctx context.Context, adminID uint64, operatorID uint64) (int, error)
}

type customerService struct {
	db       *gorm.DB
	relRepo  repository.RelationRepository
	commRepo repository.CommissionRepository
	logger   *zap.Logger
}

// NewCustomerService 创建客户归属服务。
func NewCustomerService(db *gorm.DB, relRepo repository.RelationRepository, commRepo repository.CommissionRepository, logger *zap.Logger) *customerService {
	return &customerService{db: db, relRepo: relRepo, commRepo: commRepo, logger: logger}
}

// —— 订单归属解析 ——

// ResolveForNewOrder 取客户现役归属。查询失败只记日志并返回 0：
// 下单主流程不因归属解析失败而失败（归属是附加信息，不是成交前提）。
func (s *customerService) ResolveForNewOrder(ctx context.Context, userID uint64) uint64 {
	if userID == 0 {
		return 0
	}
	rel, err := s.relRepo.FindActiveByUser(ctx, userID)
	if err != nil {
		s.logger.Warn("sales owner resolve failed", zap.Uint64("user_id", userID), zap.Error(err))
		return 0
	}
	if rel == nil {
		return 0
	}
	return rel.AdminID
}

// ResolveForRenewal 续费归属：默认跟单（源订单快照），可切跟现役归属。
// 源订单无归属快照时（存量订单/上游同步单）回落到现役归属，避免续费提成凭空丢失。
func (s *customerService) ResolveForRenewal(ctx context.Context, userID, srcOrderID uint64) uint64 {
	rates, err := s.commRepo.Rates(ctx)
	if err != nil {
		// 配置读取失败按默认「跟单」处理：默认档不会把提成错记到别人名下。
		s.logger.Warn("sales rates read failed, fallback to follow_order", zap.Error(err))
		rates = model.DefaultRates()
	}
	if rates.RenewalMode == model.RenewalModeFollowOwner {
		return s.ResolveForNewOrder(ctx, userID)
	}
	if srcOrderID > 0 {
		if owner, err := s.commRepo.OrderSalesOwner(ctx, srcOrderID); err == nil && owner > 0 {
			return owner
		}
	}
	return s.ResolveForNewOrder(ctx, userID)
}

// —— 分配与释放 ——

func (s *customerService) Assign(ctx context.Context, scope dto.Scope, operatorID uint64, operatorName string, req dto.AssignRequest) error {
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) < 5 {
		return ErrReasonRequired
	}
	exists, err := s.relRepo.UserExists(ctx, req.UserID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCustomerNotFound
	}
	candidate, err := s.relRepo.FindSalesCandidate(ctx, req.AdminID)
	if err != nil {
		return err
	}
	if candidate == nil {
		return ErrInvalidSales
	}
	// 主管只能把客户分配给自己部门的销售（超管不受限）。
	if !scope.All && scope.DepartmentID > 0 && candidate.DepartmentID != scope.DepartmentID {
		return ErrOutOfScope
	}
	// 销售本人不得改派客户（自助认领一律拒绝，由主管分配）。
	if !scope.All && scope.AdminID > 0 {
		return ErrOutOfScope
	}

	rates, err := s.commRepo.Rates(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := s.relRepo.LockActiveByUser(tx, req.UserID)
		if err != nil {
			return err
		}
		if current != nil {
			if current.AdminID == req.AdminID {
				return ErrSameOwner
			}
			// 保护期内不允许改派；保护期是给销售的跟进保护，超管也走同一规则（需要强改先释放）。
			if current.ProtectUntil != nil && current.ProtectUntil.After(now) {
				return ErrRelationProtected
			}
			if err := s.relRepo.Release(tx, current.ID, operatorID, reason, now); err != nil {
				return err
			}
		}
		var protectUntil *time.Time
		if rates.ProtectDays > 0 {
			t := now.AddDate(0, 0, rates.ProtectDays)
			protectUntil = &t
		}
		rel := &model.StaffSalesRelation{
			AdminID:      req.AdminID,
			UserID:       req.UserID,
			Status:       model.RelationStatusActive,
			Reason:       reason,
			Operator:     operatorID,
			ProtectUntil: protectUntil,
			EffectiveAt:  now,
		}
		if err := s.relRepo.Create(tx, rel); err != nil {
			return err
		}
		return s.relRepo.SetUserOwner(tx, req.UserID, req.AdminID)
	})
}

func (s *customerService) Release(ctx context.Context, scope dto.Scope, operatorID uint64, operatorName string, req dto.ReleaseRequest) error {
	exists, err := s.relRepo.UserExists(ctx, req.UserID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCustomerNotFound
	}
	now := time.Now()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := s.relRepo.LockActiveByUser(tx, req.UserID)
		if err != nil {
			return err
		}
		if current == nil {
			return ErrRelationNotFound
		}
		if !scope.All && scope.DepartmentID > 0 {
			dept, err := s.relRepo.DepartmentOfAdmin(ctx, current.AdminID)
			if err != nil {
				return err
			}
			if dept != scope.DepartmentID {
				return ErrOutOfScope
			}
		}
		if !scope.All && scope.AdminID > 0 {
			return ErrOutOfScope
		}
		if current.ProtectUntil != nil && current.ProtectUntil.After(now) {
			return ErrRelationProtected
		}
		if err := s.relRepo.Release(tx, current.ID, operatorID, strings.TrimSpace(req.Reason), now); err != nil {
			return err
		}
		return s.relRepo.SetUserOwner(tx, req.UserID, 0)
	})
}

// —— 查询 ——

func (s *customerService) List(ctx context.Context, scope dto.Scope, q dto.CustomerQuery) (*dto.CustomerListResponse, error) {
	f := repository.RelationFilter{
		Keyword: q.Keyword, Page: q.Page, PageSize: q.PageSize,
	}
	switch {
	case scope.All:
		f.AdminID = q.AdminID
		f.DepartmentID = q.DepartmentID
	case scope.AdminID > 0:
		// 销售只看自己名下的客户，忽略前端任何 admin_id 传参。
		f.AdminID = scope.AdminID
	case scope.DepartmentID > 0:
		f.AdminID = q.AdminID
		f.DepartmentID = scope.DepartmentID
	default:
		return &dto.CustomerListResponse{Items: []dto.CustomerInfo{}, Meta: dto.ListMeta{}}, nil
	}
	rows, total, err := s.relRepo.ListRows(ctx, f)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	items := make([]dto.CustomerInfo, 0, len(rows))
	for _, row := range rows {
		info := dto.CustomerInfo{
			RelationID:     row.ID,
			UserID:         row.UserID,
			Username:       row.Username,
			Email:          row.Email,
			Phone:          row.Phone,
			AdminID:        row.AdminID,
			AdminName:      row.AdminName,
			AdminRealName:  row.AdminRealName,
			DepartmentName: row.Department,
			Status:         row.Status,
			Reason:         row.Reason,
			TotalConsume:   row.TotalConsume,
			ProtectUntil:   formatTimePtr(row.ProtectAt),
			EffectiveAt:    formatTimePtr(row.EffectiveAt),
			ReleasedAt:     formatTimePtr(row.ReleasedAt),
			RegisteredAt:   formatTimePtr(row.UserCreated),
			Protected:      row.ProtectAt != nil && row.ProtectAt.After(now),
		}
		items = append(items, info)
	}
	p, ps := normalizePage(q.Page, q.PageSize)
	return &dto.CustomerListResponse{Items: items, Meta: dto.ListMeta{Page: p, PageSize: ps, Total: total}}, nil
}

func (s *customerService) Unassigned(ctx context.Context, q dto.CustomerQuery) (*dto.UnassignedListResponse, error) {
	rows, total, err := s.relRepo.ListUnassigned(ctx, q.Keyword, q.Page, q.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]dto.UnassignedInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.UnassignedInfo{
			UserID:       row.UserID,
			Username:     row.Username,
			Email:        row.Email,
			Phone:        row.Phone,
			TotalConsume: row.TotalConsume,
			RegisteredAt: formatTimePtr(row.CreatedAt),
		})
	}
	p, ps := normalizePage(q.Page, q.PageSize)
	return &dto.UnassignedListResponse{Items: items, Meta: dto.ListMeta{Page: p, PageSize: ps, Total: total}}, nil
}

func (s *customerService) Relations(ctx context.Context, userID uint64) (*dto.RelationListResponse, error) {
	rows, err := s.relRepo.ListByUser(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	items := make([]dto.RelationInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.RelationInfo{
			ID:             row.ID,
			AdminID:        row.AdminID,
			AdminName:      row.AdminName,
			AdminRealName:  row.AdminRealName,
			DepartmentName: row.Department,
			Status:         row.Status,
			Reason:         row.Reason,
			OperatorID:     row.OperatorID,
			ProtectUntil:   formatTimePtr(row.ProtectAt),
			EffectiveAt:    formatTimePtr(row.EffectiveAt),
			ReleasedAt:     formatTimePtr(row.ReleasedAt),
			CreatedAt:      formatTimePtr(row.CreatedAt),
		})
	}
	return &dto.RelationListResponse{Items: items}, nil
}

func (s *customerService) Candidates(ctx context.Context, departmentID uint64) (*dto.SalesCandidateListResponse, error) {
	rows, err := s.relRepo.ListSalesCandidates(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	items := make([]dto.SalesCandidateInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.SalesCandidateInfo{
			AdminID:         row.AdminID,
			Username:        row.Username,
			RealName:        row.RealName,
			DepartmentID:    row.DepartmentID,
			DepartmentName:  row.DepartmentName,
			ActiveCustomers: row.ActiveCustomers,
		})
	}
	return &dto.SalesCandidateListResponse{Items: items}, nil
}

// —— 自动归属与离职转派 ——

// EnsureAutoClaim 注册后自动归属：仅当客户当前无归属时生效。
// 选择规则「现役客户数最少的在职销售」，把新客户优先补给跟进压力小的销售。
func (s *customerService) EnsureAutoClaim(ctx context.Context, userID uint64) error {
	if userID == 0 {
		return nil
	}
	rates, err := s.commRepo.Rates(ctx)
	if err != nil {
		return err
	}
	if !rates.Enabled {
		return nil
	}
	existing, err := s.relRepo.FindActiveByUser(ctx, userID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	candidates, err := s.relRepo.ListSalesCandidates(ctx, 0)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	target := candidates[0]
	for _, c := range candidates[1:] {
		if c.ActiveCustomers < target.ActiveCustomers {
			target = c
		}
	}
	now := time.Now()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := s.relRepo.LockActiveByUser(tx, userID)
		if err != nil {
			return err
		}
		if current != nil {
			return nil // 并发下已被其他请求归属
		}
		var protectUntil *time.Time
		if rates.ProtectDays > 0 {
			t := now.AddDate(0, 0, rates.ProtectDays)
			protectUntil = &t
		}
		if err := s.relRepo.Create(tx, &model.StaffSalesRelation{
			AdminID:      target.AdminID,
			UserID:       userID,
			Status:       model.RelationStatusActive,
			Reason:       "注册自动归属",
			ProtectUntil: protectUntil,
			EffectiveAt:  now,
		}); err != nil {
			return err
		}
		return s.relRepo.SetUserOwner(tx, userID, target.AdminID)
	})
}

// TransferOnResign 离职转派：名下现役客户按「同部门其他在职销售」轮流分配，
// 无候选则释放归属（置 users.sales_admin_id=0），历史提成不受影响。
func (s *customerService) TransferOnResign(ctx context.Context, adminID uint64, operatorID uint64) (int, error) {
	userIDs, err := s.relRepo.ListActiveUserIDsByAdmin(ctx, adminID)
	if err != nil {
		return 0, err
	}
	if len(userIDs) == 0 {
		return 0, nil
	}
	deptID, err := s.relRepo.DepartmentOfAdmin(ctx, adminID)
	if err != nil {
		return 0, err
	}
	candidates, err := s.relRepo.ListSalesCandidates(ctx, deptID)
	if err != nil {
		return 0, err
	}
	// 排除离职人自己（resigned_at 置空前的窗口内可能仍被查出来）。
	alive := candidates[:0]
	for _, c := range candidates {
		if c.AdminID != adminID {
			alive = append(alive, c)
		}
	}
	candidates = alive

	now := time.Now()
	moved := 0
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, userID := range userIDs {
			current, err := s.relRepo.LockActiveByUser(tx, userID)
			if err != nil {
				return err
			}
			if current == nil || current.AdminID != adminID {
				continue
			}
			if err := s.relRepo.Release(tx, current.ID, operatorID, "销售离职转派", now); err != nil {
				return err
			}
			if len(candidates) == 0 {
				if err := s.relRepo.SetUserOwner(tx, userID, 0); err != nil {
					return err
				}
				continue
			}
			// 轮询分配：把客户尽量均摊到同部门剩余销售。
			target := candidates[i%len(candidates)]
			if err := s.relRepo.Create(tx, &model.StaffSalesRelation{
				AdminID:     target.AdminID,
				UserID:      userID,
				Status:      model.RelationStatusActive,
				Reason:      "销售离职转派",
				Operator:    operatorID,
				EffectiveAt: now,
			}); err != nil {
				return err
			}
			if err := s.relRepo.SetUserOwner(tx, userID, target.AdminID); err != nil {
				return err
			}
			moved++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		moved = len(userIDs)
	}
	return moved, nil
}

// —— 内部工具 ——

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// isNotFound 便于服务层统一把 gorm.ErrRecordNotFound 映射为业务错误。
func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
