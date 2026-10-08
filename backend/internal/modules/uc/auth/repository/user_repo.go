package repository

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

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
	// DefaultLevelID 起始等级（启用中权重最低的一级）；无可用等级时返回 0。
	DefaultLevelID(ctx context.Context) (uint64, error)
	// LevelOf 读取用户当前等级的展示信息（名称/图标/配色/门槛）；无等级返回 nil。
	LevelOf(ctx context.Context, userID uint64) (*LevelBrief, error)
	// DefaultUserRoleID 客户默认角色（roles.code='user'）ID；缺失返回 0。
	DefaultUserRoleID(ctx context.Context) (uint64, error)
	// BindRole 绑定客户角色关系（幂等）。
	BindRole(ctx context.Context, userID, roleID uint64) error
	// UpdateLoginProfile 更新用户登录档案（最近登录 IP、时间）。
	UpdateLoginProfile(ctx context.Context, id uint64, ip string, loginAt time.Time) error
	// UpdateProfile 更新用户基本资料（显示名、邮箱、手机、头像）。
	UpdateProfile(ctx context.Context, id uint64, name, email, phone, avatar string) error
	// UpdatePassword 更新用户密码哈希。
	UpdatePassword(ctx context.Context, id uint64, passwordHash string) error
	// PermissionsOf 返回子账号已授予的客户侧权限码（主账号返回空集，P4-09）。
	PermissionsOf(ctx context.Context, id uint64) ([]string, error)
	// FindByPhone 按手机号查找用户，未找到返回 gorm.ErrRecordNotFound（doc91 短信登录）。
	FindByPhone(ctx context.Context, phone string) (*model.User, error)
	// MarkVerified 写回邮箱/手机验证时间（channel=email/sms，doc91 C3）。
	MarkVerified(ctx context.Context, id uint64, channel string, verifiedAt time.Time) error
	// RevokeSessions 撤销该用户全部有效会话（改密/重置密码后调用，doc91 §5.5）。
	// 返回被撤销的 session_id 列表，供调用方失效会话缓存（否则撤销要等缓存 TTL 才生效）。
	RevokeSessions(ctx context.Context, id uint64, reason string) ([]string, error)
	// RevokeSession 撤销单个会话（登出：只结束当前这一个登录态，不踢掉其他设备）。
	// 返回是否真的撤销了一行（会话不存在/已失效时为 false，不算错误）。
	RevokeSession(ctx context.Context, sessionID, reason string) (bool, error)
	// OpenSession 登录成功后开一条会话记录（doc104 F15）。
	OpenSession(ctx context.Context, in SessionInput) error
}

// LevelBrief 用户当前等级的展示信息（用户端会员徽章用）。
//
// 用户端只需要「叫什么、什么图标、什么颜色」，不需要等级的运营配置细节
// （门槛、权益、子账号上限另走 member 接口），因此单独一个精简结构。
type LevelBrief struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Icon   string `json:"icon"`
	Color  string `json:"color"`
	Weight int    `json:"weight"`
	// UpgradeThreshold 该等级的消费门槛；用户端据此显示「再消费 X 元升级」。
	UpgradeThreshold float64 `json:"upgrade_threshold"`
}

// SessionInput 开会话所需的字段（对应 user_sessions 的 NOT NULL 列）。
type SessionInput struct {
	SessionID string
	UserID    uint64
	Username  string
	Platform  string
	IP        string
	UserAgent string
	// DeviceFingerprint 客户端上报的设备指纹（可空）。
	//
	// 会话列表是运营判断「这个登录态是不是本人」的第一现场；只填 IP 时，
	// 同一 NAT 出口下的所有会话看起来完全一样，无法区分。
	DeviceFingerprint string
	LoginAt           time.Time
	// ExpiredAt 会话过期时间（一般 = LoginAt + JWT 有效期）。
	//
	// 必须写：此前该列恒为 NULL，而「在线」是靠 status='active' 判的，
	// 于是用户只要登录过一次就永远算在线。写入后「有效会话」= status 为 active
	// 且 expired_at 未到期，在线数才会自然回落。
	ExpiredAt time.Time
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户中心数据访问实例。
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// 注销（软删除）的账号一律不参与登录与占用判定：注销后 username/email 应可被
// 重新注册，且不得再通过任何自助入口命中旧行（部分唯一索引也只约束未注销行）。
// 管理端要看注销用户时走 admin 模块的仓储（IncludeDeleted），不走这里。
func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("username = ? AND deleted_at IS NULL", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// DefaultLevelID 返回起始等级：启用中、权重最低的一级。
//
// 用「权重最低」而不是写死某个 code：等级是运营可改名/可增删的配置，
// 写死 code 会让运营改名当天注册链路整条断掉。无可用等级时返回 0，
// 调用方按「不设等级」处理（不阻断注册）。
func (r *userRepository) DefaultLevelID(ctx context.Context) (uint64, error) {
	var id uint64
	err := r.db.WithContext(ctx).
		Table("user_levels").
		Select("id").
		Where("status = ?", "active").
		Order("weight asc, id asc").
		Limit(1).
		Scan(&id).Error
	return id, err
}

// LevelOf 读取用户当前等级的展示信息（名称/图标/配色/门槛）。
//
// 无等级（user_level_id 为空，或指向已删除的行）返回 (nil, nil)：用户端据此
// 不渲染徽章，而不是显示一个空白标签。等级行被删的兜底 JOIN 会落空，
// 与「未设等级」同一条路径处理。
func (r *userRepository) LevelOf(ctx context.Context, userID uint64) (*LevelBrief, error) {
	var brief LevelBrief
	err := r.db.WithContext(ctx).
		Table("user_levels").
		Select("user_levels.id, user_levels.name, user_levels.code, user_levels.icon, user_levels.color, user_levels.weight, user_levels.upgrade_threshold").
		Joins("JOIN users ON users.user_level_id = user_levels.id").
		Where("users.id = ?", userID).
		Take(&brief).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &brief, nil
}

// DefaultUserRoleID 返回客户默认角色（roles.code='user'）的 ID；缺失返回 0。
func (r *userRepository) DefaultUserRoleID(ctx context.Context) (uint64, error) {
	var id uint64
	err := r.db.WithContext(ctx).
		Table("roles").
		Select("id").
		Where("code = ?", "user").
		Limit(1).
		Scan(&id).Error
	return id, err
}

// BindRole 绑定一条客户角色关系（幂等：重复绑定不报错）。
func (r *userRepository) BindRole(ctx context.Context, userID, roleID uint64) error {
	if userID == 0 || roleID == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO user_roles (user_id, role_id) VALUES (?, ?) ON CONFLICT (user_id, role_id) DO NOTHING`,
		userID, roleID).Error
}

func (r *userRepository) UpdateLoginProfile(ctx context.Context, id uint64, ip string, loginAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"last_login_at": loginAt,
		"last_login_ip": ip,
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

// PermissionsOf 读取子账号权限表；主账号不查（返回 nil）。
func (r *userRepository) PermissionsOf(ctx context.Context, id uint64) ([]string, error) {
	var isSub bool
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Select("is_sub_account").Where("id = ?", id).Scan(&isSub).Error; err != nil {
		return nil, err
	}
	if !isSub {
		return nil, nil
	}
	var codes []string
	if err := r.db.WithContext(ctx).
		Table("sub_account_permissions").
		Where("user_id = ?", id).
		Pluck("permission_code", &codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// FindByPhone 按手机号查找用户（短信验证码登录用）。同样排除注销账号。
func (r *userRepository) FindByPhone(ctx context.Context, phone string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("phone = ? AND deleted_at IS NULL", phone).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// MarkVerified 写回绑定验证时间。channel 只区分 sms / email 两类，
// 未知值按 email 处理（与 captcha 模块 TargetResolver.SetVerifiedAt 口径一致）。
func (r *userRepository) MarkVerified(ctx context.Context, id uint64, channel string, verifiedAt time.Time) error {
	column := "email_verified_at"
	if channel == "sms" {
		column = "phone_verified_at"
	}
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).
		Update(column, verifiedAt).Error
}

// RevokeSessions 撤销该用户全部有效会话。
//
// 表 user_sessions 由安全模块维护（migration 004）；这里只做状态置位，
// 不做级联删除，保留审计线索。无有效会话时影响 0 行，不算失败。
//
// 返回被撤销的 session_id：调用方（认证服务）据此失效会话缓存，
// 否则「撤销」要等缓存 TTL 到期才真正生效，而 doc91 §9 要求立即生效。
func (r *userRepository) RevokeSessions(ctx context.Context, id uint64, reason string) ([]string, error) {
	if strings.TrimSpace(reason) == "" {
		reason = "password_reset"
	}
	var sessionIDs []string
	if err := r.db.WithContext(ctx).Table("user_sessions").
		Where("user_id = ? AND status = ?", id, "active").
		Pluck("session_id", &sessionIDs).Error; err != nil {
		return nil, err
	}
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	if err := r.db.WithContext(ctx).Table("user_sessions").
		Where("user_id = ? AND status = ?", id, "active").
		Updates(map[string]any{
			"status":         "revoked",
			"revoked_reason": reason,
			"revoked_at":     time.Now(),
		}).Error; err != nil {
		return nil, err
	}
	return sessionIDs, nil
}

// RevokeSession 撤销单个会话（登出）。
//
// 与 RevokeSessions 的区别：登出只结束当前这个登录态，其他设备的登录不受影响 ——
// 「在一台机器上点退出，手机也被踢下线」是明确的错误行为。
//
// 返回是否真的更新了一行：会话不存在或已失效时返回 (false, nil)。
// 登出接口对这两种情况一视同仁（都返回成功），避免把「会话是否存在于服务端」
// 这个信息暴露给调用方。
func (r *userRepository) RevokeSession(ctx context.Context, sessionID, reason string) (bool, error) {
	if strings.TrimSpace(sessionID) == "" {
		return false, nil
	}
	if strings.TrimSpace(reason) == "" {
		reason = "logout"
	}
	res := r.db.WithContext(ctx).Table("user_sessions").
		Where("session_id = ? AND status = ?", sessionID, "active").
		Updates(map[string]any{
			"status":         "revoked",
			"revoked_reason": reason,
			"revoked_at":     time.Now(),
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// OpenSession 写一条 user_sessions（doc104 F15）。
//
// 会话表此前只有 seed 与管理员踢人两条写入路径，「登录日志有记录但会话列表
// 为空」，安全页的「当前登录态」永远看不到东西，强制下线也无对象可作用。
// 密码/验证码登录成功后补上这一笔，与 OAuth 路径（oauth/service/helpers.go
// 的 openSession）口径一致。
//
// 写失败不阻断登录：会话记录是审计增强，不是登录的前置条件。
func (r *userRepository) OpenSession(ctx context.Context, in SessionInput) error {
	platform := in.Platform
	if strings.TrimSpace(platform) == "" {
		platform = "web"
	}
	loginAt := in.LoginAt
	if loginAt.IsZero() {
		loginAt = time.Now()
	}
	// expired_at 必须写：为 NULL 时「在线」判定只能靠 status 状态位，
	// 而状态位没有任何自动回收路径，用户登录一次就永久算在线（实测过）。
	var expiredAt any
	if !in.ExpiredAt.IsZero() {
		expiredAt = in.ExpiredAt
	}
	return r.db.WithContext(ctx).Exec(`INSERT INTO user_sessions
		(session_id, user_id, username, platform, ip, user_agent, device_fingerprint, login_at, last_active_at, expired_at, status, subject_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', 'user', NOW(), NOW())`,
		in.SessionID, in.UserID, in.Username, platform,
		in.IP, truncateBytes(in.UserAgent, 255), truncateBytes(in.DeviceFingerprint, 255),
		loginAt, loginAt, expiredAt).Error
}

// truncateBytes 按字节截断到 UTF-8 字符边界（DB 列有长度上限，超长会整条 INSERT 失败）。
func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}
