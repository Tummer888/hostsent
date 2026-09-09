package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/auth/model"
)

// UserRepository 用户中心数据访问接口。
// 定义用户中心自助场景所需的数据库操作，与后台管理模块的 repository 独立。
type UserRepository interface {
	// FindByUsername 根据用户名查找用户，未找到返回 gorm.ErrRecordNotFound。
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	// FindByEmail 根据邮箱查找用户，未找到返回 gorm.ErrRecordNotFound。
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	// FindByID 根据用户 ID 查找用户。
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	// Create 创建新用户记录。
	Create(ctx context.Context, user *model.User) error
	// UpdateLoginProfile 更新用户登录档案（最近登录 IP、归属地、时间）。
	UpdateLoginProfile(ctx context.Context, id uint64, ip, ipRegion string, loginAt time.Time) error
	// UpdateProfile 更新用户基本资料（显示名、邮箱、手机、头像）。
	UpdateProfile(ctx context.Context, id uint64, name, email, phone, avatar string) error
	// UpdatePassword 更新用户密码哈希。
	UpdatePassword(ctx context.Context, id uint64, passwordHash string) error
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户中心数据访问实例。
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) UpdateLoginProfile(ctx context.Context, id uint64, ip, ipRegion string, loginAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"last_login_at":        loginAt,
		"last_login_ip":        ip,
		"last_login_ip_region": ipRegion,
	}).Error
}

func (r *userRepository) UpdateProfile(ctx context.Context, id uint64, name, email, phone, avatar string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"real_name": name,
		"email":     email,
		"phone":     phone,
		"avatar":    avatar,
	}).Error
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uint64, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("password_hash", passwordHash).Error
}
