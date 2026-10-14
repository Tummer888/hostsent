// Package service 提供用户中心订单模块业务编排：余额支付下单 → 触发上游开通。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	orderdto "hostsent/backend/internal/modules/admin/order/dto"
	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	paydto "hostsent/backend/internal/modules/admin/system/payment/dto"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/pricing"

	"hostsent/backend/internal/modules/uc/order/dto"
)

// 业务类型 / 资金流水类型常量（本模块的本地面）：
// 语义与 admin 域 model 同名列一致，落库值必须逐字相同。
const (
	// BizTypeOrder 订单支付（收银台 PrepayRequest.BizType）。
	BizTypeOrder = "order"
	// TxTypeConsume 消费流水类型（余额扣款 AdjustRequest.Type）。
	TxTypeConsume = "consume"
)

// defaultPayExpireMinutes 待支付订单默认有效期（分钟）：与系统配置 order_expire_minutes
// 的种子值保持一致；配置缺失或非法时的兜底。
//
// 默认一天：订单创建即生成「待开通」实例占位，占位随订单存续；有效期太短会让
// 用户"刚下单没付完、实例就没了"。运营可在系统配置里按需调整。
const defaultPayExpireMinutes = 1440

// 以下为装配层注入的最小暴露接口（仅声明 uc 真正用到的方法，避免依赖 admin 的 service 层）。
type (
	// productReader 商品读取（catalog.FindByID）。
	productReader interface {
		FindByID(ctx context.Context, id uint64) (*catalogdto.ProductInfo, error)
	}
	// walletPort 账务调整（余额扣款）。
	walletPort interface {
		Adjust(ctx context.Context, req accountdto.AdjustRequest, operatorID uint64) (*transdto.TransactionInfo, error)
	}
	// orderRepoPort 订单仓储读写。
	orderRepoPort interface {
		Create(ctx context.Context, item *dto.OrderRow) error
		List(ctx context.Context, query orderdto.OrderListQuery) ([]dto.OrderRow, int64, error)
		// FindByID 按主键读取订单；未找到返回 gorm.ErrRecordNotFound。
		FindByID(ctx context.Context, id uint64) (*dto.OrderRow, error)
		// TransitionStatus 条件状态迁移（CAS）：并发下只有一方生效，返回受影响行数。
		TransitionStatus(ctx context.Context, id uint64, from, to string) (int64, error)
		// ListExpiredPending 扫描超期未支付的订单（供过期关单调度器使用）。
		ListExpiredPending(ctx context.Context, before time.Time, limit int) ([]dto.OrderRow, error)
	}
	// orderItemPort 订单项写入（P5-04 补写 order_items，行级保留折扣快照）。
	orderItemPort interface {
		Create(ctx context.Context, item *dto.OrderItemRow) error
	}
	// priceResolverPort 统一算价管线（P5-03）。
	priceResolverPort interface {
		Resolve(ctx context.Context, in pricing.ResolveInput) (*pricing.Quote, error)
	}
	// orderActivator 订单履约开通（Activate）——仅在工作池未装配时同步兜底（T5.1）。
	orderActivator interface {
		Activate(ctx context.Context, id uint64) error
	}
	// provisionQueuerPort 开通履约任务投递（T5.1）：下单只落库 + 投递任务，
	// 由进程内工作池异步调用上游（上游开通约 50s，超过 app.write_timeout=10s）。
	provisionQueuerPort interface {
		EnqueueProvision(ctx context.Context, order *dto.OrderRow, sourceMode string) error
	}
	// provisionTaskLookup 开通任务状态查询（订单列表展示履约进度/失败原因）。
	provisionTaskLookup interface {
		FindByOrderIDs(ctx context.Context, orderIDs []uint64) (map[uint64]*dto.ProvisionTaskRow, error)
	}
	// salesOwnerResolver 销售归属解析（doc86 §3.4）：装配层注入，避免 uc 依赖 sales 模块。
	// 解析失败/无归属必须返回 0 且不报错，下单不因归属缺失而失败。
	salesOwnerResolver interface {
		SalesAdminForNewOrder(ctx context.Context, userID uint64) uint64
	}
	// productSkuPort 商品 SKU 读取与库存增减（T4.1：按 SKU 下单）。
	// 由 admin catalog 服务实现（装配层注入），uc 只依赖自身声明的最小接口。
	productSkuPort interface {
		// ListSpecs 列出商品下全部 SKU（含停用）：用于判断是否挂了规格并解析所选规格。
		ListSpecs(ctx context.Context, productID uint64) ([]catalogdto.ProductSpecInfo, error)
		DecrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error)
		IncrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error)
		// DecrementProductStock / IncrementProductStock 商品级库存（无 SKU 的上游转售商品）。
		DecrementProductStock(ctx context.Context, productID uint64, qty int) (int64, error)
		IncrementProductStock(ctx context.Context, productID uint64, qty int) (int64, error)
	}
	// optionLabelPort 选配项可读名解析（T4.5）：把订单里的上游 id 翻译成客户能看懂的名称。
	// 由 admin catalog 服务实现；未注入时订单不返回可读标签（前端退回不展示配置行）。
	optionLabelPort interface {
		CustomerOptions(ctx context.Context, productID uint64) ([]catalogservice.CustomerOptionGroup, error)
	}
	// optionPricingPort 客户选配项校验与计价（T4.5）：由 admin catalog 服务实现。
	optionPricingPort interface {
		// ResolveOptionPricing 校验选择并返回加价小计与归一后的选择；商品无选配项时原样回传。
		ResolveOptionPricing(ctx context.Context, productID uint64, cycle string, selections map[string]string) (*catalogservice.OptionPricing, error)
	}
	// refundPort 客户自助退款与可退性提示（装配层注入 admin 订单服务的适配器）。
	// uc 侧只做归属校验与请求转译，策略/时限/免审判定全在 admin 域（CONTRIBUTING §2 薄适配）。
	refundPort interface {
		// RequestRefund 提交退款申请，返回是否免审核自动退款。
		RequestRefund(ctx context.Context, userID, orderID uint64, req dto.UserRefundRequest) (*dto.UserRefundResult, error)
		// RefundHint 订单可退性提示（详情/列表下发，前端据此展示申请入口）。
		RefundHint(ctx context.Context, userID, orderID uint64) (*dto.OrderRefundHint, error)
	}
	// prepayPort 收银台发起支付（支付中心 Prepay）。
	prepayPort interface {
		Prepay(ctx context.Context, userID uint64, req paydto.PrepayRequest) (*paydto.OrderInfo, error)
	}
	// pendingPaymentCloser 关闭某业务单的未支付支付单（取消/超时关单时调用）。
	// 不关会留下「可继续付款但业务单已作废」的悬空资金入口。
	pendingPaymentCloser interface {
		CloseByBiz(ctx context.Context, bizType string, bizID uint64) (int, error)
	}
)

// OrderService 用户中心订单业务能力。
type OrderService interface {
	// Create 用户下单（余额支付立即开通 / 渠道支付落待支付单）。userID 为归属账号，actorID 为真实操作人（P4-04）。
	Create(ctx context.Context, userID, actorID uint64, req dto.CreateRequest) (*dto.OrderInfo, error)
	// List 我的订单。
	List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error)
	// Detail 我的订单详情（含算价快照、支付时间、支付截止时间与履约状态）。
	Detail(ctx context.Context, userID, orderID uint64) (*dto.OrderInfo, error)
	// Quote 预结算：算价明细，不落库、不扣款（P5-05）。
	Quote(ctx context.Context, userID uint64, req dto.QuoteRequest) (*dto.QuoteInfo, error)
	// Pay 待支付订单发起收银台支付：金额取订单应付（不接受前端传入），返回渠道支付参数。
	Pay(ctx context.Context, userID, orderID uint64, req dto.PayRequest) (*dto.PayInfo, error)
	// Cancel 用户取消待支付订单：置 cancelled + 关掉未支付支付单 + 回补占用的库存。
	Cancel(ctx context.Context, userID, orderID uint64) (*dto.OrderInfo, error)
	// RequestRefund 客户自助申请退款（策略按商品来源+分类判定时限与免审）。
	RequestRefund(ctx context.Context, userID, orderID uint64, req dto.UserRefundRequest) (*dto.UserRefundResult, error)
	// ExpirePending 关掉超期未付的订单（调度器调用）：置 closed，口径与用户主动取消区分。
	// expireMinutes 为配置的订单支付有效期（order_expire_minutes）。
	ExpirePending(ctx context.Context, expireMinutes int) (int, error)
	// SetPayExpireMinutes 注入待支付订单有效期（分钟，装配层读取系统配置后调用）。
	SetPayExpireMinutes(minutes int)
	// SetCashierPorts 注入收银台能力（支付中心 Prepay 与未支付单关闭）。
	SetCashierPorts(prepay prepayPort, closer pendingPaymentCloser)
	// SetOptionPricingPort 注入选配项校验与计价能力（T4.5，装配层调用）。
	SetOptionPricingPort(p optionPricingPort)
	// SetSalesOwnerResolver 注入销售归属解析（装配层调用，doc86 §3.4）。
	SetSalesOwnerResolver(r salesOwnerResolver)
	// SetInstancePlaceholder 注入「下单即建待开通实例占位」钩子（装配层调用）。
	SetInstancePlaceholder(fn func(ctx context.Context, order *dto.OrderRow) error)
	// SetPendingInstanceReleaser 注入「订单作废即删占位实例」钩子（装配层调用）。
	SetPendingInstanceReleaser(fn func(ctx context.Context, orderID uint64) (int64, error))
	// SetLogger 注入日志器（装配层调用；nil 时不记日志）。
	SetLogger(l *zap.Logger)
	// SetOptionLabelResolver 注入选配项可读名解析（装配层调用；nil 时订单详情不展示配置行）。
	SetOptionLabelResolver(port optionLabelPort)
	// SetRefundPort 注入客户自助退款端口（装配层调用；nil 时退款接口返回不可用）。
	SetRefundPort(port refundPort)
}

type orderService struct {
	catalog         productReader
	wallet          walletPort
	orderRepo       orderRepoPort
	orderItems      orderItemPort
	orderSvc        orderActivator
	provisionQueue  provisionQueuerPort // 异步开通投递（T5.1）；为 nil 时回落同步 Activate
	provisionTasks  provisionTaskLookup // 开通任务查询（订单列表状态展示）
	pricing         priceResolverPort
	ensureOpenable  func(ctx context.Context, productID uint64) error // 校验商品是否可即时开通（避免误扣款）
	isBalanceErr    func(error) bool                                  // 余额不足判定（由装配层提供，避免依赖 admin 错误变量）
	consumeRecorder func(userID uint64, amount float64)               // 累计消费/等级重算（P3-03，装配层异步化）
	actorNames      actorNameLookup                                   // 操作人用户名解析（P4-09），可为 nil
	sku             productSkuPort                                    // SKU 读取与库存（T4.1），可为 nil（无 SKU 的商品照旧下单）
	optionPricing   optionPricingPort                                 // 选配项校验与计价（T4.5），可为 nil（无选配项的商品照旧下单）
	optionLabels    optionLabelPort                                   // 选配项可读名解析（可选项；nil 时订单不带可读标签）
	refunds         refundPort                                        // 客户自助退款（装配层注入；nil 时退款接口不可用）
	salesOwner      salesOwnerResolver                                // 销售归属解析（doc86 §3.4），可为 nil（不记归属）
	prepay          prepayPort                                        // 收银台发起支付（支付中心），可为 nil（未装配时不可线上支付）
	pendingPayments pendingPaymentCloser                              // 关闭未支付支付单，可为 nil
	// instancePlaceholder 下单即建「待开通」实例占位（装配层实现），可为 nil（不建占位）。
	// 让"已下单但还没开通成功"的机器在实例列表里可见可操作（查看/重新开通），
	// 而不是开通失败后实例模块里什么都看不到（用户实测问题）。
	instancePlaceholder func(ctx context.Context, order *dto.OrderRow) error
	// pendingInstanceReleaser 删除订单的未开通占位实例（未支付关单/用户取消时），可为 nil。
	pendingInstanceReleaser func(ctx context.Context, orderID uint64) (int64, error)
	// logger 可选日志（占位实例建/删失败等"不阻断下单"的事件），由装配层注入。
	logger *zap.Logger
	// payExpireMinutes 待支付订单有效期（分钟），来自系统配置 order_expire_minutes。
	// 读接口与过期扫描都会用到（调度器每轮刷新），用原子量避免 data race。
	payExpireMinutes atomic.Int64
}

// actorNameLookup 批量解析用户 ID → 用户名，用于订单列表「操作人」列。
type actorNameLookup interface {
	NamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error)
}

// NewOrderService 创建用户中心订单服务。
// consumeRecorder/actorNames 可为 nil（累计消费与等级重算见 P3-03，操作人列见 P4-09）。
// provisionQueue 为 nil 时回落到同步 orderSvc.Activate（兼容未装配工作池的测试与旧部署）。
func NewOrderService(
	catalog productReader,
	wallet walletPort,
	orderRepo orderRepoPort,
	orderSvc orderActivator,
	ensureOpenable func(ctx context.Context, productID uint64) error,
	isBalanceErr func(error) bool,
	consumeRecorder func(userID uint64, amount float64),
	actorNames actorNameLookup,
	orderItems orderItemPort,
	pricing priceResolverPort,
	sku productSkuPort,
	provisionQueue provisionQueuerPort,
	provisionTasks provisionTaskLookup,
) OrderService {
	return &orderService{
		catalog: catalog, wallet: wallet, orderRepo: orderRepo, orderSvc: orderSvc,
		ensureOpenable: ensureOpenable, isBalanceErr: isBalanceErr,
		consumeRecorder: consumeRecorder, actorNames: actorNames,
		orderItems: orderItems, pricing: pricing, sku: sku,
		provisionQueue: provisionQueue, provisionTasks: provisionTasks,
	}
}

// SetCashierPorts 注入收银台能力（装配层调用，避免改构造签名影响既有装配点）。
// prepay 为 nil 时渠道支付不可用（Pay 返回不可支付）；closer 为 nil 时取消订单不关支付单。
func (s *orderService) SetCashierPorts(prepay prepayPort, closer pendingPaymentCloser) {
	s.prepay = prepay
	s.pendingPayments = closer
}

// SetOptionPricingPort 注入选配项校验与计价能力（T4.5，装配层调用）。
// 未注入时选配项不参与算价与校验（存量商品无选配项，行为不变）。
func (s *orderService) SetOptionPricingPort(p optionPricingPort) { s.optionPricing = p }

// resolveOptionPricing 校验客户选择并返回归一后的选择与加价小计（T4.5）。
// 未装配选配计价能力时原样回传（存量商品与服务不受影响）。
func (s *orderService) resolveOptionPricing(ctx context.Context, productID uint64, cycle string, selections map[string]string) (*catalogservice.OptionPricing, error) {
	if s.optionPricing == nil {
		out := map[string]string{}
		for k, v := range selections {
			out[k] = v
		}
		return &catalogservice.OptionPricing{Selections: out}, nil
	}
	result, err := s.optionPricing.ResolveOptionPricing(ctx, productID, cycle, selections)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &catalogservice.OptionPricing{Selections: map[string]string{}}, nil
	}
	if result.Selections == nil {
		result.Selections = map[string]string{}
	}
	return result, nil
}

// SetSalesOwnerResolver 注入销售归属解析器（装配层调用，避免改构造签名影响既有装配点）。
func (s *orderService) SetSalesOwnerResolver(r salesOwnerResolver) { s.salesOwner = r }

// SetInstancePlaceholder 注入「下单即建待开通实例」钩子（装配层调用；nil 时不建）。
func (s *orderService) SetInstancePlaceholder(fn func(ctx context.Context, order *dto.OrderRow) error) {
	s.instancePlaceholder = fn
}

// SetPendingInstanceReleaser 注入「订单作废即删占位实例」钩子（装配层调用；nil 时跳过）。
func (s *orderService) SetPendingInstanceReleaser(fn func(ctx context.Context, orderID uint64) (int64, error)) {
	s.pendingInstanceReleaser = fn
}

// SetLogger 注入日志器（装配层调用；nil 时不记日志）。
func (s *orderService) SetLogger(l *zap.Logger) { s.logger = l }

// SetRefundPort 注入客户自助退款端口（装配层调用）。
func (s *orderService) SetRefundPort(port refundPort) { s.refunds = port }

// SetOptionLabelResolver 注入选配项可读名解析（见 optionLabelPort 说明）。
func (s *orderService) SetOptionLabelResolver(port optionLabelPort) { s.optionLabels = port }

// resolveSelectionLabels 把订单里的选配 id 翻译成客户可读的「名称：取值」列表。
//
// 为什么要这层：对接商品的 config_options 存的是上游选项 id 与取值 id
// （{"163744":"565652"}），直接展示等于把对接口径摊给客户。
// 这里用商品目录（与购买页同源）还原成「网络类型：VPC网络 / 操作系统：CentOS 7.9」。
//
// 单个项翻译失败不影响其它项，也不报错：宁可不显示那一项，
// 也不能把内部 id 作为兜底展示出来。
func (s *orderService) resolveSelectionLabels(ctx context.Context, productID uint64, selections map[string]string) []string {
	if s.optionLabels == nil || len(selections) == 0 {
		return nil
	}
	groups, err := s.optionLabels.CustomerOptions(ctx, productID)
	if err != nil {
		return nil
	}
	// 建立「选项 key → 中文名」「值 → 展示名」查表。
	nameByKey := map[string]string{}
	labelByKeyValue := map[string]string{}
	for _, g := range groups {
		for _, opt := range g.Options {
			nameByKey[opt.OptionKey] = firstNonEmptyStr(opt.Name, opt.OptionKey)
			for _, v := range opt.Values {
				labelByKeyValue[selKey(opt.OptionKey, v.Value)] = v.Label
			}
		}
	}
	// 按选项 key 排序输出，保证同一订单每次展示顺序一致（map 遍历无序）。
	keys := make([]string, 0, len(selections))
	for k := range selections {
		if _, ok := nameByKey[k]; ok && strings.TrimSpace(selections[k]) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		raw := selections[k]
		label := labelByKeyValue[selKey(k, raw)]
		if label == "" {
			// 文本型（主机名/密码）没有候选值表，取值本身就是可读的。
			label = raw
		}
		// 密码类的值不回显明文。
		if k == "root_password" || k == "password" {
			label = "已设置"
		}
		out = append(out, fmt.Sprintf("%s：%s", nameByKey[k], label))
	}
	return out
}

// selKey 选配查表键：选项 key + 取值 拼成一个键。
// 用 "::" 而不是 NUL 之类的控制字符：控制字符会落到源文件里，
// 让 Go 文件被当成二进制（grep/diff/部分工具直接看不到内容），排查时非常费劲。
func selKey(optionKey, value string) string { return optionKey + "::" + value }

// firstNonEmptyStr 返回首个非空白字符串（空则回退 fallback）。
func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// logPlaceholderError 记录占位实例建/删失败：这类失败不阻断下单/关单主流程，
// 但必须在日志里留痕（否则实例"凭空消失/凭空出现"无从排查）。
func (s *orderService) logPlaceholderError(order *dto.OrderRow, err error) {
	if s.logger == nil || order == nil {
		return
	}
	s.logger.Warn("pending instance placeholder operation failed",
		zap.Uint64("order_id", order.ID), zap.String("order_no", order.OrderNo), zap.Error(err))
}

// resolveQuote 调用统一算价管线；未注入管线时回落为「商品单价 × 数量」不打折（P5-04）。
// specCode 非空时按 SKU 定价（管线内 SKU 有定价则覆盖商品级基础价）；
// cycle 非空时按周期价格矩阵取基数（矩阵未建则回落上述单价，doc25）；
// selections 为客户选配项选择（T4.5），管线据此叠加选项加价（先加价、再打折）。
func (s *orderService) resolveQuote(ctx context.Context, userID uint64, product *catalogdto.ProductInfo, specCode, cycle string, qty int, selections map[string]string) (*pricing.Quote, error) {
	if s.pricing == nil {
		amount := round2(product.Price * float64(qty))
		return &pricing.Quote{
			OriginalAmount: amount,
			DiscountAmount: 0,
			FinalAmount:    amount,
			Snapshot:       []pricing.Rule{},
		}, nil
	}
	return s.pricing.Resolve(ctx, pricing.ResolveInput{
		UserID:           userID,
		ProductID:        product.ID,
		SpecCode:         specCode,
		Cycle:            cycle,
		Quantity:         qty,
		ConfigSelections: selections,
	})
}

// resolveCycle 归一化下单周期（doc25）：
//   - 请求显式指定：非法值直接拒绝（避免静默落到默认档错价成交），
//     接受上游别名写法（month/year/quarter 等）；
//   - 留空：回落商品 price_model 对应周期，存量行为不变。
func (s *orderService) resolveCycle(product *catalogdto.ProductInfo, raw string) (string, error) {
	cycle := strings.TrimSpace(raw)
	if cycle == "" {
		return billingcycle.NormalizeOrDefault(product.PriceModel), nil
	}
	if billingcycle.IsValid(cycle) {
		return cycle, nil
	}
	if normalized := billingcycle.Normalize(cycle); normalized != "" {
		return normalized, nil
	}
	return "", fmt.Errorf("不支持的计费周期：%s", raw)
}

// resolveSku 解析下单使用的 SKU（T4.1）。
// 商品未挂任何 SKU 时返回 nil（照旧按商品级价格与规格下单，兼容存量商品）；
// 商品挂了 SKU 则必须显式选择：未传 spec_code、规格全停用或所选规格不存在都报错，
// 避免误用商品级价格或买到已下架的规格。
func (s *orderService) resolveSku(ctx context.Context, productID uint64, specCode string) (*catalogdto.ProductSpecInfo, error) {
	if s.sku == nil {
		return nil, nil
	}
	specs, err := s.sku.ListSpecs(ctx, productID)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}
	enabled := make([]catalogdto.ProductSpecInfo, 0, len(specs))
	for _, sp := range specs {
		if sp.Status == 1 {
			enabled = append(enabled, sp)
		}
	}
	if len(enabled) == 0 {
		return nil, errors.New("该商品规格已全部下架，暂不可购买")
	}
	code := strings.TrimSpace(specCode)
	if code == "" {
		return nil, errors.New("该商品有多个规格，请选择规格后再下单")
	}
	for i := range enabled {
		if enabled[i].SpecCode == code {
			return &enabled[i], nil
		}
	}
	return nil, errors.New("所选规格不存在或已下架")
}

// Create 用户下单：校验商品 → 算价 →（余额扣款 或 落待支付单）→ 触发上游开通。
//
// 两条支付链路（doc88 §6.2「待支付 + 收银台」）：
//   - pay_mode=balance（默认，存量行为）：下单即扣款，订单落 paid 并立刻投递开通；
//   - pay_mode=channel：下单只锁库存与算价，订单落 pending 等收银台付款，
//     支付成功由支付中心回调驱动（MarkPaidByChannel → ActivateByNo），此时才开通。
//
// 渠道链路必须占用库存：用户到收银台的这段时间里库存若不锁，可能超卖。
// 因此库存回补责任从「下单失败」扩展到「关单」（取消/超时），见 Cancel/ExpirePending。
func (s *orderService) Create(ctx context.Context, userID, actorID uint64, req dto.CreateRequest) (*dto.OrderInfo, error) {
	if req.ProductID == 0 {
		return nil, errors.New("商品不能为空")
	}
	payMode := strings.TrimSpace(req.PayMode)
	if payMode == "" {
		payMode = dto.PayModeBalance
	}
	if payMode != dto.PayModeBalance && payMode != dto.PayModeChannel {
		return nil, ErrPayModeInvalid
	}
	// 开放平台代客下单（P6/T6.3）是程序化渠道，不经过用户收银台：强制余额支付，
	// 否则下游接口会返回一张没人能付的待支付单。
	if req.ChannelMeta.Channel != "" {
		payMode = dto.PayModeBalance
	}
	useChannel := payMode == dto.PayModeChannel
	product, err := s.catalog.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Status != 1 {
		return nil, ErrProductOffline
	}
	// SKU 解析（T4.1）：商品挂了规格则必须选规格；未挂规格照旧按商品级下单。
	spec, err := s.resolveSku(ctx, req.ProductID, req.SpecCode)
	if err != nil {
		return nil, err
	}
	if spec == nil && product.Price <= 0 {
		return nil, errors.New("商品定价无效，暂不可购买")
	}
	if spec != nil && spec.Price <= 0 {
		return nil, errors.New("规格定价无效，暂不可购买")
	}
	// 校验商品是否可即时开通（财务型上游等不支持直连开通，避免误扣款）
	if s.ensureOpenable != nil {
		if err := s.ensureOpenable(ctx, req.ProductID); err != nil {
			return nil, err
		}
	}
	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}
	specCode := ""
	if spec != nil {
		specCode = spec.SpecCode
	}
	// 计费周期（doc25）：显式周期需为规范值，留空回落商品 price_model。
	cycle, err := s.resolveCycle(product, req.Cycle)
	if err != nil {
		return nil, err
	}
	// 客户选配项（T4.5）：校验取值合法性并归一选择（补齐必选项默认值）。
	// 归一后的 selections 才是落库与履约依据——客户只填了必选项之外的部分时，
	// 平台上仍需收到必选项的取值，否则开通会被平台拒绝。
	pricingResult, err := s.resolveOptionPricing(ctx, req.ProductID, cycle, req.ConfigSelections)
	if err != nil {
		return nil, err
	}
	selections := pricingResult.Selections
	// 统一算价管线（P5-03/P5-04）：原价、优惠、实付与折扣来源一并落库。
	// 周期价格矩阵存在时以其价为基数；该周期未开放则算价报错，订单不落库、不扣款。
	quote, err := s.resolveQuote(ctx, userID, product, specCode, cycle, qty, selections)
	if err != nil {
		return nil, err
	}
	// 选项加价小计：未装配算价管线时兜底用选配计价结果（否则前端展示的加价会丢）。
	if quote.OptionAmount == 0 && pricingResult.Amount > 0 {
		quote.OptionAmount = pricingResult.Amount
	}
	amount := quote.FinalAmount
	if amount <= 0 {
		return nil, errors.New("算价结果无效，暂不可购买")
	}
	// SKU 库存扣减（T4.1）：有限库存用条件更新原子扣减，扣不到即视为售罄。
	// 后续任何一步失败都在 defer 中回补，保证不因下单失败丢库存。
	stockDeducted := false
	// productStockDeducted 商品级库存是否已扣（无 SKU 的上游转售商品走这条线）。
	productStockDeducted := false
	if spec != nil {
		affected, err := s.sku.DecrementSpecStock(ctx, spec.ID, qty)
		if err != nil {
			return nil, err
		}
		if affected == 0 {
			return nil, errors.New("所选规格库存不足")
		}
		stockDeducted = true
	} else {
		// 商品级库存扣减：上游转售商品没有 SKU（库存挂在上游限量上），
		// 此前 product.stock 只做展示、下单完全不校验，上游限量商品可以被无限下单。
		// 口径与 SKU 一致：-1 不限量不扣减，有限库存扣不到即售罄。
		affected, err := s.sku.DecrementProductStock(ctx, product.ID, qty)
		if err != nil {
			return nil, err
		}
		if affected == 0 {
			return nil, errors.New("该商品库存不足，已售罄")
		}
		productStockDeducted = true
	}
	orderSucceeded := false
	defer func() {
		if !orderSucceeded {
			// 回补失败只影响库存准确性，不改变下单结果；由运维按订单人工核对。
			if stockDeducted {
				_, _ = s.sku.IncrementSpecStock(context.WithoutCancel(ctx), spec.ID, qty)
			}
			if productStockDeducted {
				_, _ = s.sku.IncrementProductStock(context.WithoutCancel(ctx), product.ID, qty)
			}
		}
	}()

	// 1) 余额扣款（支出方向 -1），返回资金流水。
	// 渠道支付此刻不扣款：钱在渠道侧，等回调确认到账后再标记 paid。
	// 流水类型统一为 consume（P0-02）：累计消费聚合依赖 type='consume'，BizType 仍为 order 保证幂等键不变。
	if !useChannel {
		if _, err := s.wallet.Adjust(ctx, accountdto.AdjustRequest{
			UserID:    userID,
			Type:      TxTypeConsume,
			Direction: -1, // 支出
			Amount:    amount,
			BizKey:    fmt.Sprintf("uc-order-%d-%d", userID, time.Now().UnixNano()),
			Remark:    fmt.Sprintf("购买云主机：%s", product.Name),
		}, userID); err != nil {
			if s.isBalanceErr != nil && s.isBalanceErr(err) {
				return nil, ErrInsufficientBalance
			}
			return nil, err
		}
	}

	// 2) 创建订单（含算价快照，P5-04）：余额链路落 paid，渠道链路落 pending。
	// 规格快照优先取 SKU 的 Specs（原子取值 JSON），未选规格时回落商品级 Specs。
	specsSnapshot := product.Specs
	if spec != nil && strings.TrimSpace(spec.Specs) != "" {
		specsSnapshot = spec.Specs
	}
	now := time.Now()
	order := &dto.OrderRow{
		OrderNo:     genOrderNo(userID),
		UserID:      userID,
		ProductID:   product.ID,
		ProductName: product.Name,
		Specs:       specsSnapshot,
		SpecCode:    specCode,
		// 客户选配项快照（T4.5）：履约的唯一真源（worker 重读订单即可取到）。
		ConfigOptions:  selectionsJSON(selections),
		Quantity:       qty,
		PriceModel:     product.PriceModel,
		Cycle:          cycle,
		TotalAmount:    quote.OriginalAmount,
		OriginalAmount: quote.OriginalAmount,
		DiscountAmount: quote.DiscountAmount,
		FinalAmount:    quote.FinalAmount,
		PricePolicyID:  quote.PolicyID,
		DiscountSource: quote.Source,
		PriceSnapshot:  snapshotJSON(quote.Snapshot),
		Status:         dto.OrderStatusPaid,
		PayMethod:      dto.PayModeBalance,
		PaidAmount:     amount,
		PayTime:        &now,
		// 选项加价小计落 price_snapshot 之外的量：orders 无独立列，随选配明细一起在
		// order_items 体现（行级 Amount 已含加价），此处不再新增列以免迁移面扩大。
		OperatorID: actorID, // 真实操作人（子账号下单可追溯，P4-09）
		// 客户备注：下单时填写，履约建实例占位时带到 instances.customer_remark。
		CustomerRemark: truncRunes(strings.TrimSpace(req.CustomerRemark), 255),
	}
	if useChannel {
		// 未收款：不写 paid_amount/pay_method/pay_time，避免未付款的订单混进销售额口径
		// （仓储侧 paidStatuses 已排除 pending，这里保持列语义干净）。
		order.Status = dto.OrderStatusPending
		order.PayMethod = ""
		order.PaidAmount = 0
		order.PayTime = nil
	}
	// 销售归属快照（doc86 §3.4）：下单瞬间锁定，后续改派不影响已成交订单的提成归属。
	if s.salesOwner != nil {
		order.SalesAdminID = s.salesOwner.SalesAdminForNewOrder(ctx, userID)
	}
	// 开放平台代客下单（P6/T6.3）：渠道归属由 open 模块程序化注入，UC 自有路由为空值。
	if req.ChannelMeta.Channel != "" {
		order.Channel = req.ChannelMeta.Channel
		order.OpenAppID = req.ChannelMeta.OpenAppID
		order.ChannelCustomerRef = req.ChannelMeta.ChannelCustomerRef
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}

	// 2.2) 补写订单项：行级保留原价/优惠/实付，否则 order_items 折扣字段永远为 0（P5-04）。
	// spec_code 落 SKU 编码（T4.1），供履约按 SKU 取规格原子与平台参数。
	if s.orderItems != nil {
		// 行单价取算价结果的单位原价：周期矩阵命中时即该周期基质价（doc25），
		// 保证订单项与订单头的原价口径一致。
		itemPrice := round2(quote.OriginalAmount / float64(qty))
		item := &dto.OrderItemRow{
			OrderID:        order.ID,
			ProductID:      product.ID,
			ProductName:    product.Name,
			SpecCode:       specCode,
			Specs:          specsSnapshot,
			ConfigOptions:  selectionsJSON(selections),
			Cycle:          cycle,
			Price:          itemPrice,
			Quantity:       qty,
			Amount:         amount,
			OriginalAmount: quote.OriginalAmount,
			DiscountAmount: quote.DiscountAmount,
			FinalAmount:    quote.FinalAmount,
		}
		if err := s.orderItems.Create(ctx, item); err != nil {
			return nil, err
		}
	}

	// 2.3) 下单即建「待开通」实例占位（含未支付订单）：用户实测反馈"开通失败后实例模块里
	//      什么都看不到，无法查看/重新开通"。占位行随订单存在，开通成功时就地转正，
	//      未支付超时/取消时删除。失败只记日志：订单已落库，不因占位失败而失败。
	if s.instancePlaceholder != nil {
		if err := s.instancePlaceholder(ctx, order); err != nil {
			s.logPlaceholderError(order, err)
		}
	}

	// 2.4) 渠道支付：到此为止订单已可靠落库（含库存占用），直接返回待支付单与支付截止时间。
	//      累计消费与开通都必须等到账后（未付款不该累计消费、更不能开通资源）。
	//      订单号此处即可确定，用户点「去支付」时才真正向渠道下单（避免无人支付也占用渠道单）。
	if useChannel {
		orderSucceeded = true
		info := fromAdminOrder(*order, nil)
		info.Status = dto.OrderStatusPending
		if expire := s.payExpireAt(order); expire != "" {
			info.PayExpireAt = expire
		}
		return &info, nil
	}

	// 2.5) 累计消费 + 等级重算（P3-03）：已扣款即计入，开通失败也不回退。
	//      装配层注入的实现负责异步化与错误日志，不阻塞下单主流程。
	if s.consumeRecorder != nil {
		s.consumeRecorder(userID, amount)
	}

	// 3) 投递开通履约任务（T5.1）：上游开通约 50s，超过 app.write_timeout=10s，
	//    因此下单只落库 + 投递任务，由进程内工作池异步执行 paid→provisioning→active；
	//    接口立即返回，前端按订单状态轮询。
	//    未装配工作池（provisionQueue=nil）时回落为同步开通，保持既有行为。
	if s.provisionQueue != nil {
		if err := s.provisionQueue.EnqueueProvision(ctx, order, product.SourceMode); err != nil {
			// 任务投递失败：订单保持 paid，可稍后重试；库存由 defer 回补。
			info := fromAdminOrder(*order, nil)
			info.Status = dto.OrderStatusPaid
			return &info, ErrProvisionFailed
		}
		orderSucceeded = true
		info := fromAdminOrder(*order, nil)
		info.Status = dto.OrderStatusPaid // 待工作池推进，前端轮询
		info.ProvisionStatus = dto.ProvisionTaskPending
		return &info, nil
	}

	// 兜底：同步触发上游开通（orderService.Activate：paid → 创建实例 → active）
	if err := s.orderSvc.Activate(ctx, order.ID); err != nil {
		// 已扣款，订单保持 paid，允许用户稍后重试；库存由 defer 回补。
		info := fromAdminOrder(*order, nil)
		info.Status = dto.OrderStatusPaid
		return &info, ErrProvisionFailed
	}
	orderSucceeded = true
	info := fromAdminOrder(*order, nil)
	info.Status = dto.OrderStatusActive
	return &info, nil
}

// List 我的订单。
func (s *orderService) List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.orderRepo.List(ctx, orderdto.OrderListQuery{
		UserID: userID, Status: query.Status, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	resp := &dto.ListResponse{Page: page, PageSize: pageSize, Total: total, Items: make([]dto.OrderInfo, 0, len(items))}
	actorNames := s.resolveActorNames(ctx, items)
	provisionTasks := s.resolveProvisionTasks(ctx, items)
	for _, it := range items {
		info := fromAdminOrder(it, actorNames)
		if task := provisionTasks[it.ID]; task != nil {
			info.ProvisionStatus = task.Status
			info.ProvisionError = task.LastError
		}
		info.PayExpireAt = s.payExpireAt(&it)
		resp.Items = append(resp.Items, info)
	}
	return resp, nil
}

// Detail 我的订单详情。
//
// 归属校验放在服务层而不是仓储查询条件里：findByID 后比对 UserID，
// 他人的订单与不存在的订单返回同一错误（ErrOrderNotFound），避免通过订单号探测他人数据。
func (s *orderService) Detail(ctx context.Context, userID, orderID uint64) (*dto.OrderInfo, error) {
	if orderID == 0 {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrOrderNotFound
	}

	items := []dto.OrderRow{*order}
	info := fromAdminOrder(*order, s.resolveActorNames(ctx, items))
	info.SpecCode = order.SpecCode
	info.Quantity = order.Quantity
	info.Cycle = order.Cycle
	info.PriceSnapshot = order.PriceSnapshot
	// 选配项可读名：把 config_options 里的上游 id 还原成「网络类型：VPC网络」。
	// 前端只展示这个，不展示 ConfigSelections 的原始键值对。
	info.ConfigSelectionLabels = s.resolveSelectionLabels(ctx, order.ProductID, info.ConfigSelections)
	if order.PayTime != nil {
		info.PayTime = order.PayTime.Format(time.RFC3339)
	}
	if order.ExpireTime != nil {
		info.ExpireTime = order.ExpireTime.Format(time.RFC3339)
	}
	if task := s.resolveProvisionTasks(ctx, items)[order.ID]; task != nil {
		info.ProvisionStatus = task.Status
		info.ProvisionError = task.LastError
	}
	info.PayExpireAt = s.payExpireAt(order)
	// 可退性提示：前端据此决定是否展示「申请退款」入口与剩余时限。
	if s.refunds != nil {
		if hint, herr := s.refunds.RefundHint(ctx, userID, orderID); herr == nil {
			info.RefundHint = hint
		}
	}
	return &info, nil
}

// RequestRefund 客户自助申请退款（薄适配：策略判定与资金链路全在 admin 域）。
//
// uc 侧只做两件事：归属校验（他人订单按不存在处理）与请求转译。
// 具体规则（时限/来源分流/免审自动退/人工审核）见 admin order service 的 UserCreateRefund。
func (s *orderService) RequestRefund(ctx context.Context, userID, orderID uint64, req dto.UserRefundRequest) (*dto.UserRefundResult, error) {
	if orderID == 0 {
		return nil, ErrOrderNotFound
	}
	if s.refunds == nil {
		return nil, ErrRefundUnavailable
	}
	// 归属校验前置：非本人订单一律「不存在」，不泄漏订单存在性。
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	return s.refunds.RequestRefund(ctx, userID, orderID, req)
}

// Pay 待支付订单发起收银台支付（doc88 §6.2）。
//
// 金额一律取订单算价快照，不接受前端传入：前端只选渠道与场景，避免改价支付。
// 支付成功由支付中心回调驱动入账与开通（装配层 paid hook → MarkPaidByChannel → ActivateByNo），
// 因此本方法只负责「向渠道下单并把参数交给收银台」，不碰订单资金状态。
func (s *orderService) Pay(ctx context.Context, userID, orderID uint64, req dto.PayRequest) (*dto.PayInfo, error) {
	if s.prepay == nil {
		return nil, ErrOrderNotPayable
	}
	order, err := s.ownedOrder(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != dto.OrderStatusPending {
		return nil, ErrOrderNotPayable
	}
	amount := payableAmount(order)
	if amount <= 0 {
		return nil, ErrOrderNotPayable
	}
	subject := "云产品订单支付"
	if order.ProductName != "" {
		subject = "云产品订单支付：" + order.ProductName
	}
	Scene := strings.TrimSpace(req.Scene)
	info, err := s.prepay.Prepay(ctx, userID, paydto.PrepayRequest{
		BizType:     paydto.BizTypeOrder,
		BizID:       order.ID,
		BizNo:       order.OrderNo,
		Amount:      amount,
		Subject:     subject,
		Scene:       Scene,
		ChannelCode: strings.TrimSpace(req.ChannelCode),
	})
	if err != nil {
		return nil, err
	}
	return &dto.PayInfo{
		OrderID:      order.ID,
		OrderNo:      order.OrderNo,
		PaymentID:    info.ID,
		PaymentNo:    info.PaymentNo,
		Amount:       info.Amount,
		ChannelCode:  info.ChannelCode,
		ChannelName:  info.ChannelName,
		Scene:        info.Scene,
		Status:       info.Status,
		PayURL:       info.PayURL,
		QRCode:       info.QRCode,
		Instructions: info.Instructions,
		Subject:      info.Subject,
		ExpireAt:     info.ExpireAt,
	}, nil
}

// Cancel 用户取消待支付订单。
//
// 顺序有意为之：先 CAS 关单（并发下与支付成功回调互斥，避免钱付了单被取消），
// 成功后再关掉未支付支付单并回补库存。关单失败说明订单已被支付或被系统关单，
// 直接按不可取消返回，不再动支付单与库存。
func (s *orderService) Cancel(ctx context.Context, userID, orderID uint64) (*dto.OrderInfo, error) {
	order, err := s.ownedOrder(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != dto.OrderStatusPending {
		return nil, ErrOrderNotCancellable
	}
	moved, err := s.orderRepo.TransitionStatus(ctx, order.ID, dto.OrderStatusPending, dto.OrderStatusCancelled)
	if err != nil {
		return nil, err
	}
	if moved == 0 {
		// 并发：订单刚被支付成功或已被超时关单，以库内状态为准。
		return nil, ErrOrderNotCancellable
	}
	order.Status = dto.OrderStatusCancelled
	s.releasePendingHoldings(ctx, order)
	return s.Detail(ctx, userID, order.ID)
}

// ExpirePending 关掉超期未付的订单（保留期由 order_expire_minutes 配置，调度器周期调用）。
//
// 与用户主动取消分开落 closed：后台可据此区分「用户放弃」与「超时作废」两类流失。
// 只处理有支付单的订单（仓储侧过滤），续费/后台代下单的 pending 不在关单范围内。
func (s *orderService) ExpirePending(ctx context.Context, expireMinutes int) (int, error) {
	if expireMinutes <= 0 {
		expireMinutes = defaultPayExpireMinutes
	}
	before := time.Now().Add(-time.Duration(expireMinutes) * time.Minute)
	items, err := s.orderRepo.ListExpiredPending(ctx, before, 200)
	if err != nil {
		return 0, err
	}
	closed := 0
	for i := range items {
		it := items[i]
		moved, err := s.orderRepo.TransitionStatus(ctx, it.ID, dto.OrderStatusPending, dto.OrderStatusClosed)
		if err != nil {
			// 单笔失败不影响其余订单，下一轮扫描会重试。
			continue
		}
		if moved == 0 {
			// 扫描与支付成功回调并发：已被到账，跳过（不得回补库存、不得关支付单）。
			continue
		}
		it.Status = dto.OrderStatusClosed
		s.releasePendingHoldings(ctx, &it)
		closed++
	}
	return closed, nil
}

// releasePendingHoldings 释放待支付订单占用的资源：关闭未支付支付单 + 回补 SKU 库存。
//
// 两者都是尽力而为且互不阻断：失败只影响库存/支付单准确性，不回滚订单状态（订单已确定作废，
// 由运维按订单人工核对）。支付单要尽力关掉：否则用户仍能付款到一张已作废的订单，
// 到账后又无单可开，形成悬空资金。
func (s *orderService) releasePendingHoldings(ctx context.Context, order *dto.OrderRow) {
	if s.pendingPayments != nil {
		_, _ = s.pendingPayments.CloseByBiz(ctx, paydto.BizTypeOrder, order.ID)
	}
	// 删除该订单的「待开通」占位实例：订单已作废，实例列表里不该再留一台永远不会开通的机器。
	if s.pendingInstanceReleaser != nil {
		if _, err := s.pendingInstanceReleaser(ctx, order.ID); err != nil {
			s.logPlaceholderError(order, err)
		}
	}
	if s.sku == nil || order.Quantity <= 0 {
		return
	}
	releaseStock := context.WithoutCancel(ctx)
	// 无 SKU 的上游转售商品：库存挂在商品行上，关单同样要回补，
	// 否则每取消一单就永久吃掉一份库存（越卖越少）。
	if order.SpecCode == "" {
		_, _ = s.sku.IncrementProductStock(releaseStock, order.ProductID, order.Quantity)
		return
	}
	specs, err := s.sku.ListSpecs(ctx, order.ProductID)
	if err != nil {
		// 读 SKU 失败无法定位是哪一个 SKU 占的库存。这里**不能**退回商品级回补：
		// 下单时挂了 SKU 的商品只扣了 SKU 库存（商品级未扣），再回补商品级会凭空多出库存。
		return
	}
	for i := range specs {
		if specs[i].SpecCode == order.SpecCode {
			// 独立 context：请求已可能被取消，库存回补仍要落库。
			_, _ = s.sku.IncrementSpecStock(releaseStock, specs[i].ID, order.Quantity)
			return
		}
	}
}

// ownedOrder 读取归属用户的订单；不存在与他人订单一律 ErrOrderNotFound（避免订单 ID 探测）。
func (s *orderService) ownedOrder(ctx context.Context, userID, orderID uint64) (*dto.OrderRow, error) {
	if orderID == 0 {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil || order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

// SetPayExpireMinutes 注入待支付订单有效期（装配层每轮扫描刷新配置后调用）。
func (s *orderService) SetPayExpireMinutes(minutes int) {
	if minutes <= 0 {
		minutes = defaultPayExpireMinutes
	}
	s.payExpireMinutes.Store(int64(minutes))
}

// payExpireMinutesOrDefault 当前有效期（分钟）。
func (s *orderService) payExpireMinutesOrDefault() int {
	if v := s.payExpireMinutes.Load(); v > 0 {
		return int(v)
	}
	return defaultPayExpireMinutes
}

// payExpireAt 待支付订单的支付截止时间（RFC3339，非 pending 为空串）。
// 实时推导而不落库：orders.expire_time 是实例计费到期时间，两者语义不同不可混用。
func (s *orderService) payExpireAt(order *dto.OrderRow) string {
	if order == nil || order.Status != dto.OrderStatusPending || order.CreatedAt.IsZero() {
		return ""
	}
	return order.CreatedAt.Add(time.Duration(s.payExpireMinutesOrDefault()) * time.Minute).Format(time.RFC3339)
}

// resolveProvisionTasks 批量取订单的开通任务状态（T5.1，避免 N+1）；失败降级为空映射。
func (s *orderService) resolveProvisionTasks(ctx context.Context, items []dto.OrderRow) map[uint64]*dto.ProvisionTaskRow {
	if s.provisionTasks == nil || len(items) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	tasks, err := s.provisionTasks.FindByOrderIDs(ctx, ids)
	if err != nil {
		return nil
	}
	return tasks
}

// resolveActorNames 批量解析订单操作人用户名；失败时降级为空映射（不阻断列表）。
func (s *orderService) resolveActorNames(ctx context.Context, items []dto.OrderRow) map[uint64]string {
	if s.actorNames == nil {
		return nil
	}
	ids := make([]uint64, 0, len(items))
	seen := make(map[uint64]struct{}, len(items))
	for _, it := range items {
		if it.OperatorID == 0 {
			continue
		}
		if _, ok := seen[it.OperatorID]; ok {
			continue
		}
		seen[it.OperatorID] = struct{}{}
		ids = append(ids, it.OperatorID)
	}
	if len(ids) == 0 {
		return nil
	}
	names, err := s.actorNames.NamesByIDs(ctx, ids)
	if err != nil {
		return nil
	}
	return names
}

// fromAdminOrder 映射订单为用户可见字段。
func fromAdminOrder(o dto.OrderRow, actorNames map[uint64]string) dto.OrderInfo {
	info := dto.OrderInfo{
		ID:             o.ID,
		OrderNo:        o.OrderNo,
		ProductID:      o.ProductID,
		ProductName:    o.ProductName,
		Specs:          o.Specs,
		PriceModel:     o.PriceModel,
		TotalAmount:    o.TotalAmount,
		PaidAmount:     o.PaidAmount,
		Status:         o.Status,
		PayMethod:      o.PayMethod,
		ActorID:        o.OperatorID,
		ActorName:      actorNames[o.OperatorID],
		SpecCode:       o.SpecCode,
		Quantity:       o.Quantity,
		Cycle:          o.Cycle,
		OriginalAmount: o.OriginalAmount,
		DiscountAmount: o.DiscountAmount,
		FinalAmount:    o.FinalAmount,
		DiscountSource: o.DiscountSource,
		CreatedAt:      o.CreatedAt.Format(time.RFC3339),
		// 注意这里**不带** o.Remark：它是运营在管理端写的内部备注
		// （PUT /orders/:id/remark），而本函数是订单 → 用户可见字段的唯一出口
		// （列表与详情都走它）。逐个调用点去清容易漏，在这里就不带出去。
		CustomerRemark: o.CustomerRemark,
	}
	// 客户选配项选择回显（T4.5）：详情页展示「所选配置」，售后核对也看这个。
	if strings.TrimSpace(o.ConfigOptions) != "" {
		var selections map[string]string
		if err := json.Unmarshal([]byte(o.ConfigOptions), &selections); err == nil && len(selections) > 0 {
			info.ConfigSelections = selections
		}
	}
	if o.PayTime != nil {
		info.PayTime = o.PayTime.Format(time.RFC3339)
	}
	if o.ExpireTime != nil {
		info.ExpireTime = o.ExpireTime.Format(time.RFC3339)
	}
	return info
}

// Quote 预结算：只算价不落库、不扣款（P5-05）。
func (s *orderService) Quote(ctx context.Context, userID uint64, req dto.QuoteRequest) (*dto.QuoteInfo, error) {
	if req.ProductID == 0 {
		return nil, errors.New("商品不能为空")
	}
	product, err := s.catalog.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Status != 1 {
		return nil, ErrProductOffline
	}
	// 预结算同样按 SKU 定价（T4.1），保证与下单口径一致。
	spec, err := s.resolveSku(ctx, req.ProductID, req.SpecCode)
	if err != nil {
		return nil, err
	}
	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}
	specCode := ""
	if spec != nil {
		specCode = spec.SpecCode
	}
	// 预结算周期口径与下单完全一致（doc25）：非法周期直接拒绝。
	cycle, err := s.resolveCycle(product, req.Cycle)
	if err != nil {
		return nil, err
	}
	// 选配项校验与计价（T4.5）：与下单同一套规则，保证预结算金额就是成交金额。
	pricingResult, err := s.resolveOptionPricing(ctx, req.ProductID, cycle, req.ConfigSelections)
	if err != nil {
		return nil, err
	}
	quote, err := s.resolveQuote(ctx, userID, product, specCode, cycle, qty, pricingResult.Selections)
	if err != nil {
		return nil, err
	}
	if quote.OptionAmount == 0 && pricingResult.Amount > 0 {
		quote.OptionAmount = pricingResult.Amount
	}
	return &dto.QuoteInfo{
		SpecCode:       specCode,
		Cycle:          cycle,
		OriginalAmount: quote.OriginalAmount,
		DiscountAmount: quote.DiscountAmount,
		FinalAmount:    quote.FinalAmount,
		OptionAmount:   quote.OptionAmount,
		Options:        toSelectedOptions(pricingResult.Items),
		PolicyID:       quote.PolicyID,
		Source:         quote.Source,
		Snapshot:       quote.Snapshot,
	}, nil
}

// toSelectedOptions 把选配计价明细映射为预结算返回结构。
func toSelectedOptions(items []catalogservice.OptionPricingItem) []dto.SelectedOption {
	if len(items) == 0 {
		return []dto.SelectedOption{}
	}
	out := make([]dto.SelectedOption, 0, len(items))
	for _, it := range items {
		out = append(out, dto.SelectedOption{
			OptionKey:   it.OptionKey,
			OptionName:  it.OptionName,
			Value:       it.Value,
			ValueName:   it.ValueName,
			Quantity:    it.Quantity,
			UnitPrice:   it.UnitPrice,
			Amount:      it.Amount,
			IsDefaulted: it.IsDefaulted,
		})
	}
	return out
}

// selectionsJSON 把客户选配项选择序列化为 JSONB 文本；空选择返回空串（列落 NULL）。
func selectionsJSON(selections map[string]string) string {
	if len(selections) == 0 {
		return ""
	}
	raw, err := json.Marshal(selections)
	if err != nil {
		return ""
	}
	return string(raw)
}

// snapshotJSON 将命中规则序列化为 JSONB 文本；空快照存空数组。
func snapshotJSON(rules []pricing.Rule) string {
	if rules == nil {
		rules = []pricing.Rule{}
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// round2 金额保留两位小数。
func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

// payableAmount 订单应付金额：算价快照（final_amount）优先，兼容未写快照的存量订单。
func payableAmount(o *dto.OrderRow) float64 {
	if o.FinalAmount > 0 {
		return o.FinalAmount
	}
	return o.TotalAmount
}

// genOrderNo 生成订单号：时间戳 + 随机。
func genOrderNo(userID uint64) string {
	return fmt.Sprintf("UC%d%d%04d", userID, time.Now().Unix(), rand.Intn(10000))
}

// truncRunes 按**字符**截断（不是字节）：中文备注按字节截断会切出半个字，
// 落库后前端显示成乱码方块。超长直接截断而不是报错，避免因备注长度挡下单主流程。
func truncRunes(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen])
}
