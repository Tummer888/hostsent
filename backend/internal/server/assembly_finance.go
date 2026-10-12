// 财务域装配：钱包/充值/提现/账单/对账/统计/参数 + 返现与成本（批次四 4b 合并 assembly_cost.go）。

package server

import (
	"context"
	"errors"
	finaccounthandler "hostsent/backend/internal/modules/admin/finance/account/handler"
	finrechargehandler "hostsent/backend/internal/modules/admin/finance/account/recharge/handler"
	finrechargerepo "hostsent/backend/internal/modules/admin/finance/account/recharge/repository"
	finrechargeservice "hostsent/backend/internal/modules/admin/finance/account/recharge/service"
	finaccountrepo "hostsent/backend/internal/modules/admin/finance/account/repository"
	finaccountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	finbillhandler "hostsent/backend/internal/modules/admin/finance/bill/handler"
	finbillrepo "hostsent/backend/internal/modules/admin/finance/bill/repository"
	finbillservice "hostsent/backend/internal/modules/admin/finance/bill/service"
	costhandler "hostsent/backend/internal/modules/admin/finance/cost/handler"
	costrepo "hostsent/backend/internal/modules/admin/finance/cost/repository"
	costservice "hostsent/backend/internal/modules/admin/finance/cost/service"
	referralhandler "hostsent/backend/internal/modules/admin/finance/referral/handler"
	referralrepo "hostsent/backend/internal/modules/admin/finance/referral/repository"
	referralservice "hostsent/backend/internal/modules/admin/finance/referral/service"
	finwithdrawhandler "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/handler"
	finwithdrawrepo "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/repository"
	finwithdrawservice "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/service"
	finsettingshandler "hostsent/backend/internal/modules/admin/finance/settings/handler"
	finsettingsservice "hostsent/backend/internal/modules/admin/finance/settings/service"
	settingsservice "hostsent/backend/internal/modules/admin/finance/settings/service"
	finstatshandler "hostsent/backend/internal/modules/admin/finance/stats/handler"
	finstatsrepo "hostsent/backend/internal/modules/admin/finance/stats/repository"
	finstatsservice "hostsent/backend/internal/modules/admin/finance/stats/service"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	fintransactionrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	providerrepo "hostsent/backend/internal/modules/admin/resource/provider/repository"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	systemmodel "hostsent/backend/internal/modules/admin/system/config/model"
	ucreferralhandler "hostsent/backend/internal/modules/uc/referral/handler"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/upstream"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// buildFinance 构造财务域服务与处理器。
func (a *asm) buildFinance() {
	// 财务域
	a.walletTxRepo = fintransactionrepo.NewTransactionRepository(a.database)
	a.walletRepo = finaccountrepo.NewWalletRepository(a.database)
	a.walletService = finaccountservice.NewWalletService(a.database, a.walletRepo, a.walletTxRepo)
	a.rechargeRepo = finrechargerepo.NewRechargeRepository(a.database)
	a.rechargeService = finrechargeservice.NewRechargeService(a.rechargeRepo, a.walletService)
	a.withdrawRepo = finwithdrawrepo.NewWithdrawRepository(a.database)
	a.withdrawService = finwithdrawservice.NewWithdrawService(a.withdrawRepo, a.walletService)
	a.billRepo = finbillrepo.NewBillRepository(a.database)
	a.billService = finbillservice.NewBillService(a.billRepo, a.walletTxRepo)
	// 原路退回扣点来源（doc36 §3.2）：财务侧不 import 订单模块，由装配层注入退款仓储实现。
	a.billService.SetChannelRefundReader(a.orderRefundRepo.SumChannelRefund)
	// 充值到账后开单笔充值账单：充值侧不 import 账单模块，同样由装配层接线。
	// 缺此接线时人工代充值只写资金流水，账单页看不到这笔充值（doc36 §3.4 的
	// bill_type=recharge 长期无数据即由此而来）。
	// 一笔充值一张账单（幂等键 = 充值单号），不并入当期消费账单：并进一张后
	// 用户拿到的凭证只剩一个「本期充值合计」，对不上「我的充值单」的逐笔记录。
	a.rechargeService.SetBillGenerator(func(ctx context.Context, in finrechargeservice.RechargeBill) error {
		_, err := a.billService.GenerateRechargeBill(ctx, finbillservice.RechargeBillInput{
			UserID:     in.UserID,
			RechargeNo: in.RechargeNo,
			Amount:     in.Amount,
			Method:     in.Method,
			PaidAt:     in.PaidAt,
		})
		return err
	})
	a.rechargeService.SetLogger(a.logger)
	a.reconService = finbillservice.NewReconService(a.walletRepo, a.walletTxRepo)
	a.walletHandler = finaccounthandler.NewWalletHandler(a.walletService)
	a.rechargeHandler = finrechargehandler.NewRechargeHandler(a.rechargeService)
	a.withdrawHandler = finwithdrawhandler.NewWithdrawHandler(a.withdrawService)
	a.billHandler = finbillhandler.NewBillHandler(a.billService)
	a.reconHandler = finbillhandler.NewReconHandler(a.reconService)
	// 财务统计（总览/报表数据源）与财务参数（system_configs finance 分组白名单）。
	// 两者的配置读取/写入统一走下面的 configStore/configValueReader，财务子域不 import 系统配置模块。
	a.financeStatsService = finstatsservice.NewStatsService(finstatsrepo.NewStatsRepository(a.database))
	a.financeSettingsService = finsettingsservice.NewSettingsService(newFinanceConfigStore(a.database))
	a.financeStatsHandler = finstatshandler.NewStatsHandler(a.financeStatsService)
	a.financeSettingsHandler = finsettingshandler.NewSettingsHandler(a.financeSettingsService)
}

// buildReferralCost 构造推广返现、提现与成本模块。
func (a *asm) buildReferralCost() {
	// 推广邀请返现（独立于现金钱包）：订单完成计提、退款按比例冲减。
	a.referralRepo = referralrepo.NewReferralRepository(a.database)
	a.referralSvc = referralservice.NewReferralService(a.database, a.referralRepo, a.logger)
	// 注册链路：为新用户生成邀请码并按需绑定邀请人（失败不阻断注册）。
	a.userCenterService.SetInviteBinder(a.referralSvc)
	// 注册后销售归属自动认领（doc86 S4）：无归属客户按负载补给在职销售，失败不阻断注册。
	a.userCenterService.SetSalesOwnerClaimer(a.salesBundle.customerService)
	// 成本管理（doc111）：月度成本/利润核算 + 上游余额台账 + 每日余额快照。
	// 依赖方向：成本模块不 import 资源渠道与返现模块，三者经装配层小接口接入。
	a.costBundle = buildCostBundle(a.database, a.providerService, a.providerRepo, a.referralRepo, a.financeStatsService, a.configValueReader, a.logger)

	// 返现提现/转出：转出桥接到现金钱包，biz_type=referral_transfer，ref_no=转账号（钱包侧幂等）。
	a.referralWithdrawSvc = referralservice.NewWithdrawalService(a.database, a.referralRepo,
		func(ctx context.Context, userID uint64, amount float64, bizType, refNo, remark string) error {
			_, err := a.walletService.Change(ctx, finaccountservice.ChangeRequest{
				UserID:    userID,
				Type:      transmodel.TxTypeReferralTransfer,
				Direction: transmodel.DirectionIncome,
				Amount:    amount,
				BizType:   bizType,
				RefNo:     refNo,
				Remark:    remark,
			})
			return err
		}, a.logger)
	a.adminReferralHandler = referralhandler.NewReferralHandler(a.referralSvc, a.referralWithdrawSvc)
	a.ucReferralHandler = ucreferralhandler.NewReferralHandler(a.referralSvc, a.referralWithdrawSvc,
		// 错误码映射由装配层提供（uc 不依赖 admin 错误变量，与用户中心财务一致）。
		func(err error) *apperrors.AppError {
			switch {
			case errors.Is(err, referralservice.ErrWithdrawNotFound):
				return apperrors.New(20002, err.Error())
			case errors.Is(err, referralservice.ErrWithdrawStatus), errors.Is(err, referralservice.ErrReferralDisabled):
				return apperrors.New(20003, err.Error())
			case errors.Is(err, referralservice.ErrInvalidAmount), errors.Is(err, referralservice.ErrBelowMinWithdraw):
				return apperrors.New(20001, err.Error())
			case errors.Is(err, referralservice.ErrInsufficientBalance):
				return apperrors.New(30001, err.Error())
			default:
				return apperrors.New(50001, err.Error())
			}
		})
}

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
// 由 handler 映射为 30008，前端据此把「抓取余额」按钮置灰。
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

// financeConfigStore 财务参数存取：只读写 system_configs，不校验键（白名单在 service 层）。
type financeConfigStore struct {
	db *gorm.DB
}

// newFinanceConfigStore 创建财务参数存取实现。
func newFinanceConfigStore(db *gorm.DB) settingsservice.ConfigStore {
	return &financeConfigStore{db: db}
}

// Get 按键读取原文；键不存在返回 found=false 而非错误。
func (s *financeConfigStore) Get(ctx context.Context, key string) (string, bool, error) {
	var item systemmodel.SystemConfig
	if err := s.db.WithContext(ctx).Where("config_key = ?", key).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return item.ConfigValue, true, nil
}

// Save 按键 upsert：存在则更新取值并保持分组/排序，不存在则按财务分组创建。
func (s *financeConfigStore) Save(ctx context.Context, key, value, valueType, description, group string) error {
	var item systemmodel.SystemConfig
	err := s.db.WithContext(ctx).Where("config_key = ?", key).First(&item).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		create := systemmodel.SystemConfig{
			ConfigKey:   key,
			ConfigValue: value,
			ValueType:   valueType,
			Group:       group,
			Description: description,
			Status:      systemmodel.StatusActive,
		}
		return s.db.WithContext(ctx).Create(&create).Error
	}
	item.ConfigValue = value
	item.ValueType = valueType
	item.Group = group
	if strings.TrimSpace(item.Description) == "" {
		item.Description = description
	}
	return s.db.WithContext(ctx).Save(&item).Error
}
