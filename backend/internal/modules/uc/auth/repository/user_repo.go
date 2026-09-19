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
	// UpdateLoginProfile 更新用户登录档案（最近登录 IP、归属地、时间）。
	UpdateLoginProfile(ctx context.Context, id uint64, ip, ipRegion string, loginAt time.Time) error
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
	RevokeSessions(ctx context.Context, id uint64, reason string) error
	// OpenSession 登录成功后开一条会话记录（doc104 F15）。
	OpenSession(ctx context.Context, in SessionInput) error
}

// SessionInput 开会话所需的字段（对应 user_sessions 的 NOT NULL 列）。
type SessionInput struct {
	SessionID string
	UserID    uint64
	Username  string
	Platform  string
	IP        string
	IPRegion  string
	UserAgent string
	LoginAt   time.Time
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
func (r *userRepository) RevokeSessions(ctx context.Context, id uint64, reason string) error {
	if strings.TrimSpace(reason) == "" {
		reason = "password_reset"
	}
	return r.db.WithContext(ctx).Table("user_sessions").
		Where("user_id = ? AND status = ?", id, "active").
		Updates(map[string]any{
			"status":         "revoked",
			"revoked_reason": reason,
			"revoked_at":     time.Now(),
		}).Error
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
	return r.db.WithContext(ctx).Exec(`INSERT INTO user_sessions
		(session_id, user_id, username, platform, ip, ip_region, user_agent, login_at, last_active_at, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', NOW(), NOW())`,
		in.SessionID, in.UserID, in.Username, platform,
		in.IP, in.IPRegion, truncateBytes(in.UserAgent, 255), loginAt, loginAt).Error
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
