package service

import (
	"context"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/admin/order/repository"
)

// ProvisionEnqueuer 开通履约任务投递器（T5.1）。
//
// 下单/后台代下单只落库并投递任务，实际开通由 ProvisionWorker 异步执行；
// 用户中心订单模块通过最小接口依赖本类型，避免跨包依赖具体仓储。
type ProvisionEnqueuer struct {
	repo   repository.ProvisionTaskRepository
	logger *zap.Logger
}

// NewProvisionEnqueuer 创建开通任务投递器。
func NewProvisionEnqueuer(repo repository.ProvisionTaskRepository, logger *zap.Logger) *ProvisionEnqueuer {
	return &ProvisionEnqueuer{repo: repo, logger: logger}
}

// EnqueueProvision 幂等投递某订单的开通任务（order_id 唯一）：
//   - 首次投递 → pending，工作池领取后执行；
//   - 已存在且上次失败 → 重置为 pending 立即重试（人工队列/已成功不被打回）。
//
// sourceMode 为商品链路（self/upstream，D6 单一判据），仅用于任务留痕与分派排查；
// 调用方（订单域）无法读到商品时传空串，不影响投递与履约。
func (e *ProvisionEnqueuer) EnqueueProvision(ctx context.Context, order *model.Order, sourceMode string) error {
	if order == nil {
		return nil
	}
	task := &model.ProvisionTask{
		OrderID:     order.ID,
		UserID:      order.UserID,
		ProductID:   order.ProductID,
		SourceMode:  sourceMode,
		Status:      model.ProvisionTaskPending,
		MaxAttempts: model.DefaultProvisionMaxAttempts,
	}
	if err := e.repo.Enqueue(ctx, task); err != nil {
		e.logger.Error("enqueue provision task failed",
			zap.Uint64("order_id", order.ID), zap.Uint64("user_id", order.UserID), zap.Error(err))
		return err
	}
	e.logger.Info("provision task enqueued",
		zap.Uint64("order_id", order.ID), zap.String("source_mode", sourceMode))
	return nil
}
