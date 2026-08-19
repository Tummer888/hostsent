package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
)

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uint64) error
	List(ctx context.Context, query dto.UserListQuery) ([]model.User, int64, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	ResetPassword(ctx context.Context, id uint64, passwordHash string) error
	UpsertRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
	GetRoles(ctx context.Context, userID uint64) ([]model.Role, error)
	Stats(ctx context.Context) (*model.UserStats, error)
	RegionStats(ctx context.Context) ([]model.RegionStat, error)
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

func (r *userRepository) List(ctx context.Context, query dto.UserListQuery) ([]model.User, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	base := r.db.WithContext(ctx).Model(&model.User{})
	base = applyUserFilters(base, query)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	if err := base.
		Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	for i := range users {
		roles, err := r.GetRoles(ctx, users[i].ID)
		if err != nil {
			return nil, 0, err
		}
		users[i].Role = firstRoleCode(roles)
		users[i].Roles = roleCodes(roles)
	}

	return users, total, nil
}

func applyUserFilters(db *gorm.DB, query dto.UserListQuery) *gorm.DB {
	if status := strings.TrimSpace(query.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	if region := strings.TrimSpace(query.Region); region != "" {
		db = db.Where("region = ?", region)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("username ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR real_name ILIKE ?", like, like, like, like)
	}

	switch strings.TrimSpace(query.Filter) {
	case "today":
		today := time.Now().Format("2006-01-02")
		db = db.Where("DATE(created_at) = ?", today)
	case "pending_real_name":
		db = db.Where("(real_name IS NULL OR real_name = '')")
	}

	return db
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

// Stats 汇总用户状态统计。状态取值与 seed 一致：active=正常，disabled=冻结，
// pending=待审核；待实名以 real_name 为空且账号可用为条件，避免把已冻结/待审核账号重复计入。
func (r *userRepository) Stats(ctx context.Context) (*model.UserStats, error) {
	var stats model.UserStats

	if err := r.db.WithContext(ctx).Model(&model.User{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("DATE(created_at) = ?", today).
		Count(&stats.TodayNew).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("status = ?", "active").
		Count(&stats.Active).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("status = ?", "disabled").
		Count(&stats.Disabled).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("(real_name IS NULL OR real_name = '') AND status = ?", "active").
		Count(&stats.PendingRealName).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("status = ?", "pending").
		Count(&stats.PendingReview).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}

// RegionStats 按地域聚合用户数，仅统计 region 非空的用户，按数量倒序返回。
func (r *userRepository) RegionStats(ctx context.Context) ([]model.RegionStat, error) {
	var rows []model.RegionStat
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Select("region AS region, COUNT(*) AS count").
		Where("region IS NOT NULL AND region <> ''").
		Group("region").
		Order("count DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
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
