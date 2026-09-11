package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/order/model"
)

// ProvisionTaskStore 工作池需要的任务仓储能力（由 admin/order/repository 实现）。
type ProvisionTaskStore interface {
	Claim(ctx context.Context, lease time.Duration) (*model.ProvisionTask, error)
	MarkSuccess(ctx context.Context, id uint64) error
	MarkRetry(ctx context.Context, id uint64, msg string, nextRetry time.Time) error
	MarkManual(ctx context.Context, id uint64, msg string) error
	ReleaseStale(ctx context.Context, now time.Time) (int64, error)
}

// ProvisionActivator 执行一次开通（admin OrderService.Activate 满足该签名）。
type ProvisionActivator interface {
	Activate(ctx context.Context, id uint64) error
}

// ProvisionNotifier 连续失败转人工队列时的告警回调（可选，通知中心装配层注入）。
type ProvisionNotifier func(ctx context.Context, task *model.ProvisionTask)

// ProvisionWorker 开通履约工作池（T5.1）。
//
// 设计（doc15 §6.1 / doc17 §4-T5.1，边界：不引入消息队列中间件）：
//   - DB 任务表 provision_tasks + 进程内固定数量协程轮询领取；
//   - 单条 UPDATE ... FOR UPDATE SKIP LOCKED 领取，多协程/多实例不重复开通；
//   - 失败保留订单 paid 可重试（OrderService.Activate 既有语义），按 attempts 线性退避；
//   - 达到上限转人工队列并告警；租约过期的 running 任务定期回收，避免进程崩溃后僵死。
type ProvisionWorker struct {
	store     ProvisionTaskStore
	activator ProvisionActivator
	logger    *zap.Logger
	notify    ProvisionNotifier

	workers    int
	pollEvery  time.Duration
	lease      time.Duration
	backoffMin time.Duration

	wg     sync.WaitGroup
	cancel context.CancelFunc
	once   sync.Once
}

// ProvisionWorkerOptions 工作池可调参数（零值取默认）。
type ProvisionWorkerOptions struct {
	Workers    int           // 并发协程数，默认 2
	PollEvery  time.Duration // 空轮询间隔，默认 2s
	Lease      time.Duration // 任务租约，默认 10min（上游开通约 50s，留足余量）
	BackoffMin time.Duration // 失败重试最小退避，默认 30s（乘以 attempts）
}

// NewProvisionWorker 创建开通履约工作池。
func NewProvisionWorker(store ProvisionTaskStore, activator ProvisionActivator, notify ProvisionNotifier, logger *zap.Logger, opts ProvisionWorkerOptions) *ProvisionWorker {
	w := &ProvisionWorker{
		store:      store,
		activator:  activator,
		logger:     logger,
		notify:     notify,
		workers:    opts.Workers,
		pollEvery:  opts.PollEvery,
		lease:      opts.Lease,
		backoffMin: opts.BackoffMin,
	}
	if w.workers <= 0 {
		w.workers = 2
	}
	if w.pollEvery <= 0 {
		w.pollEvery = 2 * time.Second
	}
	if w.lease <= 0 {
		w.lease = 10 * time.Minute
	}
	if w.backoffMin <= 0 {
		w.backoffMin = 30 * time.Second
	}
	return w
}

// Start 启动工作池；ctx 取消时退出。幂等：重复调用只生效一次。
func (w *ProvisionWorker) Start(ctx context.Context) {
	w.once.Do(func() {
		runCtx, cancel := context.WithCancel(ctx)
		w.cancel = cancel
		for i := 0; i < w.workers; i++ {
			w.wg.Add(1)
			go w.loop(runCtx)
		}
		w.logger.Info("provision worker started",
			zap.Int("workers", w.workers), zap.Duration("poll_every", w.pollEvery))
	})
}

// Stop 停止工作池并等待在途任务收尾（受 ctx 超时约束）。
func (w *ProvisionWorker) Stop(ctx context.Context) {
	if w.cancel != nil {
		w.cancel()
	}
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// loop 单协程轮询：领取任务 → 执行 → 记账；无任务时按间隔休眠。
func (w *ProvisionWorker) loop(ctx context.Context) {
	defer w.wg.Done()
	reapT := time.NewTicker(time.Minute)
	defer reapT.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-reapT.C:
			w.reapStale(ctx)
		default:
		}

		task, err := w.store.Claim(ctx, w.lease)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("provision worker claim failed", zap.Error(err))
			w.sleep(ctx, w.pollEvery)
			continue
		}
		if task == nil {
			w.sleep(ctx, w.pollEvery)
			continue
		}
		w.process(ctx, task)
	}
}

// process 执行单个开通任务并落库结果。
// 使用独立 context：即使停机信号到达，也让本次上游调用收尾，避免
// "上游已开通、本地未记账"的半完成状态（租约回收仍可兜底）。
func (w *ProvisionWorker) process(_ context.Context, task *model.ProvisionTask) {
	runCtx, cancel := context.WithTimeout(context.Background(), w.lease)
	defer cancel()

	start := time.Now()
	err := w.activator.Activate(runCtx, task.OrderID)
	if err == nil {
		if merr := w.store.MarkSuccess(runCtx, task.ID); merr != nil {
			w.logger.Error("provision task mark success failed",
				zap.Uint64("task_id", task.ID), zap.Uint64("order_id", task.OrderID), zap.Error(merr))
		}
		w.logger.Info("provision task succeeded",
			zap.Uint64("task_id", task.ID), zap.Uint64("order_id", task.OrderID),
			zap.Int("attempts", task.Attempts), zap.Duration("elapsed", time.Since(start)))
		return
	}

	msg := err.Error()
	if errors.Is(err, ErrStatusConflict) {
		// 订单已处于终态（已退款/已取消）或已 active：任务无意义，判成功收尾避免无限重试。
		_ = w.store.MarkSuccess(runCtx, task.ID)
		w.logger.Info("provision task closed by order state",
			zap.Uint64("task_id", task.ID), zap.Uint64("order_id", task.OrderID), zap.Error(err))
		return
	}

	if task.MaxAttempts > 0 && task.Attempts >= task.MaxAttempts {
		if merr := w.store.MarkManual(runCtx, task.ID, msg); merr != nil {
			w.logger.Error("provision task mark manual failed", zap.Uint64("task_id", task.ID), zap.Error(merr))
		}
		manual := *task
		manual.Status = model.ProvisionTaskManual
		manual.LastError = msg
		w.notifyManual(runCtx, &manual)
		return
	}

	next := time.Now().Add(w.retryDelay(task.Attempts))
	if merr := w.store.MarkRetry(runCtx, task.ID, msg, next); merr != nil {
		w.logger.Error("provision task mark retry failed", zap.Uint64("task_id", task.ID), zap.Error(merr))
	}
	w.logger.Warn("provision task failed, scheduled retry",
		zap.Uint64("task_id", task.ID), zap.Uint64("order_id", task.OrderID),
		zap.Int("attempts", task.Attempts), zap.Int("max_attempts", task.MaxAttempts),
		zap.Time("next_retry", next), zap.Error(err))
}

// retryDelay 线性退避：attempts × backoffMin，上限 30 分钟。
func (w *ProvisionWorker) retryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := time.Duration(attempts) * w.backoffMin
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	return delay
}

// reapStale 回收租约过期的 running 任务，避免进程崩溃后任务僵死。
func (w *ProvisionWorker) reapStale(ctx context.Context) {
	n, err := w.store.ReleaseStale(ctx, time.Now())
	if err != nil {
		w.logger.Error("provision worker reap stale failed", zap.Error(err))
		return
	}
	if n > 0 {
		w.logger.Warn("provision worker released stale tasks", zap.Int64("rows", n))
	}
}

// notifyManual 转人工队列告警；未注入通知时仅记录错误日志。
func (w *ProvisionWorker) notifyManual(ctx context.Context, task *model.ProvisionTask) {
	w.logger.Error("provision task moved to manual queue",
		zap.Uint64("task_id", task.ID), zap.Uint64("order_id", task.OrderID),
		zap.Uint64("user_id", task.UserID), zap.Int("attempts", task.Attempts), zap.String("last_error", task.LastError))
	if w.notify != nil {
		w.notify(ctx, task)
	}
}

// sleep 可被 ctx 取消的休眠。
func (w *ProvisionWorker) sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
