package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, query dto.UserListQuery) ([]model.User, int64, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	UpdatePassword(ctx context.Context, id uint64, passwordHash string) error
	GetRoles(ctx context.Context, userID uint64) ([]model.Role, error)
	SetRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
	Stats(ctx context.Context) (*model.UserStats, error)
	RegionStats(ctx context.Context) ([]model.RegionStat, error)
	UpdateLoginProfile(ctx context.Context, id uint64, ip string, ipRegion string, loginAt time.Time) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	db := r.db.WithContext(ctx)
	if user.ID > 0 {
		return db.Select("ID", "Username", "Email", "Phone", "PasswordHash", "Status", "UserGroupID").Create(user).Error
	}
	return db.Omit("ID").Create(user).Error
}

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

func (r *userRepository) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
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

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
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

func (r *userRepository) List(ctx context.Context, query dto.UserListQuery) ([]model.User, int64, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	base := r.db.WithContext(ctx).Model(&model.User{})
	base = applyUserFilters(base, query)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	if err := base.
		Select("users.*, user_groups.name AS user_group_name, COALESCE(consume_stats.total_consume_amount, 0) AS total_consume_amount").
		Joins("LEFT JOIN user_groups ON user_groups.id = users.user_group_id").
		Joins(`LEFT JOIN (
			SELECT user_id, COALESCE(SUM(ABS(amount)), 0) AS total_consume_amount
			FROM user_transactions
			WHERE type = ?
			GROUP BY user_id
		) AS consume_stats ON consume_stats.user_id = users.id`, "consume").
		Order("users.id DESC").
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
	if ipRegion := strings.TrimSpace(query.LastLoginIPRegion); ipRegion != "" {
		db = db.Where("last_login_ip_region = ?", ipRegion)
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

func (r *userRepository) UpdatePassword(ctx context.Context, id uint64, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("password_hash", passwordHash).Error
}

func (r *userRepository) GetRoles(ctx context.Context, userID uint64) ([]model.Role, error) {
	var roles []model.Role
	if err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.*").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.id ASC").
		Scan(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *userRepository) SetRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM user_roles WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			if err := tx.Exec("INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", userID, roleID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *userRepository) Stats(ctx context.Context) (*model.UserStats, error) {
	var stats model.UserStats

	if err := r.db.WithContext(ctx).Model(&model.User{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("DATE(created_at) = ?", time.Now().Format("2006-01-02")).Count(&stats.TodayNew).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("status = ?", "active").Count(&stats.Active).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("status = ?", "disabled").Count(&stats.Disabled).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("status = ? AND (real_name IS NULL OR real_name = '')", "active").Count(&stats.PendingRealName).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("status = ?", "pending").Count(&stats.PendingReview).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Select("COALESCE(SUM(balance), 0)").Scan(&stats.TotalBalance).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Table("orders").Select("COUNT(DISTINCT user_id)").Scan(&stats.PurchasedCount).Error; err != nil {
		stats.PurchasedCount = 0
	}

	return &stats, nil
}

// RegionStats 按登录 IP 归属地聚合用户数，仅统计 last_login_ip_region 非空的用户，按数量倒序返回。
func (r *userRepository) RegionStats(ctx context.Context) ([]model.RegionStat, error) {
	var rows []model.RegionStat
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Select("last_login_ip_region AS region, COUNT(*) AS count").
		Where("last_login_ip_region IS NOT NULL AND last_login_ip_region <> ''").
		Group("last_login_ip_region").
		Order("count DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *userRepository) UpdateLoginProfile(ctx context.Context, id uint64, ip string, ipRegion string, loginAt time.Time) error {
	updates := map[string]any{
		"last_login_at":        loginAt,
		"last_login_ip":        ip,
		"last_login_ip_region": ipRegion,
	}
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
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
		if role.Code != "" {
			codes = append(codes, role.Code)
		}
	}
	return codes
}
