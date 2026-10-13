// 订单域装配——后台代下单：规格/周期/选配读取、算价与退款实例钩子。

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	instancedto "hostsent/backend/internal/modules/admin/instance/dto"
	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	matrixservice "hostsent/backend/internal/modules/admin/product/pricing/service"
	userdto "hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/pricing"
)

// buildRefundInstanceHook 组装退款审核通过后的实例侧处置钩子（doc61 §8 第 2 条）。
//
// 钩子只做转译：把订单侧的基础类型搬成实例域的入参，规则判断（策略/全额判定）在
// 实例服务内完成。失败只记日志 —— 退款审核结果不因实例侧问题回滚。
func buildRefundInstanceHook(svc instanceservice.InstanceService, logger *zap.Logger) orderservice.OrderInstanceRefundHook {
	return func(ctx context.Context, orderID uint64, orderNo string, userID uint64, refundAmount, paidAmount float64, fullRefund bool, refundNo string) error {
		if svc == nil {
			return nil
		}
		err := svc.DisposeOnRefund(ctx, &instancedto.RefundInstanceRequest{
			OrderID:      orderID,
			OrderNo:      orderNo,
			UserID:       userID,
			RefundNo:     refundNo,
			RefundAmount: refundAmount,
			PaidAmount:   paidAmount,
			FullRefund:   fullRefund,
		})
		if err != nil && logger != nil {
			logger.Warn("instance refund dispose failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo),
				zap.String("refund_no", refundNo), zap.Error(err))
		}
		return err
	}
}

// ===== 后台代下单：规格/周期/选配项读取与算价（与用户端下单同口径）=====

// adminOrderCycle 归一化后台代下单的计费周期：
//   - 显式指定：非法值直接拒绝（避免静默落到默认档，按错价成交）；
//   - 留空：回落商品 price_model 对应周期（存量行为不变）。
func adminOrderCycle(priceModel, raw string) (string, error) {
	cycle := strings.TrimSpace(raw)
	if cycle == "" {
		return billingcycle.NormalizeOrDefault(priceModel), nil
	}
	if billingcycle.IsValid(cycle) {
		return cycle, nil
	}
	if normalized := billingcycle.Normalize(cycle); normalized != "" {
		return normalized, nil
	}
	return "", fmt.Errorf("不支持的计费周期：%s", raw)
}

// adminOrderSpec 解析后台代下单的 SKU：商品未挂规格返回 nil（按商品级下单）；
// 挂了规格则必须显式选择，避免误用商品级价格与商品级规格开通。
func adminOrderSpec(ctx context.Context, catalog catalogservice.ProductService, productID uint64, specCode string) (*catalogdto.ProductSpecInfo, error) {
	specs, err := catalog.ListSpecs(ctx, productID)
	if err != nil {
		return nil, err
	}
	enabled := make([]catalogdto.ProductSpecInfo, 0, len(specs))
	for _, sp := range specs {
		if sp.Status == 1 {
			enabled = append(enabled, sp)
		}
	}
	if len(enabled) == 0 {
		if len(specs) > 0 {
			return nil, errors.New("该商品规格已全部下架，暂不可下单")
		}
		return nil, nil
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

// normalizeRenewalPrice 归一化随单绑定的续费价：nil 或 <=0 视为不绑定（返回 nil）。
// 负价/零价在续费侧没有意义（0 元续费会让续费单被算价守卫拒绝），
// 因此统一收敛成"未绑定"，避免把一个不可能成交的价格写进实例。
func normalizeRenewalPrice(price *float64) *float64 {
	if price == nil || *price <= 0 {
		return nil
	}
	v := adminRound2(*price)
	if v <= 0 {
		return nil
	}
	return &v
}

// orderRenewalPrice 取订单上的续费价绑定（已归一，nil 表示未绑定）。
func orderRenewalPrice(order *ordermodel.Order) *float64 {
	if order == nil {
		return nil
	}
	return normalizeRenewalPrice(order.RenewalPrice)
}

// adminOrderQuote 后台代下单算价：手填价（price>0）走高优先级的 ManualAmount，
// 否则走统一算价管线（周期矩阵 → SKU 价 → 折扣 → 选配加价）。
func (a *asm) adminOrderQuote(ctx context.Context, userID uint64, product *catalogdto.ProductInfo, specCode, cycle string, qty int, selections map[string]string, manualPrice float64) (*pricing.Quote, error) {
	if a.pricePipeline == nil {
		// 未装配算价管线：按商品单价估算，手填价直接生效（降级路径）。
		amount := adminRound2(product.Price * float64(qty))
		if manualPrice > 0 {
			amount = adminRound2(manualPrice)
		}
		return &pricing.Quote{OriginalAmount: amount, FinalAmount: amount, Snapshot: []pricing.Rule{}}, nil
	}
	in := pricing.ResolveInput{
		UserID:           userID,
		ProductID:        product.ID,
		SpecCode:         specCode,
		Cycle:            cycle,
		Quantity:         qty,
		ConfigSelections: selections,
	}
	if manualPrice > 0 {
		in.ManualAmount = &manualPrice
	}
	return a.pricePipeline.Resolve(ctx, in)
}

// buildAdminOrderQuote 代下单算价预览：按所选规格/周期/选配项跑一遍下单时的算价，
// 但不落库、不扣款、不动库存（与用户端 POST /uc/orders/quote 同性质）。
//
// 选配项先经 ResolveOptionPricing 归一（补齐必选默认值、拒绝非法取值），
// 再交给算价管线，这样预览值就是把选配加价算进去后的真实基准价。
func (a *asm) buildAdminOrderQuote(ctx context.Context, userID uint64, req userdto.AdminOrderQuoteRequest) (*userdto.AdminOrderQuoteResponse, error) {
	product, err := a.prodCatalogService.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, err
	}
	cycle, err := adminOrderCycle(product.PriceModel, req.BillingCycle)
	if err != nil {
		return nil, err
	}
	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}
	spec, err := adminOrderSpec(ctx, a.prodCatalogService, product.ID, req.SpecCode)
	if err != nil {
		return nil, err
	}
	specCode := ""
	if spec != nil {
		specCode = spec.SpecCode
	}
	pricingResult, err := a.prodCatalogService.ResolveOptionPricing(ctx, product.ID, cycle, req.ConfigSelections)
	if err != nil {
		return nil, err
	}
	selections := map[string]string{}
	if pricingResult != nil {
		selections = pricingResult.Selections
	}
	quote, err := a.adminOrderQuote(ctx, userID, product, specCode, cycle, qty, selections, 0)
	if err != nil {
		return nil, err
	}
	return &userdto.AdminOrderQuoteResponse{
		OriginalAmount: quote.OriginalAmount,
		DiscountAmount: quote.DiscountAmount,
		FinalAmount:    quote.FinalAmount,
		OptionAmount:   quote.OptionAmount,
		DiscountSource: quote.Source,
		Currency:       "CNY",
	}, nil
}

// buildAdminOrderConfig 组装后台代下单可选项：SKU、可售周期、客户选配项与上游固定配置。
func buildAdminOrderConfig(ctx context.Context, catalog catalogservice.ProductService, matrix matrixservice.PriceMatrixService, productID uint64) (*userdto.AdminOrderConfigResponse, error) {
	product, err := catalog.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	resp := &userdto.AdminOrderConfigResponse{
		ProductID:  product.ID,
		Name:       product.Name,
		PriceModel: product.PriceModel,
		Price:      product.Price,
		SourceMode: product.SourceMode,
		Specs:      []userdto.AdminOrderConfigSpec{},
		Options:    []userdto.AdminOrderConfigOption{},
	}
	var productCycles []string
	cyclesBySpec := map[uint64][]string{}
	if matrix != nil {
		if pc, bySpec, cerr := matrix.SellableCycles(ctx, productID); cerr == nil {
			productCycles = pc
			cyclesBySpec = bySpec
		}
	}
	resp.Cycles = productCycles
	if specs, serr := catalog.ListSpecs(ctx, productID); serr == nil {
		for _, sp := range specs {
			if sp.Status != 1 {
				continue
			}
			cycles := cyclesBySpec[sp.ID]
			if len(cycles) == 0 {
				cycles = productCycles
			}
			resp.Specs = append(resp.Specs, userdto.AdminOrderConfigSpec{
				SpecCode:   sp.SpecCode,
				Name:       sp.Name,
				Price:      sp.Price,
				PriceModel: sp.PriceModel,
				Stock:      sp.Stock,
				Cycles:     cycles,
				Specs:      sp.Specs,
			})
		}
	}
	if groups, oerr := catalog.CustomerOptions(ctx, productID); oerr == nil {
		for _, g := range groups {
			for _, o := range g.Options {
				opt := userdto.AdminOrderConfigOption{
					OptionKey:    o.OptionKey,
					Name:         o.Name,
					Widget:       o.Widget,
					Required:     o.Required,
					DefaultValue: o.DefaultValue,
					Unit:         o.Unit,
					Help:         o.Help,
					MinValue:     o.MinValue,
					MaxValue:     o.MaxValue,
					UnitPrice:    o.UnitPrice,
					Values:       []userdto.AdminOrderConfigOptionVal{},
				}
				for _, v := range o.Values {
					opt.Values = append(opt.Values, userdto.AdminOrderConfigOptionVal{
						Value:          v.Value,
						Label:          v.Label,
						Group:          v.Group,
						Help:           v.Help,
						Default:        v.Default,
						PriceMonthly:   v.PriceMonthly,
						PriceQuarterly: v.PriceQuarterly,
						PriceAnnually:  v.PriceAnnually,
						PriceOnetime:   v.PriceOnetime,
					})
				}
				resp.Options = append(resp.Options, opt)
			}
		}
	}
	// 上游固定配置只读展示：锁定项（单值）不在客户可选项里，但客服需要看到「这台机器
	// 定死的是什么配置」，否则会以为漏了配置项而反复尝试修改。
	if product.Upstream != nil {
		up := &userdto.AdminOrderConfigUpstream{
			FixedConfig: product.Upstream.FixedConfig,
			Renewable:   product.Upstream.Renewable,
			RenewNote:   product.Upstream.RenewNote,
			Options:     []userdto.AdminOrderConfigUpstreamOption{},
		}
		for _, o := range product.Upstream.Options {
			up.Options = append(up.Options, userdto.AdminOrderConfigUpstreamOption{
				Key: o.Key, Name: o.OptionName, Locked: o.Locked, Value: o.Value,
			})
		}
		resp.Upstream = up
	}
	return resp, nil
}

// adminSelectionsJSON 选配项选择序列化为 jsonb 文本；空选择落空对象（jsonb 不接受空串）。
func adminSelectionsJSON(selections map[string]string) string {
	if len(selections) == 0 {
		return "{}"
	}
	b, err := json.Marshal(selections)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// adminSnapshotJSON 折扣快照序列化为 jsonb 文本；无明细落空数组。
func adminSnapshotJSON(rules []pricing.Rule) string {
	if len(rules) == 0 {
		return "[]"
	}
	b, err := json.Marshal(rules)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// adminRound2 金额保留两位小数（与算价管线同口径）。
func adminRound2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
