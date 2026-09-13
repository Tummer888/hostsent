package server

// 支付中心装配（doc35）：admin 支付管理与 uc 收银台的接线，以及 finance 侧
// 「提现打款」与订单侧「渠道支付/渠道退款」的回调钩子注入。
//
// 依赖方向（doc35 §3）：
//   收款：payment 支付成功事件 → 装配层钩子 → finance/order 记账/开通；
//   付款：finance 提现审批 → 注入的 PayoutPort → payment 打款单。
// payment 不 import finance，finance 也不 import payment，耦合点只在本文件。

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	accountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	billservice "hostsent/backend/internal/modules/admin/finance/bill/service"
	rechdto "hostsent/backend/internal/modules/admin/finance/recharge/dto"
	rechargeservice "hostsent/backend/internal/modules/admin/finance/recharge/service"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	withdrawdto "hostsent/backend/internal/modules/admin/finance/withdraw/dto"
	withdrawservice "hostsent/backend/internal/modules/admin/finance/withdraw/service"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	payhandler "hostsent/backend/internal/modules/admin/payment/handler"
	paymodel "hostsent/backend/internal/modules/admin/payment/model"
	payrepo "hostsent/backend/internal/modules/admin/payment/repository"
	payservice "hostsent/backend/internal/modules/admin/payment/service"
	ucpayhandler "hostsent/backend/internal/modules/uc/payment/handler"
	ucpayservice "hostsent/backend/internal/modules/uc/payment/service"
	"hostsent/backend/internal/pkg/config"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/payment"
)

// PayoutSettleFunc 打款成功后的业务域结算回调（提现单 ID + 打款凭证）。
type PayoutSettleFunc func(ctx context.Context, withdrawID uint64, channelTx, receiptURL, remark string, operatorID uint64) error

// paymentBundle 支付中心处理器集合（handler 统一挂 App 字段）。
type paymentBundle struct {
	channelHandler  *payhandler.ChannelHandler
	orderHandler    *payhandler.OrderHandler
	callbackHandler *payhandler.CallbackHandler
	refundHandler   *payhandler.RefundHandler
	payoutHandler   *payhandler.PayoutHandler
	reconHandler    *payhandler.ReconHandler
	methodHandler   *payhandler.MethodHandler
	notifyHandler   *payhandler.NotifyHandler
	// userPaymentHandler 用户端收银台（支付方式/偏好/收款账户/充值/账单支付/提现）。
	userPaymentHandler *ucpayhandler.PaymentHandler
	// payoutService 打款能力（装配层回填给销售提现，doc86 §2.5）。
	payoutService payservice.PayoutService
}

// buildPaymentBundle 装配支付中心：仓储 → 服务 → 处理器，并完成上下游钩子接线。
func buildPaymentBundle(
	cfg *config.Config,
	db *gorm.DB,
	wallet accountservice.WalletService,
	recharge rechargeservice.RechargeService,
	bill billservice.BillService,
	orders orderservice.OrderService,
	withdraw withdrawservice.WithdrawService,
	// salesSettler 销售提成提现结算回调（biz_type=sales_withdraw 时调用，doc86 §2.5）。
	salesSettler PayoutSettleFunc,
	logger *zap.Logger,
) *paymentBundle {
	// 仓储
	channelRepo := payrepo.NewChannelRepository(db)
	typeRepo := payrepo.NewTypeRepository(db)
	orderRepo := payrepo.NewOrderRepository(db)
	callbackRepo := payrepo.NewCallbackRepository(db)
	refundRepo := payrepo.NewRefundRepository(db)
	payoutRepo := payrepo.NewPayoutRepository(db)
	prefRepo := payrepo.NewPreferenceRepository(db)
	reconRepo := payrepo.NewReconRepository(db)

	// 服务
	channelSvc := payservice.NewChannelService(channelRepo, typeRepo, prefRepo, cfg.App.EncryptKey)
	resolver := payservice.NewChannelResolver(channelRepo, typeRepo, prefRepo, cfg.App.EncryptKey)
	orderSvc := payservice.NewOrderService(orderRepo, refundRepo, callbackRepo, resolver)
	refundSvc := payservice.NewRefundService(refundRepo, orderRepo, resolver)
	payoutSvc := payservice.NewPayoutService(payoutRepo, resolver)
	prefSvc := payservice.NewPreferenceService(prefRepo, channelSvc, cfg.App.EncryptKey)
	reconSvc := payservice.NewReconService(orderRepo, reconRepo)
	callbackSvc := payservice.NewCallbackService(callbackRepo)

	// 渠道类型注册表回写（幂等）：把已注册适配器的能力描述符落到 payment_types，
	// 后台据此渲染动态凭证表单；失败只记日志，不阻断启动。
	if err := channelSvc.SyncTypeRegistry(context.Background()); err != nil {
		logger.Warn("payment: sync channel type registry failed", zap.Error(err))
	}

	// 支付成功钩子：按业务类型分别入账/开通（幂等由各业务单号保证）。
	orderSvc.SetPaidHook(func(ctx context.Context, o *paymodel.PaymentOrder) error {
		methodName := channelDisplayType(o.ChannelType)
		switch o.BizType {
		case paymodel.BizTypeRecharge:
			if _, err := recharge.ApproveByNo(ctx, o.BizNo, rechdto.RechargeApproveRequest{ChannelTx: o.ChannelTx}, 0); err != nil {
				return err
			}
		case paymodel.BizTypeBill:
			if o.BizID == 0 {
				return nil
			}
			return bill.MarkPaid(ctx, o.BizID, o.AmountFen, methodName, o.ChannelID)
		case paymodel.BizTypeOrder:
			moved, err := orders.MarkPaidByChannel(ctx, o.BizNo, methodName, o.ChannelTx)
			if err != nil {
				return err
			}
			if moved {
				// 开通投递失败不回滚支付状态：订单保持 paid，可由后台「重新开通」重试。
				if aerr := orders.ActivateByNo(ctx, o.BizNo); aerr != nil {
					logger.Warn("payment: activate order after paid failed",
						zap.String("order_no", o.BizNo), zap.Error(aerr))
				}
			}
		}
		return nil
	})

	// 提现打款能力注入 finance（approved → 打款任务；api 模式失败回落人工）。
	withdraw.SetPayoutPort(payoutSvc)

	// 反向接线：在支付中心「打款管理」登记打款完成后，同步 finance 提现单结算冻结资金。
	// 缺此回调时管理端在打款页操作只改打款单，提现单会停在 approved，冻结永不释放。
	// 打款单按业务域分发结算：财务提现回 finance，提成提现回 sales（doc86 §2.5）。
	payoutSvc.SetSettler(func(ctx context.Context, bizType string, withdrawID uint64, channelTx, receiptURL, remark string, operatorID uint64) error {
		switch bizType {
		case paymodel.PayoutBizSalesWithdraw:
			if salesSettler == nil {
				return nil
			}
			return salesSettler(ctx, withdrawID, channelTx, receiptURL, remark, operatorID)
		default:
			_, err := withdraw.MarkPaid(ctx, withdrawID, withdrawdto.WithdrawPayoutRequest{
				ChannelTx:  channelTx,
				ReceiptURL: receiptURL,
				Remark:     remark,
			}, operatorID)
			return err
		}
	})

	// 退款资金出口钩子（doc36 §3.2）：按退款去向分流。
	//   channel：原路退回支付来源，有渠道支付记录则建渠道退款单并回传单号；
	//   balance：直接回补余额（资金仍在平台内，消费口径不变）。
	// 无渠道支付记录时无论何种模式都回落余额回补，与 doc34 F-03 对应。
	orders.SetChannelRefundHook(func(ctx context.Context, orderID uint64, orderNo, orderRefundNo string, userID uint64, amount, fee float64, mode string) (string, error) {
		channelRefundNo := ""
		if mode == ordermodel.RefundModeChannel {
			info, err := refundSvc.CreateForOrder(ctx, orderRefundNo, orderNo, userID, amount)
			if err != nil {
				// 渠道不支持退款等异常：不阻断审核，也不让资金悬空 —— 回落余额回补，
				// 由订单侧把 channel_refund_status 记为 failed，账单不按原路退回扣减。
				logger.Warn("payment: channel refund failed, fallback to balance",
					zap.String("order_no", orderNo), zap.String("refund_no", orderRefundNo),
					zap.Error(err))
			} else if info != nil {
				// 已建渠道退款单：原路退回，不再回补余额，避免重复退款。
				// 扣点由退款单 fee_amount 承载，账单侧按其扣减收入口径。
				logger.Info("payment: channel refund created",
					zap.String("order_no", orderNo), zap.String("refund_no", orderRefundNo),
					zap.Float64("amount", amount), zap.Float64("fee", fee))
				return info.RefundNo, nil
			}
			// info == nil：该订单无渠道支付记录（余额支付），同样回落余额回补。
		}
		if _, werr := wallet.Change(ctx, accountservice.ChangeRequest{
			UserID:    userID,
			Type:      transmodel.TxTypeRefund,
			Direction: transmodel.DirectionIncome,
			Amount:    amount,
			RefNo:     orderRefundNo,
			BizType:   "order_refund",
			Remark:    "订单退款回补余额",
		}); werr != nil {
			logger.Warn("finance: refund balance credit failed",
				zap.String("order_no", orderNo), zap.String("refund_no", orderRefundNo), zap.Error(werr))
		}
		return channelRefundNo, nil
	})

	// 用户端收银台编排（建单 → 支付 → 回绑）。
	cashier := ucpayservice.NewCashierService(recharge, orderSvc, prefSvc, withdraw, bill)

	userPaymentHandler := ucpayhandler.NewPaymentHandler(prefSvc, prefSvc, prefSvc, orderSvc, cashier,
		func(err error) *apperrors.AppError {
			return mapPaymentUserError(err)
		})

	return &paymentBundle{
		channelHandler:     payhandler.NewChannelHandler(channelSvc),
		orderHandler:       payhandler.NewOrderHandler(orderSvc),
		callbackHandler:    payhandler.NewCallbackHandler(callbackSvc),
		refundHandler:      payhandler.NewRefundHandler(refundSvc),
		payoutHandler:      payhandler.NewPayoutHandler(payoutSvc),
		reconHandler:       payhandler.NewReconHandler(reconSvc),
		methodHandler:      payhandler.NewMethodHandler(prefSvc, channelSvc),
		notifyHandler:      payhandler.NewNotifyHandler(orderSvc),
		userPaymentHandler: userPaymentHandler,
		payoutService:      payoutSvc,
	}
}

// channelDisplayType 把渠道类型映射为业务侧支付方式（order.pay_method / bill.paid_method）。
func channelDisplayType(channelType string) string {
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case "alipay":
		return "alipay"
	case "wechatpay", "wechat", "wxpay":
		return "wechat"
	case "unionpay":
		return "unionpay"
	case "bestpay":
		return "bestpay"
	case "manual", "":
		return "manual"
	default:
		return "online"
	}
}

// mapPaymentUserError 用户端支付域错误 → 统一错误码（与 uc/finance 同口径）。
func mapPaymentUserError(err error) *apperrors.AppError {
	switch {
	case err == nil:
		return apperrors.New(50001, "unknown error")
	case errors.Is(err, accountservice.ErrWalletNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, accountservice.ErrInsufficientBalance):
		return apperrors.New(30001, err.Error())
	case errors.Is(err, accountservice.ErrFrozenInsufficient):
		return apperrors.New(30002, err.Error())
	case errors.Is(err, accountservice.ErrStatusConflict):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, ucpayservice.ErrBillNotPayable):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, payservice.ErrAccountNotFound), errors.Is(err, payservice.ErrChannelNotFound),
		errors.Is(err, payservice.ErrOrderNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, payservice.ErrNoChannelAvailable):
		return apperrors.New(30007, err.Error())
	case errors.Is(err, payment.ErrAmountMismatch), errors.Is(err, payment.ErrSignatureInvalid):
		return apperrors.New(30009, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
