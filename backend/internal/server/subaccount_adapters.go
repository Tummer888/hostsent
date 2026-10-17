package server

import (
	"context"
	"errors"

	accountservice "hostsent/backend/internal/modules/admin/user/account/service"
	memberdto "hostsent/backend/internal/modules/uc/member/dto"
	memberservice "hostsent/backend/internal/modules/uc/member/service"
	appauth "hostsent/backend/internal/pkg/auth"
)

// subAccountManagerAdapter 把客户侧成员服务适配成管理端的 SubAccountManager。
//
// 为什么走适配器而不是让 admin 直接 import uc/member：
// 本仓库的分层约定是「模块之间不互相 import，跨域能力由装配层适配」（CONTRIBUTING §2/§3）。
// 这样 admin 侧只依赖自己声明的端口，uc 侧改动不会把依赖传染进管理端。
//
// 为什么要复用同一份实现而不是管理端另写一套：成员业务的规则（权限枚举过滤、
// 等级配额、唯一性校验、改密撤会话）必须与客户侧逐字一致。各写一份必然漂移 ——
// 客户侧改了口径而管理端没跟上，就会出现「主账号建不了的成员、客服能建」这种
// 只在事故里才被发现的差异。
//
// 唯一的语义改写是归属判定：客户侧用登录主账号的 ID，管理端用路径上的主账号 ID。
// 这是安全的 —— 能否操作某个主账号由管理端的 user:update 权限码决定，
// 而适配器这一层不再引入第二条归属规则。
type subAccountManagerAdapter struct {
	members memberservice.MemberService
}

// newSubAccountManagerAdapter 构造适配器。
//
// members 为 nil 时返回的适配器会在每次调用返回「能力未配置」而不是 panic：
// 装配顺序错了（在 memberService 构造前接线）应当表现为明确报错，
// 而不是一个空 body 的 500 —— 后者要翻栈才能定位（实测踩过一次）。
func newSubAccountManagerAdapter(members memberservice.MemberService) accountservice.SubAccountManager {
	return &subAccountManagerAdapter{members: members}
}

// errOwnerIsMember 目标账号本身是子账号：它没有自己的成员体系，不能再建下级。
var errOwnerIsMember = errors.New("目标账号是子账号，不能在其下创建成员")

func (a *subAccountManagerAdapter) ListMembers(ctx context.Context, ownerID uint64) (*accountservice.SubAccountMemberList, error) {
	if a == nil || a.members == nil {
		return nil, accountservice.ErrSubAccountUnavailable
	}
	// isSub=false：管理端始终以「主账号视角」查询，归属由 ownerID 决定。
	// 服务层的 isSub 参数是给客户侧自助入口做越权拦截用的，管理端不适用。
	resp, err := a.members.List(ctx, ownerID, false, memberdto.MemberListQuery{Page: 1, PageSize: 100})
	if err != nil {
		return nil, translateMemberErr(err)
	}
	out := &accountservice.SubAccountMemberList{
		Members:        make([]accountservice.SubAccountMember, 0, len(resp.Items)),
		MaxSubAccounts: resp.MaxSubAccounts,
	}
	for _, it := range resp.Items {
		out.Members = append(out.Members, toSubAccountMember(it))
	}
	return out, nil
}

func (a *subAccountManagerAdapter) CreateMember(ctx context.Context, ownerID uint64, req accountservice.SubAccountCreateInput) (*accountservice.SubAccountMember, string, error) {
	if a == nil || a.members == nil {
		return nil, "", accountservice.ErrSubAccountUnavailable
	}
	resp, err := a.members.Create(ctx, ownerID, false, memberdto.CreateRequest{
		Username:    req.Username,
		Email:       req.Email,
		Phone:       req.Phone,
		Password:    req.Password,
		Remark:      req.Remark,
		Permissions: req.Permissions,
	})
	if err != nil {
		return nil, "", translateMemberErr(err)
	}
	member := toSubAccountMember(resp.Member)
	return &member, resp.Password, nil
}

func (a *subAccountManagerAdapter) UpdateMember(ctx context.Context, ownerID, memberID uint64, req accountservice.SubAccountUpdateInput) (*accountservice.SubAccountMember, error) {
	if a == nil || a.members == nil {
		return nil, accountservice.ErrSubAccountUnavailable
	}
	info, err := a.members.Update(ctx, ownerID, false, memberID, memberdto.UpdateRequest{
		Remark: req.Remark,
		Status: req.Status,
	})
	if err != nil {
		return nil, translateMemberErr(err)
	}
	member := toSubAccountMember(*info)
	return &member, nil
}

func (a *subAccountManagerAdapter) SetMemberPermissions(ctx context.Context, ownerID, memberID uint64, codes []string) (*accountservice.SubAccountMember, error) {
	if a == nil || a.members == nil {
		return nil, accountservice.ErrSubAccountUnavailable
	}
	info, err := a.members.SetPermissions(ctx, ownerID, false, memberID, codes)
	if err != nil {
		return nil, translateMemberErr(err)
	}
	member := toSubAccountMember(*info)
	return &member, nil
}

func (a *subAccountManagerAdapter) ResetMemberPassword(ctx context.Context, ownerID, memberID uint64, password string) (string, int, error) {
	if a == nil || a.members == nil {
		return "", 0, accountservice.ErrSubAccountUnavailable
	}
	resp, err := a.members.ResetPassword(ctx, ownerID, false, memberID, memberdto.ResetPasswordRequest{Password: password})
	if err != nil {
		return "", 0, translateMemberErr(err)
	}
	return resp.Password, resp.RevokedSessions, nil
}

func (a *subAccountManagerAdapter) DisableMember(ctx context.Context, ownerID, memberID uint64) error {
	if a == nil || a.members == nil {
		return accountservice.ErrSubAccountUnavailable
	}
	return translateMemberErr(a.members.Delete(ctx, ownerID, false, memberID))
}

func (a *subAccountManagerAdapter) MemberLogs(ctx context.Context, ownerID, memberID uint64, page, pageSize int) (*accountservice.SubAccountLogPage, error) {
	if a == nil || a.members == nil {
		return nil, accountservice.ErrSubAccountUnavailable
	}
	resp, err := a.members.ListLogs(ctx, ownerID, false, memberID, page, pageSize)
	if err != nil {
		return nil, translateMemberErr(err)
	}
	out := &accountservice.SubAccountLogPage{
		Items: make([]accountservice.SubAccountLogItem, 0, len(resp.Items)),
		Total: resp.Meta.Total,
	}
	for _, it := range resp.Items {
		out.Items = append(out.Items, accountservice.SubAccountLogItem{
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
	return out, nil
}

// PermissionOptions 可选客户侧权限项。
//
// 直接从 pkg/auth 的枚举构建，而不是向 uc 服务再要一次：权限码是**编译期固定枚举**，
// 两处渲染同一份常量才是真正同源；若经由服务层再取，就多了一次「服务返回什么」
// 的中间环节，反而更容易漂移。
func (a *subAccountManagerAdapter) PermissionOptions() []accountservice.SubAccountPermissionOption {
	if a == nil {
		return nil
	}
	opts := make([]accountservice.SubAccountPermissionOption, 0, len(appauth.UserPermissionAll))
	for _, code := range appauth.UserPermissionAll {
		opts = append(opts, accountservice.SubAccountPermissionOption{
			Code:  code,
			Label: appauth.UserPermissionLabels[code],
		})
	}
	return opts
}

// toSubAccountMember 客户侧成员 DTO → 端口结构。
func toSubAccountMember(in memberdto.Info) accountservice.SubAccountMember {
	return accountservice.SubAccountMember{
		ID:          in.ID,
		Username:    in.Username,
		Name:        in.Name,
		Email:       in.Email,
		Phone:       in.Phone,
		Remark:      in.Remark,
		Status:      in.Status,
		Permissions: in.Permissions,
		CreatedAt:   in.CreatedAt,
		LastLoginAt: in.LastLoginAt,
	}
}

// translateMemberErr 把客户侧成员服务的哨兵错误翻译成管理端哨兵。
//
// 必须翻译而不是直接透传：handler 只允许依赖本域 service 的哨兵做状态码映射
// （CONTRIBUTING §3）。透传 uc 的错误会让管理端的错误分支永远匹配不上，
// 全部落进 default 变成 50001「服务器错误」，运营看不出是配额满了还是成员不存在。
func translateMemberErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, memberservice.ErrMemberNotFound):
		return accountservice.ErrSubAccountNotFound
	case errors.Is(err, memberservice.ErrNotAccountOwner):
		// 管理端不该出现这个分支（isSub 恒为 false）；出现即装配/调用有误。
		return errOwnerIsMember
	case errors.Is(err, memberservice.ErrMemberQuotaExceeded):
		return accountservice.ErrSubAccountQuotaExceeded
	case errors.Is(err, memberservice.ErrInvalidStatus):
		return accountservice.ErrSubAccountInvalidStatus
	case errors.Is(err, memberservice.ErrPasswordTooShort):
		return accountservice.ErrSubAccountPasswordTooShort
	case errors.Is(err, memberservice.ErrUsernameTaken):
		return accountservice.ErrUsernameTaken
	case errors.Is(err, memberservice.ErrEmailTaken):
		return accountservice.ErrEmailTaken
	default:
		return err
	}
}

// 自检：端口契约一旦变化，这里会先编译失败，避免适配器静默漏实现某个方法。
var _ accountservice.SubAccountManager = (*subAccountManagerAdapter)(nil)
