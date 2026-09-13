package server

// 销售体系装配（doc86 S4–S6）：客户归属、提成账本、提成提现、业绩排行的接线。
//
// 依赖方向：admin/sales 不 import 订单/生命周期/支付模块，耦合点全部在本文件：
//   - 订单归属快照：把 sales 的客户服务包成最小 port 注入 uc 订单与续费服务；
//   - 提成计提/冲减与解冻：见 server.go 的钩子接线与调度器启动；
//   - 打款：sales 只依赖 PayoutPort 契约，实现由 buildPaymentBundle 注入（wireSalesPayout）。

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"

	saleshandler "hostsent/backend/internal/modules/admin/sales/handler"
	salesrepo "hostsent/backend/internal/modules/admin/sales/repository"
	salesservice "hostsent/backend/internal/modules/admin/sales/service"
)

// salesBundle 销售体系处理器集合（admin 端）。
type salesBundle struct {
	customerHandler    *saleshandler.CustomerHandler
	commissionHandler  *saleshandler.CommissionHandler
	performanceHandler *saleshandler.PerformanceHandler

	customerService   salesservice.CustomerService
	commissionService salesservice.CommissionService
	withdrawService   salesservice.WithdrawalService
	scheduler         *salesservice.ReleaseScheduler
}

// buildSalesBundle 装配销售体系：仓储 → 服务 → 处理器 → 解冻调度器。
// 打款端口在支付中心装配完成后由 wireSalesPayout 注入（两者互相依赖，只能分两步接线）。
func buildSalesBundle(db *gorm.DB, logger *zap.Logger) *salesBundle {
	relRepo := salesrepo.NewRelationRepository(db)
	commRepo := salesrepo.NewCommissionRepository(db)
	wdRepo := salesrepo.NewWithdrawalRepository(db)

	customerSvc := salesservice.NewCustomerService(db, relRepo, commRepo, logger)
	commissionSvc := salesservice.NewCommissionService(db, commRepo, relRepo, wdRepo, logger)
	withdrawSvc := salesservice.NewWithdrawalService(db, commRepo, wdRepo, relRepo, logger)
	performanceSvc := salesservice.NewPerformanceService(db, commRepo, relRepo)

	return &salesBundle{
		customerHandler:    saleshandler.NewCustomerHandler(customerSvc),
		commissionHandler:  saleshandler.NewCommissionHandler(commissionSvc, withdrawSvc, customerSvc),
		performanceHandler: saleshandler.NewPerformanceHandler(performanceSvc),
		customerService:    customerSvc,
		commissionService:  commissionSvc,
		withdrawService:    withdrawSvc,
		scheduler:          salesservice.NewReleaseScheduler(commissionSvc, logger),
	}
}

// wireSalesPayout 把支付中心打款能力接入销售提现（装配层调用）。
func (b *salesBundle) wireSalesPayout(port salesservice.PayoutPort) {
	if b == nil || b.withdrawService == nil {
		return
	}
	if setter, ok := b.withdrawService.(interface {
		SetPayoutPort(p salesservice.PayoutPort)
	}); ok {
		setter.SetPayoutPort(port)
	}
}

// payoutSettler 销售提现结算回调（交给 buildPaymentBundle 按 biz_type 分发）。
// 与 wireSalesPayout 方向相反：前者是 sales → payment 建打款单，这里是 payment → sales 结算。
func (b *salesBundle) payoutSettler() PayoutSettleFunc {
	if b == nil || b.withdrawService == nil {
		return nil
	}
	return func(ctx context.Context, withdrawID uint64, channelTx, receiptURL, remark string, operatorID uint64) error {
		return b.withdrawService.MarkPaid(ctx, withdrawID, channelTx, receiptURL, remark, operatorID)
	}
}

// releaserPort 员工离职转派端口（注入 manager 员工服务，实现其 SalesReleaser 接口）。
func (b *salesBundle) releaserPort(logger *zap.Logger) *salesReleaserPort {
	if b == nil {
		return nil
	}
	return &salesReleaserPort{customer: b.customerService, logger: logger}
}

// newOrderOwnerPort 新单归属端口（注入 uc 订单服务）。
func (b *salesBundle) newOrderOwnerPort() *salesOwnerPort {
	if b == nil {
		return nil
	}
	return &salesOwnerPort{customer: b.customerService}
}

// renewalOwnerPort 续费归属端口（注入生命周期续费服务）。
func (b *salesBundle) renewalOwnerPort() *salesOwnerRenewalPort {
	if b == nil {
		return nil
	}
	return &salesOwnerRenewalPort{customer: b.customerService}
}

// salesOwnerPort 销售归属 port（注入 uc 订单服务）。
type salesOwnerPort struct {
	customer salesservice.CustomerService
}

// SalesAdminForNewOrder 新单归属快照解析（doc86 §3.4）。
func (p *salesOwnerPort) SalesAdminForNewOrder(ctx context.Context, userID uint64) uint64 {
	if p.customer == nil {
		return 0
	}
	return p.customer.ResolveForNewOrder(ctx, userID)
}

// salesOwnerRenewalPort 续费归属 port（注入生命周期续费服务）。
type salesOwnerRenewalPort struct {
	customer salesservice.CustomerService
}

// SalesAdminForRenewal 续费归属快照解析：默认跟源订单，可配置改跟现役归属。
func (p *salesOwnerRenewalPort) SalesAdminForRenewal(ctx context.Context, userID, srcOrderID uint64) uint64 {
	if p.customer == nil {
		return 0
	}
	return p.customer.ResolveForRenewal(ctx, userID, srcOrderID)
}

// salesReleaserPort 员工离职交待在途客户（注入 manager 员工服务，实现其 SalesReleaser 接口）。
type salesReleaserPort struct {
	customer salesservice.CustomerService
	logger   *zap.Logger
}

// ReleaseStaff 把离职销售名下客户转派给同部门其他在职销售；无候选则释放归属。
// 离职主流程不因转派失败而回滚，失败只记日志（与返现/通知钩子同一约定）。
func (p *salesReleaserPort) ReleaseStaff(ctx context.Context, adminID uint64, transferTo uint64) error {
	if p.customer == nil {
		return nil
	}
	moved, err := p.customer.TransferOnResign(ctx, adminID, 0)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("sales customers transfer on resign failed",
				zap.Uint64("admin_id", adminID), zap.Error(err))
		}
		return err
	}
	if moved > 0 && p.logger != nil {
		p.logger.Info("sales customers transferred on resign",
			zap.Uint64("admin_id", adminID), zap.Int("moved", moved))
	}
	return nil
}
