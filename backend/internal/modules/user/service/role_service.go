package service

import (
	"context"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
	"hostsent/backend/internal/modules/user/repository"
)

type RoleService interface {
	List(ctx context.Context) ([]dto.RoleInfo, error)
	Create(ctx context.Context, req dto.RoleCreateRequest) (*dto.RoleInfo, error)
	FindByID(ctx context.Context, id uint64) (*dto.RoleInfo, error)
	Update(ctx context.Context, id uint64, req dto.RoleUpdateRequest) (*dto.RoleInfo, error)
	Delete(ctx context.Context, id uint64) error
	Permissions(ctx context.Context, id uint64) ([]uint64, error)
	AssignPermissions(ctx context.Context, id uint64, permissionIDs []uint64) error
}

type roleService struct {
	repo repository.RoleRepository
}

func NewRoleService(repo repository.RoleRepository) RoleService {
	return &roleService{repo: repo}
}

func (s *roleService) List(ctx context.Context) ([]dto.RoleInfo, error) {
	roles, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.RoleInfo, 0, len(roles))
	for _, role := range roles {
		resp = append(resp, toRoleInfo(role))
	}
	return resp, nil
}

func (s *roleService) Create(ctx context.Context, req dto.RoleCreateRequest) (*dto.RoleInfo, error) {
	role := &model.Role{Name: req.Name, Code: req.Code, Status: req.Status}
	if role.Status == "" {
		role.Status = "active"
	}
	if err := s.repo.Create(ctx, role); err != nil {
		return nil, err
	}
	return ptrRoleInfo(*role), nil
}

func (s *roleService) FindByID(ctx context.Context, id uint64) (*dto.RoleInfo, error) {
	role, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return ptrRoleInfo(*role), nil
}

func (s *roleService) Update(ctx context.Context, id uint64, req dto.RoleUpdateRequest) (*dto.RoleInfo, error) {
	role, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	role.Name = req.Name
	role.Code = req.Code
	role.Status = req.Status
	if err := s.repo.Update(ctx, role); err != nil {
		return nil, err
	}
	return ptrRoleInfo(*role), nil
}

func (s *roleService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *roleService) Permissions(ctx context.Context, id uint64) ([]uint64, error) {
	return s.repo.GetPermissionIDs(ctx, id)
}

func (s *roleService) AssignPermissions(ctx context.Context, id uint64, permissionIDs []uint64) error {
	return s.repo.UpsertPermissions(ctx, id, permissionIDs)
}

func toRoleInfo(role model.Role) dto.RoleInfo {
	return dto.RoleInfo{ID: role.ID, Name: role.Name, Code: role.Code, Status: role.Status}
}

func ptrRoleInfo(role model.Role) *dto.RoleInfo {
	info := toRoleInfo(role)
	return &info
}
