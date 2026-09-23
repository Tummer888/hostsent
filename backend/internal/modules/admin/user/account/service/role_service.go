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
	// List 按作用域列出角色；scope 为空等价于 admin（只出后台角色）。
	List(ctx context.Context, scope string) ([]dto.RoleInfo, error)
	Create(ctx context.Context, req dto.RoleCreateRequest) (*dto.RoleInfo, error)
	FindByID(ctx context.Context, id uint64) (*dto.RoleInfo, error)
	Update(ctx context.Context, id uint64, req dto.RoleUpdateRequest) (*dto.RoleInfo, error)
	Delete(ctx context.Context, id uint64) error
	Permissions(ctx context.Context, id uint64) ([]uint64, error)
	AssignPermissions(ctx context.Context, id uint64, permissionIDs []uint64) error
	// ValidateScope 校验一组角色 ID 是否全部属于指定作用域。
	// 跨域（如把后台角色分配给客户账号）返回 ErrRoleScopeMismatch。
	ValidateScope(ctx context.Context, roleIDs []uint64, want string) error
}

type roleService struct {
	repo  repository.RoleRepository
	cache middleware.PermissionCache
}

func NewRoleService(repo repository.RoleRepository, cache middleware.PermissionCache) RoleService {
	return &roleService{repo: repo, cache: cache}
}

// List 按作用域列出角色。
//
// scope 为空按 admin 处理：后台角色列表是历史行为，改造前只按 scope=user 做内存过滤，
// 一旦该列在存量库上取到错值（实测 roles.id=7 曾是 scope='admin'）过滤就整体失效，
// 客户角色会被当成后台角色返给员工建号的角色下拉。现在过滤下沉到 SQL，
// 并额外排除空 scope 以外的所有非 admin 值。
func (s *roleService) List(ctx context.Context, scope string) ([]dto.RoleInfo, error) {
	if scope == "" {
		scope = model.RoleScopeAdmin
	}
	roles, err := s.repo.List(ctx, scope)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.RoleInfo, 0, len(roles))
	for _, role := range roles {
		resp = append(resp, toRoleInfo(role))
	}
	return resp, nil
}

// ErrRoleScopeMismatch 角色作用域不匹配：角色不属于目标账号域。
var ErrRoleScopeMismatch = errors.New("角色不属于该账号域，不能跨域分配")

// ValidateScope 校验角色 ID 集合全部落在 want 作用域内。
//
// 存在性也在这里兜住：传入不存在的角色 ID 视为不合法，避免调用方以为
// 「没报错就是绑定成功了」。空集合直接返回 nil —— 是否允许清空由调用方决定。
func (s *roleService) ValidateScope(ctx context.Context, roleIDs []uint64, want string) error {
	if len(roleIDs) == 0 {
		return nil
	}
	if want == "" {
		want = model.RoleScopeAdmin
	}
	roles, err := s.repo.FindByIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	byID := make(map[uint64]model.Role, len(roles))
	for _, role := range roles {
		byID[role.ID] = role
	}
	for _, id := range roleIDs {
		role, ok := byID[id]
		if !ok {
			return errors.New("角色不存在")
		}
		scope := role.Scope
		if scope == "" {
			scope = model.RoleScopeAdmin
		}
		if scope != want {
			return ErrRoleScopeMismatch
		}
	}
	return nil
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

// ensureAdminRole 拒绝在后台角色管理里操作客户角色（scope=user）。
//
// 客户角色是客户侧权限树的占位（doc81 §4.1 已决定客户侧不用角色，改用
// 「用户组 + 等级」），它不该出现在后台角色列表，更不该被改名/改标识/删权限 ——
// 一旦被改，所有历史 user_roles 绑定的语义就跟着漂移。
func ensureAdminRole(role *model.Role) error {
	if role.Scope == model.RoleScopeUser {
		return errors.New("客户角色不在后台角色管理中维护")
	}
	return nil
}

func (s *roleService) Update(ctx context.Context, id uint64, req dto.RoleUpdateRequest) (*dto.RoleInfo, error) {
	role, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureAdminRole(role); err != nil {
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
	if err := ensureAdminRole(role); err != nil {
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
	if err := ensureAdminRole(role); err != nil {
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
