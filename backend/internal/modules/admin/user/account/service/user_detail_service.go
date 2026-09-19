package service

import (
	"context"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/repository"
)

type UserDetailService interface {
	GetAggregate(ctx context.Context, userID uint64) (*dto.UserDetailAggregateResponse, error)
}

type userDetailService struct {
	userRepo   repository.UserRepository
	detailRepo repository.UserDetailRepository
}

func NewUserDetailService(userRepo repository.UserRepository, detailRepo repository.UserDetailRepository) UserDetailService {
	return &userDetailService{userRepo: userRepo, detailRepo: detailRepo}
}

// GetAggregate 汇集详情页所需的「资料 + 各域计数」。
//
// 容错策略（本轮修正）：原实现 6 次顺序查询任一失败即中止，整个详情页 500。
// 现实是这些域分属不同模块，任何一张表的数据异常都不应该让管理员看不到用户资料。
// 因此只有 profile（身份本身）失败才返回错误，其余各段失败只记入 Degraded，
// 前端据此在对应 Tab 显示「数据暂不可用」。
//
// 「近期摘要」五个数组已于本轮移除（doc104 §3.3，F20）：它们从未被前端读取，
// 却让每次详情页加载都多打 5 条带 LIMIT 的查询。各 Tab 的完整列表走各自的分页接口。
func (s *userDetailService) GetAggregate(ctx context.Context, userID uint64) (*dto.UserDetailAggregateResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := &dto.UserDetailAggregateResponse{
		Profile:     toUserInfo(*user),
		RbacRoles:   []dto.UserRoleBrief{},
		Permissions: []string{},
		Degraded:    []string{},
	}

	if roles, err := s.detailRepo.ListRbacRolesByUserID(ctx, userID); err != nil {
		resp.Degraded = append(resp.Degraded, "rbac_roles")
	} else {
		for _, role := range roles {
			resp.RbacRoles = append(resp.RbacRoles, dto.UserRoleBrief{
				ID: role.ID, Code: role.Code, Name: role.Name, Scope: role.Scope,
			})
		}
	}

	if codes, err := s.detailRepo.ListCustomerPermissions(ctx, userID); err != nil {
		resp.Degraded = append(resp.Degraded, "permissions")
	} else {
		resp.Permissions = codes
	}

	if counts, err := s.detailRepo.Counts(ctx, userID); err != nil {
		resp.Degraded = append(resp.Degraded, "summary")
	} else {
		resp.Summary = dto.UserDetailSummary{
			InstanceCount:      counts.InstanceCount,
			RunningInstance:    counts.RunningInstance,
			OrderCount:         counts.OrderCount,
			OrderTotalAmount:   counts.OrderTotalAmount,
			BillCount:          counts.BillCount,
			UnpaidBillCount:    counts.UnpaidBillCount,
			TransactionCount:   counts.TransactionCount,
			TicketCount:        counts.TicketCount,
			OpenTicketCount:    counts.OpenTicketCount,
			LoginCount:         counts.LoginCount,
			ActiveSessionCount: counts.ActiveSessionCount,
			RiskEventCount:     counts.RiskEventCount,
			OperationLogCount:  counts.OperationLogCount,
			VerificationCount:  counts.VerificationCount,
		}
	}

	return resp, nil
}
