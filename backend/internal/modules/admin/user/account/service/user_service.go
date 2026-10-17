package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
	"hostsent/backend/internal/modules/admin/user/account/repository"
	pkgauth "hostsent/backend/internal/pkg/auth"
)

// translateUserFindErr 把「查无此行」的仓储错误翻译成 ErrUserNotFound：
// handler 只依赖本包哨兵，不得越过 service 判断 gorm 语义（CONTRIBUTING §3）。
func translateUserFindErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	return err
}

type UserService interface {
	List(ctx context.Context, query dto.UserListQuery) (*dto.UserListResponse, error)
	Create(ctx context.Context, req dto.UserCreateRequest) (*dto.UserInfo, error)
	FindByID(ctx context.Context, id uint64) (*dto.UserInfo, error)
	Update(ctx context.Context, id uint64, req dto.UserUpdateRequest) (*dto.UserInfo, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	ResetPassword(ctx context.Context, id uint64, password string) error
	AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
	// Delete 已移除（doc104 §4.9）：原实现是无路由的硬删除，且会把该用户
	// 在全库留下的引用变成悬空行。注销一律走 DeletionService.SoftDelete，
	// 到期清理走 DeletionService.Purge。留一个同名入口只会让「哪个才是注销」永久含糊。
	GetStats(ctx context.Context) (*dto.UserStatsResponse, error)
	// ActivityOverview 总览页活动统计：在线用户 + 最近登录用户（含 IP）。
	// 替代原 GetRegionStats（IP 归属地分布已整体下线）。
	ActivityOverview(ctx context.Context, limit, windowHours int) (*dto.UserActivityOverviewResponse, error)
	// Impersonate 代登录：按用户 ID 签发用户端 token（仅 active 用户）。
	// 除签发令牌外，还会补写 login_logs 与 user_sessions（platform=admin），
	// 让代登录在安全页留下可审计的痕迹（doc104 F15 的同口径延伸）。
	Impersonate(ctx context.Context, req ImpersonateRequest) (*dto.ImpersonateResponse, error)
	// Recharge 用户充值（人工调账）
	Recharge(ctx context.Context, id uint64, amount float64, remark string, operatorID uint64) (*RechargeResult, error)
	// ListMembers 查询某主账号名下的成员（子账号）及权限（P4-10）
	ListMembers(ctx context.Context, ownerID uint64) (*dto.SubAccountMemberListResponse, error)
	// —— 管理端子账号管理（P4-10 管理面补全；实现复用客户侧 memberService）——
	// CreateSubAccount 新建子账号，返回一次响应含一次性密码。
	CreateSubAccount(ctx context.Context, ownerID uint64, req dto.AdminSubAccountCreateRequest) (*dto.CreateSubAccountResult, error)
	// UpdateSubAccount 改子账号备注/状态（禁用时连带撤销其会话）。
	UpdateSubAccount(ctx context.Context, ownerID, memberID uint64, req dto.AdminSubAccountUpdateRequest) (*dto.SubAccountMemberInfo, error)
	// SetSubAccountPermissions 覆盖式设置子账号权限。
	SetSubAccountPermissions(ctx context.Context, ownerID, memberID uint64, codes []string) (*dto.SubAccountMemberInfo, error)
	// ResetSubAccountPassword 重置子账号密码（留空则生成一次性密码），并撤销其会话。
	ResetSubAccountPassword(ctx context.Context, ownerID, memberID uint64, password string) (*dto.AdminSubAccountResetPasswordResponse, error)
	// DisableSubAccount 禁用子账号并撤销其会话。
	DisableSubAccount(ctx context.Context, ownerID, memberID uint64) error
	// ListSubAccountLogs 子账号操作日志。
	ListSubAccountLogs(ctx context.Context, ownerID, memberID uint64, page, pageSize int) (*dto.SubAccountLogListResponse, error)
	// SubAccountPermissionOptions 可选客户侧权限项（含中文标签）。
	SubAccountPermissionOptions() []dto.SubAccountPermissionOption
	// SetSubAccountManager 注入子账号管理能力（可选，装配层调用）。
	SetSubAccountManager(manager SubAccountManager)
	// SetDefaultGroupProvider 注入默认用户组解析能力（可选，装配层调用）。
	SetDefaultGroupProvider(provider DefaultGroupProvider)
	// SetLoginRecorder 注入代登录审计写入能力（可选，装配层调用）。
	SetLoginRecorder(recorder LoginRecorder)
	// SetRoleScopeValidator 注入角色作用域校验能力（可选，装配层调用）。
	SetRoleScopeValidator(validator RoleScopeValidator)
	// SetLevelAssigner 注入用户等级调整能力（可选，装配层调用）。
	SetLevelAssigner(assigner LevelAssigner)
	// SetDefaultUserRoleProvider 注入默认客户角色解析能力（可选，装配层调用）。
	SetDefaultUserRoleProvider(provider DefaultUserRoleProvider)
	// SetAgentLevelValidator 注入代理等级校验能力（可选，装配层调用，doc108）。
	SetAgentLevelValidator(validator AgentLevelValidator)
	// SetLogger 注入日志器（可选，装配层调用）。
	SetLogger(logger *zap.Logger)
	// SetSessionRevoker 注入「按用户撤销全部有效会话」能力（可选，装配层调用）。
	SetSessionRevoker(revoker SessionRevoker)
	// SetSessionInvalidator 注入会话校验缓存失效能力（可选，装配层调用）。
	SetSessionInvalidator(invalidator SessionInvalidator)
	// HasSessionRevoker 装配自检：会话撤销能力是否已接线。
	HasSessionRevoker() bool
}

// SessionRevoker 按用户撤销其全部有效会话，返回被撤销的 session_id（供缓存失效）。
// 由 uc/auth 的 userRepository 实现（RevokeSessions）。
type SessionRevoker interface {
	RevokeSessions(ctx context.Context, id uint64, reason string) ([]string, error)
}

// SessionInvalidator 让会话有效性缓存立即失效（由 pkg/sessionguard.Guard 实现）。
type SessionInvalidator interface {
	Invalidate(ctx context.Context, sessionIDs ...string)
}

// RoleScopeValidator 校验一组角色 ID 是否全部属于指定作用域（由 roleService 实现）。
//
// 定义成端口而不是直接调 roleService：用户建号/改角色时若把后台角色（scope=admin）
// 绑到客户账号，该账号会在用户端菜单/权限上拿到员工语义的角色，而员工侧的
// admin_roles 又完全不认这条绑定 —— 两端各说各话。跨域必须挡在服务层，
// 不能只靠前端下拉过滤。
type RoleScopeValidator interface {
	ValidateScope(ctx context.Context, roleIDs []uint64, want string) error
}

// Recharger 充值能力适配器（由装配层注入）。
//
// 正数走充值单链路（登记 + 到账 + 归集当期账单），负数走人工调账扣减；
// 由装配层按符号分流 —— 用户模块不该知道充值单与账单的存在。
type Recharger func(ctx context.Context, userID uint64, amount float64, remark string, operatorID uint64) (*RechargeResult, error)

// RechargeResult 一次人工充值/调账产生的可追溯单号。
//
// 只回单号不回金额：金额调用方本来就知道，运营需要的是「去财务哪个页面能查到
// 这一笔」——正数是充值单号（充值管理页），负数是流水号（资金流水页）。
type RechargeResult struct {
	// RechargeNo 充值单号（正数充值时有值）。
	RechargeNo string
	// TxNo 资金流水号（扣减调账时有值）。
	TxNo string
}

// SubAccountManager 客户侧子账号（成员）管理能力（由 uc/member 服务实现，装配层注入）。
//
// 管理端不重复实现一套成员业务：权限枚举过滤、等级配额、唯一性校验、改密撤会话
// 这些规则必须与客户侧完全一致，各写一份必然漂移（客户侧改了口径、管理端没跟上）。
// 因此这里只声明端口，实现复用同一个 memberService —— 差别仅在归属判定：
// 客户侧以登录主账号为归属，管理端以路径上的主账号 ID 为归属（见 AdminSubAccountManager）。
type SubAccountManager interface {
	// ListMembers 某主账号名下的成员及各自权限。
	ListMembers(ctx context.Context, ownerID uint64) (*SubAccountMemberList, error)
	// CreateMember 新建成员；返回一次性密码（未指定时生成）。
	CreateMember(ctx context.Context, ownerID uint64, req SubAccountCreateInput) (*SubAccountMember, string, error)
	// UpdateMember 改备注/状态。
	UpdateMember(ctx context.Context, ownerID, memberID uint64, req SubAccountUpdateInput) (*SubAccountMember, error)
	// SetMemberPermissions 覆盖式设置权限。
	SetMemberPermissions(ctx context.Context, ownerID, memberID uint64, codes []string) (*SubAccountMember, error)
	// ResetMemberPassword 重置密码，返回一次性密码与连带撤销的会话数。
	ResetMemberPassword(ctx context.Context, ownerID, memberID uint64, password string) (string, int, error)
	// DisableMember 禁用成员并撤销其会话。
	DisableMember(ctx context.Context, ownerID, memberID uint64) error
	// MemberLogs 成员操作日志。
	MemberLogs(ctx context.Context, ownerID, memberID uint64, page, pageSize int) (*SubAccountLogPage, error)
	// PermissionOptions 可选权限项（含中文标签），供前端渲染开关。
	PermissionOptions() []SubAccountPermissionOption
}

// SubAccountCreateInput 新建成员的入参（与客户侧同字段，避免管理端另立口径）。
type SubAccountCreateInput struct {
	Username    string
	Email       string
	Phone       string
	Password    string
	Remark      string
	Permissions []string
}

// SubAccountUpdateInput 修改成员的入参。
type SubAccountUpdateInput struct {
	Remark string
	Status string
}

// SubAccountMember 成员信息。
type SubAccountMember struct {
	ID          uint64
	Username    string
	Name        string
	Email       string
	Phone       string
	Remark      string
	Status      string
	Permissions []string
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

// SubAccountMemberList 成员列表（含配额与可选权限，供前端渲染）。
type SubAccountMemberList struct {
	Members        []SubAccountMember
	MaxSubAccounts int
}

// SubAccountPermissionOption 可选权限项。
type SubAccountPermissionOption struct {
	Code  string
	Label string
}

// SubAccountLogItem 成员操作日志项。
// CreatedAt 用已格式化的字符串：客户侧服务返回的就是展示串，
// 这里再转一次 time.Time 只为了在 DTO 层格式化回同样的字符串，属于无谓往返。
type SubAccountLogItem struct {
	ID        uint64
	ActorID   uint64
	ActorName string
	Module    string
	Action    string
	Target    string
	Detail    string
	IP        string
	CreatedAt string
}

// SubAccountLogPage 成员操作日志分页。
type SubAccountLogPage struct {
	Items []SubAccountLogItem
	Total int64
}

// DefaultGroupProvider 提供默认用户组 ID（由用户组服务实现，装配层注入）；
// 返回 0 表示未配置默认组，此时新用户保持未分组。
type DefaultGroupProvider interface {
	DefaultGroupID(ctx context.Context) (uint64, error)
}

// LoginRecorder 代登录的审计写入能力（由装配层注入，直接落 login_logs /
// user_sessions 两张表）。
//
// 为什么不让本服务直接持有 *gorm.DB 写这两张表：login_logs / user_sessions 的
// 模型归属 admin/user/security 模块，直接 import 会让 account 模块反向依赖
// security 模块。抽成函数端口与 Recharger 同形态 —— 装配层知道两边，模块之间不必知道。
//
// 返回新建会话的 session_id：代登录令牌必须带上 sid 才能被会话校验放行，
// 而会话行是这里写的 —— 只有本端口知道 id 是什么。返回空串表示未写会话
// （loginRecord 未装配，或表不可用），此时调用方应放弃签发令牌。
type LoginRecorder func(ctx context.Context, in ImpersonationRecord) (string, error)

// ImpersonationRecord 一次代登录要落库的审计事实。
//
// AdminID/AdminName 由 handler 从管理端 claims 取出后填入：服务层只负责
// 「谁被代登录 + 从哪来」，不参与管理端身份解析。
type ImpersonationRecord struct {
	UserID    uint64
	Username  string
	AdminID   uint64
	AdminName string
	IP        string
	UserAgent string
}

// ImpersonateRequest 代登录入参。
type ImpersonateRequest struct {
	// UserID 被代登录的用户。
	UserID uint64
	// AdminID / AdminName 发起代登录的管理员（落 login_logs.failure_reason，
	// 现有表结构没有 operator 列，用该列承载「谁代的」是唯一不新增迁移的做法）。
	AdminID   uint64
	AdminName string
	// IP / UserAgent 管理端请求的来源，落会话与登录日志。
	IP        string
	UserAgent string
}

type userService struct {
	repo               repository.UserRepository
	jwtIssuer          *pkgauth.JWTIssuer
	recharge           Recharger
	defaultGroup       DefaultGroupProvider
	loginRecord        LoginRecorder
	roleScopes         RoleScopeValidator
	levels             LevelAssigner
	defaultRole        DefaultUserRoleProvider
	agentLevels        AgentLevelValidator
	logger             *zap.Logger
	sessionRevoker     SessionRevoker
	subAccounts        SubAccountManager
	sessionInvalidator SessionInvalidator
}

func NewUserService(repo repository.UserRepository, jwtIssuer *pkgauth.JWTIssuer, recharge Recharger) UserService {
	return &userService{repo: repo, jwtIssuer: jwtIssuer, recharge: recharge}
}

// LevelAssigner 用户等级调整能力（由用户等级服务实现，装配层注入）。
//
// 抽成端口而不是直接调 levelService：account 模块不该依赖 level 模块，
// 而等级调整必须走 level 服务的校验与留痕（写 user_level_change_logs），
// 不能在这里直接改 users.user_level_id。
type LevelAssigner interface {
	// AssignLevel 手工调整用户等级并写变更日志。
	AssignLevel(ctx context.Context, userID, levelID uint64) error
	// DefaultLevelID 起始等级（启用中权重最低的一级）；无可用等级返回 0。
	DefaultLevelID(ctx context.Context) (uint64, error)
	// CheckAssignable 校验等级存在且启用中，供建号时先验后写。
	CheckAssignable(ctx context.Context, levelID uint64) error
}

// DefaultUserRoleProvider 提供客户默认角色（roles.code='user'）ID；
// 返回 0 表示角色缺失，调用方按「不挂角色」处理（不阻断建号）。
type DefaultUserRoleProvider interface {
	DefaultUserRoleID(ctx context.Context) (uint64, error)
}

// SetDefaultUserRoleProvider 注入默认客户角色解析能力（可选，装配层调用）。
func (s *userService) SetDefaultUserRoleProvider(provider DefaultUserRoleProvider) {
	s.defaultRole = provider
}

// resolveDefaultLevelID 解析起始等级；无可用等级/查询失败时返回 nil（不设等级）。
func (s *userService) resolveDefaultLevelID(ctx context.Context) *uint64 {
	if s.levels == nil {
		return nil
	}
	id, err := s.levels.DefaultLevelID(ctx)
	if err != nil || id == 0 {
		return nil
	}
	return &id
}

// defaultUserRoleID 解析默认客户角色 ID；未装配或缺失时返回 0。
func (s *userService) defaultUserRoleID(ctx context.Context) (uint64, error) {
	if s.defaultRole == nil {
		return 0, nil
	}
	return s.defaultRole.DefaultUserRoleID(ctx)
}

// SetLevelAssigner 注入等级调整能力（可选，装配层调用）。
//
// 未注入时 Update 会拒绝「改等级」的请求（返回 ErrLevelAssignUnavailable），
// 而不是静默忽略：静默忽略会让运营以为保存成功，实际等级没变。
func (s *userService) SetLevelAssigner(assigner LevelAssigner) {
	s.levels = assigner
}

// SetRoleScopeValidator 注入角色作用域校验能力（可选，装配层调用）。
func (s *userService) SetRoleScopeValidator(validator RoleScopeValidator) {
	s.roleScopes = validator
}

// AgentLevelValidator 代理等级校验能力（由代理等级服务实现，装配层注入）。
//
// 抽成端口同 LevelAssigner：account 模块不该依赖 agentlevel 模块。
// 代理等级是折扣的授予依据（doc108），写错等级会让用户按错误价格下单，
// 因此在落库前必须先验存在且启用 —— 这也是改造前 user_group_id 缺失的那道校验。
type AgentLevelValidator interface {
	// CheckAssignable 校验代理等级存在且启用（levelID=0 视为非法）。
	CheckAssignable(ctx context.Context, levelID uint64) error
}

// SetAgentLevelValidator 注入代理等级校验能力（可选，装配层调用）。
// 未注入时 Update 会拒绝「改代理等级」的请求（返回 ErrAgentLevelUnavailable），
// 而不是静默忽略：静默忽略会让运营以为设成了代理，实际没设。
func (s *userService) SetAgentLevelValidator(validator AgentLevelValidator) {
	s.agentLevels = validator
}

// SetLoginRecorder 注入代登录审计写入能力（可选，装配层调用）。
func (s *userService) SetLoginRecorder(recorder LoginRecorder) {
	s.loginRecord = recorder
}

// SetLogger 注入日志器（可选，装配层调用）；未注入时审计失败只静默忽略。
func (s *userService) SetLogger(logger *zap.Logger) {
	s.logger = logger
}

// SetSessionRevoker 注入「按用户撤销全部有效会话」能力（可选，装配层调用）。
func (s *userService) SetSessionRevoker(revoker SessionRevoker) {
	s.sessionRevoker = revoker
}

// SetSessionInvalidator 注入会话校验缓存失效能力（可选，装配层调用）。
func (s *userService) SetSessionInvalidator(invalidator SessionInvalidator) {
	s.sessionInvalidator = invalidator
}

// HasSessionRevoker 装配自检：会话撤销能力是否已接线。
// 漏接线不会编译失败，只会让「禁用用户」不即时踢掉登录态（实测踩过）。
func (s *userService) HasSessionRevoker() bool {
	return s.sessionRevoker != nil
}

// SetSubAccountManager 注入子账号管理能力（可选，装配层调用）。
func (s *userService) SetSubAccountManager(manager SubAccountManager) {
	s.subAccounts = manager
}

// CreateSubAccount 管理端新建子账号。
//
// 复用客户侧 memberService：权限枚举过滤、等级配额、唯一性校验都在那一处实现，
// 管理端只做「归属 = 路径上的主账号」这一次改写。
func (s *userService) CreateSubAccount(ctx context.Context, ownerID uint64, req dto.AdminSubAccountCreateRequest) (*dto.CreateSubAccountResult, error) {
	if s.subAccounts == nil {
		return nil, ErrSubAccountUnavailable
	}
	member, password, err := s.subAccounts.CreateMember(ctx, ownerID, SubAccountCreateInput{
		Username:    req.Username,
		Email:       req.Email,
		Phone:       req.Phone,
		Password:    req.Password,
		Remark:      req.Remark,
		Permissions: req.Permissions,
	})
	if err != nil {
		return nil, err
	}
	return &dto.CreateSubAccountResult{Member: toSubAccountMemberInfo(*member), Password: password}, nil
}

// UpdateSubAccount 管理端改子账号备注/状态。
func (s *userService) UpdateSubAccount(ctx context.Context, ownerID, memberID uint64, req dto.AdminSubAccountUpdateRequest) (*dto.SubAccountMemberInfo, error) {
	if s.subAccounts == nil {
		return nil, ErrSubAccountUnavailable
	}
	member, err := s.subAccounts.UpdateMember(ctx, ownerID, memberID, SubAccountUpdateInput{Remark: req.Remark, Status: req.Status})
	if err != nil {
		return nil, err
	}
	info := toSubAccountMemberInfo(*member)
	return &info, nil
}

// SetSubAccountPermissions 管理端覆盖式设置子账号权限。
func (s *userService) SetSubAccountPermissions(ctx context.Context, ownerID, memberID uint64, codes []string) (*dto.SubAccountMemberInfo, error) {
	if s.subAccounts == nil {
		return nil, ErrSubAccountUnavailable
	}
	member, err := s.subAccounts.SetMemberPermissions(ctx, ownerID, memberID, codes)
	if err != nil {
		return nil, err
	}
	info := toSubAccountMemberInfo(*member)
	return &info, nil
}

// ResetSubAccountPassword 管理端重置子账号密码（留空生成一次性密码）并撤销其会话。
func (s *userService) ResetSubAccountPassword(ctx context.Context, ownerID, memberID uint64, password string) (*dto.AdminSubAccountResetPasswordResponse, error) {
	if s.subAccounts == nil {
		return nil, ErrSubAccountUnavailable
	}
	generated, revoked, err := s.subAccounts.ResetMemberPassword(ctx, ownerID, memberID, password)
	if err != nil {
		return nil, err
	}
	return &dto.AdminSubAccountResetPasswordResponse{Password: generated, RevokedSessions: revoked}, nil
}

// DisableSubAccount 管理端禁用子账号（并撤销其会话）。
func (s *userService) DisableSubAccount(ctx context.Context, ownerID, memberID uint64) error {
	if s.subAccounts == nil {
		return ErrSubAccountUnavailable
	}
	return s.subAccounts.DisableMember(ctx, ownerID, memberID)
}

// ListSubAccountLogs 管理端查看子账号操作日志。
func (s *userService) ListSubAccountLogs(ctx context.Context, ownerID, memberID uint64, page, pageSize int) (*dto.SubAccountLogListResponse, error) {
	if s.subAccounts == nil {
		return nil, ErrSubAccountUnavailable
	}
	pageData, err := s.subAccounts.MemberLogs(ctx, ownerID, memberID, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]dto.SubAccountLogInfo, 0, len(pageData.Items))
	for _, it := range pageData.Items {
		items = append(items, dto.SubAccountLogInfo{
			ID:        it.ID,
			ActorID:   it.ActorID,
			ActorName: it.ActorName,
			Module:    it.Module,
			Action:    it.Action,
			Target:    it.Target,
			Detail:    it.Detail,
			IP:        it.IP,
			CreatedAt: it.CreatedAt,
		})
	}
	return &dto.SubAccountLogListResponse{Items: items, Total: pageData.Total}, nil
}

// SubAccountPermissionOptions 可选客户侧权限项（前端渲染开关用）。
func (s *userService) SubAccountPermissionOptions() []dto.SubAccountPermissionOption {
	if s.subAccounts == nil {
		return []dto.SubAccountPermissionOption{}
	}
	opts := s.subAccounts.PermissionOptions()
	out := make([]dto.SubAccountPermissionOption, 0, len(opts))
	for _, o := range opts {
		out = append(out, dto.SubAccountPermissionOption{Code: o.Code, Label: o.Label})
	}
	return out
}

// toSubAccountMemberInfo 端口结构 → 管理端 DTO。
func toSubAccountMemberInfo(m SubAccountMember) dto.SubAccountMemberInfo {
	perms := m.Permissions
	if perms == nil {
		perms = []string{}
	}
	return dto.SubAccountMemberInfo{
		ID:          m.ID,
		Username:    m.Username,
		Name:        m.Name,
		Email:       m.Email,
		Phone:       m.Phone,
		Remark:      m.Remark,
		Status:      m.Status,
		Permissions: perms,
		CreatedAt:   m.CreatedAt,
		LastLoginAt: m.LastLoginAt,
	}
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
	user := &model.User{ID: req.ID, Username: req.Username, Email: req.Email, Phone: req.Phone, PasswordHash: string(hash), Status: req.Status, UserGroupID: req.UserGroupID, UserLevelID: req.UserLevelID, AgentLevelID: req.AgentLevelID}
	// 建号即校验角色域：客户账号只能挂客户角色（scope=user）。
	// 改造前后台「新建用户」的角色下拉直接调 /admin/roles（返回的是后台角色），
	// 建出来的客户号会挂上 super_admin 之类的员工角色（实测 users.id=12 即如此）。
	// 校验放在落库之前：被拒的角色列表不该留下一条没有角色的用户行。
	if err := s.validateUserRoles(ctx, req.RoleIDs); err != nil {
		return nil, err
	}
	// 未指定分组时归入默认组（未配置默认组则保持未分组）。
	if user.UserGroupID == nil {
		user.UserGroupID = s.resolveDefaultGroupID(ctx)
	}
	// 未指定等级时给起始等级：后台建出来的客户号同样是客户，不该停在「无等级」。
	// 无等级会让子账号上限读到 0（=不限制）且用户端显示无等级。
	// 显式指定的等级先校验存在且启用中（放在落库之前，与角色域校验同一条理由）。
	if user.UserLevelID == nil {
		user.UserLevelID = s.resolveDefaultLevelID(ctx)
	} else if s.levels != nil {
		if err := s.levels.CheckAssignable(ctx, *user.UserLevelID); err != nil {
			return nil, err
		}
	}
	// 代理等级是可选授予（nil = 非代理）；给了就必须先验存在且启用，
	// 与角色/等级同一条理由：被拒的值不该留下一条已落库的用户行。
	if user.AgentLevelID != nil {
		if s.agentLevels == nil {
			return nil, ErrAgentLevelUnavailable
		}
		if err := s.agentLevels.CheckAssignable(ctx, *user.AgentLevelID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	// 未指定角色时挂「普通用户」：与注册链路同一口径，否则后台建的客户号
	// 在用户列表的「客户角色」列是空的，运营无法从界面确认它是普通客户。
	roleIDs := req.RoleIDs
	if len(roleIDs) == 0 {
		if id, rerr := s.defaultUserRoleID(ctx); rerr == nil && id > 0 {
			roleIDs = []uint64{id}
		}
	}
	if len(roleIDs) > 0 {
		if err := s.repo.SetRoles(ctx, user.ID, roleIDs); err != nil {
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
		return nil, translateUserFindErr(err)
	}
	return ptrUserInfo(*user), nil
}

// Update 用户资料部分更新：nil 字段不改，显式空串清空。
//
// 原实现是无条件全量赋值（`user.Username = req.Username` …），配合 required 校验
// 导致「只想改个昵称」也必须把手机邮箱全部带上，且 real_name/region 从未落库。
func (s *userService) Update(ctx context.Context, id uint64, req dto.UserUpdateRequest) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, translateUserFindErr(err)
	}
	if err := rejectIfDeleted(user); err != nil {
		return nil, err
	}
	if req.Username != nil {
		user.Username = strings.TrimSpace(*req.Username)
	}
	if req.RealName != nil {
		user.RealName = strings.TrimSpace(*req.RealName)
	}
	if req.Email != nil {
		user.Email = strings.TrimSpace(*req.Email)
	}
	if req.Phone != nil {
		user.Phone = strings.TrimSpace(*req.Phone)
	}
	if req.Region != nil {
		user.Region = strings.TrimSpace(*req.Region)
	}
	if req.SubAccountRemark != nil {
		user.SubAccountRemark = strings.TrimSpace(*req.SubAccountRemark)
	}
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		user.Status = strings.TrimSpace(*req.Status)
	}
	// nil 表示不修改分组；0 表示移出分组；其余为组 ID。
	if req.UserGroupID != nil {
		if *req.UserGroupID == 0 {
			user.UserGroupID = nil
		} else {
			user.UserGroupID = req.UserGroupID
		}
	}
	// 等级调整走等级服务（校验 + 写变更日志），不在 user 行上直写。
	//
	// 分两步：先只校验（不落库），把资料更新做掉，最后才真正改等级。
	// 反过来先改等级的话，资料更新失败（用户名冲突等）会让这次「报错的请求」
	// 已经改掉了等级 —— 等级是权益授予依据，不该在返回失败时静默变化。
	// 两步之间不是事务（两侧仓储各自持有连接），所以校验必须放在最前面，
	// 让「等级非法」这类必然失败在写任何东西之前就被挡住。
	if req.UserLevelID != nil {
		if s.levels == nil {
			return nil, ErrLevelAssignUnavailable
		}
		if err := s.levels.CheckAssignable(ctx, *req.UserLevelID); err != nil {
			return nil, err
		}
	}
	// 代理等级：nil 不修改，0 取消代理身份，其余为等级 ID。
	// 与用户组不同，0 是合法取值（"取消代理"），非 0 必须先验存在且启用 ——
	// 代理等级决定拿货价，挂到禁用/不存在的等级上会让折扣静默失效。
	if req.AgentLevelID != nil {
		if *req.AgentLevelID == 0 {
			user.AgentLevelID = nil
		} else {
			if s.agentLevels == nil {
				return nil, ErrAgentLevelUnavailable
			}
			if err := s.agentLevels.CheckAssignable(ctx, *req.AgentLevelID); err != nil {
				return nil, err
			}
			user.AgentLevelID = req.AgentLevelID
		}
	}
	if user.Username == "" {
		return nil, errors.New("用户名不能为空")
	}
	if user.Email == "" {
		return nil, errors.New("邮箱不能为空")
	}
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, mapUserWriteErr(err, user.Username, user.Email)
	}
	if req.UserLevelID != nil {
		if err := s.levels.AssignLevel(ctx, id, *req.UserLevelID); err != nil {
			return nil, err
		}
	}
	fresh, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return ptrUserInfo(*fresh), nil
}

// mapUserWriteErr 把用户名/邮箱唯一索引冲突翻译成可读的哨兵错误。
//
// 迁移 051 把普通唯一索引换成了部分唯一索引，约束名随之改变：
//
//	idx_users_username → uk_users_username_active
//	idx_users_email    → uk_users_email_active
//
// 两个名字都要认——老库在迁移未跑之前仍会用旧名，只认新名会让 409 退化成 500
// （R2 基线：唯一冲突必须返回 409）。
func mapUserWriteErr(err error, username, email string) error {
	if !isUniqueViolation(err) {
		return err
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "uk_users_email_active"), strings.Contains(msg, "idx_users_email"):
		return ErrEmailTaken
	case strings.Contains(msg, "uk_users_username_active"), strings.Contains(msg, "idx_users_username"):
		return ErrUsernameTaken
	}
	// 约束名不可得时按内容兜底：错误文本里带着冲突值。
	if email != "" && strings.Contains(msg, email) {
		return ErrEmailTaken
	}
	return ErrUsernameTaken
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	// 找不到 pgx 错误类型时退回文本匹配（驱动被替换时不至于失效）。
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key value")
}

// rejectIfDeleted 拒绝对已注销用户的常规写操作。
//
// 已注销用户只应通过「恢复」重新变为可操作；放行改名/改状态会让回收站里的行
// 出现「已注销但仍在被编辑」的中间态，也会让「注销后账号名可被重新注册」
// 这条部分唯一索引语义出现歧义。
func rejectIfDeleted(user *model.User) error {
	if user != nil && user.DeletedAt != nil {
		return ErrUserDeleted
	}
	return nil
}

// UpdateStatus 调整用户状态，并在「禁用」时级联处置其子账号与登录态。
//
// 为什么要级联：主账号是客户主体，子账号从属其下（设计 §4.6.2）。此前禁用主账号
// 只改这一行 status，子账号的令牌与会话毫无变化 —— 实测确认被禁用主账号的子账号
// 仍能正常调接口、操作主账号资源。会话校验侧已补账号状态判定（pkg/sessionguard），
// 这里再做一次显式级联撤销，让「停用」即时生效而不必等校验缓存过期。
func (s *userService) UpdateStatus(ctx context.Context, id uint64, status string) error {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := rejectIfDeleted(user); err != nil {
		return err
	}
	if !model.IsValidStatus(status) {
		return fmt.Errorf("非法状态：%s", status)
	}
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}
	if status == "active" {
		return nil
	}
	// 非 active（禁用/暂停）→ 同步子账号并撤销双方的会话，避免旧令牌继续可用。
	subIDs, err := s.repo.CascadeStatusToSubAccounts(ctx, id, status)
	if err != nil {
		// 级联失败不回滚主账号状态：主账号已停用是既定事实，
		// 而 sessionguard 的账号状态判定仍会兜住子账号的每一个请求。
		s.log().Warn("cascade sub-account status failed",
			zap.Uint64("owner_id", id), zap.Error(err))
	}
	s.revokeSessionsOf(ctx, append(subIDs, id), "主账号被禁用，登录态一并失效")
	return nil
}

// revokeSessionsOf 撤销一批用户（客户域）的全部有效会话并让校验缓存立即失效。
//
// 失败只告警：撤销落库本身若失败，sessionguard 的账号状态判定仍会拒绝这些令牌；
// 这里是「尽快生效」而非唯一防线，不该让禁用用户这个动作整体失败。
func (s *userService) revokeSessionsOf(ctx context.Context, userIDs []uint64, reason string) {
	if s.sessionRevoker == nil {
		return
	}
	for _, uid := range userIDs {
		sessionIDs, err := s.sessionRevoker.RevokeSessions(ctx, uid, reason)
		if err != nil {
			s.log().Warn("revoke sessions failed",
				zap.Uint64("user_id", uid), zap.Error(err))
			continue
		}
		if s.sessionInvalidator != nil && len(sessionIDs) > 0 {
			s.sessionInvalidator.Invalidate(ctx, sessionIDs...)
		}
	}
}

// log 返回日志器，未注入时为 no-op，避免各处判空。
func (s *userService) log() *zap.Logger {
	if s.logger == nil {
		return zap.NewNop()
	}
	return s.logger
}

func (s *userService) ResetPassword(ctx context.Context, id uint64, password string) error {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := rejectIfDeleted(user); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, id, string(hash))
}

// AssignRoles 覆盖用户角色。
//
// 服务层再兜一次非空：validator 的 `required` 对空 slice 不触发（doc104 §3.1 F4），
// 实测 `{"role_ids":[]}` 曾通过校验并静默清空用户全部角色。binding 已改 min=1，
// 这里防的是将来有调用方绕过 binding 直接调服务。
func (s *userService) AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	if len(roleIDs) == 0 {
		return ErrEmptyRoleList
	}
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := rejectIfDeleted(user); err != nil {
		return err
	}
	if err := s.validateUserRoles(ctx, roleIDs); err != nil {
		return err
	}
	return s.repo.SetRoles(ctx, userID, roleIDs)
}

// validateUserRoles 校验待绑定的角色全部属于客户域（scope=user）。
//
// 未注入校验器时放行：装配层漏注入不该让「分配角色」整体不可用，但会把
// 越域绑定留在库里 —— 因此装配层必须调用 SetRoleScopeValidator（见 server.go）。
func (s *userService) validateUserRoles(ctx context.Context, roleIDs []uint64) error {
	if s.roleScopes == nil || len(roleIDs) == 0 {
		return nil
	}
	return s.roleScopes.ValidateScope(ctx, roleIDs, model.RoleScopeUser)
}

// ErrEmptyRoleList 角色列表为空：会清空用户全部角色，属误操作。

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
		Deleted:         stats.Deleted,
		SubAccountTotal: stats.SubAccountTotal,
	}, nil
}

func toUserInfo(user model.User) dto.UserInfo {
	return dto.UserInfo{
		ID:                     user.ID,
		Username:               user.Username,
		RealName:               user.RealName,
		Role:                   user.Role,
		Roles:                  user.Roles,
		Email:                  user.Email,
		Phone:                  user.Phone,
		UserGroupID:            user.UserGroupID,
		UserGroupName:          user.UserGroupName,
		UserLevelID:            user.UserLevelID,
		UserLevelName:          user.UserLevelName,
		UserLevelCode:          user.UserLevelCode,
		UserLevelIcon:          user.UserLevelIcon,
		UserLevelColor:         user.UserLevelColor,
		UserLevelWeight:        user.UserLevelWeight,
		AgentLevelID:           user.AgentLevelID,
		AgentLevelName:         user.AgentLevelName,
		AgentLevelCode:         user.AgentLevelCode,
		Region:                 user.Region,
		Avatar:                 user.Avatar,
		Tier:                   user.Tier,
		LastLoginIP:            user.LastLoginIP,
		OAuthProvider:          user.OAuthProvider,
		OAuthOpenID:            user.OAuthOpenID,
		Balance:                user.Balance,
		TotalConsumeAmount:     user.TotalConsumeAmount,
		Status:                 user.Status,
		PhoneVerifiedAt:        user.PhoneVerifiedAt,
		EmailVerifiedAt:        user.EmailVerifiedAt,
		InviteCode:             user.InviteCode,
		InviterUserID:          user.InviterUserID,
		InviterName:            user.InviterName,
		InvitedAt:              user.InvitedAt,
		IsSubAccount:           user.IsSubAccount,
		OwnerUserID:            user.OwnerUserID,
		OwnerName:              user.OwnerName,
		SubAccountRemark:       user.SubAccountRemark,
		SalesAdminID:           user.SalesAdminID,
		SalesAdminName:         user.SalesAdminName,
		CreatedAt:              user.CreatedAt,
		UpdatedAt:              user.UpdatedAt,
		LastLoginAt:            user.LastLoginAt,
		DeletedAt:              user.DeletedAt,
		DeletedBy:              user.DeletedBy,
		DeletedByName:          user.DeletedByName,
		DeleteReason:           user.DeleteReason,
		StatusBeforeDelete:     user.StatusBeforeDelete,
		RealNameVerifiedAt:     user.RealNameVerifiedAt,
		RealNameVerifiedSource: user.RealNameVerifiedSource,
		OAuthProviders:         oauthOrEmpty(user.OAuthProviders),
	}
}

// oauthOrEmpty 把 nil 切片规范成空切片：前端 `v-for` 对 null 与 [] 的处理不同，
// 返回 null 会让「未绑定任何渠道」与「字段缺失」在界面上无法区分。
func oauthOrEmpty(providers []string) []string {
	if providers == nil {
		return []string{}
	}
	return providers
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
//
// 除签发令牌外还补写 login_logs（login_type=impersonate）与 user_sessions
// （platform=admin）：代登录本质是「管理员以用户身份进入用户中心」，
// 是权限最大的一次操作，不落痕就等于安全页看不到任何迹象 ——
// 既无法回答「这个用户的会话是哪来的」，也让「强制下线」对该会话失效。
//
// 顺序是「先写会话、再签令牌」：令牌里的 sid 必须是真实存在的会话，
// 中间件会拿它查 user_sessions，查不到就 401。反过来先签令牌再补会话的话，
// 补写失败会签出一张立刻不可用的令牌。
//
// 审计写入失败**阻断**代登录（与登录链路的 openSession 同口径）：没有会话
// 就没有可撤销的登录态，而代登录恰恰是权限最大、最需要能撤销的一次操作。
func (s *userService) Impersonate(ctx context.Context, req ImpersonateRequest) (*dto.ImpersonateResponse, error) {
	user, err := s.repo.FindByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, errors.New("仅可代登录正常状态用户")
	}
	if s.jwtIssuer == nil {
		return nil, errors.New("代登录能力未配置")
	}
	if s.loginRecord == nil {
		return nil, errors.New("代登录审计未配置")
	}
	info, err := s.FindByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	sessionID, err := s.loginRecord(ctx, ImpersonationRecord{
		UserID:    user.ID,
		Username:  user.Username,
		AdminID:   req.AdminID,
		AdminName: req.AdminName,
		IP:        req.IP,
		UserAgent: req.UserAgent,
	})
	if err != nil {
		return nil, err
	}
	if sessionID == "" {
		return nil, errors.New("代登录会话未创建，请稍后重试")
	}
	// 代登录同样带上归属信息（P4-03），子账号被代登录时权限语义与本人登录一致。
	ownerID := uint64(0)
	if user.OwnerUserID != nil {
		ownerID = *user.OwnerUserID
	}
	token, err := s.jwtIssuer.GenerateUserSession(user.Username, user.ID, user.Tier, ownerID, user.IsSubAccount, sessionID)
	if err != nil {
		return nil, err
	}
	return &dto.ImpersonateResponse{Token: token, UserInfo: *info}, nil
}

// ActivityOverview 总览页活动统计（在线用户 + 最近登录用户，含 IP）。
func (s *userService) ActivityOverview(ctx context.Context, limit, windowHours int) (*dto.UserActivityOverviewResponse, error) {
	data, err := s.repo.ActivityOverview(ctx, limit, windowHours)
	if err != nil {
		return nil, err
	}
	resp := &dto.UserActivityOverviewResponse{
		OnlineTotal:       data.OnlineTotal,
		OnlineUsers:       make([]dto.OnlineUserItem, 0, len(data.OnlineUsers)),
		RecentTotal:       data.RecentTotal,
		RecentUsers:       make([]dto.RecentLoginItem, 0, len(data.RecentUsers)),
		RecentWindowHours: data.RecentWindowHours,
	}
	for _, row := range data.OnlineUsers {
		resp.OnlineUsers = append(resp.OnlineUsers, dto.OnlineUserItem{
			UserID:     row.UserID,
			Username:   row.Username,
			Platform:   row.Platform,
			IP:         row.IP,
			SessionID:  row.SessionID,
			LoginAt:    row.LoginAt,
			LastActive: row.LastActive,
			ExpiredAt:  row.ExpiredAt,
		})
	}
	for _, row := range data.RecentUsers {
		resp.RecentUsers = append(resp.RecentUsers, dto.RecentLoginItem{
			UserID:      row.UserID,
			Username:    row.Username,
			IP:          row.IP,
			LastLoginAt: row.LastLoginAt,
			Status:      row.Status,
		})
	}
	return resp, nil
}

// ErrInvalidAdjustAmount 调账金额为 0：既不是充值也不是扣减，视为误操作。

// Recharge 用户钱包人工充值 / 调账，委托给注入的 Recharger。
//
// 金额为正表示充值（会生成充值单并归集当期账单），为负表示扣减（人工调账）。
// 详情页的「调整余额」需要双向能力，原先固定 direction=1，扣减只能靠负数的
// 语义歧义绕过去。
//
// 子账号一律拒绝：所有消费路径（下单/续费）都按 EffectiveUserID 取主账号，
// 子账号 wallet 里的余额是「孤儿钱」——既花不掉也无法提现，只会让财务
// 对账多出一笔无处归集的账。要充值请对主账号操作。
func (s *userService) Recharge(ctx context.Context, id uint64, amount float64, remark string, operatorID uint64) (*RechargeResult, error) {
	if amount == 0 {
		return nil, ErrInvalidAdjustAmount
	}
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		// 用户不存在时把 GORM 的 "record not found" 翻成领域错误：否则 handler
		// 只能落到 default 分支回 500「服务器内部错误」，运营看不出是自己
		// 传了一个已经不存在的用户 ID。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if user.IsSubAccount {
		return nil, ErrRechargeSubAccountDenied
	}
	if s.recharge == nil {
		return nil, errors.New("充值能力未配置")
	}
	return s.recharge(ctx, id, amount, remark, operatorID)
}
