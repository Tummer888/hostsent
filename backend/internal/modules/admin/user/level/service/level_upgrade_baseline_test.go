package service

// 回归基线 R7（doc104 §2 表）：消费升级只升不降，退款不回退 total_consume_amount。
//
// 这是**设计而非缺陷**，所以它特别容易被后来者「顺手修好」：看到退款后累计消费
// 没减，很自然就想补一句 AddUserConsume(-amount)。但那样做会连带把等级降下来，
// 而 user_level_change_logs 里已经写了「某月升到 pro」的历史记录——等级是权益
// 授予的依据，撤销权益要有人为动作与留痕，不能由一次退款静默触发。
//
// 本轮新增的硬删除清理同样不得触碰这套语义：purge 删的是用户行与个人数据，
// user_level_change_logs 随用户一起清（它属于个人数据），但**绝不重算**等级。

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/level/dto"
	"hostsent/backend/internal/modules/admin/user/level/model"
	"hostsent/backend/internal/modules/admin/user/level/repository"
)

// errUnexpectedLevelRepoCall 测试未预期的仓储调用。
//
// 不用「返回零值」糊弄过去：漏实现的方法被误调时应当立刻炸出来，
// 否则测试会因为「顺手调了别的方法」而静默通过。
var errUnexpectedLevelRepoCall = errors.New("测试未预期的等级仓储调用")

// fakeLevelRepo 只实现消费升级用到的四个方法。
type fakeLevelRepo struct {
	levels         map[uint64]*model.UserLevel
	currentLevelID *uint64
	totalConsume   float64

	target        *model.UserLevel
	targetErr     error
	userErr       error
	consumes      []float64
	setTierCalls  []uint64
	changeLogs    []*model.UserLevelChangeLog
	createLogErr  error
	setTierErr    error
	findBestCalls int
}

func (f *fakeLevelRepo) List(context.Context, dto.ListQuery) ([]model.UserLevel, int64, error) {
	return nil, 0, errUnexpectedLevelRepoCall
}

func (f *fakeLevelRepo) FindByID(_ context.Context, id uint64) (*model.UserLevel, error) {
	if item, ok := f.levels[id]; ok {
		return item, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeLevelRepo) FindByCode(context.Context, string) (*model.UserLevel, error) {
	return nil, errUnexpectedLevelRepoCall
}

func (f *fakeLevelRepo) Create(context.Context, *model.UserLevel) error {
	return errUnexpectedLevelRepoCall
}
func (f *fakeLevelRepo) Update(context.Context, *model.UserLevel) error {
	return errUnexpectedLevelRepoCall
}
func (f *fakeLevelRepo) Delete(context.Context, uint64) error { return errUnexpectedLevelRepoCall }

func (f *fakeLevelRepo) FindBestByThreshold(context.Context, float64) (*model.UserLevel, error) {
	f.findBestCalls++
	if f.targetErr != nil {
		return nil, f.targetErr
	}
	if f.target == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.target, nil
}

func (f *fakeLevelRepo) GetUserTier(context.Context, uint64) (*uint64, float64, error) {
	if f.userErr != nil {
		return nil, 0, f.userErr
	}
	return f.currentLevelID, f.totalConsume, nil
}

func (f *fakeLevelRepo) AddUserConsume(_ context.Context, _ uint64, delta float64) (float64, error) {
	f.consumes = append(f.consumes, delta)
	f.totalConsume += delta
	return f.totalConsume, nil
}

func (f *fakeLevelRepo) SetUserTier(_ context.Context, _ uint64, levelID uint64) error {
	if f.setTierErr != nil {
		return f.setTierErr
	}
	f.setTierCalls = append(f.setTierCalls, levelID)
	id := levelID
	f.currentLevelID = &id
	return nil
}

func (f *fakeLevelRepo) CreateChangeLog(_ context.Context, item *model.UserLevelChangeLog) error {
	if f.createLogErr != nil {
		return f.createLogErr
	}
	f.changeLogs = append(f.changeLogs, item)
	return nil
}

// 编译期断言：假仓储必须始终满足真实接口（接口加方法时这里先报错）。
var _ repository.UserLevelRepository = (*fakeLevelRepo)(nil)

func lvl(id uint64, code string, weight int, threshold float64) *model.UserLevel {
	return &model.UserLevel{ID: id, Code: code, Weight: weight, UpgradeThreshold: threshold, Status: "active"}
}

// TestApplyConsume_RefundDoesNotRevert 退款（零/负数）不得回退累计消费与等级。
//
// ApplyConsume 对 amount <= 0 直接返回：这是 R7 的实现点，一旦有人把它改成
// 「负数也要记账」，这条断言会立刻失败，从而逼出一个显式的设计决定。
func TestApplyConsume_RefundDoesNotRevert(t *testing.T) {
	repo := &fakeLevelRepo{totalConsume: 1000, target: lvl(2, "pro", 20, 500)}
	svc := NewLevelUpgradeService(repo)

	for _, amount := range []float64{0, -100, -9999} {
		if err := svc.ApplyConsume(context.Background(), 190, amount); err != nil {
			t.Fatalf("退款金额 %v 不应报错: %v", amount, err)
		}
	}
	if len(repo.consumes) != 0 {
		t.Fatalf("非正数金额不得写入累计消费，实际写入 %v", repo.consumes)
	}
	if repo.totalConsume != 1000 {
		t.Fatalf("累计消费必须保持不变（只升不降），实际 %v", repo.totalConsume)
	}
	if len(repo.setTierCalls) != 0 {
		t.Fatalf("退款不得触发等级变更，实际 %v", repo.setTierCalls)
	}
}

// TestApplyConsume_ZeroUserIsNoop 匿名/系统单（userID=0）不得写库。
func TestApplyConsume_ZeroUserIsNoop(t *testing.T) {
	repo := &fakeLevelRepo{totalConsume: 500}
	svc := NewLevelUpgradeService(repo)

	if err := svc.ApplyConsume(context.Background(), 0, 100); err != nil {
		t.Fatalf("userID=0 不应报错: %v", err)
	}
	if len(repo.consumes) != 0 {
		t.Fatalf("userID=0 不得记账，实际 %v", repo.consumes)
	}
}

// TestRecalculate_NeverDowngrades 目标等级权重不高于当前时不动等级、不写变更日志。
func TestRecalculate_NeverDowngrades(t *testing.T) {
	current := uint64(3)
	repo := &fakeLevelRepo{
		levels:         map[uint64]*model.UserLevel{3: lvl(3, "pro", 20, 500)},
		currentLevelID: &current,
		totalConsume:   100,
		target:         lvl(1, "free", 10, 0), // 权重低于当前
	}
	svc := NewLevelUpgradeService(repo)

	changed, from, to, err := svc.Recalculate(context.Background(), 190)
	if err != nil {
		t.Fatalf("重算失败: %v", err)
	}
	if changed {
		t.Fatal("目标等级不高于当前时必须返回 changed=false（只升不降）")
	}
	if from != "pro" || to != "free" {
		t.Fatalf("from/to 应回显「当前 pro → 命中 free 但不升级」，实际 from=%q to=%q", from, to)
	}
	if len(repo.setTierCalls) != 0 {
		t.Fatalf("降级路径不得调用 SetUserTier，实际 %v", repo.setTierCalls)
	}
	if len(repo.changeLogs) != 0 {
		t.Fatalf("等级未变更时不得写 user_level_change_logs，实际 %d 条", len(repo.changeLogs))
	}
}

// TestRecalculate_UpgradeWritesHistory 升级路径必须留下变更日志（含消费额快照）。
func TestRecalculate_UpgradeWritesHistory(t *testing.T) {
	repo := &fakeLevelRepo{totalConsume: 2000, target: lvl(2, "pro", 20, 500)}
	svc := NewLevelUpgradeService(repo)

	changed, from, to, err := svc.Recalculate(context.Background(), 190)
	if err != nil {
		t.Fatalf("重算失败: %v", err)
	}
	if !changed || to != "pro" {
		t.Fatalf("应升级到 pro，实际 changed=%v to=%q", changed, to)
	}
	if from != "" {
		t.Fatalf("原先无等级时 from 应为空，实际 %q", from)
	}
	if len(repo.setTierCalls) != 1 || repo.setTierCalls[0] != 2 {
		t.Fatalf("应把等级设为 2，实际 %v", repo.setTierCalls)
	}
	if len(repo.changeLogs) != 1 {
		t.Fatalf("应写 1 条变更日志，实际 %d", len(repo.changeLogs))
	}
	log := repo.changeLogs[0]
	if log.Reason != "consume_upgrade" {
		t.Errorf("变更原因应为 consume_upgrade，实际 %q", log.Reason)
	}
	if log.TotalConsumeAmount != 2000 {
		t.Errorf("日志应快照当时的累计消费 2000，实际 %v", log.TotalConsumeAmount)
	}
	if log.ToLevelID != 2 {
		t.Errorf("日志的目标等级应为 2，实际 %d", log.ToLevelID)
	}
	if log.FromLevelID != nil {
		t.Errorf("原先无等级时 FromLevelID 应为 nil，实际 %v", *log.FromLevelID)
	}
}

// TestRecalculate_LogFailureStillUpgrades 日志写失败不影响等级更新，但错误要上抛。
//
// 静默吞掉会让「等级升了但查不到为什么升」成为常态；反过来因为日志失败而回滚
// 等级又会让权益与日志长期不一致。当前实现选择「等级生效 + 错误上抛」。
func TestRecalculate_LogFailureStillUpgrades(t *testing.T) {
	boom := errors.New("log table unavailable")
	repo := &fakeLevelRepo{totalConsume: 2000, target: lvl(2, "pro", 20, 500), createLogErr: boom}
	svc := NewLevelUpgradeService(repo)

	changed, _, _, err := svc.Recalculate(context.Background(), 190)
	if !errors.Is(err, boom) {
		t.Fatalf("日志失败必须上抛给调用方记录，实际 %v", err)
	}
	if !changed {
		t.Fatal("日志失败不应回滚已生效的等级更新")
	}
	if len(repo.setTierCalls) != 1 {
		t.Fatalf("等级仍应被更新，实际 %v", repo.setTierCalls)
	}
}

// TestRecalculate_NoThresholdKeepsCurrent 未达任何门槛时保持现状（不回落、不报错）。
func TestRecalculate_NoThresholdKeepsCurrent(t *testing.T) {
	repo := &fakeLevelRepo{totalConsume: 10, targetErr: gorm.ErrRecordNotFound}
	svc := NewLevelUpgradeService(repo)

	changed, _, _, err := svc.Recalculate(context.Background(), 190)
	if err != nil {
		t.Fatalf("未达门槛不应报错: %v", err)
	}
	if changed {
		t.Fatal("未达门槛时不应变更等级")
	}
	if len(repo.setTierCalls) != 0 {
		t.Fatalf("未达门槛不得写等级，实际 %v", repo.setTierCalls)
	}
}

// TestRecalculate_UserMissingIsNotError 用户行不存在时静默返回。
//
// 注销后的异步任务（订单回调、积分结算）仍可能带着已消失的 user_id 走到这里，
// 报错只会往日志里灌无意义噪声。
func TestRecalculate_UserMissingIsNotError(t *testing.T) {
	repo := &fakeLevelRepo{userErr: gorm.ErrRecordNotFound}
	svc := NewLevelUpgradeService(repo)

	if _, _, _, err := svc.Recalculate(context.Background(), 190); err != nil {
		t.Fatalf("用户不存在不应视为错误: %v", err)
	}
}

// TestApplyConsume_PropagatesRepoError 仓储真错误必须上抛，不能被当成「未达门槛」吞掉。
func TestApplyConsume_PropagatesRepoError(t *testing.T) {
	boom := errors.New("db down")
	repo := &fakeLevelRepo{userErr: boom}
	svc := NewLevelUpgradeService(repo)

	if err := svc.ApplyConsume(context.Background(), 190, 100); !errors.Is(err, boom) {
		t.Fatalf("仓储错误必须原样上抛，实际 %v", err)
	}
}
