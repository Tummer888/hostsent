package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
)

// purchasedOrderStatuses 判定「已购」的订单状态集合，与订单域的销售额口径
// （order/repository 的 paidStatuses）同义：凡成交过（含退款中/已退款）都算买过。
// 定义在这里而不是引用订单仓储，是因为仓储层的 var 不可导出，
// 而用户统计与列表筛选必须共用同一份取值，不能各写各的。
var purchasedOrderStatuses = []string{
	ordermodel.OrderStatusPaid,
	ordermodel.OrderStatusProvisioning,
	ordermodel.OrderStatusActive,
	ordermodel.OrderStatusRefunding,
	ordermodel.OrderStatusRefunded,
	ordermodel.OrderStatusCompleted,
}

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	// Delete 已废弃（硬删除无路由）：保留仅为兼容，勿在新代码调用。
	// 注销走 SoftDelete，到期清理走 PurgeUser。
	Delete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, query dto.UserListQuery) ([]model.User, int64, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	UpdatePassword(ctx context.Context, id uint64, passwordHash string) error
	GetRoles(ctx context.Context, userID uint64) ([]model.Role, error)
	// RolesByUserIDs 批量查询 userID → 角色列表，替换列表页逐行 GetRoles 的 N+1。
	RolesByUserIDs(ctx context.Context, ids []uint64) (map[uint64][]model.Role, error)
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
	// OAuthProvidersByUserIDs 批量查询用户已绑定的第三方渠道（doc104 §6.1）。
	OAuthProvidersByUserIDs(ctx context.Context, ids []uint64) (map[uint64][]string, error)
	// —— 软删除与留存期清理（doc104 §4）——
	// DeletionCheck 注销前置校验的计数汇总（单条 SQL）。
	DeletionCheck(ctx context.Context, id uint64) (*DeletionCheckRow, error)
	// SoftDelete 注销（软删除）：写删除标记并把状态收敛为 cancelled。
	SoftDelete(ctx context.Context, id uint64, operatorID uint64, reason, statusBefore string, now time.Time) error
	// Restore 恢复已注销用户；restoreStatus 为空时回落 disabled。
	Restore(ctx context.Context, id uint64, restoreStatus string) error
	// ListPurgeCandidates 留存期已过的待清理用户。
	ListPurgeCandidates(ctx context.Context, before time.Time, limit int) ([]model.User, error)
	// PurgeUser 硬删除一个用户及其全部个人数据（单事务）。
	PurgeUser(ctx context.Context, id uint64) error
	// CountDeleted 已注销用户数。
	CountDeleted(ctx context.Context) (int64, error)
	// AdminNamesByIDs 批量查询管理员 ID → 显示名（回收站「注销人」列）。
	AdminNamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error)
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
		Select("users.*, user_groups.name AS user_group_name, user_levels.name AS user_level_name, user_levels.code AS user_level_code, owner.username AS owner_name, inviter.username AS inviter_name, COALESCE(NULLIF(sales.real_name, ''), sales.username, '') AS sales_admin_name, COALESCE(NULLIF(deleter.real_name, ''), deleter.username, '') AS deleted_by_name").
		Joins("LEFT JOIN user_groups ON user_groups.id = users.user_group_id").
		Joins("LEFT JOIN user_levels ON user_levels.id = users.user_level_id").
		Joins("LEFT JOIN users AS owner ON owner.id = users.owner_user_id").
		Joins("LEFT JOIN users AS inviter ON inviter.id = users.inviter_user_id").
		Joins("LEFT JOIN admins AS sales ON sales.id = users.sales_admin_id").
		// 注销人姓名（详情页「注销信息」段）。adminNamesByIDs 走的是批量接口，
		// 详情只查一行，单独 JOIN 比再发一次批量查询更省往返。
		Joins("LEFT JOIN admins AS deleter ON deleter.id = users.deleted_by").
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
		Select("users.*, user_groups.name AS user_group_name, user_levels.name AS user_level_name, user_levels.code AS user_level_code, owner.username AS owner_name, inviter.username AS inviter_name, COALESCE(NULLIF(sales.real_name, ''), sales.username, '') AS sales_admin_name").
		Joins("LEFT JOIN user_groups ON user_groups.id = users.user_group_id").
		Joins("LEFT JOIN user_levels ON user_levels.id = users.user_level_id").
		Joins("LEFT JOIN users AS owner ON owner.id = users.owner_user_id").
		Joins("LEFT JOIN users AS inviter ON inviter.id = users.inviter_user_id").
		Joins("LEFT JOIN admins AS sales ON sales.id = users.sales_admin_id").
		Order("users.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	// 角色一次性批量取回：原实现每行调用一次 GetRoles，翻页 10 行即 10 次额外查询。
	roleMap, err := r.RolesByUserIDs(ctx, userIDs(users))
	if err != nil {
		return nil, 0, err
	}
	// 第三方绑定同理批量取回（doc104 §6.1）：前端列表的 oauth_providers 列
	// 此前读的是一个后端从不生产的字段，图标恒为未绑定态。
	oauthMap, err := r.OAuthProvidersByUserIDs(ctx, userIDs(users))
	if err != nil {
		return nil, 0, err
	}
	// 注销人姓名同样批量取回（回收站列表的「注销人」列）。
	adminNames, err := r.AdminNamesByIDs(ctx, deletedByIDs(users))
	if err != nil {
		return nil, 0, err
	}
	for i := range users {
		roles := roleMap[users[i].ID]
		users[i].Role = firstRoleCode(roles)
		users[i].Roles = roleCodes(roles)
		providers := oauthMap[users[i].ID]
		if providers == nil {
			providers = []string{}
		}
		users[i].OAuthProviders = providers
		users[i].DeletedByName = adminNames[users[i].DeletedBy]
	}

	return users, total, nil
}

// deletedByIDs 提取非零的 deleted_by（0 = 系统或用户自助，无对应管理员）。
func deletedByIDs(users []model.User) []uint64 {
	seen := make(map[uint64]struct{}, len(users))
	ids := make([]uint64, 0, len(users))
	for i := range users {
		id := users[i].DeletedBy
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func userIDs(users []model.User) []uint64 {
	ids := make([]uint64, 0, len(users))
	for i := range users {
		ids = append(ids, users[i].ID)
	}
	return ids
}

func applyUserFilters(db *gorm.DB, query dto.UserListQuery) *gorm.DB {
	// 注销态筛选（doc104 §4.3）：
	//   默认排除已注销；filter=deleted 只看回收站；include_deleted=true 全看。
	// 三个开关互斥，include_deleted 优先级最高（回收站翻页时不会因残留参数被放大）。
	switch {
	case query.IncludeDeleted:
		// 不追加条件
	case strings.EqualFold(strings.TrimSpace(query.Filter), "deleted"):
		db = db.Where("users.deleted_at IS NOT NULL")
	default:
		db = db.Where("users.deleted_at IS NULL")
	}

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

	// 归属销售筛选（doc86 §4.1.10）：未归属优先，其次按销售 ID。
	switch strings.TrimSpace(query.UnassignedSales) {
	case "true", "1":
		db = db.Where("users.sales_admin_id = 0")
	default:
		if query.SalesAdminID > 0 {
			db = db.Where("users.sales_admin_id = ?", query.SalesAdminID)
		}
	}

	switch strings.TrimSpace(query.Filter) {
	case "today":
		today := time.Now().Format("2006-01-02")
		db = db.Where("DATE(users.created_at) = ?", today)
	case "pending_real_name":
		// 口径统一（doc104 §4.8，F17）：原实现是 `real_name 为空`，而统计卡的
		// PendingRealName 额外带 status='active'，导致卡片 21 与列表 24 对不上。
		// 现在两边都用同一个判据：**未实名认证**（real_name_verified_at 为空），
		// 与 real_name 展示名解耦——改昵称不再等于已实名（§5.3）。
		db = db.Where("users.real_name_verified_at IS NULL")
	case "purchased":
		// 已购用户（doc104 §3.3，F23）：此前 overview 快捷入口把 filter=purchased
		// 传给列表，而后端 switch 没有这个分支 —— 参数被静默忽略，点进去看到的是
		// 全部用户。这里补上分支，口径与统计卡的 PurchasedCount 共用
		// purchasedOrderStatuses。
		db = db.Where("EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id AND orders.deleted_at IS NULL AND orders.status IN ?)", purchasedOrderStatuses)
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

// RolesByUserIDs 批量查询多个用户的角色（列表页专用，消除 N+1）。
// 返回的 map 只包含有角色的用户；无角色用户取到的是 nil 切片，调用方按空处理。
func (r *userRepository) RolesByUserIDs(ctx context.Context, ids []uint64) (map[uint64][]model.Role, error) {
	result := make(map[uint64][]model.Role, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		UserID uint64 `gorm:"column:user_id"`
		model.Role
	}
	if err := r.db.WithContext(ctx).
		Table("user_roles").
		Select("user_roles.user_id AS user_id, roles.*").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id IN ?", ids).
		Order("user_roles.user_id ASC, roles.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.Role)
	}
	return result, nil
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

	// 统计口径（doc104 §4.8）：除 Deleted 外一律只统计**未注销**用户，
	// 否则注销后总数/活跃数/余额合计都会虚高。
	active := r.db.WithContext(ctx).Model(&model.User{}).Where("deleted_at IS NULL")

	if err := active.Session(&gorm.Session{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}
	if err := active.Session(&gorm.Session{}).Where("DATE(created_at) = ?", time.Now().Format("2006-01-02")).Count(&stats.TodayNew).Error; err != nil {
		return nil, err
	}
	if err := active.Session(&gorm.Session{}).Where("status = ?", model.StatusActive).Count(&stats.Active).Error; err != nil {
		return nil, err
	}
	if err := active.Session(&gorm.Session{}).Where("status = ?", model.StatusDisabled).Count(&stats.Disabled).Error; err != nil {
		return nil, err
	}
	// 待实名：与列表 filter=pending_real_name 同一判据（未认证，而非 real_name 为空）。
	if err := active.Session(&gorm.Session{}).Where("real_name_verified_at IS NULL").Count(&stats.PendingRealName).Error; err != nil {
		return nil, err
	}
	if err := active.Session(&gorm.Session{}).Where("status = ?", model.StatusPending).Count(&stats.PendingReview).Error; err != nil {
		return nil, err
	}
	if err := active.Session(&gorm.Session{}).Select("COALESCE(SUM(balance), 0)").Scan(&stats.TotalBalance).Error; err != nil {
		return nil, err
	}
	// 已购用户数：原实现吞掉错误并置 0，导致统计卡静默显示 0 而无人察觉。
	// 现在把错误抛出去，让统计接口整体失败而不是给出一个错的数字。
	// 口径与列表 filter=purchased 共用 purchasedOrderStatuses（doc104 §3.3，F23）：
	// 两处都限定「订单已成交 + 订单未删除 + 用户未注销」，
	// 否则卡片数字会大于点进列表后看到的条数。
	if err := r.db.WithContext(ctx).Table("orders").
		Joins("JOIN users ON users.id = orders.user_id").
		Where("orders.deleted_at IS NULL").
		Where("users.deleted_at IS NULL").
		Where("orders.status IN ?", purchasedOrderStatuses).
		Select("COUNT(DISTINCT orders.user_id)").Scan(&stats.PurchasedCount).Error; err != nil {
		return nil, err
	}
	// 已注销用户数（回收站入口展示）。
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("deleted_at IS NOT NULL").Count(&stats.Deleted).Error; err != nil {
		return nil, err
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
		Where("deleted_at IS NULL").
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
