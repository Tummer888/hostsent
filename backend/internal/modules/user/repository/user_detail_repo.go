package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/model"
)

type UserDetailRepository interface {
	ListPermissionsByUserID(ctx context.Context, userID uint64) ([]model.Permission, error)
	ListInstancesByUserID(ctx context.Context, userID uint64) ([]model.UserInstance, error)
	ListOrdersByUserID(ctx context.Context, userID uint64) ([]model.UserOrder, error)
	ListBillsByUserID(ctx context.Context, userID uint64) ([]model.UserBill, error)
	ListTransactionsByUserID(ctx context.Context, userID uint64) ([]model.UserTransaction, error)
	ListTicketsByUserID(ctx context.Context, userID uint64) ([]model.UserTicket, error)
}

type userDetailRepository struct {
	db *gorm.DB
}

func NewUserDetailRepository(db *gorm.DB) UserDetailRepository {
	return &userDetailRepository{db: db}
}

func (r *userDetailRepository) ListPermissionsByUserID(ctx context.Context, userID uint64) ([]model.Permission, error) {
	var items []model.Permission
	err := r.db.WithContext(ctx).
		Table("permissions").
		Select("DISTINCT permissions.id, permissions.parent_id, permissions.name, permissions.code, permissions.type, permissions.path, permissions.component, permissions.icon, permissions.sort_order, permissions.status, permissions.created_at, permissions.updated_at").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ?", userID).
		Order("permissions.parent_id asc, permissions.sort_order asc, permissions.id asc").
		Scan(&items).Error
	return items, err
}

func (r *userDetailRepository) ListInstancesByUserID(ctx context.Context, userID uint64) ([]model.UserInstance, error) {
	var items []model.UserInstance
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("expire_at asc, id desc").Find(&items).Error
	return items, err
}

func (r *userDetailRepository) ListOrdersByUserID(ctx context.Context, userID uint64) ([]model.UserOrder, error) {
	var items []model.UserOrder
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Find(&items).Error
	return items, err
}

func (r *userDetailRepository) ListBillsByUserID(ctx context.Context, userID uint64) ([]model.UserBill, error) {
	var items []model.UserBill
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("billing_month desc, id desc").Find(&items).Error
	return items, err
}

func (r *userDetailRepository) ListTransactionsByUserID(ctx context.Context, userID uint64) ([]model.UserTransaction, error) {
	var items []model.UserTransaction
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc, id desc").Find(&items).Error
	return items, err
}

func (r *userDetailRepository) ListTicketsByUserID(ctx context.Context, userID uint64) ([]model.UserTicket, error) {
	var items []model.UserTicket
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at desc, id desc").Find(&items).Error
	return items, err
}
