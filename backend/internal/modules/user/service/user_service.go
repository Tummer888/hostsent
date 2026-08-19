package service

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
	"hostsent/backend/internal/modules/user/repository"
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
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
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
	user := &model.User{Username: req.Username, Email: req.Email, Phone: req.Phone, PasswordHash: string(hash), Status: req.Status}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	if len(req.RoleIDs) > 0 {
		if err := s.repo.UpsertRoles(ctx, user.ID, req.RoleIDs); err != nil {
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
	return s.repo.ResetPassword(ctx, id, string(hash))
}

func (s *userService) AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	return s.repo.UpsertRoles(ctx, userID, roleIDs)
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
		ID:          user.ID,
		Username:    user.Username,
		RealName:    user.RealName,
		Role:        user.Role,
		Roles:       user.Roles,
		Email:       user.Email,
		Phone:       user.Phone,
		Region:      user.Region,
		Balance:     user.Balance,
		Status:      user.Status,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}
}

func ptrUserInfo(user model.User) *dto.UserInfo {
	info := toUserInfo(user)
	return &info
}
