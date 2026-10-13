// 订单域装配（批次四 4b；含自 assembly.go 与 server.go 尾部迁入的履约适配器）。

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	finaccountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	referralservice "hostsent/backend/internal/modules/admin/finance/referral/service"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	instancedto "hostsent/backend/internal/modules/admin/instance/dto"
	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	orderhandler "hostsent/backend/internal/modules/admin/order/handler"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderrepo "hostsent/backend/internal/modules/admin/order/repository"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	matrixservice "hostsent/backend/internal/modules/admin/product/pricing/service"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
	salesservice "hostsent/backend/internal/modules/admin/system/sales/service"
	userdto "hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	openservice "hostsent/backend/internal/modules/open/service"
	ucorderhandler "hostsent/backend/internal/modules/uc/order/handler"
	ucorderservice "hostsent/backend/internal/modules/uc/order/service"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/observability"
	"hostsent/backend/internal/pkg/pricing"
	"hostsent/backend/internal/pkg/safego"
	"hostsent/backend/internal/pkg/specatom"
	"hostsent/backend/internal/pkg/upstream"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"
)

// buildOrder 构造订单服务（履约适配器/返现提成钩子/销售归属）与用户中心下单、后台代下单。
func (a *asm) buildOrder() {
	// 订单履约：上游开通适配器（打通订单 paid/provisioning → 上游创建实例）
	a.orderService = orderservice.NewOrderService(a.orderRepo, a.orderItemRepo, a.orderRefundRepo, orderservice.NewUpstreamProvisionAdapter(buildOrderProvisionDeps(a.prodCatalogService, a.providerService, a.upstreamMgr, a.syncRepo, a.openEventPublisher, a.orderRepo, a.configValueReader, a.logger)))
	// 异步开通（T5.1）：下单/后台重试只投递任务，工作池领取后执行上游开通（约 50s > write_timeout）。
	a.provisionEnqueuer = orderservice.NewProvisionEnqueuer(a.provisionTaskRepo, a.logger)
	a.orderService.SetProvisionQueue(a.provisionEnqueuer)
	// 订单列表/详情展示开通履约状态与失败原因（manual=需人工处理，如上游余额不足）。
	a.orderService.SetProvisionTaskReader(a.provisionTaskRepo.FindByOrderIDs)
	a.orderService.SetProductSourceModeResolver(buildProductSourceModeResolver(a.prodCatalogService))
	// 订单开通成功 → 给邀请人计提返现 + 给归属销售计提提成（各自失败只记日志，不影响订单）。
	// SetCashbackHook 是单槽 setter：两者必须在同一个闭包里串行调用，分别 Set 会互相覆盖。
	a.orderService.SetCashbackHook(func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, baseAmount float64, isRenewal bool) error {
		if err := a.referralSvc.AccrueForOrder(ctx, referralservice.AccrualInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID, BaseAmount: baseAmount, IsRenewal: isRenewal,
		}); err != nil {
			a.logger.Warn("referral cashback accrual failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo),
				zap.Uint64("user_id", buyerUserID), zap.Error(err))
		}
		if err := a.salesBundle.commissionService.AccrueForOrder(ctx, salesservice.AccrualInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID, BaseAmount: baseAmount, IsRenewal: isRenewal,
		}); err != nil {
			a.logger.Warn("sales commission accrual failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo), zap.Error(err))
		}
		return nil
	})
	// 退款审核通过 → 按退款额占比冲减邀请人返现与销售提成（失败只记日志，不影响退款）。
	a.orderService.SetRefundHook(func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, paidAmount, refundAmount float64, refundNo string) error {
		if err := a.referralSvc.ClawbackForRefund(ctx, referralservice.ClawbackInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID,
			PaidAmount: paidAmount, RefundAmount: refundAmount, RefundNo: refundNo,
		}); err != nil {
			a.logger.Warn("referral cashback clawback failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo),
				zap.String("refund_no", refundNo), zap.Error(err))
		}
		if err := a.salesBundle.commissionService.ClawbackForRefund(ctx, salesservice.ClawbackInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID,
			PaidAmount: paidAmount, RefundAmount: refundAmount, RefundNo: refundNo,
		}); err != nil {
			a.logger.Warn("sales commission clawback failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo),
				zap.String("refund_no", refundNo), zap.Error(err))
		}
		return nil
	})
	// 销售归属快照：下单/续费瞬间锁定（doc86 §3.4），改派不影响已成交订单。
	a.orderService.SetSalesOwnerResolver(a.salesBundle.newOrderOwnerPort())
	a.orderHandler = orderhandler.NewOrderHandler(a.orderService)
	a.userHandler = handler.NewUserHandler(a.userService,
		// 为用户创建订单：余额支付并开通，或仅创建待支付。
		// 与用户端下单同口径：按 SKU + 周期 + 客户选配项计价（价格矩阵/折扣/选配加价），
		// 客服可按需手填实付价（ManualAmount，最高优先级）。此前该入口只传商品与单价，
		// 代下单开出来的机器拿不到规格与选配，与用户自助购买不一致。
		func(ctx context.Context, userID uint64, req userdto.AdminCreateOrderRequest, operatorID uint64) (*userdto.AdminOrderBrief, error) {
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
			// SKU 解析：挂了规格的商品必须显式选规格，与用户端一致（否则会按商品级价与
			// 商品级规格开通，SKU 库存/绑定参数全部丢失）。
			spec, err := adminOrderSpec(ctx, a.prodCatalogService, product.ID, req.SpecCode)
			if err != nil {
				return nil, err
			}
			specCode := ""
			if spec != nil {
				specCode = spec.SpecCode
			}
			// 选配项校验与归一（T4.5）：补齐必选默认值，非法取值直接拒绝。
			pricingResult, err := a.prodCatalogService.ResolveOptionPricing(ctx, product.ID, cycle, req.ConfigSelections)
			if err != nil {
				return nil, err
			}
			selections := map[string]string{}
			if pricingResult != nil {
				selections = pricingResult.Selections
			}
			// 算价：手填价走 ManualAmount（最高优先级），否则走统一算价管线。
			quote, err := a.adminOrderQuote(ctx, userID, product, specCode, cycle, qty, selections, req.Price)
			if err != nil {
				return nil, err
			}
			price := quote.FinalAmount
			if price <= 0 {
				return nil, errors.New("算价结果无效，请检查商品定价")
			}
			// 库存扣减（有限库存的 SKU）：下单失败原路回补，避免丢库存。
			stockDeducted := false
			if spec != nil && spec.Stock >= 0 {
				affected, derr := a.prodCatalogService.DecrementSpecStock(ctx, spec.ID, qty)
				if derr != nil {
					return nil, derr
				}
				if affected == 0 {
					return nil, errors.New("所选规格库存不足")
				}
				stockDeducted = true
			}
			orderSucceeded := false
			defer func() {
				if stockDeducted && !orderSucceeded {
					_, _ = a.prodCatalogService.IncrementSpecStock(context.WithoutCancel(ctx), spec.ID, qty)
				}
			}()

			payMode := req.PayMode
			if payMode == "" {
				payMode = "create"
			}
			specsSnapshot := product.Specs
			if spec != nil && strings.TrimSpace(spec.Specs) != "" {
				specsSnapshot = spec.Specs
			}
			now := time.Now()
			order := &ordermodel.Order{
				OrderNo:        fmt.Sprintf("AO%d%d%04d", userID, now.Unix(), rand.Intn(10000)),
				UserID:         userID,
				ProductID:      product.ID,
				ProductName:    product.Name,
				Specs:          specsSnapshot,
				SpecCode:       specCode,
				ConfigOptions:  adminSelectionsJSON(selections),
				Quantity:       qty,
				PriceModel:     product.PriceModel,
				Cycle:          cycle,
				TotalAmount:    quote.OriginalAmount,
				OriginalAmount: quote.OriginalAmount,
				DiscountAmount: quote.DiscountAmount,
				FinalAmount:    quote.FinalAmount,
				PricePolicyID:  quote.PolicyID,
				DiscountSource: quote.Source,
				PriceSnapshot:  adminSnapshotJSON(quote.Snapshot),
				Status:         ordermodel.OrderStatusPending,
				// 代下单的操作人留痕：orders.operator_id 是「最近操作人」，
				// 后台客服代客下单时指向管理员，用户自助下单为 0。
				OperatorID: operatorID,
			}
			// 后台代下单同样锁定销售归属快照（doc86 §3.4）：代客成交也计入销售的业绩与提成。
			order.SalesAdminID = a.salesBundle.customerService.ResolveForNewOrder(ctx, userID)
			// 余额支付：扣款 + 标记已支付 + 投递开通任务（T5.1 异步履约）。
			if payMode == "balance" {
				if _, err := a.walletService.Adjust(ctx, accountdto.AdjustRequest{
					UserID: userID, Type: transmodel.TxTypeAdminOrder, Direction: -1, Amount: price,
					BizKey: fmt.Sprintf("admin-order-%d", now.UnixNano()),
					Remark: fmt.Sprintf("后台为用户下单：%s（%s）", product.Name, cycle),
				}, userID); err != nil {
					if errors.Is(err, finaccountservice.ErrInsufficientBalance) {
						return nil, errors.New("用户余额不足")
					}
					return nil, err
				}
				order.Status = ordermodel.OrderStatusPaid
				order.PayMethod = "balance"
				order.PaidAmount = price
				order.PayTime = &now
			}
			if err := a.orderRepo.Create(ctx, order); err != nil {
				return nil, err
			}
			orderSucceeded = true
			// 订单项补写（与用户端同口径）：行级保留规格/选配/折扣快照，
			// 否则订单详情与后续对账看不到买的到底是什么配置。
			if a.orderItemRepo != nil {
				item := &ordermodel.OrderItem{
					OrderID:        order.ID,
					ProductID:      product.ID,
					ProductName:    product.Name,
					SpecCode:       specCode,
					Specs:          specsSnapshot,
					ConfigOptions:  adminSelectionsJSON(selections),
					Cycle:          cycle,
					Price:          adminRound2(quote.OriginalAmount / float64(qty)),
					Quantity:       qty,
					Amount:         price,
					OriginalAmount: quote.OriginalAmount,
					DiscountAmount: quote.DiscountAmount,
					FinalAmount:    quote.FinalAmount,
				}
				if err := a.orderItemRepo.Create(ctx, item); err != nil {
					return nil, err
				}
			}
			brief := &userdto.AdminOrderBrief{
				ID: order.ID, OrderNo: order.OrderNo, ProductName: product.Name,
				BillingCycle: cycle, TotalAmount: price, Status: order.Status, PayMethod: order.PayMethod,
			}
			if payMode == "balance" {
				// 异步开通：投递任务即返回 paid，前端/后台按订单状态轮询（T5.1）。
				if err := a.provisionEnqueuer.EnqueueProvision(ctx, order, product.SourceMode); err == nil {
					brief.Status = ordermodel.OrderStatusPaid
				}
			}
			return brief, nil
		},
	)
	// 代下单可选项读取（规格/周期/选配项）：与用户端商品详情同源，后台不另维护口径。
	a.userHandler.SetOrderConfigReader(func(ctx context.Context, productID uint64) (*userdto.AdminOrderConfigResponse, error) {
		return buildAdminOrderConfig(ctx, a.prodCatalogService, a.priceMatrixService, productID)
	})
	a.refundHandler = orderhandler.NewRefundHandler(a.orderService)
	// 用户中心订单：余额支付下单 + 复用履约适配器开通上游
	// ensureOpenable：下单前校验商品能否直连开通，财务型上游（账单推送制）不支持单次开通，先拒绝避免误扣款。
	a.ucOrderService = ucorderservice.NewOrderService(a.prodCatalogService, a.walletService, a.orderRepo, a.orderService,
		buildEnsureOpenable(a.prodCatalogService, a.providerService),
		func(err error) bool { return errors.Is(err, finaccountservice.ErrInsufficientBalance) },
		func(userID uint64, amount float64) {
			// 异步执行：不阻塞下单；使用独立 context，避免请求结束后被取消。
			go func() {
				defer safego.Recover(a.logger)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if err := a.levelUpgradeService.ApplyConsume(ctx, userID, amount); err != nil {
					a.logger.Warn("user level: apply consume failed",
						zap.Uint64("user_id", userID), zap.Float64("amount", amount), zap.Error(err))
				}
			}()
		},
		a.userRepo,           // 订单「操作人」列：批量解析 operator_id → 用户名（P4-09）
		a.orderItemRepo,      // 订单项落库：行级折扣快照（P5-04）
		a.pricePipeline,      // 统一算价管线（P5-03/P5-04）
		a.prodCatalogService, // SKU 读取与库存增减（T4.1，按 SKU 下单）
		a.provisionEnqueuer,  // 异步开通任务投递（T5.1）
		a.provisionTaskRepo,  // 开通任务状态查询（订单列表轮询展示）
	)
	// 销售归属快照（doc86 §3.4）：用户中心下单瞬间锁定归属。
	a.ucOrderService.SetSalesOwnerResolver(a.salesBundle.newOrderOwnerPort())
	a.ucOrderHandler = ucorderhandler.NewOrderHandler(a.ucOrderService)
}

// buildOrderProvisionDeps 组装订单履约的上游开通适配器依赖（打通订单 paid/provisioning → 上游创建实例）。
// events 为开放平台事件发布器（P6/T6.5，可为 nil）：开通成功落库后发布 instance.created。
func buildOrderProvisionDeps(
	catalog catalogservice.ProductService,
	provider providerservice.ProviderService,
	upmgr *upstream.ProviderManager,
	syncRepo syncrepo.SyncRepository,
	events *openservice.EventPublisher,
	orderRepo orderrepo.OrderRepository,
	configValueReader func(ctx context.Context, key string) (string, bool, error),
	logger *zap.Logger,
) orderservice.ProvisionDeps {
	return orderservice.ProvisionDeps{
		Logger: logger,
		// 上游账单 id 落库（开通幂等）：账单生成即记，重试复用同一张，不再重复下单。
		RecordUpstreamInvoice: func(ctx context.Context, orderID uint64, invoiceID int64) error {
			if invoiceID <= 0 {
				return nil
			}
			return orderRepo.SetUpstreamInvoiceID(ctx, orderID, uint64(invoiceID))
		},
		// 上游对接令牌落库（首次履约生成后固化）：上游据此反查既有账单/主机。
		RecordDownstreamToken: orderRepo.SetDownstreamToken,
		// 本站对外地址（上游握手标识）：运营可在系统配置 site.public_url 填真实地址。
		PublicURL: func(ctx context.Context) string {
			if raw, ok, err := configValueReader(ctx, "site.public_url"); err == nil && ok {
				return raw
			}
			return ""
		},
		BuildProvisionRequest: func(ctx context.Context, productID uint64, name, specSnapshot, specCode string, selections map[string]string) (interface{}, error) {
			return catalog.BuildProvisionRequest(ctx, productID, name, specSnapshot, specCode, selections)
		},
		BuildProviderConfig: provider.BuildProviderConfig,
		CreateInstance: func(ctx context.Context, cfg *upstream.ProviderConfig, req *pkgmodel.CreateInstanceRequest) (*pkgmodel.StandardInstance, error) {
			provider, err := upmgr.Build(cfg.Type, cfg)
			if err != nil {
				return nil, err
			}
			provisioner, ok := provider.(upstream.InstanceProvisioning)
			if !ok {
				return nil, errors.New("上游不支持开通操作")
			}
			var inst *pkgmodel.StandardInstance
			err = observability.Timed("upstream_create_instance", func() error {
				var cerr error
				inst, cerr = provisioner.CreateInstance(ctx, req)
				return cerr
			})
			if err != nil {
				return nil, err
			}
			return inst, nil
		},
		RecordInstance: func(ctx context.Context, inst *pkgmodel.StandardInstance, order *ordermodel.Order) error {
			// 链路判据（D6）：实例归属由商品 source_mode 决定，而非"经订单开通即自营"
			// ——代理商品（upstream）经订单开通后同样是上游链路实例，否则续费会被
			// markRenewalSuccess 误判为链路 B 而静默本地顺延。
			sourceMode := ""
			var upstreamProductID uint64
			if product, perr := catalog.FindByID(ctx, order.ProductID); perr == nil && product != nil {
				sourceMode = product.SourceMode
				upstreamProductID = product.SourceProductID
			}
			row := buildRecordedInstance(inst, order, sourceMode, upstreamProductID)
			if row.InstanceID == "" {
				return nil
			}
			if err := syncRepo.UpsertInstances(ctx, []syncmodel.Instance{row}); err != nil {
				return err
			}
			// 开放平台 instance.created（P6/T6.5）：仅对代客下单订单发布，事件只落库不阻塞履约。
			if order.OpenAppID != 0 {
				events.PublishToOwner(ctx, order.UserID, openservice.EventInstanceCreated, map[string]any{
					"instance_id":     row.ID,
					"mark":            row.InstanceID,
					"name":            row.Name,
					"status":          row.Status,
					"order_no":        order.OrderNo,
					"customer_ref":    order.ChannelCustomerRef,
					"sell_product_id": order.ProductID,
				})
			}
			return nil
		},
		HasInstance: func(ctx context.Context, userID, productID uint64) (bool, error) {
			n, err := syncRepo.CountInstancesByProductUser(ctx, userID, productID)
			return n > 0, err
		},
	}
}

// buildEnsureOpenable 返回下单前校验函数：校验商品能否直连开通，
// 财务型上游等不支持单次开通的场景先行拒绝，避免误扣款。

// buildEnsureOpenable 返回下单前校验函数：校验商品能否直连开通，
// 财务型上游等不支持单次开通的场景先行拒绝，避免误扣款。
func buildEnsureOpenable(catalog catalogservice.ProductService, provider providerservice.ProviderService) func(ctx context.Context, productID uint64) error {
	return func(ctx context.Context, productID uint64) error {
		preq, err := catalog.BuildProvisionRequest(ctx, productID, "", "", "", nil)
		if err != nil {
			return err
		}
		if preq == nil || preq.ProviderID == 0 {
			return errors.New("该商品未绑定上游，暂不可在线购买")
		}
		cfg, err := provider.BuildProviderConfig(ctx, preq.ProviderID)
		if err != nil {
			return errors.New("该商品上游配置不可用，暂不可购买")
		}
		// 财务型上游（zjmf）现已支持下游下单开通，不再拦截。
		_ = cfg
		return nil
	}
}

// buildProviderResolver 返回用户中心主机模块的 ProviderResolver（按提供商 ID 实例化上游适配器）。

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

// buildRefundPolicyReader 读取退款后实例处置策略（lifecycle_policies.refund_action）。
// 读取失败或值为空一律回落 none：处置线上实例是不可逆动作，配置不可用时必须选「不动作」。
// buildProductSourceModeResolver 返回商品链路判据解析（T5.1 任务留痕用）：
// 读不到商品时返回空串，不影响开通投递。
func buildProductSourceModeResolver(catalog catalogservice.ProductService) func(ctx context.Context, productID uint64) string {
	return func(ctx context.Context, productID uint64) string {
		product, err := catalog.FindByID(ctx, productID)
		if err != nil || product == nil {
			return ""
		}
		return product.SourceMode
	}
}

// provisionManualNotifier 桥接开通任务连续失败告警到通知中心（T5.1）。
// 复用 system 事件避免为一个提示新增模板（与调价待确认一致）。

// buildRecordedInstance 将标准实例转换为同步实例记录（订单履约开通成功后落库）。
// sourceMode 为商品链路判据（D6），upstreamProductID 为代理商品对应的上游资源商品 ID。
func buildRecordedInstance(inst *model.StandardInstance, order *ordermodel.Order, sourceMode string, upstreamProductID uint64) syncmodel.Instance {
	rawJSON, _ := json.Marshal(inst.RawData)
	// 规格回落（T4.1）：上游详情拉取失败时适配器只返回最小实例（规格为空），
	// 此时用订单规格快照（SKU 原子取值 JSON）补齐，保证实例列表能显示下单所选配置。
	specs := inst.Specs
	if specs.CPU == 0 && specs.Memory == 0 && specs.Disk == 0 {
		if snapshot := specsFromOrder(order); snapshot != nil && (snapshot.CPU > 0 || snapshot.Memory > 0 || snapshot.Disk > 0) {
			specs = *snapshot
		}
	}
	if sourceMode == "" {
		sourceMode = syncmodel.SourceModeSelf
	}
	row := syncmodel.Instance{
		InstanceID: inst.UpstreamID,
		ProviderID: uint64(inst.ProviderID),
		UserID:     order.UserID,
		Name:       firstNonEmpty(inst.Name, order.ProductName),
		CPU:        specs.CPU,
		Memory:     specs.Memory,
		Disk:       specs.Disk,
		DiskType:   specs.DiskType,
		Bandwidth:  specs.Bandwidth,
		OS:         specs.OS,
		Region:     firstNonEmpty(inst.Region, specs.Region),
		Zone:       firstNonEmpty(inst.Zone, specs.Zone),
		Status:     string(inst.Status),
		PublicIP:   inst.PublicIP,
		PrivateIP:  inst.PrivateIP,
		RawData:    string(rawJSON),
		// 计费周期优先取订单 cycle（doc25）：实例账期推进与续费按真实周期走。
		BillingMode: firstNonEmpty(order.Cycle, order.PriceModel),
		// 子账号下单时 order.OperatorID 为真实操作人，落到实例「操作人」列（P4-09）。
		ActorUserID: order.OperatorID,
		// 记录来源订单，实例运维台据此展示关联订单（见 61 实施计划 §4.1）。
		OrderID: order.ID,
		// 双链路语义（P1/T1.2，D6）：判据取自商品 source_mode，而非"是否经订单开通"
		//——代理商品（upstream）经订单开通后仍是上游链路实例，续费/暂停/销毁须调上游。
		// sell_product_id 恒为我方售出的 products.id（续费算价与展示的权威来源）；
		// 上游链路另记 upstream_product_id（resource_products.id）便于对账与回溯源。
		SourceMode:         sourceMode,
		SellProductID:      order.ProductID,
		UpstreamProductID:  upstreamProductID,
		ProviderInstanceID: inst.UpstreamID,
	}
	if !inst.ExpireAt.IsZero() {
		exp := inst.ExpireAt
		row.ExpireAt = &exp
	} else if base, cycles, ok := orderBillingPeriod(order); ok {
		// 上游未返回账期时按订单周期本地起算（自营平台普遍不返回 expire_at）。
		// 没有这一步，实例的 expire_at 恒为 NULL，生命周期推进器（到期暂停/宽限/销毁）
		// 永远筛不到它——"账单计费→到期暂停"这条链路就是断的。
		exp := billingcycle.Advance(base, cycles, 1)
		row.ExpireAt = &exp
	}
	return row
}

// orderBillingPeriod 由订单推出计费起算时间与周期编码：
// 起算时间取支付时间（缺失回退下单时间），周期取 cycle（缺失回退 price_model）。
// 识别不出周期或一次性买断（onetime）时不产生账期，交由上游/人工处理。

// orderBillingPeriod 由订单推出计费起算时间与周期编码：
// 起算时间取支付时间（缺失回退下单时间），周期取 cycle（缺失回退 price_model）。
// 识别不出周期或一次性买断（onetime）时不产生账期，交由上游/人工处理。
func orderBillingPeriod(order *ordermodel.Order) (time.Time, string, bool) {
	cycle := order.Cycle
	if cycle == "" {
		cycle = order.PriceModel
	}
	code := billingcycle.Normalize(cycle)
	if !billingcycle.IsValid(code) || code == billingcycle.Onetime {
		return time.Time{}, "", false
	}
	base := order.CreatedAt
	if order.PayTime != nil {
		base = *order.PayTime
	}
	if base.IsZero() {
		return time.Time{}, "", false
	}
	return base, code, true
}

// specsFromOrder 解析订单的规格快照（SKU 原子取值 JSON）为标准规格；解析失败返回 nil。

// specsFromOrder 解析订单的规格快照（SKU 原子取值 JSON）为标准规格；解析失败返回 nil。
func specsFromOrder(order *ordermodel.Order) *model.StandardProductSpec {
	raw := strings.TrimSpace(order.Specs)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	spec := specatom.FromMap(m)
	return &spec
}

// firstNonEmpty 返回第一个非空字符串。
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
