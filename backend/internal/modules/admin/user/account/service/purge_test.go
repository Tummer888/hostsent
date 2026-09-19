package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
	"hostsent/backend/internal/modules/admin/user/account/repository"
)

// fakeUserRepo 只实现注销/恢复/清理链路用到的仓储方法；其余方法被调用即返回
// errUnexpectedRepoCall，避免测试因「顺手调了别的方法」而静默通过。
type fakeUserRepo struct {
	user *model.User
	row  *repository.DeletionCheckRow

	softDeleteCalls int
	softDeleteArg   struct {
		operatorID   uint64
		reason       string
		statusBefore string
		now          time.Time
	}
	restoreStatus string
	purged        []uint64
	purgeErr      error
	candidates    []model.User
}

var errUnexpectedRepoCall = errors.New("测试未预期的仓储调用")

func (f *fakeUserRepo) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	if f.user == nil || f.user.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	copied := *f.user
	return &copied, nil
}

func (f *fakeUserRepo) DeletionCheck(ctx context.Context, id uint64) (*repository.DeletionCheckRow, error) {
	if f.row == nil {
		return &repository.DeletionCheckRow{}, nil
	}
	return f.row, nil
}

func (f *fakeUserRepo) SoftDelete(ctx context.Context, id uint64, operatorID uint64, reason, statusBefore string, now time.Time) error {
	f.softDeleteCalls++
	f.softDeleteArg.operatorID = operatorID
	f.softDeleteArg.reason = reason
	f.softDeleteArg.statusBefore = statusBefore
	f.softDeleteArg.now = now
	f.user.DeletedAt = &now
	f.user.Status = model.StatusCancelled
	f.user.StatusBeforeDelete = statusBefore
	return nil
}

func (f *fakeUserRepo) Restore(ctx context.Context, id uint64, restoreStatus string) error {
	f.restoreStatus = restoreStatus
	f.user.DeletedAt = nil
	f.user.Status = restoreStatus
	return nil
}

func (f *fakeUserRepo) ListPurgeCandidates(ctx context.Context, before time.Time, limit int) ([]model.User, error) {
	out := make([]model.User, 0, len(f.candidates))
	for _, u := range f.candidates {
		if u.DeletedAt != nil && u.DeletedAt.Before(before) {
			out = append(out, u)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeUserRepo) PurgeUser(ctx context.Context, id uint64) error {
	if f.purgeErr != nil {
		return f.purgeErr
	}
	f.purged = append(f.purged, id)
	return nil
}

// —— 以下方法注销链路不应触达 ——

func (f *fakeUserRepo) Create(context.Context, *model.User) error { return errUnexpectedRepoCall }
func (f *fakeUserRepo) Update(context.Context, *model.User) error { return errUnexpectedRepoCall }
func (f *fakeUserRepo) Delete(context.Context, uint64) error      { return errUnexpectedRepoCall }
func (f *fakeUserRepo) FindByUsername(context.Context, string) (*model.User, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) List(context.Context, dto.UserListQuery) ([]model.User, int64, error) {
	return nil, 0, errUnexpectedRepoCall
}
func (f *fakeUserRepo) UpdateStatus(context.Context, uint64, string) error {
	return errUnexpectedRepoCall
}
func (f *fakeUserRepo) UpdatePassword(context.Context, uint64, string) error {
	return errUnexpectedRepoCall
}
func (f *fakeUserRepo) GetRoles(context.Context, uint64) ([]model.Role, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) RolesByUserIDs(context.Context, []uint64) (map[uint64][]model.Role, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) SetRoles(context.Context, uint64, []uint64) error {
	return errUnexpectedRepoCall
}
func (f *fakeUserRepo) Stats(context.Context) (*model.UserStats, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) RegionStats(context.Context) ([]model.RegionStat, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) UpdateLoginProfile(context.Context, uint64, string, string, time.Time) error {
	return errUnexpectedRepoCall
}
func (f *fakeUserRepo) NamesByIDs(context.Context, []uint64) (map[uint64]string, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) ListSubAccounts(context.Context, uint64) ([]model.User, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) PermissionsByUserIDs(context.Context, []uint64) (map[uint64][]string, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) OAuthProvidersByUserIDs(context.Context, []uint64) (map[uint64][]string, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeUserRepo) CountDeleted(context.Context) (int64, error) { return 0, errUnexpectedRepoCall }
func (f *fakeUserRepo) AdminNamesByIDs(context.Context, []uint64) (map[uint64]string, error) {
	return nil, errUnexpectedRepoCall
}

// cfgReader 返回固定留存期，便于断言 cutoff 计算。
func cfgReader(days int) ConfigIntReader {
	return func(ctx context.Context, key string, fallback int) int {
		if key == "user.deletion_retention_days" {
			return days
		}
		return fallback
	}
}

func activeUser() *model.User {
	return &model.User{ID: 190, Username: "probe", Status: model.StatusActive}
}

func newDeletionSvc(repo *fakeUserRepo, days int) *deletionService {
	svc := NewDeletionService(repo, cfgReader(days)).(*deletionService)
	// 固定时钟：留存期 cutoff 的断言不能随真实时间漂移。
	svc.now = func() time.Time { return time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC) }
	return svc
}

// 阻断项（在管实例）优先级高于警告项：即使 force=true 也必须拒绝。
func TestCheck_InstancesBlockDeletion(t *testing.T) {
	repo := &fakeUserRepo{user: activeUser(), row: &repository.DeletionCheckRow{
		Instances: 2, BalanceNonZero: 1, OpenTickets: 1,
	}}
	svc := newDeletionSvc(repo, 180)

	check, err := svc.Check(context.Background(), 190)
	if err != nil {
		t.Fatalf("前置校验失败: %v", err)
	}
	if check.CanDelete {
		t.Fatal("存在在管实例时 can_delete 必须为 false")
	}
	if len(check.Blockers) != 1 || check.Blockers[0].Code != "instances" {
		t.Fatalf("应只有 instances 一个阻断项，实际 %+v", check.Blockers)
	}
	if len(check.Warnings) != 2 {
		t.Fatalf("余额与工单应为警告项，实际 %+v", check.Warnings)
	}

	err = svc.SoftDelete(context.Background(), 190, "测试", true, 1)
	if err == nil {
		t.Fatal("force=true 也不能绕过在管实例阻断")
	}
	if !errors.Is(err, ErrDeletionBlocked) {
		t.Fatalf("错误应为 ErrDeletionBlocked，实际: %v", err)
	}
	if repo.softDeleteCalls != 0 {
		t.Fatal("被阻断时不应写库")
	}
}

// 警告项：force=false 拒绝，force=true 放行（让操作人显式确认）。
func TestSoftDelete_WarningsNeedForce(t *testing.T) {
	repo := &fakeUserRepo{user: activeUser(), row: &repository.DeletionCheckRow{BalanceNonZero: 1}}
	svc := newDeletionSvc(repo, 180)

	err := svc.SoftDelete(context.Background(), 190, "测试", false, 1)
	if err == nil {
		t.Fatal("存在警告项且未 force 时必须拒绝")
	}
	if !errors.Is(err, ErrDeletionNeedsForce) {
		t.Fatalf("错误应为 ErrDeletionNeedsForce，实际: %v", err)
	}
	if repo.softDeleteCalls != 0 {
		t.Fatal("未确认时不应写库")
	}

	if err := svc.SoftDelete(context.Background(), 190, "测试", true, 63); err != nil {
		t.Fatalf("force=true 应放行: %v", err)
	}
	if repo.softDeleteArg.operatorID != 63 {
		t.Fatalf("注销操作人必须来自令牌（63），实际 %d", repo.softDeleteArg.operatorID)
	}
	if repo.softDeleteArg.statusBefore != model.StatusActive {
		t.Fatalf("应记录注销前状态，实际 %q", repo.softDeleteArg.statusBefore)
	}
}

// 操作人缺失必须拒绝：注销是高危动作，留痕缺失就无法追责（doc104 F5 的同款要求）。
func TestSoftDelete_RequiresOperator(t *testing.T) {
	repo := &fakeUserRepo{user: activeUser(), row: &repository.DeletionCheckRow{}}
	svc := newDeletionSvc(repo, 180)

	err := svc.SoftDelete(context.Background(), 190, "测试", false, 0)
	if err == nil {
		t.Fatal("operatorID=0 必须被拒")
	}
	if !errors.Is(err, ErrInvalidOperator) {
		t.Fatalf("错误应为 ErrInvalidOperator，实际: %v", err)
	}
}

// 注销原因必填：没有原因的注销在审计里等于没发生。
func TestSoftDelete_RequiresReason(t *testing.T) {
	repo := &fakeUserRepo{user: activeUser(), row: &repository.DeletionCheckRow{}}
	svc := newDeletionSvc(repo, 180)

	if err := svc.SoftDelete(context.Background(), 190, "   ", false, 1); err == nil {
		t.Fatal("注销原因为空必须被拒")
	}
}

// 重复注销必须被拒，而不是把 deleted_at 与 status_before_delete 覆盖一遍。
func TestSoftDelete_RejectsAlreadyDeleted(t *testing.T) {
	repo := &fakeUserRepo{user: activeUser(), row: &repository.DeletionCheckRow{}}
	svc := newDeletionSvc(repo, 180)
	now := time.Now()
	repo.user.DeletedAt = &now

	err := svc.SoftDelete(context.Background(), 190, "测试", true, 1)
	if err == nil {
		t.Fatal("对已注销用户再次注销必须被拒")
	}
	if !errors.Is(err, ErrUserDeleted) {
		t.Fatalf("错误应为 ErrUserDeleted，实际: %v", err)
	}
}

// 状态收敛：注销 = deleted_at 非空 + status=cancelled，且记录注销前状态。
func TestSoftDelete_ConvergesStatus(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: 190, Username: "p", Status: model.StatusDisabled}, row: &repository.DeletionCheckRow{}}
	svc := newDeletionSvc(repo, 180)

	if err := svc.SoftDelete(context.Background(), 190, "用户申请", true, 1); err != nil {
		t.Fatalf("注销失败: %v", err)
	}
	if repo.user.DeletedAt == nil {
		t.Fatal("注销必须写 deleted_at")
	}
	if repo.user.Status != model.StatusCancelled {
		t.Fatalf("注销后状态应收敛为 cancelled，实际 %s", repo.user.Status)
	}
	if repo.user.StatusBeforeDelete != model.StatusDisabled {
		t.Fatalf("应记录注销前状态 disabled，实际 %s", repo.user.StatusBeforeDelete)
	}
}

// 恢复必须还原到 status_before_delete，而不是一律 active（否则等于顺手解封）。
func TestRestore_UsesStatusBeforeDelete(t *testing.T) {
	repo := &fakeUserRepo{user: activeUser(), row: &repository.DeletionCheckRow{}}
	svc := newDeletionSvc(repo, 180)
	now := time.Now()
	repo.user.DeletedAt = &now
	repo.user.Status = model.StatusCancelled
	repo.user.StatusBeforeDelete = model.StatusDisabled

	if err := svc.Restore(context.Background(), 190); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	if repo.restoreStatus != model.StatusDisabled {
		t.Fatalf("应还原为 disabled，实际 %s", repo.restoreStatus)
	}
}

// 快照缺失或为 cancelled 时回落 disabled：恢复成 cancelled 等于没恢复。
func TestRestore_FallsBackToDisabled(t *testing.T) {
	for _, before := range []string{"", model.StatusCancelled} {
		repo := &fakeUserRepo{user: activeUser()}
		svc := newDeletionSvc(repo, 180)
		now := time.Now()
		repo.user.DeletedAt = &now
		repo.user.StatusBeforeDelete = before

		if err := svc.Restore(context.Background(), 190); err != nil {
			t.Fatalf("恢复失败: %v", err)
		}
		if repo.restoreStatus != model.StatusDisabled {
			t.Fatalf("快照为 %q 时应回落 disabled，实际 %s", before, repo.restoreStatus)
		}
	}
}

// 恢复一个没注销的用户必须被拒（否则会把正常用户的状态改成 disabled）。
func TestRestore_RejectsNotDeleted(t *testing.T) {
	repo := &fakeUserRepo{user: activeUser()}
	svc := newDeletionSvc(repo, 180)

	err := svc.Restore(context.Background(), 190)
	if err == nil {
		t.Fatal("未注销的用户不应允许恢复")
	}
	if !errors.Is(err, ErrUserNotDeleted) {
		t.Fatalf("错误应为 ErrUserNotDeleted，实际: %v", err)
	}
}

// 留存期 cutoff：早于 now-retentionDays 注销的用户才进入清理范围。
func TestPurge_CutoffAndDryRun(t *testing.T) {
	old := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // 早于 cutoff（2026-03-22）
	recent := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeUserRepo{candidates: []model.User{
		{ID: 1, Username: "old", DeletedAt: &old},
		{ID: 2, Username: "recent", DeletedAt: &recent},
	}}
	svc := newDeletionSvc(repo, 180)

	resp, err := svc.Purge(context.Background(), true, 0)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if resp.RetentionDays != 180 {
		t.Fatalf("留存天数应为 180，实际 %d", resp.RetentionDays)
	}
	if !resp.Cutoff.Equal(time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("cutoff 计算不符: %s", resp.Cutoff)
	}
	if len(resp.Candidates) != 1 || resp.Candidates[0].ID != 1 {
		t.Fatalf("只应列出留存期已过的用户，实际 %+v", resp.Candidates)
	}
	if resp.Purged != 0 {
		t.Fatal("dry_run 必须一个字节都不写")
	}
	if len(repo.purged) != 0 {
		t.Fatal("dry_run 不应调用 PurgeUser")
	}
}

// 留存期配成 0 或负数意味着「立刻硬删除」，属于配置事故，必须回落默认值。
func TestPurge_InvalidRetentionFallsBack(t *testing.T) {
	for _, days := range []int{0, -5} {
		repo := &fakeUserRepo{}
		svc := newDeletionSvc(repo, days)
		resp, err := svc.Purge(context.Background(), true, 0)
		if err != nil {
			t.Fatalf("清理失败: %v", err)
		}
		if resp.RetentionDays != DefaultDeletionRetentionDays {
			t.Fatalf("留存期 %d 应回落默认 %d，实际 %d", days, DefaultDeletionRetentionDays, resp.RetentionDays)
		}
	}
}

// 单轮上限：一次手工触发不能把库锁死；超出上限时给出 has_more 提示。
func TestPurge_LimitAndHasMore(t *testing.T) {
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var users []model.User
	for i := uint64(1); i <= 5; i++ {
		d := old
		users = append(users, model.User{ID: i, Username: "u", DeletedAt: &d})
	}
	repo := &fakeUserRepo{candidates: users}
	svc := newDeletionSvc(repo, 180)

	resp, err := svc.Purge(context.Background(), false, 3)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if resp.Purged != 3 {
		t.Fatalf("本轮应清理 3 个，实际 %d", resp.Purged)
	}
	if !resp.HasMore {
		t.Fatal("取满上限时应提示仍有积压")
	}

	// 超过硬上限时必须被夹到 maxPurgeBatch，防止一次删十万行。
	repo2 := &fakeUserRepo{candidates: users}
	svc2 := newDeletionSvc(repo2, 180)
	resp2, err := svc2.Purge(context.Background(), true, 100000)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if len(resp2.Candidates) > maxPurgeBatch {
		t.Fatalf("单轮不得超过 %d，实际 %d", maxPurgeBatch, len(resp2.Candidates))
	}
}

// 单个用户清理失败（例如仍有在管实例）时跳过并记录，不中断整轮。
func TestPurge_SkipsFailuresAndContinues(t *testing.T) {
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d1, d2 := old, old
	repo := &fakeUserRepo{
		candidates: []model.User{
			{ID: 1, Username: "blocked", DeletedAt: &d1},
			{ID: 2, Username: "ok", DeletedAt: &d2},
		},
		purgeErr: repository.ErrUserHasInstances,
	}
	svc := newDeletionSvc(repo, 180)

	resp, err := svc.Purge(context.Background(), false, 0)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if resp.Purged != 0 {
		t.Fatalf("全部失败时 purged 应为 0，实际 %d", resp.Purged)
	}
	if len(resp.Skipped) != 2 {
		t.Fatalf("两个都应被跳过并记录，实际 %+v", resp.Skipped)
	}
	if resp.Skipped[0].Reason != "存在未释放的实例" {
		t.Fatalf("跳过原因应为可读文案，实际 %q", resp.Skipped[0].Reason)
	}
}

// 批量注销逐条独立：一个失败不该让其余一起失败，且重复 ID 只处理一次。
func TestBatchSoftDelete_IndependentAndDeduped(t *testing.T) {
	repo := &fakeUserRepo{user: activeUser(), row: &repository.DeletionCheckRow{BalanceNonZero: 1}}
	svc := newDeletionSvc(repo, 180)

	res, err := svc.BatchSoftDelete(context.Background(), []uint64{190, 190, 0, 191}, "测试", false, 1)
	if err != nil {
		t.Fatalf("批量注销失败: %v", err)
	}
	// 190 有警告项且未 force → 跳过；191 不存在 → 跳过；0 与重复项被剔除。
	if res.Affected != 0 {
		t.Fatalf("affected 应为 0，实际 %d", res.Affected)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("应记录 2 条跳过（190 与 191），实际 %+v", res.Skipped)
	}
	if res.Skipped[0].ID != 190 {
		t.Fatalf("跳过项应去重后按序返回，实际 %+v", res.Skipped)
	}
}

// 配置解析：非法值一律回落 fallback（留存期是合规参数，不能被脏配置打穿）。
func TestParseRetentionDays(t *testing.T) {
	if got := ParseRetentionDays("90", 180); got != 90 {
		t.Fatalf("应解析出 90，实际 %d", got)
	}
	for _, raw := range []string{"", "abc", "0", "-1", "  "} {
		if got := ParseRetentionDays(raw, 180); got != 180 {
			t.Fatalf("%q 应回落 180，实际 %d", raw, got)
		}
	}
}
