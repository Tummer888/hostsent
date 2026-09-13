package adapter

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/captcha/service"
)

// TargetResolver 基于数据库的 OTP 目标解析（users / admins 表）。
type TargetResolver struct {
	db *gorm.DB
}

// NewTargetResolver 创建解析器。
func NewTargetResolver(db *gorm.DB) *TargetResolver {
	return &TargetResolver{db: db}
}

// ResolveUser 按用户 ID 或账号解析目标。
//
// 账号匹配顺序：用户名 → 邮箱 → 手机号（登录页传的是用户输入，可能是任意一种）。
func (r *TargetResolver) ResolveUser(ctx context.Context, userID uint64, account string) (*service.TargetInfo, error) {
	type row struct {
		ID              uint64
		Username        string
		Email           string
		Phone           string
		PhoneVerifiedAt *time.Time
		EmailVerifiedAt *time.Time
	}
	var out row
	q := r.db.WithContext(ctx).Table("users").
		Select("id, COALESCE(username,'') AS username, COALESCE(email,'') AS email, COALESCE(phone,'') AS phone, phone_verified_at, email_verified_at")
	if userID > 0 {
		q = q.Where("id = ?", userID)
	} else {
		acc := strings.TrimSpace(account)
		if acc == "" {
			return &service.TargetInfo{}, nil
		}
		// users.username / email 为唯一；phone 允许为空（空串不参与匹配）。
		q = q.Where("username = ? OR email = ? OR (phone <> '' AND phone = ?)", acc, strings.ToLower(acc), acc).Limit(1)
	}
	if err := q.Scan(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &service.TargetInfo{}, nil
		}
		return nil, err
	}
	if out.ID == 0 {
		return &service.TargetInfo{}, nil
	}
	return &service.TargetInfo{
		UserID:        out.ID,
		Phone:         out.Phone,
		PhoneVerified: out.PhoneVerifiedAt != nil,
		Email:         out.Email,
		EmailVerified: out.EmailVerifiedAt != nil,
		Username:      out.Username,
		Exists:        true,
	}, nil
}

// ResolveAdmin 按管理员 ID 或用户名解析目标。
func (r *TargetResolver) ResolveAdmin(ctx context.Context, adminID uint64, username string) (*service.TargetInfo, error) {
	type row struct {
		ID       uint64
		Username string
		Email    string
		Phone    string
	}
	var out row
	q := r.db.WithContext(ctx).Table("admins").
		Select("id, COALESCE(username,'') AS username, COALESCE(email,'') AS email, COALESCE(phone,'') AS phone")
	if adminID > 0 {
		q = q.Where("id = ?", adminID)
	} else {
		acc := strings.TrimSpace(username)
		if acc == "" {
			return &service.TargetInfo{}, nil
		}
		q = q.Where("username = ?", acc).Limit(1)
	}
	if err := q.Scan(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &service.TargetInfo{}, nil
		}
		return nil, err
	}
	if out.ID == 0 {
		return &service.TargetInfo{}, nil
	}
	return &service.TargetInfo{
		Phone:    out.Phone,
		Email:    out.Email,
		Username: out.Username,
		Exists:   true,
	}, nil
}

// SetVerifiedAt 写回绑定验证时间（注册/绑定成功后调用）。
func (r *TargetResolver) SetVerifiedAt(ctx context.Context, userID uint64, channel string, at time.Time) error {
	column := "email_verified_at"
	if channel == "sms" {
		column = "phone_verified_at"
	}
	return r.db.WithContext(ctx).Table("users").Where("id = ?", userID).
		Update(column, at).Error
}

// LoginLogRecorder 登录日志落库（login_logs）+ 降级失败计数查询。
type LoginLogRecorder struct {
	db *gorm.DB
}

// NewLoginLogRecorder 创建记录器。
func NewLoginLogRecorder(db *gorm.DB) *LoginLogRecorder {
	return &LoginLogRecorder{db: db}
}

// RecordLogin 写入一条登录日志；失败不影响登录主流程（由调用方吞掉错误）。
func (r *LoginLogRecorder) RecordLogin(ctx context.Context, e service.LoginLogEntry) error {
	if r.db == nil {
		return nil
	}
	platform := e.Platform
	if platform == "" {
		platform = "web"
	}
	return r.db.WithContext(ctx).Exec(`INSERT INTO login_logs
		(user_id, username, login_type, result, failure_reason, ip, ip_region, user_agent, platform, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		e.UserID, e.Username, e.LoginType, e.Result, e.FailureReason,
		e.IP, e.IPRegion, truncate(e.UserAgent, 255), platform).Error
}

// CountRecentFailures 统计窗口内失败次数（Redis 降级路径）。
func (r *LoginLogRecorder) CountRecentFailures(ctx context.Context, account, ip string, since time.Time) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	var n int64
	q := r.db.WithContext(ctx).Table("login_logs").
		Where("result = ? AND created_at > ?", service.LoginResultFailed, since)
	if account != "" {
		q = q.Where("username = ?", account)
	}
	if ip != "" {
		// 账号或 IP 任一命中即计入（与 Redis 双维度计数口径一致）。
		if account != "" {
			q = q.Where("username = ? OR ip = ?", account, ip)
		} else {
			q = q.Where("ip = ?", ip)
		}
	}
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
