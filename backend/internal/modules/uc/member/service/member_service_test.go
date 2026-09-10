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
		copy := *m
		return &copy, nil
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
	copy := *member
	f.members[member.ID] = &copy
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
