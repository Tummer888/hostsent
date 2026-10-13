package service

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/pkg/jobrun"
	"hostsent/backend/internal/pkg/safego"
)

// ProvisioningStaleStore 扫描「开通中」滞留订单（由 orderRepository 实现）。
type ProvisioningStaleStore interface {
	ListProvisioningStale(ctx context.Context, before time.Time, limit int) ([]model.Order, error)
}

// InstanceRefresher 按订单回源刷新其实例并返回最新状态（装配层用实例运维台实现）。
// 无实例（或尚未在上游开通）返回空串，调用方据此跳过。
type InstanceRefresher interface {
	RefreshByOrder(ctx context.Context, orderID uint64) (status string, err error)
}

// ProvisioningPromoter 把「上游主机已就绪」的滞留订单推进到服务中。
//
// 为什么需要它（用户实测）：上游创建主机是**异步**的 —— `apply_credit` 支付成功
// 只代表账单已付、主机开始创建，此时 host/header 的 domainstatus 还是 `Pending`，
// 适配器返回的实例状态为 creating，订单停在 provisioning。
// 而财务型上游（zjmf）**没有实例同步 scope**（ZJMFProvider 不实现 InstanceAdministration），
// 也就是说没有任何后台任务会再来看这台主机 —— 订单与实例会永远停在
// provisioning/creating，直到有人手动点一次「同步刷新」。
//
// 推进策略（幂等、不产生额外上游副作用）：
//  1. 捞「开通中且滞留超过阈值」的订单；
//  2. 回源拉一次主机详情（只读 host/header，不重新下单）；
//  3. 主机已 running（Active）才推进订单到服务中；仍创建中则保持原状，下一轮再看。
type ProvisioningPromoter struct {
	store     ProvisioningStaleStore
	refresher InstanceRefresher
	activator ProvisionActivator
	logger    *zap.Logger

	interval   time.Duration
	staleAfter time.Duration
	batch      int

	backoffBase time.Duration
	backoffMax  time.Duration

	// backoff 记录"上一轮刷新无进展"的订单与其下次可查时间。
	//
	// 为什么需要：主机的 Pending 可能是暂时的（上游排队），也可能是永久的
	// （上游配置/容量问题，实测有卡 40 分钟不动的）。对后者每轮重查一次
	// host/header 既能刷爆上游接口，也会在实例流水里堆出一串一模一样的
	// 系统同步记录（实测某实例 30 分钟内 32 条失败 + 35 条成功）。指数退避
	// 把"等待"的成本降到可忽略，同时保留最终恢复的能力。
	mu      sync.Mutex
	backoff map[uint64]*promoteBackoff
}

// promoteBackoff 单个订单的推进退避状态。
type promoteBackoff struct {
	failures int
	nextAt   time.Time
}

// ProvisioningPromoterOptions 推进器可调参数（零值取默认）。
type ProvisioningPromoterOptions struct {
	Interval   time.Duration // 扫描间隔，默认 1 分钟
	StaleAfter time.Duration // 滞留阈值，默认 2 分钟（留出上游正常开通时间）
	Batch      int           // 单轮上限，默认 100
	// BackoffBase 无进展订单的退避基数，默认 5 分钟；连续无进展时长按
	// BackoffBase × 2^(failures-1) 增长，上限 BackoffMax。
	BackoffBase time.Duration
	// BackoffMax 退避上限，默认 1 小时（沉睡订单仍会周期性复查）。
	BackoffMax time.Duration
}

// NewProvisioningPromoter 创建滞留开通订单推进器。
func NewProvisioningPromoter(store ProvisioningStaleStore, refresher InstanceRefresher, activator ProvisionActivator, logger *zap.Logger, opts ProvisioningPromoterOptions) *ProvisioningPromoter {
	p := &ProvisioningPromoter{
		store:      store,
		refresher:  refresher,
		activator:  activator,
		logger:     logger,
		interval:   opts.Interval,
		staleAfter: opts.StaleAfter,
		batch:      opts.Batch,
		backoff:    map[uint64]*promoteBackoff{},
	}
	if p.interval <= 0 {
		p.interval = time.Minute
	}
	if p.staleAfter <= 0 {
		p.staleAfter = 2 * time.Minute
	}
	if p.batch <= 0 {
		p.batch = 100
	}
	p.backoffBase = opts.BackoffBase
	if p.backoffBase <= 0 {
		p.backoffBase = 5 * time.Minute
	}
	p.backoffMax = opts.BackoffMax
	if p.backoffMax <= 0 {
		p.backoffMax = time.Hour
	}
	return p
}

// backoffFor 查询订单是否处于退避窗口内（是则本轮跳过）。
func (p *ProvisioningPromoter) backoffFor(orderID uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	state, ok := p.backoff[orderID]
	if !ok {
		return false
	}
	return time.Now().Before(state.nextAt)
}

// noteNoProgress 记录一次"回源无进展"并计算指数退避；返回本次退避时长。
//
// 无进展 = 刷新成功但状态仍是 creating（主机还没就绪），或刷新失败/无实例。
// 真正的推进（running）或订单离开 provisionning 会清空状态（forget）。
func (p *ProvisioningPromoter) noteNoProgress(orderID uint64) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.backoff[orderID]
	if state == nil {
		state = &promoteBackoff{}
		p.backoff[orderID] = state
	}
	state.failures++
	wait := p.backoffBase << (state.failures - 1)
	if wait > p.backoffMax || wait <= 0 {
		wait = p.backoffMax
	}
	state.nextAt = time.Now().Add(wait)
	return wait
}

// forget 清除订单的退避状态（已推进/离开开通中，或本轮已确认订单不再滞留）。
func (p *ProvisioningPromoter) forget(orderID uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.backoff, orderID)
}

// Start 启动推进循环；ctx 取消时退出。启动时先跑一轮，进程重启期间的滞留也能被收掉。
func (p *ProvisioningPromoter) Start(ctx context.Context) {
	p.runSafely(ctx)
	go func() {
		defer safego.Recover(p.logger)
		t := time.NewTicker(p.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				p.logger.Info("provisioning promoter stopped")
				return
			case <-t.C:
				p.runSafely(ctx)
			}
		}
	}()
	p.logger.Info("provisioning promoter started",
		zap.Duration("interval", p.interval), zap.Duration("stale_after", p.staleAfter))
}

// runSafely 单轮 panic 防护：单轮异常只记日志，不停摆循环。
func (p *ProvisioningPromoter) runSafely(ctx context.Context) {
	defer safego.Recover(p.logger)
	p.runOnce(ctx)
}

// runOnce 单轮：捞滞留订单 → 回源刷新 → 就绪才推进。
//
// 退避：连续"无进展"的订单按指数退避跳过，避免对卡死的主机每分钟重查上游
// 并刷出重复的实例同步流水；一旦订单离开滞留名单（已推进/已作废）即清除状态。
func (p *ProvisioningPromoter) runOnce(ctx context.Context) {
	jobrun.RunErr(ctx, "provision_promote", "order", jobrun.TriggerScheduled,
		func(ctx context.Context) (int, int, map[string]any, error) {
			before := time.Now().Add(-p.staleAfter)
			items, err := p.store.ListProvisioningStale(ctx, before, p.batch)
			if err != nil {
				return 0, 0, nil, err
			}
			// 本轮仍在滞留名单里的订单；扫描结束后据此清理退避状态。
			alive := make(map[uint64]bool, len(items))
			promoted, skipped := 0, 0
			for i := range items {
				order := items[i]
				alive[order.ID] = true
				if p.backoffFor(order.ID) {
					skipped++
					continue
				}
				// 单笔独立超时：一台主机拉详情慢不能拖住整轮。
				runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
				status, rerr := p.refresher.RefreshByOrder(runCtx, order.ID)
				cancel()
				if rerr != nil || status == "" {
					wait := p.noteNoProgress(order.ID)
					p.logger.Debug("provisioning promoter: refresh skipped",
						zap.Uint64("order_id", order.ID), zap.String("status", status),
						zap.Duration("retry_in", wait), zap.Error(rerr))
					continue
				}
				if status != "running" {
					// 主机还在创建中/已停止/已被上游删除：保持原状并退避，
					// 下一轮到期再查（不报错，避免刷日志）。
					wait := p.noteNoProgress(order.ID)
					p.logger.Debug("provisioning promoter: instance not ready",
						zap.Uint64("order_id", order.ID), zap.String("status", status),
						zap.Duration("retry_in", wait))
					continue
				}
				runCtx, cancel = context.WithTimeout(ctx, 45*time.Second)
				aerr := p.activator.Activate(runCtx, order.ID)
				cancel()
				if aerr != nil {
					wait := p.noteNoProgress(order.ID)
					p.logger.Warn("provisioning promoter: order activate failed",
						zap.Uint64("order_id", order.ID), zap.Duration("retry_in", wait), zap.Error(aerr))
					continue
				}
				p.forget(order.ID)
				promoted++
			}
			p.pruneBackoff(alive)
			if promoted > 0 || skipped > 0 {
				p.logger.Info("provisioning promoter: orders advanced",
					zap.Int("scanned", len(items)), zap.Int("promoted", promoted), zap.Int("backoff", skipped))
			}
			return promoted, len(items),
				map[string]any{"scanned": len(items), "promoted": promoted, "backoff": skipped}, nil
		})
}

// pruneBackoff 清理已不在滞留名单的订单退避状态（订单已推进/已作废），
// 避免 map 随历史订单无限增长。
func (p *ProvisioningPromoter) pruneBackoff(alive map[uint64]bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id := range p.backoff {
		if !alive[id] {
			delete(p.backoff, id)
		}
	}
}
