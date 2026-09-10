package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
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
	// NamesByIDs 批量查询用户 ID → 用户名，用于订单/实例的「操作人」列（P4-09）。
	NamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// ListSubAccounts 查询某主账号名下的全部子账号（P4-10）。
	ListSubAccounts(ctx context.Context, ownerID uint64) ([]model.User, error)
	// PermissionsByUserIDs 批量查询子账号已授予的权限码（P4-10）。
	PermissionsByUserIDs(ctx context.Context, ids []uint64) (map[uint64][]string, error)
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
	if err := r.db.WithContext(ctx).
		Select("users.*, user_groups.name AS user_group_name, user_levels.name AS user_level_name, user_levels.code AS user_level_code, owner.username AS owner_name").
		Joins("LEFT JOIN user_groups ON user_groups.id = users.user_group_id").
		Joins("LEFT JOIN user_levels ON user_levels.id = users.user_level_id").
		Joins("LEFT JOIN users AS owner ON owner.id = users.owner_user_id").
		First(&user, "users.id = ?", id).Error; err != nil {
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

// NamesByIDs 批量查询用户 ID → 用户名（用户名优先，回退为空），用于「操作人」列。
func (r *userRepository) NamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	result := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ID       uint64
		Username string
	}
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Select("id", "username").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Username
	}
	return result, nil
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
	// total_consume_amount 自 P3-01 起为 users 表落列字段（消费升级服务维护），无需再实时聚合。
	if err := base.
		Select("users.*, user_groups.name AS user_group_name, user_levels.name AS user_level_name, user_levels.code AS user_level_code, owner.username AS owner_name").
		Joins("LEFT JOIN user_groups ON user_groups.id = users.user_group_id").
		Joins("LEFT JOIN user_levels ON user_levels.id = users.user_level_id").
		Joins("LEFT JOIN users AS owner ON owner.id = users.owner_user_id").
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
		db = db.Where("users.status = ?", status)
	}
	if ipRegion := strings.TrimSpace(query.LastLoginIPRegion); ipRegion != "" {
		db = db.Where("users.last_login_ip_region = ?", ipRegion)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("users.username ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ? OR users.real_name ILIKE ?", like, like, like, like)
	}
	if query.UserLevelID > 0 {
		db = db.Where("users.user_level_id = ?", query.UserLevelID)
	}
	if query.UserGroupID > 0 {
		db = db.Where("users.user_group_id = ?", query.UserGroupID)
	}
	// 主账号 / 子账号筛选（P4-10）。
	switch strings.TrimSpace(query.IsSubAccount) {
	case "true", "1":
		db = db.Where("users.is_sub_account = ?", true)
	case "false", "0":
		db = db.Where("users.is_sub_account = ?", false)
	}

	switch strings.TrimSpace(query.Filter) {
	case "today":
		today := time.Now().Format("2006-01-02")
		db = db.Where("DATE(users.created_at) = ?", today)
	case "pending_real_name":
		db = db.Where("(users.real_name IS NULL OR users.real_name = '')")
	}

	return db
}

// ListSubAccounts 查询某主账号名下的全部子账号（P4-10）。
func (r *userRepository) ListSubAccounts(ctx context.Context, ownerID uint64) ([]model.User, error) {
	var users []model.User
	if err := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND is_sub_account = ?", ownerID, true).
		Order("id ASC").
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// PermissionsByUserIDs 批量查询子账号权限码，返回 userID → 权限码列表（P4-10）。
func (r *userRepository) PermissionsByUserIDs(ctx context.Context, ids []uint64) (map[uint64][]string, error) {
	result := make(map[uint64][]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		UserID         uint64 `gorm:"column:user_id"`
		PermissionCode string `gorm:"column:permission_code"`
	}
	if err := r.db.WithContext(ctx).
		Table("sub_account_permissions").
		Select("user_id", "permission_code").
		Where("user_id IN ?", ids).
		Order("user_id ASC, permission_code ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.PermissionCode)
	}
	return result, nil
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
