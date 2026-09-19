package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
	"hostsent/backend/internal/modules/admin/user/account/repository"
)

// 注销 / 恢复 / 留存期清理的哨兵错误。
// handler 逐条映射 HTTP 语义，而不是把仓储原文抛成 500。
var (
	// ErrUserDeleted 目标用户已注销（重复注销、对已注销用户做常规写操作）。
	ErrUserDeleted = errors.New("用户已注销")
	// ErrUserNotDeleted 目标用户未注销（恢复一个没注销的用户）。
	ErrUserNotDeleted = errors.New("用户未注销，无需恢复")
	// ErrDeletionBlocked 存在硬阻断项（在管实例），force 也绕不过。
	ErrDeletionBlocked = errors.New("用户仍存在未释放的实例，无法注销")
	// ErrDeletionNeedsForce 存在警告项（余额/未结账单/工单/订单），需 force=true。
	ErrDeletionNeedsForce = errors.New("用户存在未结清事项，需确认后强制注销")
	// ErrInvalidOperator 注销操作人缺失（无法追溯「谁注销的」）。
	ErrInvalidOperator = errors.New("注销操作人不能为空")
	// ErrRestoreConflict 恢复时该账号名/邮箱已被新注册用户占用（doc104 §4.2）。
	//
	// 注销期间 username/email 是「释放」的（部分唯一索引只约束未注销行），
	// 别人可能已经用同样的账号名注册了。此时**必须明确失败**，不做静默改名——
	// 静默改名会让「这条审计记录对应的是谁」变得不可追溯。
	ErrRestoreConflict = errors.New("该用户名或邮箱已被占用，请先修改后再恢复")
)

// 默认留存期（天）。制度要求个人数据至少留存一段时间再彻底删除（doc104 §4.1），
// 180 天对齐日志中心既有保留策略，避免两套保留期口径打架。
const DefaultDeletionRetentionDays = 180

// 单轮清理上限的默认值与硬上限。
// 硬上限的意义：一次手工触发就把十万行删掉会把库锁死，宁可多轮慢跑。
const (
	defaultPurgeBatch = 50
	maxPurgeBatch     = 500
)

// ConfigIntReader 读整数系统配置（缺失或非法返回 fallback）。
// 与 logcenter 的同名适配器同语义，由装配层注入，避免本包依赖 system 仓储。
type ConfigIntReader func(ctx context.Context, key string, fallback int) int

// DeletionService 用户注销与留存期清理（doc104 §4）。
type DeletionService interface {
	// Check 注销前置校验：返回阻断项与警告项。
	Check(ctx context.Context, id uint64) (*dto.UserDeletionCheckResponse, error)
	// SoftDelete 注销单个用户；force=false 时存在警告项即拒绝。
	SoftDelete(ctx context.Context, id uint64, reason string, force bool, operatorID uint64) error
	// Restore 恢复已注销用户。
	Restore(ctx context.Context, id uint64) error
	// BatchSoftDelete 批量注销，返回成功数与逐条跳过原因。
	BatchSoftDelete(ctx context.Context, ids []uint64, reason string, force bool, operatorID uint64) (*dto.UserBatchResult, error)
	// BatchRestore 批量恢复。
	BatchRestore(ctx context.Context, ids []uint64) (*dto.UserBatchResult, error)
	// Purge 留存期清理。dryRun=true 只预览不写库。
	Purge(ctx context.Context, dryRun bool, limit int) (*dto.UserPurgeResponse, error)
}

type deletionService struct {
	repo repository.UserRepository
	cfg  ConfigIntReader
	// now 便于测试注入时钟；生产走 time.Now。
	now func() time.Time
}

// NewDeletionService 创建注销服务。cfg 可为 nil（此时一律用默认留存期）。
func NewDeletionService(repo repository.UserRepository, cfg ConfigIntReader) DeletionService {
	return &deletionService{repo: repo, cfg: cfg, now: time.Now}
}

// retentionDays 解析留存期。配置缺失/非正数一律回落默认值 ——
// 留存期配成 0 或负数意味着「立刻硬删除」，那是配置事故，不能让它生效。
func (s *deletionService) retentionDays(ctx context.Context) int {
	if s.cfg == nil {
		return DefaultDeletionRetentionDays
	}
	days := s.cfg(ctx, "user.deletion_retention_days", DefaultDeletionRetentionDays)
	if days <= 0 {
		return DefaultDeletionRetentionDays
	}
	return days
}

// Check 注销前置校验。
//
// 分级（doc104 §4.4）：
//   - 阻断（force 无效）：在管实例。instances.user_id 是全库唯一指向 users 的外键，
//     硬删除必然撞 23503，只能先释放实例。
//   - 警告（force 可绕过）：余额非零、未结账单、未处理工单、未完成订单。
//     这些在库层面不阻断删除，但会造成用户资产/待办凭空消失，必须让操作人显式确认。
func (s *deletionService) Check(ctx context.Context, id uint64) (*dto.UserDeletionCheckResponse, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.DeletionCheck(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := &dto.UserDeletionCheckResponse{
		UserID:   user.ID,
		Username: user.Username,
		Blockers: []dto.UserDeletionBlockerItem{},
		Warnings: []dto.UserDeletionBlockerItem{},
	}
	if row.Instances > 0 {
		resp.Blockers = append(resp.Blockers, dto.UserDeletionBlockerItem{
			Code: "instances", Label: "在管实例", Count: row.Instances,
		})
	}
	if row.BalanceNonZero > 0 {
		resp.Warnings = append(resp.Warnings, dto.UserDeletionBlockerItem{
			Code: "balance", Label: "账户余额非零", Count: row.BalanceNonZero,
		})
	}
	if row.UnpaidBills > 0 {
		resp.Warnings = append(resp.Warnings, dto.UserDeletionBlockerItem{
			Code: "unpaid_bills", Label: "未结清账单", Count: row.UnpaidBills,
		})
	}
	if row.OpenTickets > 0 {
		resp.Warnings = append(resp.Warnings, dto.UserDeletionBlockerItem{
			Code: "open_tickets", Label: "未处理工单", Count: row.OpenTickets,
		})
	}
	if row.OpenOrders > 0 {
		resp.Warnings = append(resp.Warnings, dto.UserDeletionBlockerItem{
			Code: "open_orders", Label: "未完成订单", Count: row.OpenOrders,
		})
	}
	resp.CanDelete = len(resp.Blockers) == 0
	return resp, nil
}

// SoftDelete 注销单个用户。
//
// 状态收敛（用户选定的方案）：注销 = deleted_at 非空 + status=cancelled，
// 并记录 status_before_delete 供恢复时精确还原。不新增枚举值 —— 前端的状态
// 下拉、列表筛选枚举都已含 cancelled，多一个「deleted」状态只会到处漏改。
func (s *deletionService) SoftDelete(ctx context.Context, id uint64, reason string, force bool, operatorID uint64) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("注销原因不能为空")
	}
	// 操作人必填：注销是高危不可逆动作（对用户而言），留痕缺失时无法追责。
	// 既有代码里那种 `operatorID := uint64(1)` 的兜底（doc104 §3.1 F5）在这里被明确拒绝。
	if operatorID == 0 {
		return ErrInvalidOperator
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if user.DeletedAt != nil {
		return ErrUserDeleted
	}

	check, err := s.Check(ctx, id)
	if err != nil {
		return err
	}
	if len(check.Blockers) > 0 {
		return fmt.Errorf("%w：%s", ErrDeletionBlocked, blockerSummary(check.Blockers))
	}
	if len(check.Warnings) > 0 && !force {
		return fmt.Errorf("%w：%s", ErrDeletionNeedsForce, blockerSummary(check.Warnings))
	}

	statusBefore := user.Status
	if !model.IsValidStatus(statusBefore) {
		statusBefore = model.StatusActive
	}
	return s.repo.SoftDelete(ctx, id, operatorID, reason, statusBefore, s.now())
}

// Restore 恢复已注销用户。
//
// 恢复到 status_before_delete（而不是 active）：注销前若是 disabled，
// 恢复成 active 等于顺手解封，属于越权；仓储层在快照为空时回落 disabled。
func (s *deletionService) Restore(ctx context.Context, id uint64) error {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if user.DeletedAt == nil {
		return ErrUserNotDeleted
	}
	// 恢复成 cancelled 没有意义（等于没恢复），显式回落 disabled。
	restoreStatus := user.StatusBeforeDelete
	if restoreStatus == "" || restoreStatus == model.StatusCancelled {
		restoreStatus = model.StatusDisabled
	}
	// 冲突检测交给数据库：注销期间 username/email 已被新用户占用时，
	// 清 deleted_at 会撞上部分唯一索引（uk_users_username_active / uk_users_email_active）。
	// 捕获 23505 并翻成明确的 409，而不是把 PG 原文抛成 500。
	if err := s.repo.Restore(ctx, id, restoreStatus); err != nil {
		if isUniqueViolation(err) {
			return ErrRestoreConflict
		}
		return err
	}
	return nil
}

// BatchSoftDelete 批量注销。
//
// 逐条独立处理而不是整批事务：40 个用户里有一个带在管实例，不应该让另外 39 个
// 一起失败。跳过项带原因回给前端逐条展示。
func (s *deletionService) BatchSoftDelete(ctx context.Context, ids []uint64, reason string, force bool, operatorID uint64) (*dto.UserBatchResult, error) {
	result := &dto.UserBatchResult{Skipped: []dto.UserBatchSkipItem{}}
	for _, id := range uniqueIDs(ids) {
		err := s.SoftDelete(ctx, id, reason, force, operatorID)
		if err != nil {
			result.Skipped = append(result.Skipped, dto.UserBatchSkipItem{ID: id, Reason: batchSkipReason(err)})
			continue
		}
		result.Affected++
	}
	return result, nil
}

// BatchRestore 批量恢复。语义同批量注销。
func (s *deletionService) BatchRestore(ctx context.Context, ids []uint64) (*dto.UserBatchResult, error) {
	result := &dto.UserBatchResult{Skipped: []dto.UserBatchSkipItem{}}
	for _, id := range uniqueIDs(ids) {
		if err := s.Restore(ctx, id); err != nil {
			result.Skipped = append(result.Skipped, dto.UserBatchSkipItem{ID: id, Reason: batchSkipReason(err)})
			continue
		}
		result.Affected++
	}
	return result, nil
}

// Purge 留存期清理（doc104 §4.5）。
//
// dryRun=true 只返回候选列表，一个字节都不写 —— 运营需要先看清「这一轮会删掉谁」
// 再决定是否真的执行。这与 logcenter 的清理任务同款做法。
func (s *deletionService) Purge(ctx context.Context, dryRun bool, limit int) (*dto.UserPurgeResponse, error) {
	if limit <= 0 {
		limit = defaultPurgeBatch
	}
	if limit > maxPurgeBatch {
		limit = maxPurgeBatch
	}
	days := s.retentionDays(ctx)
	cutoff := s.now().AddDate(0, 0, -days)

	// 多取 1 条判断是否还有积压，日志与前端都能据此提示「下一轮还有」。
	candidates, err := s.repo.ListPurgeCandidates(ctx, cutoff, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(candidates) > limit
	if hasMore {
		candidates = candidates[:limit]
	}

	resp := &dto.UserPurgeResponse{
		RetentionDays: days,
		Cutoff:        cutoff,
		DryRun:        dryRun,
		Candidates:    make([]dto.UserPurgePreviewItem, 0, len(candidates)),
		Skipped:       []dto.UserBatchSkipItem{},
		HasMore:       hasMore,
	}
	for i := range candidates {
		resp.Candidates = append(resp.Candidates, dto.UserPurgePreviewItem{
			ID:        candidates[i].ID,
			Username:  candidates[i].Username,
			DeletedAt: candidates[i].DeletedAt,
			Reason:    candidates[i].DeleteReason,
		})
	}
	if dryRun {
		return resp, nil
	}

	for i := range candidates {
		id := candidates[i].ID
		if err := s.repo.PurgeUser(ctx, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 并发下已被另一轮删掉：不算失败。
				continue
			}
			// 在管实例等硬阻断：跳过并记录，不中断整轮（其余用户照常清理）。
			resp.Skipped = append(resp.Skipped, dto.UserBatchSkipItem{ID: id, Reason: batchSkipReason(err)})
			continue
		}
		resp.Purged++
	}
	return resp, nil
}

// blockerSummary 把阻断项拼成一句人话，供服务端日志与错误消息复用。
func blockerSummary(items []dto.UserDeletionBlockerItem) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprintf("%s %d 项", it.Label, it.Count))
	}
	return strings.Join(parts, "、")
}

// batchSkipReason 把错误转成给运营看的一句原因（避免把 gorm 原文暴露出去）。
func batchSkipReason(err error) string {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return "用户不存在"
	case errors.Is(err, ErrUserDeleted):
		return "用户已注销"
	case errors.Is(err, ErrUserNotDeleted):
		return "用户未注销"
	case errors.Is(err, repository.ErrUserHasInstances):
		return "存在未释放的实例"
	default:
		return err.Error()
	}
}

// uniqueIDs 去重并剔除 0，避免同一批里重复注销导致的假失败。
func uniqueIDs(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// ParseRetentionDays 解析配置文本，非法值返回 fallback。
// 导出给装配层复用，避免各处重复实现一遍 strconv 容错。
func ParseRetentionDays(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
