package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	accountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	lifecycledto "hostsent/backend/internal/modules/admin/lifecycle/dto"
	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
	lifecyclerepo "hostsent/backend/internal/modules/admin/lifecycle/repository"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/money"
	"hostsent/backend/internal/pkg/pricing"
	"hostsent/backend/internal/pkg/upstream"
)

// lifecycleBizTypeRenewal 资金流水业务类型：续费扣款
const lifecycleBizTypeRenewal = "renewal"

// RenewalService 续费服务：手动续费 / 管理员代续费 / 自动续费。
type RenewalService interface {
	// UserRenew 用户发起续费：生成续费记录 + 待支付订单，并立即尝试余额支付
	UserRenew(ctx context.Context, userID, instanceID uint64, req *lifecycledto.UserRenewRequest) (*lifecycledto.RenewalCreatedResponse, error)
	// AdminRenew 管理员代续费：线下放行，直接完成支付并延长到期时间
	AdminRenew(ctx context.Context, adminID, instanceID uint64, req *lifecycledto.AdminRenewRequest) (*lifecycledto.RenewalInfo, error)
	// ToggleAutoRenew 用户设置实例自动续费开关
	ToggleAutoRenew(ctx context.Context, userID, instanceID uint64, req *lifecycledto.AutoRenewToggleRequest) error
	// ProcessAutoRenewals 扫描到期且开启自动续费的实例并执行续费（调度器调用）
	ProcessAutoRenewals(ctx context.Context) error
	// ListRenewals 管理端续费记录分页
	ListRenewals(ctx context.Context, q *lifecycledto.RenewalListQuery) (*lifecycledto.RenewalListResponse, error)
	// GetRenewal 管理端续费记录详情
	GetRenewal(ctx context.Context, id uint64) (*lifecycledto.RenewalInfo, error)
	// UserRenewalRecords 用户端我的续费记录
	UserRenewalRecords(ctx context.Context, userID uint64, q *lifecycledto.UserRenewalListQuery) (*lifecycledto.RenewalListResponse, error)
	// UserRenewalsView 用户端续费管理聚合视图（我的实例到期情况 + 策略摘要）
	UserRenewalsView(ctx context.Context, userID uint64) (*lifecycledto.UserRenewalsViewResponse, error)
	// UserRenewalDetail 用户端续费记录详情（校验归属）
	UserRenewalDetail(ctx context.Context, userID, id uint64) (*lifecycledto.RenewalInfo, error)
	// CompleteRenewalByOrderID 订单支付成功钩子：完成续费单并延长实例到期时间
	CompleteRenewalByOrderID(ctx context.Context, orderID uint64) error
	// SetCashbackHook 注入续费完成后的推广返现计提钩子（装配层调用）
	SetCashbackHook(hook RenewalCashbackHook)
	// SetUpstreamRenewer 注入上游/平台续费执行器（装配层调用，T5.2）
	SetUpstreamRenewer(renewer UpstreamRenewer)
	// SetRenewedHook 注入续费完成事件回调（P6/T6.5 开放平台回调，装配层调用）
	SetRenewedHook(hook RenewedHook)
	// SetPointEarner 注入续费完成后的积分发放钩子（装配层调用，doc36）
	SetPointEarner(hook RenewalPointHook)
	// SetSalesOwnerResolver 注入续费订单的销售归属解析（装配层调用，doc86 §3.4）
	SetSalesOwnerResolver(r SalesOwnerResolver)
}

// UpstreamRenewer 上游/平台续费执行器（装配层注入，避免生命周期模块依赖 provider 模块）。
//
// 链路 A（inst.SourceMode=upstream）由适配器调上游续费接口并返回权威新到期时间；
// 适配器不具备续费能力时返回 upstream.ErrCapabilityMissing，由服务层按链路决定降级。
type UpstreamRenewer func(ctx context.Context, inst *syncmodel.Instance, periodCount int, cycle string) (*upstream.RenewResult, error)

// RenewalCashbackHook 续费完成后的推广返现计提钩子。
// 仅传基础类型，避免生命周期模块反向依赖返现模块。
type RenewalCashbackHook func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, amount float64, isRenewal bool) error

// SalesOwnerResolver 续费订单的销售归属解析（doc86 §3.4）：装配层注入，
// 避免 lifecycle 反向依赖 sales 模块。srcOrderID 为实例的来源订单（可能为 0）。
type SalesOwnerResolver interface {
	SalesAdminForRenewal(ctx context.Context, userID, srcOrderID uint64) uint64
}

// lifecycleRenewalGetter 续费完成时按 ID 读取订单（用于获取支付方式快照）。
type lifecycleRenewalGetter interface {
	GetByID(ctx context.Context, id uint64) (*ordermodel.Order, error)
}

// renewalPriceResolver 统一算价管线（P5-04）；为 nil 时回落为「单价 × 期数」不打折。
type renewalPriceResolver interface {
	Resolve(ctx context.Context, in pricing.ResolveInput) (*pricing.Quote, error)
}

// renewalQuote 续费算价结果：原价/优惠/实付 + 折扣来源快照，随订单落库。
type renewalQuote struct {
	Original float64
	Discount float64
	Final    float64
	PolicyID *uint64
	Source   string
	Snapshot string
}

type renewalService struct {
	db           *gorm.DB
	renewalRepo  lifecyclerepo.RenewalRepository
	policyRepo   lifecyclerepo.PolicyRepository
	autoRepo     lifecyclerepo.AutoRenewRepository
	instanceRepo lifecyclerepo.InstanceReader
	orderWriter  lifecyclerepo.OrderWriter
	orderReader  lifecycleRenewalGetter // 可选：读取订单支付方式
	walletSvc    accountservice.WalletService
	pricingSvc   renewalPriceResolver // 可选：统一算价管线（P5-04）
	cashbackHook RenewalCashbackHook  // 可选：续费完成后的推广返现计提
	upstream     UpstreamRenewer      // 可选：上游/平台续费执行器（T5.2）
	renewedHook  RenewedHook          // 可选：续费完成事件（P6/T6.5 开放平台回调）
	pointHook    RenewalPointHook     // 可选：续费完成后的积分发放（doc36）
	salesOwner   SalesOwnerResolver   // 可选：续费订单的销售归属解析（doc86 §3.4）
	logger       *zap.Logger
}

// NewRenewalService 创建续费服务。pricingSvc 为 nil 时续费按「单价 × 期数」计不加折扣。
func NewRenewalService(
	db *gorm.DB,
	renewalRepo lifecyclerepo.RenewalRepository,
	policyRepo lifecyclerepo.PolicyRepository,
	autoRepo lifecyclerepo.AutoRenewRepository,
	instanceRepo lifecyclerepo.InstanceReader,
	orderWriter lifecyclerepo.OrderWriter,
	walletSvc accountservice.WalletService,
	pricingSvc renewalPriceResolver,
	logger *zap.Logger,
) RenewalService {
	return &renewalService{
		db:           db,
		renewalRepo:  renewalRepo,
		policyRepo:   policyRepo,
		autoRepo:     autoRepo,
		instanceRepo: instanceRepo,
		orderWriter:  orderWriter,
		walletSvc:    walletSvc,
		pricingSvc:   pricingSvc,
		logger:       logger,
	}
}

// SetSalesOwnerResolver 注入续费订单的销售归属解析器（装配层调用）。
func (s *renewalService) SetSalesOwnerResolver(r SalesOwnerResolver) { s.salesOwner = r }

// SetOrderReader 注入订单读取器（可选依赖，用于支付钩子读取支付方式）。
func (s *renewalService) SetOrderReader(r lifecycleRenewalGetter) {
	s.orderReader = r
}

// SetCashbackHook 注入续费完成后的推广返现计提钩子。
func (s *renewalService) SetCashbackHook(hook RenewalCashbackHook) {
	s.cashbackHook = hook
}

// RenewedHook 续费完成事件钩子（P6/T6.5）：参数为已落库成功的续费单。
type RenewedHook func(ctx context.Context, renewal *lifecyclemodel.InstanceRenewal)

// RenewalPointHook 续费完成后的积分发放钩子（doc36）。
// 只传基础类型，避免生命周期模块反向依赖积分模块；失败不影响续费结果。
type RenewalPointHook func(ctx context.Context, orderID uint64, orderNo string, userID uint64, amount float64)

// SetRenewedHook 注入续费完成事件回调（装配层调用）。
func (s *renewalService) SetRenewedHook(hook RenewedHook) {
	s.renewedHook = hook
}

// SetPointEarner 注入续费完成后的积分发放钩子（装配层调用，doc36）。
func (s *renewalService) SetPointEarner(hook RenewalPointHook) {
	s.pointHook = hook
}

// SetUpstreamRenewer 注入上游/平台续费执行器（T5.2）。
func (s *renewalService) SetUpstreamRenewer(renewer UpstreamRenewer) {
	s.upstream = renewer
}

// resolveRenewalQuote 续费算价（P5-04）：命中管线则走统一算价；未注入时回落单价×期数。
// manual 非 nil 表示管理员手动改价，直接作为实付（最高优先级）。
// 算价仍以 product_id（售出商品）为口径，产品名/兜底单价改用实例的双链路列解析（T1.2）。
func (s *renewalService) resolveRenewalQuote(ctx context.Context, userID uint64, instance *syncmodel.Instance, periodCount int, manual *float64) (renewalQuote, error) {
	if s.pricingSvc != nil {
		quote, err := s.pricingSvc.Resolve(ctx, pricing.ResolveInput{
			UserID:       userID,
			ProductID:    instance.SellProductID,
			Quantity:     periodCount,
			Period:       periodCount,
			ManualAmount: manual,
		})
		if err != nil {
			return renewalQuote{}, err
		}
		return renewalQuote{
			Original: quote.OriginalAmount,
			Discount: quote.DiscountAmount,
			Final:    quote.FinalAmount,
			PolicyID: quote.PolicyID,
			Source:   quote.Source,
			Snapshot: marshalRenewalSnapshot(quote.Snapshot),
		}, nil
	}
	_, unitPrice, err := s.instanceRepo.ResolveProduct(ctx, instance)
	if err != nil {
		return renewalQuote{}, err
	}
	original := money.Round2(unitPrice * float64(periodCount))
	final := original
	source := ""
	if manual != nil {
		final = money.Round2(*manual)
		source = pricing.SourceManual
	}
	if final <= 0 {
		return renewalQuote{}, ErrPolicyInvalid
	}
	// Snapshot 落到 jsonb 列：无折扣明细时也必须给合法空数组，否则写库报错。
	return renewalQuote{Original: original, Discount: money.Round2(math.Max(0, original-final)), Final: final, Source: source, Snapshot: "[]"}, nil
}

// marshalRenewalSnapshot 折扣快照序列化为 jsonb 文本，失败时退化为空数组。
func marshalRenewalSnapshot(rules []pricing.Rule) string {
	if len(rules) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// UserRenew 用户手动续费：创建续费记录 + 待支付订单，随后立即尝试余额支付。
// 余额充足则一步完成；不足时保持 pending，等待充值后由支付钩子驱动完成。
func (s *renewalService) UserRenew(ctx context.Context, userID, instanceID uint64, req *lifecycledto.UserRenewRequest) (*lifecycledto.RenewalCreatedResponse, error) {
	instance, err := s.loadOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.ExpireAt == nil {
		return nil, ErrStatusNotAllowed
	}
	periodCount := normalizePeriod(req.PeriodCount)
	productName, _, err := s.instanceRepo.ResolveProduct(ctx, &instance.Instance)
	if err != nil {
		return nil, err
	}
	// 统一算价管线（P5-04）：用户组/代理折扣在此生效。
	quote, err := s.resolveRenewalQuote(ctx, userID, &instance.Instance, periodCount, nil)
	if err != nil {
		return nil, err
	}
	renewal, err := s.createPendingRenewal(ctx, instance, productName, periodCount, quote, lifecyclemodel.RenewalSourceManual, 0, "")
	if err != nil {
		return nil, err
	}
	// 余额支付：余额不足不视为请求失败，续费单保持待支付等充值
	if perr := s.payRenewalOrder(ctx, renewal, ordermodel.PayMethodBalance, 0); perr != nil && !errors.Is(perr, ErrInsufficientBalance) {
		s.logger.Warn("auto balance pay after user renew failed",
			zap.String("renewal_no", renewal.RenewalNo), zap.Error(perr))
	}
	latest, err := s.renewalRepo.FindByID(ctx, renewal.ID)
	if err != nil {
		return nil, err
	}
	return &lifecycledto.RenewalCreatedResponse{Renewal: *buildRenewalInfo(latest)}, nil
}

// AdminRenew 管理员代续费：人工放行（线下结算），直接完成支付并延长到期时间。
func (s *renewalService) AdminRenew(ctx context.Context, adminID, instanceID uint64, req *lifecycledto.AdminRenewRequest) (*lifecycledto.RenewalInfo, error) {
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}
	if instance.ExpireAt == nil {
		return nil, ErrStatusNotAllowed
	}
	periodCount := normalizePeriod(req.PeriodCount)
	productName, _, err := s.instanceRepo.ResolveProduct(ctx, instance)
	if err != nil {
		return nil, err
	}
	// 管理员指定金额 → 走 ManualAmount 分支（discount_source='manual'）；未指定则按管线算价（P5-04）。
	var manual *float64
	if req.Amount > 0 {
		amount := money.Round2(req.Amount)
		manual = &amount
	}
	quote, err := s.resolveRenewalQuote(ctx, instance.UserID, instance, periodCount, manual)
	if err != nil {
		return nil, err
	}
	renewal, err := s.createPendingRenewal(ctx, &lifecyclerepo.InstanceWithUser{Instance: *instance}, productName, periodCount, quote, lifecyclemodel.RenewalSourceAdmin, adminID, req.Remark)
	if err != nil {
		return nil, err
	}
	if err := s.markRenewalSuccess(ctx, renewal, ordermodel.PayMethodManual, adminID); err != nil {
		return nil, err
	}
	latest, err := s.renewalRepo.FindByID(ctx, renewal.ID)
	if err != nil {
		return nil, err
	}
	return buildRenewalInfo(latest), nil
}

// ToggleAutoRenew 设置实例自动续费开关（仅实例归属人可操作）。
func (s *renewalService) ToggleAutoRenew(ctx context.Context, userID, instanceID uint64, req *lifecycledto.AutoRenewToggleRequest) error {
	if _, err := s.loadOwnedInstance(ctx, userID, instanceID); err != nil {
		return err
	}
	return s.autoRepo.Upsert(ctx, &lifecyclemodel.InstanceAutoRenewal{
		InstanceID:  instanceID,
		UserID:      userID,
		Enabled:     req.Enabled,
		PeriodCount: normalizePeriod(req.PeriodCount),
	})
}

// ProcessAutoRenewals 自动续费扫描：对开启自动续费且已到期的实例逐一尝试余额扣款续费。
// 余额不足时续费单置为 failed，等待下轮扫描重试（每轮新建续费单，避免积压大量 pending）。
func (s *renewalService) ProcessAutoRenewals(ctx context.Context) error {
	switches, err := s.autoRepo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	if len(switches) == 0 {
		return nil
	}
	now := time.Now()
	instanceIDs := make([]uint64, 0, len(switches))
	periodByInstance := make(map[uint64]int, len(switches))
	userByInstance := make(map[uint64]uint64, len(switches))
	for _, sw := range switches {
		instanceIDs = append(instanceIDs, sw.InstanceID)
		periodByInstance[sw.InstanceID] = normalizePeriod(sw.PeriodCount)
		userByInstance[sw.InstanceID] = sw.UserID
	}
	instances, err := s.instanceRepo.ListDueForAutoRenew(ctx, now, instanceIDs)
	if err != nil {
		return err
	}
	for i := range instances {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		instance := &instances[i]
		if instance.UserID == 0 {
			instance.UserID = userByInstance[instance.ID]
		}
		if err := s.processOneAutoRenewal(ctx, instance, periodByInstance[instance.ID]); err != nil {
			s.logger.Warn("auto renew instance failed",
				zap.Uint64("instance_id", instance.ID), zap.Error(err))
		}
	}
	return nil
}

// processOneAutoRenewal 单实例自动续费：查价 → 建续费单 → 余额扣款 → 完成。
// 余额不足时续费单置 failed 留痕，不影响其余实例。
func (s *renewalService) processOneAutoRenewal(ctx context.Context, instance *syncmodel.Instance, periodCount int) error {
	if instance.ExpireAt == nil {
		return nil
	}
	productName, _, err := s.instanceRepo.ResolveProduct(ctx, instance)
	if err != nil {
		return err
	}
	// 自动续费同样走统一算价管线（P5-04），折扣口径与手动续费一致。
	quote, err := s.resolveRenewalQuote(ctx, instance.UserID, instance, periodCount, nil)
	if err != nil {
		return err
	}
	if quote.Final <= 0 {
		return ErrPolicyInvalid
	}
	renewal, err := s.createPendingRenewal(ctx, &lifecyclerepo.InstanceWithUser{Instance: *instance}, productName, periodCount, quote, lifecyclemodel.RenewalSourceAuto, 0, "")
	if err != nil {
		return err
	}
	if err := s.payRenewalOrder(ctx, renewal, ordermodel.PayMethodBalance, 0); err != nil {
		// 余额不足等失败：续费单置 failed，下轮扫描重建重试
		renewal.Status = lifecyclemodel.RenewalStatusFailed
		renewal.FailReason = err.Error()
		if uerr := s.renewalRepo.Update(ctx, renewal); uerr != nil {
			return uerr
		}
		if errors.Is(err, ErrInsufficientBalance) {
			return nil
		}
		return err
	}
	return nil
}

// ListRenewals 管理端续费记录分页。
func (s *renewalService) ListRenewals(ctx context.Context, q *lifecycledto.RenewalListQuery) (*lifecycledto.RenewalListResponse, error) {
	items, total, err := s.renewalRepo.List(ctx, lifecyclerepo.RenewalListParams{
		Keyword:   q.Keyword,
		UserID:    q.UserID,
		Status:    q.Status,
		Source:    q.Source,
		StartTime: q.StartTime,
		EndTime:   q.EndTime,
		Page:      q.Page,
		PageSize:  q.PageSize,
	})
	if err != nil {
		return nil, err
	}
	return buildRenewalListResponse(items, total, q.Page, q.PageSize), nil
}

func (s *renewalService) GetRenewal(ctx context.Context, id uint64) (*lifecycledto.RenewalInfo, error) {
	renewal, err := s.renewalRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRenewalNotFound
		}
		return nil, err
	}
	return buildRenewalInfo(renewal), nil
}

func (s *renewalService) UserRenewalRecords(ctx context.Context, userID uint64, q *lifecycledto.UserRenewalListQuery) (*lifecycledto.RenewalListResponse, error) {
	items, total, err := s.renewalRepo.ListByUser(ctx, userID, q.Status, q.Page, q.PageSize)
	if err != nil {
		return nil, err
	}
	return buildRenewalListResponse(items, total, q.Page, q.PageSize), nil
}

func (s *renewalService) UserRenewalDetail(ctx context.Context, userID, id uint64) (*lifecycledto.RenewalInfo, error) {
	renewal, err := s.renewalRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRenewalNotFound
		}
		return nil, err
	}
	if renewal.UserID != userID {
		return nil, ErrPermissionDenied
	}
	return buildRenewalInfo(renewal), nil
}

// UserRenewalsView 用户端续费管理聚合视图：我的实例到期情况 + 自动续费开关 + 策略摘要。
func (s *renewalService) UserRenewalsView(ctx context.Context, userID uint64) (*lifecycledto.UserRenewalsViewResponse, error) {
	instances, err := s.instanceRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	policy, perr := s.policyRepo.Get(ctx)
	if perr != nil && !errors.Is(perr, gorm.ErrRecordNotFound) {
		return nil, perr
	}
	now := time.Now()
	items := make([]lifecycledto.UserInstanceRenewalItem, 0, len(instances))
	for i := range instances {
		item := &instances[i]
		if item.ExpireAt == nil {
			continue
		}
		productName, unitPrice, perr := s.instanceRepo.ResolveProduct(ctx, &item.Instance)
		if perr != nil {
			return nil, perr
		}
		stage := ""
		if policy != nil {
			stage = deriveStageForUser(*item.ExpireAt, now, policy)
		}
		items = append(items, lifecycledto.UserInstanceRenewalItem{
			ID:           item.ID,
			Name:         item.Name,
			InstanceMark: item.InstanceID,
			ProductID:    item.SellProductID,
			ProductName:  productName,
			UnitPrice:    unitPrice,
			BillingMode:  item.BillingMode,
			Status:       item.Status,
			ExpireAt:     formatTime(item.ExpireAt),
			DaysLeft:     daysLeft(*item.ExpireAt, now),
			Stage:        stage,
			AutoRenew:    item.AutoRenew,
			AutoPeriod:   item.AutoPeriod,
		})
	}
	resp := &lifecycledto.UserRenewalsViewResponse{Items: items}
	if policy != nil {
		policyResp := buildPolicyResponse(policy)
		resp.Policy = *policyResp
	}
	return resp, nil
}

// deriveStageForUser 用户端展示用的派生阶段（与管理端 DeriveStage 同规则）。
func deriveStageForUser(expireAt, now time.Time, policy *lifecyclemodel.LifecyclePolicy) string {
	graceEnd := expireAt.AddDate(0, 0, policy.GraceDays)
	destroyEnd := graceEnd.AddDate(0, 0, policy.DestroyKeepDays)
	switch {
	case now.Before(expireAt):
		return lifecyclemodel.StageActive
	case now.Before(graceEnd):
		return lifecyclemodel.StageGrace
	case now.Before(destroyEnd):
		return lifecyclemodel.StageSuspended
	default:
		return lifecyclemodel.StageDestroyed
	}
}

// CompleteRenewalByOrderID 订单支付成功钩子：按订单完成关联续费单并延长到期时间。
// 幂等：续费单非 pending 状态直接跳过。
func (s *renewalService) CompleteRenewalByOrderID(ctx context.Context, orderID uint64) error {
	renewal, err := s.renewalRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 非续费订单，忽略
		}
		return err
	}
	if renewal.Status != lifecyclemodel.RenewalStatusPending {
		return nil
	}
	payMethod := ""
	if s.orderReader != nil {
		if order, err := s.orderReader.GetByID(ctx, orderID); err == nil {
			payMethod = order.PayMethod
		}
	}
	return s.markRenewalSuccess(ctx, renewal, payMethod, 0)
}

// ---- 内部辅助 ----

// loadOwnedInstance 加载归属用户的实例。
func (s *renewalService) loadOwnedInstance(ctx context.Context, userID, instanceID uint64) (*lifecyclerepo.InstanceWithUser, error) {
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}
	if instance.UserID != userID {
		return nil, ErrPermissionDenied
	}
	return &lifecyclerepo.InstanceWithUser{Instance: *instance}, nil
}

// createPendingRenewal 创建续费记录 + 待支付续费订单（同事务落库保证关联一致）。
// quote 携带算价明细，随订单落库以还原历史优惠（P5-04）。
func (s *renewalService) createPendingRenewal(ctx context.Context, instance *lifecyclerepo.InstanceWithUser, productName string, periodCount int, quote renewalQuote, source string, operatorID uint64, remark string) (*lifecyclemodel.InstanceRenewal, error) {
	amount := quote.Final
	order := &ordermodel.Order{
		OrderNo:        genOrderNoLocal(),
		UserID:         instance.UserID,
		ProductID:      instance.SellProductID,
		ProductName:    productName,
		Specs:          fmt.Sprintf(`{"type":"renewal","instance_id":%d,"period_count":%d}`, instance.ID, periodCount),
		Quantity:       periodCount,
		TotalAmount:    quote.Original,
		PaidAmount:     amount,
		OriginalAmount: quote.Original,
		DiscountAmount: quote.Discount,
		FinalAmount:    quote.Final,
		PricePolicyID:  quote.PolicyID,
		DiscountSource: quote.Source,
		PriceSnapshot:  quote.Snapshot,
		Status:         ordermodel.OrderStatusPending,
		Remark:         remark,
		OperatorID:     operatorID,
	}
	// 销售归属快照（doc86 §3.4）：默认跟源订单（instance.OrderID），可配置改跟现役归属。
	if s.salesOwner != nil {
		order.SalesAdminID = s.salesOwner.SalesAdminForRenewal(ctx, instance.UserID, instance.OrderID)
	}
	renewal := &lifecyclemodel.InstanceRenewal{
		RenewalNo:    genRenewalNoLocal(),
		InstanceID:   instance.ID,
		InstanceMark: instance.InstanceID,
		UserID:       instance.UserID,
		ProductID:    instance.SellProductID,
		ProductName:  productName,
		BillingMode:  instance.BillingMode,
		PeriodCount:  periodCount,
		Amount:       amount,
		Source:       source,
		Status:       lifecyclemodel.RenewalStatusPending,
		Remark:       remark,
		ExpireBefore: instance.ExpireAt,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		renewal.OrderID = order.ID
		renewal.OrderNo = order.OrderNo
		if err := tx.Create(renewal).Error; err != nil {
			return err
		}
		// 回写订单续费关联，供支付成功钩子识别（doc60）
		return tx.Model(order).Update("renewal_id", renewal.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return renewal, nil
}

// payRenewalOrder 续费订单余额支付：扣款 → 标记订单 → 完成续费（扣款幂等，重复调用不重复记账）。
func (s *renewalService) payRenewalOrder(ctx context.Context, renewal *lifecyclemodel.InstanceRenewal, payMethod string, operatorID uint64) error {
	if _, err := s.walletSvc.Change(ctx, accountservice.ChangeRequest{
		UserID:    renewal.UserID,
		Type:      transmodel.TxTypeConsume,
		Direction: transmodel.DirectionExpense,
		Amount:    renewal.Amount,
		OrderID:   renewal.OrderID,
		OrderNo:   renewal.OrderNo,
		RefNo:     renewal.RenewalNo,
		BizType:   lifecycleBizTypeRenewal,
		Remark:    "云主机续费：" + renewal.ProductName,
	}); err != nil {
		if errors.Is(err, accountservice.ErrInsufficientBalance) {
			return ErrInsufficientBalance
		}
		return err
	}
	return s.markRenewalSuccess(ctx, renewal, payMethod, operatorID)
}

// markRenewalSuccess 完成续费（T5.2）：上游/平台续费 → 标记订单已支付 → 续费单 success → 落实例到期时间。
//
// 链路差异（doc15 §6.2）：
//   - 链路 A（SourceMode=upstream）：必须调上游续费接口，以上游返回的 NewExpireAt 为权威账期；
//     上游失败或缺能力时**不得只改本地账期**，直接报错让续费单保持可重试。
//   - 链路 B（SourceMode=self）：平台无独立续费接口时按本地账期顺延（sync_state=local_only），
//     平台若实现了续费接口同样调用；平台调用失败则整体失败（本地顺延回滚）。
//
// 幂等：可重复调用（状态机幂等；钱包扣款以 renewal_no 为幂等键）。
func (s *renewalService) markRenewalSuccess(ctx context.Context, renewal *lifecyclemodel.InstanceRenewal, payMethod string, operatorID uint64) error {
	// 0) 解析实例：链路判据 + provider 实例号（续费必需）。
	instance, err := s.instanceRepo.GetByID(ctx, renewal.InstanceID)
	if err != nil {
		return err
	}
	if renewal.ExpireBefore == nil {
		renewal.ExpireBefore = instance.ExpireAt
	}
	if renewal.ExpireBefore == nil {
		now := time.Now()
		renewal.ExpireBefore = &now
	}
	// 本地顺延账期（链路 B 权威；链路 A 在上游未返回新账期时兜底）。
	finalExpire := s.nextExpireAt(*renewal.ExpireBefore, renewal.BillingMode, renewal.PeriodCount)
	syncState := lifecyclemodel.RenewalSyncLocalOnly

	// 1) 上游/平台续费。
	if s.upstream != nil {
		res, rerr := s.upstream(ctx, instance, renewal.PeriodCount, renewal.BillingMode)
		switch {
		case rerr == nil:
			syncState = lifecyclemodel.RenewalSyncUpstreamOK
			if res != nil {
				renewal.UpstreamOrderID = res.UpstreamOrderRef
				if !res.NewExpireAt.IsZero() {
					t := res.NewExpireAt
					finalExpire = &t
				}
			}
		case errors.Is(rerr, upstream.ErrCapabilityMissing):
			if instance.SourceMode == syncmodel.SourceModeUpstream {
				// 链路 A 缺续费能力：显式失败并留痕，绝不静默只改本地账期。
				s.markRenewalSyncFailed(ctx, renewal, rerr)
				return fmt.Errorf("上游续费失败：%w", rerr)
			}
			// 链路 B：平台无独立续费接口属预期，本地账期顺延。
			s.logger.Info("renewal upstream not supported, fallback to local period",
				zap.String("renewal_no", renewal.RenewalNo), zap.Uint64("instance_id", renewal.InstanceID))
		default:
			// 平台续费失败（链路 B 亦不放过）：整体失败，本地顺延不生效。
			s.markRenewalSyncFailed(ctx, renewal, rerr)
			return fmt.Errorf("上游续费失败：%w", rerr)
		}
	}

	// 2) 落订单支付 + 续费单成功 + 实例新账期。
	if err := s.orderWriter.MarkOrderPaid(ctx, renewal.OrderID, renewal.Amount, payMethod, operatorID); err != nil {
		return err
	}
	now := time.Now()
	payTime := now
	renewal.Status = lifecyclemodel.RenewalStatusSuccess
	renewal.PayTime = &payTime
	renewal.SyncState = syncState
	renewal.FailReason = ""
	renewal.ExpireAfter = finalExpire
	if err := s.renewalRepo.Update(ctx, renewal); err != nil {
		return err
	}
	if err := s.instanceRepo.ExtendExpireAt(ctx, renewal.InstanceID, *renewal.ExpireAfter); err != nil {
		return err
	}
	// 续费完成后的推广返现计提：失败不影响续费结果，钩子实现负责记录错误。
	if s.cashbackHook != nil {
		_ = s.cashbackHook(ctx, renewal.OrderID, renewal.OrderNo, renewal.UserID, renewal.Amount, true)
	}
	// 开放平台续费完成事件（P6/T6.5）：失败只记日志，不影响续费结果。
	if s.renewedHook != nil {
		s.renewedHook(ctx, renewal)
	}
	// 续费完成后的积分发放（doc36）：幂等键为订单号，失败只记日志。
	if s.pointHook != nil {
		s.pointHook(ctx, renewal.OrderID, renewal.OrderNo, renewal.UserID, renewal.Amount)
	}
	s.logger.Info("renewal completed",
		zap.String("renewal_no", renewal.RenewalNo),
		zap.Uint64("instance_id", renewal.InstanceID),
		zap.String("sync_state", syncState),
		zap.String("upstream_order_id", renewal.UpstreamOrderID),
		zap.Float64("amount", renewal.Amount))
	return nil
}

// markRenewalSyncFailed 记录续费同步失败（留痕，便于运维重试与对账）；写库失败只记日志。
func (s *renewalService) markRenewalSyncFailed(ctx context.Context, renewal *lifecyclemodel.InstanceRenewal, cause error) {
	renewal.SyncState = lifecyclemodel.RenewalSyncFailed
	renewal.FailReason = truncateReason(cause.Error())
	if err := s.renewalRepo.Update(ctx, renewal); err != nil {
		s.logger.Warn("record renewal sync failure failed",
			zap.String("renewal_no", renewal.RenewalNo), zap.Error(err))
	}
}

// truncateReason 截断失败原因到 fail_reason 列长（255），避免写库报错。
func truncateReason(msg string) string {
	if len(msg) <= 255 {
		return msg
	}
	return msg[:255]
}

// nextExpireAt 按计费周期计算续费后的到期时间（doc25 §8）。
// 周期口径统一走 pkg/billingcycle：支持按小时/日/月/季/半年/年/两年/三年，
// 未知周期回落按月（保持既有兜底语义，存量 billing_mode 行为不变）。
func (s *renewalService) nextExpireAt(base time.Time, billingMode string, periodCount int) *time.Time {
	// 已过期实例从当前时间起算，避免续费期落在过去时段
	if base.Before(time.Now()) {
		base = time.Now()
	}
	t := billingcycle.Advance(base, billingMode, periodCount)
	return &t
}

// genOrderNoLocal 生成续费订单号：RO + 时间戳 + 随机。
func genOrderNoLocal() string {
	return fmt.Sprintf("RO%s%06d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000)
}

// genRenewalNoLocal 生成续费单号：RN + 时间戳 + 随机。
func genRenewalNoLocal() string {
	return fmt.Sprintf("RN%s%06d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000)
}

func normalizePeriod(period int) int {
	if period <= 0 {
		return 1
	}
	if period > 36 {
		return 36
	}
	return period
}

// buildRenewalListResponse 续费列表统一组装。
func buildRenewalListResponse(items []lifecyclemodel.InstanceRenewal, total int64, page, pageSize int) *lifecycledto.RenewalListResponse {
	resp := make([]lifecycledto.RenewalInfo, 0, len(items))
	for i := range items {
		resp = append(resp, *buildRenewalInfo(&items[i]))
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 10
	}
	return &lifecycledto.RenewalListResponse{
		Items: resp,
		Meta:  lifecycledto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}
}

// buildRenewalInfo 模型转 DTO。
func buildRenewalInfo(renewal *lifecyclemodel.InstanceRenewal) *lifecycledto.RenewalInfo {
	info := &lifecycledto.RenewalInfo{
		ID:           renewal.ID,
		RenewalNo:    renewal.RenewalNo,
		InstanceID:   renewal.InstanceID,
		InstanceMark: renewal.InstanceMark,
		UserID:       renewal.UserID,
		ProductID:    renewal.ProductID,
		ProductName:  renewal.ProductName,
		BillingMode:  renewal.BillingMode,
		PeriodCount:  renewal.PeriodCount,
		Amount:       renewal.Amount,
		Source:       renewal.Source,
		Status:       renewal.Status,
		OrderID:      renewal.OrderID,
		OrderNo:      renewal.OrderNo,
		Remark:       renewal.Remark,
		FailReason:   renewal.FailReason,
		CreatedAt:    renewal.CreatedAt.Format(time.RFC3339),
	}
	if renewal.PayTime != nil {
		info.PayTime = renewal.PayTime.Format(time.RFC3339)
	}
	if renewal.ExpireBefore != nil {
		info.ExpireBefore = renewal.ExpireBefore.Format(time.RFC3339)
	}
	if renewal.ExpireAfter != nil {
		info.ExpireAfter = renewal.ExpireAfter.Format(time.RFC3339)
	}
	return info
}
