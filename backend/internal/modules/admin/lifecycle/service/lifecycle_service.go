package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	lifecycledto "hostsent/backend/internal/modules/admin/lifecycle/dto"
	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
	lifecyclerepo "hostsent/backend/internal/modules/admin/lifecycle/repository"
)

// Notifier 通知中心联动点（通知展示与投递由通知中心模块承接）。
type Notifier interface {
	// Publish 发布用户通知（到期提醒、续费结果等）。
	Publish(ctx context.Context, userID uint64, title, content string) error
}

// noopNotifier 阶段一空实现：仅由调用方记录日志。
type noopNotifier struct{}

func (noopNotifier) Publish(_ context.Context, _ uint64, _ string, _ string) error { return nil }

// LifecycleService 生命周期服务：策略、到期视图、调度扫描。
type LifecycleService interface {
	// GetPolicy 获取全局生命周期策略
	GetPolicy(ctx context.Context) (*lifecycledto.PolicyResponse, error)
	// UpdatePolicy 更新全局生命周期策略
	UpdatePolicy(ctx context.Context, req *lifecycledto.PolicyUpdateRequest) (*lifecycledto.PolicyResponse, error)
	// DeriveStage 按到期时间与策略推导生命周期阶段
	DeriveStage(expireAt, now time.Time, policy *lifecyclemodel.LifecyclePolicy) string
	// ListExpiring 到期实例分页列表（按阶段筛选）
	ListExpiring(ctx context.Context, q *lifecycledto.ExpiringListQuery) (*lifecycledto.ExpiringListResponse, error)
	// RunScanOnce 手动触发一轮扫描（提醒 + 自动续费），供运维接口调用
	RunScanOnce(ctx context.Context) error
	// SetNotifier 注入通知实现（通知中心模块就绪后替换空实现）
	SetNotifier(n Notifier)
}

type lifecycleService struct {
	db           *gorm.DB
	policyRepo   lifecyclerepo.PolicyRepository
	instanceRepo lifecyclerepo.InstanceReader
	renewalSvc   RenewalService
	notifier     Notifier
	logger       *zap.Logger
}

// NewLifecycleService 创建生命周期服务。
func NewLifecycleService(
	db *gorm.DB,
	policyRepo lifecyclerepo.PolicyRepository,
	instanceRepo lifecyclerepo.InstanceReader,
	renewalSvc RenewalService,
	logger *zap.Logger,
) LifecycleService {
	return &lifecycleService{
		db:           db,
		policyRepo:   policyRepo,
		instanceRepo: instanceRepo,
		renewalSvc:   renewalSvc,
		notifier:     noopNotifier{},
		logger:       logger,
	}
}

func (s *lifecycleService) SetNotifier(n Notifier) {
	if n != nil {
		s.notifier = n
	}
}

func (s *lifecycleService) GetPolicy(ctx context.Context) (*lifecycledto.PolicyResponse, error) {
	policy, err := s.loadPolicy(ctx)
	if err != nil {
		return nil, err
	}
	return buildPolicyResponse(policy), nil
}

func (s *lifecycleService) UpdatePolicy(ctx context.Context, req *lifecycledto.PolicyUpdateRequest) (*lifecycledto.PolicyResponse, error) {
	if req.RemindDays != "" {
		if _, err := parseRemindDays(req.RemindDays); err != nil {
			return nil, ErrPolicyInvalid
		}
	}
	policy, err := s.loadPolicy(ctx)
	if err != nil {
		return nil, err
	}
	if req.RemindDays != "" {
		policy.RemindDays = strings.ReplaceAll(req.RemindDays, " ", "")
	}
	if req.AutoRenewDefault != nil {
		policy.AutoRenewDefault = *req.AutoRenewDefault
	}
	if req.GraceDays != nil {
		if *req.GraceDays < 0 || *req.GraceDays > 365 {
			return nil, ErrPolicyInvalid
		}
		policy.GraceDays = *req.GraceDays
	}
	if req.DestroyKeepDays != nil {
		if *req.DestroyKeepDays < 0 || *req.DestroyKeepDays > 3650 {
			return nil, ErrPolicyInvalid
		}
		policy.DestroyKeepDays = *req.DestroyKeepDays
	}
	if err := s.policyRepo.Update(ctx, policy); err != nil {
		return nil, err
	}
	return buildPolicyResponse(policy), nil
}

// DeriveStage 生命周期派生状态（不落库到 instances.status，避免与上游同步冲突）：
// active: expire_at > now；grace: 到期后宽限期内；suspended: 宽限期结束；destroyed: 保留期结束。
func (s *lifecycleService) DeriveStage(expireAt, now time.Time, policy *lifecyclemodel.LifecyclePolicy) string {
	graceEnd := expireAt.AddDate(0, 0, policy.GraceDays)
	destroyEnd := graceEnd.AddDate(0, 0, policy.DestroyKeepDays)
	switch {
	case now.Before(expireAt):
		return lifecyclemodel.StageActive
	case now.Before(graceEnd):
		return lifecyclemodel.StageGrace
	case now.Before(destroyEnd):
		return lifecyclemodel.StageSuspended
	default:
		return lifecyclemodel.StageDestroyed
	}
}

func (s *lifecycleService) ListExpiring(ctx context.Context, q *lifecycledto.ExpiringListQuery) (*lifecycledto.ExpiringListResponse, error) {
	policy, err := s.loadPolicy(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	window := stageWindow(q.Stage, now, policy)
	items, total, err := s.instanceRepo.ListByStage(ctx, window, q.Keyword, q.Page, q.PageSize)
	if err != nil {
		return nil, err
	}
	resp := make([]lifecycledto.ExpiringInstanceItem, 0, len(items))
	for _, item := range items {
		expireAt := time.Time{}
		if item.ExpireAt != nil {
			expireAt = *item.ExpireAt
		}
		productName, unitPrice, perr := s.instanceRepo.ResolveProduct(ctx, &item.Instance)
		if perr != nil {
			return nil, perr
		}
		resp = append(resp, lifecycledto.ExpiringInstanceItem{
			ID:           item.ID,
			InstanceMark: item.InstanceID,
			Name:         item.Name,
			UserID:       item.UserID,
			Username:     item.Username,
			ProductID:    item.ProductID,
			ProductName:  productName,
			UnitPrice:    unitPrice,
			BillingMode:  item.BillingMode,
			Status:       item.Status,
			ExpireAt:     formatTime(item.ExpireAt),
			DaysLeft:     daysLeft(expireAt, now),
			Stage:        s.DeriveStage(expireAt, now, policy),
			AutoRenew:    item.AutoRenew,
		})
	}
	page, pageSize := normalizePage(q.Page, q.PageSize)
	return &lifecycledto.ExpiringListResponse{
		Items: resp,
		Meta:  lifecycledto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

// RunScanOnce 单轮扫描：到期提醒 → 自动续费。
// 宽限期/暂停/销毁为派生状态，无需落库推进；阶段二经 provider adapter 联动上游动作。
func (s *lifecycleService) RunScanOnce(ctx context.Context) error {
	if err := s.SendDueReminders(ctx); err != nil {
		s.logger.Error("send due reminders failed", zap.Error(err))
	}
	if err := s.renewalSvc.ProcessAutoRenewals(ctx); err != nil {
		return err
	}
	return nil
}

// SendDueReminders 对策略提醒天数命中的实例发布到期提醒（通知中心未就绪时仅记录日志）。
func (s *lifecycleService) SendDueReminders(ctx context.Context) error {
	policy, err := s.loadPolicy(ctx)
	if err != nil {
		return err
	}
	remindDays, err := parseRemindDays(policy.RemindDays)
	if err != nil || len(remindDays) == 0 {
		return nil
	}
	// 提醒窗口上限 = 最大提醒天数
	maxDay := 0
	for _, d := range remindDays {
		if d > maxDay {
			maxDay = d
		}
	}
	now := time.Now()
	upper := now.AddDate(0, 0, maxDay+1)
	items, _, err := s.instanceRepo.ListByStage(ctx, &lifecyclerepo.StageWindow{ExpireAfter: &now, ExpireBefore: &upper}, "", 1, 200)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ExpireAt == nil {
			continue
		}
		days := daysLeft(*item.ExpireAt, now)
		if !containsInt(remindDays, days) {
			continue
		}
		title := "云主机到期提醒"
		content := "您的云主机 " + item.Name + "（" + item.InstanceID + "）将于 " +
			item.ExpireAt.Format("2006-01-02 15:04") + " 到期，剩余 " + strconv.Itoa(days) + " 天，请及时续费。"
		if err := s.notifier.Publish(ctx, item.UserID, title, content); err != nil {
			s.logger.Warn("publish remind notification failed",
				zap.Uint64("user_id", item.UserID), zap.Error(err))
		} else {
			s.logger.Info("renew remind published",
				zap.Uint64("user_id", item.UserID), zap.String("instance", item.InstanceID), zap.Int("days_left", days))
		}
	}
	return nil
}

// loadPolicy 读取策略（不存在时注入默认行）。
func (s *lifecycleService) loadPolicy(ctx context.Context) (*lifecyclemodel.LifecyclePolicy, error) {
	policy, err := s.policyRepo.Get(ctx)
	if err == nil {
		return policy, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	// 单行 seed 缺失时兜底创建
	return ensureDefaultPolicyRow(s.db.WithContext(ctx))
}

// ensureDefaultPolicyRow 策略单行缺失时注入默认行（ID=1：7,3,1 / 7 天宽限 / 30 天保留）。
func ensureDefaultPolicyRow(tx *gorm.DB) (*lifecyclemodel.LifecyclePolicy, error) {
	policy := &lifecyclemodel.LifecyclePolicy{
		ID:               1,
		RemindDays:       "7,3,1",
		AutoRenewDefault: false,
		GraceDays:        7,
		DestroyKeepDays:  30,
		Status:           "active",
	}
	if err := tx.Create(policy).Error; err != nil {
		return nil, err
	}
	return policy, nil
}

// stageWindow 将阶段筛选转换为 expire_at 时间窗口。
func stageWindow(stage string, now time.Time, policy *lifecyclemodel.LifecyclePolicy) *lifecyclerepo.StageWindow {
	graceEnd := now.AddDate(0, 0, -policy.GraceDays)
	destroyEnd := graceEnd.AddDate(0, 0, -policy.DestroyKeepDays)
	expireAt := func(t time.Time) *time.Time { return &t }
	switch stage {
	case "active":
		return &lifecyclerepo.StageWindow{ExpireAfter: expireAt(now)}
	case "expiring": // 即将到期（30 天内）
		return &lifecyclerepo.StageWindow{ExpireAfter: expireAt(now), ExpireBefore: expireAt(now.AddDate(0, 0, 30))}
	case lifecyclemodel.StageGrace:
		// 宽限期中：now-graceDays < expire_at <= now
		return &lifecyclerepo.StageWindow{ExpireAfter: expireAt(graceEnd), ExpireBefore: expireAt(now)}
	case lifecyclemodel.StageSuspended:
		return &lifecyclerepo.StageWindow{ExpireAfter: expireAt(destroyEnd), ExpireBefore: expireAt(graceEnd)}
	case lifecyclemodel.StageDestroyed:
		return &lifecyclerepo.StageWindow{ExpireBefore: expireAt(destroyEnd)}
	default:
		return nil // 全部
	}
}

// parseRemindDays 解析提醒天数配置 "7,3,1" → [7 3 1]。
func parseRemindDays(raw string) ([]int, error) {
	raw = strings.ReplaceAll(raw, " ", "")
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	days := make([]int, 0, len(parts))
	for _, part := range parts {
		d, err := strconv.Atoi(part)
		if err != nil || d < 0 || d > 365 {
			return nil, ErrPolicyInvalid
		}
		days = append(days, d)
	}
	return days, nil
}

// daysLeft 剩余天数（向上取整；负数表示已过期天数）。
func daysLeft(expireAt, now time.Time) int {
	diff := expireAt.Sub(now)
	sign := 1
	if diff < 0 {
		sign = -1
		diff = -diff
	}
	days := int((diff + 23*time.Hour) / (24 * time.Hour))
	return sign * days
}

func containsInt(list []int, target int) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 10
	}
	return page, pageSize
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func buildPolicyResponse(policy *lifecyclemodel.LifecyclePolicy) *lifecycledto.PolicyResponse {
	return &lifecycledto.PolicyResponse{
		RemindDays:       policy.RemindDays,
		AutoRenewDefault: policy.AutoRenewDefault,
		GraceDays:        policy.GraceDays,
		DestroyKeepDays:  policy.DestroyKeepDays,
		UpdatedAt:        policy.UpdatedAt.Format(time.RFC3339),
	}
}
