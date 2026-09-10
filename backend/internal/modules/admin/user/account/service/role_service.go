package service

import (
	"context"
	"errors"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
	"hostsent/backend/internal/modules/admin/user/account/repository"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/middleware"
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
	repo  repository.RoleRepository
	cache middleware.PermissionCache
}

func NewRoleService(repo repository.RoleRepository, cache middleware.PermissionCache) RoleService {
	return &roleService{repo: repo, cache: cache}
}

// List 只返回后台角色（scope=admin）；客户角色不进后台权限树（81 §4.1）。
func (s *roleService) List(ctx context.Context) ([]dto.RoleInfo, error) {
	roles, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.RoleInfo, 0, len(roles))
	for _, role := range roles {
		if role.Scope == model.RoleScopeUser {
			continue
		}
		resp = append(resp, toRoleInfo(role))
	}
	return resp, nil
}

func (s *roleService) Create(ctx context.Context, req dto.RoleCreateRequest) (*dto.RoleInfo, error) {
	role := &model.Role{Name: req.Name, Code: req.Code, Scope: model.RoleScopeAdmin, Status: req.Status}
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
	// P1-09：超管角色不可改 code（防止把自己锁死）
	if role.Code == appauth.SuperAdminRoleCode && req.Code != role.Code {
		return nil, errors.New("超级管理员角色不允许修改标识")
	}
	role.Name = req.Name
	role.Code = req.Code
	role.Status = req.Status
	if err := s.repo.Update(ctx, role); err != nil {
		return nil, err
	}
	s.invalidateRole(id)
	return ptrRoleInfo(*role), nil
}

func (s *roleService) Delete(ctx context.Context, id uint64) error {
	role, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	// P1-09：超管角色不可删除
	if role.Code == appauth.SuperAdminRoleCode {
		return errors.New("超级管理员角色不允许删除")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateRole(id)
	return nil
}

func (s *roleService) Permissions(ctx context.Context, id uint64) ([]uint64, error) {
	return s.repo.GetPermissionIDs(ctx, id)
}

// AssignPermissions 覆盖式分配权限；超管角色拥有通配权限，禁止手工分配（P1-09）。
func (s *roleService) AssignPermissions(ctx context.Context, id uint64, permissionIDs []uint64) error {
	role, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if role.Code == appauth.SuperAdminRoleCode {
		return errors.New("超级管理员拥有全部权限，无需分配")
	}
	if err := s.repo.UpsertPermissions(ctx, id, permissionIDs); err != nil {
		return err
	}
	// 权限变更立即生效：清理该角色下所有管理员的缓存
	s.invalidateRole(id)
	return nil
}

func (s *roleService) invalidateRole(roleID uint64) {
	if s.cache != nil {
		s.cache.InvalidateRole(roleID)
	}
}

func toRoleInfo(role model.Role) dto.RoleInfo {
	scope := role.Scope
	if scope == "" {
		scope = model.RoleScopeAdmin
	}
	return dto.RoleInfo{
		ID:          role.ID,
		Name:        role.Name,
		Code:        role.Code,
		Description: role.Description,
		Scope:       scope,
		Builtin:     role.Code == appauth.SuperAdminRoleCode,
		Status:      role.Status,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}
}

func ptrRoleInfo(role model.Role) *dto.RoleInfo {
	info := toRoleInfo(role)
	return &info
}
