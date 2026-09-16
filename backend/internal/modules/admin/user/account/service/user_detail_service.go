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

// GetAggregate 汇集详情页所需的「资料 + 各域计数 + 近期摘要」。
//
// 容错策略（本轮修正）：原实现 6 次顺序查询任一失败即中止，整个详情页 500。
// 现实是这些域分属不同模块，任何一张表的数据异常都不应该让管理员看不到用户资料。
// 因此只有 profile（身份本身）失败才返回错误，其余各段失败只记入 Degraded，
// 前端据此在对应 Tab 显示「数据暂不可用」。
func (s *userDetailService) GetAggregate(ctx context.Context, userID uint64) (*dto.UserDetailAggregateResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := &dto.UserDetailAggregateResponse{
		Profile:            toUserInfo(*user),
		RbacRoles:          []dto.UserRoleBrief{},
		Permissions:        []string{},
		RecentInstances:    []dto.UserInstanceBrief{},
		RecentOrders:       []dto.UserOrderBrief{},
		RecentBills:        []dto.UserBillBrief{},
		RecentTransactions: []dto.UserTransactionBrief{},
		RecentTickets:      []dto.UserTicketBrief{},
		Degraded:           []string{},
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

	if items, err := s.detailRepo.ListInstancesByUserID(ctx, userID); err != nil {
		resp.Degraded = append(resp.Degraded, "instances")
	} else {
		for _, item := range items {
			resp.RecentInstances = append(resp.RecentInstances, dto.UserInstanceBrief{
				ID: item.ID, InstanceID: item.InstanceID, Name: item.Name,
				Region: item.Region, Zone: item.Zone,
				CPU: item.CPU, Memory: item.Memory, Disk: item.Disk,
				OS: item.OS, PublicIP: item.PublicIP, Status: item.Status,
				BillingMode: item.BillingMode, LifecycleStage: item.LifecycleStage,
				OrderID: item.OrderID, SourceMode: item.SourceMode,
				ExpireAt: item.ExpireAt, CreatedAt: item.CreatedAt,
			})
		}
	}

	if items, err := s.detailRepo.ListOrdersByUserID(ctx, userID); err != nil {
		resp.Degraded = append(resp.Degraded, "orders")
	} else {
		for _, item := range items {
			resp.RecentOrders = append(resp.RecentOrders, dto.UserOrderBrief{
				ID: item.ID, OrderNo: item.OrderNo, ProductName: item.ProductName,
				FinalAmount: item.FinalAmount, Status: item.Status,
				PayMethod: item.PayMethod, RenewalID: item.RenewalID,
				CreatedAt: item.CreatedAt, PaidAt: item.PaidAt,
			})
		}
	}

	if items, err := s.detailRepo.ListBillsByUserID(ctx, userID); err != nil {
		resp.Degraded = append(resp.Degraded, "bills")
	} else {
		for _, item := range items {
			resp.RecentBills = append(resp.RecentBills, dto.UserBillBrief{
				ID: item.ID, BillNo: item.BillNo, BillingMonth: item.BillingMonth,
				Amount: item.Amount, BillType: item.BillType,
				Status: item.Status, CreatedAt: item.CreatedAt,
			})
		}
	}

	if items, err := s.detailRepo.ListTransactionsByUserID(ctx, userID); err != nil {
		resp.Degraded = append(resp.Degraded, "transactions")
	} else {
		for _, item := range items {
			resp.RecentTransactions = append(resp.RecentTransactions, dto.UserTransactionBrief{
				ID: item.ID, TxnNo: item.TxnNo, Type: item.Type,
				Direction: item.Direction, Amount: item.Amount,
				BalanceAfter: item.BalanceAfter, Remark: item.Remark,
				CreatedAt: item.CreatedAt,
			})
		}
	}

	if items, err := s.detailRepo.ListTicketsByUserID(ctx, userID); err != nil {
		resp.Degraded = append(resp.Degraded, "tickets")
	} else {
		for _, item := range items {
			resp.RecentTickets = append(resp.RecentTickets, dto.UserTicketBrief{
				ID: item.ID, TicketNo: item.TicketNo, Title: item.Title,
				Category: item.Category, Priority: item.Priority, Status: item.Status,
				UpdatedAt: item.UpdatedAt, CreatedAt: item.CreatedAt,
			})
		}
	}

	return resp, nil
}