package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/member/dto"
	"hostsent/backend/internal/modules/uc/member/model"
)

// fakeMemberRepo 内存版成员仓储，覆盖成员管理服务的核心分支。
type fakeMemberRepo struct {
	members     map[uint64]*model.Member
	permissions map[uint64][]string
	maxSubs     int
	nextID      uint64
}

func newFakeMemberRepo() *fakeMemberRepo {
	return &fakeMemberRepo{
		members:     map[uint64]*model.Member{},
		permissions: map[uint64][]string{},
		nextID:      100,
	}
}

func (f *fakeMemberRepo) List(_ context.Context, ownerID uint64, _ dto.MemberListQuery) ([]model.Member, int64, error) {
	var items []model.Member
	for _, m := range f.members {
		if m.OwnerUserID != nil && *m.OwnerUserID == ownerID && m.IsSubAccount {
			items = append(items, *m)
		}
	}
	return items, int64(len(items)), nil
}

func (f *fakeMemberRepo) FindByID(_ context.Context, id uint64) (*model.Member, error) {
	if m, ok := f.members[id]; ok {
		snapshot := *m
		return &snapshot, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMemberRepo) FindByUsername(_ context.Context, username string) (*model.Member, error) {
	for _, m := range f.members {
		if m.Username == username {
			return m, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMemberRepo) FindByEmail(_ context.Context, email string) (*model.Member, error) {
	for _, m := range f.members {
		if m.Email == email {
			return m, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMemberRepo) FindByPhone(_ context.Context, phone string) (*model.Member, error) {
	if phone == "" {
		return nil, gorm.ErrRecordNotFound
	}
	for _, m := range f.members {
		if m.Phone == phone {
			return m, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeMemberRepo) Create(_ context.Context, member *model.Member) error {
	f.nextID++
	member.ID = f.nextID
	snapshot := *member
	f.members[member.ID] = &snapshot
	return nil
}

func (f *fakeMemberRepo) UpdateProfileFields(_ context.Context, id uint64, fields map[string]any) error {
	m, ok := f.members[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	if remark, ok := fields["sub_account_remark"].(string); ok {
		m.SubAccountRemark = remark
	}
	if status, ok := fields["status"].(string); ok {
		m.Status = status
	}
	return nil
}

func (f *fakeMemberRepo) UpdatePasswordHash(_ context.Context, id uint64, passwordHash string) error {
	m, ok := f.members[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	m.PasswordHash = passwordHash
	return nil
}

func (f *fakeMemberRepo) CountSubAccounts(_ context.Context, ownerID uint64) (int64, error) {
	var count int64
	for _, m := range f.members {
		if m.OwnerUserID != nil && *m.OwnerUserID == ownerID && m.IsSubAccount {
			count++
		}
	}
	return count, nil
}

func (f *fakeMemberRepo) MaxSubAccountsOf(_ context.Context, _ uint64) (int, error) {
	return f.maxSubs, nil
}

func (f *fakeMemberRepo) PermissionsOf(_ context.Context, userID uint64) ([]string, error) {
	return f.permissions[userID], nil
}

func (f *fakeMemberRepo) ReplacePermissions(_ context.Context, userID uint64, codes []string) error {
	f.permissions[userID] = codes
	return nil
}

func (f *fakeMemberRepo) CreateOperationLog(_ context.Context, _ *model.OperationLog) error {
	return nil
}

func (f *fakeMemberRepo) ListOperationLogs(_ context.Context, _ uint64, _, _ int) ([]model.OperationLog, int64, error) {
	return nil, 0, nil
}

func newTestMember(id, ownerID uint64) *model.Member {
	owner := ownerID
	return &model.Member{
		ID:               id,
		Username:         "member",
		Email:            "member@example.com",
		Status:           "active",
		OwnerUserID:      &owner,
		IsSubAccount:     true,
		SubAccountRemark: "测试成员",
		CreatedAt:        time.Now(),
	}
}

// TestMemberQuotaLimit 超出 max_sub_accounts 上限时返回业务错误。
func TestMemberQuotaLimit(t *testing.T) {
	repo := newFakeMemberRepo()
	repo.maxSubs = 1

	svc := NewMemberService(repo)
	// 第 1 个成员：未达上限，应创建成功
	if _, err := svc.Create(context.Background(), 5, false, dto.CreateRequest{
		Username: "first",
		Email:    "first@example.com",
	}); err != nil {
		t.Fatalf("首个成员创建失败: %v", err)
	}

	// 第 2 个成员：已达上限，应被拒绝
	_, err := svc.Create(context.Background(), 5, false, dto.CreateRequest{
		Username: "second",
		Email:    "second@example.com",
	})
	if !errors.Is(err, ErrMemberQuotaExceeded) {
		t.Fatalf("超限创建 err = %v, want ErrMemberQuotaExceeded", err)
	}

	// max=0 表示未配置上限，不拦截
	repo.maxSubs = 0
	if _, err := svc.Create(context.Background(), 5, false, dto.CreateRequest{
		Username: "third",
		Email:    "third@example.com",
	}); err != nil {
		t.Fatalf("未配置上限时创建失败: %v", err)
	}
}

// TestMemberCreateAppliesDefaultPermissions 新建成员时按可授予白名单落权限，拒绝资金/实名码。
func TestMemberCreateAppliesDefaultPermissions(t *testing.T) {
	repo := newFakeMemberRepo()
	repo.maxSubs = 5
	svc := NewMemberService(repo)

	resp, err := svc.Create(context.Background(), 5, false, dto.CreateRequest{
		Username:    "ops",
		Email:       "ops@example.com",
		Permissions: []string{"instance:view", "billing:recharge", "realname:submit", "subaccount:manage"},
	})
	if err != nil {
		t.Fatalf("Create 返回错误: %v", err)
	}
	if resp.Password == "" {
		t.Fatal("未生成一次性密码")
	}
	got := repo.permissions[resp.Member.ID]
	if len(got) != 1 || got[0] != "instance:view" {
		t.Fatalf("落库权限 = %v, want [instance:view]（资金/实名/成员管理被过滤）", got)
	}
}

// TestSubAccountCannotReadOthersData 主账号只能操作自己名下的成员（越权关键点）。
func TestSubAccountCannotReadOthersData(t *testing.T) {
	repo := newFakeMemberRepo()
	repo.members[1] = newTestMember(1, 5)  // 归属账号 5
	repo.members[2] = newTestMember(2, 99) // 归属账号 99

	svc := NewMemberService(repo)

	// 账号 5 越权访问成员 2 → 视为不存在
	if _, err := svc.SetPermissions(context.Background(), 5, false, 2, []string{"instance:view"}); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("越权设置权限 err = %v, want ErrMemberNotFound", err)
	}
	if err := svc.Delete(context.Background(), 5, false, 2); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("越权删除 err = %v, want ErrMemberNotFound", err)
	}
	if _, err := svc.ListLogs(context.Background(), 5, false, 2, 1, 10); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("越权查日志 err = %v, want ErrMemberNotFound", err)
	}

	// 自己的成员正常放行
	if _, err := svc.SetPermissions(context.Background(), 5, false, 1, []string{"instance:view"}); err != nil {
		t.Fatalf("操作自己的成员失败: %v", err)
	}
	// 归属账号 99 的成员未被账号 5 改动
	if len(repo.permissions[2]) != 0 {
		t.Fatalf("他人成员权限被改动: %v", repo.permissions[2])
	}
}

// TestSubAccountCannotManageMembers 子账号不能管理成员（服务层二次校验）。
func TestSubAccountCannotManageMembers(t *testing.T) {
	repo := newFakeMemberRepo()
	svc := NewMemberService(repo)

	if _, err := svc.List(context.Background(), 5, true, dto.MemberListQuery{}); !errors.Is(err, ErrNotAccountOwner) {
		t.Fatalf("子账号列表 err = %v, want ErrNotAccountOwner", err)
	}
	if _, err := svc.Create(context.Background(), 5, true, dto.CreateRequest{Username: "x", Email: "x@example.com"}); !errors.Is(err, ErrNotAccountOwner) {
		t.Fatalf("子账号建成员 err = %v, want ErrNotAccountOwner", err)
	}
}

// TestMemberDeleteDisablesInsteadOfRemoving 删除成员走禁用（R5 只增不删），并清空权限。
func TestMemberDeleteDisablesInsteadOfRemoving(t *testing.T) {
	repo := newFakeMemberRepo()
	repo.members[1] = newTestMember(1, 5)
	repo.permissions[1] = []string{"instance:view"}

	svc := NewMemberService(repo)
	if err := svc.Delete(context.Background(), 5, false, 1); err != nil {
		t.Fatalf("Delete 返回错误: %v", err)
	}
	if repo.members[1] == nil {
		t.Fatal("成员记录被物理删除，应保留并置为 disabled")
	}
	if repo.members[1].Status != "disabled" {
		t.Fatalf("成员状态 = %s, want disabled", repo.members[1].Status)
	}
	if len(repo.permissions[1]) != 0 {
		t.Fatalf("删除后权限未清空: %v", repo.permissions[1])
	}
}

// —— 重置密码与撤会话（本轮补全）——

// fakeSessionRevoker 记录被撤销会话的用户，验证改密/禁用是否连带撤销登录态。
type fakeSessionRevoker struct {
	revoked map[uint64]int
}

func (f *fakeSessionRevoker) RevokeSessions(_ context.Context, id uint64, _ string) ([]string, error) {
	if f.revoked == nil {
		f.revoked = map[uint64]int{}
	}
	f.revoked[id]++
	return []string{"sess-" + itoa(id)}, nil
}

// fakeSessionInvalidator 记录缓存失效调用（应收到被撤销的 session_id）。
type fakeSessionInvalidator struct{ invalidated []string }

func (f *fakeSessionInvalidator) Invalidate(_ context.Context, sessionIDs ...string) {
	f.invalidated = append(f.invalidated, sessionIDs...)
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func newMemberWithSessions(t *testing.T) (*memberService, *fakeMemberRepo, *fakeSessionRevoker, *fakeSessionInvalidator) {
	t.Helper()
	repo := newFakeMemberRepo()
	svc := NewMemberService(repo).(*memberService)
	revoker := &fakeSessionRevoker{}
	invalidator := &fakeSessionInvalidator{}
	svc.SetSessionRevoker(revoker)
	svc.SetSessionInvalidator(invalidator)
	return svc, repo, revoker, invalidator
}

// TestResetPasswordGeneratesOneTimePasswordAndRevokesSessions 改密应生成一次性密码
// 并撤销该成员全部会话（旧密码泄露期间建立的登录态不能继续可用）。
func TestResetPasswordGeneratesOneTimePasswordAndRevokesSessions(t *testing.T) {
	svc, repo, revoker, invalidator := newMemberWithSessions(t)
	owner := uint64(1)
	member := &model.Member{ID: 200, Username: "sub_a", Email: "sub_a@example.com",
		Status: "active", OwnerUserID: &owner, IsSubAccount: true, PasswordHash: "old-hash"}
	repo.members[member.ID] = member

	resp, err := svc.ResetPassword(context.Background(), owner, false, member.ID, dto.ResetPasswordRequest{})
	if err != nil {
		t.Fatalf("重置密码失败: %v", err)
	}
	if len(resp.Password) != 14 {
		t.Fatalf("未指定密码时应生成 14 位一次性密码，实际 %q", resp.Password)
	}
	if repo.members[member.ID].PasswordHash == "old-hash" {
		t.Fatal("密码哈希未更新")
	}
	if revoker.revoked[member.ID] == 0 {
		t.Fatal("改密后应撤销该成员会话")
	}
	if len(invalidator.invalidated) == 0 {
		t.Fatal("撤销会话后应失效会话校验缓存")
	}
	if resp.RevokedSessions == 0 {
		t.Fatal("响应应回报被撤销的会话数")
	}
}

// TestResetPasswordRejectsShortPassword 显式指定的密码过短应被拒（不落库、不撤会话）。
func TestResetPasswordRejectsShortPassword(t *testing.T) {
	svc, repo, revoker, _ := newMemberWithSessions(t)
	owner := uint64(1)
	member := &model.Member{ID: 201, Username: "sub_b", Status: "active",
		OwnerUserID: &owner, IsSubAccount: true, PasswordHash: "old-hash"}
	repo.members[member.ID] = member

	_, err := svc.ResetPassword(context.Background(), owner, false, member.ID, dto.ResetPasswordRequest{Password: "123"})
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("短密码应返回 ErrPasswordTooShort，实际 %v", err)
	}
	if repo.members[member.ID].PasswordHash != "old-hash" {
		t.Fatal("被拒的请求不该改动密码")
	}
	if revoker.revoked[member.ID] != 0 {
		t.Fatal("被拒的请求不该撤销会话")
	}
}

// TestResetPasswordForOthersMemberDenied 主账号不能重置别人成员的密码（越权关键点）。
func TestResetPasswordForOthersMemberDenied(t *testing.T) {
	svc, repo, _, _ := newMemberWithSessions(t)
	owner := uint64(1)
	other := uint64(2)
	member := &model.Member{ID: 202, Username: "sub_c", Status: "active",
		OwnerUserID: &other, IsSubAccount: true, PasswordHash: "old-hash"}
	repo.members[member.ID] = member

	_, err := svc.ResetPassword(context.Background(), owner, false, member.ID, dto.ResetPasswordRequest{Password: "newpass123"})
	if !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("他人成员应视为不存在，实际 %v", err)
	}
}

// TestResetPasswordDeniedForSubAccount 子账号不能重置成员密码。
func TestResetPasswordDeniedForSubAccount(t *testing.T) {
	svc, repo, _, _ := newMemberWithSessions(t)
	owner := uint64(1)
	member := &model.Member{ID: 203, Username: "sub_d", Status: "active",
		OwnerUserID: &owner, IsSubAccount: true, PasswordHash: "old-hash"}
	repo.members[member.ID] = member

	_, err := svc.ResetPassword(context.Background(), owner, true, member.ID, dto.ResetPasswordRequest{Password: "newpass123"})
	if !errors.Is(err, ErrNotAccountOwner) {
		t.Fatalf("子账号应被拒，实际 %v", err)
	}
}

// TestDisableRevokesSessions 禁用成员应撤销其会话（否则「禁用」等于没生效）。
func TestDisableRevokesSessions(t *testing.T) {
	svc, repo, revoker, _ := newMemberWithSessions(t)
	owner := uint64(1)
	member := &model.Member{ID: 204, Username: "sub_e", Status: "active",
		OwnerUserID: &owner, IsSubAccount: true, PasswordHash: "h"}
	repo.members[member.ID] = member

	if err := svc.Delete(context.Background(), owner, false, member.ID); err != nil {
		t.Fatalf("禁用成员失败: %v", err)
	}
	if repo.members[member.ID].Status != "disabled" {
		t.Fatal("成员状态应为 disabled")
	}
	if revoker.revoked[member.ID] == 0 {
		t.Fatal("禁用成员后应撤销其会话")
	}
}

// TestUpdateStatusDisabledRevokesSessions 通过改状态禁用时同样撤会话。
func TestUpdateStatusDisabledRevokesSessions(t *testing.T) {
	svc, repo, revoker, _ := newMemberWithSessions(t)
	owner := uint64(1)
	member := &model.Member{ID: 205, Username: "sub_f", Status: "active",
		OwnerUserID: &owner, IsSubAccount: true, PasswordHash: "h"}
	repo.members[member.ID] = member

	if _, err := svc.Update(context.Background(), owner, false, member.ID, dto.UpdateRequest{Status: "disabled"}); err != nil {
		t.Fatalf("改状态失败: %v", err)
	}
	if revoker.revoked[member.ID] == 0 {
		t.Fatal("由启用改为禁用时应撤销会话")
	}
}
