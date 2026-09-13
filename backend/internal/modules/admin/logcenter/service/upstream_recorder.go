package service

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
	"hostsent/backend/internal/pkg/upstream"
)

// UpstreamRecorder 上游调用日志的异步批量写入器（doc92 §3.1）。
//
// 为什么必须异步：Record 是在上游请求主链路上被同步调用的。若在这里直接写库，
// 一次慢查询就会拖慢业务请求，把日志系统的问题变成业务故障。所以入口只做
// 「投进带缓冲的 channel」，缓冲满就丢弃并计数 —— 丢日志是可接受的，
// 阻塞业务不可接受。
type UpstreamRecorder struct {
	repo     logrepo.UpstreamLogRepository
	logger   *zap.Logger
	ch       chan upstream.Record
	dropped  atomic.Int64
	written  atomic.Int64
	flushMax int
	flushDur time.Duration
}

// UpstreamRecorderOptions 采集写入参数。
type UpstreamRecorderOptions struct {
	Buffer    int           // channel 缓冲条数
	Batch     int           // 攒批条数
	FlushTime time.Duration // 最长等待时间
}

// DefaultUpstreamRecorderOptions 默认参数。
func DefaultUpstreamRecorderOptions() UpstreamRecorderOptions {
	return UpstreamRecorderOptions{Buffer: 2048, Batch: 200, FlushTime: 2 * time.Second}
}

// NewUpstreamRecorder 创建采集写入器。
func NewUpstreamRecorder(repo logrepo.UpstreamLogRepository, logger *zap.Logger, opts UpstreamRecorderOptions) *UpstreamRecorder {
	if opts.Buffer <= 0 {
		opts.Buffer = 2048
	}
	if opts.Batch <= 0 {
		opts.Batch = 200
	}
	if opts.FlushTime <= 0 {
		opts.FlushTime = 2 * time.Second
	}
	return &UpstreamRecorder{
		repo:     repo,
		logger:   logger,
		ch:       make(chan upstream.Record, opts.Buffer),
		flushMax: opts.Batch,
		flushDur: opts.FlushTime,
	}
}

// Record 实现 upstream.Recorder：非阻塞投递。
func (r *UpstreamRecorder) Record(rec upstream.Record) {
	if r == nil {
		return
	}
	select {
	case r.ch <- rec:
	default:
		// 缓冲满：丢弃并计数，绝不阻塞上游调用（doc92 §3.1）。
		if n := r.dropped.Add(1); n%100 == 1 {
			r.logger.Warn("upstream log dropped: buffer full", zap.Int64("dropped_total", n))
		}
	}
}

// Start 启动批量 flush goroutine（由 Server.Run 的 ctx 控制生命周期）。
func (r *UpstreamRecorder) Start(ctx context.Context) {
	if r == nil || r.repo == nil {
		return
	}
	ticker := time.NewTicker(r.flushDur)
	defer ticker.Stop()
	buf := make([]logmodel.UpstreamAPILog, 0, r.flushMax)
	for {
		select {
		case <-ctx.Done():
			// 退出前尽力落盘一次，避免进程重启丢掉已采集的排障线索。
			r.flush(context.Background(), buf)
			return
		case rec := <-r.ch:
			buf = append(buf, toModel(rec))
			if len(buf) >= r.flushMax {
				r.flush(ctx, buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) == 0 {
				continue
			}
			r.flush(ctx, buf)
			buf = buf[:0]
		}
	}
}

// flush 落库一批；失败只打日志（最多重试一次），不重试到卡死。
func (r *UpstreamRecorder) flush(ctx context.Context, rows []logmodel.UpstreamAPILog) {
	if len(rows) == 0 {
		return
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := r.repo.CreateBatch(writeCtx, rows); err != nil {
		// 重试一次：多数失败是瞬时连接问题。
		if retryErr := r.repo.CreateBatch(writeCtx, rows); retryErr != nil {
			r.logger.Warn("upstream log batch write failed",
				zap.Int("rows", len(rows)), zap.Error(retryErr))
			return
		}
	}
	r.written.Add(int64(len(rows)))
}

// Stats 采集侧计数（暴露到 /logs/stats，让运营能看见日志系统自己丢了多少）。
func (r *UpstreamRecorder) Stats() (written, dropped int64) {
	if r == nil {
		return 0, 0
	}
	return r.written.Load(), r.dropped.Load()
}

// toModel 采集记录 → 落库模型。
func toModel(rec upstream.Record) logmodel.UpstreamAPILog {
	created := rec.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	return logmodel.UpstreamAPILog{
		ProviderID:        uint64(rec.ProviderID),
		ProviderName:      rec.ProviderName,
		ProviderType:      rec.ProviderType,
		Op:                truncate(rec.Op, 64),
		Method:            truncate(rec.Method, 8),
		URL:               truncate(rec.URL, 500),
		RequestDigest:     rec.RequestDigest,
		RequestBody:       rec.RequestBody,
		RequestBytes:      rec.RequestBytes,
		RequestTruncated:  rec.RequestTruncated,
		StatusCode:        rec.StatusCode,
		ResponseDigest:    rec.ResponseDigest,
		ResponseBody:      rec.ResponseBody,
		ResponseBytes:     rec.ResponseBytes,
		ResponseTruncated: rec.ResponseTruncated,
		Success:           rec.Success,
		ErrorCode:         truncate(rec.ErrorCode, 64),
		ErrorMessage:      truncate(rec.ErrorMessage, 500),
		DurationMS:        rec.DurationMS,
		RetryIndex:        rec.RetryIndex,
		TraceID:           truncate(rec.TraceID, 64),
		CreatedAt:         created,
	}
}
