package service

import (
	"context"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
	"hostsent/backend/internal/modules/user/repository"
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

func (s *userDetailService) GetAggregate(ctx context.Context, userID uint64) (*dto.UserDetailAggregateResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	permissions, err := s.detailRepo.ListPermissionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	instances, err := s.detailRepo.ListInstancesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	orders, err := s.detailRepo.ListOrdersByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	bills, err := s.detailRepo.ListBillsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	transactions, err := s.detailRepo.ListTransactionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	tickets, err := s.detailRepo.ListTicketsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := &dto.UserDetailAggregateResponse{
		Profile:      toUserInfo(*user),
		Permissions:  make([]dto.UserPermissionItem, 0, len(permissions)),
		Instances:    make([]dto.UserInstanceItem, 0, len(instances)),
		Orders:       make([]dto.UserOrderItem, 0, len(orders)),
		Bills:        make([]dto.UserBillItem, 0, len(bills)),
		Transactions: make([]dto.UserTransactionItem, 0, len(transactions)),
		Tickets:      make([]dto.UserTicketItem, 0, len(tickets)),
	}

	for _, item := range permissions {
		resp.Permissions = append(resp.Permissions, dto.UserPermissionItem{ID: item.ID, Name: item.Name, Code: item.Code, Type: item.Type, Path: item.Path})
	}
	for _, item := range instances {
		resp.Instances = append(resp.Instances, dto.UserInstanceItem{ID: item.ID, Name: item.Name, Region: item.Region, Specs: item.Specs, Status: item.Status, ExpireAt: item.ExpireAt})
	}
	for _, item := range orders {
		resp.Orders = append(resp.Orders, dto.UserOrderItem{ID: item.ID, OrderNo: item.OrderNo, Product: item.Product, Amount: item.Amount, Status: item.Status, CreatedAt: item.CreatedAt})
	}
	for _, item := range bills {
		resp.Bills = append(resp.Bills, dto.UserBillItem{ID: item.ID, BillingMonth: item.BillingMonth, Amount: item.Amount, Status: item.Status})
	}
	for _, item := range transactions {
		resp.Transactions = append(resp.Transactions, dto.UserTransactionItem{ID: item.ID, TxnNo: item.TxnNo, Type: item.Type, Amount: item.Amount, CreatedAt: item.CreatedAt})
	}
	for _, item := range tickets {
		resp.Tickets = append(resp.Tickets, dto.UserTicketItem{ID: item.ID, TicketNo: item.TicketNo, Title: item.Title, Category: item.Category, Priority: item.Priority, Status: item.Status, UpdatedAt: item.UpdatedAt})
	}

	return resp, nil
}

var _ = model.User{}
