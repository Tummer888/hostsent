// 确认收款（场外收款人工入账）的装配层适配器：
// 订单域声明的两个最小端口（支付台账登记 / 到账提醒）在这里落到支付中心与通知模块。
//
// CONTRIBUTING §3：订单模块不 import 支付中心与通知模块的 dto/model，
// 出入参的转换职责落在装配层（同 uc_order_adapters.go 的做法）。

package server

import (
	"context"
	"errors"
	"strconv"
	"strings"

	accountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	notifydto "hostsent/backend/internal/modules/admin/system/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/system/notification/model"
	notifyservice "hostsent/backend/internal/modules/admin/system/notification/service"
	paydto "hostsent/backend/internal/modules/admin/system/payment/dto"
	payservice "hostsent/backend/internal/modules/admin/system/payment/service"
	"hostsent/backend/internal/pkg/payment"
)

// offlinePaymentRecorderAdapter 把支付中心的「人工入账」能力适配为订单域的台账登记端口。
type offlinePaymentRecorderAdapter struct {
	svc payservice.OrderService
}

// Record 补登一张 origin=offline 的已收款台账。
//
// bizNo 用订单号而非订单 ID：支付中心侧的检索（订单列表按支付单号/渠道流水号反查订单）
// 与退款原路退回定位，走的都是 biz_no 这条链路。
func (a offlinePaymentRecorderAdapter) Record(ctx context.Context, order *ordermodel.Order, req orderservice.OfflinePaymentRecord) (*orderservice.OfflinePaymentRecordResult, error) {
	if order == nil {
		return nil, orderservice.ErrOrderNotFound
	}
	info, err := a.svc.RecordOfflinePayment(ctx, paydto.OfflinePaymentRequest{
		PayMethod:    req.PayMethod,
		ChannelCode:  req.ChannelCode,
		EvidenceNo:   req.EvidenceNo,
		Remark:       req.Remark,
		OperatorName: req.OperatorName,
	}, order.UserID, paydto.BizTypeOrder, order.ID, order.OrderNo, req.PaidAmount, req.OperatorID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, orderservice.ErrOfflinePaymentUnavailable
	}
	// Created 表达「这次是新登记的，还是命中了既有台账」。
	// 台账一建就是 paid 状态，故以到账时间是否为空区分：人工入账单必然有 paid_at。
	return &orderservice.OfflinePaymentRecordResult{
		PaymentNo: info.PaymentNo,
		Created:   info.PaidAt != "",
	}, nil
}

// orderPaymentNotifierAdapter 把通知中心适配为订单域的到账提醒端口。
//
// 只转发「订单收款成功」这一个事件（模板 event=order_paid）：模板选择、投递排期
// 都留在通知模块，订单域只负责"通知谁、用哪些渠道"。
type orderPaymentNotifierAdapter struct {
	svc notifyservice.NotificationService
}

func (a orderPaymentNotifierAdapter) NotifyOrderPaid(ctx context.Context, order *ordermodel.Order, channels []string) ([]string, map[string]string) {
	skipped := map[string]string{}
	if order == nil || len(channels) == 0 {
		return nil, skipped
	}
	// 渠道归一 + 去重：前端可能传大写或重复项，通知模块按标准值（inbox/mail/sms）匹配。
	want := make([]string, 0, len(channels))
	seen := map[string]bool{}
	for _, raw := range channels {
		c := normalizeNotifyChannel(raw)
		if c == "" {
			skipped[strings.TrimSpace(raw)] = "不支持的提醒渠道"
			continue
		}
		if seen[c] {
			continue
		}
		seen[c] = true
		want = append(want, c)
	}
	if len(want) == 0 {
		return nil, skipped
	}
	err := a.svc.Publish(ctx, notifydto.PublishInput{
		Event:  notifymodel.EventOrderPaid,
		UserID: order.UserID,
		Target: notifymodel.TargetUser,
		Vars: map[string]string{
			"order_no": order.OrderNo,
			"amount":   formatMoney(order.PaidAmount),
		},
		SourceModule: "order",
		// 幂等键带渠道维度：同一订单的邮件与短信由投递队列的唯一索引
		// (source_module, source_id, channel, target) 各自去重，不会互相顶掉。
		SourceID:      order.OrderNo,
		ForceChannels: want,
	})
	if err != nil {
		// 提醒是附加动作：发送失败不回滚收款确认，但逐渠道记为跳过 ——
		// 让运营知道"这条提醒没发出去"，而不是以为客户已经收到。
		for _, c := range want {
			skipped[c] = "发送失败：" + err.Error()
		}
		return nil, skipped
	}
	return want, skipped
}

// normalizeNotifyChannel 提醒渠道归一：只认 inbox/mail/sms，其余返回空串。
// email 是前端习惯叫法，映射到通知模块的 mail。
func normalizeNotifyChannel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case notifymodel.ChannelInbox:
		return notifymodel.ChannelInbox
	case notifymodel.ChannelMail, "email":
		return notifymodel.ChannelMail
	case notifymodel.ChannelSMS:
		return notifymodel.ChannelSMS
	default:
		return ""
	}
}

// balanceDeductorAdapter 把财务账务核心适配为订单域的余额扣款端口。
//
// 确认收款选「余额支付」时用这条真实扣款路径：余额支付的钱本就在平台内，
// 确认收款必然意味着这笔钱从客户余额划走。类型取 consume（而非后台代客下单的 order），
// 使账单与累计消费在同一条链路上自动计数（ConsumeTypes 两者都认）。
type balanceDeductorAdapter struct {
	wallet accountservice.WalletService
}

func (a balanceDeductorAdapter) DeductOrderPayment(ctx context.Context, order *ordermodel.Order, amount float64, operatorID uint64) error {
	if a.wallet == nil {
		return orderservice.ErrBalanceDeductUnavailable
	}
	_, err := a.wallet.Change(ctx, accountservice.ChangeRequest{
		UserID:    order.UserID,
		Type:      transmodel.TxTypeConsume,
		Direction: transmodel.DirectionExpense,
		Amount:    amount,
		OrderID:   order.ID,
		OrderNo:   order.OrderNo,
		// 幂等键：同一订单重复确认不会二次扣款。
		// 命名空间与下单/续费分开，避免与同单号的其他流水撞车被当成重复丢弃。
		RefNo:      order.OrderNo,
		BizType:    bizTypeConfirmPayment,
		Remark:     "后台确认收款（余额支付）：" + order.ProductName,
		OperatorID: operatorID,
	})
	if err != nil {
		if errors.Is(err, accountservice.ErrInsufficientBalance) {
			return orderservice.ErrInsufficientBalance
		}
		return err
	}
	return nil
}

// bizTypeConfirmPayment 后台确认收款的资金流水业务命名空间。
// 与下单（cashier）、续费（renewal）分开：同名不同业务会被幂等键互相吞掉。
const bizTypeConfirmPayment = "order_confirm"

// defaultPayMethodAdapter 把支付中心的用户偏好适配为 uc 订单的默认支付方式解析。
//
// 口径：用户中心设置的默认支付渠道 > 平台默认渠道 > 空串（未配任何渠道）。
// 只返回渠道编码，不返回渠道名 —— 订单 pay_method 存的是可筛选的口径值。
type defaultPayMethodAdapter struct {
	pref payservice.PreferenceService
}

func (a defaultPayMethodAdapter) DefaultPayMethod(ctx context.Context, userID uint64, scene string) string {
	if a.pref == nil || userID == 0 {
		return ""
	}
	if scene == "" {
		scene = payment.SceneNative
	}
	// MethodOptions 已经实现了「用户偏好置顶 + 平台默认兜底」的排序，
	// 直接取它的 Default：这条口径与收银台展示的默认项完全一致，
	// 不会出现「结算页显示支付宝、订单里写着微信」这种两套口径。
	opts, err := a.pref.MethodOptions(ctx, userID, scene, 0)
	if err != nil || opts == nil {
		return ""
	}
	return strings.TrimSpace(opts.Default)
}

// resolveAdminOrderPayMethod 后台代下单（仅创建不支付）的支付方式解析。
//
// 显式传了就归一后用（拒绝把 channel/online 这类"支付模式"当收款方式落库）；
// 没传则按用户默认支付方式解析，都没有则留空 —— 不编造支付方式，
// 否则按 pay_method 筛选与退款去向判定都会被污染。
func (a *asm) resolveAdminOrderPayMethod(ctx context.Context, userID uint64, raw string) string {
	if m := strings.TrimSpace(raw); m != "" {
		if normalized, ok := orderservice.NormalizePayMethod(m); ok {
			return normalized
		}
		return ""
	}
	return defaultPayMethodAdapter{pref: a.paymentBundle.preferenceService}.DefaultPayMethod(ctx, userID, "")
}

// formatMoney 金额两位小数（与通知模板 ¥{amount} 的展示口径一致）。
func formatMoney(v float64) string {
	s := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(v, 'f', 2, 64), "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}
