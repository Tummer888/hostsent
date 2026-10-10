package server

// 成本管理装配（doc111）：
//
//   - handler：成本总览 / 成本项配置 / 上游余额台账；
//   - 每日快照调度器：按财务配置 finance.cost_snapshot_hour（默认 3 点）抓取上游余额并落快照；
//   - 依赖注入端口：收入口径（财务统计服务）、返现计提（返现仓储）、上游账户余额（资源渠道服务）。
//
// 依赖方向：成本模块不 import 资源渠道模块与返现模块 —— 三者都通过本文件用小接口接进来，
// 与「账务核心不 import 订单模块」同一取舍；上游接口本身不提供余额时，
// 抓取返回 ErrBalanceUnsupported，页面引导手工录入（见 doc111 §5）。

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	costhandler "hostsent/backend/internal/modules/admin/finance/cost/handler"
	costrepo "hostsent/backend/internal/modules/admin/finance/cost/repository"
	costservice "hostsent/backend/internal/modules/admin/finance/cost/service"
	referralrepo "hostsent/backend/internal/modules/admin/finance/referral/repository"
	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	providerrepo "hostsent/backend/internal/modules/admin/resource/provider/repository"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	"hostsent/backend/internal/pkg/upstream"
)

// costBundle 成本模块装配产物。
type costBundle struct {
	handler   *costhandler.CostHandler
	scheduler *costservice.SnapshotScheduler
}

// buildCostBundle 装配成本模块。
func buildCostBundle(
	database *gorm.DB,
	providerService providerservice.ProviderService,
	providerRepo providerrepo.ProviderRepository,
	referralRepo referralrepo.ReferralRepository,
	revenueReader costservice.RevenueReader,
	configReader costservice.ConfigReader,
	logger *zap.Logger,
) costBundle {
	service := costservice.NewCostService(costrepo.NewCostRepository(database), revenueReader)
	adapter := upstreamAccountAdapter{providers: providerService, repo: providerRepo}
	service.SetUpstreamPort(adapter)
	// 上游账本（消费/充值流水 + 到期清单）：同一适配器实现，成本模块经端口消费。
	service.SetUpstreamLedgerPort(adapter)
	// 返现仓储实现 SumAccrual（返现计提净额）：成本模块经接口消费，不直读他域台账表。
	service.SetReferralReader(referralRepo)
	service.SetConfigReader(configReader)
	return costBundle{
		handler:   costhandler.NewCostHandler(service),
		scheduler: costservice.NewSnapshotScheduler(service, logger),
	}
}

// upstreamAccountAdapter 上游账户余额与账本端口：只依赖资源渠道模块的公开能力。
type upstreamAccountAdapter struct {
	providers providerservice.ProviderService
	repo      providerrepo.ProviderRepository
}

// ListAccountProviders 需要跟踪余额的渠道：启用的上游转售渠道（自营算力平台无账户余额概念）。
func (a upstreamAccountAdapter) ListAccountProviders(ctx context.Context) ([]costservice.UpstreamRef, error) {
	all, err := a.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]costservice.UpstreamRef, 0, len(all))
	for _, item := range all {
		if item.Status != 1 {
			continue
		}
		if item.Kind != "" && item.Kind != providermodel.KindUpstream {
			continue
		}
		refs = append(refs, costservice.UpstreamRef{
			ID:               item.ID,
			Name:             item.Name,
			ProviderType:     item.ProviderType,
			BalanceSupported: a.providers.SupportsAccountBalance(item.ID, item.ProviderType),
			LedgerSupported:  a.providers.SupportsUpstreamLedger(item.ID, item.ProviderType),
		})
	}
	return refs, nil
}

// FetchBalance 抓取余额：把渠道模块的「能力不支持」错误翻译成成本模块的同义错误，
// 由 handler 映射为 30008 并在前端提示手工录入。
func (a upstreamAccountAdapter) FetchBalance(ctx context.Context, providerID uint64) (float64, string, error) {
	info, err := a.providers.AccountBalance(ctx, providerID)
	if err != nil {
		if errors.Is(err, providerservice.ErrAccountBalanceUnsupported) {
			return 0, "", costservice.ErrBalanceUnsupported
		}
		return 0, "", err
	}
	if info == nil {
		return 0, "", costservice.ErrBalanceUnsupported
	}
	return info.Balance, info.Currency, nil
}

// LedgerPage 上游账本分页（消费/充值流水）。方向常量在协议层与成本模块同值（consume/topup），
// 因此按字符串透传；provider_service 内按协议常量分支。
func (a upstreamAccountAdapter) LedgerPage(ctx context.Context, providerID uint64, kind string, page, limit int) ([]upstream.LedgerEntry, int, error) {
	entries, total, err := a.providers.LedgerPage(ctx, providerID, kind, page, limit)
	if err != nil {
		if errors.Is(err, providerservice.ErrUpstreamLedgerUnsupported) {
			return nil, 0, costservice.ErrLedgerUnsupported
		}
		return nil, 0, err
	}
	return entries, total, nil
}

// DueHosts 上游主机的续费信息（余额水位告警用）。
func (a upstreamAccountAdapter) DueHosts(ctx context.Context, providerID uint64) ([]upstream.DueHost, error) {
	dues, err := a.providers.DueHosts(ctx, providerID)
	if err != nil {
		if errors.Is(err, providerservice.ErrUpstreamLedgerUnsupported) {
			return nil, costservice.ErrLedgerUnsupported
		}
		return nil, err
	}
	return dues, nil
}
