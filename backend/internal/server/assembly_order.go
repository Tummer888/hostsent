// 订单域装配（批次四 4b；含自 assembly.go 与 server.go 尾部迁入的履约适配器）。

package server

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	finaccountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	referralservice "hostsent/backend/internal/modules/admin/finance/referral/service"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	orderhandler "hostsent/backend/internal/modules/admin/order/handler"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderrepo "hostsent/backend/internal/modules/admin/order/repository"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	salesservice "hostsent/backend/internal/modules/admin/system/sales/service"
	userdto "hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	ucorderdto "hostsent/backend/internal/modules/uc/order/dto"
	ucorderhandler "hostsent/backend/internal/modules/uc/order/handler"
	ucorderservice "hostsent/backend/internal/modules/uc/order/service"
	"hostsent/backend/internal/pkg/storage"
)

// buildOrder 构造订单服务（履约适配器/返现提成钩子/销售归属）与用户中心下单、后台代下单。
func (a *asm) buildOrder() {
	// 订单履约：上游开通适配器（打通订单 paid/provisioning → 上游创建实例）
	a.orderService = orderservice.NewOrderService(a.orderRepo, a.orderItemRepo, a.orderRefundRepo, orderservice.NewUpstreamProvisionAdapter(buildOrderProvisionDeps(a.prodCatalogService, a.providerService, a.upstreamMgr, a.syncRepo, a.openEventPublisher, a.orderRepo, a.configValueReader, a.logger, a.cfg.App.EncryptKey)))
	// 异步开通（T5.1）：下单/后台重试只投递任务，工作池领取后执行上游开通（约 50s > write_timeout）。
	a.provisionEnqueuer = orderservice.NewProvisionEnqueuer(a.provisionTaskRepo, a.logger)
	a.orderService.SetProvisionQueue(a.provisionEnqueuer)
	// 订单列表/详情展示开通履约状态与失败原因（manual=需人工处理，如上游余额不足）。
	a.orderService.SetProvisionTaskReader(a.provisionTaskRepo.FindByOrderIDs)
	a.orderService.SetLogger(a.logger)
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

	// 退款策略（按商品来源+分类的可退时限与免审开关）：
	//   商品来源/分类与分类祖先链都由装配层适配进订单域，订单模块不 import 商品模块
	//   （CONTRIBUTING §3）。
	refundPolicyRepo := orderrepo.NewRefundPolicyRepository(a.database)
	a.refundPolicyService = orderservice.NewRefundPolicyService(
		refundPolicyRepo,
		productPolicyInfoReader{svc: a.prodCatalogService},
		categoryChainReader{svc: a.prodCategoryService},
	)
	a.refundPolicyService.SetCategoryNameReader(func(ctx context.Context, ids []uint64) (map[uint64]string, error) {
		names := make(map[uint64]string, len(ids))
		for _, id := range ids {
			if info, err := a.prodCategoryService.FindByID(ctx, id); err == nil && info != nil {
				names[id] = info.Name
			}
		}
		return names, nil
	})
	a.refundPolicyHandler = orderhandler.NewRefundPolicyHandler(a.refundPolicyService)

	// 退款审核凭据附件（本地存储，与工单附件同口径）。
	refundAttachmentRepo := orderrepo.NewRefundAttachmentRepository(a.database)
	var refundStore *storage.LocalStore
	if st, serr := storage.NewLocalStore(a.cfg.Storage.Root); serr == nil {
		refundStore = st
	} else {
		a.logger.Warn("退款凭据附件存储初始化失败，附件功能不可用", zap.Error(serr))
	}
	a.refundAttachmentService = orderservice.NewRefundAttachmentService(refundAttachmentRepo, refundStore)
	a.orderService.SetRefundAttachmentReader(a.refundAttachmentService)

	// 退款策略匹配注入订单服务：客户自助退款的时限与免审判定依据。
	a.orderService.SetRefundPolicyMatcher(a.refundPolicyService)
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
			// 库存扣减：下单失败原路回补，避免丢库存。
			//
			// 与用户自助下单同口径：挂了 SKU 的商品扣 SKU 库存；**没挂 SKU 的商品
			// （上游转售）扣商品级库存** —— 此前代下单只处理 spec != nil 的分支，
			// 上游限量商品被客服无限代下单（实测：库存 0 仍能开出订单）。
			stockDeducted := false
			productStockDeducted := false
			if spec != nil {
				if spec.Stock >= 0 {
					affected, derr := a.prodCatalogService.DecrementSpecStock(ctx, spec.ID, qty)
					if derr != nil {
						return nil, derr
					}
					if affected == 0 {
						return nil, errors.New("所选规格库存不足")
					}
					stockDeducted = true
				}
			} else {
				affected, derr := a.prodCatalogService.DecrementProductStock(ctx, product.ID, qty)
				if derr != nil {
					return nil, derr
				}
				if affected == 0 {
					return nil, errors.New("该商品库存不足，已售罄")
				}
				productStockDeducted = true
			}
			orderSucceeded := false
			defer func() {
				if orderSucceeded {
					return
				}
				if stockDeducted {
					_, _ = a.prodCatalogService.IncrementSpecStock(context.WithoutCancel(ctx), spec.ID, qty)
				}
				if productStockDeducted {
					_, _ = a.prodCatalogService.IncrementProductStock(context.WithoutCancel(ctx), product.ID, qty)
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
			// 续费价绑定：仅首次价与续费价分开谈定的代下单才带（nil=不绑定，续费按商品定价算）。
			renewalPrice := normalizeRenewalPrice(req.RenewalPrice)
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
				// 续费价承诺随单落库：异步开通 worker 只重读订单，实例绑定价由此转写。
				RenewalPrice: renewalPrice,
				// 代下单的操作人留痕：orders.operator_id 是「最近操作人」，
				// 后台客服代客下单时指向管理员，用户自助下单为 0。
				OperatorID: operatorID,
			}
			// 后台代下单同样锁定销售归属快照（doc86 §3.4）：代客成交也计入销售的业绩与提成。
			order.SalesAdminID = a.salesBundle.customerService.ResolveForNewOrder(ctx, userID)
			// 余额支付：扣款 + 标记已支付 + 投递开通任务（T5.1 异步履约）。
			//
			// OrderNo 一并落进流水：账单按 wallet_transactions.order_no 关联订单区分
			// 普通消费与续费消费，不带的话这笔钱在账面上没有业务单据可追。
			// 此处订单行尚未入库（下面才 Create），但订单号已生成，落的是快照字符串，
			// 订单创建后即可关联 —— 与余额扣款先于订单落库的既有顺序不冲突。
			if payMode == "balance" {
				if _, err := a.walletService.Adjust(ctx, accountdto.AdjustRequest{
					UserID: userID, Type: transmodel.TxTypeAdminOrder, Direction: -1, Amount: price,
					OrderNo: order.OrderNo,
					BizKey:  order.OrderNo,
					Remark:  fmt.Sprintf("代客下单：%s（%s）", product.Name, cycle),
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
			if order.PayMethod == "" {
				// 仅创建不支付（payMode=create）：仍把「打算怎么付」写进订单，
				// 使订单在下单那一刻就与支付方式绑定 —— 后台列表能看出客户用什么付，
				// 场外收款走「确认收款」入账时也有默认值可预填。
				// 留空按用户默认支付方式解析（用户中心偏好 > 平台默认渠道）。
				order.PayMethod = a.resolveAdminOrderPayMethod(ctx, userID, req.PayMethod)
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
			// 下单即建「待开通」实例占位（与用户中心下单同一口径）：后台代下单后
			// 即使没开通成功，实例列表也能看到这台机器并重新开通。
			if err := a.instancePlaceholderHook()(ctx, order); err != nil {
				a.logger.Warn("create pending instance placeholder failed",
					zap.Uint64("order_id", order.ID), zap.Error(err))
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
	// 代下单算价预览：与真正下单共用 adminOrderQuote（同一条管线），
	// 保证弹窗里看到的原价就是落库的原价，不会出现"预览一个数、成交另一个数"。
	a.userHandler.SetOrderQuoteReader(func(ctx context.Context, userID uint64, req userdto.AdminOrderQuoteRequest) (*userdto.AdminOrderQuoteResponse, error) {
		return a.buildAdminOrderQuote(ctx, userID, req)
	})
	a.refundHandler = orderhandler.NewRefundHandler(a.orderService)
	a.refundHandler.SetAttachmentService(a.refundAttachmentService)
	// 用户中心订单：余额支付下单 + 复用履约适配器开通上游
	// ensureOpenable：下单前校验商品能否直连开通，财务型上游（账单推送制）不支持单次开通，先拒绝避免误扣款。
	a.ucOrderService = ucorderservice.NewOrderService(a.prodCatalogService, a.walletService, ucOrderRepoAdapter{repo: a.orderRepo}, a.orderService,
		buildEnsureOpenable(a.prodCatalogService, a.providerService),
		func(err error) bool { return errors.Is(err, finaccountservice.ErrInsufficientBalance) },
		a.userRepo, // 订单「操作人」列：批量解析 operator_id → 用户名（P4-09）
		ucOrderItemAdapter{repo: a.orderItemRepo}, // 订单项落库：行级折扣快照（P5-04）
		a.pricePipeline,      // 统一算价管线（P5-03/P5-04）
		a.prodCatalogService, // SKU 读取与库存增减（T4.1，按 SKU 下单）
		ucProvisionEnqueueAdapter{enqueuer: a.provisionEnqueuer}, // 异步开通任务投递（T5.1）
		ucProvisionTaskAdapter{repo: a.provisionTaskRepo},        // 开通任务状态查询（订单列表轮询展示）
	)
	// 销售归属快照（doc86 §3.4）：用户中心下单瞬间锁定归属。
	a.ucOrderService.SetSalesOwnerResolver(a.salesBundle.newOrderOwnerPort())
	// 下单即建「待开通」实例占位（用户实测问题：开通失败后实例列表里什么都看不到），
	// 开通成功时由 RecordInstance 把占位行就地转正；订单未支付作废时删除占位。
	// 建/删失败只记日志：不阻断下单与关单主流程。
	a.ucOrderService.SetLogger(a.logger)
	// 客户自助退款：uc 薄适配转发到 admin 订单服务（策略/资金链路都在 admin 域）。
	a.ucOrderService.SetRefundPort(ucRefundAdapter{svc: a.orderService})
	// 订单选配项可读名：复用商品目录的客户可选项（与购买页同源），
	// 把订单里落的上游选项/取值 id 翻译成「网络类型：VPC网络」再返回给客户。
	a.ucOrderService.SetOptionLabelResolver(a.prodCatalogService)
	// 占位钩子面向 admin 订单模型；经本地行投影适配给 uc 服务。
	a.ucOrderService.SetInstancePlaceholder(func(ctx context.Context, row *ucorderdto.OrderRow) error {
		return a.instancePlaceholderHook()(ctx, toAdminOrder(row))
	})
	a.ucOrderService.SetPendingInstanceReleaser(a.buildInstancePlaceholderReleaser())
	a.orderService.SetPendingInstanceReleaser(a.buildInstancePlaceholderReleaser())
	// 后台取消待支付订单时回补库存（与用户侧 releasePendingHoldings 同口径）。
	a.orderService.SetStockReleaser(a.buildOrderStockReleaser())
	a.ucOrderHandler = ucorderhandler.NewOrderHandler(a.ucOrderService)
}
