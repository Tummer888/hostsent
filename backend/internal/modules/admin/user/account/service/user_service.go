package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
	"hostsent/backend/internal/modules/admin/user/account/repository"
	pkgauth "hostsent/backend/internal/pkg/auth"
)

// 用户资料更新的哨兵错误：handler 据此映射 409，而不是把 GORM 原文抛成 500。
var (
	ErrUsernameTaken = errors.New("用户名已被占用")
	ErrEmailTaken    = errors.New("邮箱已被占用")
	// ErrLevelAssignUnavailable 装配层未注入等级调整能力，改等级请求无法处理。
	ErrLevelAssignUnavailable = errors.New("用户等级调整能力未配置")
	// ErrLevelNotFound 目标等级不存在。
	ErrLevelNotFound = errors.New("用户等级不存在")
	// ErrLevelDisabled 目标等级已停用。
	ErrLevelDisabled = errors.New("目标等级已停用，不能指派")
)

type UserService interface {
	List(ctx context.Context, query dto.UserListQuery) (*dto.UserListResponse, error)
	// ExportList 导出用户列表（doc104 §3.3 F19）：与 List 共用同一份筛选条件与
	// 仓储查询，保证「界面上筛出来的」与「导出的」一致；上限 exportLimit 条。
	ExportList(ctx context.Context, query dto.UserListQuery) ([]dto.UserInfo, error)
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
	// SetLogger 注入日志器（可选，装配层调用）。
	SetLogger(logger *zap.Logger)
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
	repo         repository.UserRepository
	jwtIssuer    *pkgauth.JWTIssuer
	recharge     Recharger
	defaultGroup DefaultGroupProvider
	loginRecord  LoginRecorder
	roleScopes   RoleScopeValidator
	levels       LevelAssigner
	defaultRole  DefaultUserRoleProvider
	logger       *zap.Logger
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

// SetLoginRecorder 注入代登录审计写入能力（可选，装配层调用）。
func (s *userService) SetLoginRecorder(recorder LoginRecorder) {
	s.loginRecord = recorder
}

// SetLogger 注入日志器（可选，装配层调用）；未注入时审计失败只静默忽略。
func (s *userService) SetLogger(logger *zap.Logger) {
	s.logger = logger
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

// exportLimit 单次导出的最大行数。与登录日志/审计日志导出的 1000 条同口径：
// 用户表是全平台最大的表之一，不设上限的导出会把整库拉进内存。
const exportLimit = 1000

// ExportList 导出用户列表。复用 List 的仓储查询与筛选语义，
// 只把分页固定为「第 1 页、最多 exportLimit 条」。
func (s *userService) ExportList(ctx context.Context, query dto.UserListQuery) ([]dto.UserInfo, error) {
	query.Page = 1
	query.PageSize = exportLimit
	users, _, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	items := make([]dto.UserInfo, 0, len(users))
	for _, user := range users {
		items = append(items, toUserInfo(user))
	}
	return items, nil
}

func (s *userService) Create(ctx context.Context, req dto.UserCreateRequest) (*dto.UserInfo, error) {
	if req.Status == "" {
		req.Status = "active"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{ID: req.ID, Username: req.Username, Email: req.Email, Phone: req.Phone, PasswordHash: string(hash), Status: req.Status, UserGroupID: req.UserGroupID, UserLevelID: req.UserLevelID}
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
		return nil, err
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
		return nil, err
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
	return s.repo.UpdateStatus(ctx, id, status)
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
var ErrEmptyRoleList = errors.New("角色列表不能为空")

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
var ErrInvalidAdjustAmount = errors.New("调账金额不能为 0")

// Recharge 用户钱包人工充值 / 调账，委托给注入的 Recharger。
//
// 金额为正表示充值（会生成充值单并归集当期账单），为负表示扣减（人工调账）。
// 详情页的「调整余额」需要双向能力，原先固定 direction=1，扣减只能靠负数的
// 语义歧义绕过去。
func (s *userService) Recharge(ctx context.Context, id uint64, amount float64, remark string, operatorID uint64) (*RechargeResult, error) {
	if amount == 0 {
		return nil, ErrInvalidAdjustAmount
	}
	if s.recharge == nil {
		return nil, errors.New("充值能力未配置")
	}
	return s.recharge(ctx, id, amount, remark, operatorID)
}
