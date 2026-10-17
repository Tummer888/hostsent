package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/admin/order/repository"
)

// OrderService 定义订单域业务能力（含退款管理）。
type OrderService interface {
	// 订单管理
	List(ctx context.Context, q dto.OrderListQuery) (*dto.OrderListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.OrderDetail, error)
	Cancel(ctx context.Context, id uint64, operatorID uint64, operatorName string) error
	UpdateRemark(ctx context.Context, id uint64, remark string, operatorID uint64) error
	CreateRefund(ctx context.Context, id uint64, req dto.RefundCreateRequest, operatorID uint64, operatorName string) (*dto.RefundInfo, error)
	Activate(ctx context.Context, id uint64) error
	// EnqueueProvision 投递异步开通任务（T5.1）：下单/人工重试统一入口，
	// 未注入任务队列时回落为同步 Activate，保持既有行为。
	EnqueueProvision(ctx context.Context, id uint64) error
	Stats(ctx context.Context) (*dto.OrderStatsResponse, error)
	// SetCashbackHook 注入订单完成后的推广返现计提钩子（装配层调用）。
	SetCashbackHook(hook OrderCashbackHook)
	// SetRefundHook 注入退款审核通过后的返现冲减钩子（装配层调用）。
	SetRefundHook(hook OrderRefundHook)
	// SetProvisionQueue 注入异步开通任务队列（装配层调用，T5.1）。
	SetProvisionQueue(queue ProvisionQueue)
	// SetProvisionTaskReader 注入开通任务批量读取（装配层调用，供订单列表/详情
	// 展示履约状态与失败原因）。
	SetProvisionTaskReader(fn func(ctx context.Context, orderIDs []uint64) (map[uint64]*model.ProvisionTask, error))
	// SetPendingInstanceReleaser 注入「订单作废即删占位实例」钩子（装配层调用）。
	SetPendingInstanceReleaser(fn func(ctx context.Context, orderID uint64) (int64, error))
	// SetStockReleaser 注入「订单作废即回补库存」钩子（装配层调用）。
	// 代下单同样占用库存（SKU 或商品级），取消/关单必须回补，否则每取消一单就永久少一份库存。
	// 传整单而不是 orderID：回补要读 SpecCode/Quantity/ProductID，只有 ID 还得再查一次。
	SetStockReleaser(fn func(ctx context.Context, order *model.Order) error)
	// SetLogger 注入日志器（装配层调用）。
	SetLogger(l *zap.Logger)
	// SetProductSourceModeResolver 注入商品链路判据解析（装配层调用，仅用于任务留痕）。
	SetProductSourceModeResolver(fn func(ctx context.Context, productID uint64) string)
	// SetSalesOwnerResolver 注入销售归属快照解析（装配层调用，doc86 §3.4，仅用于后台代下单）。
	SetSalesOwnerResolver(r SalesOwnerResolver)
	// MarkPaidByChannel 支付中心回调确认到账：pending→paid 并记录渠道支付方式。
	// 已是 paid/active 时幂等返回；返回是否发生了状态迁移（供装配层决定是否继续开通）。
	MarkPaidByChannel(ctx context.Context, orderNo, payMethod, channelTx string) (bool, error)
	// ActivateByNo 按订单号触发开通（支付成功后调用，复用异步任务队列）。
	ActivateByNo(ctx context.Context, orderNo string) error
	// SetChannelRefundHook 注入渠道退款钩子（装配层调用）：退款审核通过后原路退回，
	// 无渠道支付记录时钩子实现回落为余额回补。
	SetChannelRefundHook(hook OrderChannelRefundHook)
	// SetInstanceRefundHook 注入实例侧退款钩子（装配层调用，doc61 §8 第 2 条）：
	// 退款审核通过后按策略（lifecycle_policies.refund_action）暂停/销毁关联实例。
	SetInstanceRefundHook(hook OrderInstanceRefundHook)
	// 退款管理
	ListRefunds(ctx context.Context, q dto.RefundListQuery) (*dto.RefundListResponse, error)
	FindRefund(ctx context.Context, id uint64) (*dto.RefundInfo, error)
	ApproveRefund(ctx context.Context, id uint64, operatorID uint64, operatorName string, req dto.RefundApproveRequest) (*dto.RefundInfo, error)
	RejectRefund(ctx context.Context, id uint64, operatorID uint64, operatorName string, req dto.RefundApproveRequest) (*dto.RefundInfo, error)
	// UserCreateRefund 客户自助申请退款（uc 通道）：按退款策略判定时限与免审，
	// 命中免审条件时直接完成退款并置事后待复核；其余情况落待审核单等人工处理。
	UserCreateRefund(ctx context.Context, userID, orderID uint64, req dto.UserRefundRequest) (*dto.UserRefundResult, error)
	// RefundPolicyHint 订单可退性提示（uc/管理端详情展示是否给申请入口）。
	RefundPolicyHint(ctx context.Context, order *model.Order) *dto.OrderRefundPolicyHint
	// ConfirmPayment 后台确认收款（场外收款人工入账）：待支付 → 已支付，
	// 强制记录支付方式与实收金额，登记支付台账并投递开通，可选发到账提醒。
	ConfirmPayment(ctx context.Context, id uint64, req dto.ConfirmPaymentRequest, operatorID uint64, operatorName string) (*dto.ConfirmPaymentResult, error)
	// SetOfflinePaymentRecorder 注入「补登支付台账」端口（装配层调用，支付中心实现）。
	// 未注入时 ConfirmPayment 返回错误而不静默记账：钱收了却查不到台账是财务事故。
	SetOfflinePaymentRecorder(r OfflinePaymentRecorder)
	// SetPaymentNotifier 注入到账提醒发送端口（装配层调用，通知模块实现）。
	// 未注入时跳过提醒（订单支付状态不受影响）。
	SetPaymentNotifier(n PaymentNotifier)
	// SetBalanceDeductor 注入余额扣款端口（装配层调用，财务账务核心实现）。
	// 未注入时「余额支付」确认收款返回错误（不静默放过扣款）。
	SetBalanceDeductor(d BalanceDeductor)
	// 事后复核（免审核自动退款）
	ReviewRefund(ctx context.Context, id uint64, operatorID uint64, operatorName, remark string) (*dto.RefundInfo, error)
	// SetRefundPolicyMatcher 注入退款策略匹配（装配层调用）。
	SetRefundPolicyMatcher(m RefundPolicyMatcher)
	// SetChannelPaymentChecker 注入「订单是否有渠道支付记录」判据（装配层调用）：
	// 无渠道支付记录的订单（余额支付）不允许选原路退回。
	SetChannelPaymentChecker(fn func(ctx context.Context, orderNo string) bool)
	// SetRefundAttachmentReader 注入退款凭据附件读取（装配层调用，详情展示用）。
	SetRefundAttachmentReader(svc RefundAttachmentService)
	// 订单核验（运营人工核对开通结果，仅标记不流转状态）
	Verify(ctx context.Context, id uint64, operatorID uint64, operatorName string, remark string) error
	// 批量操作：逐条复用单条逻辑（含状态守卫与副作用），返回成功/跳过/失败明细。
	BatchCancel(ctx context.Context, ids []uint64, operatorID uint64, operatorName string) (*dto.BatchActionResponse, error)
	BatchActivate(ctx context.Context, ids []uint64) (*dto.BatchActionResponse, error)
	BatchVerify(ctx context.Context, ids []uint64, operatorID uint64, operatorName string, remark string) (*dto.BatchActionResponse, error)
	BatchDelete(ctx context.Context, ids []uint64) (*dto.BatchActionResponse, error)
	BatchApproveRefunds(ctx context.Context, ids []uint64, operatorID uint64, operatorName string, remark string) (*dto.BatchActionResponse, error)
	BatchRejectRefunds(ctx context.Context, ids []uint64, operatorID uint64, operatorName string, remark string) (*dto.BatchActionResponse, error)
	// SetInstanceRefReader 注入订单关联实例的批量读取（装配层调用，列表/详情展示跳转用）。
	SetInstanceRefReader(fn func(ctx context.Context, orderIDs []uint64) (map[uint64]OrderInstanceRef, error))
	// SetVerifyRequiredReader 注入「是否需要人工核验」判据（装配层读系统配置 order_verify_required）。
	SetVerifyRequiredReader(fn func(ctx context.Context) bool)
}

// OrderInstanceRef 订单关联实例的最小引用。
//
// 只在订单域内使用的基础类型：由装配层把实例域的行转成它，订单模块不 import 实例域
// （CONTRIBUTING §3 依赖方向）。
type OrderInstanceRef struct {
	ID uint64 // instances.id（前端跳转 /instances/detail/:id 用）
	No string // 实例号（上游实例号优先，占位期为 pending-<订单号>）
	// Pending 表示该行仍是「待开通」占位实例（provider_instance_id 为空未转正）。
	Pending bool
}

// OrderCashbackHook 订单完成（开通成功）后的推广返现计提钩子。
// 仅传基础类型，避免订单模块反向依赖返现模块。
type OrderCashbackHook func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, baseAmount float64, isRenewal bool) error

// OrderRefundHook 退款审核通过后的返现冲减钩子：按退款额占实付比例冲减邀请人返现。
type OrderRefundHook func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, paidAmount, refundAmount float64, refundNo string) error

// OrderChannelRefundHook 退款审核通过后的资金出口钩子（doc34 F-03 / doc35 §6.4 / doc36 §3.2）。
//
// 钩子实现按退款去向决定资金动作，并回传「资金实际怎么出去的」：
//
//	RefundOutcomeBalance 回补余额（余额支付，或渠道不可用回落）；
//	RefundOutcomeChannel 已建渠道退款单，按渠道异步流出；
//	RefundOutcomeManual  收款本身是场外人工入账 → 无渠道可退，等财务线下汇款。
//
// 为什么必须区分 manual 而不能沿用"建单失败就回落余额"：
// 场外收款的钱是客户打到公司账户的，退款时若又回补平台余额，等于退了两份
// （银行那笔 + 余额这笔），客户白拿一倍。manual 表示"钱要汇出去、但还没汇"，
// 由财务按凭据执行，不进余额。
//
// 失败不影响退款审核结果，装配层负责记录日志并留待人工处理。
// channelRefundNo 供退款单回填（无渠道退款时为空串）。
type OrderChannelRefundHook func(ctx context.Context, orderID uint64, orderNo string, orderRefundNo string, userID uint64, amount, fee float64, mode string) (channelRefundNo string, outcome string, err error)

// 退款资金出口结果（OrderChannelRefundHook 的第二返回值）。
const (
	// RefundOutcomeBalance 资金回补到平台余额（钱仍在平台内）。
	RefundOutcomeBalance string = "balance"
	// RefundOutcomeChannel 已建渠道退款单，按渠道异步流出平台。
	RefundOutcomeChannel string = "channel"
	// RefundOutcomeManual 待财务线下汇款（场外收款的原路退回：无渠道接口可调）。
	RefundOutcomeManual string = "manual"
)

// OrderInstanceRefundHook 退款审核通过后的「实例侧」钩子（doc61 §8 第 2 条）。
//
// 职责单一：判断该订单是否有关联实例，并按策略（lifecycle_policies.refund_action）
// 决定暂停/销毁。是否动作由**策略**决定，钩子实现不硬编码规则 —— doc61 只说
// 「按规则停服/销毁」却没定义规则，这里把规则外置为可配置项，默认 none（不动作）。
//
// fullRefund 表示本次退款是否已覆盖订单全部实付：按常理只有全额退款才谈得上处置实例，
// 部分退款（补偿）不应动客户在用的机器；是否采纳由钩子实现按策略决定。
type OrderInstanceRefundHook func(ctx context.Context, orderID uint64, orderNo string, userID uint64, refundAmount, paidAmount float64, fullRefund bool, refundNo string) error

// ProvisionQueue 异步开通任务队列（T5.1，由 ProvisionEnqueuer 实现）。
// 订单模块只依赖这一最小接口，避免与 uc/装配层形成反向依赖。
type ProvisionQueue interface {
	EnqueueProvision(ctx context.Context, order *model.Order, sourceMode string) error
}

// OfflinePaymentRecorder 场外收款台账登记端口（由支付中心实现，装配层注入）。
//
// 只声明订单域真正需要的方法（CONTRIBUTING §3：跨域走最小接口），
// 参数用基础类型，避免订单模块反向 import 支付中心 dto。
type OfflinePaymentRecorder interface {
	// Record 补登一张「已收款」人工台账，返回台账单号与是否新建（false=命中幂等复用）。
	Record(ctx context.Context, order *model.Order, req OfflinePaymentRecord) (*OfflinePaymentRecordResult, error)
}

// OfflinePaymentRecord 台账登记入参（订单域 → 支付中心的本地结构）。
type OfflinePaymentRecord struct {
	PayMethod    string  // 支付方式（客户实际付款方式）
	PaidAmount   float64 // 实收金额（元）
	PayTime      time.Time
	EvidenceNo   string // 收款凭证号
	ChannelCode  string // 收款渠道实例编码（可空）
	Remark       string
	OperatorID   uint64
	OperatorName string
}

// OfflinePaymentRecordResult 台账登记结果。
type OfflinePaymentRecordResult struct {
	PaymentNo string // 支付台账单号
	Created   bool   // 本次是否新建（false=已有台账，幂等复用）
}

// PaymentNotifier 到账提醒发送端口（由通知模块实现，装配层注入）。
//
// 只发「订单已收款」这一件事，且由人工入账操作人显式勾选渠道；
// 传订单号而非常规通知事件参数，避免订单域理解通知模块的事件/模板模型。
type PaymentNotifier interface {
	// NotifyOrderPaid 发送订单收款提醒，返回实际发出的渠道与跳过原因（渠道 → 原因）。
	NotifyOrderPaid(ctx context.Context, order *model.Order, channels []string) (notified []string, skipped map[string]string)
}

// BalanceDeductor 余额扣款端口（由财务账务核心实现，装配层注入）。
//
// 确认收款选「余额支付」时走这条真实扣款路径（与后台代客下单同口径）：
// 余额支付的钱本就在平台内，确认收款必然意味着这笔钱从客户余额划走。
// 只声明订单域用到的方法（CONTRIBUTING §3），金额与订单信息用基础类型传入。
type BalanceDeductor interface {
	// DeductOrderPayment 扣客户余额付订单款，幂等键为订单号（重复调用不重复扣）。
	// 余额不足时返回 ErrInsufficientBalance（由装配层适配 admin 域错误变量）。
	DeductOrderPayment(ctx context.Context, order *model.Order, amount float64, operatorID uint64) error
}

type orderService struct {
	orderRepo  repository.OrderRepository
	itemRepo   repository.OrderItemRepository
	refundRepo repository.RefundRepository
	provision  ProvisionAdapter
	// provisionQueue 可选：异步开通任务队列，未注入时 Activate/EnqueueProvision 走同步履约。
	provisionQueue ProvisionQueue
	// productSourceMode 可选：按商品 ID 解析链路判据（self/upstream，D6），仅用于任务留痕。
	productSourceMode func(ctx context.Context, productID uint64) string
	// cashbackHook 可选：订单开通成功后计提推广返现，为 nil 时跳过。
	cashbackHook OrderCashbackHook
	// refundHook 可选：退款审核通过后冲减已计提返现，为 nil 时跳过。
	refundHook OrderRefundHook
	// channelRefundHook 可选：退款审核通过后走渠道原路退回（doc35 S1），为 nil 时跳过。
	channelRefundHook OrderChannelRefundHook
	// instanceRefundHook 可选：退款审核通过后按策略处置关联实例（doc61 §8），为 nil 时跳过。
	instanceRefundHook OrderInstanceRefundHook
	// salesOwner 可选：后台代下单时解析销售归属快照（doc86 §3.4），为 nil 时快照落 0。
	salesOwner SalesOwnerResolver
	// provisionTasks 可选：批量读取开通履约任务，供订单列表/详情展示"开通失败原因"
	// 与人工待处理状态（manual）。为 nil 时列表不返回该字段（不影响既有功能）。
	provisionTasks func(ctx context.Context, orderIDs []uint64) (map[uint64]*model.ProvisionTask, error)
	// pendingInstanceReleaser 可选：删除订单的「待开通」占位实例（后台取消待支付订单时）。
	// 为 nil 时跳过（不影响取消主流程）。
	pendingInstanceReleaser func(ctx context.Context, orderID uint64) (int64, error)
	// stockReleaser 可选：订单作废时回补占用的库存（SKU 或商品级）。
	// 为 nil 时跳过（不影响取消主流程）。
	stockReleaser func(ctx context.Context, order *model.Order) error
	// instanceRefs 可选：批量读取订单关联实例（列表/详情「关联实例」跳转用）。
	// 为 nil 时列表不返回该字段（不影响既有功能）。
	instanceRefs func(ctx context.Context, orderIDs []uint64) (map[uint64]OrderInstanceRef, error)
	// verifyRequired 可选：是否需要人工核验（系统配置 order_verify_required）。
	// 为 nil 或返回 false 时，读取期不推导「待核验」，核验入口仍可用（运营可随时手工核验）。
	verifyRequired func(ctx context.Context) bool
	// refundPolicy 可选：退款策略匹配（按商品来源+分类判定时限与免审）。
	// 为 nil 时客户自助退款一律拒绝（保守默认：未配置策略即不开放，见 RefundPolicyHint）。
	refundPolicy RefundPolicyMatcher
	// channelPaymentChecker 可选：订单是否存在渠道支付记录。
	// 为 nil 时保守处理（视为无渠道记录 → 只允许退回余额）。
	channelPaymentChecker func(ctx context.Context, orderNo string) bool
	// refundAttachments 可选：退款凭据附件读取（退款详情展示）。
	refundAttachments RefundAttachmentService
	// offlinePayments 可选：场外收款台账登记（支付中心实现）。
	// ConfirmPayment 未注入时直接报错：确认收款必须留下台账，不能只改订单状态。
	offlinePayments OfflinePaymentRecorder
	// balanceDeductor 可选：确认收款选「余额支付」时的真实扣款（财务账务核心实现）。
	balanceDeductor BalanceDeductor
	// notifier 可选：到账提醒发送（通知模块实现）。为 nil 时跳过提醒。
	notifier PaymentNotifier
	// logger 可选日志。
	logger *zap.Logger
}

// SetPendingInstanceReleaser 注入「订单作废即删占位实例」钩子（装配层调用）。
func (s *orderService) SetPendingInstanceReleaser(fn func(ctx context.Context, orderID uint64) (int64, error)) {
	s.pendingInstanceReleaser = fn
}

// SetStockReleaser 注入「订单作废即回补库存」钩子（装配层调用）。
func (s *orderService) SetStockReleaser(fn func(ctx context.Context, order *model.Order) error) {
	s.stockReleaser = fn
}

// SetProvisionTaskReader 注入开通任务批量读取（装配层调用，避免订单服务依赖任务仓储）。
func (s *orderService) SetProvisionTaskReader(fn func(ctx context.Context, orderIDs []uint64) (map[uint64]*model.ProvisionTask, error)) {
	s.provisionTasks = fn
}

// SetInstanceRefReader 注入订单关联实例的批量读取（装配层调用）。
func (s *orderService) SetInstanceRefReader(fn func(ctx context.Context, orderIDs []uint64) (map[uint64]OrderInstanceRef, error)) {
	s.instanceRefs = fn
}

// SetVerifyRequiredReader 注入「是否需要人工核验」判据（装配层读系统配置）。
func (s *orderService) SetVerifyRequiredReader(fn func(ctx context.Context) bool) {
	s.verifyRequired = fn
}

// SetRefundPolicyMatcher 注入退款策略匹配（装配层调用）。
func (s *orderService) SetRefundPolicyMatcher(m RefundPolicyMatcher) {
	s.refundPolicy = m
}

// SetChannelPaymentChecker 注入渠道支付记录判据（装配层调用）。
func (s *orderService) SetChannelPaymentChecker(fn func(ctx context.Context, orderNo string) bool) {
	s.channelPaymentChecker = fn
}

// SetRefundAttachmentReader 注入退款凭据附件读取（装配层调用，详情展示用）。
func (s *orderService) SetRefundAttachmentReader(svc RefundAttachmentService) {
	s.refundAttachments = svc
}

// SetOfflinePaymentRecorder 注入场外收款台账登记（装配层调用，支付中心实现）。
func (s *orderService) SetOfflinePaymentRecorder(r OfflinePaymentRecorder) {
	s.offlinePayments = r
}

// SetPaymentNotifier 注入到账提醒发送（装配层调用，通知模块实现）。
func (s *orderService) SetPaymentNotifier(n PaymentNotifier) { s.notifier = n }

// SetBalanceDeductor 注入余额扣款（装配层调用，财务账务核心实现）。
func (s *orderService) SetBalanceDeductor(d BalanceDeductor) { s.balanceDeductor = d }

// SetLogger 注入日志器（装配层调用）。
func (s *orderService) SetLogger(l *zap.Logger) { s.logger = l }

// SalesOwnerResolver 销售归属解析（装配层注入，避免订单模块依赖销售模块）。
type SalesOwnerResolver interface {
	SalesAdminForNewOrder(ctx context.Context, userID uint64) uint64
}

// SetSalesOwnerResolver 注入销售归属解析（装配层调用）。
func (s *orderService) SetSalesOwnerResolver(r SalesOwnerResolver) {
	s.salesOwner = r
}

// NewOrderService 创建订单业务服务。
func NewOrderService(orderRepo repository.OrderRepository, itemRepo repository.OrderItemRepository, refundRepo repository.RefundRepository, provision ProvisionAdapter) OrderService {
	return &orderService{orderRepo: orderRepo, itemRepo: itemRepo, refundRepo: refundRepo, provision: provision}
}

// SetCashbackHook 注入推广返现计提钩子（装配层调用，避免改构造签名影响既有装配点）。
func (s *orderService) SetCashbackHook(hook OrderCashbackHook) {
	s.cashbackHook = hook
}

// SetRefundHook 注入返现冲减钩子（装配层调用）。
func (s *orderService) SetRefundHook(hook OrderRefundHook) {
	s.refundHook = hook
}

// SetProvisionQueue 注入异步开通任务队列（装配层调用，T5.1）。
func (s *orderService) SetProvisionQueue(queue ProvisionQueue) {
	s.provisionQueue = queue
}

// SetProductSourceModeResolver 注入商品链路判据解析（装配层调用）。
func (s *orderService) SetProductSourceModeResolver(fn func(ctx context.Context, productID uint64) string) {
	s.productSourceMode = fn
}

// SetChannelRefundHook 注入渠道退款钩子（装配层调用）。
func (s *orderService) SetChannelRefundHook(hook OrderChannelRefundHook) {
	s.channelRefundHook = hook
}

// SetInstanceRefundHook 注入实例侧退款钩子（装配层调用，doc61 §8）。
func (s *orderService) SetInstanceRefundHook(hook OrderInstanceRefundHook) {
	s.instanceRefundHook = hook
}

// MarkPaidByChannel 支付中心确认到账：pending→paid，记录渠道支付方式与渠道交易号。
// 已 paid/active/provisioning 等已收款状态幂等返回 false；返回 true 表示本次完成了迁移。
//
// 渠道路径下单时不扣款，paid_amount 仍为 0，这里以订单应付（total_amount）回写：
// 不回写会让销售额统计（SUM(paid_amount)）与订单详情的实付金额漏掉全部渠道单。
func (s *orderService) MarkPaidByChannel(ctx context.Context, orderNo, payMethod, channelTx string) (bool, error) {
	item, err := s.orderRepo.FindByNo(ctx, orderNo)
	if err != nil {
		return false, mapOrderErr(err)
	}
	if item.Status != model.OrderStatusPending {
		// 已收款/已开通：幂等，不重复迁移与开通。
		return false, nil
	}
	if err := EnsureStatus(item.Status, model.OrderStatusPaid); err != nil {
		return false, err
	}
	now := time.Now()
	item.Status = model.OrderStatusPaid
	if payMethod != "" {
		item.PayMethod = payMethod
	}
	item.PayTime = &now
	item.Remark = remarkWithChannelTx(item.Remark, channelTx)
	if item.PaidAmount <= 0 {
		item.PaidAmount = payableAmount(item)
	}
	if err := s.orderRepo.Update(ctx, item); err != nil {
		return false, err
	}
	// 订单支付成功扩展点（doc60）：续费联动 / 积分 / 通知。钩子失败不回滚支付状态，
	// 与渠道回调的容错口径一致（到账已确认，业务侧问题由钩子实现自行留痕）。
	if OnPaid != nil {
		OnPaid(ctx, item)
	}
	return true, nil
}

// ActivateByNo 按订单号触发开通（支付成功回调路径复用异步任务队列）。
func (s *orderService) ActivateByNo(ctx context.Context, orderNo string) error {
	item, err := s.orderRepo.FindByNo(ctx, orderNo)
	if err != nil {
		return mapOrderErr(err)
	}
	return s.EnqueueProvision(ctx, item.ID)
}

// remarkWithChannelTx 把渠道交易号追加到订单备注（保留人工备注，幂等去重）。
func remarkWithChannelTx(remark, channelTx string) string {
	channelTx = strings.TrimSpace(channelTx)
	if channelTx == "" || strings.Contains(remark, channelTx) {
		return remark
	}
	tag := "渠道流水号:" + channelTx
	if remark == "" {
		return tag
	}
	return remark + " | " + tag
}

// EnqueueProvision 投递异步开通任务（T5.1）。
//
// 未注入队列时回落为同步 Activate：人工重试接口（/:id/activate）无论哪种装配都可用。
// 队列路径下订单状态由工作池推进（paid→provisioning→active），本方法只负责投递。
//
// 续费订单（renewal_id != 0）不投递开通任务：它的商品就是被续费实例的售卖商品，
// 按商品开通会再创建一台新实例（重复扣资源），资源侧由 lifecycle 的续费完成链路负责
// （CompleteRenewalByOrderID → 上游续费 + 顺延到期时间）。
func (s *orderService) EnqueueProvision(ctx context.Context, id uint64) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	if item.RenewalID != 0 {
		return nil
	}
	if s.provisionQueue == nil {
		return s.Activate(ctx, id)
	}
	// source_mode 判据（D6）：按订单绑定的商品取链路；读取失败不影响投递（仅留痕）。
	sourceMode := ""
	if s.productSourceMode != nil {
		sourceMode = s.productSourceMode(ctx, item.ProductID)
	}
	if err := s.provisionQueue.EnqueueProvision(ctx, item, sourceMode); err != nil {
		return err
	}
	return nil
}

func (s *orderService) List(ctx context.Context, q dto.OrderListQuery) (*dto.OrderListResponse, error) {
	items, total, err := s.orderRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	respItems := make([]dto.OrderInfo, 0, len(items))
	tasks := s.provisionTaskMap(ctx, items)
	refs := s.instanceRefMap(ctx, items)
	verifyRequired := s.verifyRequiredNow(ctx)
	for _, item := range items {
		info := buildOrderInfo(item)
		if task := tasks[item.ID]; task != nil {
			info.ProvisionStatus = task.Status
			info.ProvisionError = task.LastError
		}
		applyInstanceRef(&info, refs[item.ID])
		applyVerifyState(&info, verifyRequired)
		respItems = append(respItems, info)
	}
	return &dto.OrderListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

// instanceRefMap 批量读取订单关联实例（未注入读取器或读取失败返回空表：
// 关联实例只是列表的附加信息，不能因为读不到就让订单列表整体失败）。
func (s *orderService) instanceRefMap(ctx context.Context, items []model.Order) map[uint64]OrderInstanceRef {
	if s.instanceRefs == nil || len(items) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	refs, err := s.instanceRefs(ctx, ids)
	if err != nil {
		return nil
	}
	return refs
}

// verifyRequiredNow 当前是否要求人工核验（未注入判据时视为不要求）。
func (s *orderService) verifyRequiredNow(ctx context.Context) bool {
	if s.verifyRequired == nil {
		return false
	}
	return s.verifyRequired(ctx)
}

// applyInstanceRef 回填关联实例字段。
func applyInstanceRef(info *dto.OrderInfo, ref OrderInstanceRef) {
	if ref.ID == 0 {
		return
	}
	info.InstanceID = ref.ID
	info.InstanceNo = ref.No
	info.InstancePending = ref.Pending
}

// applyVerifyState 回填核验字段并推导「待核验」。
//
// 推导口径：要求核验 + 服务中（active）+ 未核验 → pending。
// 只在读取期推导，不落库、不改状态机：核验不阻断任何流转（详见 model.Order 注释）。
func applyVerifyState(info *dto.OrderInfo, verifyRequired bool) {
	info.VerifyRequired = verifyRequired
	if info.VerifiedAt != "" {
		info.VerifyState = dto.VerifyStateVerified
		return
	}
	if verifyRequired && info.Status == model.OrderStatusActive {
		info.VerifyState = dto.VerifyStatePending
	}
}

// provisionTaskMap 批量读取订单的开通任务（未注入读取器或读取失败返回空表：
// 履约状态只是列表的附加信息，不能因为读不到就让订单列表整体失败）。
func (s *orderService) provisionTaskMap(ctx context.Context, items []model.Order) map[uint64]*model.ProvisionTask {
	if s.provisionTasks == nil || len(items) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	tasks, err := s.provisionTasks(ctx, ids)
	if err != nil {
		return nil
	}
	return tasks
}

func (s *orderService) FindByID(ctx context.Context, id uint64) (*dto.OrderDetail, error) {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	orderItems, err := s.itemRepo.ListByOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	refunds, err := s.refundRepo.FindByOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	itemInfos := make([]dto.OrderItemInfo, 0, len(orderItems))
	for _, it := range orderItems {
		itemInfos = append(itemInfos, buildOrderItemInfo(it))
	}
	refundInfos := make([]dto.RefundInfo, 0, len(refunds))
	for _, rf := range refunds {
		refundInfos = append(refundInfos, buildRefundInfo(rf, item.OrderNo))
	}
	info := buildOrderInfo(*item)
	// 履约任务状态（failed/manual + 失败原因）：订单详情是运营排查"为什么没开通"的入口。
	if tasks := s.provisionTaskMap(ctx, []model.Order{*item}); tasks != nil {
		if task := tasks[item.ID]; task != nil {
			info.ProvisionStatus = task.Status
			info.ProvisionError = task.LastError
			info.ProvisionAttempts = task.Attempts
		}
	}
	// 关联实例（跳转实例详情）与核验状态。
	if refs := s.instanceRefMap(ctx, []model.Order{*item}); refs != nil {
		applyInstanceRef(&info, refs[item.ID])
	}
	applyVerifyState(&info, s.verifyRequiredNow(ctx))
	return &dto.OrderDetail{OrderInfo: info, Items: itemInfos, Refunds: refundInfos}, nil
}

// Cancel 取消订单：仅待支付可取消，校验状态机并留痕。
func (s *orderService) Cancel(ctx context.Context, id uint64, operatorID uint64, operatorName string) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	if err := EnsureStatus(item.Status, model.OrderStatusCancelled); err != nil {
		return err
	}
	item.Status = model.OrderStatusCancelled
	item.OperatorID = operatorID
	if err := s.orderRepo.Update(ctx, item); err != nil {
		return err
	}
	// 后台取消待支付订单：同时删除其「待开通」占位实例，避免实例列表里留下
	// 一台永远不会开通的机器。删除失败不回滚取消（订单已作废是主结果），只记日志。
	if s.pendingInstanceReleaser != nil {
		if _, rerr := s.pendingInstanceReleaser(ctx, item.ID); rerr != nil && s.logger != nil {
			s.logger.Warn("release pending instance after cancel failed",
				zap.Uint64("order_id", item.ID), zap.Error(rerr))
		}
	}
	// 回补库存：代下单同样占用了库存（SKU 或商品级），不回补会越卖越少。
	// 失败只记日志：订单已作废是主结果，库存准确性由运维按订单核对。
	if s.stockReleaser != nil {
		if rerr := s.stockReleaser(context.WithoutCancel(ctx), item); rerr != nil && s.logger != nil {
			s.logger.Warn("release stock after cancel failed",
				zap.Uint64("order_id", item.ID), zap.Error(rerr))
		}
	}
	return nil
}

func (s *orderService) UpdateRemark(ctx context.Context, id uint64, remark string, operatorID uint64) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	item.Remark = remark
	item.OperatorID = operatorID
	return s.orderRepo.Update(ctx, item)
}

// CreateRefund 管理端发起退款：校验可退金额，创建待审核退款单，服务中订单转入退款中。
//
// 运营保留完全裁量权：管理端发起**不受退款策略（时限/来源）限制**，
// 策略只约束客户自助申请（见 UserCreateRefund）。
func (s *orderService) CreateRefund(ctx context.Context, id uint64, req dto.RefundCreateRequest, operatorID uint64, operatorName string) (*dto.RefundInfo, error) {
	return s.createRefund(ctx, id, req, operatorID, operatorName, model.RefundOriginAdmin)
}

// createRefund 退款单创建（管理端与用户自助共用）。
//
// 与调用方的分工：本函数只做「订单状态 + 金额 + 去向」校验与落单，
// 策略/时限/免审判定由 UserCreateRefund 前置完成。
func (s *orderService) createRefund(ctx context.Context, id uint64, req dto.RefundCreateRequest, operatorID uint64, operatorName, origin string) (*dto.RefundInfo, error) {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	switch item.Status {
	case model.OrderStatusPaid, model.OrderStatusProvisioning, model.OrderStatusActive, model.OrderStatusRefunding:
	default:
		return nil, ErrStatusConflict
	}
	if req.Amount <= 0 {
		return nil, ErrRefundExceeded
	}
	effective, err := s.refundRepo.SumEffective(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Amount+effective > item.PaidAmount {
		return nil, ErrRefundExceeded
	}
	// 退款去向（doc36 §3.2）：默认退回余额；原路退回时扣点为渠道不退还的手续费。
	refundMode := strings.TrimSpace(req.RefundMode)
	if refundMode == "" {
		refundMode = model.RefundModeBalance
	}
	if refundMode != model.RefundModeBalance && refundMode != model.RefundModeChannel {
		return nil, ErrRefundModeInvalid
	}
	fee := 0.0
	if refundMode == model.RefundModeChannel {
		if req.FeeAmount < 0 || req.FeeAmount > req.Amount {
			return nil, ErrRefundFeeInvalid
		}
		fee = round2(req.FeeAmount)
	}

	refund := &model.OrderRefund{
		RefundNo:   genRefundNo(),
		OrderID:    item.ID,
		UserID:     item.UserID,
		Amount:     req.Amount,
		Reason:     req.Reason,
		Status:     model.RefundStatusPending,
		RefundMode: refundMode,
		FeeAmount:  fee,
		NetAmount:  round2(req.Amount + fee),
		Origin:     origin,
	}
	if err := s.refundRepo.Create(ctx, refund); err != nil {
		return nil, err
	}

	// 服务中订单转入退款中（状态机校验：active → refunding）
	if item.Status == model.OrderStatusActive {
		if err := EnsureStatus(item.Status, model.OrderStatusRefunding); err != nil {
			return nil, err
		}
		item.Status = model.OrderStatusRefunding
		item.OperatorID = operatorID
		if err := s.orderRepo.Update(ctx, item); err != nil {
			return nil, err
		}
	}
	return s.FindRefund(ctx, refund.ID)
}

// —— 客户自助退款（策略 + 分级审核）——

// RefundPolicyHint 计算订单当前的可退性提示。
//
// 判定要点：
//   - 未命中策略（含未配策略）→ 不可自助申请（保守默认）；
//   - 已开通的订单按「开通成功时间 + 策略时限」判是否过期；未开通（paid/provisioning）
//     不受时限约束（尚未交付，客户可随时申请），但仍需人工审核；
//   - AutoApprove 只在「退回余额 + 全额」时才可能为 true（见 UserCreateRefund）。
func (s *orderService) RefundPolicyHint(ctx context.Context, order *model.Order) *dto.OrderRefundPolicyHint {
	if order == nil {
		return nil
	}
	effective, err := s.refundRepo.SumEffective(ctx, order.ID)
	if err != nil {
		effective = 0
	}
	maxAmount := round2(order.PaidAmount - effective)
	hint := &dto.OrderRefundPolicyHint{
		MaxAmount:      maxAmount,
		ChannelAllowed: s.hasChannelPayment(ctx, order.OrderNo),
	}
	// 订单状态本身不可退时直接说明（终态/未支付）。
	switch order.Status {
	case model.OrderStatusPaid, model.OrderStatusProvisioning, model.OrderStatusActive, model.OrderStatusRefunding:
	default:
		hint.Reason = "当前订单状态不可申请退款"
		return hint
	}
	if maxAmount <= 0 {
		hint.Reason = "该订单已无可退金额"
		return hint
	}
	decision, derr := s.resolveRefundPolicy(ctx, order.ProductID)
	if derr != nil || decision == nil || !decision.Refundable {
		hint.Reason = "该商品不支持自助申请退款，请联系客服"
		return hint
	}
	hint.WindowHours = decision.WindowHours
	hint.AutoApprove = decision.AutoApprove
	// 时限只对「已开通」的订单生效；未开通订单不设截止时间。
	if order.ActivatedAt != nil && decision.WindowHours > 0 {
		deadline := order.ActivatedAt.Add(time.Duration(decision.WindowHours) * time.Hour)
		hint.Deadline = deadline.Format(time.RFC3339)
		if time.Now().After(deadline) {
			hint.Reason = fmt.Sprintf("已超过 %d 小时可退时限", decision.WindowHours)
			return hint
		}
	}
	hint.Refundable = true
	return hint
}

// resolveRefundPolicy 解析订单商品的生效策略（未注入匹配器时返回 nil）。
func (s *orderService) resolveRefundPolicy(ctx context.Context, productID uint64) (*RefundPolicyDecision, error) {
	if s.refundPolicy == nil {
		return nil, nil
	}
	return s.refundPolicy.Resolve(ctx, productID)
}

// hasChannelPayment 订单是否有渠道支付记录（nil 判据时保守返回 false）。
func (s *orderService) hasChannelPayment(ctx context.Context, orderNo string) bool {
	if s.channelPaymentChecker == nil {
		return false
	}
	return s.channelPaymentChecker(ctx, orderNo)
}

// UserCreateRefund 客户自助申请退款。
//
// 分流规则（与运营确认的口径）：
//  1. 归属与状态校验 → 可退上限校验；
//  2. 去向校验：只有存在渠道支付记录的订单才能选「原路退回」，余额支付订单强制「退回余额」；
//  3. 时限校验：策略按「商品来源 + 分类」匹配；已开通订单须在 window 内
//     （未开通订单不受时限约束 —— 尚未交付）；未命中策略或 strategy.Refundable=false 一律拒绝；
//  4. 分级：
//     · 全额 + 退回余额 + 策略 auto_approve + 在时限内 → **免审核自动退款**，
//     走完整审核链路（余额回补/返现冲减/实例按生命周期策略处置），
//     并置 review_state=pending 留一道事后复核；
//     · 其余全部（原路退回 / 部分金额 / 免审关闭 / 超时）→ 落待审核单等人工处理。
//
// 重要：原路退回永远不自动退 —— 目前没有渠道自动退款接口，钱要人工打出去，
// 必须有人在这一步核对并上传凭据（evidence_no + 附件）。
func (s *orderService) UserCreateRefund(ctx context.Context, userID, orderID uint64, req dto.UserRefundRequest) (*dto.UserRefundResult, error) {
	item, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	// 归属校验：他人订单一律按「不存在」处理，不泄漏订单存在性。
	if item.UserID != userID {
		return nil, ErrOrderNotFound
	}
	hint := s.RefundPolicyHint(ctx, item)
	if hint == nil || !hint.Refundable {
		reason := "该订单当前不可申请退款"
		if hint != nil && hint.Reason != "" {
			reason = hint.Reason
		}
		return nil, fmt.Errorf("%w：%s", ErrRefundNotAllowed, reason)
	}

	// 去向：默认退回余额；选原路退回必须先有渠道支付记录。
	refundMode := strings.TrimSpace(req.RefundMode)
	if refundMode == "" {
		refundMode = model.RefundModeBalance
	}
	if refundMode != model.RefundModeBalance && refundMode != model.RefundModeChannel {
		return nil, ErrRefundModeInvalid
	}
	if refundMode == model.RefundModeChannel && !hint.ChannelAllowed {
		return nil, ErrRefundChannelUnavailable
	}

	// 金额：自助申请固定按可退上限全额（部分退款属补偿，由运营在管理端发起）。
	amount := hint.MaxAmount
	if amount <= 0 {
		return nil, ErrRefundExceeded
	}

	// 免审资格在落单**之前**判定：createRefund 会把服务中订单推进为退款中，
	// 落单后再读状态已经不是下判断时的样子了。
	// 仅「全额 + 退回余额 + 策略允许 + 已开通」才自动退；未开通订单虽然可申请，
	// 但一律走人工审核（可能涉及上游退单等线下动作）。
	autoEligible := refundMode == model.RefundModeBalance &&
		hint.AutoApprove &&
		item.Status == model.OrderStatusActive &&
		item.ActivatedAt != nil

	createReq := dto.RefundCreateRequest{
		Amount:     amount,
		Reason:     strings.TrimSpace(req.Reason),
		RefundMode: refundMode,
	}
	info, err := s.createRefund(ctx, orderID, createReq, userID, "", model.RefundOriginUser)
	if err != nil {
		return nil, err
	}

	if !autoEligible {
		return &dto.UserRefundResult{
			RefundNo: info.RefundNo,
			Amount:   amount,
			Status:   model.RefundStatusPending,
			Hint:     "退款申请已提交，等待人工审核",
		}, nil
	}

	// 自动退款：复用管理端审核链路（资金出口 + 返现冲减 + 实例处置钩子全部生效），
	// 之后把退款单置为「待复核」——自动退出去的钱必须有人回头核对。
	approved, err := s.ApproveRefund(ctx, info.ID, 0, "系统自动退款",
		dto.RefundApproveRequest{Remark: "命中免审核策略，系统自动通过"})
	if err != nil {
		// 自动审核失败不吞掉申请：退款单已落库为 pending，转人工处理更安全。
		if s.logger != nil {
			s.logger.Warn("auto approve refund failed, fallback to manual",
				zap.Uint64("refund_id", info.ID), zap.Uint64("order_id", orderID), zap.Error(err))
		}
		return &dto.UserRefundResult{
			RefundNo: info.RefundNo,
			Amount:   amount,
			Status:   model.RefundStatusPending,
			Hint:     "退款申请已提交，等待人工审核",
		}, nil
	}
	if err := s.markRefundPendingReview(ctx, info.ID); err != nil && s.logger != nil {
		s.logger.Warn("mark refund pending review failed", zap.Uint64("refund_id", info.ID), zap.Error(err))
	}
	return &dto.UserRefundResult{
		RefundNo:     approved.RefundNo,
		AutoApproved: true,
		Amount:       amount,
		Status:       approved.Status,
		Hint:         "已自动退款至余额，财务将尽快复核",
	}, nil
}

// markRefundPendingReview 把自动退款单标记为「待复核」。
func (s *orderService) markRefundPendingReview(ctx context.Context, refundID uint64) error {
	refund, err := s.refundRepo.FindByID(ctx, refundID)
	if err != nil {
		return err
	}
	refund.ReviewState = model.RefundReviewPending
	return s.refundRepo.Update(ctx, refund)
}

// ReviewRefund 事后复核（免审核自动退款的最后一道人工确认）。
//
// 幂等：仅 review_state=pending 的单可复核，重复复核返回状态冲突。
// 已驳回/未通过的退款单不允许复核（那是审核环节的事）。
func (s *orderService) ReviewRefund(ctx context.Context, id uint64, operatorID uint64, operatorName, remark string) (*dto.RefundInfo, error) {
	refund, err := s.refundRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRefundErr(err)
	}
	if refund.ReviewState != model.RefundReviewPending {
		return nil, ErrStatusConflict
	}
	now := time.Now()
	refund.ReviewState = model.RefundReviewDone
	refund.ReviewRemark = strings.TrimSpace(remark)
	refund.ReviewedBy = operatorID
	refund.ReviewedByName = operatorName
	refund.ReviewedAt = &now
	if err := s.refundRepo.Update(ctx, refund); err != nil {
		return nil, err
	}
	return s.FindRefund(ctx, id)
}

// Activate 重新触发开通实例（幂等）。
func (s *orderService) Activate(ctx context.Context, id uint64) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	if err := s.provision.Activate(ctx, item); err != nil {
		return err
	}
	if err := s.orderRepo.Update(ctx, item); err != nil {
		return err
	}
	// 订单开通成功后的推广返现计提：失败不回滚订单，钩子实现负责记录错误。
	if s.cashbackHook != nil {
		_ = s.cashbackHook(ctx, item.ID, item.OrderNo, item.UserID, item.PaidAmount, item.RenewalID != 0)
	}
	return nil
}

func (s *orderService) Stats(ctx context.Context) (*dto.OrderStatsResponse, error) {
	todayOrders, err := s.orderRepo.StatsCountToday(ctx)
	if err != nil {
		return nil, err
	}
	todaySales, err := s.orderRepo.StatsSalesToday(ctx)
	if err != nil {
		return nil, err
	}
	total, sales, err := s.orderRepo.StatsTotal(ctx)
	if err != nil {
		return nil, err
	}
	refundAmount, err := s.refundRepo.SumRefundedAmount(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.orderRepo.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}
	trendPoints, err := s.orderRepo.StatsTrend(ctx, 7)
	if err != nil {
		return nil, err
	}

	avgOrderValue := 0.0
	if total > 0 {
		avgOrderValue = round2(sales / float64(total))
	}
	refundRate := 0.0
	if sales > 0 {
		refundRate = round2(refundAmount / sales * 100)
	}
	distribution := make([]dto.OrderStatusStat, 0, len(counts))
	for _, c := range counts {
		distribution = append(distribution, dto.OrderStatusStat{Status: c.Status, Count: c.Count})
	}

	return &dto.OrderStatsResponse{
		TodaySales:         round2(todaySales),
		TodayOrders:        todayOrders,
		AvgOrderValue:      avgOrderValue,
		RefundRate:         refundRate,
		StatusDistribution: distribution,
		Trend:              fillTrend(trendPoints, 7),
	}, nil
}

func (s *orderService) ListRefunds(ctx context.Context, q dto.RefundListQuery) (*dto.RefundListResponse, error) {
	items, total, err := s.refundRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	return s.refundListResponse(ctx, items, total, page, pageSize)
}

func (s *orderService) FindRefund(ctx context.Context, id uint64) (*dto.RefundInfo, error) {
	refund, err := s.refundRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRefundErr(err)
	}
	orderNo, err := s.orderNoByID(ctx, refund.OrderID)
	if err != nil {
		return nil, err
	}
	info := buildRefundInfo(*refund, orderNo)
	// 凭据附件（详情接口才填；列表接口为 N+1 风险不填）。
	if s.refundAttachments != nil {
		if list, aerr := s.refundAttachments.ListByRefund(ctx, refund.ID); aerr == nil {
			info.Attachments = list
		}
	}
	return &info, nil
}

// ApproveRefund 审核通过：通过后若覆盖全部实付金额则回写订单为已退款。
func (s *orderService) ApproveRefund(ctx context.Context, id uint64, operatorID uint64, operatorName string, req dto.RefundApproveRequest) (*dto.RefundInfo, error) {
	remark := req.Remark
	refund, err := s.refundRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRefundErr(err)
	}
	if refund.Status != model.RefundStatusPending {
		return nil, ErrStatusConflict
	}
	refund.Status = model.RefundStatusApproved
	refund.AuditBy = operatorID
	refund.AuditByName = operatorName
	now := time.Now()
	refund.AuditedAt = &now
	// 审核依据与凭据：此前 remark 收了参数却从未落库，导致"为什么退的钱"无据可查。
	if trimmed := strings.TrimSpace(remark); trimmed != "" {
		refund.AuditRemark = trimmed
	}
	// 人工退款凭据外部单号（转账/渠道流水号）：原路退回没有自动接口，
	// 这笔钱是人工打出去的，必须留下可定位的凭证号。
	if ev := strings.TrimSpace(req.EvidenceNo); ev != "" {
		refund.EvidenceNo = ev
	}
	if err := s.refundRepo.Update(ctx, refund); err != nil {
		return nil, err
	}

	if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil {
		effective, perr := s.refundRepo.SumEffective(ctx, order.ID)
		if perr == nil && effective >= order.PaidAmount {
			// 全额退款：paid / provisioning / refunding → refunded
			switch order.Status {
			case model.OrderStatusPaid, model.OrderStatusProvisioning, model.OrderStatusRefunding:
				if err := EnsureStatus(order.Status, model.OrderStatusRefunded); err == nil {
					order.Status = model.OrderStatusRefunded
					order.OperatorID = operatorID
					_ = s.orderRepo.Update(ctx, order)
				}
			}
		} else if perr == nil && order.Status == model.OrderStatusRefunding {
			// 部分退款：服务中订单回退为服务中
			order.Status = model.OrderStatusActive
			order.OperatorID = operatorID
			_ = s.orderRepo.Update(ctx, order)
		}
	}
	// 退款审核通过 → 按退款额占比冲减邀请人已计提返现（失败不回滚退款，钩子实现负责记录错误）。
	if s.refundHook != nil {
		if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil {
			_ = s.refundHook(ctx, order.ID, order.OrderNo, order.UserID, order.PaidAmount, refund.Amount, refund.RefundNo)
		}
	}
	// 退款审核通过 → 按去向执行资金出口（doc36 §3.2）。
	// balance：直接退回余额，消费口径不变；channel：原路退回，收入口径扣减本金与扣点。
	// 失败不回滚审核结果：支付中心留存退款单与失败原因，由财务人工重试或线下处理。
	if s.channelRefundHook != nil {
		if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil {
			channelNo, outcome, herr := s.channelRefundHook(ctx, order.ID, order.OrderNo, refund.RefundNo,
				order.UserID, refund.Amount, refund.FeeAmount, refund.RefundMode)
			refund.ChannelRefundNo = channelNo
			switch {
			case herr == nil && outcome == RefundOutcomeChannel && channelNo != "":
				// 渠道退款单已建：资金将按渠道异步流出，账单侧据此扣减口径。
				refund.ChannelRefundStatus = model.ChannelRefundStatusPending
			case herr == nil && outcome == RefundOutcomeManual:
				// 场外收款的原路退回：没有渠道接口可调，钱由财务线下汇回，不进余额。
				// 标 manual 而非 failed —— 账单口径按「资金将流出」扣减本金，
				// 且运营在退款详情能看到"待人工汇款"，不会以为已经退完了。
				refund.ChannelRefundStatus = model.ChannelRefundStatusManual
			case refund.RefundMode == model.RefundModeChannel:
				// 意图原路退回但未能建渠道退款单（无渠道支付记录 / 渠道不支持 / 渠道缺失）：
				// 钩子实现已回落余额回补，资金并未流出平台，标记 failed。
				// 账单口径只统计 pending/success/manual，故不会误扣本金与扣点。
				refund.ChannelRefundStatus = model.ChannelRefundStatusFailed
			}
			_ = s.refundRepo.Update(ctx, refund)
		}
	}
	// 退款审核通过 → 按策略处置关联实例（doc61 §8 第 2 条）。
	// 放在资金出口之后：先保证钱已退回，再谈机器；钩子实现按 lifecycle_policies.refund_action
	// 决定是否暂停/销毁，默认 none（不动作）。失败不回滚退款审核结果（人工跟进）。
	// fullRefund 由本次退款是否覆盖全部实付推算，只有全额退款才谈得上处置实例。
	if s.instanceRefundHook != nil {
		if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil {
			effective, perr := s.refundRepo.SumEffective(ctx, order.ID)
			fullRefund := perr == nil && order.PaidAmount > 0 && effective >= order.PaidAmount
			// 与既有 refundHook 同一约定：失败只记日志（钩子实现持有 logger），不回滚退款审核。
			_ = s.instanceRefundHook(ctx, order.ID, order.OrderNo, order.UserID,
				refund.Amount, order.PaidAmount, fullRefund, refund.RefundNo)
		}
	}
	return s.FindRefund(ctx, refund.ID)
}

// RejectRefund 驳回退款：若订单无其他待审核退款则回退为服务中。
func (s *orderService) RejectRefund(ctx context.Context, id uint64, operatorID uint64, operatorName string, req dto.RefundApproveRequest) (*dto.RefundInfo, error) {
	remark := req.Remark
	refund, err := s.refundRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRefundErr(err)
	}
	if refund.Status != model.RefundStatusPending {
		return nil, ErrStatusConflict
	}
	refund.Status = model.RefundStatusRejected
	refund.AuditBy = operatorID
	refund.AuditByName = operatorName
	now := time.Now()
	refund.AuditedAt = &now
	if trimmed := strings.TrimSpace(remark); trimmed != "" {
		refund.AuditRemark = trimmed
	}
	if err := s.refundRepo.Update(ctx, refund); err != nil {
		return nil, err
	}

	if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil && order.Status == model.OrderStatusRefunding {
		remains, rerr := s.refundRepo.FindByOrder(ctx, order.ID)
		hasInFlight := false
		for _, r := range remains {
			if r.Status == model.RefundStatusPending || r.Status == model.RefundStatusApproved {
				hasInFlight = true
				break
			}
		}
		if rerr == nil && !hasInFlight {
			order.Status = model.OrderStatusActive
			order.OperatorID = operatorID
			_ = s.orderRepo.Update(ctx, order)
		}
	}
	return s.FindRefund(ctx, refund.ID)
}

// —— 核验（运营人工核对开通结果：仅标记，不流转状态）——

// Verify 核验订单：CAS 写入核验人/时间/备注。
//
// 幂等语义：已核验的订单再次核验返回 ErrStatusConflict（与"只有未核验才能核验"一致，
// 批量场景下由调用方按 skipped 处理）。已作废/未开通的订单不允许核验：
// 核验的对象是"这台机器我看过了"，没开通的订单没有可核验的实体。
func (s *orderService) Verify(ctx context.Context, id uint64, operatorID uint64, operatorName string, remark string) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	switch item.Status {
	case model.OrderStatusActive, model.OrderStatusRefunding, model.OrderStatusCompleted:
	default:
		return ErrVerifyNotAllowed
	}
	if item.VerifiedAt != nil {
		return ErrStatusConflict
	}
	rows, err := s.orderRepo.MarkVerified(ctx, id, operatorID, operatorName, strings.TrimSpace(remark), time.Now())
	if err != nil {
		return err
	}
	if rows == 0 {
		// CAS 未命中：并发核验时另一方已写入。
		return ErrStatusConflict
	}
	return nil
}

// —— 批量操作（逐条复用单条逻辑，含状态守卫与副作用；返回逐条明细）——

// maxBatchIDs 单次批量上限：与用户批量注销同口径（100），防止一次请求打爆上游。
const maxBatchIDs = 100

// normalizeBatchIDs 去重并保序，超过上限直接拒绝。
func normalizeBatchIDs(ids []uint64) ([]uint64, error) {
	if len(ids) == 0 {
		return nil, ErrBatchEmpty
	}
	if len(ids) > maxBatchIDs {
		return nil, ErrBatchTooLarge
	}
	seen := make(map[uint64]bool, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, ErrBatchEmpty
	}
	return out, nil
}

// batchRun 逐条执行并把错误分类成 success/skipped/failed。
//
// 分类口径（与实例批量动作一致）：skipCheck 判定可预期的不允许（状态不符），
// 记 skipped；其余错误记 failed。整批不因单条失败而中止 —— 40 个订单里有
// 一个已支付，不该让另外 39 个待支付的一起不能取消。
func batchRun(ids []uint64, skipCheck func(error) bool, run func(id uint64) error) *dto.BatchActionResponse {
	resp := &dto.BatchActionResponse{Items: make([]dto.BatchItemResult, 0, len(ids))}
	for _, id := range ids {
		err := run(id)
		item := dto.BatchItemResult{ID: id, Status: dto.BatchStatusSuccess}
		switch {
		case err == nil:
			resp.Succeeded++
		case skipCheck(err):
			item.Status = dto.BatchStatusSkipped
			item.Message = err.Error()
			resp.Skipped++
		default:
			item.Status = dto.BatchStatusFailed
			item.Message = err.Error()
			resp.Failed++
		}
		resp.Items = append(resp.Items, item)
	}
	return resp
}

// BatchCancel 批量取消：仅待支付订单可取消，逐条走完整取消链
// （CAS 状态迁移 + 关支付单 + 删占位实例 + 回补库存）。
func (s *orderService) BatchCancel(ctx context.Context, ids []uint64, operatorID uint64, operatorName string) (*dto.BatchActionResponse, error) {
	list, err := normalizeBatchIDs(ids)
	if err != nil {
		return nil, err
	}
	return batchRun(list, isSkipErr, func(id uint64) error {
		return s.Cancel(ctx, id, operatorID, operatorName)
	}), nil
}

// BatchActivate 批量重新开通：仅已支付/开通中的订单，复用幂等开通队列
// （重试复用已生成的上游账单，不会重复下单，doc119）。
func (s *orderService) BatchActivate(ctx context.Context, ids []uint64) (*dto.BatchActionResponse, error) {
	list, err := normalizeBatchIDs(ids)
	if err != nil {
		return nil, err
	}
	return batchRun(list, isSkipErr, func(id uint64) error {
		return s.EnqueueProvision(ctx, id)
	}), nil
}

// BatchVerify 批量核验：逐条 CAS。
func (s *orderService) BatchVerify(ctx context.Context, ids []uint64, operatorID uint64, operatorName string, remark string) (*dto.BatchActionResponse, error) {
	list, err := normalizeBatchIDs(ids)
	if err != nil {
		return nil, err
	}
	return batchRun(list, isSkipErr, func(id uint64) error {
		return s.Verify(ctx, id, operatorID, operatorName, remark)
	}), nil
}

// deletableOrderStatuses 允许软删除的订单状态：只有「已取消 / 已关闭」的作废单可删。
// 已支付/服务中/退款中的订单是活着的资金凭证，任何情况下都不允许从列表抹掉。
var deletableOrderStatuses = []string{model.OrderStatusCancelled, model.OrderStatusClosed}

// BatchDelete 批量软删除：仅「已取消/已关闭」且无任何退款记录的订单。
//
// 订单是财务凭证，删除只做软删除（deleted_at），行与金额始终留在库里可审计；
// 走过退款流程的订单（哪怕退款被驳回）保留在列表 —— 那是资金纠纷的线索。
func (s *orderService) BatchDelete(ctx context.Context, ids []uint64) (*dto.BatchActionResponse, error) {
	list, err := normalizeBatchIDs(ids)
	if err != nil {
		return nil, err
	}
	return batchRun(list, isSkipErr, func(id uint64) error {
		item, err := s.orderRepo.FindByID(ctx, id)
		if err != nil {
			return mapOrderErr(err)
		}
		switch item.Status {
		case model.OrderStatusCancelled, model.OrderStatusClosed:
		default:
			return fmt.Errorf("%w：仅已取消/已关闭的订单可删除", ErrStatusConflict)
		}
		has, err := s.refundRepo.ExistsByOrder(ctx, id)
		if err != nil {
			return err
		}
		if has {
			return ErrOrderHasRefund
		}
		rows, err := s.orderRepo.SoftDelete(ctx, id, deletableOrderStatuses, time.Now())
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrStatusConflict
		}
		return nil
	}), nil
}

// BatchApproveRefunds 批量审核通过退款单：逐条复用 ApproveRefund
// （含渠道退款/余额回补/实例处置/返现冲减钩子，不得绕过）。
func (s *orderService) BatchApproveRefunds(ctx context.Context, ids []uint64, operatorID uint64, operatorName string, remark string) (*dto.BatchActionResponse, error) {
	return s.batchRefundAudit(ctx, ids, operatorID, operatorName, remark, true)
}

// BatchRejectRefunds 批量驳回退款单。
func (s *orderService) BatchRejectRefunds(ctx context.Context, ids []uint64, operatorID uint64, operatorName string, remark string) (*dto.BatchActionResponse, error) {
	return s.batchRefundAudit(ctx, ids, operatorID, operatorName, remark, false)
}

func (s *orderService) batchRefundAudit(ctx context.Context, ids []uint64, operatorID uint64, operatorName, remark string, approve bool) (*dto.BatchActionResponse, error) {
	list, err := normalizeBatchIDs(ids)
	if err != nil {
		return nil, err
	}
	return batchRun(list, isSkipErr, func(id uint64) error {
		var err error
		req := dto.RefundApproveRequest{Remark: remark}
		if approve {
			_, err = s.ApproveRefund(ctx, id, operatorID, operatorName, req)
		} else {
			_, err = s.RejectRefund(ctx, id, operatorID, operatorName, req)
		}
		return err
	}), nil
}

// isSkipErr 判定「可预期的跳过」：状态不符/不存在/不允许核验等，属正常业务结果，
// 不是执行失败（批量场景下逐条回显原因，不算 failed）。
func isSkipErr(err error) bool {
	switch {
	case errors.Is(err, ErrStatusConflict),
		errors.Is(err, ErrOrderNotFound),
		errors.Is(err, ErrRefundNotFound),
		errors.Is(err, ErrVerifyNotAllowed),
		errors.Is(err, ErrOrderHasRefund),
		errors.Is(err, ErrRefundExceeded),
		errors.Is(err, ErrRefundModeInvalid),
		errors.Is(err, ErrRefundFeeInvalid),
		errors.Is(err, ErrRefundNotAllowed),
		errors.Is(err, ErrRefundChannelUnavailable):
		return true
	}
	return false
}

func (s *orderService) refundListResponse(ctx context.Context, items []model.OrderRefund, total int64, page, pageSize int) (*dto.RefundListResponse, error) {
	respItems := make([]dto.RefundInfo, 0, len(items))
	orderNos := make(map[uint64]string, len(items))
	var missing []uint64
	for _, rf := range items {
		if _, ok := orderNos[rf.OrderID]; !ok {
			missing = append(missing, rf.OrderID)
		}
	}
	if len(missing) > 0 {
		if orders, err := s.orderRepo.FindByIDs(ctx, missing); err == nil {
			for _, o := range orders {
				orderNos[o.ID] = o.OrderNo
			}
		}
	}
	for _, rf := range items {
		respItems = append(respItems, buildRefundInfo(rf, orderNos[rf.OrderID]))
	}
	return &dto.RefundListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *orderService) orderNoByID(ctx context.Context, orderID uint64) (string, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return "", mapOrderErr(err)
	}
	return order.OrderNo, nil
}

// —— 构建器 ——

func buildOrderInfo(item model.Order) dto.OrderInfo {
	return dto.OrderInfo{
		ID:          item.ID,
		OrderNo:     item.OrderNo,
		UserID:      item.UserID,
		ProductID:   item.ProductID,
		ProductName: item.ProductName,
		Specs:       item.Specs,
		Quantity:    item.Quantity,
		PriceModel:  item.PriceModel,
		TotalAmount: item.TotalAmount,
		PaidAmount:  item.PaidAmount,
		Status:      item.Status,
		PayMethod:   item.PayMethod,
		PayTime:     formatTime(item.PayTime),
		ExpireTime:  formatTime(item.ExpireTime),
		Remark:      item.Remark,
		// 计费周期与续费标记（doc36 §3.4）：续费订单以 renewal_id != 0 判定。
		Cycle:     item.Cycle,
		RenewalID: item.RenewalID,
		// 算价快照（P5-01/P5-04）：原价 / 优惠 / 实付与命中来源。
		OriginalAmount: item.OriginalAmount,
		DiscountAmount: item.DiscountAmount,
		FinalAmount:    item.FinalAmount,
		DiscountSource: item.DiscountSource,
		// 支付单号与渠道流水号（doc36 §3.1）：由仓储从支付中心反查填充，空即未走渠道。
		PaymentNo: item.PaymentNo,
		ChannelTx: item.ChannelTx,
		// 核验标记（落库部分）：VerifyState/VerifyRequired 由读取期按配置推导，见 applyVerifyState。
		VerifiedAt:     formatTime(item.VerifiedAt),
		VerifiedBy:     item.VerifiedBy,
		VerifiedByName: item.VerifiedByName,
		VerifyRemark:   item.VerifyRemark,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      item.UpdatedAt.Format(time.RFC3339),
	}
}

func buildOrderItemInfo(item model.OrderItem) dto.OrderItemInfo {
	return dto.OrderItemInfo{
		ID:          item.ID,
		OrderID:     item.OrderID,
		ProductID:   item.ProductID,
		ProductName: item.ProductName,
		SpecCode:    item.SpecCode,
		Specs:       item.Specs,
		Cycle:       item.Cycle,
		Price:       item.Price,
		Quantity:    item.Quantity,
		Amount:      item.Amount,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
	}
}

func buildRefundInfo(rf model.OrderRefund, orderNo string) dto.RefundInfo {
	return dto.RefundInfo{
		ID:          rf.ID,
		RefundNo:    rf.RefundNo,
		OrderID:     rf.OrderID,
		OrderNo:     orderNo,
		UserID:      rf.UserID,
		Amount:      rf.Amount,
		Reason:      rf.Reason,
		Status:      rf.Status,
		AuditBy:     rf.AuditBy,
		AuditByName: rf.AuditByName,
		AuditedAt:   formatTime(rf.AuditedAt),
		// 退款去向与扣点（doc36 §3.2）：空值视为余额退回（兼容迁移前存量数据）。
		RefundMode:          defaultRefundMode(rf.RefundMode),
		FeeAmount:           rf.FeeAmount,
		NetAmount:           rf.NetAmount,
		ChannelRefundNo:     rf.ChannelRefundNo,
		ChannelRefundStatus: rf.ChannelRefundStatus,
		// 申请来源与审核凭据（人工退款查账定位用）。
		Origin:      rf.Origin,
		AuditRemark: rf.AuditRemark,
		EvidenceNo:  rf.EvidenceNo,
		// 事后复核（免审核自动退款路径）。
		ReviewState:    rf.ReviewState,
		ReviewRemark:   rf.ReviewRemark,
		ReviewedBy:     rf.ReviewedBy,
		ReviewedByName: rf.ReviewedByName,
		ReviewedAt:     formatTime(rf.ReviewedAt),
		CreatedAt:      rf.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      rf.UpdatedAt.Format(time.RFC3339),
	}
}

// defaultRefundMode 兼容存量退款单：迁移前无 refund_mode 列，一律按余额退回口径。
func defaultRefundMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return model.RefundModeBalance
	}
	return mode
}

// —— 工具 ——

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func genRefundNo() string {
	return fmt.Sprintf("RF%s%03d", time.Now().Format("20060102150405"), rand.Intn(1000))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// payableAmount 订单应付金额：算价快照（final_amount）优先，兼容未写快照的存量订单。
func payableAmount(o *model.Order) float64 {
	if o.FinalAmount > 0 {
		return o.FinalAmount
	}
	return o.TotalAmount
}

// fillTrend 保证返回连续 N 日趋势点（缺失日期补零）。
func fillTrend(points []model.OrderTrendPoint, days int) []dto.OrderTrendPoint {
	byDate := make(map[string]model.OrderTrendPoint, len(points))
	for _, p := range points {
		byDate[p.Date] = p
	}
	result := make([]dto.OrderTrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		p, ok := byDate[d]
		if !ok {
			p = model.OrderTrendPoint{Date: d}
		}
		result = append(result, dto.OrderTrendPoint{Date: d, Sales: round2(p.Sales), Orders: p.Orders})
	}
	return result
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

func mapRefundErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRefundNotFound
	}
	return err
}
