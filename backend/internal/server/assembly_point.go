package server

// 积分体系装配（doc36）：积分服务构建、支付成功钩子接线与错误映射。
//
// 依赖方向（doc36 §2.1）：积分账本与资金账本完全独立。
//   admin/point 不 import finance/order/payment；
//   uc/point 只依赖装配层注入的最小接口；
//   唯一耦合点在本文件 —— 业务完成事件里追加一次 Earn 调用。
//
// 积分绝不可作为支付方式：本文件不向 payment/收银台注入任何积分能力。

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	pointhandler "hostsent/backend/internal/modules/admin/point/handler"
	pointrepo "hostsent/backend/internal/modules/admin/point/repository"
	pointservice "hostsent/backend/internal/modules/admin/point/service"
	ucpointhandler "hostsent/backend/internal/modules/uc/point/handler"
	apperrors "hostsent/backend/internal/pkg/errors"
)

// PointEarner 积分发放入口（装配层以闭包形式注入到各业务完成事件）。
// 只传基础类型，业务模块无需 import 积分包。
type PointEarner func(ctx context.Context, userID uint64, bizType, bizNo string, amount float64, isRenewal bool)

// pointBundle 积分体系处理器集合（admin + uc）。
type pointBundle struct {
	adminHandler *pointhandler.PointHandler
	userHandler  *ucpointhandler.PointHandler
}

// buildPointBundle 装配积分体系：仓储 → 服务 → 处理器，并返回发放入口闭包。
func buildPointBundle(db *gorm.DB, logger *zap.Logger) (*pointBundle, PointEarner) {
	repo := pointrepo.NewPointRepository(db)
	svc := pointservice.NewPointService(db, repo)
	bundle := &pointBundle{
		adminHandler: pointhandler.NewPointHandler(svc),
		userHandler:  ucpointhandler.NewPointHandler(svc, mapPointUserError),
	}
	return bundle, newPointEarner(svc, logger)
}

// newPointEarner 构造积分发放入口。
//
// 容错策略与返现/退款钩子一致：发放失败只记日志，绝不回滚支付、开通或续费。
// 幂等由 (user_id, biz_type, ref_no) 唯一键保证，重复事件不会重发。
func newPointEarner(svc pointservice.PointService, logger *zap.Logger) PointEarner {
	return func(ctx context.Context, userID uint64, bizType, bizNo string, amount float64, isRenewal bool) {
		if svc == nil || userID == 0 {
			return
		}
		scene := pointScene(bizType, isRenewal)
		if scene == "" {
			return
		}
		if err := svc.Earn(ctx, pointservice.EarnInput{
			UserID:  userID,
			BizType: bizType,
			RefNo:   bizNo,
			Scene:   scene,
			Amount:  amount,
		}); err != nil && logger != nil {
			logger.Warn("point: earn failed",
				zap.Uint64("user_id", userID), zap.String("biz_type", bizType),
				zap.String("biz_no", bizNo), zap.Float64("amount", amount), zap.Error(err))
		}
	}
}

// pointScene 业务类型 + 是否续费 → 积分场景；不支持的组合返回空串（不发放）。
func pointScene(bizType string, isRenewal bool) string {
	switch strings.TrimSpace(bizType) {
	case "order":
		if isRenewal {
			return pointservice.SceneOrderRenewal
		}
		return pointservice.SceneOrderPurchase
	case "bill":
		return pointservice.SceneBillPayment
	default:
		// 充值属资金搬运而非消费，默认不发放积分（规则表可后续扩展）。
		return ""
	}
}

// mapPointUserError 用户端积分域错误 → 统一错误码。
func mapPointUserError(err error) *apperrors.AppError {
	switch {
	case err == nil:
		return apperrors.New(50001, "unknown error")
	case errors.Is(err, pointservice.ErrInvalidPoints), errors.Is(err, pointservice.ErrInvalidRule):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, pointservice.ErrInsufficientPoints):
		return apperrors.New(30001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
