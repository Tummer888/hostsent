// Package publishsched 提供内容定时发布的调度器（公告与内容文章共用）。
//
// 为什么需要它：公告与内容文章都支持「定时发布时间」（draft + 未来 publish_at），
// 但两者的保存逻辑都只在写入时判断一次「publish_at 是否已到」，到点之后没有任何
// 东西把它们翻成 published。结果是运营约好 10:00 的维护公告，10:00 什么都不会发生
// —— 界面上写着「定时发布」，实际是「定时存草稿」，比一开始不提供这个选项更伤可信度。
//
// 放在 pkg 而不是某个业务模块下，是为了不让它把管理端 service 拖进来：调度器只
// 依赖「推进到点内容」这一件事的两个最小接口，由装配层把仓储塞进来。
package publishsched

import (
	"context"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/pkg/jobrun"
	"hostsent/backend/internal/pkg/revalidate"
)

// AnnouncementPublisher 把到点的公告从 draft 推进为 published。
type AnnouncementPublisher interface {
	PublishDue(ctx context.Context, now time.Time) (int64, error)
}

// ArticlePublisher 把到点的内容文章从 draft 推进为 published。
type ArticlePublisher interface {
	PublishDue(ctx context.Context, now time.Time) (int64, error)
}

// Scheduler 定时发布调度器。
//
// 扫描周期取 1 分钟：定时发布的最小粒度就是分钟（管理端时间控件精确到秒，但运营
// 实际按分钟约），一分钟一轮意味着最长滞后 60 秒，对「约好整点发公告」的体感足够，
// 又不会像 10 秒一轮那样在 job_run_logs 里堆无意义的心跳行。
type Scheduler struct {
	announcements AnnouncementPublisher
	articles      ArticlePublisher
	logger        *zap.Logger
	interval      time.Duration
}

// NewScheduler 创建调度器；任一路为 nil 时跳过对应的推进。
func NewScheduler(announcements AnnouncementPublisher, articles ArticlePublisher, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		announcements: announcements,
		articles:      articles,
		logger:        logger,
		interval:      time.Minute,
	}
}

// Start 启动调度；ctx 取消时退出。
//
// 启动时先跑一轮：进程停机期间到点的内容不必等下一个 tick 才可见。
func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || (s.announcements == nil && s.articles == nil) {
		return
	}
	s.runOnce(ctx)
	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("publish scheduler stopped")
				return
			case <-t.C:
				s.runOnce(ctx)
			}
		}
	}()
	s.logger.Info("publish scheduler started", zap.Duration("interval", s.interval))
}

// runOnce 单轮推进。失败只记录不中断：调度器停摆比一次推进失败严重得多。
func (s *Scheduler) runOnce(ctx context.Context) {
	now := time.Now()
	jobrun.RunErr(ctx, "content_scheduled_publish", "content", jobrun.TriggerScheduled,
		func(ctx context.Context) (int, int, map[string]any, error) {
			announcements, articles := 0, 0
			if s.announcements != nil {
				n, err := s.announcements.PublishDue(ctx, now)
				if err != nil {
					s.logger.Error("scheduled publish: announcements failed", zap.Error(err))
					return announcements + articles, 0, nil, err
				}
				announcements = int(n)
			}
			if s.articles != nil {
				n, err := s.articles.PublishDue(ctx, now)
				if err != nil {
					s.logger.Error("scheduled publish: articles failed", zap.Error(err))
					return announcements + articles, 0, nil, err
				}
				articles = int(n)
			}
			affected := announcements + articles
			if affected > 0 {
				s.logger.Info("scheduled content published",
					zap.Int("announcements", announcements), zap.Int("articles", articles))
				// 到点发布的内容同样要立刻在门户可见，否则「定时发布」只兑现了一半：
				// 库里状态对了，前台还要再等一个 SWR TTL。
				keys := make([]string, 0, 2)
				if announcements > 0 {
					keys = append(keys, revalidate.KeyAnnouncement)
				}
				if articles > 0 {
					keys = append(keys, revalidate.KeyArticle)
				}
				revalidate.Notify(ctx, keys...)
			}
			return affected, affected, map[string]any{
				"announcements": announcements,
				"articles":      articles,
			}, nil
		})
}
