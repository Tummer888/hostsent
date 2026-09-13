package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/model"
	"hostsent/backend/internal/modules/admin/manager/repository"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/netutil"
	"hostsent/backend/internal/pkg/security"
)

type AdminService interface {
	Login(ctx context.Context, req dto.AdminLoginRequest, ip, userAgent string) (*dto.AdminLoginResponse, error)
	// VerifyLoginOTP 完成登录二次验证，成功返回正式令牌（doc91 §5.2）。
	VerifyLoginOTP(ctx context.Context, req dto.AdminVerifyOTPRequest, ip, userAgent string) (*dto.AdminLoginResponse, error)
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
	// Resign 员工离职（S1）：置离职标记 + 禁用账号 + 交待在途客户（S4 装配后生效）。
	Resign(ctx context.Context, id uint64, req dto.AdminResignRequest) error
	// ListAuditLogs 管理端操作审计查询（P2-06）。
	ListAuditLogs(ctx context.Context, query dto.AdminAuditLogQuery) (*dto.AdminAuditLogResponse, error)
	// SetSecurityDeps 注入登录安全端口（doc91 C3，装配层在 captcha 装配后调用）。
	SetSecurityDeps(port security.Port, ipRegion netutil.IPRegionResolver)
}

// DepartmentNameResolver 部门名解析（由部门仓储实现）。
// 定义在服务包内，避免 manager service → repository 的接口膨胀。
type DepartmentNameResolver interface {
	NameMap(ctx context.Context, ids []uint64) (map[uint64]string, error)
}

// SalesReleaser 员工离职时的在途客户交待人（S4 销售归属装配后注入；未装配时为 nil，跳过）。
type SalesReleaser interface {
	ReleaseStaff(ctx context.Context, adminID uint64, transferTo uint64) error
}

type adminService struct {
	repo      repository.AdminRepository
	rbac      repository.RBACRepository
	audit     repository.AdminAuditRepository
	cache     middleware.PermissionCache
	jwtIssuer *appauth.JWTIssuer
	// deptNames 部门名解析（列表/详情回填 department_name），可为 nil。
	deptNames DepartmentNameResolver
	// salesReleaser 离职交待在途客户，可为 nil（S1 阶段尚未装配）。
	salesReleaser SalesReleaser
	// sec 登录安全端口（图形码/OTP/锁定/登录日志），doc91 C3；未装配时全部放行。
	sec security.Port
	// ipRegion 登录日志的 IP 归属地解析，可为 nil。
	ipRegion netutil.IPRegionResolver
}

func NewAdminService(
	repo repository.AdminRepository,
	rbac repository.RBACRepository,
	audit repository.AdminAuditRepository,
	cache middleware.PermissionCache,
	jwtIssuer *appauth.JWTIssuer,
	deptNames DepartmentNameResolver,
	salesReleaser SalesReleaser,
) AdminService {
	return &adminService{
		repo: repo, rbac: rbac, audit: audit, cache: cache, jwtIssuer: jwtIssuer,
		deptNames: deptNames, salesReleaser: salesReleaser,
	}
}

// SetSecurityDeps 注入登录安全依赖（doc91 C3，装配层调用）。
//
// 抽成 setter 而不是构造参数：Login 与 admin 的其余职责无关，且既有装配顺序
// （captchaBundle 在 adminService 之后构建）决定了端口只能后置注入。
func (s *adminService) SetSecurityDeps(port security.Port, ipRegion netutil.IPRegionResolver) {
	s.sec = port
	s.ipRegion = ipRegion
}

// loginFail 记录一次登录失败：写 login_logs + 失败计数（doc91 §9.1）。
// 账号不存在也照写，便于运营发现撞库（不暴露账号是否存在给调用方）。
func (s *adminService) loginFail(ctx context.Context, username, ip, userAgent, reason string) {
	if s.sec == nil {
		return
	}
	s.sec.RecordFailure(ctx, username, ip)
	s.sec.Log(ctx, security.LoginLogEntry{
		Username:      username,
		LoginType:     "password",
		Result:        security.LoginResultFailed,
		FailureReason: reason,
		IP:            ip,
		IPRegion:      s.resolveIPRegion(ctx, ip),
		UserAgent:     userAgent,
		Platform:      "admin",
	})
}

// resolveIPRegion IP 归属地（解析器未装配时返回空串）。
func (s *adminService) resolveIPRegion(ctx context.Context, ip string) string {
	if s.ipRegion == nil || ip == "" {
		return ""
	}
	return s.ipRegion.Resolve(ctx, ip)
}

// Login 管理端登录（doc91 §5.2）。
//
// 顺序：锁定检查 → 图形码（策略要求时）→ 账号/密码 → 二次验证。
// 二次验证命中时**不签发访问令牌**，只返回待验证令牌。
func (s *adminService) Login(ctx context.Context, req dto.AdminLoginRequest, ip, userAgent string) (*dto.AdminLoginResponse, error) {
	// ① 锁定检查：默认 login_fail_lock=false，行为与升级前一致。
	if s.sec != nil {
		if err := s.sec.CheckLocked(ctx, req.Username, ip); err != nil {
			s.loginFail(ctx, req.Username, ip, userAgent, "locked")
			return nil, err
		}
	}
	// ② 图形码：策略要求时必须先过（admin_login 默认不要求）。
	if s.sec != nil && s.sec.EffectiveImageRequired(ctx, security.SceneAdminLogin, security.Subject{IsAdmin: true}) {
		if err := s.sec.VerifyImage(ctx, security.SceneAdminLogin, req.CaptchaKey, req.CaptchaCode); err != nil {
			s.loginFail(ctx, req.Username, ip, userAgent, "captcha")
			return nil, err
		}
	}

	admin, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.loginFail(ctx, req.Username, ip, userAgent, "user_not_found")
			return nil, security.ErrInvalidCredential
		}
		return nil, err
	}

	if admin.Status != "active" {
		s.loginFail(ctx, req.Username, ip, userAgent, "disabled")
		return nil, security.ErrLoginDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		s.loginFail(ctx, req.Username, ip, userAgent, "bad_password")
		return nil, security.ErrInvalidCredential
	}

	// ③ 二次验证：命中则下发 OTP 并返回待验证令牌，不签发访问令牌。
	if s.sec != nil {
		if required, _ := s.sec.EffectiveOTP(ctx, security.SceneAdminLogin, security.Subject{IsAdmin: true, ID: admin.ID}); required {
			pending, perr := s.sec.IssueOTPPending(ctx, security.SceneAdminLogin,
				security.Subject{IsAdmin: true, ID: admin.ID}, ip, userAgent)
			if perr != nil {
				return nil, perr
			}
			return &dto.AdminLoginResponse{
				NeedOTP:         true,
				OTPToken:        pending.Token,
				OTPChannel:      pending.Channel,
				OTPTargetMasked: pending.TargetMasked,
				OTPExpireIn:     pending.ExpireIn,
			}, nil
		}
	}

	if err := s.repo.UpdateLoginProfile(ctx, admin.ID, ip, time.Now()); err != nil {
		return nil, err
	}
	// ④ 成功：清零失败计数 + 写成功日志。
	if s.sec != nil {
		s.sec.ResetFailure(ctx, req.Username, ip)
		s.sec.Log(ctx, security.LoginLogEntry{
			UserID: admin.ID, Username: admin.Username, LoginType: "password",
			Result: security.LoginResultSuccess, IP: ip,
			IPRegion: s.resolveIPRegion(ctx, ip), UserAgent: userAgent, Platform: "admin",
		})
	}
	return s.issueAdminLogin(ctx, admin)
}

// issueAdminLogin 组装管理端登录成功响应（令牌 + 权限）。
func (s *adminService) issueAdminLogin(ctx context.Context, admin *model.Admin) (*dto.AdminLoginResponse, error) {
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

// VerifyLoginOTP 完成管理端登录二次验证（doc91 §5.2）。
func (s *adminService) VerifyLoginOTP(ctx context.Context, req dto.AdminVerifyOTPRequest, ip, userAgent string) (*dto.AdminLoginResponse, error) {
	if s.sec == nil {
		return nil, security.ErrInvalidOTPToken
	}
	pending, err := s.sec.ConsumeOTPPending(ctx, req.OTPToken, req.Code)
	if err != nil {
		return nil, err
	}
	// aud/jti 已由端口校验；这里再确认令牌确实是管理端的（防用户令牌换管理端登录）。
	if !pending.IsAdmin || pending.Scene != security.SceneAdminLogin {
		return nil, security.ErrInvalidOTPToken
	}
	admin, err := s.repo.FindByID(ctx, pending.UserID)
	if err != nil {
		return nil, security.ErrInvalidOTPToken
	}
	if admin.Status != "active" {
		return nil, security.ErrLoginDisabled
	}
	if err := s.repo.UpdateLoginProfile(ctx, admin.ID, ip, time.Now()); err != nil {
		return nil, err
	}
	s.sec.ResetFailure(ctx, admin.Username, ip)
	s.sec.Log(ctx, security.LoginLogEntry{
		UserID: admin.ID, Username: admin.Username, LoginType: "password",
		Result: security.LoginResultSuccess, IP: ip,
		IPRegion: s.resolveIPRegion(ctx, ip), UserAgent: userAgent, Platform: "admin",
	})
	return s.issueAdminLogin(ctx, admin)
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
	// 平台密码策略（doc91 §9.1）：6 个 password_* 开关在此生效。
	if err := s.validatePassword(ctx, req.NewPassword); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordAndFlag(ctx, adminID, string(hash), false)
}

// validatePassword 校验新密码是否符合平台策略（端口未装配时按默认策略）。
func (s *adminService) validatePassword(ctx context.Context, password string) error {
	if s.sec == nil {
		return nil
	}
	return s.sec.ValidatePassword(ctx, password)
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
	deptIDs := make([]uint64, 0, len(admins))
	for _, admin := range admins {
		info := toAdminInfo(admin)
		if roles, rerr := s.rbac.FindRoleCodesByAdminID(ctx, admin.ID); rerr == nil {
			info.Roles = roles
			if len(roles) > 0 {
				info.Role = roles[0]
			}
		}
		items = append(items, *info)
		if admin.DepartmentID > 0 {
			deptIDs = append(deptIDs, admin.DepartmentID)
		}
	}
	if s.deptNames != nil && len(deptIDs) > 0 {
		names, nerr := s.deptNames.NameMap(ctx, deptIDs)
		if nerr == nil {
			for i := range items {
				if name, ok := names[items[i].DepartmentID]; ok {
					items[i].DepartmentName = name
				}
			}
		}
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
	if req.StaffType != "" && !model.IsValidStaffType(req.StaffType) {
		return nil, errors.New("员工类型不合法")
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

	staffType := req.StaffType
	if staffType == "" {
		staffType = model.StaffTypeAdmin
	}
	admin := &model.Admin{
		Username:           req.Username,
		Email:              req.Email,
		PasswordHash:       string(hash),
		Role:               roleCode,
		Department:         req.Department,
		Position:           req.Position,
		Status:             req.Status,
		RealName:           strings.TrimSpace(req.RealName),
		Phone:              strings.TrimSpace(req.Phone),
		DepartmentID:       req.DepartmentID,
		StaffType:          staffType,
		SalesEnabled:       req.SalesEnabled,
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
	return s.FindByID(ctx, admin.ID)
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
	if s.deptNames != nil && admin.DepartmentID > 0 {
		if names, nerr := s.deptNames.NameMap(ctx, []uint64{admin.DepartmentID}); nerr == nil {
			info.DepartmentName = names[admin.DepartmentID]
		}
	}
	return info, nil
}

func (s *adminService) Update(ctx context.Context, id uint64, req dto.AdminUpdateRequest) (*dto.AdminInfo, error) {
	admin, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.StaffType != "" && !model.IsValidStaffType(req.StaffType) {
		return nil, errors.New("员工类型不合法")
	}
	admin.Email = req.Email
	admin.Department = req.Department
	admin.Position = req.Position
	admin.Status = req.Status
	admin.RealName = strings.TrimSpace(req.RealName)
	admin.Phone = strings.TrimSpace(req.Phone)
	admin.DepartmentID = req.DepartmentID
	admin.SalesEnabled = req.SalesEnabled
	if req.StaffType != "" {
		admin.StaffType = req.StaffType
	}
	// 兼容旧前端：传了单 role code 时同步为唯一角色（多角色走 SetRoles）。
	if req.Role != "" {
		if ids, rerr := s.rbac.FindRoleIDsByCodes(ctx, []string{req.Role}); rerr == nil && len(ids) > 0 {
			if err := s.rbac.ReplaceAdminRoles(ctx, id, ids); err != nil {
				return nil, err
			}
			admin.Role = req.Role
		}
	}
	// Save 会写全量列：显式 Select 保证零值（如清空 phone、关闭 sales_enabled）也能落库。
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

// Resign 员工离职（doc86 §2.1/S4）：
//  1. 置 resigned_at + status=disabled（保留账号与角色绑定，历史工单/提成可追溯）；
//  2. 交待在途客户（salesReleaser 由 S4 装配，未装配时跳过，不影响离职本身）；
//  3. 失效权限缓存，旧 token 立即失效。
//
// 提成不受影响：已产生的提成照常解冻与提现，离职不等于清账。
func (s *adminService) Resign(ctx context.Context, id uint64, req dto.AdminResignRequest) error {
	admin, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if admin.ResignedAt != nil {
		return errors.New("该员工已离职")
	}
	if err := s.repo.Resign(ctx, id, time.Now()); err != nil {
		return err
	}
	// 客户交待失败不回滚离职：离职是人事事实，客户归属可事后补办；
	// 失败原因由装配层实现负责记录日志（与返现/通知钩子同一约定）。
	if s.salesReleaser != nil {
		if rerr := s.salesReleaser.ReleaseStaff(ctx, id, req.TransferToAdminID); rerr != nil {
			// 交由上层日志记录，不阻断接口（离职状态已落库）。
			_ = rerr
		}
	}
	s.invalidate(ctx, id)
	return nil
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
		Status:             admin.Status,
		RealName:           admin.RealName,
		Phone:              admin.Phone,
		DepartmentID:       admin.DepartmentID,
		StaffType:          admin.StaffType,
		SalesEnabled:       admin.SalesEnabled,
		JoinedAt:           admin.JoinedAt,
		ResignedAt:         admin.ResignedAt,
		IsResigned:         admin.ResignedAt != nil,
		ServiceGroupID:     admin.ServiceGroupID,
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
