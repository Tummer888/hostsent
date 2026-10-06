package service

// 用户等级人工调整（用户详情页「编辑资料」改等级）的行为基线。
//
// 与消费升级是两条通道：那条只升不降、由累计消费驱动；这条允许升降、由人工决定。
// 两条都写 user_level_change_logs，靠 reason 区分。这里的断言覆盖
// 「允许降级」「变更留痕」「目标等级必须启用中」「等级没变不写日志」四件事。

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/level/dto"
	"hostsent/backend/internal/modules/admin/user/account/level/model"
	"hostsent/backend/internal/modules/admin/user/account/level/repository"
)

// fakeAssignRepo 只实现人工调整链路用到的四个方法，其余调用立刻炸出来。
type fakeAssignRepo struct {
	levels map[uint64]*model.UserLevel
	list   []model.UserLevel

	currentLevelID *uint64
	totalConsume   float64
	userErr        error

	setTierCalls []uint64
	changeLogs   []*model.UserLevelChangeLog
	created      []*model.UserLevel
	logErr       error
}

func (f *fakeAssignRepo) List(context.Context, dto.ListQuery) ([]model.UserLevel, int64, error) {
	return f.list, int64(len(f.list)), nil
}

func (f *fakeAssignRepo) FindByID(_ context.Context, id uint64) (*model.UserLevel, error) {
	if item, ok := f.levels[id]; ok {
		return item, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAssignRepo) FindByCode(context.Context, string) (*model.UserLevel, error) {
	return nil, errUnexpectedLevelRepoCall
}

func (f *fakeAssignRepo) Create(_ context.Context, item *model.UserLevel) error {
	item.ID = 77
	f.created = append(f.created, item)
	return nil
}
func (f *fakeAssignRepo) Update(context.Context, *model.UserLevel) error {
	return errUnexpectedLevelRepoCall
}
func (f *fakeAssignRepo) Delete(context.Context, uint64) error { return errUnexpectedLevelRepoCall }

func (f *fakeAssignRepo) FindBestByThreshold(context.Context, float64) (*model.UserLevel, error) {
	return nil, errUnexpectedLevelRepoCall
}

func (f *fakeAssignRepo) GetUserTier(context.Context, uint64) (*uint64, float64, error) {
	if f.userErr != nil {
		return nil, 0, f.userErr
	}
	return f.currentLevelID, f.totalConsume, nil
}

func (f *fakeAssignRepo) AddUserConsume(context.Context, uint64, float64) (float64, error) {
	return 0, errUnexpectedLevelRepoCall
}

func (f *fakeAssignRepo) SetUserTier(_ context.Context, _ uint64, levelID uint64) error {
	f.setTierCalls = append(f.setTierCalls, levelID)
	return nil
}

func (f *fakeAssignRepo) CreateChangeLog(_ context.Context, item *model.UserLevelChangeLog) error {
	if f.logErr != nil {
		return f.logErr
	}
	f.changeLogs = append(f.changeLogs, item)
	return nil
}

// 编译期断言：假仓储必须始终满足真实接口（接口加方法时这里先报错）。
var _ repository.UserLevelRepository = (*fakeAssignRepo)(nil)

func activeLevel(id uint64, code string, weight int) *model.UserLevel {
	return &model.UserLevel{ID: id, Code: code, Name: code, Weight: weight, Status: "active"}
}

// TestAssignLevel_AllowsDowngrade 人工调整允许降级 —— 这是与消费升级最大的差别。
//
// 消费升级的 Recalculate 遇到「目标权重不高于当前」会直接返回；人工调整不能沿用
// 那个判断，否则运营把误升的用户降回来时会「保存成功但等级没变」。
func TestAssignLevel_AllowsDowngrade(t *testing.T) {
	current := uint64(3)
	repo := &fakeAssignRepo{
		levels:         map[uint64]*model.UserLevel{1: activeLevel(1, "silver", 10), 3: activeLevel(3, "diamond", 40)},
		currentLevelID: &current,
		totalConsume:   25000,
	}
	svc := NewUserLevelService(repo)

	if err := svc.AssignLevel(context.Background(), 190, 1); err != nil {
		t.Fatalf("降级调整不应报错: %v", err)
	}
	if len(repo.setTierCalls) != 1 || repo.setTierCalls[0] != 1 {
		t.Fatalf("应把等级改为 1，实际 %v", repo.setTierCalls)
	}
	if len(repo.changeLogs) != 1 {
		t.Fatalf("人工调整必须留痕，实际 %d 条", len(repo.changeLogs))
	}
	log := repo.changeLogs[0]
	if log.Reason != model.ReasonManualAdjust {
		t.Errorf("变更原因应为 manual_adjust，实际 %q", log.Reason)
	}
	if log.FromLevelCode != "diamond" || log.ToLevelCode != "silver" {
		t.Errorf("日志应记录 diamond → silver，实际 %q → %q", log.FromLevelCode, log.ToLevelCode)
	}
	if log.FromLevelID == nil || *log.FromLevelID != 3 {
		t.Errorf("日志的来源等级应为 3，实际 %v", log.FromLevelID)
	}
	if log.TotalConsumeAmount != 25000 {
		t.Errorf("日志应快照当时累计消费 25000，实际 %v", log.TotalConsumeAmount)
	}
}

// TestAssignLevel_SameLevelNoLog 等级没变时不写日志：一条「从 A 调到 A」的记录
// 只会污染变更历史，让运营以为发生过调整。
func TestAssignLevel_SameLevelNoLog(t *testing.T) {
	current := uint64(1)
	repo := &fakeAssignRepo{
		levels:         map[uint64]*model.UserLevel{1: activeLevel(1, "silver", 10)},
		currentLevelID: &current,
	}
	svc := NewUserLevelService(repo)

	if err := svc.AssignLevel(context.Background(), 190, 1); err != nil {
		t.Fatalf("同等级调整不应报错: %v", err)
	}
	if len(repo.setTierCalls) != 0 {
		t.Fatalf("等级未变时不应写 users，实际 %v", repo.setTierCalls)
	}
	if len(repo.changeLogs) != 0 {
		t.Fatalf("等级未变时不应写变更日志，实际 %d 条", len(repo.changeLogs))
	}
}

// TestAssignLevel_RejectsDisabledLevel 停用等级不可指派。
//
// 挂到停用等级上，用户端会显示一个运营已下线的等级，而自动升级（只升不降）
// 又不会把他从那里挪走。
func TestAssignLevel_RejectsDisabledLevel(t *testing.T) {
	repo := &fakeAssignRepo{
		levels: map[uint64]*model.UserLevel{
			9: {ID: 9, Code: "king", Weight: 60, Status: "disabled"},
		},
	}
	svc := NewUserLevelService(repo)

	if err := svc.AssignLevel(context.Background(), 190, 9); !errors.Is(err, ErrLevelDisabled) {
		t.Fatalf("指派停用等级应返回 ErrLevelDisabled，实际 %v", err)
	}
	if len(repo.setTierCalls) != 0 {
		t.Fatalf("被拒的指派不得写 users，实际 %v", repo.setTierCalls)
	}
}

// TestAssignLevel_RejectsMissingLevel 等级不存在时返回仓储的 ErrRecordNotFound，
// 由装配层的适配器翻成「用户等级不存在」——不能让 handler 把它当成「用户不存在」。
func TestAssignLevel_RejectsMissingLevel(t *testing.T) {
	repo := &fakeAssignRepo{levels: map[uint64]*model.UserLevel{}}
	svc := NewUserLevelService(repo)

	if err := svc.AssignLevel(context.Background(), 190, 999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("等级不存在应返回 ErrRecordNotFound，实际 %v", err)
	}
}

// TestAssignLevel_RejectsZeroIDs 空 ID 直接拒绝，不落库也不写日志。
func TestAssignLevel_RejectsZeroIDs(t *testing.T) {
	repo := &fakeAssignRepo{levels: map[uint64]*model.UserLevel{}}
	svc := NewUserLevelService(repo)

	for _, tc := range []struct{ userID, levelID uint64 }{{0, 1}, {190, 0}, {0, 0}} {
		if err := svc.AssignLevel(context.Background(), tc.userID, tc.levelID); !errors.Is(err, ErrInvalidAssignTarget) {
			t.Fatalf("userID=%d levelID=%d 应返回 ErrInvalidAssignTarget，实际 %v", tc.userID, tc.levelID, err)
		}
	}
	if len(repo.changeLogs) != 0 {
		t.Fatalf("非法入参不得写日志，实际 %d 条", len(repo.changeLogs))
	}
}

// TestAssignLevel_LogFailureSurfaces 日志写失败上抛：等级已经写进去了，
// 调用方必须知道「这次调整没有留痕」。
func TestAssignLevel_LogFailureSurfaces(t *testing.T) {
	boom := errors.New("log table unavailable")
	repo := &fakeAssignRepo{
		levels:       map[uint64]*model.UserLevel{1: activeLevel(1, "silver", 10)},
		totalConsume: 0,
		logErr:       boom,
	}
	svc := NewUserLevelService(repo)

	if err := svc.AssignLevel(context.Background(), 190, 1); !errors.Is(err, boom) {
		t.Fatalf("日志失败必须上抛，实际 %v", err)
	}
	if len(repo.setTierCalls) != 1 {
		t.Fatalf("日志失败不应回滚已生效的等级更新，实际 %v", repo.setTierCalls)
	}
}

// TestDefaultLevelID_PicksLowestWeight 起始等级取「权重最低的启用等级」，
// 而不是写死某个 code：等级是运营可改名/可增删的配置。
func TestDefaultLevelID_PicksLowestWeight(t *testing.T) {
	repo := &fakeAssignRepo{list: []model.UserLevel{
		{ID: 8, Code: "star", Weight: 50, Status: "active"},
		{ID: 1, Code: "silver", Weight: 10, Status: "active"},
		{ID: 6, Code: "platinum", Weight: 30, Status: "active"},
	}}
	svc := NewUserLevelService(repo)

	id, err := svc.DefaultLevelID(context.Background())
	if err != nil {
		t.Fatalf("解析起始等级失败: %v", err)
	}
	if id != 1 {
		t.Fatalf("应取权重最低的等级 1，实际 %d", id)
	}
}

// TestDefaultLevelID_EmptyTableReturnsZero 等级表为空时返回 0（调用方按「不设等级」处理），
// 不报错：注册/建号不能因为等级表没配好而整体失败。
func TestDefaultLevelID_EmptyTableReturnsZero(t *testing.T) {
	svc := NewUserLevelService(&fakeAssignRepo{})

	id, err := svc.DefaultLevelID(context.Background())
	if err != nil {
		t.Fatalf("空等级表不应报错: %v", err)
	}
	if id != 0 {
		t.Fatalf("空等级表应返回 0，实际 %d", id)
	}
}

// TestNormalizeBenefits benefits 列是 jsonb：写空串会抛 SQLSTATE 22P02，
// 运营在「新建等级」留空权益时看到的是 500「服务器内部错误」。这里把它收敛成
// 合法 JSONB，同时保持「已是 JSON 就原样透传」。
func TestNormalizeBenefits(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"空串", "", `{"benefits":[]}`},
		{"纯空白", "   \n ", `{"benefits":[]}`},
		{"已是对像", `{"benefits":["A"]}`, `{"benefits":["A"]}`},
		{"已是数组", `["A","B"]`, `["A","B"]`},
		{"换行分隔", "A\nB", `{"benefits":["A","B"]}`},
		{"逗号分隔", "A, B", `{"benefits":["A","B"]}`},
		{"中文逗号", "A，B", `{"benefits":["A","B"]}`},
		{"裸数字不是权益", "123", `{"benefits":["123"]}`},
	}
	for _, tc := range cases {
		if got := normalizeBenefits(tc.in); got != tc.want {
			t.Errorf("%s: normalizeBenefits(%q) = %q，期望 %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// TestCreate_EmptyBenefitsIsValidJSONB 建等级时留空权益必须能落库。
func TestCreate_EmptyBenefitsIsValidJSONB(t *testing.T) {
	repo := &fakeAssignRepo{}
	svc := NewUserLevelService(repo)

	info, err := svc.Create(context.Background(), dto.CreateRequest{Name: "临时", Code: "tmp"})
	if err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	if info.Benefits != `{"benefits":[]}` {
		t.Fatalf("空权益应写成合法 JSONB 空集，实际 %q", info.Benefits)
	}
}

// TestCheckAssignable 建号先验：存在且启用中才放行。
func TestCheckAssignable(t *testing.T) {
	repo := &fakeAssignRepo{levels: map[uint64]*model.UserLevel{
		1: activeLevel(1, "silver", 10),
		9: {ID: 9, Code: "king", Weight: 60, Status: "disabled"},
	}}
	svc := NewUserLevelService(repo)

	if err := svc.CheckAssignable(context.Background(), 1); err != nil {
		t.Fatalf("启用中的等级应放行: %v", err)
	}
	if err := svc.CheckAssignable(context.Background(), 9); !errors.Is(err, ErrLevelDisabled) {
		t.Fatalf("停用等级应返回 ErrLevelDisabled，实际 %v", err)
	}
	if err := svc.CheckAssignable(context.Background(), 999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("不存在的等级应返回 ErrRecordNotFound，实际 %v", err)
	}
	if err := svc.CheckAssignable(context.Background(), 0); !errors.Is(err, ErrInvalidAssignTarget) {
		t.Fatalf("levelID=0 应返回 ErrInvalidAssignTarget，实际 %v", err)
	}
}
