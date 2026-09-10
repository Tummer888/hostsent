package service

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
	"hostsent/backend/internal/modules/admin/user/account/repository"
	pkgauth "hostsent/backend/internal/pkg/auth"
)

type UserService interface {
	List(ctx context.Context, query dto.UserListQuery) (*dto.UserListResponse, error)
	Create(ctx context.Context, req dto.UserCreateRequest) (*dto.UserInfo, error)
	FindByID(ctx context.Context, id uint64) (*dto.UserInfo, error)
	Update(ctx context.Context, id uint64, req dto.UserUpdateRequest) (*dto.UserInfo, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	ResetPassword(ctx context.Context, id uint64, password string) error
	AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
	Delete(ctx context.Context, id uint64) error
	GetStats(ctx context.Context) (*dto.UserStatsResponse, error)
	GetRegionStats(ctx context.Context) (*dto.RegionStatsResponse, error)
	// Impersonate 代登录：按用户 ID 签发用户端 token（仅 active 用户）
	Impersonate(ctx context.Context, id uint64) (*dto.ImpersonateResponse, error)
	// Recharge 用户充值（人工调账）
	Recharge(ctx context.Context, id uint64, amount float64, remark string, operatorID uint64) error
	// ListMembers 查询某主账号名下的成员（子账号）及权限（P4-10）
	ListMembers(ctx context.Context, ownerID uint64) (*dto.SubAccountMemberListResponse, error)
	// SetDefaultGroupProvider 注入默认用户组解析能力（可选，装配层调用）。
	SetDefaultGroupProvider(provider DefaultGroupProvider)
}

// Recharger 充值能力适配器（由装配层注入，内部调用财务钱包调账）。
type Recharger func(ctx context.Context, userID uint64, amount float64, remark string, operatorID uint64) error

// DefaultGroupProvider 提供默认用户组 ID（由用户组服务实现，装配层注入）；
// 返回 0 表示未配置默认组，此时新用户保持未分组。
type DefaultGroupProvider interface {
	DefaultGroupID(ctx context.Context) (uint64, error)
}

type userService struct {
	repo         repository.UserRepository
	jwtIssuer    *pkgauth.JWTIssuer
	recharge     Recharger
	defaultGroup DefaultGroupProvider
}

func NewUserService(repo repository.UserRepository, jwtIssuer *pkgauth.JWTIssuer, recharge Recharger) UserService {
	return &userService{repo: repo, jwtIssuer: jwtIssuer, recharge: recharge}
}

// SetDefaultGroupProvider 注入默认用户组解析能力（可选，装配层调用）。
func (s *userService) SetDefaultGroupProvider(provider DefaultGroupProvider) {
	s.defaultGroup = provider
}

// resolveDefaultGroupID 解析默认用户组；未配置或解析失败都返回 nil（保持未分组，不阻断建号）。
func (s *userService) resolveDefaultGroupID(ctx context.Context) *uint64 {
	if s.defaultGroup == nil {
		return nil
	}
	id, err := s.defaultGroup.DefaultGroupID(ctx)
	if err != nil || id == 0 {
		return nil
	}
	return &id
}

func (s *userService) List(ctx context.Context, query dto.UserListQuery) (*dto.UserListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	users, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	items := make([]dto.UserInfo, 0, len(users))
	for _, user := range users {
		items = append(items, toUserInfo(user))
	}
	return &dto.UserListResponse{
		Items: items,
		Meta: dto.UserListMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func (s *userService) Create(ctx context.Context, req dto.UserCreateRequest) (*dto.UserInfo, error) {
	if req.Status == "" {
		req.Status = "active"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{ID: req.ID, Username: req.Username, Email: req.Email, Phone: req.Phone, PasswordHash: string(hash), Status: req.Status, UserGroupID: req.UserGroupID}
	// 未指定分组时归入默认组（未配置默认组则保持未分组）。
	if user.UserGroupID == nil {
		user.UserGroupID = s.resolveDefaultGroupID(ctx)
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	if len(req.RoleIDs) > 0 {
		if err := s.repo.SetRoles(ctx, user.ID, req.RoleIDs); err != nil {
			return nil, err
		}
	}
	fresh, err := s.repo.FindByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return ptrUserInfo(*fresh), nil
}

func (s *userService) FindByID(ctx context.Context, id uint64) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return ptrUserInfo(*user), nil
}

func (s *userService) Update(ctx context.Context, id uint64, req dto.UserUpdateRequest) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user.Username = req.Username
	user.Email = req.Email
	user.Phone = req.Phone
	user.Status = req.Status
	// nil 表示不修改分组；0 表示移出分组；其余为组 ID。
	if req.UserGroupID != nil {
		if *req.UserGroupID == 0 {
			user.UserGroupID = nil
		} else {
			user.UserGroupID = req.UserGroupID
		}
	}
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	fresh, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return ptrUserInfo(*fresh), nil
}

func (s *userService) UpdateStatus(ctx context.Context, id uint64, status string) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *userService) ResetPassword(ctx context.Context, id uint64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, id, string(hash))
}

func (s *userService) AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	return s.repo.SetRoles(ctx, userID, roleIDs)
}

func (s *userService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *userService) GetStats(ctx context.Context) (*dto.UserStatsResponse, error) {
	stats, err := s.repo.Stats(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.UserStatsResponse{
		Total:           stats.Total,
		TodayNew:        stats.TodayNew,
		Active:          stats.Active,
		Disabled:        stats.Disabled,
		PendingRealName: stats.PendingRealName,
		PendingReview:   stats.PendingReview,
		TotalBalance:    stats.TotalBalance,
		PurchasedCount:  stats.PurchasedCount,
	}, nil
}

func (s *userService) GetRegionStats(ctx context.Context) (*dto.RegionStatsResponse, error) {
	rows, err := s.repo.RegionStats(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dto.RegionStatItem, 0, len(rows))
	var total int64
	for _, r := range rows {
		items = append(items, dto.RegionStatItem{Region: r.Region, Count: r.Count})
		total += r.Count
	}
	return &dto.RegionStatsResponse{Items: items, Total: total}, nil
}

func toUserInfo(user model.User) dto.UserInfo {
	return dto.UserInfo{
		ID:                 user.ID,
		Username:           user.Username,
		RealName:           user.RealName,
		Role:               user.Role,
		Roles:              user.Roles,
		Email:              user.Email,
		Phone:              user.Phone,
		UserGroupID:        user.UserGroupID,
		UserGroupName:      user.UserGroupName,
		UserLevelID:        user.UserLevelID,
		UserLevelName:      user.UserLevelName,
		UserLevelCode:      user.UserLevelCode,
		Region:             user.Region,
		LastLoginIP:        user.LastLoginIP,
		LastLoginIPRegion:  user.LastLoginIPRegion,
		OAuthProvider:      user.OAuthProvider,
		Balance:            user.Balance,
		TotalConsumeAmount: user.TotalConsumeAmount,
		Status:             user.Status,
		IsSubAccount:       user.IsSubAccount,
		OwnerUserID:        user.OwnerUserID,
		OwnerName:          user.OwnerName,
		SubAccountRemark:   user.SubAccountRemark,
		CreatedAt:          user.CreatedAt,
		LastLoginAt:        user.LastLoginAt,
	}
}

// ListMembers 查询某主账号名下的成员（子账号）及各自权限（P4-10，管理端成员 Tab）。
func (s *userService) ListMembers(ctx context.Context, ownerID uint64) (*dto.SubAccountMemberListResponse, error) {
	members, err := s.repo.ListSubAccounts(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.ID)
	}
	perms, err := s.repo.PermissionsByUserIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]dto.SubAccountMemberInfo, 0, len(members))
	for _, m := range members {
		codes := perms[m.ID]
		if codes == nil {
			codes = []string{}
		}
		items = append(items, dto.SubAccountMemberInfo{
			ID:          m.ID,
			Username:    m.Username,
			Name:        m.RealName,
			Email:       m.Email,
			Phone:       m.Phone,
			Remark:      m.SubAccountRemark,
			Status:      m.Status,
			Permissions: codes,
			CreatedAt:   m.CreatedAt,
			LastLoginAt: m.LastLoginAt,
		})
	}
	return &dto.SubAccountMemberListResponse{Items: items, Total: int64(len(items))}, nil
}

func ptrUserInfo(user model.User) *dto.UserInfo {
	info := toUserInfo(user)
	return &info
}

// Impersonate 代登录：按用户 ID 签发用户端 JWT。
func (s *userService) Impersonate(ctx context.Context, id uint64) (*dto.ImpersonateResponse, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, errors.New("仅可代登录正常状态用户")
	}
	if s.jwtIssuer == nil {
		return nil, errors.New("代登录能力未配置")
	}
	info, err := s.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 代登录同样带上归属信息（P4-03），子账号被代登录时权限语义与本人登录一致。
	ownerID := uint64(0)
	if user.OwnerUserID != nil {
		ownerID = *user.OwnerUserID
	}
	token, err := s.jwtIssuer.GenerateUserFull(user.Username, user.ID, "", ownerID, user.IsSubAccount)
	if err != nil {
		return nil, err
	}
	return &dto.ImpersonateResponse{Token: token, UserInfo: *info}, nil
}

// Recharge 用户充值（人工调账），委托给注入的 Recharger。
func (s *userService) Recharge(ctx context.Context, id uint64, amount float64, remark string, operatorID uint64) error {
	if s.recharge == nil {
		return errors.New("充值能力未配置")
	}
	return s.recharge(ctx, id, amount, remark, operatorID)
}
