// Package service 提供用户中心订单模块业务编排：余额支付下单 → 触发上游开通。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	orderdto "hostsent/backend/internal/modules/admin/order/dto"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/pkg/pricing"

	"hostsent/backend/internal/modules/uc/order/dto"
)

var (
	// ErrProductOffline 商品下架不可购买
	ErrProductOffline = errors.New("商品已下架，不可购买")
	// ErrInsufficientBalance 余额不足
	ErrInsufficientBalance = errors.New("余额不足，请先充值")
	// ErrProvisionFailed 开通上游失败（已扣款，可稍后在订单页重试）
	ErrProvisionFailed = errors.New("开通资源失败，已扣款，可稍后重试")
)

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
		Create(ctx context.Context, item *ordermodel.Order) error
		List(ctx context.Context, query orderdto.OrderListQuery) ([]ordermodel.Order, int64, error)
	}
	// orderItemPort 订单项写入（P5-04 补写 order_items，行级保留折扣快照）。
	orderItemPort interface {
		Create(ctx context.Context, item *ordermodel.OrderItem) error
	}
	// priceResolverPort 统一算价管线（P5-03）。
	priceResolverPort interface {
		Resolve(ctx context.Context, in pricing.ResolveInput) (*pricing.Quote, error)
	}
	// orderActivator 订单履约开通（Activate）。
	orderActivator interface {
		Activate(ctx context.Context, id uint64) error
	}
	// productSkuPort 商品 SKU 读取与库存增减（T4.1：按 SKU 下单）。
	// 由 admin catalog 服务实现（装配层注入），uc 只依赖自身声明的最小接口。
	productSkuPort interface {
		// ListSpecs 列出商品下全部 SKU（含停用）：用于判断是否挂了规格并解析所选规格。
		ListSpecs(ctx context.Context, productID uint64) ([]catalogdto.ProductSpecInfo, error)
		DecrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error)
		IncrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error)
	}
)

// OrderService 用户中心订单业务能力。
type OrderService interface {
	// Create 用户下单（余额支付 + 开通上游）。userID 为归属账号，actorID 为真实操作人（P4-04）。
	Create(ctx context.Context, userID, actorID uint64, req dto.CreateRequest) (*dto.OrderInfo, error)
	// List 我的订单。
	List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error)
	// Quote 预结算：算价明细，不落库、不扣款（P5-05）。
	Quote(ctx context.Context, userID uint64, req dto.QuoteRequest) (*dto.QuoteInfo, error)
}

type orderService struct {
	catalog         productReader
	wallet          walletPort
	orderRepo       orderRepoPort
	orderItems      orderItemPort
	orderSvc        orderActivator
	pricing         priceResolverPort
	ensureOpenable  func(ctx context.Context, productID uint64) error // 校验商品是否可即时开通（避免误扣款）
	isBalanceErr    func(error) bool                                  // 余额不足判定（由装配层提供，避免依赖 admin 错误变量）
	consumeRecorder func(userID uint64, amount float64)               // 累计消费/等级重算（P3-03，装配层异步化）
	actorNames      actorNameLookup                                   // 操作人用户名解析（P4-09），可为 nil
	sku             productSkuPort                                    // SKU 读取与库存（T4.1），可为 nil（无 SKU 的商品照旧下单）
}

// actorNameLookup 批量解析用户 ID → 用户名，用于订单列表「操作人」列。
type actorNameLookup interface {
	NamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error)
}

// NewOrderService 创建用户中心订单服务。
// consumeRecorder/actorNames 可为 nil（累计消费与等级重算见 P3-03，操作人列见 P4-09）。
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
) OrderService {
	return &orderService{
		catalog: catalog, wallet: wallet, orderRepo: orderRepo, orderSvc: orderSvc,
		ensureOpenable: ensureOpenable, isBalanceErr: isBalanceErr,
		consumeRecorder: consumeRecorder, actorNames: actorNames,
		orderItems: orderItems, pricing: pricing, sku: sku,
	}
}

// resolveQuote 调用统一算价管线；未注入管线时回落为「商品单价 × 数量」不打折（P5-04）。
// specCode 非空时按 SKU 定价（管线内 SKU 有定价则覆盖商品级基础价）。
func (s *orderService) resolveQuote(ctx context.Context, userID uint64, product *catalogdto.ProductInfo, specCode string, qty int) (*pricing.Quote, error) {
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
		UserID:    userID,
		ProductID: product.ID,
		SpecCode:  specCode,
		Quantity:  qty,
	})
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

// Create 用户下单：校验商品 → 算价 → 余额扣款 → 创建已支付订单 → 触发上游开通。
func (s *orderService) Create(ctx context.Context, userID, actorID uint64, req dto.CreateRequest) (*dto.OrderInfo, error) {
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
	// 统一算价管线（P5-03/P5-04）：原价、优惠、实付与折扣来源一并落库。
	quote, err := s.resolveQuote(ctx, userID, product, specCode, qty)
	if err != nil {
		return nil, err
	}
	amount := quote.FinalAmount
	if amount <= 0 {
		return nil, errors.New("算价结果无效，暂不可购买")
	}
	// SKU 库存扣减（T4.1）：有限库存用条件更新原子扣减，扣不到即视为售罄。
	// 后续任何一步失败都在 defer 中回补，保证不因下单失败丢库存。
	stockDeducted := false
	if spec != nil {
		affected, err := s.sku.DecrementSpecStock(ctx, spec.ID, qty)
		if err != nil {
			return nil, err
		}
		if affected == 0 {
			return nil, errors.New("所选规格库存不足")
		}
		stockDeducted = true
	}
	orderSucceeded := false
	defer func() {
		if stockDeducted && !orderSucceeded {
			// 回补失败只影响库存准确性，不改变下单结果；由运维按订单人工核对。
			_, _ = s.sku.IncrementSpecStock(context.WithoutCancel(ctx), spec.ID, qty)
		}
	}()

	// 1) 余额扣款（支出方向 -1），返回资金流水
	// 流水类型统一为 consume（P0-02）：累计消费聚合依赖 type='consume'，BizType 仍为 order 保证幂等键不变。
	if _, err := s.wallet.Adjust(ctx, accountdto.AdjustRequest{
		UserID:    userID,
		Type:      transmodel.TxTypeConsume,
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

	// 2) 创建已支付订单（含算价快照，P5-04）。
	// 规格快照优先取 SKU 的 Specs（原子取值 JSON），未选规格时回落商品级 Specs。
	specsSnapshot := product.Specs
	if spec != nil && strings.TrimSpace(spec.Specs) != "" {
		specsSnapshot = spec.Specs
	}
	now := time.Now()
	order := &ordermodel.Order{
		OrderNo:        genOrderNo(userID),
		UserID:         userID,
		ProductID:      product.ID,
		ProductName:    product.Name,
		Specs:          specsSnapshot,
		SpecCode:       specCode,
		Quantity:       qty,
		PriceModel:     product.PriceModel,
		TotalAmount:    quote.OriginalAmount,
		PaidAmount:     amount,
		OriginalAmount: quote.OriginalAmount,
		DiscountAmount: quote.DiscountAmount,
		FinalAmount:    quote.FinalAmount,
		PricePolicyID:  quote.PolicyID,
		DiscountSource: quote.Source,
		PriceSnapshot:  snapshotJSON(quote.Snapshot),
		Status:         ordermodel.OrderStatusPaid,
		PayMethod:      "balance",
		PayTime:        &now,
		OperatorID:     actorID, // 真实操作人（子账号下单可追溯，P4-09）
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}

	// 2.2) 补写订单项：行级保留原价/优惠/实付，否则 order_items 折扣字段永远为 0（P5-04）。
	// spec_code 落 SKU 编码（T4.1），供履约按 SKU 取规格原子与平台参数。
	if s.orderItems != nil {
		itemPrice := product.Price
		if spec != nil {
			itemPrice = spec.Price
		}
		item := &ordermodel.OrderItem{
			OrderID:        order.ID,
			ProductID:      product.ID,
			ProductName:    product.Name,
			SpecCode:       specCode,
			Specs:          specsSnapshot,
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

	// 2.5) 累计消费 + 等级重算（P3-03）：已扣款即计入，开通失败也不回退。
	//      装配层注入的实现负责异步化与错误日志，不阻塞下单主流程。
	if s.consumeRecorder != nil {
		s.consumeRecorder(userID, amount)
	}

	// 3) 触发上游开通（orderService.Activate：paid → 创建实例 → active）
	if err := s.orderSvc.Activate(ctx, order.ID); err != nil {
		// 已扣款，订单保持 paid，允许用户稍后重试；库存由 defer 回补。
		info := fromAdminOrder(*order, nil)
		info.Status = ordermodel.OrderStatusPaid
		return &info, ErrProvisionFailed
	}
	orderSucceeded = true
	info := fromAdminOrder(*order, nil)
	info.Status = ordermodel.OrderStatusActive
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
	for _, it := range items {
		resp.Items = append(resp.Items, fromAdminOrder(it, actorNames))
	}
	return resp, nil
}

// resolveActorNames 批量解析订单操作人用户名；失败时降级为空映射（不阻断列表）。
func (s *orderService) resolveActorNames(ctx context.Context, items []ordermodel.Order) map[uint64]string {
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
func fromAdminOrder(o ordermodel.Order, actorNames map[uint64]string) dto.OrderInfo {
	return dto.OrderInfo{
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
		OriginalAmount: o.OriginalAmount,
		DiscountAmount: o.DiscountAmount,
		FinalAmount:    o.FinalAmount,
		DiscountSource: o.DiscountSource,
		CreatedAt:      o.CreatedAt.Format(time.RFC3339),
	}
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
	quote, err := s.resolveQuote(ctx, userID, product, specCode, qty)
	if err != nil {
		return nil, err
	}
	return &dto.QuoteInfo{
		SpecCode:       specCode,
		OriginalAmount: quote.OriginalAmount,
		DiscountAmount: quote.DiscountAmount,
		FinalAmount:    quote.FinalAmount,
		PolicyID:       quote.PolicyID,
		Source:         quote.Source,
		Snapshot:       quote.Snapshot,
	}, nil
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

// genOrderNo 生成订单号：时间戳 + 随机。
func genOrderNo(userID uint64) string {
	return fmt.Sprintf("UC%d%d%04d", userID, time.Now().Unix(), rand.Intn(10000))
}
