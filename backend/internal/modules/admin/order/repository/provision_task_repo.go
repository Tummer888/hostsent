package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/order/model"
)

// ProvisionTaskRepository 开通履约任务仓储（T5.1）。
type ProvisionTaskRepository interface {
	// Enqueue 幂等投递：order_id 冲突时保留既有行（不覆盖已成功/人工的任务）。
	// 若既有行为 failed（可重试），则重置为 pending 并清空租约，让工作池立即重试。
	Enqueue(ctx context.Context, task *model.ProvisionTask) error
	// Claim 原子领取一条待处理任务（FOR UPDATE SKIP LOCKED），无任务返回 nil。
	// 领取时置 running、attempts++、写入租约 locked_until。
	Claim(ctx context.Context, lease time.Duration) (*model.ProvisionTask, error)
	// MarkSuccess 标记任务成功（终态）。
	MarkSuccess(ctx context.Context, id uint64) error
	// MarkRetry 记录一次失败并安排退避重试：置 failed，locked_until=下次可领取时间。
	MarkRetry(ctx context.Context, id uint64, msg string, nextRetry time.Time) error
	// MarkManual 连续失败达上限：置 manual 转人工处理队列（终态，需人工介入）。
	MarkManual(ctx context.Context, id uint64, msg string) error
	// ReleaseStale 回收租约过期的 running 任务，重置为 pending 供重试；返回回收条数。
	ReleaseStale(ctx context.Context, now time.Time) (int64, error)
	// FindByOrderID 按订单查询任务（订单列表展示履约状态）。
	FindByOrderID(ctx context.Context, orderID uint64) (*model.ProvisionTask, error)
	// FindByOrderIDs 批量查询任务：orderID → task（订单列表避免 N+1）。
	FindByOrderIDs(ctx context.Context, orderIDs []uint64) (map[uint64]*model.ProvisionTask, error)
	// ListManual 人工处理队列（连续失败/人工介入）分页。
	ListManual(ctx context.Context, page, pageSize int) ([]model.ProvisionTask, int64, error)
}

type provisionTaskRepository struct {
	db *gorm.DB
}

// NewProvisionTaskRepository 创建开通任务仓储。
func NewProvisionTaskRepository(db *gorm.DB) ProvisionTaskRepository {
	return &provisionTaskRepository{db: db}
}

func (r *provisionTaskRepository) Enqueue(ctx context.Context, task *model.ProvisionTask) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "order_id"}},
		// 已存在则仅当处于 failed（可重试）时重置为 pending；success/manual/running/pending 保持原样，
		// 避免重复投递把已完成任务或人工队列任务打回。
		DoUpdates: clause.Assignments(map[string]interface{}{
			"status": gorm.Expr(
				"CASE WHEN provision_tasks.status = ? THEN ? ELSE provision_tasks.status END",
				model.ProvisionTaskFailed, model.ProvisionTaskPending,
			),
			"locked_until": gorm.Expr(
				"CASE WHEN provision_tasks.status = ? THEN NULL ELSE provision_tasks.locked_until END",
				model.ProvisionTaskFailed,
			),
			"updated_at": time.Now(),
		}),
	}).Create(task).Error
}

// claimSQL 原子领取：用 FOR UPDATE SKIP LOCKED 选出可执行行并直接更新为 running。
// 单条 UPDATE ... RETURNING 完成"选取 + 占用"，多实例/多协程并发下不会重复领取。
//
// 可执行 = 待处理/上次失败已过退避时间，或 running 但租约已过期（僵死回收）。
// locked_until 双重语义：running 时是租约到期；failed 时是下次可重试时间。
const claimSQL = `
UPDATE provision_tasks
SET status = 'running',
    attempts = attempts + 1,
    started_at = COALESCE(started_at, now()),
    locked_until = now() + make_interval(secs => ?),
    updated_at = now()
WHERE id = (
    SELECT id FROM provision_tasks
    WHERE (status IN ('pending','failed') AND (locked_until IS NULL OR locked_until <= now()))
       OR (status = 'running' AND locked_until IS NOT NULL AND locked_until < now())
    ORDER BY created_at ASC, id ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, order_id, user_id, product_id, COALESCE(source_mode, ''), status, attempts, max_attempts,
          COALESCE(last_error, ''), locked_until, started_at, completed_at, created_at, updated_at`

func (r *provisionTaskRepository) Claim(ctx context.Context, lease time.Duration) (*model.ProvisionTask, error) {
	seconds := lease.Seconds()
	if seconds <= 0 {
		seconds = 600
	}
	var task model.ProvisionTask
	res := r.db.WithContext(ctx).Raw(claimSQL, seconds).Scan(&task)
	if res.Error != nil {
		return nil, res.Error
	}
	if task.ID == 0 {
		return nil, nil
	}
	return &task, nil
}

func (r *provisionTaskRepository) MarkSuccess(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ProvisionTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       model.ProvisionTaskSuccess,
			"completed_at": &now,
			"locked_until": nil,
			"last_error":   "",
		}).Error
}

func (r *provisionTaskRepository) MarkRetry(ctx context.Context, id uint64, msg string, nextRetry time.Time) error {
	return r.db.WithContext(ctx).Model(&model.ProvisionTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       model.ProvisionTaskFailed,
			"last_error":   msg,
			"locked_until": &nextRetry, // failed 行的 locked_until 表示"下次可领取时间"
		}).Error
}

func (r *provisionTaskRepository) MarkManual(ctx context.Context, id uint64, msg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ProvisionTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       model.ProvisionTaskManual,
			"last_error":   msg,
			"locked_until": nil,
			"completed_at": &now,
		}).Error
}

func (r *provisionTaskRepository) ReleaseStale(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.ProvisionTask{}).
		Where("status = ? AND locked_until IS NOT NULL AND locked_until < ?", model.ProvisionTaskRunning, now).
		Updates(map[string]interface{}{
			"status":       model.ProvisionTaskPending,
			"locked_until": nil,
		})
	return res.RowsAffected, res.Error
}

func (r *provisionTaskRepository) FindByOrderID(ctx context.Context, orderID uint64) (*model.ProvisionTask, error) {
	var task model.ProvisionTask
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *provisionTaskRepository) FindByOrderIDs(ctx context.Context, orderIDs []uint64) (map[uint64]*model.ProvisionTask, error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}
	var tasks []model.ProvisionTask
	if err := r.db.WithContext(ctx).Where("order_id IN ?", orderIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]*model.ProvisionTask, len(tasks))
	for i := range tasks {
		out[tasks[i].OrderID] = &tasks[i]
	}
	return out, nil
}

func (r *provisionTaskRepository) ListManual(ctx context.Context, page, pageSize int) ([]model.ProvisionTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	base := r.db.WithContext(ctx).Model(&model.ProvisionTask{}).Where("status = ?", model.ProvisionTaskManual)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.ProvisionTask
	if err := base.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
