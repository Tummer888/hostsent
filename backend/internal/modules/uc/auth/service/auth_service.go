package service

import (
	"context"
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
)

// AuthService 用户中心认证服务接口。
// 提供用户自助场景下的注册、登录、信息查询等能力。
type AuthService interface {
	// Login 用户登录：校验用户名密码 → 检查状态 → 更新登录档案 → 签发 JWT。
	Login(ctx context.Context, req dto.LoginRequest, ip string) (*dto.LoginResponse, error)
	// Register 用户注册：校验唯一性 → 密码加密 → 创建用户 → 返回用户 ID。
	Register(ctx context.Context, req dto.RegisterRequest) (uint64, error)
	// UserInfo 获取当前登录用户信息。
	UserInfo(ctx context.Context, userID uint64) (*dto.UserInfo, error)
	// UpdateProfile 更新当前用户基本资料，返回更新后的用户信息。
	UpdateProfile(ctx context.Context, userID uint64, req dto.UpdateProfileRequest) (*dto.UserInfo, error)
	// ChangePassword 修改密码：校验旧密码后写入新密码哈希。
	ChangePassword(ctx context.Context, userID uint64, req dto.ChangePasswordRequest) error
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
}

// NewAuthService 创建用户中心认证服务实例。
func NewAuthService(repo repository.UserRepository, jwtIssuer *appauth.JWTIssuer, ipRegionResolver netutil.IPRegionResolver, logger *zap.Logger) AuthService {
	return &authService{repo: repo, jwtIssuer: jwtIssuer, ipRegionResolver: ipRegionResolver, logger: logger}
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

// Login 执行用户登录流程：
//  1. 按用户名查找用户，不存在返回 "用户名或密码错误"
//  2. 检查用户状态是否为 "active"
//  3. 校验 bcrypt 密码哈希
//  4. 更新登录档案（IP、归属地、时间）
//  5. 签发普通用户 JWT（UserClaims）
//  6. 组装登录响应（对齐前端字段）
func (s *authService) Login(ctx context.Context, req dto.LoginRequest, ip string) (*dto.LoginResponse, error) {
	user, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误") // 不暴露用户是否存在
		}
		return nil, err
	}

	if user.Status != "active" {
		return nil, errors.New("用户已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	// 异步更新登录档案（主流程不阻塞）
	if err := s.updateLoginProfile(ctx, user.ID, ip); err != nil {
		return nil, err
	}

	// 重新读取以获取最新登录档案
	user, err = s.repo.FindByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return s.buildLoginResponse(ctx, user), nil
}

// Register 执行用户注册流程：
//  1. 检查用户名是否已存在
//  2. 检查邮箱是否已存在
//  3. 对密码进行 bcrypt 加密
//  4. 创建用户（默认 tier=free, status=active）
//  5. 返回新用户 ID
func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (uint64, error) {
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

	// 推广邀请：失败一律不阻断注册（邀请码只影响返现归属，不影响账号可用性）。
	s.setupInvite(ctx, user.ID, req.InviteCode)
	// 销售归属自动认领：新客户无归属时按负载补给在职销售；失败同样不阻断注册。
	s.setupSalesOwner(ctx, user.ID)
	return user.ID, nil
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
//  1. 若修改邮箱，校验新邮箱未被其他用户占用
//  2. 写入显示名、邮箱、手机、头像
//  3. 重新读取并返回最新用户信息
func (s *authService) UpdateProfile(ctx context.Context, userID uint64, req dto.UpdateProfileRequest) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
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

	updated, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	info := s.toUserInfo(ctx, updated)
	return &info, nil
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

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, userID, string(hash))
}
