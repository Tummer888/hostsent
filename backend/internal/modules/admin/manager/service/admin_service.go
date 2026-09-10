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
	"hostsent/backend/internal/pkg/middleware"
)

type AdminService interface {
	Login(ctx context.Context, req dto.AdminLoginRequest, ip string) (*dto.AdminLoginResponse, error)
	Me(ctx context.Context, adminID uint64) (*dto.AdminInfo, error)
	ChangePassword(ctx context.Context, adminID uint64, req dto.AdminChangePasswordRequest) error
	List(ctx context.Context, query dto.AdminListQuery) (*dto.AdminListResponse, error)
	Create(ctx context.Context, req dto.AdminCreateRequest) (*dto.AdminInfo, error)
	FindByID(ctx context.Context, id uint64) (*dto.AdminInfo, error)
	Update(ctx context.Context, id uint64, req dto.AdminUpdateRequest) (*dto.AdminInfo, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	ResetPassword(ctx context.Context, id uint64, password string) error
	Delete(ctx context.Context, id uint64) error
	SetRoles(ctx context.Context, id uint64, roleIDs []uint64) error
	// ListAuditLogs 管理端操作审计查询（P2-06）。
	ListAuditLogs(ctx context.Context, query dto.AdminAuditLogQuery) (*dto.AdminAuditLogResponse, error)
}

type adminService struct {
	repo      repository.AdminRepository
	rbac      repository.RBACRepository
	audit     repository.AdminAuditRepository
	cache     middleware.PermissionCache
	jwtIssuer *appauth.JWTIssuer
}

func NewAdminService(repo repository.AdminRepository, rbac repository.RBACRepository, audit repository.AdminAuditRepository, cache middleware.PermissionCache, jwtIssuer *appauth.JWTIssuer) AdminService {
	return &adminService{repo: repo, rbac: rbac, audit: audit, cache: cache, jwtIssuer: jwtIssuer}
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

	grant, err := middleware.LoadAdminGrant(ctx, s.cache, s.rbac, admin.ID)
	if err != nil {
		return nil, err
	}
	perms := grant.Perms.Slice()
	primaryRole := admin.Role
	if len(grant.Roles) > 0 {
		primaryRole = grant.Roles[0]
	}

	token, err := s.jwtIssuer.GenerateAdmin(admin.Username, admin.ID, primaryRole)
	if err != nil {
		return nil, err
	}

	info := toAdminInfo(*admin)
	info.Role = primaryRole
	info.Roles = grant.Roles
	info.Permissions = perms

	return &dto.AdminLoginResponse{
		Token:              token,
		UserInfo:           *info,
		Permissions:        perms,
		Roles:              grant.Roles,
		Menus:              nil, // 菜单改由 GET /menus/tree 按权限过滤返回（P1-13）
		MustChangePassword: admin.MustChangePassword,
	}, nil
}

func (s *adminService) Me(ctx context.Context, adminID uint64) (*dto.AdminInfo, error) {
	admin, err := s.repo.FindByID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	info := toAdminInfo(*admin)
	grant, err := middleware.LoadAdminGrant(ctx, s.cache, s.rbac, adminID)
	if err != nil {
		return nil, err
	}
	info.Roles = grant.Roles
	info.Permissions = grant.Perms.Slice()
	if len(grant.Roles) > 0 {
		info.Role = grant.Roles[0]
	}
	return info, nil
}

// ChangePassword 管理员自助改密，成功后清除强制改密标记。
func (s *adminService) ChangePassword(ctx context.Context, adminID uint64, req dto.AdminChangePasswordRequest) error {
	admin, err := s.repo.FindByID(ctx, adminID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.OldPassword)); err != nil {
		return errors.New("原密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordAndFlag(ctx, adminID, string(hash), false)
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
		info := toAdminInfo(admin)
		if roles, rerr := s.rbac.FindRoleCodesByAdminID(ctx, admin.ID); rerr == nil {
			info.Roles = roles
			if len(roles) > 0 {
				info.Role = roles[0]
			}
		}
		items = append(items, *info)
	}
	return &dto.AdminListResponse{
		Items: items,
		Meta:  dto.AdminListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *adminService) Create(ctx context.Context, req dto.AdminCreateRequest) (*dto.AdminInfo, error) {
	if req.Status == "" {
		req.Status = "active"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	roleIDs, err := s.resolveRoleIDs(ctx, req.RoleIDs, req.Role)
	if err != nil {
		return nil, err
	}
	roleCode := ""
	if codes, cerr := s.rbac.FindRoleCodesByIDs(ctx, roleIDs); cerr == nil && len(codes) > 0 {
		roleCode = codes[0]
	}
	if roleCode == "" {
		roleCode = "admin"
	}

	admin := &model.Admin{
		Username:           req.Username,
		Email:              req.Email,
		PasswordHash:       string(hash),
		Role:               roleCode,
		Department:         req.Department,
		Position:           req.Position,
		Status:             req.Status,
		MustChangePassword: true, // 新建员工首次登录强制改密（P1-08）
	}
	if err := s.repo.Create(ctx, admin); err != nil {
		return nil, err
	}
	if len(roleIDs) > 0 {
		if err := s.rbac.ReplaceAdminRoles(ctx, admin.ID, roleIDs); err != nil {
			return nil, err
		}
	}
	info := toAdminInfo(*admin)
	info.Roles = mustRoleCodes(ctx, s.rbac, roleIDs)
	return info, nil
}

func (s *adminService) FindByID(ctx context.Context, id uint64) (*dto.AdminInfo, error) {
	admin, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := toAdminInfo(*admin)
	info.Roles = mustRoleCodes(ctx, s.rbac, nil)
	if roles, rerr := s.rbac.FindRoleCodesByAdminID(ctx, id); rerr == nil {
		info.Roles = roles
		if len(roles) > 0 {
			info.Role = roles[0]
		}
	}
	return info, nil
}

func (s *adminService) Update(ctx context.Context, id uint64, req dto.AdminUpdateRequest) (*dto.AdminInfo, error) {
	admin, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	admin.Email = req.Email
	admin.Department = req.Department
	admin.Position = req.Position
	admin.Status = req.Status
	// 兼容旧前端：传了单 role code 时同步为唯一角色（多角色走 SetRoles）。
	if req.Role != "" {
		if ids, rerr := s.rbac.FindRoleIDsByCodes(ctx, []string{req.Role}); rerr == nil && len(ids) > 0 {
			if err := s.rbac.ReplaceAdminRoles(ctx, id, ids); err != nil {
				return nil, err
			}
			admin.Role = req.Role
		}
	}
	if err := s.repo.Update(ctx, admin); err != nil {
		return nil, err
	}
	s.invalidate(ctx, id)
	return s.FindByID(ctx, id)
}

func (s *adminService) UpdateStatus(ctx context.Context, id uint64, status string) error {
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}
	s.invalidate(ctx, id)
	return nil
}

func (s *adminService) ResetPassword(ctx context.Context, id uint64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	// 重置后要求下次登录强制改密
	return s.repo.UpdatePasswordAndFlag(ctx, id, string(hash), true)
}

func (s *adminService) Delete(ctx context.Context, id uint64) error {
	if err := s.rbac.ReplaceAdminRoles(ctx, id, nil); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidate(ctx, id)
	return nil
}

// SetRoles 覆盖式设置员工角色，并清理其权限缓存（P1-09：不允许清空为无角色）。
func (s *adminService) SetRoles(ctx context.Context, id uint64, roleIDs []uint64) error {
	if len(roleIDs) == 0 {
		return errors.New("至少保留一个角色")
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	valid, err := s.rbac.FindRoleCodesByIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	if len(valid) == 0 {
		return errors.New("角色不存在")
	}
	if err := s.rbac.ReplaceAdminRoles(ctx, id, roleIDs); err != nil {
		return err
	}
	s.invalidate(ctx, id)
	return nil
}

func (s *adminService) invalidate(_ context.Context, adminID uint64) {
	if s.cache != nil {
		s.cache.InvalidateAdmin(adminID)
	}
}

// resolveRoleIDs 合并 role_ids 与兼容的 role code 入参，得到最终角色 ID 集合。
func (s *adminService) resolveRoleIDs(ctx context.Context, roleIDs []uint64, roleCode string) ([]uint64, error) {
	ids := make([]uint64, 0, len(roleIDs)+1)
	ids = append(ids, roleIDs...)
	if roleCode != "" {
		extra, err := s.rbac.FindRoleIDsByCodes(ctx, []string{roleCode})
		if err != nil {
			return nil, err
		}
		ids = append(ids, extra...)
	}
	return ids, nil
}

func mustRoleCodes(ctx context.Context, rbac repository.RBACRepository, roleIDs []uint64) []string {
	codes, err := rbac.FindRoleCodesByIDs(ctx, roleIDs)
	if err != nil {
		return nil
	}
	return codes
}

func toAdminInfo(admin model.Admin) *dto.AdminInfo {
	return &dto.AdminInfo{
		ID:                 admin.ID,
		Username:           admin.Username,
		Email:              admin.Email,
		Avatar:             admin.Avatar,
		Role:               admin.Role,
		Department:         admin.Department,
		Position:           admin.Position,
		ServiceGroupID:     admin.ServiceGroupID,
		Status:             admin.Status,
		MustChangePassword: admin.MustChangePassword,
		LastLoginIP:        admin.LastLoginIP,
		LastLoginAt:        admin.LastLoginAt,
		CreatedAt:          admin.CreatedAt,
	}
}

// ListAuditLogs 管理端操作审计查询（P2-06）。
func (s *adminService) ListAuditLogs(ctx context.Context, query dto.AdminAuditLogQuery) (*dto.AdminAuditLogResponse, error) {
	if s.audit == nil {
		return &dto.AdminAuditLogResponse{Items: []dto.AdminAuditLogInfo{}, Meta: dto.AdminListMeta{Page: 1, PageSize: 20}}, nil
	}
	items, total, err := s.audit.ListAuditLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	infos := make([]dto.AdminAuditLogInfo, 0, len(items))
	for _, item := range items {
		detail := ""
		if item.Detail != nil {
			detail = *item.Detail
		}
		infos = append(infos, dto.AdminAuditLogInfo{
			ID:            item.ID,
			AdminID:       item.AdminID,
			AdminName:     item.AdminName,
			Module:        item.Module,
			ResourceType:  item.ResourceType,
			ResourceID:    item.ResourceID,
			Action:        item.Action,
			RequestMethod: item.RequestMethod,
			RequestPath:   item.RequestPath,
			ResponseCode:  item.ResponseCode,
			IP:            item.IP,
			UserAgent:     item.UserAgent,
			Detail:        detail,
			CreatedAt:     item.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return &dto.AdminAuditLogResponse{
		Items: infos,
		Meta:  dto.AdminListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}
