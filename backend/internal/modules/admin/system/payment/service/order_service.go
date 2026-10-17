package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/system/payment/dto"
	"hostsent/backend/internal/modules/admin/system/payment/model"
	"hostsent/backend/internal/modules/admin/system/payment/repository"
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
	// RecordOfflinePayment 后台人工入账（场外收款登记）：为业务单补登一张「已收款」支付台账。
	//
	// 与 Confirm 的分工：Confirm 是给「客户已发起的支付单」确认到账；
	// 本方法是客户没走收银台（对公转账/线下现金）时，直接补登一张已收款台账，
	// 让这笔钱在支付台账里可查、可对账、退款时能定位资金来源。
	// 幂等：同业务单已有已支付台账时直接返回既有单，不重复登记。
	RecordOfflinePayment(ctx context.Context, req dto.OfflinePaymentRequest, userID uint64, bizType string, bizID uint64, bizNo string, amount float64, operatorID uint64) (*dto.OrderInfo, error)
	// IsOfflinePaymentOrder 该业务单的已收款台账是否来自后台人工入账（origin=offline）。
	//
	// 退款侧据此分流：场外收款没有可调用的渠道退款接口，原路退回只能由财务线下汇款
	// （返回 manual 让订单侧标记待汇款），**不能回落余额回补** —— 那会让客户拿到两份钱。
	IsOfflinePaymentOrder(ctx context.Context, bizNo string) (bool, error)
	// Close 关闭支付单。
	Close(ctx context.Context, id uint64) (*dto.OrderInfo, error)
	// CloseByBiz 关闭某业务单的全部未支付支付单（用户取消/超时关单时调用），返回关闭数量。
	// 业务单作废后支付单必须一起关：否则用户仍可付款到已作废的业务单，形成悬空资金。
	CloseByBiz(ctx context.Context, bizType string, bizID uint64) (int, error)
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

// RecordOfflinePayment 后台人工入账（场外收款登记）。
//
// 这条链路与收银台预支付（Prepay）是并列的两条「支付台账来源」：
// 客户没走收银台时，支付单行根本不存在，所以只能由后台补登，而不是确认已有的单。
// 登记出来的支付单 origin=offline，与渠道回调单区分开 —— 原路退回时不能调渠道
// 退款接口（没有渠道交易可退），渠道对账也不该把它算进去。
//
// 幂等：同业务单已存在已支付台账时直接返回既有单。重复点击不会产生第二笔收入，
// 也避免同一订单出现两条 paid 台账（销售/对账口径会翻倍）。
func (s *orderService) RecordOfflinePayment(
	ctx context.Context,
	req dto.OfflinePaymentRequest,
	userID uint64, bizType string, bizID uint64, bizNo string, amount float64,
	operatorID uint64,
) (*dto.OrderInfo, error) {
	if bizNo == "" || bizType == "" || userID == 0 {
		return nil, ErrOrderNotFound
	}
	// 幂等前置：已有已支付台账直接复用（换渠道重复登记也走这里）。
	if existing, err := s.orderRepo.FindPaidByBiz(ctx, bizType, bizNo); err == nil && existing != nil {
		return s.infoWithChannelName(ctx, existing, "")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	amountFen := yuanToFen(amount)
	if amountFen <= 0 {
		return nil, ErrOfflineAmountInvalid
	}
	payMethod := strings.TrimSpace(req.PayMethod)
	if payMethod == "" {
		return nil, ErrOfflinePayMethodRequired
	}

	now := time.Now()
	order := &model.PaymentOrder{
		PaymentNo:  genPaymentNo(),
		UserID:     userID,
		BizType:    bizType,
		BizID:      bizID,
		BizNo:      bizNo,
		AmountFen:  amountFen,
		Currency:   "CNY",
		Status:     model.OrderStatusPaid,
		ChannelTx:  strings.TrimSpace(req.EvidenceNo),
		Subject:    defaultSubject(bizType),
		PaidAt:     &now,
		ExpireAt:   nil,
		Origin:     model.OriginOffline,
		OperatorID: operatorID,
		Remark:     offlineRemark(req),
	}
	// 渠道实例：填了就按该渠道登记（客户确实走了这个渠道，只是没经过收银台）；
	// 不填则留空，前端按「线下人工」展示 —— 这样即使平台没配任何渠道也能入账。
	if code := strings.TrimSpace(req.ChannelCode); code != "" {
		if ch, err := s.findChannelByCode(ctx, code); err == nil {
			order.ChannelID = ch.ID
			order.ChannelCode = ch.ChannelCode
			order.ChannelType = ch.Type
		}
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}
	// 入账钩子照常触发：订单侧据此置为已支付并投递开通。
	// 钩子失败不回滚台账（钱确实收了），返回错误让运营看到并重试开通。
	if s.paidHook != nil {
		if err := s.paidHook(ctx, order); err != nil {
			return nil, fmt.Errorf("入账成功但后续处理失败: %w", err)
		}
	}
	return s.infoWithChannelName(ctx, order, "")
}

// IsOfflinePaymentOrder 该业务单的已收款台账是否来自后台人工入账（origin=offline）。
func (s *orderService) IsOfflinePaymentOrder(ctx context.Context, bizNo string) (bool, error) {
	if strings.TrimSpace(bizNo) == "" {
		return false, nil
	}
	o, err := s.orderRepo.FindPaidByBiz(ctx, dto.BizTypeOrder, bizNo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return o.Origin == model.OriginOffline, nil
}

// offlineRemark 组装人工入账备注：保留运营填写的说明，并补上登记人。
// 场外收款是"人在系统外收了钱"，对账时第一个要问的就是"谁登记的"。
func offlineRemark(req dto.OfflinePaymentRequest) string {
	parts := make([]string, 0, 3)
	if r := strings.TrimSpace(req.Remark); r != "" {
		parts = append(parts, r)
	}
	parts = append(parts, "后台人工入账")
	if name := strings.TrimSpace(req.OperatorName); name != "" {
		parts = append(parts, "登记人:"+name)
	}
	out := strings.Join(parts, " | ")
	// payment_orders.remark 是 varchar(255)，超长截断避免入库报错。
	if len(out) > 255 {
		out = out[:255]
	}
	return out
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
				if closer, ok := gw.(payment.Closer); ok {
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

// CloseByBiz 关闭某业务单的全部未支付支付单（用户取消/超时关单时调用）。
//
// 逐张走 Close：需要尽力通知渠道关单，直接改库会漏掉渠道侧的单（用户还能在渠道页付）。
// 单张失败不阻断其余（下一轮调度或人工可再处理），返回成功关闭数量。
func (s *orderService) CloseByBiz(ctx context.Context, bizType string, bizID uint64) (int, error) {
	if bizID == 0 {
		return 0, nil
	}
	items, err := s.orderRepo.ListPendingByBiz(ctx, bizType, bizID)
	if err != nil {
		return 0, err
	}
	closed := 0
	for _, it := range items {
		if _, err := s.Close(ctx, it.ID); err != nil {
			continue
		}
		closed++
	}
	return closed, nil
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
	querier, ok := gw.(payment.Querier)
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
