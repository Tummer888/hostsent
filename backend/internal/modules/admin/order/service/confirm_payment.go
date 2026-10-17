package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
)

// payMethodAliases 支付方式别名归一：渠道类型名 → 订单侧方法名。
//
// 与装配层 channelDisplayType 同口径（微信在各渠道里写作 wechatpay/wxpay）。
// 后台手填与渠道落库因此指向同一个方法名，按 pay_method 筛选才不会漏。
var payMethodAliases = map[string]string{
	"wechatpay": "wechat",
	"wxpay":     "wechat",
	"weixin":    "wechat",
	"zhifubao":  "alipay",
	"offline":   model.PayMethodManual,
	"manual":    model.PayMethodManual,
}

// reservedPayMethods 非支付方式的历史桶名，禁止落进 orders.pay_method。
// "channel" 是下单请求里的支付模式（余额/线上），"online" 是渠道类型的兜底桶，
// 两者都不是"客户用什么付的钱"，落库后无法用于退款去向判定。
var reservedPayMethods = map[string]bool{"channel": true, "online": true, "": true}

// NormalizePayMethod 归一支付方式；第二返回值表示是否合法。
//
// 合法 = 非保留桶名 + 长度 ≤20 + 仅小写字母/数字/下划线/连字符。
// 不设固定枚举：渠道类型可由支付中心插件式注册（payment_types），
// 写死白名单会让新接入的渠道无法在后台登记收款。
func NormalizePayMethod(raw string) (string, bool) {
	m := strings.ToLower(strings.TrimSpace(raw))
	if alias, ok := payMethodAliases[m]; ok {
		m = alias
	}
	if reservedPayMethods[m] {
		return "", false
	}
	if len(m) > 20 {
		return "", false
	}
	for _, r := range m {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return "", false
		}
	}
	return m, true
}

// ConfirmPayment 后台确认收款（场外收款人工入账）。
//
// 链路（顺序即正确性）：
//  1. 仅待支付订单可确认（CAS 置为已支付，与关单/取消并发时只有一方生效）；
//  2. 余额支付 → 真实扣款（与后台代客下单同口径，账务核心自动计入累计消费与账单）；
//     其他方式 → 由支付中心补登一张 origin=offline 的已收款台账；
//  3. 第 2 步失败则把订单退回待支付：宁可让运营重试，也不能出现
//     「订单说收了钱、台账/余额里没有」这种账实不符；
//  4. 支付成功钩子（续费联动 / 积分 / 累计消费）+ 开通投递 + 可选的到账提醒。
//
// 为什么要 CAS 而不是直接 Save：关单调度器与用户取消都会改同一行状态，
// 先读后写会把「已被超时关掉的订单」复活（读到的 pending 已经不是当前状态）。
func (s *orderService) ConfirmPayment(
	ctx context.Context,
	id uint64,
	req dto.ConfirmPaymentRequest,
	operatorID uint64,
	operatorName string,
) (*dto.ConfirmPaymentResult, error) {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	if item.Status != model.OrderStatusPending {
		return nil, ErrStatusConflict
	}
	payMethod, ok := NormalizePayMethod(req.PayMethod)
	if !ok {
		return nil, ErrPayMethodInvalid
	}
	paidAmount := req.PaidAmount
	if paidAmount <= 0 {
		paidAmount = payableAmount(item)
	}
	if paidAmount <= 0 {
		return nil, ErrConfirmAmountInvalid
	}
	payTime := time.Now()
	if raw := strings.TrimSpace(req.PayTime); raw != "" {
		parsed, perr := parsePayTime(raw)
		if perr != nil {
			return nil, ErrPayTimeInvalid
		}
		payTime = parsed
	}
	remark := confirmRemark(item.Remark, req, operatorName, payMethod, paidAmount)

	moved, err := s.orderRepo.ConfirmPayment(ctx, id, payMethod, paidAmount, payTime, operatorID, remark)
	if err != nil {
		return nil, err
	}
	if moved == 0 {
		// CAS 落空：并发下订单被取消/超时关单，或已被另一次确认收款抢先。
		return nil, ErrStatusConflict
	}

	confirmed := *item
	confirmed.Status = model.OrderStatusPaid
	confirmed.PayMethod = payMethod
	confirmed.PaidAmount = paidAmount
	confirmed.PayTime = &payTime
	confirmed.OperatorID = operatorID
	confirmed.Remark = remark

	result := &dto.ConfirmPaymentResult{}

	// 资金侧：余额支付真实扣款，其余方式补登人工台账。
	// 两者都必须成功 —— 收款确认的核心价值就是「钱有账可查」。
	if payMethod == model.PayMethodBalance {
		if s.balanceDeductor == nil {
			s.revertConfirm(ctx, item.ID, "余额扣款能力未配置")
			return nil, ErrBalanceDeductUnavailable
		}
		if derr := s.balanceDeductor.DeductOrderPayment(ctx, &confirmed, paidAmount, operatorID); derr != nil {
			s.revertConfirm(ctx, item.ID, "余额扣款失败")
			if errors.Is(derr, ErrInsufficientBalance) {
				return nil, ErrInsufficientBalance
			}
			return nil, derr
		}
	} else {
		if s.offlinePayments == nil {
			s.revertConfirm(ctx, item.ID, "支付台账登记能力未配置")
			return nil, ErrOfflinePaymentUnavailable
		}
		rec, rerr := s.offlinePayments.Record(ctx, &confirmed, OfflinePaymentRecord{
			PayMethod:    payMethod,
			PaidAmount:   paidAmount,
			PayTime:      payTime,
			EvidenceNo:   strings.TrimSpace(req.EvidenceNo),
			ChannelCode:  strings.TrimSpace(req.ChannelCode),
			Remark:       strings.TrimSpace(req.Remark),
			OperatorID:   operatorID,
			OperatorName: operatorName,
		})
		if rerr != nil {
			s.revertConfirm(ctx, item.ID, "支付台账登记失败")
			return nil, rerr
		}
		if rec != nil {
			result.PaymentNo = rec.PaymentNo
		}
	}

	// 支付成功扩展点：与渠道支付到账同一条路径（续费联动 / 积分 / 标准站内信）。
	// 钩子失败不回滚收款：钱确实收了，业务侧问题由钩子实现留痕。
	if OnPaid != nil {
		OnPaid(ctx, &confirmed)
	}

	// 开通投递：此前只有「渠道回调」与「手工点重新开通」两个入口，
	// 场外收款确认后同样要进入履约队列（续费单在 EnqueueProvision 内自行短路）。
	if err := s.EnqueueProvision(ctx, item.ID); err != nil {
		if s.logger != nil {
			s.logger.Warn("enqueue provision after confirm payment failed",
				zap.Uint64("order_id", item.ID), zap.Error(err))
		}
	} else {
		result.ProvisionQueued = true
	}

	// 到账提醒：客户看不到"支付成功"页面，是否告知由操作人显式选择。
	if s.notifier != nil && len(req.NotifyChannels) > 0 {
		notified, skipped := s.notifier.NotifyOrderPaid(ctx, &confirmed, req.NotifyChannels)
		result.Notified = notified
		result.NotifySkipped = skipped
	}

	detail, derr := s.FindByID(ctx, id)
	if derr != nil {
		// 订单已确认成功，详情读取失败不该把整次操作判为失败。
		result.Order = buildOrderInfo(confirmed)
		return result, nil
	}
	result.Order = detail.OrderInfo
	return result, nil
}

// revertConfirm 确认收款失败时把订单退回待支付（补偿动作）。
//
// 只在「CAS 赢了但资金侧失败」的窄窗口内使用：此时订单是我们自己刚置为 paid 的，
// 退回不会覆盖任何并发写入。退回失败只记日志 —— 极端情况下订单停在 paid 但没有
// 台账，由运营按订单号在资金流水/支付台账里核对，比抛错更可追。
func (s *orderService) revertConfirm(ctx context.Context, id uint64, reason string) {
	moved, err := s.orderRepo.TransitionStatus(ctx, id, model.OrderStatusPaid, model.OrderStatusPending)
	if err != nil || moved == 0 {
		if s.logger != nil {
			s.logger.Error("confirm payment rollback failed",
				zap.Uint64("order_id", id), zap.String("reason", reason), zap.Error(err))
		}
		return
	}
	if s.logger != nil {
		s.logger.Warn("confirm payment rolled back",
			zap.Uint64("order_id", id), zap.String("reason", reason))
	}
}

// parsePayTime 解析到账时间：接受「2006-01-02 15:04:05」与 RFC3339。
// 不做宽松解析（如只给日期）：到账时间要跟银行流水对，含糊的时间不如让运营填准。
func parsePayTime(raw string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, ErrPayTimeInvalid
}

// confirmRemark 组装确认收款的订单备注：保留原有内部备注，追加本次入账摘要。
//
// 摘要里必须带支付方式与实收金额：订单备注是运营在订单页能直接看到的信息，
// 后续对账/退款争议时不必再去翻支付台账。
func confirmRemark(existing string, req dto.ConfirmPaymentRequest, operatorName, payMethod string, paidAmount float64) string {
	parts := make([]string, 0, 4)
	if base := strings.TrimSpace(existing); base != "" {
		parts = append(parts, base)
	}
	tag := "后台确认收款:" + payMethod + " ¥" + formatAmount(paidAmount)
	if ev := strings.TrimSpace(req.EvidenceNo); ev != "" {
		tag += " 凭证:" + ev
	}
	if name := strings.TrimSpace(operatorName); name != "" {
		tag += " 操作人:" + name
	}
	parts = append(parts, tag)
	out := strings.Join(parts, " | ")
	// orders.remark 是 varchar(255)，超长截断避免入库失败（备注不是账本，截断可接受）。
	if len(out) > 255 {
		out = strings.ToValidUTF8(out[:255], "")
	}
	return out
}

// formatAmount 金额格式化（两位小数，去掉无意义的末尾零，避免 30 与 30.00 混排）。
func formatAmount(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	// 只修剪小数部分的尾零（"0.00"→"0"、"12.50"→"12.5"），整数十位零不能动（"10.00"→"10"）。
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	return s
}
