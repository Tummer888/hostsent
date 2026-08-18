package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/model"
)

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
	List(ctx context.Context) ([]model.User, error)
	UpsertRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	user.Role = "admin"
	user.Roles = []string{"admin"}
	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	user.Role = "admin"
	user.Roles = []string{"admin"}
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) List(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if err := r.db.WithContext(ctx).Order("id desc").Find(&users).Error; err != nil {
		return nil, err
	}
	for i := range users {
		users[i].Role = "admin"
		users[i].Roles = []string{"admin"}
	}
	return users, nil
}

func (r *userRepository) UpsertRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	if err := r.db.WithContext(ctx).Table("user_roles").Where("user_id = ?", userID).Delete(nil).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]model.UserRole, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		rows = append(rows, model.UserRole{UserID: userID, RoleID: roleID})
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}
