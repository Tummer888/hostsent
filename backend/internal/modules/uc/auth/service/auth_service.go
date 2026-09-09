package service

import (
	"context"
	"errors"
	"time"

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
}

type authService struct {
	repo             repository.UserRepository
	jwtIssuer        *appauth.JWTIssuer
	ipRegionResolver netutil.IPRegionResolver
}

// NewAuthService 创建用户中心认证服务实例。
func NewAuthService(repo repository.UserRepository, jwtIssuer *appauth.JWTIssuer, ipRegionResolver netutil.IPRegionResolver) AuthService {
	return &authService{repo: repo, jwtIssuer: jwtIssuer, ipRegionResolver: ipRegionResolver}
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

	return s.buildLoginResponse(user), nil
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
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return 0, err
	}
	return user.ID, nil
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
	info := s.toUserInfo(user)
	return &info, nil
}

// buildLoginResponse 组装登录响应，使用 GenerateUser 签发普通用户 JWT。
func (s *authService) buildLoginResponse(user *model.User) *dto.LoginResponse {
	token, err := s.jwtIssuer.GenerateUser(user.Username, user.ID, user.Tier)
	if err != nil {
		return nil
	}
	userInfo := s.toUserInfo(user)
	return &dto.LoginResponse{
		Token: token,
		User:  userInfo,
	}
}

// toUserInfo 将用户模型转换为响应 DTO，name 优先取 real_name，为空时回退为 username。
func (s *authService) toUserInfo(user *model.User) dto.UserInfo {
	name := user.RealName
	if name == "" {
		name = user.Username
	}
	return dto.UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Name:     name,
		Email:    user.Email,
		Phone:    user.Phone,
		Avatar:   user.Avatar,
		Role:     "user",
		Tier:     user.Tier,
		Status:   user.Status,
	}
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
	info := s.toUserInfo(updated)
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
