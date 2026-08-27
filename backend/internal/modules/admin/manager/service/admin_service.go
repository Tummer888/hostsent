package service

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/model"
	"hostsent/backend/internal/modules/admin/manager/repository"
	appauth "hostsent/backend/internal/pkg/auth"
)

type AdminService interface {
	Login(ctx context.Context, req dto.AdminLoginRequest, ip string) (*dto.AdminLoginResponse, error)
	Me(ctx context.Context, adminID uint64) (*dto.AdminInfo, error)
	List(ctx context.Context, query dto.AdminListQuery) (*dto.AdminListResponse, error)
	Create(ctx context.Context, req dto.AdminCreateRequest) (*dto.AdminInfo, error)
	FindByID(ctx context.Context, id uint64) (*dto.AdminInfo, error)
	Update(ctx context.Context, id uint64, req dto.AdminUpdateRequest) (*dto.AdminInfo, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	ResetPassword(ctx context.Context, id uint64, password string) error
	Delete(ctx context.Context, id uint64) error
}

type adminService struct {
	repo      repository.AdminRepository
	jwtIssuer *appauth.JWTIssuer
}

func NewAdminService(repo repository.AdminRepository, jwtIssuer *appauth.JWTIssuer) AdminService {
	return &adminService{repo: repo, jwtIssuer: jwtIssuer}
}

func (s *adminService) Login(ctx context.Context, req dto.AdminLoginRequest, ip string) (*dto.AdminLoginResponse, error) {
	admin, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}

	if admin.Status != "active" {
		return nil, errors.New("管理员已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	if err := s.repo.UpdateLoginProfile(ctx, admin.ID, ip, time.Now()); err != nil {
		return nil, err
	}

	token, err := s.jwtIssuer.GenerateAdmin(admin.Username, admin.ID, admin.Role)
	if err != nil {
		return nil, err
	}

	return &dto.AdminLoginResponse{
		Token:       token,
		UserInfo:    *toAdminInfo(*admin),
		Permissions: []string{"admin:*"},
		Menus:       []string{"dashboard", "users", "roles"},
	}, nil
}

func (s *adminService) Me(ctx context.Context, adminID uint64) (*dto.AdminInfo, error) {
	admin, err := s.repo.FindByID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	return toAdminInfo(*admin), nil
}

func (s *adminService) List(ctx context.Context, query dto.AdminListQuery) (*dto.AdminListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	admins, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	items := make([]dto.AdminInfo, 0, len(admins))
	for _, admin := range admins {
		items = append(items, *toAdminInfo(admin))
	}
	return &dto.AdminListResponse{
		Items: items,
		Meta:  dto.AdminListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *adminService) Create(ctx context.Context, req dto.AdminCreateRequest) (*dto.AdminInfo, error) {
	if req.Role == "" {
		req.Role = "admin"
	}
	if req.Status == "" {
		req.Status = "active"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	admin := &model.Admin{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         req.Role,
		Department:   req.Department,
		Status:       req.Status,
	}
	if err := s.repo.Create(ctx, admin); err != nil {
		return nil, err
	}
	return toAdminInfo(*admin), nil
}

func (s *adminService) FindByID(ctx context.Context, id uint64) (*dto.AdminInfo, error) {
	admin, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toAdminInfo(*admin), nil
}

func (s *adminService) Update(ctx context.Context, id uint64, req dto.AdminUpdateRequest) (*dto.AdminInfo, error) {
	admin, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	admin.Email = req.Email
	admin.Role = req.Role
	admin.Department = req.Department
	admin.Status = req.Status
	if err := s.repo.Update(ctx, admin); err != nil {
		return nil, err
	}
	return toAdminInfo(*admin), nil
}

func (s *adminService) UpdateStatus(ctx context.Context, id uint64, status string) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *adminService) ResetPassword(ctx context.Context, id uint64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, id, string(hash))
}

func (s *adminService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func toAdminInfo(admin model.Admin) *dto.AdminInfo {
	return &dto.AdminInfo{
		ID:          admin.ID,
		Username:    admin.Username,
		Email:       admin.Email,
		Avatar:      admin.Avatar,
		Role:        admin.Role,
		Department:  admin.Department,
		Status:      admin.Status,
		LastLoginIP: admin.LastLoginIP,
		LastLoginAt: admin.LastLoginAt,
		CreatedAt:   admin.CreatedAt,
	}
}
