// 待开通实例占位（下单即可见）装配：下单建占位、开通成功转正、订单作废即删、
// 实例维度「重新开通」。
//
// 背景（用户实测）：订单开通失败（上游余额不足）后，实例列表里什么都看不到 ——
// 用户既无法查看"这台机器"的情况，也无法重新开通/拉取信息。根因是实例行只在
// 上游创建成功后才落库。本文件把实例行的生命周期前移到「下单」：
//
//	下单  → 占位实例（provider_instance_id 空，status=creating，"待开通"）
//	开通成功 → 占位行就地转正（写入上游实例号/IP/账期），不产生第二行
//	订单未支付超时/取消 → 删除占位行
//
// 判定"是否已在上游开通"的唯一依据是 provider_instance_id 是否为空。

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	catalogmodel "hostsent/backend/internal/modules/admin/product/catalog/model"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
)

// pendingInstancePrefix 占位实例号前缀（本地生成，转正时被真实上游实例号替换）。
// 保留前缀语义：仅凭 instance_id 也能一眼看出"这台还没在上游开通"。
const pendingInstancePrefix = "pending-"

// pendingInstanceMark 占位实例号：按订单生成，稳定且全局唯一（orders.order_no 唯一）。
func pendingInstanceMark(orderNo string) string {
	no := strings.TrimSpace(orderNo)
	if no == "" {
		return ""
	}
	return pendingInstancePrefix + no
}

// 占位钩子的三个窄依赖（装配层用产品/同步服务实现；单元测试可用假实现）。
type instancePlaceholderPorts struct {
	// provisionProviderID 该订单会开通到的提供商 ID（0 表示不是可开通机器，不建占位）。
	provisionProviderID func(ctx context.Context, order *ordermodel.Order) uint64
	// productLink 商品的链路语义：source_mode 与上游资源商品镜像 ID。
	productLink func(ctx context.Context, productID uint64) (sourceMode string, sourceProductID uint64)
	// create 落库占位行（幂等）。
	create func(ctx context.Context, row *syncmodel.Instance) error
}

// instancePlaceholderHook 用本进程的产品/同步服务装配占位钩子（uc 下单与后台代下单共用）。
func (a *asm) instancePlaceholderHook() func(ctx context.Context, order *ordermodel.Order) error {
	return buildInstancePlaceholder(instancePlaceholderPorts{
		provisionProviderID: func(ctx context.Context, order *ordermodel.Order) uint64 {
			// 能构建出开通请求且有提供商 ID 的商品才建占位（虚拟/纯本地商品不建）。
			preq, err := a.prodCatalogService.BuildProvisionRequest(ctx, order.ProductID, order.ProductName, order.Specs, order.SpecCode, nil)
			if err != nil || preq == nil {
				return 0
			}
			return preq.ProviderID
		},
		productLink: func(ctx context.Context, productID uint64) (string, uint64) {
			product, err := a.prodCatalogService.FindByID(ctx, productID)
			if err != nil || product == nil {
				return "", 0
			}
			return product.SourceMode, product.SourceProductID
		},
		create: a.syncRepo.CreatePendingInstance,
	}, a.logger)
}

// buildInstancePlaceholder 构造「下单即建待开通实例」钩子，由 uc 订单服务在订单落库后调用。
//
// 只有"确实会开通一台机器"的订单才建占位：能构建出开通请求且带提供商 ID 的商品
// （虚拟/纯本地商品不建，避免实例列表被非机器订单污染）。构建失败按"不建"处理
// （下单主流程不因占位失败而失败，开通成功仍走常规落库）。
func buildInstancePlaceholder(ports instancePlaceholderPorts, logger *zap.Logger) func(ctx context.Context, order *ordermodel.Order) error {
	return func(ctx context.Context, order *ordermodel.Order) error {
		if order == nil || order.ID == 0 || order.RenewalID != 0 {
			// 续费订单不建占位：续费是对既有实例延长账期，不产生新机器。
			return nil
		}
		mark := pendingInstanceMark(order.OrderNo)
		if mark == "" {
			return nil
		}
		providerID := ports.provisionProviderID(ctx, order)
		if providerID == 0 {
			return nil
		}
		sourceMode := catalogmodel.SourceModeSelf
		var upstreamProductID uint64
		if ports.productLink != nil {
			if mode, resourceID := ports.productLink(ctx, order.ProductID); mode != "" || resourceID != 0 {
				if mode != "" {
					sourceMode = mode
				}
				upstreamProductID = resourceID
			}
		}
		row := syncmodel.Instance{
			InstanceID: mark,
			ProviderID: providerID,
			UserID:     order.UserID,
			Name:       order.ProductName,
			Status:     string(pkgmodel.InstanceStatusCreating),
			// 未开通前没有账期：expire_at 在转正时按上游/订单周期写入，
			// 否则生命周期推进器会把"还没开通"的机器当成到期实例处置。
			BillingMode:        firstNonEmpty(order.Cycle, order.PriceModel),
			ActorUserID:        order.OperatorID,
			OrderID:            order.ID,
			SourceMode:         sourceMode,
			SellProductID:      order.ProductID,
			UpstreamProductID:  upstreamProductID,
			ProviderInstanceID: "", // 空 = 尚未在上游开通（转正判据）
			// 续费价绑定随即落占位行：开通失败重试/占位转正都复用这一行，
			// 在这里写入即可保证"下单时谈定的续费价"不因链路中断而丢失。
			RenewalPrice: orderRenewalPrice(order),
			// 客户备注随下单落占位行：占位行就是这台机器将来那一行（转正是就地更新），
			// 因此这里写入即可，无需在转正路径再补一次。
			CustomerRemark: order.CustomerRemark,
		}
		if specs := specsFromOrder(order); specs != nil {
			row.CPU = specs.CPU
			row.Memory = specs.Memory
			row.Disk = specs.Disk
			row.DiskType = specs.DiskType
			row.Bandwidth = specs.Bandwidth
			row.OS = specs.OS
			row.Region = specs.Region
			row.Zone = specs.Zone
		}
		if err := ports.create(ctx, &row); err != nil {
			return err
		}
		if logger != nil {
			logger.Info("pending instance created for order",
				zap.Uint64("order_id", order.ID), zap.String("mark", mark), zap.Uint64("provider_id", providerID))
		}
		return nil
	}
}

// buildInstancePlaceholderReleaser 构造「订单作废即删占位实例」钩子：
// 未支付订单超时关单/用户取消时调用。只删未转正的占位行，已开通实例不受影响。
func (a *asm) buildInstancePlaceholderReleaser() func(ctx context.Context, orderID uint64) (int64, error) {
	return func(ctx context.Context, orderID uint64) (int64, error) {
		return a.syncRepo.DeletePendingInstanceByOrder(ctx, orderID)
	}
}

// buildOrderStockReleaser 构造「订单作废即回补库存」钩子（订单管理端取消/关单）。
//
// 口径与用户自助下单的 releasePendingHoldings 一致：有 SKU 回补 SKU 库存，
// 没有 SKU（上游转售商品）回补商品级库存。
// 为什么必须成对：代下单同样原子扣减库存，不回补就会"取消一单少一份库存"，
// 商品会莫名其妙越来越难卖。
func (a *asm) buildOrderStockReleaser() func(ctx context.Context, order *ordermodel.Order) error {
	return func(ctx context.Context, order *ordermodel.Order) error {
		if order == nil || order.Quantity <= 0 {
			return nil
		}
		if order.SpecCode == "" {
			_, err := a.prodCatalogService.IncrementProductStock(ctx, order.ProductID, order.Quantity)
			return err
		}
		specs, err := a.prodCatalogService.ListSpecs(ctx, order.ProductID)
		if err != nil {
			return err
		}
		for i := range specs {
			if specs[i].SpecCode == order.SpecCode {
				_, err := a.prodCatalogService.IncrementSpecStock(ctx, specs[i].ID, order.Quantity)
				return err
			}
		}
		return nil
	}
}

// buildInstanceProvisionRetry 构造实例维度「重新开通」钩子（管理端/用户中心共用）。
//
// 语义：占位实例（未开通）→ 重新投递来源订单的开通履约任务。人工队列（manual）
// 的任务会被投递重置为 pending 并给足重试预算，因此"上游充值后再点一次"即可复用
// 已生成的上游账单完成开通（不会重复下单）。
//
// 订单状态守卫：只有已支付/开通中的订单可重试；未支付的请先付款（或等超时关单），
// 已关闭/已退款的需重新下单——避免对着作废订单空跑。
func (a *asm) buildInstanceProvisionRetry() func(ctx context.Context, orderID uint64) error {
	return func(ctx context.Context, orderID uint64) error {
		if orderID == 0 {
			return errors.New("该实例未关联订单，无法重新开通")
		}
		order, err := a.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			return errors.New("来源订单不存在，无法重新开通")
		}
		switch order.Status {
		case ordermodel.OrderStatusPaid, ordermodel.OrderStatusProvisioning:
		default:
			return fmt.Errorf("订单当前状态（%s）不可开通：未支付请先完成支付，已关闭/已退款请重新下单", order.Status)
		}
		return a.orderService.EnqueueProvision(ctx, order.ID)
	}
}
