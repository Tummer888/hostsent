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
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uint64) error
	List(ctx context.Context) ([]model.User, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	ResetPassword(ctx context.Context, id uint64, passwordHash string) error
	UpsertRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
	GetRoles(ctx context.Context, userID uint64) ([]model.Role, error)
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
	roles, err := r.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Role = firstRoleCode(roles)
	user.Roles = roleCodes(roles)
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
	roles, err := r.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Role = firstRoleCode(roles)
	user.Roles = roleCodes(roles)
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

func (r *userRepository) List(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if err := r.db.WithContext(ctx).Order("id desc").Find(&users).Error; err != nil {
		return nil, err
	}
	for i := range users {
		roles, err := r.GetRoles(ctx, users[i].ID)
		if err != nil {
			return nil, err
		}
		users[i].Role = firstRoleCode(roles)
		users[i].Roles = roleCodes(roles)
	}
	return users, nil
}

func (r *userRepository) UpdateStatus(ctx context.Context, id uint64, status string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("status", status).Error
}

func (r *userRepository) ResetPassword(ctx context.Context, id uint64, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("password_hash", passwordHash).Error
}

func (r *userRepository) UpsertRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
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

func (r *userRepository) GetRoles(ctx context.Context, userID uint64) ([]model.Role, error) {
	var roles []model.Role
	if err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.id, roles.name, roles.code, roles.status, roles.created_at, roles.updated_at").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Scan(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func firstRoleCode(roles []model.Role) string {
	if len(roles) == 0 {
		return ""
	}
	return roles[0].Code
}

func roleCodes(roles []model.Role) []string {
	codes := make([]string, 0, len(roles))
	for _, role := range roles {
		codes = append(codes, role.Code)
	}
	return codes
}
