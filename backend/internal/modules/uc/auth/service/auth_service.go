package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/auth/dto"
	"hostsent/backend/internal/modules/uc/auth/model"
	"hostsent/backend/internal/modules/uc/auth/repository"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/netutil"
	"hostsent/backend/internal/pkg/security"
)

// AuthService 用户中心认证服务接口。
// 提供用户自助场景下的注册、登录、信息查询等能力。
type AuthService interface {
	// Login 用户登录（doc91 §5.3）：password / sms / email 三种方式。
	// 返回 NeedOTP=true 时不签发访问令牌，需再调 VerifyLoginOTP。
	Login(ctx context.Context, req dto.LoginRequest, ip, userAgent string) (*dto.LoginResponse, error)
	// VerifyLoginOTP 完成登录二次验证，成功返回正式令牌（doc91 §4.6）。
	VerifyLoginOTP(ctx context.Context, req dto.VerifyOTPRequest, ip, userAgent string) (*dto.LoginResponse, error)
	// Register 用户注册：图形码 → 邮箱 OTP → 唯一性/加密/创建 → 写 email_verified_at。
	Register(ctx context.Context, req dto.RegisterRequest, ip, userAgent string) (uint64, error)
	// ForgotPassword 忘记密码：下发 OTP，恒定返回 sent=true（防账号枚举）。
	ForgotPassword(ctx context.Context, req dto.ForgotPasswordRequest, ip, userAgent string) (*dto.ForgotPasswordResponse, error)
	// ResetPassword 重置密码：校验 OTP → 改密 → 撤销历史会话。
	ResetPassword(ctx context.Context, req dto.ResetPasswordRequest, ip, userAgent string) error
	// UserInfo 获取当前登录用户信息。
	UserInfo(ctx context.Context, userID uint64) (*dto.UserInfo, error)
	// UpdateProfile 更新当前用户基本资料，返回更新后的用户信息。
	UpdateProfile(ctx context.Context, userID uint64, req dto.UpdateProfileRequest) (*dto.UserInfo, error)
	// ChangePassword 修改密码：校验旧密码后写入新密码哈希。
	ChangePassword(ctx context.Context, userID uint64, req dto.ChangePasswordRequest) error
	// SetSecurityPort 注入登录安全端口（图形码/OTP/锁定/登录日志，doc91 C3）。
	SetSecurityPort(port security.Port)
	// SetInviteBinder 注入推广邀请关系绑定能力（可选，装配层调用）。
	SetInviteBinder(binder InviteBinder)
	// SetDefaultGroupResolver 注入默认用户组解析能力（可选，装配层调用）。
	SetDefaultGroupResolver(resolver DefaultGroupResolver)
	// SetSalesOwnerClaimer 注入注册后的销售归属自动认领能力（可选，装配层调用，doc86 S4）。
	SetSalesOwnerClaimer(claimer SalesOwnerClaimer)
}

// InviteBinder 注册时的推广邀请关系绑定能力（由返现模块实现，装配层注入）。
// 抽成接口是为了避免 uc/auth 直接依赖 admin 返现服务。
type InviteBinder interface {
	// ResolveInviter 按邀请码解析邀请人；无效码返回 0（不报错）。
	ResolveInviter(ctx context.Context, code string) (uint64, error)
	// EnsureInviteCode 返回用户邀请码，缺失时生成并落库。
	EnsureInviteCode(ctx context.Context, userID uint64) (string, error)
	// BindInviter 绑定单级邀请关系（一次性，已有邀请人不覆盖）。
	BindInviter(ctx context.Context, inviteeID, inviterID uint64) error
}

// SalesOwnerClaimer 注册后的销售归属自动认领（由销售模块实现，装配层注入）。
// 抽成接口是为了避免 uc/auth 直接依赖 admin/sales 服务。
type SalesOwnerClaimer interface {
	// EnsureAutoClaim 客户无归属时按「客户数最少的在职销售」自动归属；失败/无候选不报错。
	EnsureAutoClaim(ctx context.Context, userID uint64) error
}

// DefaultGroupResolver 解析「默认用户组」，注册时兜底归组（由用户组服务实现，装配层注入）。
// 抽成接口是为了避免 uc/auth 直接依赖 admin 用户组模块。
type DefaultGroupResolver interface {
	// DefaultGroupID 返回默认用户组 ID；未配置时返回 0（不报错）。
	DefaultGroupID(ctx context.Context) (uint64, error)
}

type authService struct {
	repo             repository.UserRepository
	jwtIssuer        *appauth.JWTIssuer
	ipRegionResolver netutil.IPRegionResolver
	logger           *zap.Logger
	inviteBinder     InviteBinder         // 可选：注册时生成邀请码并绑定邀请关系
	defaultGroup     DefaultGroupResolver // 可选：注册时兜底归入默认用户组
	salesClaimer     SalesOwnerClaimer    // 可选：注册后自动归属销售（doc86 S4）
	// sec 登录安全端口（doc91 C3）；未装配时登录行为与升级前完全一致。
	sec security.Port
}

// NewAuthService 创建用户中心认证服务实例。
func NewAuthService(repo repository.UserRepository, jwtIssuer *appauth.JWTIssuer, ipRegionResolver netutil.IPRegionResolver, logger *zap.Logger) AuthService {
	return &authService{repo: repo, jwtIssuer: jwtIssuer, ipRegionResolver: ipRegionResolver, logger: logger}
}

// SetSecurityPort 注入登录安全端口（doc91 C3，装配层调用）。
func (s *authService) SetSecurityPort(port security.Port) {
	s.sec = port
}

// SetInviteBinder 注入推广邀请绑定能力。
func (s *authService) SetInviteBinder(binder InviteBinder) {
	s.inviteBinder = binder
}

// SetDefaultGroupResolver 注入默认用户组解析能力。
func (s *authService) SetDefaultGroupResolver(resolver DefaultGroupResolver) {
	s.defaultGroup = resolver
}

// SetSalesOwnerClaimer 注入销售归属自动认领能力。
func (s *authService) SetSalesOwnerClaimer(claimer SalesOwnerClaimer) {
	s.salesClaimer = claimer
}

// Login 执行用户登录（doc91 §5.3）。
//
// 三种 login_type：
//   - password：图形码（user_login）→ 锁定检查 → 密码 → 二次验证（user_login）
//   - sms：图形码（user_login_sms）→ 锁定检查 → 按手机号查用户 → 消费短信 OTP → 发 token
//   - email：同上，按邮箱登录（user_login_email）
//
// sms/email 场景的 OTP 是一次性登录凭据，与二次验证的 OTP 场景 key 隔离，
// 因此不存在「拿二次验证的码走短信登录」这类跨场景重放。
func (s *authService) Login(ctx context.Context, req dto.LoginRequest, ip, userAgent string) (*dto.LoginResponse, error) {
	loginType := strings.ToLower(strings.TrimSpace(req.LoginType))
	if loginType == "" {
		loginType = dto.LoginTypePassword
	}
	if loginType == dto.LoginTypeSMS || loginType == dto.LoginTypeEmail {
		return s.loginByCode(ctx, loginType, req, ip, userAgent)
	}
	return s.loginByPassword(ctx, req, ip, userAgent)
}

// loginByPassword 账号密码登录。
func (s *authService) loginByPassword(ctx context.Context, req dto.LoginRequest, ip, userAgent string) (*dto.LoginResponse, error) {
	account := strings.TrimSpace(req.Username)
	if account == "" || req.Password == "" {
		return nil, errors.New("用户名和密码不能为空")
	}
	if s.sec != nil {
		// ① 锁定检查（默认关，行为与升级前一致）。
		if err := s.sec.CheckLocked(ctx, account, ip); err != nil {
			s.logLogin(ctx, 0, account, "password", security.LoginResultFailed, "locked", ip, userAgent)
			return nil, err
		}
		// ② 图形码（user_login 场景要求时）。
		if s.sec.EffectiveImageRequired(ctx, security.SceneUserLogin, security.Subject{}) {
			if err := s.sec.VerifyImage(ctx, security.SceneUserLogin, req.CaptchaKey, req.CaptchaCode); err != nil {
				s.logLogin(ctx, 0, account, "password", security.LoginResultFailed, "captcha", ip, userAgent)
				return nil, err
			}
		}
	}

	user, err := s.repo.FindByUsername(ctx, account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.failLogin(ctx, account, "password", "user_not_found", ip, userAgent)
			return nil, security.ErrInvalidCredential // 不暴露用户是否存在
		}
		return nil, err
	}

	if user.Status != "active" {
		s.failLogin(ctx, account, "password", "disabled", ip, userAgent)
		return nil, security.ErrLoginDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.failLogin(ctx, account, "password", "bad_password", ip, userAgent)
		return nil, security.ErrInvalidCredential
	}

	// ③ 二次验证：命中则不签发令牌，只返回待验证令牌。
	if s.sec != nil {
		if required, _ := s.sec.EffectiveOTP(ctx, security.SceneUserLogin, security.Subject{ID: user.ID}); required {
			pending, perr := s.sec.IssueOTPPending(ctx, security.SceneUserLogin, security.Subject{ID: user.ID}, ip, userAgent)
			if perr != nil {
				return nil, perr
			}
			return &dto.LoginResponse{
				NeedOTP:         true,
				OTPToken:        pending.Token,
				OTPChannel:      pending.Channel,
				OTPTargetMasked: pending.TargetMasked,
				OTPExpireIn:     pending.ExpireIn,
			}, nil
		}
	}

	return s.finishLogin(ctx, user, "password", ip, userAgent)
}

// loginByCode 短信/邮箱验证码登录：验证码即登录凭据，不再走二次验证。
func (s *authService) loginByCode(ctx context.Context, loginType string, req dto.LoginRequest, ip, userAgent string) (*dto.LoginResponse, error) {
	scene := security.SceneUserLoginSMS
	rawTarget := strings.TrimSpace(req.Phone)
	if loginType == dto.LoginTypeEmail {
		scene = security.SceneUserLoginEmail
		rawTarget = strings.TrimSpace(req.Email)
	}
	if rawTarget == "" || strings.TrimSpace(req.Code) == "" {
		return nil, errors.New("请填写登录账号与验证码")
	}
	if s.sec != nil {
		if err := s.sec.CheckLocked(ctx, rawTarget, ip); err != nil {
			s.logLogin(ctx, 0, rawTarget, loginType, security.LoginResultFailed, "locked", ip, userAgent)
			return nil, err
		}
		// 图形码先于验证码校验：防脚本批量试码（两个登录场景基线都开图形码）。
		if s.sec.EffectiveImageRequired(ctx, scene, security.Subject{}) {
			if err := s.sec.VerifyImage(ctx, scene, req.CaptchaKey, req.CaptchaCode); err != nil {
				s.logLogin(ctx, 0, rawTarget, loginType, security.LoginResultFailed, "captcha", ip, userAgent)
				return nil, err
			}
		}
	}

	user, err := s.findByLoginTarget(ctx, loginType, rawTarget)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 未绑定/不存在都按同一提示，避免枚举手机号与邮箱。
			s.failLogin(ctx, rawTarget, loginType, "user_not_found", ip, userAgent)
			return nil, security.ErrTargetUnbound
		}
		return nil, err
	}
	if user.Status != "active" {
		s.failLogin(ctx, rawTarget, loginType, "disabled", ip, userAgent)
		return nil, security.ErrLoginDisabled
	}
	// 未绑定目标：手机号为空/邮箱为空时不能走验证码登录。
	if (loginType == dto.LoginTypeSMS && strings.TrimSpace(user.Phone) == "") ||
		(loginType == dto.LoginTypeEmail && strings.TrimSpace(user.Email) == "") {
		s.failLogin(ctx, rawTarget, loginType, "target_unbound", ip, userAgent)
		return nil, security.ErrTargetUnbound
	}

	if s.sec != nil {
		// 消费一次性登录凭据：场景 key 与二次验证隔离，不可重放。
		if err := s.sec.VerifyOTP(ctx, scene, rawTarget, req.Code); err != nil {
			s.failLogin(ctx, rawTarget, loginType, "bad_code", ip, userAgent)
			return nil, err
		}
	}

	// 验证码登录成功即视为已验证该联系方式（用户已持有该手机/邮箱）。
	_ = s.repo.MarkVerified(ctx, user.ID, loginType, time.Now())
	return s.finishLogin(ctx, user, loginType, ip, userAgent)
}

// findByLoginTarget 按登录目标查用户：手机号优先，其次邮箱（用户可能用邮箱登录但填了手机）。
func (s *authService) findByLoginTarget(ctx context.Context, loginType, target string) (*model.User, error) {
	if loginType == dto.LoginTypeSMS {
		if u, err := s.repo.FindByPhone(ctx, target); err == nil {
			return u, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, gorm.ErrRecordNotFound
	}
	return s.repo.FindByEmail(ctx, target)
}

// finishLogin 登录成功的收尾：更新登录档案 + 开会话 + 清零失败计数 + 写成功日志 + 签发令牌。
func (s *authService) finishLogin(ctx context.Context, user *model.User, loginType, ip, userAgent string) (*dto.LoginResponse, error) {
	if err := s.updateLoginProfile(ctx, user.ID, ip); err != nil {
		return nil, err
	}
	// 写 user_sessions（doc104 F15）：失败只告警，不影响登录本身。
	if err := s.openSession(ctx, user, loginType, ip, userAgent); err != nil && s.logger != nil {
		s.logger.Warn("open user session failed",
			zap.Uint64("user_id", user.ID), zap.String("login_type", loginType), zap.Error(err))
	}
	if s.sec != nil {
		s.sec.ResetFailure(ctx, user.Username, ip)
	}
	s.logLogin(ctx, user.ID, user.Username, loginType, security.LoginResultSuccess, "", ip, userAgent)

	// 重新读取以获取最新登录档案
	latest, err := s.repo.FindByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return s.buildLoginResponse(ctx, latest), nil
}

// openSession 写一条 user_sessions 记录（doc104 F15）。
//
// session_id 与 JWT 无关，只用于安全页展示与「强制下线」定位；真正的令牌
// 失效靠 user_sessions.status 状态位（见 RevokeSessions）。
func (s *authService) openSession(ctx context.Context, user *model.User, loginType, ip, userAgent string) error {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	ipRegion := ""
	if s.ipRegionResolver != nil && ip != "" {
		ipRegion = s.ipRegionResolver.Resolve(ctx, ip)
	}
	return s.repo.OpenSession(ctx, repository.SessionInput{
		SessionID: loginType + "_" + hex.EncodeToString(buf),
		UserID:    user.ID,
		Username:  user.Username,
		Platform:  "web",
		IP:        ip,
		IPRegion:  ipRegion,
		UserAgent: userAgent,
		LoginAt:   time.Now(),
	})
}

// VerifyLoginOTP 完成登录二次验证（doc91 §4.6）。
//
// 端口层已校验 JWT 的 aud=otp_pending 与 jti 一次性；这里再确认场景确实
// 是用户端登录场景，防止把其它用途的待验证令牌拿来换登录。
func (s *authService) VerifyLoginOTP(ctx context.Context, req dto.VerifyOTPRequest, ip, userAgent string) (*dto.LoginResponse, error) {
	if s.sec == nil {
		return nil, security.ErrInvalidOTPToken
	}
	pending, err := s.sec.ConsumeOTPPending(ctx, req.OTPToken, req.Code)
	if err != nil {
		return nil, err
	}
	if pending.IsAdmin || pending.Scene != security.SceneUserLogin {
		return nil, security.ErrInvalidOTPToken
	}
	user, err := s.repo.FindByID(ctx, pending.UserID)
	if err != nil {
		return nil, security.ErrInvalidOTPToken
	}
	if user.Status != "active" {
		return nil, security.ErrLoginDisabled
	}
	// 二次验证通过即视为该通道可用（SMS 场景顺带把手机号标记为已验证）。
	if pending.Channel == security.ChannelSMS {
		_ = s.repo.MarkVerified(ctx, user.ID, security.ChannelSMS, time.Now())
	}
	return s.finishLogin(ctx, user, "password", ip, userAgent)
}

// Register 执行用户注册（doc91 §5.4）。
//
// 顺序：注册开关 → 图形码（user_register）→ 邮箱 OTP（策略要求时）
// → 密码策略 → 唯一性 → bcrypt → 创建 → 写 email_verified_at。
func (s *authService) Register(ctx context.Context, req dto.RegisterRequest, ip, userAgent string) (uint64, error) {
	// ① 注册开关（现状后端不读；关闭时返回 20016）。
	if s.sec != nil && !s.sec.RegisterEnabled(ctx) {
		return 0, security.ErrRegisterDisabled
	}
	if s.sec != nil {
		// ② 图形码（user_register 基线要求）。
		if s.sec.EffectiveImageRequired(ctx, security.SceneUserRegister, security.Subject{}) {
			if err := s.sec.VerifyImage(ctx, security.SceneUserRegister, req.CaptchaKey, req.CaptchaCode); err != nil {
				return 0, err
			}
		}
	}
	// ③ 邮箱验证码：基线 otp_required=true 时必须校验（现状注册邮箱未验证）。
	// 端口未装配（单测/裁剪部署）时无从校验，按不需要处理。
	emailOTPRequired := false
	if s.sec != nil {
		required, _ := s.sec.EffectiveOTP(ctx, security.SceneUserRegister, security.Subject{})
		emailOTPRequired = required
	}
	if emailOTPRequired {
		if strings.TrimSpace(req.EmailCode) == "" {
			return 0, errors.New("请填写邮箱验证码")
		}
		if err := s.sec.VerifyOTP(ctx, security.SceneUserRegister, req.Email, req.EmailCode); err != nil {
			return 0, err
		}
	}
	// ④ 密码策略（6 个 password_* 开关本轮接线）。
	if s.sec != nil {
		if err := s.sec.ValidatePassword(ctx, req.Password); err != nil {
			return 0, err
		}
	}

	// 检查用户名唯一性
	if _, err := s.repo.FindByUsername(ctx, req.Username); err == nil {
		return 0, errors.New("用户名已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	// 检查邮箱唯一性
	if _, err := s.repo.FindByEmail(ctx, req.Email); err == nil {
		return 0, errors.New("邮箱已被注册")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
		Status:       "active",
		Tier:         "free",
		UserGroupID:  s.resolveDefaultGroupID(ctx),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return 0, err
	}

	// ⑤ 注册即完成邮箱验证：上面的 OTP 校验已经证明邮箱可用。
	_ = s.repo.MarkVerified(ctx, user.ID, "email", time.Now())

	// 推广邀请：失败一律不阻断注册（邀请码只影响返现归属，不影响账号可用性）。
	s.setupInvite(ctx, user.ID, req.InviteCode)
	// 销售归属自动认领：新客户无归属时按负载补给在职销售；失败同样不阻断注册。
	s.setupSalesOwner(ctx, user.ID)
	return user.ID, nil
}

// ForgotPassword 忘记密码第一步：向账号绑定的邮箱/手机下发 OTP（doc91 §5.5）。
//
// 防账号枚举：无论账号是否存在都返回 sent=true，且不回显目标打码值。
// 账号不存在时不真发（省通道成本），但响应与成功一致。
func (s *authService) ForgotPassword(ctx context.Context, req dto.ForgotPasswordRequest, ip, userAgent string) (*dto.ForgotPasswordResponse, error) {
	account := strings.TrimSpace(req.Account)
	if account == "" {
		return nil, errors.New("请填写账号")
	}
	if s.sec == nil {
		return &dto.ForgotPasswordResponse{Sent: true}, nil
	}
	// 图形码先校验：这是短信/邮件轰炸的第一道闸。
	if s.sec.EffectiveImageRequired(ctx, security.ScenePasswordReset, security.Subject{}) {
		if err := s.sec.VerifyImage(ctx, security.ScenePasswordReset, req.CaptchaKey, req.CaptchaCode); err != nil {
			return nil, err
		}
	}
	target, err := s.sec.LookupTarget(ctx, account)
	if err != nil || target == nil || !target.Exists {
		// 不真发、不报错：响应与账号存在时完全一致。
		if err != nil {
			s.warn("找回密码查询账号失败", 0, err)
		}
		return &dto.ForgotPasswordResponse{Sent: true}, nil
	}
	// 通道按账号可用性选择：优先基线通道，不可用时退回另一种。
	channel, addr := chooseResetChannel(ctx, s.sec, target)
	if addr == "" {
		return &dto.ForgotPasswordResponse{Sent: true}, nil
	}
	// 图形码已在上面消费，这里跳过服务层的二次校验。
	if _, serr := s.sec.SendToTarget(ctx, security.SendInput{
		Scene:       security.ScenePasswordReset,
		Channel:     channel,
		Target:      addr,
		Subject:     security.Subject{ID: target.UserID},
		IP:          ip,
		UserAgent:   userAgent,
		SkipCaptcha: true,
	}); serr != nil {
		// 频控/通道故障都不向调用方暴露（否则等于确认账号存在）。
		s.warn("找回密码下发验证码失败", target.UserID, serr)
	}
	return &dto.ForgotPasswordResponse{Sent: true}, nil
}

// chooseResetChannel 选择重置密码的下发通道与目标。
func chooseResetChannel(ctx context.Context, port security.Port, target *security.Target) (string, string) {
	_, channel := port.EffectiveOTP(ctx, security.ScenePasswordReset, security.Subject{ID: target.UserID})
	if channel == security.ChannelSMS && strings.TrimSpace(target.Phone) != "" {
		return security.ChannelSMS, target.Phone
	}
	if strings.TrimSpace(target.Email) != "" {
		return security.ChannelEmail, target.Email
	}
	if strings.TrimSpace(target.Phone) != "" {
		return security.ChannelSMS, target.Phone
	}
	return "", ""
}

// ResetPassword 忘记密码第二步：校验 OTP → 改密 → 撤销历史会话（doc91 §5.5）。
func (s *authService) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest, ip, userAgent string) error {
	account := strings.TrimSpace(req.Account)
	if account == "" {
		return errors.New("请填写账号")
	}
	if s.sec == nil {
		return security.ErrSceneDisabled
	}
	target, err := s.sec.LookupTarget(ctx, account)
	if err != nil {
		return err
	}
	if target == nil || !target.Exists {
		return security.ErrCaptchaInvalid // 不暴露账号是否存在
	}
	channel, addr := chooseResetChannel(ctx, s.sec, target)
	if addr == "" {
		return security.ErrTargetUnbound
	}
	if err := s.sec.VerifyOTP(ctx, security.ScenePasswordReset, addr, req.Code); err != nil {
		return err
	}
	if err := s.sec.ValidatePassword(ctx, req.NewPassword); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(ctx, target.UserID, string(hash)); err != nil {
		return err
	}
	// 改密后撤销全部历史会话（JWT 无状态，靠 user_sessions 状态位失效）。
	if err := s.repo.RevokeSessions(ctx, target.UserID, "password_reset"); err != nil {
		s.warn("撤销会话失败", target.UserID, err)
	}
	// 重置即验证了该通道可用。
	_ = s.repo.MarkVerified(ctx, target.UserID, channel, time.Now())
	s.logLogin(ctx, target.UserID, target.Username, "password", security.LoginResultSuccess, "", ip, userAgent)
	return nil
}

// logLogin 写一条登录日志（成功与失败都写，doc91 C6）。
func (s *authService) logLogin(ctx context.Context, userID uint64, username, loginType, result, reason, ip, userAgent string) {
	if s.sec == nil {
		return
	}
	ipRegion := ""
	if s.ipRegionResolver != nil && ip != "" {
		ipRegion = s.ipRegionResolver.Resolve(ctx, ip)
	}
	s.sec.Log(ctx, security.LoginLogEntry{
		UserID: userID, Username: username, LoginType: loginType,
		Result: result, FailureReason: reason, IP: ip, IPRegion: ipRegion,
		UserAgent: userAgent, Platform: "web",
	})
}

// failLogin 记录一次登录失败（计数 + 日志）。
func (s *authService) failLogin(ctx context.Context, username, loginType, reason, ip, userAgent string) {
	if s.sec == nil {
		return
	}
	s.sec.RecordFailure(ctx, username, ip)
	s.logLogin(ctx, 0, username, loginType, security.LoginResultFailed, reason, ip, userAgent)
}

// setupSalesOwner 注册后自动归属销售（doc86 S4）。
// 归属失败只影响提成归谁，不影响账号可用性，因此与邀请码同样「只记日志」。
func (s *authService) setupSalesOwner(ctx context.Context, userID uint64) {
	if s.salesClaimer == nil {
		return
	}
	if err := s.salesClaimer.EnsureAutoClaim(ctx, userID); err != nil {
		s.warn("自动归属销售失败", userID, err)
	}
}

// resolveDefaultGroupID 解析默认用户组；未配置、解析失败或为 0 都返回 nil（保持未分组，不阻断注册）。
func (s *authService) resolveDefaultGroupID(ctx context.Context) *uint64 {
	if s.defaultGroup == nil {
		return nil
	}
	id, err := s.defaultGroup.DefaultGroupID(ctx)
	if err != nil {
		s.warn("解析默认用户组失败", 0, err)
		return nil
	}
	if id == 0 {
		return nil
	}
	return &id
}

// setupInvite 为新注册用户生成自有邀请码，并在携带邀请码时绑定单级邀请关系。
func (s *authService) setupInvite(ctx context.Context, userID uint64, inviteCode string) {
	if s.inviteBinder == nil {
		return
	}
	if _, err := s.inviteBinder.EnsureInviteCode(ctx, userID); err != nil {
		s.warn("生成邀请码失败", userID, err)
	}
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return
	}
	inviterID, err := s.inviteBinder.ResolveInviter(ctx, inviteCode)
	if err != nil {
		s.warn("解析邀请码失败", userID, err)
		return
	}
	if inviterID == 0 {
		s.warn("邀请码无效", userID, errors.New("invite code not found"))
		return
	}
	if inviterID == userID {
		s.warn("不接受自邀", userID, errors.New("self invitation"))
		return
	}
	if err := s.inviteBinder.BindInviter(ctx, userID, inviterID); err != nil {
		s.warn("绑定邀请关系失败", userID, err)
	}
}

func (s *authService) warn(msg string, userID uint64, err error) {
	if s.logger == nil {
		return
	}
	s.logger.Warn(msg, zap.Uint64("user_id", userID), zap.Error(err))
}

// UserInfo 根据用户 ID 查询用户信息。
func (s *authService) UserInfo(ctx context.Context, userID uint64) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}
	info := s.toUserInfo(ctx, user)
	return &info, nil
}

// buildLoginResponse 组装登录响应，签发带归属信息的普通用户 JWT（P4-03）。
func (s *authService) buildLoginResponse(ctx context.Context, user *model.User) *dto.LoginResponse {
	ownerID := uint64(0)
	if user.OwnerUserID != nil {
		ownerID = *user.OwnerUserID
	}
	token, err := s.jwtIssuer.GenerateUserFull(user.Username, user.ID, user.Tier, ownerID, user.IsSubAccount)
	if err != nil {
		return nil
	}
	userInfo := s.toUserInfo(ctx, user)
	return &dto.LoginResponse{
		Token: token,
		User:  userInfo,
	}
}

// toUserInfo 将用户模型转换为响应 DTO，name 优先取 real_name，为空时回退为 username。
func (s *authService) toUserInfo(ctx context.Context, user *model.User) dto.UserInfo {
	name := user.RealName
	if name == "" {
		name = user.Username
	}
	ownerID := uint64(0)
	if user.OwnerUserID != nil {
		ownerID = *user.OwnerUserID
	}
	info := dto.UserInfo{
		ID:           user.ID,
		Username:     user.Username,
		Name:         name,
		Email:        user.Email,
		Phone:        user.Phone,
		Avatar:       user.Avatar,
		Role:         "user",
		Tier:         user.Tier,
		Status:       user.Status,
		IsSubAccount: user.IsSubAccount,
		OwnerUserID:  ownerID,
		Remark:       user.SubAccountRemark,
	}
	if user.IsSubAccount && ownerID > 0 {
		if owner, err := s.repo.FindByID(ctx, ownerID); err == nil && owner != nil {
			info.OwnerName = owner.Username
		}
	}
	if codes, err := s.repo.PermissionsOf(ctx, user.ID); err == nil {
		if codes == nil {
			codes = []string{}
		}
		info.Permissions = codes
	} else {
		info.Permissions = []string{}
	}
	return info
}

// updateLoginProfile 解析 IP 归属地并更新用户的登录档案。
func (s *authService) updateLoginProfile(ctx context.Context, userID uint64, ip string) error {
	ipRegion := ""
	if s.ipRegionResolver != nil && ip != "" {
		ipRegion = s.ipRegionResolver.Resolve(ctx, ip)
	}
	return s.repo.UpdateLoginProfile(ctx, userID, ip, ipRegion, time.Now())
}

// UpdateProfile 更新当前用户基本资料：
//  1. 按字段变化动态要求二次验证（改手机 → phone_bind，改邮箱 → email_bind，doc91 §6.3）
//  2. 若修改邮箱，校验新邮箱未被其他用户占用
//  3. 写入显示名、邮箱、手机、头像
//  4. 重新读取并返回最新用户信息
func (s *authService) UpdateProfile(ctx context.Context, userID uint64, req dto.UpdateProfileRequest) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}

	// 动态判定：该路由同时改姓名/头像/手机/邮箱，无法用固定 scene 的中间件。
	// 策略要求（默认不要求）时，对应场景必须携带一次性票据。
	if err := s.verifyBindingChange(ctx, user, req); err != nil {
		return nil, err
	}

	// 邮箱变更时校验唯一性（排除自身）
	if req.Email != "" && req.Email != user.Email {
		if _, err := s.repo.FindByEmail(ctx, req.Email); err == nil {
			return nil, errors.New("邮箱已被其他账号使用")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	name := req.Name
	if name == "" {
		name = user.RealName
	}
	email := req.Email
	if email == "" {
		email = user.Email
	}

	if err := s.repo.UpdateProfile(ctx, userID, name, email, req.Phone, req.Avatar); err != nil {
		return nil, err
	}

	// 变更成功即认为新联系方式已验证（用户已完成该场景的 OTP）。
	if s.sec != nil {
		if strings.TrimSpace(req.Phone) != "" && strings.TrimSpace(req.Phone) != user.Phone {
			_ = s.repo.MarkVerified(ctx, userID, security.ChannelSMS, time.Now())
		}
		if req.Email != "" && req.Email != user.Email {
			_ = s.repo.MarkVerified(ctx, userID, security.ChannelEmail, time.Now())
		}
	}

	updated, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	info := s.toUserInfo(ctx, updated)
	return &info, nil
}

// verifyBindingChange 按「哪个字段变了」要求对应的二次验证票据。
//
// 策略不要求 OTP 时直接通过，因此默认（captcha_enabled=false）此函数零影响。
func (s *authService) verifyBindingChange(ctx context.Context, user *model.User, req dto.UpdateProfileRequest) error {
	if s.sec == nil {
		return nil
	}
	subject := security.Subject{ID: user.ID}
	phoneChanged := strings.TrimSpace(req.Phone) != "" && strings.TrimSpace(req.Phone) != user.Phone
	emailChanged := strings.TrimSpace(req.Email) != "" && req.Email != user.Email
	if !phoneChanged && !emailChanged {
		return nil
	}
	scenes := make([]string, 0, 2)
	if phoneChanged {
		scenes = append(scenes, security.ScenePhoneBind)
	}
	if emailChanged {
		scenes = append(scenes, security.SceneEmailBind)
	}
	ticket := strings.TrimSpace(req.VerifyTicket)
	for _, scene := range scenes {
		if required, _ := s.sec.EffectiveOTP(ctx, scene, subject); !required {
			continue
		}
		if !s.sec.ConsumeVerifyTicket(ctx, scene, subject, ticket) {
			return security.ErrNeedVerification
		}
	}
	return nil
}

// ChangePassword 修改当前用户密码：
//  1. 校验旧密码是否正确
//  2. 对新密码进行 bcrypt 加密
//  3. 更新密码哈希
func (s *authService) ChangePassword(ctx context.Context, userID uint64, req dto.ChangePasswordRequest) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}

	// 校验旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return errors.New("原密码错误")
	}

	// 平台密码策略（6 个 password_* 开关，doc91 §9.1）。
	if s.sec != nil {
		if err := s.sec.ValidatePassword(ctx, req.NewPassword); err != nil {
			return err
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, userID, string(hash))
}
