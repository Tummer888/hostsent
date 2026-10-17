// Package service 提供用户中心成员（子账号）模块的业务编排。
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/member/dto"
	"hostsent/backend/internal/modules/uc/member/model"
	"hostsent/backend/internal/modules/uc/member/repository"
	appauth "hostsent/backend/internal/pkg/auth"
)

// MemberService 成员管理业务能力（P4-07）。
type MemberService interface {
	List(ctx context.Context, ownerID uint64, isSub bool, query dto.MemberListQuery) (*dto.ListResponse, error)
	Create(ctx context.Context, ownerID uint64, isSub bool, req dto.CreateRequest) (*dto.CreateResponse, error)
	Update(ctx context.Context, ownerID uint64, isSub bool, id uint64, req dto.UpdateRequest) (*dto.Info, error)
	SetPermissions(ctx context.Context, ownerID uint64, isSub bool, id uint64, codes []string) (*dto.Info, error)
	ResetPassword(ctx context.Context, ownerID uint64, isSub bool, id uint64, req dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error)
	Delete(ctx context.Context, ownerID uint64, isSub bool, id uint64) error
	ListLogs(ctx context.Context, ownerID uint64, isSub bool, id uint64, page, pageSize int) (*dto.OperationLogListResponse, error)
	// SetSessionRevoker 注入按用户撤销会话能力（可选，装配层调用）。
	SetSessionRevoker(revoker SessionRevoker)
	// SetSessionInvalidator 注入会话校验缓存失效能力（可选，装配层调用）。
	SetSessionInvalidator(invalidator SessionInvalidator)
	// HasSessionRevoker 装配自检：会话撤销能力是否已接线。
	HasSessionRevoker() bool
}

// SessionRevoker 按用户撤销其全部有效会话，返回被撤销的 session_id。
// 由 uc/auth 的 userRepository 实现（RevokeSessions），装配层注入。
type SessionRevoker interface {
	RevokeSessions(ctx context.Context, id uint64, reason string) ([]string, error)
}

// SessionInvalidator 让会话校验缓存立即失效（由 pkg/sessionguard.Guard 实现）。
type SessionInvalidator interface {
	Invalidate(ctx context.Context, sessionIDs ...string)
}

type memberService struct {
	repo        repository.MemberRepository
	revoker     SessionRevoker
	invalidator SessionInvalidator
}

// NewMemberService 创建成员业务服务。
func NewMemberService(repo repository.MemberRepository) MemberService {
	return &memberService{repo: repo}
}

// SetSessionRevoker 注入按用户撤销会话能力（可选，装配层调用）。
func (s *memberService) SetSessionRevoker(revoker SessionRevoker) {
	s.revoker = revoker
}

// SetSessionInvalidator 注入会话校验缓存失效能力（可选，装配层调用）。
func (s *memberService) SetSessionInvalidator(invalidator SessionInvalidator) {
	s.invalidator = invalidator
}

// HasSessionRevoker 装配自检：会话撤销能力是否已接线。
func (s *memberService) HasSessionRevoker() bool {
	return s.revoker != nil
}

// revokeMemberSessions 撤销某成员的全部有效会话并失效其校验缓存。
//
// 失败只吞掉不向上抛：调用点（禁用/删除/改密）的主动作已经完成，
// 而 sessionguard 的账号状态判定会兜住「已被禁用成员」的每一个请求。
// 返回撤销条数供调用方回显。
func (s *memberService) revokeMemberSessions(ctx context.Context, memberID uint64, reason string) int {
	if s.revoker == nil {
		return 0
	}
	sessionIDs, err := s.revoker.RevokeSessions(ctx, memberID, reason)
	if err != nil {
		return 0
	}
	if s.invalidator != nil && len(sessionIDs) > 0 {
		s.invalidator.Invalidate(ctx, sessionIDs...)
	}
	return len(sessionIDs)
}

func (s *memberService) List(ctx context.Context, ownerID uint64, isSub bool, query dto.MemberListQuery) (*dto.ListResponse, error) {
	if isSub {
		return nil, ErrNotAccountOwner
	}
	items, total, err := s.repo.List(ctx, ownerID, query)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalize(query.Page, query.PageSize)
	resp := &dto.ListResponse{
		Items:          make([]dto.Info, 0, len(items)),
		Meta:           dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
		Permissions:    permissionOptions(),
		MaxSubAccounts: 0,
	}
	for _, item := range items {
		info, err := s.toInfo(ctx, item)
		if err != nil {
			return nil, err
		}
		resp.Items = append(resp.Items, info)
	}
	if quota, err := s.repo.MaxSubAccountsOf(ctx, ownerID); err == nil {
		resp.MaxSubAccounts = quota
	}
	return resp, nil
}

func (s *memberService) Create(ctx context.Context, ownerID uint64, isSub bool, req dto.CreateRequest) (*dto.CreateResponse, error) {
	if isSub {
		return nil, ErrNotAccountOwner
	}

	// 等级上限校验（0 表示未配置上限，不拦截）
	if quota, err := s.repo.MaxSubAccountsOf(ctx, ownerID); err == nil && quota > 0 {
		count, cerr := s.repo.CountSubAccounts(ctx, ownerID)
		if cerr != nil {
			return nil, cerr
		}
		if count >= int64(quota) {
			return nil, ErrMemberQuotaExceeded
		}
	}

	// 唯一性校验（users 表的唯一索引是最终防线，这里给出友好错误）
	if _, err := s.repo.FindByUsername(ctx, req.Username); err == nil {
		return nil, ErrUsernameTaken
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if _, err := s.repo.FindByEmail(ctx, req.Email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if req.Phone != "" {
		if _, err := s.repo.FindByPhone(ctx, req.Phone); err == nil {
			return nil, ErrPhoneTaken
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// 密码：主账号未指定时生成一次性密码，仅在本次响应返回
	password := req.Password
	if password == "" {
		password = generatePassword()
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	owner := ownerID
	member := &model.Member{
		Username:         req.Username,
		Email:            req.Email,
		Phone:            req.Phone,
		PasswordHash:     string(hash),
		Status:           "active",
		OwnerUserID:      &owner,
		IsSubAccount:     true,
		SubAccountRemark: req.Remark,
	}
	if err := s.repo.Create(ctx, member); err != nil {
		return nil, err
	}

	if err := s.repo.ReplacePermissions(ctx, member.ID, grantablePermissions(req.Permissions)); err != nil {
		return nil, err
	}

	info, err := s.toInfo(ctx, *member)
	if err != nil {
		return nil, err
	}
	return &dto.CreateResponse{Member: info, Password: password}, nil
}

func (s *memberService) Update(ctx context.Context, ownerID uint64, isSub bool, id uint64, req dto.UpdateRequest) (*dto.Info, error) {
	if isSub {
		return nil, ErrNotAccountOwner
	}
	member, err := s.findOwnedMember(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{"sub_account_remark": req.Remark}
	if req.Status != "" {
		if req.Status != "active" && req.Status != "disabled" {
			return nil, ErrInvalidStatus
		}
		fields["status"] = req.Status
	}
	if len(fields) > 0 {
		if err := s.repo.UpdateProfileFields(ctx, member.ID, fields); err != nil {
			return nil, err
		}
		// 由启用改为禁用 → 连带撤销其会话，让「停用」立即生效。
		if req.Status == "disabled" && member.Status != "disabled" {
			s.revokeMemberSessions(ctx, member.ID, "member_disabled")
		}
		member, err = s.repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	info, err := s.toInfo(ctx, *member)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *memberService) SetPermissions(ctx context.Context, ownerID uint64, isSub bool, id uint64, codes []string) (*dto.Info, error) {
	if isSub {
		return nil, ErrNotAccountOwner
	}
	member, err := s.findOwnedMember(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplacePermissions(ctx, member.ID, grantablePermissions(codes)); err != nil {
		return nil, err
	}
	info, err := s.toInfo(ctx, *member)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// ResetPassword 重置成员密码（主账号或管理端代设）。
//
// 留空则生成一次性密码，仅在本次响应返回。改密后连带撤销该成员的全部有效会话：
// 代设密码意味着「账号已被接管」，旧密码期间建立的登录态不该继续可用。
func (s *memberService) ResetPassword(ctx context.Context, ownerID uint64, isSub bool, id uint64, req dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
	if isSub {
		return nil, ErrNotAccountOwner
	}
	member, err := s.findOwnedMember(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		password = generatePassword()
	}
	if len(password) < 6 {
		return nil, ErrPasswordTooShort
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdatePasswordHash(ctx, member.ID, string(hash)); err != nil {
		return nil, err
	}
	revoked := s.revokeMemberSessions(ctx, member.ID, "member_password_reset")
	return &dto.ResetPasswordResponse{Password: password, RevokedSessions: revoked}, nil
}

// Delete 删除成员：不能删主账号自己；成员名下资源归属主账号，无需迁移。
// 采用「禁用」而非物理删除（R5 只增不删），成员将无法再登录。
//
// 同时撤销其全部有效会话：禁用是「立即停用」，留着登录态等于没禁用
// （sessionguard 的账号状态判定虽有兜底，但显式撤销让禁用即时可见）。
func (s *memberService) Delete(ctx context.Context, ownerID uint64, isSub bool, id uint64) error {
	if isSub {
		return ErrNotAccountOwner
	}
	member, err := s.findOwnedMember(ctx, ownerID, id)
	if err != nil {
		return err
	}
	if member.ID == ownerID {
		return ErrMemberNotFound
	}
	if err := s.repo.UpdateProfileFields(ctx, member.ID, map[string]any{"status": "disabled"}); err != nil {
		return err
	}
	if err := s.repo.ReplacePermissions(ctx, member.ID, nil); err != nil {
		return err
	}
	s.revokeMemberSessions(ctx, member.ID, "member_disabled")
	return nil
}

func (s *memberService) ListLogs(ctx context.Context, ownerID uint64, isSub bool, id uint64, page, pageSize int) (*dto.OperationLogListResponse, error) {
	if isSub {
		return nil, ErrNotAccountOwner
	}
	if _, err := s.findOwnedMember(ctx, ownerID, id); err != nil {
		return nil, err
	}
	logs, total, err := s.repo.ListOperationLogs(ctx, id, page, pageSize)
	if err != nil {
		return nil, err
	}
	p, ps := normalize(page, pageSize)
	resp := &dto.OperationLogListResponse{
		Items: make([]dto.OperationLogInfo, 0, len(logs)),
		Meta:  dto.ListMeta{Page: p, PageSize: ps, Total: total},
	}
	for _, item := range logs {
		resp.Items = append(resp.Items, dto.OperationLogInfo{
			ID:        item.ID,
			ActorID:   item.ActorUserID,
			ActorName: item.ActorName,
			Module:    item.Module,
			Action:    item.Action,
			Target:    item.Target,
			Detail:    item.Detail,
			IP:        item.IP,
			CreatedAt: item.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return resp, nil
}

// findOwnedMember 取成员并校验归属，避免主账号操作他人成员（越权关键点）。
func (s *memberService) findOwnedMember(ctx context.Context, ownerID, id uint64) (*model.Member, error) {
	member, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	if !member.IsSubAccount || member.OwnerUserID == nil || *member.OwnerUserID != ownerID {
		return nil, ErrMemberNotFound
	}
	return member, nil
}

func (s *memberService) toInfo(ctx context.Context, member model.Member) (dto.Info, error) {
	codes, err := s.repo.PermissionsOf(ctx, member.ID)
	if err != nil {
		return dto.Info{}, err
	}
	name := member.RealName
	if name == "" {
		name = member.Username
	}
	return dto.Info{
		ID:          member.ID,
		Username:    member.Username,
		Name:        name,
		Email:       member.Email,
		Phone:       member.Phone,
		Remark:      member.SubAccountRemark,
		Status:      member.Status,
		Permissions: codes,
		CreatedAt:   member.CreatedAt,
		LastLoginAt: member.LastLoginAt,
	}, nil
}

// grantablePermissions 过滤出可授予子账号的权限码（拒绝资金/实名/成员管理）。
// 请求为空时回退为默认权限集，避免新建成员「零权限」无法使用。
func grantablePermissions(codes []string) []string {
	if len(codes) == 0 {
		return append([]string(nil), appauth.UserPermissionDefault...)
	}
	result := make([]string, 0, len(codes))
	seen := map[string]bool{}
	for _, code := range codes {
		if seen[code] || !appauth.IsGrantableUserPermission(code) {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	return result
}

func permissionOptions() []dto.PermissionOption {
	options := make([]dto.PermissionOption, 0, len(appauth.UserPermissionAll))
	for _, code := range appauth.UserPermissionAll {
		options = append(options, dto.PermissionOption{Code: code, Label: appauth.UserPermissionLabels[code]})
	}
	return options
}

const passwordAlphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"

// generatePassword 生成 14 位一次性密码（去除易混淆字符）。
func generatePassword() string {
	buf := make([]byte, 14)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordAlphabet))))
		if err != nil {
			// 熵源异常时回退到时间戳派生，保证仍可登录且不重复
			return time.Now().Format("20060102150405")
		}
		buf[i] = passwordAlphabet[n.Int64()]
	}
	return string(buf)
}

func normalize(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
