package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/verification/dto"
	"hostsent/backend/internal/modules/admin/user/verification/model"
)

// fakeRepo 只实现审核状态机用到的几个方法，其余方法在测试里被调用即 panic ——
// 这样一旦状态机多调了一个仓储方法（例如意外写库），测试会立刻暴露而不是静默通过。
type fakeRepo struct {
	app *model.VerificationApplication

	updateReviewCalls int
	reviewLog         *model.VerificationReviewLog
	verifiedAt        *time.Time
	verifiedSource    string
	cleared           int
}

var errUnexpectedRepoCall = errors.New("测试未预期的仓储调用")

func (f *fakeRepo) FindByID(ctx context.Context, id uint64) (*model.VerificationApplication, error) {
	if f.app == nil || f.app.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	// 返回副本：状态机不应该靠改指针原地生效，必须走 UpdateReview。
	copied := *f.app
	return &copied, nil
}

func (f *fakeRepo) UpdateReview(ctx context.Context, app *model.VerificationApplication, log *model.VerificationReviewLog) error {
	f.updateReviewCalls++
	f.reviewLog = log
	f.app.Status = app.Status
	f.app.ReviewedAt = app.ReviewedAt
	f.app.ReviewedBy = app.ReviewedBy
	f.app.ReviewerName = app.ReviewerName
	f.app.ReviewNote = app.ReviewNote
	f.app.RejectReason = app.RejectReason
	f.app.RejectReasonCode = app.RejectReasonCode
	return nil
}

func (f *fakeRepo) MarkUserVerified(ctx context.Context, userID uint64, at time.Time, source string) error {
	t := at
	f.verifiedAt = &t
	f.verifiedSource = source
	return nil
}

func (f *fakeRepo) ClearUserVerified(ctx context.Context, userID uint64) error {
	f.cleared++
	f.verifiedAt = nil
	return nil
}

// —— 以下方法审核状态机不应触达 ——

func (f *fakeRepo) ListByStatus(context.Context, string, dto.VerificationListQuery) ([]model.VerificationApplication, int64, error) {
	return nil, 0, errUnexpectedRepoCall
}
func (f *fakeRepo) LatestByUser(context.Context, uint64) (*model.VerificationApplication, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) ListByUser(context.Context, uint64, int, int) ([]model.VerificationApplication, int64, error) {
	return nil, 0, errUnexpectedRepoCall
}
func (f *fakeRepo) Create(context.Context, *model.VerificationApplication) error {
	return errUnexpectedRepoCall
}
func (f *fakeRepo) UpdateProviderResult(context.Context, *model.VerificationApplication) error {
	return errUnexpectedRepoCall
}
func (f *fakeRepo) WriteReviewLog(context.Context, *model.VerificationReviewLog) error {
	return errUnexpectedRepoCall
}
func (f *fakeRepo) ListReviewLogs(context.Context, uint64) ([]model.VerificationReviewLog, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) NextReviewRound(context.Context, uint64) (int, error) {
	return 0, errUnexpectedRepoCall
}
func (f *fakeRepo) ListDocuments(context.Context, uint64) ([]model.VerificationDocument, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) SaveDocument(context.Context, *model.VerificationDocument) error {
	return errUnexpectedRepoCall
}
func (f *fakeRepo) FindDocument(context.Context, uint64) (*model.VerificationDocument, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) SaveEnterprise(context.Context, *model.VerificationEnterprise) error {
	return errUnexpectedRepoCall
}
func (f *fakeRepo) FindEnterprise(context.Context, uint64) (*model.VerificationEnterprise, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) CountPending(context.Context) (int64, error) { return 0, errUnexpectedRepoCall }
func (f *fakeRepo) ListConfigs(context.Context, dto.VerificationConfigListQuery) ([]model.VerificationConfig, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) FindConfigByKey(context.Context, string) (*model.VerificationConfig, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) SaveConfig(context.Context, *model.VerificationConfig) error {
	return errUnexpectedRepoCall
}
func (f *fakeRepo) DeleteConfig(context.Context, uint64) error { return errUnexpectedRepoCall }
func (f *fakeRepo) ListProviders(context.Context) ([]model.RealnameProvider, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) FindProviderByID(context.Context, uint64) (*model.RealnameProvider, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) FindDefaultProvider(context.Context, string) (*model.RealnameProvider, error) {
	return nil, errUnexpectedRepoCall
}
func (f *fakeRepo) CreateProvider(context.Context, *model.RealnameProvider) error {
	return errUnexpectedRepoCall
}
func (f *fakeRepo) UpdateProvider(context.Context, *model.RealnameProvider) error {
	return errUnexpectedRepoCall
}
func (f *fakeRepo) DeleteProvider(context.Context, uint64) error        { return errUnexpectedRepoCall }
func (f *fakeRepo) ClearDefaultProviders(context.Context, uint64) error { return errUnexpectedRepoCall }
func (f *fakeRepo) IsUserVerified(context.Context, uint64) (bool, error) {
	return false, errUnexpectedRepoCall
}

func newTestService(repo *fakeRepo) VerificationService {
	return NewVerificationService(Deps{Repo: repo})
}

func pendingApp() *model.VerificationApplication {
	return &model.VerificationApplication{
		ID:          7,
		UserID:      190,
		Username:    "probe_user",
		Status:      model.StatusPending,
		RealName:    "张三",
		Provider:    "manual",
		ReviewRound: 1,
	}
}

// 整单通过：pending → approved，写信任信号，并留一条 approve 审核日志。
// 三者缺一都会留下「通过了但查不到为什么」的黑洞（doc104 §5.1）。
func TestApprove_WritesTrustSignalAndLog(t *testing.T) {
	repo := &fakeRepo{app: pendingApp()}
	svc := newTestService(repo)

	info, err := svc.Approve(context.Background(), 7, dto.ReviewRequest{Note: "材料齐全"}, 1, "admin")
	if err != nil {
		t.Fatalf("通过失败: %v", err)
	}
	if info.Status != model.StatusApproved {
		t.Fatalf("状态应为 approved，实际 %s", info.Status)
	}
	if repo.verifiedAt == nil {
		t.Fatal("通过后必须写 users.real_name_verified_at（唯一的实名信任信号）")
	}
	if repo.verifiedSource != "manual" {
		t.Fatalf("信任信号来源应为生效的 provider，实际 %q", repo.verifiedSource)
	}
	if repo.reviewLog == nil {
		t.Fatal("必须留一条审核日志")
	}
	if repo.reviewLog.Action != model.ActionApprove {
		t.Fatalf("日志 action 应为 approve，实际 %s", repo.reviewLog.Action)
	}
	if repo.reviewLog.FromStatus != model.StatusPending || repo.reviewLog.ToStatus != model.StatusApproved {
		t.Fatalf("日志状态迁移不符: %s → %s", repo.reviewLog.FromStatus, repo.reviewLog.ToStatus)
	}
	if repo.reviewLog.OperatorID != 1 || repo.reviewLog.OperatorName != "admin" {
		t.Fatalf("日志应记录真实审核人: %+v", repo.reviewLog)
	}
}

// 重复审核必须被拒：再审会覆盖前一次的审核人与时间，让「谁放行的」永久丢失。
func TestApprove_RejectsNonPending(t *testing.T) {
	for _, status := range []string{model.StatusApproved, model.StatusRejected} {
		repo := &fakeRepo{app: pendingApp()}
		repo.app.Status = status
		svc := newTestService(repo)

		_, err := svc.Approve(context.Background(), 7, dto.ReviewRequest{}, 1, "admin")
		if err == nil {
			t.Fatalf("状态为 %s 时不应允许再次通过", status)
		}
		if !errors.Is(err, ErrStatusConflict) {
			t.Fatalf("错误应为 ErrStatusConflict，实际: %v", err)
		}
		if repo.updateReviewCalls != 0 {
			t.Fatal("被拒时不应写库")
		}
	}
}

// 审核人缺失必须被拒：审核动作没有留痕就失去可追溯性。
func TestApprove_RequiresOperator(t *testing.T) {
	repo := &fakeRepo{app: pendingApp()}
	svc := newTestService(repo)

	_, err := svc.Approve(context.Background(), 7, dto.ReviewRequest{}, 0, "")
	if err == nil {
		t.Fatal("operatorID=0 必须被拒")
	}
	if !errors.Is(err, ErrOperatorRequired) {
		t.Fatalf("错误应为 ErrOperatorRequired，实际: %v", err)
	}
	if repo.updateReviewCalls != 0 {
		t.Fatal("被拒时不应写库")
	}
}

// 驳回：理由必填，写 reject_reason，且必须清空实名信任信号。
func TestReject_RequiresReasonAndClearsTrust(t *testing.T) {
	repo := &fakeRepo{app: pendingApp()}
	svc := newTestService(repo)

	if _, err := svc.Reject(context.Background(), 7, dto.ReviewRequest{}, 1, "admin"); err == nil {
		t.Fatal("驳回理由为空必须被拒")
	}
	if repo.updateReviewCalls != 0 {
		t.Fatal("理由为空时不应写库")
	}

	info, err := svc.Reject(context.Background(), 7, dto.ReviewRequest{
		RejectReasonCode: "blurry", RejectReason: "证件照模糊",
	}, 1, "admin")
	if err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	if info.Status != model.StatusRejected {
		t.Fatalf("状态应为 rejected，实际 %s", info.Status)
	}
	if info.RejectReason != "证件照模糊" || info.RejectReasonCode != "blurry" {
		t.Fatalf("驳回理由未落库: %+v", info)
	}
	if repo.cleared == 0 {
		t.Fatal("驳回必须清空实名信任信号")
	}
	if repo.verifiedAt != nil {
		t.Fatal("驳回后不应留下实名信任信号")
	}
	if repo.reviewLog.Action != model.ActionReject {
		t.Fatalf("日志 action 应为 reject，实际 %s", repo.reviewLog.Action)
	}
}

// 撤销只允许对已通过的申请执行：撤销是审核方的判定，不该把 pending 直接废掉。
func TestRevoke_OnlyApproved(t *testing.T) {
	repo := &fakeRepo{app: pendingApp()}
	svc := newTestService(repo)

	if _, err := svc.Revoke(context.Background(), 7, dto.ReviewRequest{}, 1, "admin"); err == nil {
		t.Fatal("pending 状态不应允许撤销")
	}

	repo = &fakeRepo{app: pendingApp()}
	repo.app.Status = model.StatusApproved
	svc = newTestService(repo)
	info, err := svc.Revoke(context.Background(), 7, dto.ReviewRequest{Note: "材料造假"}, 1, "admin")
	if err != nil {
		t.Fatalf("撤销失败: %v", err)
	}
	if info.Status != model.StatusRejected {
		t.Fatalf("撤销后应置回 rejected（不是 pending），实际 %s", info.Status)
	}
	if repo.cleared == 0 {
		t.Fatal("撤销必须清空实名信任信号")
	}
	if repo.reviewLog.Action != model.ActionRevoke {
		t.Fatalf("日志 action 应为 revoke，实际 %s", repo.reviewLog.Action)
	}
}

// 申请不存在必须映射为 ErrApplicationNotFound，而不是把 gorm 原文抛给上层。
func TestApprove_NotFound(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo)

	_, err := svc.Approve(context.Background(), 999, dto.ReviewRequest{}, 1, "admin")
	if err == nil {
		t.Fatal("不存在的申请必须报错")
	}
	if !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("错误应为 ErrApplicationNotFound，实际: %v", err)
	}
}

// 证件号规范化：个人 18 位身份证（末位可为 X），企业 18 位统一社会信用代码。
func TestNormalizeID(t *testing.T) {
	idType, id, err := normalizeID(model.TypePersonal, "", "11010119900101123x")
	if err != nil {
		t.Fatalf("合法身份证号不应报错: %v", err)
	}
	if idType != "IDENTITY_CARD" {
		t.Fatalf("个人认证的证件类型应为 IDENTITY_CARD，实际 %s", idType)
	}
	if id != "11010119900101123X" {
		t.Fatalf("证件号应统一大写，实际 %s", id)
	}

	if _, _, err := normalizeID(model.TypePersonal, "", "11010119900101123"); err == nil {
		t.Fatal("长度不足必须报错")
	}
	if _, _, err := normalizeID(model.TypePersonal, "", "1101011990010112AB"); err == nil {
		t.Fatal("非数字（末位非 X）必须报错")
	}

	entType, entID, err := normalizeID(model.TypeEnterprise, "", "91110000000000001X")
	if err != nil {
		t.Fatalf("合法统一社会信用代码不应报错: %v", err)
	}
	if entType != "CREDIT_CODE" {
		t.Fatalf("企业认证默认证件类型应为 CREDIT_CODE，实际 %s", entType)
	}
	if entID != "91110000000000001X" {
		t.Fatalf("企业证件号应原样保留大写，实际 %s", entID)
	}
}

// 脱敏口径：证件号保留前 6 后 4，手机号保留前 3 后 4。
func TestMaskHelpers(t *testing.T) {
	if got := maskIDNumber("110101199001011234"); got != "110101********1234" {
		t.Fatalf("证件号脱敏不符: %s", got)
	}
	if got := maskMobile("13700009999"); got != "137****9999" {
		t.Fatalf("手机号脱敏不符: %s", got)
	}
	if got := maskMobile(""); got != "" {
		t.Fatalf("空手机号应原样返回，实际 %q", got)
	}
	if got := maskCreditCode("91110000000000001X"); got != "9111**********001X" {
		t.Fatalf("统一社会信用代码脱敏不符: %s", got)
	}
}
