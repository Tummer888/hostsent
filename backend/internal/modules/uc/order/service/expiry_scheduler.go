package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// PendingExpireScheduler 待支付订单过期关单调度器（doc88 §6.2）。
//
// 为什么需要独立调度：渠道支付路径下订单会长时间停在 pending（用户点了「去支付」却没付完），
// 这些单会一直占用 SKU 库存、并让订单列表里堆着永远付不掉的单。调度器按固定周期把
// 超期未付的订单置为 closed 并释放库存。
//
// 有效期取系统配置 order_expire_minutes（分组 order），每轮扫描前重读：
// 运维改了配置无需重启即可生效。
type PendingExpireScheduler struct {
	svc      OrderService
	readConf func(ctx context.Context) int
	logger   *zap.Logger
	interval time.Duration
}

// NewPendingExpireScheduler 创建调度器，默认 10 分钟扫描一轮。
func NewPendingExpireScheduler(svc OrderService, readConf func(ctx context.Context) int, logger *zap.Logger) *PendingExpireScheduler {
	if readConf == nil {
		readConf = func(context.Context) int { return 0 }
	}
	return &PendingExpireScheduler{
		svc:      svc,
		readConf: readConf,
		logger:   logger,
		interval: 10 * time.Minute,
	}
}

// Start 启动调度器 goroutine；ctx 取消时退出。
//
// 启动时先跑一轮：进程重启期间到期的订单不必等下一个 tick 才被关掉，
// 同时把配置里的有效期灌进服务（读接口的支付截止时间也依赖它）。
func (s *PendingExpireScheduler) Start(ctx context.Context) {
	s.runOnce(ctx)
	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("pending order expire scheduler stopped")
				return
			case <-t.C:
				s.runOnce(ctx)
			}
		}
	}()
	s.logger.Info("pending order expire scheduler started", zap.Duration("interval", s.interval))
}

// runOnce 单轮扫描：读配置 → 关单；失败只记录，不中断调度。
func (s *PendingExpireScheduler) runOnce(ctx context.Context) {
	minutes := s.readConf(ctx)
	if minutes <= 0 {
		minutes = defaultPayExpireMinutes
	}
	// 读接口展示的支付截止时间与关单口径共用同一份配置，这里一并刷新。
	s.svc.SetPayExpireMinutes(minutes)
	closed, err := s.svc.ExpirePending(ctx, minutes)
	if err != nil {
		s.logger.Error("pending order expire scan failed", zap.Error(err))
		return
	}
	if closed > 0 {
		s.logger.Info("pending orders closed by expiry",
			zap.Int("count", closed), zap.Int("expire_minutes", minutes))
	}
}
