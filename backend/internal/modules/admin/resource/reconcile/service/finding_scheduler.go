package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/pkg/safego"
)

// FindingScheduler 对账差异扫描的周期调度器（doc120）。
//
// 为什么要独立调度而不是挂进 sync_schedules：sync_schedules 是「渠道 × scope」
// 的同步任务（去上游拉数），而对账是**纯本地比对**（instances × products ×
// resource_products，不出网）。把它建模成"需要配定时同步的渠道任务"会让运营
// 以为要按渠道逐个开启；这里固定节奏跑即可。
//
// 节奏：每 tickEvery 检查一次，距上次扫描超过 interval 就跑一轮。
// 进程重启不重复执行（内存态 lastRun），单轮 panic 只记日志不停摆。
type FindingScheduler struct {
	scanner   FindingScanner
	logger    *zap.Logger
	tickEvery time.Duration
	interval  time.Duration
	lastRun   time.Time
}

// NewFindingScheduler 创建差异扫描调度器。
func NewFindingScheduler(scanner FindingScanner, logger *zap.Logger) *FindingScheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FindingScheduler{
		scanner:   scanner,
		logger:    logger,
		tickEvery: time.Minute,
		interval:  10 * time.Minute,
	}
}

// Start 启动调度循环（阻塞在独立 goroutine，随 ctx 结束退出）。
func (s *FindingScheduler) Start(ctx context.Context) {
	if s.scanner == nil {
		return
	}
	go func() {
		defer safego.Recover(s.logger)
		// 启动后先等一个 tick：与启动期其它初始化错峰，也避免每次重启都立刻扫一遍。
		timer := time.NewTimer(s.tickEvery)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.runSafely(ctx)
				timer.Reset(s.tickEvery)
			}
		}
	}()
}

func (s *FindingScheduler) runSafely(ctx context.Context) {
	defer safego.Recover(s.logger)
	if !s.lastRun.IsZero() && time.Since(s.lastRun) < s.interval {
		return
	}
	s.lastRun = time.Now()
	result, err := s.scanner.Scan(ctx)
	if err != nil {
		s.logger.Warn("对账差异扫描失败", zap.Error(err))
		return
	}
	if result.Opened > 0 || result.Resolved > 0 {
		s.logger.Info("对账差异扫描完成",
			zap.Int("scanned", result.Scanned), zap.Int("findings", result.Findings),
			zap.Int("opened", result.Opened), zap.Int64("resolved", result.Resolved))
	}
}
