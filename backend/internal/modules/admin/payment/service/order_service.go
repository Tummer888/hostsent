package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
	"hostsent/backend/internal/modules/admin/payment/repository"
	"hostsent/backend/internal/pkg/payment"
)

// PaidHook 支付成功钩子：由装配层注入，负责调 finance 记账（入账/开通）。
// 只传基础类型与已归一的关键字段，避免 payment 反向依赖 finance。
type PaidHook func(ctx context.Context, o *model.PaymentOrder) error

// OrderService 支付单能力：下单 → 支付 → 回调确认 → 事件通知。
type OrderService interface {
	// Prepay 发起支付：创建支付单并调用渠道下单（线下渠道返回操作指引）。
	Prepay(ctx context.Context, userID uint64, req dto.PrepayRequest) (*dto.OrderInfo, error)
	List(ctx context.Context, q dto.OrderListQuery) (*dto.OrderListResponse, error)
	Get(ctx context.Context, id uint64) (*dto.OrderInfo, error)
	GetByNo(ctx context.Context, paymentNo string) (*dto.OrderInfo, error)
	// Confirm 后台人工确认到账（线下渠道）。
	Confirm(ctx context.Context, id uint64, req dto.OrderConfirmRequest, operatorID uint64) (*dto.OrderInfo, error)
	// Close 关闭支付单。
	Close(ctx context.Context, id uint64) (*dto.OrderInfo, error)
	// Sync 主动向渠道查单并推进状态（补偿回调丢失）。
	Sync(ctx context.Context, id uint64) (*dto.OrderInfo, error)
	// HandleNotify 处理渠道回调：验签 → 比额 → 幂等 → 置为已支付 → 触发钩子。
	HandleNotify(ctx context.Context, channelCode string, r *http.Request, rawBody []byte, sourceIP string) (*payment.NotifyEvent, error)
	SetPaidHook(hook PaidHook)
}

type orderService struct {
	orderRepo  repository.OrderRepository
	refundRepo repository.RefundRepository
	cbRepo     repository.CallbackRepository
	resolver   ChannelResolver
	paidHook   PaidHook
	// expireMinutes 支付单有效期（分钟）。
	expireMinutes int
}

// NewOrderService 创建支付单服务。
func NewOrderService(orderRepo repository.OrderRepository, refundRepo repository.RefundRepository, cbRepo repository.CallbackRepository, resolver ChannelResolver) OrderService {
	return &orderService{orderRepo: orderRepo, refundRepo: refundRepo, cbRepo: cbRepo, resolver: resolver, expireMinutes: 30}
}

func (s *orderService) SetPaidHook(hook PaidHook) { s.paidHook = hook }

// Prepay 发起支付。
// 幂等：同业务已有未支付单时直接复用（返回原单与支付参数），避免重复下单。
func (s *orderService) Prepay(ctx context.Context, userID uint64, req dto.PrepayRequest) (*dto.OrderInfo, error) {
	if userID == 0 {
		return nil, ErrOrderNotFound
	}
	bizType := strings.TrimSpace(req.BizType)
	if bizType == "" {
		return nil, fmt.Errorf("缺少业务类型")
	}
	amountFen := yuanToFen(req.Amount)
	if amountFen <= 0 {
		return nil, fmt.Errorf("支付金额必须大于 0")
	}
	// 复用未支付单
	if req.BizID > 0 {
		if existing, err := s.orderRepo.FindPendingByBiz(ctx, bizType, req.BizID); err == nil && existing != nil {
			return s.infoWithChannelName(ctx, existing, "")
		}
	}
	scene := req.Scene
	if scene == "" {
		scene = payment.SceneNative
	}
	ch, err := s.resolver.Resolve(ctx, userID, scene, amountFen, req.ChannelCode)
	if err != nil {
		return nil, err
	}
	cfg, err := s.resolver.BuildConfig(ctx, ch)
	if err != nil {
		return nil, err
	}
	gw, err := payment.Build(ch.Type, cfg)
	if err != nil {
		return nil, err
	}
	collector, ok := gw.(payment.Collector)
	if !ok {
		return nil, payment.ErrNotImplemented
	}
	paymentNo := genPaymentNo()
	expire := time.Now().Add(time.Duration(s.expireMinutes) * time.Minute)
	subject := req.Subject
	if subject == "" {
		subject = defaultSubject(bizType)
	}
	order := &model.PaymentOrder{
		PaymentNo:   paymentNo,
		UserID:      userID,
		BizType:     bizType,
		BizID:       req.BizID,
		BizNo:       req.BizNo,
		AmountFen:   amountFen,
		Currency:    "CNY",
		ChannelID:   ch.ID,
		ChannelCode: ch.ChannelCode,
		ChannelType: ch.Type,
		Scene:       scene,
		Status:      model.OrderStatusPaying,
		Subject:     subject,
		ClientIP:    req.ClientIP,
		ExpireAt:    &expire,
		FeeFen:      calcFee(amountFen, ch.FeeRate),
	}
	res, err := collector.Prepay(ctx, cfg, &payment.PrepayRequest{
		OutTradeNo: paymentNo,
		AmountFen:  amountFen,
		Currency:   "CNY",
		Subject:    subject,
		Scene:      scene,
		ClientIP:   req.ClientIP,
		ReturnURL:  req.ReturnURL,
	})
	if err != nil {
		return nil, err
	}
	order.PayURL = res.PayURL
	order.QRCode = res.QRCode
	order.ChannelTx = res.ChannelTx
	order.Instructions = res.Instructions
	// prepay_params 是 jsonb 列，空串非法；无附加参数时统一写 "{}"。
	order.PrepayParams = "{}"
	if len(res.Params) > 0 {
		if raw, err := jsonMarshal(res.Params); err == nil {
			order.PrepayParams = raw
		}
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}
	return s.infoWithChannelName(ctx, order, ch.Name)
}

func (s *orderService) List(ctx context.Context, q dto.OrderListQuery) (*dto.OrderListResponse, error) {
	items, total, err := s.orderRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	names := s.channelNames(ctx)
	resp := make([]dto.OrderInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildOrderInfo(item, names[item.ChannelCode]))
	}
	return &dto.OrderListResponse{
		Items: resp,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}

func (s *orderService) Get(ctx context.Context, id uint64) (*dto.OrderInfo, error) {
	o, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	return s.infoWithChannelName(ctx, o, "")
}

func (s *orderService) GetByNo(ctx context.Context, paymentNo string) (*dto.OrderInfo, error) {
	o, err := s.orderRepo.FindByNo(ctx, paymentNo)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	return s.infoWithChannelName(ctx, o, "")
}

// Confirm 后台人工确认到账：仅允许对未支付/支付中的支付单操作。
func (s *orderService) Confirm(ctx context.Context, id uint64, req dto.OrderConfirmRequest, operatorID uint64) (*dto.OrderInfo, error) {
	o, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	if o.Status == model.OrderStatusPaid {
		return s.infoWithChannelName(ctx, o, "")
	}
	if o.Status != model.OrderStatusPending && o.Status != model.OrderStatusPaying {
		return nil, ErrOrderNotPayable
	}
	if err := s.markPaid(ctx, o, req.ChannelTx, req.Remark); err != nil {
		return nil, err
	}
	return s.infoWithChannelName(ctx, o, "")
}

// markPaid 置为已支付并触发入账钩子（钩子失败仅记录，不阻断支付状态）。
func (s *orderService) markPaid(ctx context.Context, o *model.PaymentOrder, channelTx, remark string) error {
	if channelTx != "" {
		o.ChannelTx = channelTx
	}
	now := time.Now()
	o.Status = model.OrderStatusPaid
	o.PaidAt = &now
	if remark != "" {
		o.Remark = remark
	}
	if err := s.orderRepo.Update(ctx, o); err != nil {
		return err
	}
	if s.paidHook != nil {
		if err := s.paidHook(ctx, o); err != nil {
			return fmt.Errorf("支付成功后处理失败: %w", err)
		}
	}
	return nil
}

func (s *orderService) Close(ctx context.Context, id uint64) (*dto.OrderInfo, error) {
	o, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	if o.Status == model.OrderStatusPaid || o.Status == model.OrderStatusRefunded {
		return nil, ErrOrderNotPayable
	}
	// 尽力通知渠道关单（渠道不支持时忽略）。
	if ch, cerr := s.resolveChannelByID(ctx, o); cerr == nil {
		if cfg, cerr := s.resolver.BuildConfig(ctx, ch); cerr == nil {
			if gw, cerr := payment.Build(ch.Type, cfg); cerr == nil {
				if closer, ok := gw.(payment.PaymentCloser); ok {
					_ = closer.Close(ctx, cfg, &payment.CloseRequest{OutTradeNo: o.PaymentNo, ChannelTx: o.ChannelTx})
				}
			}
		}
	}
	o.Status = model.OrderStatusClosed
	if err := s.orderRepo.Update(ctx, o); err != nil {
		return nil, err
	}
	return s.infoWithChannelName(ctx, o, "")
}

// Sync 主动查单：仅对支付中/待支付单有效；渠道报已支付则走 markPaid。
func (s *orderService) Sync(ctx context.Context, id uint64) (*dto.OrderInfo, error) {
	o, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	if o.Status != model.OrderStatusPending && o.Status != model.OrderStatusPaying {
		return s.infoWithChannelName(ctx, o, "")
	}
	ch, err := s.resolveChannelByID(ctx, o)
	if err != nil {
		return nil, err
	}
	cfg, err := s.resolver.BuildConfig(ctx, ch)
	if err != nil {
		return nil, err
	}
	gw, err := payment.Build(ch.Type, cfg)
	if err != nil {
		return nil, err
	}
	querier, ok := gw.(payment.PaymentQuerier)
	if !ok {
		return nil, payment.ErrNotImplemented
	}
	state, err := querier.Query(ctx, cfg, &payment.QueryRequest{OutTradeNo: o.PaymentNo, ChannelTx: o.ChannelTx})
	if err != nil {
		return nil, err
	}
	switch state.Status {
	case model.OrderStatusPaid:
		if state.AmountFen > 0 && state.AmountFen != o.AmountFen {
			return nil, payment.ErrAmountMismatch
		}
		if err := s.markPaid(ctx, o, state.ChannelTx, "主动查单确认"); err != nil {
			return nil, err
		}
	case model.OrderStatusClosed, model.OrderStatusFailed:
		o.Status = state.Status
		_ = s.orderRepo.Update(ctx, o)
	}
	return s.infoWithChannelName(ctx, o, "")
}

// HandleNotify 处理渠道回调：验签 → 比额 → 幂等 → 置为已支付。
func (s *orderService) HandleNotify(ctx context.Context, channelCode string, r *http.Request, rawBody []byte, sourceIP string) (*payment.NotifyEvent, error) {
	ch, err := s.findChannelByCode(ctx, channelCode)
	if err != nil {
		return nil, err
	}
	cfg, err := s.resolver.BuildConfig(ctx, ch)
	if err != nil {
		return nil, err
	}
	log := &model.PaymentCallbackLog{
		ChannelID:   ch.ID,
		ChannelCode: ch.ChannelCode,
		RawBody:     string(rawBody),
		Headers:     headerSnapshot(r),
		SourceIP:    sourceIP,
	}
	gw, err := payment.Build(ch.Type, cfg)
	if err != nil {
		log.HandleStatus = "error"
		log.HandleMsg = err.Error()
		_ = s.cbRepo.Create(ctx, log)
		return nil, err
	}
	parser, ok := gw.(payment.NotifyParser)
	if !ok {
		log.HandleStatus = "error"
		log.HandleMsg = "渠道未实现回调解析"
		_ = s.cbRepo.Create(ctx, log)
		return nil, payment.ErrNotImplemented
	}
	event, err := parser.ParseNotify(ctx, cfg, r, rawBody)
	if err != nil {
		log.HandleStatus = "error"
		log.HandleMsg = "验签/解析失败: " + err.Error()
		_ = s.cbRepo.Create(ctx, log)
		return nil, err
	}
	log.NotifyID = event.NotifyID
	log.PaymentNo = event.OutTradeNo
	log.VerifyOK = true
	log.AmountFen = event.AmountFen

	// 幂等：同渠道同 notify_id 已处理过则直接忽略。
	if dup, _ := s.cbRepo.ExistsNotify(ctx, ch.ID, event.NotifyID); dup {
		log.HandleStatus = "duplicate"
		log.HandleMsg = "重复回调，已忽略"
		_ = s.cbRepo.Create(ctx, log)
		return event, nil
	}
	o, err := s.orderRepo.FindByNo(ctx, event.OutTradeNo)
	if err != nil {
		log.HandleStatus = "error"
		log.HandleMsg = "支付单不存在: " + event.OutTradeNo
		_ = s.cbRepo.Create(ctx, log)
		return nil, ErrOrderNotFound
	}
	// 比额：渠道回报金额必须与本地支付单一致。
	if event.AmountFen > 0 && event.AmountFen != o.AmountFen {
		log.HandleStatus = "error"
		log.HandleMsg = fmt.Sprintf("金额不一致: 渠道 %d 分, 本地 %d 分", event.AmountFen, o.AmountFen)
		_ = s.cbRepo.Create(ctx, log)
		return nil, payment.ErrAmountMismatch
	}
	if o.Status == model.OrderStatusPaid {
		log.HandleStatus = "duplicate"
		log.HandleMsg = "支付单已支付，忽略重复回调"
		_ = s.cbRepo.Create(ctx, log)
		return event, nil
	}
	if event.Status != model.OrderStatusPaid {
		log.HandleStatus = "ignored"
		log.HandleMsg = "渠道状态非成功: " + event.Status
		_ = s.cbRepo.Create(ctx, log)
		return event, nil
	}
	if err := s.markPaid(ctx, o, event.ChannelTx, "渠道回调确认"); err != nil {
		log.HandleStatus = "error"
		log.HandleMsg = err.Error()
		_ = s.cbRepo.Create(ctx, log)
		return nil, err
	}
	log.HandleStatus = "ok"
	log.HandleMsg = "入账成功"
	_ = s.cbRepo.Create(ctx, log)
	return event, nil
}

// ---- 内部工具 ----

func (s *orderService) infoWithChannelName(ctx context.Context, o *model.PaymentOrder, name string) (*dto.OrderInfo, error) {
	if name == "" {
		names := s.channelNames(ctx)
		name = names[o.ChannelCode]
	}
	info := buildOrderInfo(*o, name)
	return &info, nil
}

func (s *orderService) channelNames(ctx context.Context) map[string]string {
	out := map[string]string{}
	if cr, ok := s.resolver.(*channelService); ok {
		if items, err := cr.repo.ListEnabled(ctx); err == nil {
			for _, c := range items {
				out[c.ChannelCode] = c.Name
			}
		}
	}
	return out
}

func (s *orderService) findChannelByCode(ctx context.Context, code string) (*model.PaymentChannel, error) {
	cr, ok := s.resolver.(*channelService)
	if !ok {
		return nil, ErrChannelNotFound
	}
	ch, err := cr.repo.FindByCode(ctx, code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	return ch, nil
}

func (s *orderService) resolveChannelByID(ctx context.Context, o *model.PaymentOrder) (*model.PaymentChannel, error) {
	cr, ok := s.resolver.(*channelService)
	if !ok {
		return nil, ErrChannelNotFound
	}
	ch, err := cr.repo.FindByID(ctx, o.ChannelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	return ch, nil
}

func mapOrderErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOrderNotFound
	}
	return err
}

func defaultSubject(bizType string) string {
	switch bizType {
	case model.BizTypeRecharge:
		return "账户余额充值"
	case model.BizTypeOrder:
		return "云产品订单支付"
	case model.BizTypeBill:
		return "账单支付"
	default:
		return "平台费用支付"
	}
}

// calcFee 按费率计算渠道手续费（分，四舍五入）。
func calcFee(amountFen int64, feeRate float64) int64 {
	if feeRate <= 0 {
		return 0
	}
	return int64(float64(amountFen)*feeRate + 0.5)
}

func headerSnapshot(r *http.Request) string {
	if r == nil {
		return ""
	}
	keys := []string{"Content-Type", "User-Agent", "X-Forwarded-For"}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if v := r.Header.Get(k); v != "" {
			parts = append(parts, k+": "+v)
		}
	}
	return strings.Join(parts, "\n")
}
