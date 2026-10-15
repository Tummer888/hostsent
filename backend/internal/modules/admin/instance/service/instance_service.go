// Package service 提供管理端跨用户实例运维台业务编排：
// 列表/统计/详情/电源操作/远程控制台/回源刷新/备注/变配/销毁/操作流水/关联记录。
//
// 与用户中心 uc/instance 的差别：本服务不带实例归属校验（面向运营），
// 但每次写操作都会落一条 instance_operations 流水，并通过权限码细分高危动作。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
	"hostsent/backend/internal/modules/admin/instance/model"
	"hostsent/backend/internal/modules/admin/instance/repository"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/credentials"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/money"
	"hostsent/backend/internal/pkg/upstream"
)

// defaultExpireWithinDays 未指定时的临期判定天数。
const defaultExpireWithinDays = 7

// 生命周期阶段字面量（与 lifecycle/model 的 Stage* 同值）。
//
// 刻意用本地常量而非 import lifecycle 包：instance 服务与 lifecycle 模块是并列关系，
// 生命周期模块已通过装配层反向注入回调给本模块（见 SetEventListener/DisposeOnRefund），
// 直接 import 会形成循环依赖。
const (
	lifecycleStageActive    = "active"
	lifecycleStageSuspended = "suspended"
	lifecycleStageDestroyed = "destroyed"
)

// ProviderResolver 按提供商 ID 解析上游适配器（装配层注入，与用户中心同签名）。
type ProviderResolver func(ctx context.Context, providerID uint64) (upstream.Provider, error)

// Operator 操作人标识（写入操作流水）。
type Operator struct {
	Type string // admin / user / system
	ID   uint64
	Name string
}

// InstanceService 实例运维台业务能力。
type InstanceService interface {
	List(ctx context.Context, query *dto.ListQuery) (*dto.ListResponse, error)
	Stats(ctx context.Context, query *dto.StatsQuery) (*dto.StatsResponse, error)
	// InstanceTypes 产品线导航数据（类型清单 + 各自实例数）。
	InstanceTypes(ctx context.Context) (*dto.InstanceTypeSummary, error)
	Detail(ctx context.Context, id uint64, live bool) (*dto.DetailInfo, error)
	Power(ctx context.Context, op Operator, id uint64, action string) error
	VNC(ctx context.Context, op Operator, id uint64) (*dto.VNCResult, error)
	Sync(ctx context.Context, op Operator, id uint64) (*dto.DetailInfo, error)
	Resize(ctx context.Context, op Operator, id uint64, req *dto.ResizeRequest) error
	SetRemark(ctx context.Context, op Operator, id uint64, remark string) error
	// AdjustBilling 手工调整账期/付款周期/绑定续费价（管理端自定义，不产生账单）。
	AdjustBilling(ctx context.Context, op Operator, id uint64, req *dto.AdjustRequest) (*dto.DetailInfo, error)
	// AdjustContext 读账期调整上下文（可选周期以**上游能力**为准 + 上游/我方各周期价）。
	AdjustContext(ctx context.Context, id uint64) (*dto.AdjustContext, error)
	// ReinstallImages 读重装可选镜像：自营取渠道镜像目录，对接取上游商品的 os 配置子项。
	ReinstallImages(ctx context.Context, id uint64) (*dto.ReinstallImages, error)
	// RelinkUpstream 按上游实例号重新关联（上游换机/换号；改上游号 = 改变后续所有上游动作的作用对象）。
	RelinkUpstream(ctx context.Context, op Operator, id uint64, req *dto.RelinkRequest) (*dto.RelinkResult, error)
	// RebindProduct 更换绑定的本地售出商品与上游镜像（上游换产品场景）。
	RebindProduct(ctx context.Context, op Operator, id uint64, req *dto.RebindProductRequest) (*dto.RebindProductResult, error)
	// RebindCandidates 读更换对接商品的候选项（选框式 UI 的数据源，替代手填 ID）。
	RebindCandidates(ctx context.Context, id uint64) (*dto.RebindProductCandidates, error)
	// RevealCredential 查看实例登录明文密码（高危：写审计流水后才返回）。
	RevealCredential(ctx context.Context, op Operator, id uint64) (*dto.CredentialRevealResult, error)
	Destroy(ctx context.Context, op Operator, id uint64, req *dto.DestroyRequest) error
	// Suspend / Unsuspend 暂停/恢复实例（T5.5 按能力分派）。
	Suspend(ctx context.Context, op Operator, id uint64, reason string) error
	Unsuspend(ctx context.Context, op Operator, id uint64) error
	// 维护类操作（平台接口存在即提供，能力位见 Detail 的 Capabilities）。
	Reinstall(ctx context.Context, op Operator, id uint64, req *dto.ReinstallRequest) (*dto.ReinstallResult, error)
	ResetPassword(ctx context.Context, op Operator, id uint64, req *dto.ResetPasswordRequest) error
	Rescue(ctx context.Context, op Operator, id uint64, req *dto.RescueRequest) error
	ExitRescue(ctx context.Context, op Operator, id uint64) error
	Snapshots(ctx context.Context, id uint64, snapshotType string) ([]dto.SnapshotInfo, error)
	CreateSnapshot(ctx context.Context, op Operator, id uint64, req *dto.SnapshotCreateRequest) error
	DeleteSnapshot(ctx context.Context, op Operator, id uint64, snapshotID string) error
	RestoreSnapshot(ctx context.Context, op Operator, id uint64, req *dto.SnapshotRestoreRequest) error
	SetBandwidth(ctx context.Context, op Operator, id uint64, req *dto.BandwidthRequest) error
	AddIP(ctx context.Context, op Operator, id uint64, req *dto.AddIPRequest) error
	AttachDataDisk(ctx context.Context, op Operator, id uint64, req *dto.AttachDiskRequest) error
	// BatchAction 批量运维：对多个实例下发同一动作，返回逐台结果（部分失败不影响其余）。
	BatchAction(ctx context.Context, op Operator, req *dto.BatchActionRequest) (*dto.BatchActionResponse, error)
	// BatchDestroy 批量销毁：逐台独立执行，每台须带自己的确认文本（不可逆动作的护栏）。
	BatchDestroy(ctx context.Context, op Operator, req *dto.BatchDestroyRequest) (*dto.BatchDestroyResponse, error)
	// DisposeOnRefund 退款审核通过后的实例侧处置（doc61 §8 第 2 条，系统操作人）。
	// 是否动作由 lifecyclpolicy.refund_action 决定（默认 none：不动作）。
	DisposeOnRefund(ctx context.Context, in *dto.RefundInstanceRequest) error
	// SetRefundPolicyReader 注入退款处置策略读取（装配层调用，返回 none/suspend/destroy）。
	SetRefundPolicyReader(fn func(ctx context.Context) string)
	// RecordStageAction 写入生命周期阶段推进审计（T5.4 推进器调用，系统操作人）。
	RecordStageAction(ctx context.Context, in StageActionInput) error
	// SetEventListener 注入实例操作事件监听（P6/T6.5 开放平台回调；装配层调用，可为 nil）。
	SetEventListener(fn func(ctx context.Context, evt InstanceEvent))
	// SetDetailRepository 注入详情页关联数据仓储（装配层调用；为 nil 时详情仅返回实例本体）。
	SetDetailRepository(repo repository.InstanceDetailRepository)
	// SetDetailOrderConfigReader 注入订单选配读取（装配层调用；为 nil 时配置项退化为商品默认）。
	SetDetailOrderConfigReader(fn func(ctx context.Context, orderID uint64) (string, error))
	// SetCredentialSecretKey 注入应用加密密钥（装配层调用）：
	// 重置密码/重装产生的新密码据此加密留存，供「查看密码」回显。
	SetCredentialSecretKey(key string)
	// RetryProvision 重新开通待开通实例：重投来源订单的履约任务（占位行专用）。
	RetryProvision(ctx context.Context, op Operator, id uint64) (*dto.DetailInfo, error)
	// RefreshByOrder 按订单回源刷新其实例，返回刷新后的状态（无实例返回空串）。
	// 供「开通中滞留推进器」使用：上游主机异步就绪后，由它来把订单推向服务中。
	RefreshByOrder(ctx context.Context, orderID uint64) (string, error)
	// SetProvisionRetry 注入「重新开通」执行者（装配层调用；为 nil 时该入口不可用）。
	SetProvisionRetry(fn func(ctx context.Context, orderID uint64) error)
	Operations(ctx context.Context, id uint64, query *dto.OperationListQuery) (*dto.OperationListResponse, error)
	// OperationLogs 全局操作流水（跨实例审计视图）。
	OperationLogs(ctx context.Context, query *dto.OperationLogQuery) (*dto.OperationLogListResponse, error)
	Related(ctx context.Context, id uint64) (*dto.RelatedInfo, error)
}

type instanceService struct {
	repo    repository.InstanceRepository
	opRepo  repository.OperationRepository
	related repository.RelatedRepository
	resolve ProviderResolver
	logger  *zap.Logger
	// detail 详情页关联数据只读仓储（订单/商品/上游/配置/凭证）；为 nil 时详情仅返回实例本体。
	detail repository.InstanceDetailRepository
	// detailOrderConfig 读订单选配 JSON（装配层注入；为 nil 时配置项退化为商品默认）。
	detailOrderConfig func(ctx context.Context, orderID uint64) (string, error)
	// refundPolicy 退款处置策略读取（装配层注入 lifecycle_policies.refund_action）；为 nil 视为 none。
	refundPolicy func(ctx context.Context) string
	// eventListener 实例操作/阶段事件监听（P6/T6.5 开放平台回调经装配层注入，可为 nil）。
	eventListener func(ctx context.Context, evt InstanceEvent)
	// provisionRetry 实例维度「重新开通」（装配层注入：重投来源订单的履约任务），可为 nil。
	provisionRetry func(ctx context.Context, orderID uint64) error
	// secretKey 应用加密密钥：密码类操作（重置/重装）落库前用它加密（迁移 100）。
	// 为空时跳过硬留（重置仍生效，只是事后无法回看），绝不落明文。
	secretKey string
}

// InstanceEvent 实例操作事件：record() 与 RecordStageAction 收口后的对外通知载体。
type InstanceEvent struct {
	InstanceRowID uint64
	InstanceMark  string
	UserID        uint64
	Action        string // power_on/power_off/suspend/unsuspend/stage...
	FromStatus    string
	ToStatus      string
	Err           error
}

// NewInstanceService 创建实例运维台服务。
func NewInstanceService(
	repo repository.InstanceRepository,
	opRepo repository.OperationRepository,
	related repository.RelatedRepository,
	resolve ProviderResolver,
	logger *zap.Logger,
) InstanceService {
	return &instanceService{repo: repo, opRepo: opRepo, related: related, resolve: resolve, logger: logger}
}

// SetDetailRepository 注入详情页关联数据仓储（装配层调用）。
func (s *instanceService) SetDetailRepository(repo repository.InstanceDetailRepository) {
	s.detail = repo
}

// SetDetailOrderConfigReader 注入订单选配读取（装配层调用）。
func (s *instanceService) SetDetailOrderConfigReader(fn func(ctx context.Context, orderID uint64) (string, error)) {
	s.detailOrderConfig = fn
}

// SetCredentialSecretKey 注入应用加密密钥（装配层调用）：
// 重置密码/重装成功后用它加密留存新密码（instances.credential_password，迁移 100）。
// 未注入时这些操作仍照常执行，只是不更新留存（「查看密码」会提示改用重置密码）。
func (s *instanceService) SetCredentialSecretKey(key string) {
	s.secretKey = key
}

// SetProvisionRetry 注入「重新开通」执行者（装配层调用：重投来源订单的履约任务）。
func (s *instanceService) SetProvisionRetry(fn func(ctx context.Context, orderID uint64) error) {
	s.provisionRetry = fn
}

// RefreshByOrder 按订单回源刷新其实例（推进器调用；系统操作人留痕）。
//
// 与用户点「同步刷新」同一套动作：拉上游主机详情 → 回写本地状态/IP。
// 无实例（订单不是云主机商品）返回空串，调用方跳过；上游不可达返回错误。
func (s *instanceService) RefreshByOrder(ctx context.Context, orderID uint64) (string, error) {
	if orderID == 0 {
		return "", nil
	}
	row, err := s.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	if !provisioned(row) {
		// 占位实例（尚未在上游开通）：无对象可刷。
		return "", nil
	}
	before := row.Status
	if err := s.refreshUpstream(ctx, row); err != nil {
		s.record(ctx, row, Operator{Type: model.OperatorTypeSystem, Name: "开通就绪推进"}, model.ActionSync, nil, before, before, err)
		return "", err
	}
	fresh, ferr := s.repo.FindByID(ctx, row.ID)
	after := before
	if ferr == nil && fresh != nil {
		after = fresh.Status
	}
	s.record(ctx, row, Operator{Type: model.OperatorTypeSystem, Name: "开通就绪推进"}, model.ActionSync, nil, before, after, nil)
	return after, nil
}

// RetryProvision 重新开通：对待开通实例（占位行）重投来源订单的开通履约任务。
//
// 语义是"复用已生成的上游账单再支付"，不会重复下单（见 119 号文档的幂等修复）；
// 人工队列（manual）的任务会被投递重置为 pending 并给足重试预算，因此上游充值后
// 点一次即可完成开通。
func (s *instanceService) RetryProvision(ctx context.Context, op Operator, id uint64) (*dto.DetailInfo, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	if provisioned(row) {
		return nil, fmt.Errorf("%w：实例已开通（上游实例号 %s）", ErrStatusConflict, row.ProviderInstanceID)
	}
	if s.provisionRetry == nil {
		return nil, ErrProvisionRetryUnavailable
	}
	before := row.Status
	if err := s.provisionRetry(ctx, row.OrderID); err != nil {
		s.record(ctx, row, op, model.ActionProvisionRetry, nil, before, before, err)
		return nil, err
	}
	s.record(ctx, row, op, model.ActionProvisionRetry, nil, before, string(pkgmodel.InstanceStatusCreating), nil)
	return s.Detail(ctx, id, false)
}

// SetEventListener 注入实例操作事件监听（P6/T6.5）。
func (s *instanceService) SetEventListener(fn func(ctx context.Context, evt InstanceEvent)) {
	s.eventListener = fn
}

// SetRefundPolicyReader 注入退款处置策略读取（装配层调用）。
func (s *instanceService) SetRefundPolicyReader(fn func(ctx context.Context) string) {
	s.refundPolicy = fn
}

// DisposeOnRefund 退款审核通过后的实例侧处置（doc61 §8 第 2 条）。
//
// 语义：
//   - 策略为 none（默认）→ 不动作，直接返回（退款只走资金链路）；
//   - 只有全额退款才处置实例（部分退款是补偿，客户机器仍在用）；
//   - destroy 要求二次确认标识，这里由系统代为填写（策略已由运营显式开启即视为授权）；
//   - 订单没有关联实例（非云主机订单、存量 order_id=0）→ 静默跳过，属正常情况。
//
// 无论成功失败都写一条 refund_dispose 流水，保证「钱退了但机器还在」这种事后争议可查。
func (s *instanceService) DisposeOnRefund(ctx context.Context, in *dto.RefundInstanceRequest) error {
	if in == nil {
		return nil
	}
	action := "none"
	if s.refundPolicy != nil {
		action = s.refundPolicy(ctx)
	}
	if action != "suspend" && action != "destroy" {
		return nil
	}
	if !in.FullRefund {
		// 部分退款不动机器：这条规则刻在代码里，避免运营把「部分退款」也配成销毁。
		return nil
	}
	row, err := s.repo.FindByOrderID(ctx, in.OrderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil // 该订单没有实例，正常（虚拟商品等）。
		}
		return err
	}

	op := Operator{Type: model.OperatorTypeSystem, Name: "退款联动"}
	reason := "订单退款通过（" + in.RefundNo + "，订单 " + in.OrderNo + "）"
	if action == "destroy" {
		// 复用 Destroy/Suspend：能力分派、状态守卫、流水、事件回调一并具备，
		// 且理由中带退款单号，全局操作流水按退款单号可反查（不重复写第二条流水）。
		return s.Destroy(ctx, op, row.ID, &dto.DestroyRequest{
			ConfirmMark: row.InstanceID,
			Reason:      reason,
		})
	}
	return s.Suspend(ctx, op, row.ID, reason)
}

// notifyEvent 收口通知：监听器失败只记日志，不影响操作结果。
func (s *instanceService) notifyEvent(ctx context.Context, evt InstanceEvent) {
	if s.eventListener == nil {
		return
	}
	s.eventListener(ctx, evt)
}

// List 跨用户实例分页列表。
func (s *instanceService) List(ctx context.Context, query *dto.ListQuery) (*dto.ListResponse, error) {
	if query == nil {
		query = &dto.ListQuery{}
	}
	rows, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	days := normalizeDays(query.ExpireWithinDays)
	items := make([]dto.InstanceItem, 0, len(rows))
	for i := range rows {
		items = append(items, s.toItem(&rows[i], days))
	}
	page, pageSize := query.Page, query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return &dto.ListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

// Stats 全局概览统计。
func (s *instanceService) Stats(ctx context.Context, query *dto.StatsQuery) (*dto.StatsResponse, error) {
	days := defaultExpireWithinDays
	if query != nil {
		days = normalizeDays(query.ExpireWithinDays)
	}
	return s.repo.Stats(ctx, days)
}

// InstanceTypes 产品线导航数据：登记的类型 + 各自实例数 + 总量。
//
// 返回**全量登记类型**（含当前没有实例的预埋类型，Reserved=true），由前端决定是否显示 ——
// 后端不做"要不要显示空类型"的产品决策，但要把事实（该类型存在、当前 0 台）如实给出。
func (s *instanceService) InstanceTypes(ctx context.Context) (*dto.InstanceTypeSummary, error) {
	counts, err := s.repo.TypeCounts(ctx)
	if err != nil {
		return nil, err
	}
	kinds := pkgmodel.ProductKinds()
	out := &dto.InstanceTypeSummary{Types: make([]dto.InstanceTypeOption, 0, len(kinds))}
	var total int64
	for _, n := range counts {
		total += n
	}
	for _, kind := range kinds {
		n := counts[kind.Value]
		out.Types = append(out.Types, dto.InstanceTypeOption{
			Value:    kind.Value,
			Label:    kind.Label,
			Count:    n,
			Reserved: n == 0,
		})
	}
	out.Total = total
	return out, nil
}

// Detail 实例详情；live=true 时先尝试回源刷新（失败降级为本地数据）。
//
// 返回内容分五块：实例本体与能力位、来源订单财务口径、售出商品/对接关系、
// 生效配置项、登录连接信息。后四块都是「读失败即降级为空」——详情页的首要
// 职责是把实例本体现出来，不能因为某个关联表查询出错就整页打不开。
func (s *instanceService) Detail(ctx context.Context, id uint64, live bool) (*dto.DetailInfo, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	if live {
		// 详情页的实时刷新属尽力而为：上游不可用时仍要能看到本地数据。
		_ = s.refreshUpstream(ctx, row)
		if fresh, ferr := s.repo.FindByID(ctx, id); ferr == nil {
			row = fresh
		}
	}
	item := s.toItem(row, defaultExpireWithinDays)
	caps, capErr := s.capabilities(ctx, row.ProviderID)
	detail := &dto.DetailInfo{InstanceItem: item, Capabilities: caps}
	// 待开通实例（占位行）：能力位整体关闭并给出原因，避免界面按钮能点但点了必失败。
	if !provisioned(row) {
		detail.Capabilities = dto.Capabilities{}
		if capErr != nil {
			detail.CapabilityError = capErr.Error()
		}
		if detail.CapabilityError == "" {
			detail.CapabilityError = notProvisionedReason(row)
		}
	} else if capErr != nil {
		detail.CapabilityError = capErr.Error()
	}
	s.attachDetailContext(ctx, row, detail)
	// NAT 机型补齐访问地址（doc122）：NAT 没有独立 IP，只显示 public_ip 客户无从下手。
	s.attachNatAccess(ctx, row, detail)
	// 待开通实例：带上履约状态与失败原因，运营/用户在详情页直接看到"为什么还没开通"。
	if s.detail != nil && !provisioned(row) {
		if status, lastErr, perr := s.detail.Provision(ctx, row.OrderID); perr == nil {
			detail.ProvisionStatus = status
			detail.ProvisionError = lastErr
		}
	}
	return detail, nil
}

// provisioned 判断实例是否已在（上游/平台）开通：占位行 provider_instance_id 为空。
func provisioned(row *repository.InstanceRow) bool {
	return row != nil && row.ProviderInstanceID != ""
}

// resaleInstance 判断实例是否走**转售链路**（source_mode=upstream）。
//
// 为什么需要这个判据来管住界面与守卫：自营实例（source_mode=self）用的是本平台
// 自己的算力渠道，实例号就是本系统生成并下发的号，不存在"上游把机器换了一台、
// 我们要跟着改号"这回事；而 products 表里也没有"对接商品"这一层。
//
// 此前多处只看"实例是否已开通"，于是自营实例的详情页照样显示「上游实例号」
// 与「重新关联」「更换对接商品」入口 —— 运营照着重关联的提示去填一个"上游号"，
// 点下去只会得到一句渠道不可用的报错，或者更糟：把一个正确的号改错
// （用户实测反馈："虽然是我自己的，但它这里会显示上游实例号，这个一看就是有问题"）。
//
// 兜底：source_mode 为空的历史行（迁移前数据）按"是否绑定了上游镜像"判定 ——
// 有 upstream_product_id 说明确实对着一个上游资源商品，按对接处理更安全
// （宁可多给入口，也不要让真正对接的实例失去换绑能力）。
func resaleInstance(row *repository.InstanceRow) bool {
	if row == nil {
		return false
	}
	switch strings.TrimSpace(row.SourceMode) {
	case "upstream":
		return true
	case "self":
		return false
	}
	return row.UpstreamProductID != 0
}

// notProvisionedReason 待开通实例的说明文案（含履约失败原因，如有）。
func notProvisionedReason(row *repository.InstanceRow) string {
	_ = row
	return "实例尚未开通（待开通状态）：开通成功后可在此查看实时状态；若开通失败，原因见「来源订单」区块，修复后可点「重新开通」重试"
}

// ensureProvisioned 上游操作前置校验：待开通实例（无上游实例号）一律拒绝，
// 给出可操作的说明而不是把空实例号发到上游换来一个费解的报错。
func (s *instanceService) ensureProvisioned(row *repository.InstanceRow) error {
	if provisioned(row) {
		return nil
	}
	return ErrInstanceNotProvisioned
}

// attachDetailContext 补齐详情页的关联信息（订单/商品/上游/配置/凭证）。
// 逐块独立容错：任一块读失败只留空，不影响其余块与实例本体。
func (s *instanceService) attachDetailContext(ctx context.Context, row *repository.InstanceRow, detail *dto.DetailInfo) {
	if s.detail == nil || detail == nil || row == nil {
		return
	}
	quantity := 1
	// 来源订单：订单里同时带付款周期与数量，续费金额按实例当前周期现算。
	// 传实例 ID：绑定价（instances.renewal_price）要压过商品周期价（与续费实际计费一致）。
	order, oerr := s.detail.SourceOrder(ctx, row.OrderID, row.ID, row.BillingMode, quantity)
	if oerr == nil && order != nil {
		quantity = order.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		detail.Order = order
	}
	// 售出商品：自营取 sell_product_id；上游链路 sell_product_id 可能为空，
	// 此时商品信息缺失属正常（该实例不是经我方商品开通的）。
	// 商品行被删（重新导入/清理）时按订单快照兜底展示并标 Deleted，避免详情页
	// 把"经我方商品开通、但商品已删"的实例误判成"非我方商品开通"。
	product, perr := s.detail.Product(ctx, row.SellProductID)
	if (perr != nil || product == nil) && detail.Order != nil && detail.Order.ProductID != 0 && detail.Order.ProductID != row.SellProductID {
		product, _ = s.detail.Product(ctx, detail.Order.ProductID)
	}
	if product == nil && detail.Order != nil && detail.Order.ProductName != "" {
		product = &dto.ProductInfo{ID: detail.Order.ProductID, Name: detail.Order.ProductName, Deleted: true}
	}
	if product != nil {
		if product.Name == "" && detail.Order != nil {
			product.Name = detail.Order.ProductName
		}
		detail.Product = product
	}
	// 上游商品：优先实例绑定的镜像（upstream_product_id）；为空或镜像已删时
	// 回落到售出商品记录的 source_product_id —— 两者本应指向同一行，
	// 兜底可避免"对接商品有、上游商品空"的不对称展示。
	resourceID := row.UpstreamProductID
	if resourceID == 0 && detail.Product != nil {
		resourceID = detail.Product.SourceProductID
	}
	upstream, upstreamRenew, uerr := s.detail.Upstream(ctx, resourceID, row.BillingMode)
	if uerr == nil && upstream == nil && resourceID != 0 && resourceID != row.UpstreamProductID && row.UpstreamProductID != 0 {
		// 实例绑定的镜像已被删除：再按售出商品记录的回退镜像试一次。
		if detail.Product != nil && detail.Product.SourceProductID != 0 {
			upstream, upstreamRenew, uerr = s.detail.Upstream(ctx, detail.Product.SourceProductID, row.BillingMode)
		}
	}
	if uerr == nil && upstream != nil {
		// 渠道一致性校准：镜像所属渠道与实例渠道不一致时显式标记（历史数据常见）。
		if upstream.ProviderID != 0 && row.ProviderID != 0 && upstream.ProviderID != row.ProviderID {
			upstream.ProviderMismatch = true
		}
		detail.Upstream = upstream
	} else {
		upstreamRenew = 0
	}
	if detail.Order != nil {
		detail.Order.UpstreamRenewAmount = upstreamRenew
		if upstreamRenew > 0 {
			detail.Order.UpstreamRenewCycle = billingcycle.NormalizeOrDefault(row.BillingMode)
		}
	}
	// 生效配置项：商品配置项 + 订单选配，再用实例表字段覆盖（实例字段最准）。
	orderConfig := ""
	if detail.Order != nil {
		orderConfig = s.orderConfigOptions(ctx, detail.Order.ID)
	}
	configs, cerr := s.detail.ProductConfigs(ctx, row.SellProductID, orderConfig)
	if cerr == nil {
		detail.Configs = mergeInstanceConfigs(configs, row)
	}
	// 登录连接信息（不含明文密码）。
	if cred, crerr := s.detail.Credentials(ctx, row.ID); crerr == nil {
		detail.Credential = cred
	}
}

// orderConfigOptions 读订单的客户选配 JSON（读失败返回空，调用方按商品默认降级）。
func (s *instanceService) orderConfigOptions(ctx context.Context, orderID uint64) string {
	if s.detailOrderConfig == nil {
		return ""
	}
	raw, err := s.detailOrderConfig(ctx, orderID)
	if err != nil {
		return ""
	}
	return raw
}

// attachNatAccess 补齐 NAT 机型的访问地址（doc122 缺陷修复）。
//
// 背景：NAT 机型（ip_num=0）没有独立公网 IP，客户唯一的入口是节点转发地址
// nat_host_ip:ext_port。详情页只显示 public_ip 的话，NAT 客户的"连接信息"是空的——
// 机器开着却无从下手。所以这里把转发地址读出来一并下发。
//
// 只对确认是 NAT 的实例发起请求：独立 IP 机型的面板端点返回 null，白跑一次请求没意义
// （更重要的是本方法可能被详情页高频调用）。
//
// 容错：读取失败只记 Error 不返回错误——详情页其余信息（含"是不是 NAT"这个结论）
// 仍应正常展示，否则上游抖动会让整个详情页打不开。
func (s *instanceService) attachNatAccess(ctx context.Context, row *repository.InstanceRow, detail *dto.DetailInfo) {
	if row == nil || detail == nil {
		return
	}
	isNat := instanceIsNat(row)
	if !isNat {
		return
	}
	info := &dto.NatAccessInfo{IsNat: true}
	detail.NatAccess = info

	if !provisioned(row) {
		// 待开通实例还没有上游实例号，读不到规则属于正常，不当作错误。
		return
	}
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		info.Error = err.Error()
		return
	}
	reader, ok := provider.(upstream.NatReader)
	if !ok {
		// 渠道没声明 NAT 读取能力：不是错误，只是拿不到地址。
		info.Error = "当前渠道不支持读取 NAT 访问地址"
		return
	}
	mark := row.InstanceMark()
	forwards, ferr := reader.ListNatForwards(ctx, mark)
	if ferr != nil {
		info.Error = ferr.Error()
		return
	}
	for _, f := range forwards {
		if addr := upstream.FormatNatForwardAddr(f.HostIP, f.ExternalPort); addr != "" {
			info.Forwards = append(info.Forwards, addr)
		}
	}
	webs, werr := reader.ListNatWebs(ctx, mark)
	if werr != nil {
		// 端口转发已拿到时不因建站读失败而整体失败，只在两者都空时提示。
		if len(info.Forwards) == 0 {
			info.Error = werr.Error()
		}
		return
	}
	for _, w := range webs {
		if w.Domain != "" {
			info.Domains = append(info.Domains, w.Domain)
		}
	}
}

// instanceIsNat 判断实例是否走 NAT 链路（无独立 IP）。
// 实现放在 upstream 包（IsNatInstance），与管理端/用户中心共用同一口径，
// 避免同一条实例在两边显示不同的连接方式。
func instanceIsNat(row *repository.InstanceRow) bool {
	if row == nil {
		return false
	}
	return upstream.IsNatInstance(row.RawData)
}

// mergeInstanceConfigs 用实例表字段覆盖配置项取值（CPU/内存/磁盘/带宽/系统/区域/可用区）。
//
// 为什么以实例字段为准：配置项表记录的是"商品允许什么"，实例字段记录的是"这台机器实际
// 是什么"——后者经回源刷新，是运行态真相。两者不一致时运营要看的是后者。
//
// 同时改单位：商品配置项的单位是为"可选项展示"写的（内存可能标 GB 而实例列存 MB），
// 一旦取值换成实例字段，单位必须跟着换成实例字段的真实口径，否则会出现"4096 GB"。
func mergeInstanceConfigs(items []dto.ConfigItem, row *repository.InstanceRow) []dto.ConfigItem {
	if row == nil {
		return items
	}
	actual := map[string]string{
		"cpu":              intToString(row.CPU),
		"memory":           intToString(row.Memory),
		"disk":             intToString(row.Disk),
		"system_disk_size": intToString(row.Disk),
		"bw":               intToString(row.Bandwidth),
		"bandwidth":        intToString(row.Bandwidth),
		"os":               row.OS,
		"area":             row.Region,
		"region":           row.Region,
		"node":             row.Zone,
		"zone":             row.Zone,
	}
	instanceUnits := map[string]string{
		"cpu":              "核",
		"memory":           "MB",
		"disk":             "GB",
		"system_disk_size": "GB",
		"bw":               "Mbps",
		"bandwidth":        "Mbps",
	}
	out := make([]dto.ConfigItem, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		if v, ok := actual[it.Key]; ok && v != "" {
			it.Value, it.Source = v, "instance"
			if u, ok := instanceUnits[it.Key]; ok {
				it.Unit = u
			}
			seen[it.Key] = true
		}
		out = append(out, it)
	}
	// 商品没登记但实例有的关键项补一行，避免"配置区缺 CPU"这种匪夷所思的展示。
	// disk 与 system_disk_size 同源（instances.disk 记的是系统盘），商品已登记
	// 系统盘时不再补磁盘行，否则同一个值会出现两遍。
	for _, key := range []string{"cpu", "memory", "disk", "bw", "os", "area", "node"} {
		if seen[key] {
			continue
		}
		if key == "disk" && seen["system_disk_size"] {
			continue
		}
		v := actual[key]
		if v == "" {
			continue
		}
		out = append(out, dto.ConfigItem{
			Key:    key,
			Label:  instanceConfigLabel(key),
			Value:  v,
			Source: "instance",
			Unit:   instanceUnits[key],
		})
	}
	return out
}

// instanceConfigLabel 实例字段的展示名（补齐配置项时用）。
func instanceConfigLabel(key string) string {
	switch key {
	case "cpu":
		return "CPU"
	case "memory":
		return "内存"
	case "disk":
		return "磁盘"
	case "bw", "bandwidth":
		return "带宽"
	case "os":
		return "操作系统"
	case "area", "region":
		return "区域"
	case "node", "zone":
		return "可用区"
	default:
		return key
	}
}

func intToString(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

// Power 电源操作：开机/关机/硬关机/重启/硬重启。
func (s *instanceService) Power(ctx context.Context, op Operator, id uint64, action string) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}

	normalized, logAction, err := normalizeAction(action)
	if err != nil {
		return err
	}
	// 过渡态拦截（迁移 112）：同一台机器上还有未确认的动作时，拒绝新指令。
	//
	// 为什么必须拦在服务端：上游下发是异步的，界面上的按钮 loading 只覆盖了
	// "接口往返"那几秒；真正的执行窗口有几十秒到几分钟。期间运营刷新页面、
	// 或换个标签页再点一次，前端 loading 完全拦不住 —— 同一台机器会堆多次重启
	// （用户实测反馈）。服务端的过渡态是唯一能跨页面、跨会话生效的闸门。
	if err := s.ensureNoActiveTransition(row, logAction); err != nil {
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, row.Status, row.Status, err)
		return err
	}
	if err := ensurePowerAllowed(row.Status, normalized); err != nil {
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, row.Status, row.Status, err)
		return err
	}

	// **原子抢占过渡态**：先占位再下发，保证并发的重复点击只有一个能到上游。
	// 放在状态守卫之后：守卫拦掉的请求（已在运行中/已关机）不该占位。
	//
	// 抢不到说明另一个并发请求刚抢先（上面那次读到的是旧行，看不到它）。
	// 此时返回与 ensureNoActiveTransition 同样的提示，让用户知道有人先动手了。
	transition := transitionOfPowerAction(normalized)
	claimed, claimedAt, _, cerr := s.claimTransition(ctx, row.ID, transition)
	if cerr != nil {
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, row.Status, row.Status, cerr)
		return cerr
	}
	if !claimed {
		busyErr := s.transitionConflict(ctx, row)
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, row.Status, row.Status, busyErr)
		return busyErr
	}

	before := row.Status
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		s.releaseTransition(ctx, row.ID, transition)
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, before, before, err)
		return err
	}
	ctrl, ok := provider.(upstream.InstanceControl)
	if !ok {
		err := ErrCapabilityUnsupported
		s.releaseTransition(ctx, row.ID, transition)
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, before, before, err)
		return err
	}
	// 硬关/硬重启优先走平台的独立硬电源指令（有则用），否则退回 force 语义。
	hard, hasHard := provider.(upstream.InstancePowerHard)
	mark := row.InstanceMark()

	var opErr error
	switch normalized {
	case "on":
		opErr = ctrl.StartInstance(ctx, mark)
	case "off":
		opErr = ctrl.StopInstance(ctx, mark, false)
	case "hard_off":
		if hasHard {
			opErr = hard.HardStopInstance(ctx, mark)
		} else {
			opErr = ctrl.StopInstance(ctx, mark, true)
		}
	case "reboot":
		opErr = ctrl.RestartInstance(ctx, mark)
	case "hard_reboot":
		if hasHard {
			opErr = hard.HardRestartInstance(ctx, mark)
		} else {
			// 上游没有独立硬重启：退化为硬关再开（语义等价，中间态可见）。
			if opErr = ctrl.StopInstance(ctx, mark, true); opErr == nil {
				opErr = ctrl.StartInstance(ctx, mark)
			}
		}
	}

	after := before
	if opErr == nil {
		after = expectedStatus(normalized, before)
		_ = s.repo.UpdateStatus(ctx, row.ID, after)
		// 过渡态已在上面**抢占**写入（claimTransition）—— 抢占那一刻界面就该看到
		// 「重启中」并拦截重复指令，不必等指令下发完（下发本身也要一个往返）。
		//
		// 上游状态有延迟，回源一次让列表与上游对齐；失败不影响本次操作结果。
		//
		// staleStatus=before：上游还没转过来时回源拿到的是**操作前的旧状态**，
		// 此时不能让回源覆盖刚刚写入的前瞻状态 —— 否则「关机」成功后又立刻变回
		// 「运行中」，紧接着的「开机」被状态守卫判成"实例已在运行中"而失败
		// （实测：连点关机/开机必定复现，运营以为是 500 报错）。
		//
		// transition 非空：回源若已确认新状态，说明上游执行完了，可以清过渡态。
		_ = s.refreshUpstreamFromTransition(ctx, row, before, transition, claimedAt)
	} else {
		// 下发失败：把自己抢到的占位还回去（仅当仍属于自己），
		// 否则实例会被一个从未执行的动作锁到窗口超时。
		s.releaseTransition(ctx, row.ID, transition)
	}
	s.record(ctx, row, op, logAction, map[string]any{"action": action}, before, after, opErr)
	return opErr
}

// VNC 获取远程控制台地址（高危：拿到控制台等于拿到客户机器，需 instance:console）。
func (s *instanceService) VNC(ctx context.Context, op Operator, id uint64) (*dto.VNCResult, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return nil, err
	}
	ctrl, err := s.control(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionVNC, nil, row.Status, row.Status, err)
		return nil, err
	}
	res, err := ctrl.VNC(ctx, row.InstanceID)
	s.record(ctx, row, op, model.ActionVNC, nil, row.Status, row.Status, err)
	if err != nil {
		return nil, err
	}
	return &dto.VNCResult{URL: res.URL, Password: res.Password, External: res.External}, nil
}

// Sync 单实例回源刷新（回写状态/IP/到期/原始数据）。
func (s *instanceService) Sync(ctx context.Context, op Operator, id uint64) (*dto.DetailInfo, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	before := row.Status
	if !provisioned(row) {
		// 待开通实例：本地还没有上游实例号。但**不代表上游一定没有机器** ——
		// "余额不足 → 在上游侧充值并付掉账单"这条恢复链里，机器只存在于上游，
		// 本地无从得知（实测用户场景：上游已开好，本地一直显示创建中）。
		//
		// 所以这里不只是报"无对象可刷"：顺带重投一次开通核对（复用「重新开通」的
		// 幂等投递：带下游令牌握手 → 上游有主机就接管，没有则复用原账单补付。
		// 绝不重复下单）。投递成功即告知"已提交核对"，由工作池秒级完成。
		if s.provisionRetry != nil {
			if rerr := s.provisionRetry(ctx, row.OrderID); rerr != nil {
				s.record(ctx, row, op, model.ActionSync, nil, before, before, rerr)
				return nil, rerr
			}
			s.record(ctx, row, op, model.ActionSync, nil, before, before, ErrNothingToSync)
			return nil, ErrNothingToSync
		}
		// 未装配重投能力：给出可操作说明而不是把本地占位号发到上游换回一个费解的报错。
		s.record(ctx, row, op, model.ActionSync, nil, before, before, ErrNothingToSync)
		return nil, ErrNothingToSync
	}
	sErr := s.refreshUpstream(ctx, row)
	// 手动同步兼作"过渡态的人工收尾"：运营看到「重启中」一直不消失时点同步，
	// 就宣告"我已经核对过了"，可以收尾。
	//
	// 判据只看"回源成功 + 已过最短占用窗口"：
	//   · 回源失败 → 保留过渡态（宁可多显示一会儿，也不假装完成）；
	//   · 窗口内 → 保留，避免"点完立刻点同步"绕过拦截把锁提前拿掉。
	//
	// 刻意**不**再要求"状态发生了变化"：下发时已乐观写入目标状态（关机写 stopped），
	// 回源拿到的往往与它相同，于是"没变化"成了常态 —— 那会让过渡态一直挂到窗口
	// 超时（实测：开完机点同步，「开机中」不消失，最长要等 3 分钟）。
	if sErr == nil && row.TransitionState != "" {
		if fresh, ferr := s.repo.FindByID(ctx, id); ferr == nil && fresh.TransitionState != "" &&
			!transitionWithinMinHold(fresh.TransitionState, derefTime(fresh.TransitionStartedAt)) {
			_ = s.repo.ClearTransition(ctx, id)
		}
	}
	after := before
	if fresh, ferr := s.repo.FindByID(ctx, id); ferr == nil {
		after = fresh.Status
	}
	s.record(ctx, row, op, model.ActionSync, nil, before, after, sErr)
	if sErr != nil {
		return nil, sErr
	}
	return s.Detail(ctx, id, false)
}

// Resize 变配（上游须实现 InstanceAdministration）。
//
// 注意：本阶段只变更上游规格并回写本地快照，**不含补差价计费**（见 61 实施计划 §1.3/§9-P2）。
func (s *instanceService) Resize(ctx context.Context, op Operator, id uint64, req *dto.ResizeRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil {
		return ErrInvalidAction
	}
	specs := &pkgmodel.StandardProductSpec{
		CPU:      pickInt(req.CPU, row.CPU),
		Memory:   pickInt(req.Memory, row.Memory),
		Disk:     pickInt(req.Disk, row.Disk),
		DiskType: pickString(req.DiskType, row.DiskType),
	}
	params := map[string]any{
		"cpu": specs.CPU, "memory": specs.Memory, "disk": specs.Disk, "disk_type": specs.DiskType,
		"reason": req.Reason,
	}

	before := row.Status
	admin, err := s.administration(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionResize, params, before, before, err)
		return err
	}
	if err := admin.ResizeInstance(ctx, row.InstanceID, specs); err != nil {
		s.record(ctx, row, op, model.ActionResize, params, before, before, err)
		return err
	}
	// 回写本地规格快照，避免列表继续显示旧配置。
	_ = s.repo.UpdateSpecs(ctx, row.ID, req.CPU, req.Memory, req.Disk, req.DiskType)
	s.record(ctx, row, op, model.ActionResize, params, before, before, nil)
	return nil
}

// Reinstall 重装系统（可换镜像）。
//
// 平台重装是长任务：调用成功只代表"已受理"。平台会在响应里给出新系统的初始凭据
// （魔方云返回 user/password）。本库 instances 表**没有**凭据列（开通时也不落盘），
// 因此这里不写库，而是把凭据**原样回传**给调用方，由界面一次性展示给运营/客户
// ——丢了就只能去面板重置。流水里只记"已签发新凭据"，绝不落密码明文。
func (s *instanceService) Reinstall(ctx context.Context, op Operator, id uint64, req *dto.ReinstallRequest) (*dto.ReinstallResult, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.OS) == "" {
		return nil, fmt.Errorf("%w: 请选择目标镜像", ErrInvalidAction)
	}
	if row.Status == string(pkgmodel.InstanceStatusDeleted) {
		return nil, fmt.Errorf("%w: 实例已销毁，无法重装", ErrStatusConflict)
	}
	// 过渡态闸门：重装要重建系统盘，与任何进行中的动作互斥
	//（重装到一半再叠加一个重装/重启只会让状态更乱，且无法判断最终结果）。
	if err := s.ensureNoActiveTransition(row, model.ActionReinstall); err != nil {
		s.record(ctx, row, op, model.ActionReinstall, nil, row.Status, row.Status, err)
		return nil, err
	}
	// 原子抢占（与电源同一口径）：两个并发重装请求只有一个能下发。
	claimed, _, _, cerr := s.claimTransition(ctx, row.ID, pkgmodel.InstanceTransitionReinstalling)
	if cerr != nil {
		s.record(ctx, row, op, model.ActionReinstall, nil, row.Status, row.Status, cerr)
		return nil, cerr
	}
	if !claimed {
		busyErr := s.transitionConflict(ctx, row)
		s.record(ctx, row, op, model.ActionReinstall, nil, row.Status, row.Status, busyErr)
		return nil, busyErr
	}
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		s.releaseTransition(ctx, row.ID, pkgmodel.InstanceTransitionReinstalling)
		s.record(ctx, row, op, model.ActionReinstall, nil, row.Status, row.Status, err)
		return nil, err
	}
	reinstaller, ok := provider.(upstream.InstanceReinstall)
	if !ok {
		err := fmt.Errorf("%w: %s", ErrCapabilityUnsupported, "该服务商不支持重装系统")
		s.releaseTransition(ctx, row.ID, pkgmodel.InstanceTransitionReinstalling)
		s.record(ctx, row, op, model.ActionReinstall, nil, row.Status, row.Status, err)
		return nil, err
	}
	params := map[string]any{
		"os": req.OS, "port": req.Port,
		"format_data_disk": req.FormatDataDisk, "system_disk_size": req.SystemDiskSize,
		"reason": req.Reason,
	}
	before := row.Status
	res, rerr := reinstaller.ReinstallInstance(ctx, &upstream.ReinstallRequest{
		ProviderInstanceID: row.InstanceMark(),
		OS:                 req.OS,
		Port:               req.Port,
		FormatDataDisk:     req.FormatDataDisk,
		SystemDiskSize:     req.SystemDiskSize,
	})
	if rerr != nil {
		s.releaseTransition(ctx, row.ID, pkgmodel.InstanceTransitionReinstalling)
		s.record(ctx, row, op, model.ActionReinstall, params, before, before, rerr)
		return nil, rerr
	}
	out := &dto.ReinstallResult{}
	if res != nil {
		out.Username = res.Username
		out.Password = res.Password
		if res.Password != "" {
			params["credential_issued"] = true
			// 平台重装会重新生成初始凭据：同步覆盖留存，让「查看密码」给出的是重装后
			// 真正可用的那一个（否则用户会拿一个已被重装作废的旧密码去登）。
			params["credential_saved"] = s.saveCredentialPassword(ctx, row, res.Password)
		}
	}
	s.record(ctx, row, op, model.ActionReinstall, params, before, before, nil)
	// 重装的过渡态已在上面**抢占**写入：界面立即显示「重装中」并拦住重复提交，
	// 不必等上游回"已受理"（那也是一个往返）。
	return out, nil
}

// ResetPassword 重置实例登录密码（不重装，系统与数据保留）。
func (s *instanceService) ResetPassword(ctx context.Context, op Operator, id uint64, req *dto.ResetPasswordRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil || strings.TrimSpace(req.Password) == "" {
		return fmt.Errorf("%w: 新密码不能为空", ErrInvalidAction)
	}
	if row.Status == string(pkgmodel.InstanceStatusDeleted) {
		return fmt.Errorf("%w: 实例已销毁，无法重置密码", ErrStatusConflict)
	}
	// 过渡态闸门：重置密码通常伴随强制重启，与进行中的电源动作叠加会让
	// "最终密码/最终状态是哪个"无法判断。
	if err := s.ensureNoActiveTransition(row, model.ActionResetPassword); err != nil {
		s.record(ctx, row, op, model.ActionResetPassword, nil, row.Status, row.Status, err)
		return err
	}
	// 原子抢占：连点两次重置密码同样只允许一个下发。
	claimed, _, _, cerr := s.claimTransition(ctx, row.ID, pkgmodel.InstanceTransitionResettingPassword)
	if cerr != nil {
		s.record(ctx, row, op, model.ActionResetPassword, nil, row.Status, row.Status, cerr)
		return cerr
	}
	if !claimed {
		busyErr := s.transitionConflict(ctx, row)
		s.record(ctx, row, op, model.ActionResetPassword, nil, row.Status, row.Status, busyErr)
		return busyErr
	}
	resetter, err := s.passwordResetter(ctx, row.ProviderID)
	if err != nil {
		s.releaseTransition(ctx, row.ID, pkgmodel.InstanceTransitionResettingPassword)
		s.record(ctx, row, op, model.ActionResetPassword, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	// 流水里只记"已重置"，绝不落密码明文。
	params := map[string]any{"password_changed": true}
	if perr := resetter.ResetInstancePassword(ctx, row.InstanceMark(), req.Password); perr != nil {
		s.releaseTransition(ctx, row.ID, pkgmodel.InstanceTransitionResettingPassword)
		s.record(ctx, row, op, model.ActionResetPassword, params, before, before, perr)
		return perr
	}
	// 新密码即刻成为权威值：留存必须同步覆盖，否则「查看密码」会一直显示旧密码
	// （旧密码在重置成功那一刻已经失效，比查不到更误导）。
	// 留存失败不影响重置结果本身（机器上的密码确实改了），只记流水说明查看会不准。
	params["credential_saved"] = s.saveCredentialPassword(ctx, row, req.Password)
	s.record(ctx, row, op, model.ActionResetPassword, params, before, before, nil)
	// 过渡态已在上面**抢占**写入：界面立即显示「重置密码中」并拦住重复提交，
	// 提示运营这段时间机器可能不可达（比只弹一句"已提交"更贴近实际）。
	return nil
}

// saveCredentialPassword 把新密码加密留存到实例行（迁移 100），返回是否成功。
//
// 只存密文；未配置加密密钥或加密失败时返回 false —— 调用方据此在流水里留痕，
// 绝不退化为明文落库。失败原因写日志（不含密码本身）。
func (s *instanceService) saveCredentialPassword(ctx context.Context, row *repository.InstanceRow, plaintext string) bool {
	if row == nil || strings.TrimSpace(plaintext) == "" {
		return false
	}
	if strings.TrimSpace(s.secretKey) == "" {
		if s.logger != nil {
			s.logger.Warn("未配置加密密钥，跳过实例密码留存（查看密码将不可用）",
				zap.Uint64("instance_id", row.ID))
		}
		return false
	}
	enc, err := credentials.EncryptSecret(plaintext, s.secretKey)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("实例密码加密失败，跳过留存", zap.Uint64("instance_id", row.ID), zap.Error(err))
		}
		return false
	}
	if err := s.repo.UpdateCredentialPassword(ctx, row.ID, enc); err != nil {
		if s.logger != nil {
			s.logger.Warn("实例密码留存写入失败", zap.Uint64("instance_id", row.ID), zap.Error(err))
		}
		return false
	}
	return true
}

// Rescue 进入救援系统（system 为平台口径的救援系统类型，tempPassword 为临时密码）。
func (s *instanceService) Rescue(ctx context.Context, op Operator, id uint64, req *dto.RescueRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil || req.System == 0 || strings.TrimSpace(req.TempPassword) == "" {
		return fmt.Errorf("%w: 请选择救援系统类型并填写临时密码", ErrInvalidAction)
	}
	if row.Status == string(pkgmodel.InstanceStatusDeleted) {
		return fmt.Errorf("%w: 实例已销毁，无法进入救援系统", ErrStatusConflict)
	}
	rescuer, err := s.rescuer(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionRescue, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	params := map[string]any{"system": req.System}
	if rerr := rescuer.RescueInstance(ctx, row.InstanceMark(), req.System, req.TempPassword); rerr != nil {
		s.record(ctx, row, op, model.ActionRescue, params, before, before, rerr)
		return rerr
	}
	s.record(ctx, row, op, model.ActionRescue, params, before, before, nil)
	return nil
}

// ExitRescue 退出救援系统（回到原系统）。
func (s *instanceService) ExitRescue(ctx context.Context, op Operator, id uint64) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	rescuer, err := s.rescuer(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionExitRescue, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	if rerr := rescuer.ExitRescueInstance(ctx, row.InstanceMark()); rerr != nil {
		s.record(ctx, row, op, model.ActionExitRescue, nil, before, before, rerr)
		return rerr
	}
	s.record(ctx, row, op, model.ActionExitRescue, nil, before, before, nil)
	return nil
}

// Snapshots 列出实例的快照/备份（snapshotType 为空表示两类都返回）。
func (s *instanceService) Snapshots(ctx context.Context, id uint64, snapshotType string) ([]dto.SnapshotInfo, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return nil, err
	}
	snapshotter, err := s.snapshotter(ctx, row.ProviderID)
	if err != nil {
		return nil, err
	}
	rows, err := snapshotter.ListSnapshots(ctx, row.InstanceMark(), snapshotType)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SnapshotInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.SnapshotInfo{
			ID: r.ID, Name: r.Name, Type: r.Type, Size: r.Size, Status: r.Status,
			DiskID: r.DiskID, DiskName: r.DiskName, CreateTime: r.CreateTime, Remarks: r.Remarks,
		})
	}
	return out, nil
}

// CreateSnapshot 创建快照/备份。diskID 留空时取实例系统盘（快照挂在磁盘上）。
func (s *instanceService) CreateSnapshot(ctx context.Context, op Operator, id uint64, req *dto.SnapshotCreateRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil {
		req = &dto.SnapshotCreateRequest{}
	}
	if row.Status == string(pkgmodel.InstanceStatusDeleted) {
		return fmt.Errorf("%w: 实例已销毁，无法创建快照", ErrStatusConflict)
	}
	snapshotter, err := s.snapshotter(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionSnapCreate, nil, row.Status, row.Status, err)
		return err
	}
	diskID := strings.TrimSpace(req.DiskID)
	if diskID == "" {
		// 平台要求指定磁盘；测试/既有实例的系统盘 ID 需要回源取。
		diskID = s.systemDiskID(ctx, row)
	}
	if diskID == "" {
		err := fmt.Errorf("%w: 无法确定实例的系统盘 ID，请显式指定磁盘", ErrInvalidAction)
		s.record(ctx, row, op, model.ActionSnapCreate, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	snapType := strings.TrimSpace(req.Type)
	if snapType == "" {
		snapType = upstream.SnapshotTypeSnap
	}
	params := map[string]any{"type": snapType, "name": req.Name, "disk_id": diskID}
	if cerr := snapshotter.CreateSnapshot(ctx, &upstream.CreateSnapshotRequest{
		ProviderInstanceID: row.InstanceMark(),
		DiskID:             diskID,
		Type:               req.Type,
		Name:               req.Name,
	}); cerr != nil {
		s.record(ctx, row, op, model.ActionSnapCreate, params, before, before, cerr)
		return cerr
	}
	s.record(ctx, row, op, model.ActionSnapCreate, params, before, before, nil)
	return nil
}

// DeleteSnapshot 删除快照/备份（按快照 ID）。
func (s *instanceService) DeleteSnapshot(ctx context.Context, op Operator, id uint64, snapshotID string) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if strings.TrimSpace(snapshotID) == "" {
		return fmt.Errorf("%w: 缺少快照 ID", ErrInvalidAction)
	}
	snapshotter, err := s.snapshotter(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionSnapDelete, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	params := map[string]any{"snapshot_id": snapshotID}
	if derr := snapshotter.DeleteSnapshot(ctx, snapshotID); derr != nil {
		s.record(ctx, row, op, model.ActionSnapDelete, params, before, before, derr)
		return derr
	}
	s.record(ctx, row, op, model.ActionSnapDelete, params, before, before, nil)
	return nil
}

// RestoreSnapshot 用快照/备份恢复实例（高危：会覆盖当前系统盘，需二次确认）。
func (s *instanceService) RestoreSnapshot(ctx context.Context, op Operator, id uint64, req *dto.SnapshotRestoreRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil || strings.TrimSpace(req.SnapshotID) == "" {
		return fmt.Errorf("%w: 缺少快照 ID", ErrInvalidAction)
	}
	mark := strings.TrimSpace(req.ConfirmMark)
	if mark != row.InstanceID && mark != strconv.FormatUint(row.ID, 10) {
		return fmt.Errorf("%w: 请准确输入实例标识 %s", ErrPermissionDenied, row.InstanceID)
	}
	snapshotter, err := s.snapshotter(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionSnapRestore, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	params := map[string]any{"snapshot_id": req.SnapshotID}
	if rerr := snapshotter.RestoreSnapshot(ctx, row.InstanceMark(), req.SnapshotID); rerr != nil {
		s.record(ctx, row, op, model.ActionSnapRestore, params, before, before, rerr)
		return rerr
	}
	s.record(ctx, row, op, model.ActionSnapRestore, params, before, before, nil)
	return nil
}

// SetBandwidth 直接修改上下行带宽（硬件变更，不经变配流程）。
func (s *instanceService) SetBandwidth(ctx context.Context, op Operator, id uint64, req *dto.BandwidthRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil || (req.InBw <= 0 && req.OutBw <= 0) {
		return fmt.Errorf("%w: 至少需要一个方向的带宽值", ErrInvalidAction)
	}
	hw, err := s.hardware(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionBandwidth, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	params := map[string]any{"in_bw": req.InBw, "out_bw": req.OutBw, "reason": req.Reason}
	if uerr := hw.UpdateBandwidth(ctx, row.InstanceMark(), req.InBw, req.OutBw); uerr != nil {
		s.record(ctx, row, op, model.ActionBandwidth, params, before, before, uerr)
		return uerr
	}
	if req.InBw > 0 || req.OutBw > 0 {
		bw := req.InBw
		if bw == 0 {
			bw = req.OutBw
		}
		_ = s.repo.UpdateBandwidth(ctx, row.ID, bw)
	}
	s.record(ctx, row, op, model.ActionBandwidth, params, before, before, nil)
	return nil
}

// AddIP 增加 IP 或 IPv6 地址。
func (s *instanceService) AddIP(ctx context.Context, op Operator, id uint64, req *dto.AddIPRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil || req.Num <= 0 {
		return fmt.Errorf("%w: 数量必须大于 0", ErrInvalidAction)
	}
	hw, err := s.hardware(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionAddIP, nil, row.Status, row.Status, err)
		return err
	}
	version := req.Version
	if version == 0 {
		version = 4
	}
	before := row.Status
	params := map[string]any{"version": version, "num": req.Num, "ip_group": req.IPGroup}
	var aerr error
	if version == 6 {
		aerr = hw.AddIPv6(ctx, row.InstanceMark(), req.Num)
	} else {
		aerr = hw.AddIPs(ctx, row.InstanceMark(), req.Num, req.IPGroup)
	}
	if aerr != nil {
		s.record(ctx, row, op, model.ActionAddIP, params, before, before, aerr)
		return aerr
	}
	s.record(ctx, row, op, model.ActionAddIP, params, before, before, nil)
	return nil
}

// AttachDataDisk 挂载数据盘。
func (s *instanceService) AttachDataDisk(ctx context.Context, op Operator, id uint64, req *dto.AttachDiskRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil || req.SizeGB <= 0 {
		return fmt.Errorf("%w: 数据盘大小必须大于 0", ErrInvalidAction)
	}
	hw, err := s.hardware(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionAttachDisk, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	params := map[string]any{"size_gb": req.SizeGB, "store": req.Store, "reason": req.Reason}
	if aerr := hw.AttachDataDisk(ctx, row.InstanceMark(), req.SizeGB, req.Store); aerr != nil {
		s.record(ctx, row, op, model.ActionAttachDisk, params, before, before, aerr)
		return aerr
	}
	if uerr := s.repo.UpdateDisk(ctx, row.ID, row.Disk+req.SizeGB); uerr != nil {
		s.record(ctx, row, op, model.ActionAttachDisk, params, before, before,
			fmt.Errorf("数据盘已挂载，但本地磁盘容量回写失败（下次同步会纠正）：%w", uerr))
		return nil
	}
	s.record(ctx, row, op, model.ActionAttachDisk, params, before, before, nil)
	return nil
}

// systemDiskID 取实例系统盘的平台 ID（快照必须挂在磁盘上）。
// 平台详情里 disk[] 每项带 id/type，type=system 的那块即系统盘。
func (s *instanceService) systemDiskID(ctx context.Context, row *repository.InstanceRow) string {
	ctrl, err := s.control(ctx, row.ProviderID)
	if err != nil {
		return ""
	}
	inst, err := ctrl.GetInstance(ctx, row.InstanceID)
	if err != nil || inst == nil || inst.RawData == nil {
		return ""
	}
	disks, ok := inst.RawData["disk"].([]interface{})
	if !ok {
		// 平台有时返回 JSON 字符串或 map，统一按 JSON 重新解。
		raw, merr := json.Marshal(inst.RawData["disk"])
		if merr != nil {
			return ""
		}
		_ = json.Unmarshal(raw, &disks)
	}
	for _, d := range disks {
		if m, ok := d.(map[string]interface{}); ok && fmt.Sprintf("%v", m["type"]) == "system" {
			return fmt.Sprintf("%v", m["id"])
		}
	}
	// 找不到 system 时退回第一块盘（多数实例只有系统盘）。
	if len(disks) > 0 {
		if m, ok := disks[0].(map[string]interface{}); ok {
			return fmt.Sprintf("%v", m["id"])
		}
	}
	return ""
}

// SetRemark 写管理员内部备注（同步流程不会覆盖该字段）。
func (s *instanceService) SetRemark(ctx context.Context, op Operator, id uint64, remark string) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	remark = strings.TrimSpace(remark)
	if len(remark) > 255 {
		remark = remark[:255]
	}
	if err := s.repo.UpdateRemark(ctx, id, remark); err != nil {
		return err
	}
	// 备注也属实例语义操作，必须留痕（doc61「有操作必有记录」）。
	s.record(ctx, row, op, model.ActionRemark,
		map[string]any{"remark": remark}, row.Status, row.Status, nil)
	return nil
}

// AdjustBilling 手工调整账期/付款周期/绑定续费价（管理端自定义，见 dto.AdjustRequest 注释）。
//
// 安全边界（刻在代码里，不随调用方改变）：
//   - 至少改一样，否则是空操作，直接拒绝（避免前端误发空请求留下无意义流水）；
//   - 对接商品（source_mode=upstream）不加 force 一律拒绝：上游账期才是权威，
//     本地改了会被回源覆盖，而生命周期推进器可能按这个本地值误停服；
//   - 到期时间只接受未来时间或清空（把到期时间改到过去等于立刻判死一个在跑的机器，
//     要停服请用「暂停」——那是显式语义且有平台动作）；
//   - 付款周期必须被**上游支持**（对接链路）：选一个上游不卖的周期（如给只支持
//     monthly/quarterly 的渠道选「两年」），下游会按该周期推账单，上游按别的周期扣费，
//     钱与账期同时错位（实测缺陷）。
func (s *instanceService) AdjustBilling(ctx context.Context, op Operator, id uint64, req *dto.AdjustRequest) (*dto.DetailInfo, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrBillingAdjustInvalid
	}
	expireRaw := strings.TrimSpace(req.ExpireAt)
	billingMode := strings.TrimSpace(req.BillingMode)
	if expireRaw == "" && !req.ClearExpire && billingMode == "" && req.RenewalPrice == nil {
		return nil, fmt.Errorf("%w: 请至少填写到期时间、付款周期或续费价", ErrBillingAdjustInvalid)
	}
	if billingMode != "" {
		normalized := billingcycle.Normalize(billingMode)
		if !billingcycle.IsValid(normalized) {
			return nil, fmt.Errorf("%w: 不支持的付款周期 %s", ErrBillingAdjustInvalid, billingMode)
		}
		// 上游核对：对接链路的目标周期必须在上游支持列表里。
		// 只对"要改成别的周期"做校验——若改的就是当前周期（或读不到上游列表），不拦。
		if normalized != row.BillingMode {
			if supported, ok := s.upstreamCycles(ctx, row.ProviderID); ok && len(supported) > 0 && !containsString(supported, normalized) {
				return nil, fmt.Errorf("%w: 上游（%s）不支持「%s」，可选：%s",
					ErrBillingAdjustInvalid, row.ProviderName, billingcycle.Name(normalized), joinCycleNames(supported))
			}
		}
		billingMode = normalized
	}
	if row.SourceMode == syncmodel.SourceModeUpstream && !req.Force {
		return nil, fmt.Errorf("%w: 该实例为上游对接链路，账期以上游为准；确需本地调整请勾选「强制本地调整」", ErrStatusConflict)
	}
	var expireAt *time.Time
	if expireRaw != "" {
		t, perr := parseFlexibleTime(expireRaw)
		if perr != nil {
			return nil, fmt.Errorf("%w: 到期时间格式无法识别", ErrBillingAdjustInvalid)
		}
		if !t.After(time.Now()) {
			return nil, fmt.Errorf("%w: 到期时间必须晚于当前时间；要停服请使用「暂停服务」", ErrBillingAdjustInvalid)
		}
		expireAt = &t
	}
	params := map[string]any{
		"before_expire_at": fmtTimePtr(row.ExpireAt),
		"before_billing":   row.BillingMode,
		"expire_at":        expireRaw,
		"clear_expire":     req.ClearExpire,
		"billing_mode":     billingMode,
		"force":            req.Force,
		"reason":           req.Reason,
	}
	before := row.Status
	if aerr := s.repo.AdjustBilling(ctx, row.ID, expireAt, req.ClearExpire, billingMode); aerr != nil {
		s.record(ctx, row, op, model.ActionBillingAdjust, params, before, before, aerr)
		return nil, aerr
	}
	// 绑定续费价：只在显式传了 renewal_price 时才动这一列（nil = 本次不改）。
	// 负值 = 清除绑定（回到按商品实时算价），0 视为非法（0 元续费会被续费守卫拒绝，
	// 与其存一个永远算不出钱的绑定价，不如明确报错）。
	if req.RenewalPrice != nil {
		price := money.Round2(*req.RenewalPrice)
		clearPrice := false
		switch {
		case price < 0:
			clearPrice = true
		case price == 0:
			s.record(ctx, row, op, model.ActionBillingAdjust, params, before, before, ErrBillingAdjustInvalid)
			return nil, fmt.Errorf("%w: 续费价必须大于 0；要取消绑定请传 -1", ErrBillingAdjustInvalid)
		}
		params["renewal_price"] = req.RenewalPrice
		params["renewal_price_cleared"] = clearPrice
		if perr := s.repo.SetRenewalPrice(ctx, row.ID, req.RenewalPrice); perr != nil {
			s.record(ctx, row, op, model.ActionBillingAdjust, params, before, before, perr)
			return nil, perr
		}
	}
	s.record(ctx, row, op, model.ActionBillingAdjust, params, before, before, nil)
	return s.Detail(ctx, id, false)
}

// AdjustContext 读账期调整所需的上游周期与价格上下文（对话框数据源）。
//
// 为什么必须由后端算：可选周期以上游能力为准、上游价来自镜像快照、
// 我方价来自价格矩阵 —— 三项都不是前端能推导的，前端自己列周期就会列出上游不卖的档。
func (s *instanceService) AdjustContext(ctx context.Context, id uint64) (*dto.AdjustContext, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	out := &dto.AdjustContext{
		CurrentCycle:        billingcycle.NormalizeOrDefault(row.BillingMode),
		CurrentRenewalPrice: row.RenewalPrice,
		RenewalPriceHint:    "绑定价：续费按此单价 × 期数计费，压过商品调价与折扣（下单/前次调整时谈定的承诺价）。留空表示按商品实时算价。",
	}
	// 上游支持周期：读渠道能力描述符。读失败回落到平台全集并标记未核对
	// ——调整对话框不能因为渠道不可用就打不开，但必须让运营知道列表未核对。
	supported, ok := s.upstreamCycles(ctx, row.ProviderID)
	if !ok || len(supported) == 0 {
		out.UpstreamCyclesUnavailable = true
		supported = billingcycle.AllCodes()
	}
	// 上游各周期价：对接链路才有（读镜像快照，与详情页同一份数据）。
	var upstream *dto.UpstreamInfo
	if resourceID := upstreamResourceID(row); resourceID != 0 && s.detail != nil {
		upstream, _, _ = s.detail.Upstream(ctx, resourceID, row.BillingMode)
	}
	// 我方价格矩阵：售价/成本按周期。
	productPrices := map[string]repository.ProductCyclePrice{}
	if s.detail != nil && row.SellProductID != 0 {
		if m, perr := s.detail.ProductCyclePrices(ctx, row.SellProductID); perr == nil {
			productPrices = m
		}
	}
	out.Cycles = make([]dto.BillingCycleOption, 0, len(supported))
	for _, cycle := range supported {
		opt := dto.BillingCycleOption{
			Cycle:    cycle,
			Name:     billingcycle.Name(cycle),
			Upstream: true,
		}
		if upstream != nil {
			opt.UpstreamList = money.Round2(upstream.ListCycles[cycle])
			opt.UpstreamCost = money.Round2(upstream.CostCycles[cycle])
		}
		if p, ok := productPrices[cycle]; ok {
			opt.Sale = money.Round2(p.Sale)
			opt.Cost = money.Round2(p.Cost)
			opt.HasProductPrice = true
		}
		out.Cycles = append(out.Cycles, opt)
	}
	return out, nil
}

// ReinstallImages 读重装可选镜像（重装对话框下拉数据源）。
//
// 三级来源，按可用性依次回落：
//  1. 商品侧（自营取商品 os 配置项、对接取上游镜像的 os 配置子项）—— 最准，
//     且与开通/下发给上游的取值口径完全一致；
//  2. 渠道镜像目录（自营平台 /resources 的 images，如魔方云已装镜像）；
//  3. 都没有：返回空列表 + 说明，前端提示手填（不假装有数据）。
//
// 单独收口到后端的原因：镜像值在两链条路上语义不同（魔方云=镜像 ID，
// 魔方财务=上游配置子项 ID），前端无从自行推断该用哪套。
func (s *instanceService) ReinstallImages(ctx context.Context, id uint64) (*dto.ReinstallImages, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	out := &dto.ReinstallImages{Images: []dto.ReinstallImage{}, Source: dto.ReinstallSourceNone}

	// 来源 1：商品配置（含对接链路的上游镜像配置子项）。
	if s.detail != nil {
		productID := row.SellProductID
		if productID == 0 && row.UpstreamProductID != 0 {
			// 上游同步发现的实例可能没有本地商品：退回按镜像所属商品的配置读。
			productID = row.SellProductID
		}
		if productID != 0 {
			if imgs, ok, ierr := s.detail.ReinstallOSOptions(ctx, productID); ierr == nil && ok && len(imgs) > 0 {
				out.Images = imgs
				out.Source = dto.ReinstallSourceProduct
				out.Hint = "镜像取自该商品的操作系统配置项（下发给上游的是配置子项 ID，与开通口径一致）"
				return out, nil
			}
		}
	}

	// 来源 2：渠道平台镜像目录（自营平台有已装镜像列表）。
	if pr, perr := s.buildProvider(ctx, row.ProviderID); perr == nil {
		if catalog, ok := pr.(upstream.PlatformResourceReader); ok {
			if res, cerr := catalog.ListPlatformResources(ctx); cerr == nil && len(res.Images) > 0 {
				imgs := make([]dto.ReinstallImage, 0, len(res.Images))
				seen := map[string]bool{}
				for _, img := range res.Images {
					if img.Value == "" || seen[img.Value] {
						continue
					}
					seen[img.Value] = true
					imgs = append(imgs, dto.ReinstallImage{
						Value:  img.Value,
						Label:  firstNonEmptyStr(img.Label, img.Value),
						Group:  img.Group,
						Status: img.Status,
					})
				}
				if len(imgs) > 0 {
					out.Images = imgs
					out.Source = dto.ReinstallSourcePlatform
					out.Hint = "镜像取自渠道平台镜像目录"
					return out, nil
				}
			}
		}
	}

	out.Hint = "未能自动获取镜像列表：该商品未配置操作系统项，渠道也不提供镜像目录。请到上游面板确认镜像 ID 后手填。"
	return out, nil
}

// upstreamCycles 读上游渠道声明的计费周期（能力描述符 BillingCycles），已归一化。
// 第二个返回值表示"是否成功读到"（渠道不可用/未声明时为 false）。
func (s *instanceService) upstreamCycles(ctx context.Context, providerID uint64) ([]string, bool) {
	if providerID == 0 || s.resolve == nil {
		return nil, false
	}
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil || provider == nil {
		return nil, false
	}
	desc := upstream.CapabilitiesOf(provider)
	if len(desc.BillingCycles) == 0 {
		return nil, false
	}
	return billingcycle.NormalizeList(desc.BillingCycles), true
}

// upstreamResourceID 实例绑定的上游镜像 id（resource_products.id）。
func upstreamResourceID(row *repository.InstanceRow) uint64 {
	if row == nil {
		return 0
	}
	return row.UpstreamProductID
}

// containsString 判断切片是否含某值。
func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// joinCycleNames 把周期码拼成中文名列表（错误信息用）。
func joinCycleNames(cycles []string) string {
	names := make([]string, 0, len(cycles))
	for _, c := range cycles {
		names = append(names, billingcycle.Name(c))
	}
	return strings.Join(names, "、")
}

// RelinkUpstream 按上游实例号重新关联（上游换机/换号场景）。
//
// 实测需求：上游把机器换了（或上游侧实例号变了），本地留的还是旧号，
// 此后电源/控制台/续费/销毁全部打在已不存在的机器上，且界面看不出问题。
//
// 三道守卫（按顺序，任一不过即拒）：
//  1. 目标号在上游**真实存在**（GetInstance 探活）：凭空写号 = 把实例指向虚空；
//  2. 目标号未被**别的本地实例**占用：否则两台实例抢同一台机器，
//     一台的操作会串到另一台上；
//  3. ConfirmMark 必须等于当前实例标识或记录 ID（与销毁同口径）：
//     改上游号会改变此后所有上游动作的作用对象，属高影响操作。
func (s *instanceService) RelinkUpstream(ctx context.Context, op Operator, id uint64, req *dto.RelinkRequest) (*dto.RelinkResult, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	if req == nil {
		return nil, ErrInvalidAction
	}
	// 自营实例没有"上游号可换"这回事：它的实例号是本系统生成并下发的。
	// 放行会诱导运营去填一个来路不明的号，把本来正确的机器指到别处。
	if !resaleInstance(row) {
		err := fmt.Errorf("%w: 该实例为自营链路（机器由本平台开通），实例号由本系统管理，无需也无法「按上游号重新关联」", ErrInvalidAction)
		s.record(ctx, row, op, model.ActionRelinkUpstream, nil, row.Status, row.Status, err)
		return nil, err
	}
	target := strings.TrimSpace(req.UpstreamInstanceID)
	if target == "" {
		return nil, fmt.Errorf("%w: 请填写上游实例号", ErrInvalidAction)
	}
	mark := strings.TrimSpace(req.ConfirmMark)
	if mark != row.InstanceID && mark != strconv.FormatUint(row.ID, 10) {
		return nil, fmt.Errorf("%w: 请准确输入当前实例标识 %s", ErrPermissionDenied, row.InstanceID)
	}
	if target == row.ProviderInstanceID {
		return nil, fmt.Errorf("%w: 目标上游实例号与当前一致，无需关联", ErrInvalidAction)
	}
	// 守卫②：目标号不得被别的实例占用。
	if n, cerr := s.repo.CountByProviderInstanceID(ctx, target, row.ID); cerr == nil && n > 0 {
		return nil, fmt.Errorf("%w: 上游实例号 %s 已被本系统另一台实例占用，请先处理那台实例", ErrStatusConflict, target)
	}
	// 守卫①：上游必须真实存在该实例（不探活就把本地指向虚空）。
	provider, perr := s.buildProvider(ctx, row.ProviderID)
	if perr != nil {
		s.record(ctx, row, op, model.ActionRelinkUpstream, nil, row.Status, row.Status, perr)
		return nil, perr
	}
	ctrl, ok := provider.(upstream.InstanceControl)
	if !ok {
		err := fmt.Errorf("%w: %s", ErrCapabilityUnsupported, "该服务商不支持按实例号拉取信息")
		s.record(ctx, row, op, model.ActionRelinkUpstream, nil, row.Status, row.Status, err)
		return nil, err
	}
	inst, ierr := ctrl.GetInstance(ctx, target)
	if ierr != nil {
		err := fmt.Errorf("上游不存在该实例号或拉取失败：%w", ierr)
		s.record(ctx, row, op, model.ActionRelinkUpstream, nil, row.Status, row.Status, err)
		return nil, err
	}
	if inst == nil {
		err := fmt.Errorf("%w: 上游未返回实例 %s 的信息", ErrStatusConflict, target)
		s.record(ctx, row, op, model.ActionRelinkUpstream, nil, row.Status, row.Status, err)
		return nil, err
	}
	// 迁移本地展示号：仅当原本两者一致时才有意义（否则 instance_id 是本地自建号，
	// 改它会让历史流水里的实例标识与新号对不上）。
	migrate := req.MigrateInstanceID && row.InstanceID == row.ProviderInstanceID
	if migrate {
		// instance_id 带唯一索引：目标号可能仍被**已销毁**的旧行占着（上游回收号段后
		// 把老号发给新机器是常见做法）。那种行不参与任何上游动作，把它的 instance_id
		// 让出来即可 —— 否则重新关联会直接撞唯一约束（实测 23505）。
		if _, ferr := s.repo.FreeInstanceIDForDestroyed(ctx, target, row.ID); ferr != nil {
			s.record(ctx, row, op, model.ActionRelinkUpstream, nil, row.Status, row.Status, ferr)
			return nil, ferr
		}
	}
	snap := &repository.UpstreamSnapshot{
		Status:    string(inst.Status),
		PublicIP:  inst.PublicIP,
		PrivateIP: inst.PrivateIP,
		Name:      inst.Name,
		OS:        inst.Specs.OS,
		Region:    inst.Region,
	}
	if len(inst.RawData) > 0 {
		if b, merr := json.Marshal(inst.RawData); merr == nil {
			snap.RawData = string(b)
		}
	}
	// 终态不复活：已销毁实例即便上游仍有旧号，也不允许借此"复活"。
	if row.LifecycleStage == lifecycleStageDestroyed {
		err := fmt.Errorf("%w: 实例已销毁，不能重新关联", ErrStatusConflict)
		s.record(ctx, row, op, model.ActionRelinkUpstream, nil, row.Status, row.Status, err)
		return nil, err
	}
	before := row.Status
	params := map[string]any{
		"old_upstream_id":     row.ProviderInstanceID,
		"new_upstream_id":     target,
		"migrate_instance_id": migrate,
		"reason":              req.Reason,
	}
	if uerr := s.repo.RelinkUpstream(ctx, row.ID, target, migrate, snap); uerr != nil {
		s.record(ctx, row, op, model.ActionRelinkUpstream, params, before, before, uerr)
		return nil, uerr
	}
	after := snap.Status
	if after == "" {
		after = row.Status
	}
	s.record(ctx, row, op, model.ActionRelinkUpstream, params, before, after, nil)
	result := &dto.RelinkResult{
		OldProviderInstanceID: row.ProviderInstanceID,
		NewProviderInstanceID: target,
		InstanceID:            row.InstanceID,
		Status:                after,
		PublicIP:              inst.PublicIP,
		PrivateIP:             inst.PrivateIP,
		Name:                  firstNonEmptyStr(inst.Name, row.Name),
		OS:                    firstNonEmptyStr(inst.Specs.OS, row.OS),
		Region:                firstNonEmptyStr(inst.Region, row.Region),
		InstanceIDMigrated:    migrate,
	}
	if migrate {
		result.InstanceID = target
	}
	return result, nil
}

// firstNonEmptyStr 取第一个非空字符串。
func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// RebindProduct 更换实例绑定的本地售出商品 / 上游镜像（上游换产品场景）。
//
// 实测需求：上游把我们卖的五区商品换成了六区，本地商品还是五区那条 ——
// 此后续费按五区价算（与上游账单对不上），配置也还按五区推。
//
// 校验：新绑定的商品 / 镜像必须**真实存在**（否则续费算价与配置展示会一起失空）；
// 上游镜像还必须属于同一渠道（跨渠道镜像的价格与字段口径都不同，绑过去必错）。
func (s *instanceService) RebindProduct(ctx context.Context, op Operator, id uint64, req *dto.RebindProductRequest) (*dto.RebindProductResult, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	if req == nil {
		return nil, ErrInvalidAction
	}
	if req.SellProductID == 0 && req.UpstreamProductID == nil && req.RenewalPrice == nil {
		return nil, fmt.Errorf("%w: 请至少指定要更换的本地商品或上游镜像", ErrInvalidAction)
	}
	// 自营实例没有"对接商品"这一层：既没有上游镜像可换，也没有"上游把商品换了"
	// 这个场景。这里只拒绝**换上游镜像**，换本地商品与改绑定价仍然允许 ——
	// 自营商品同样会下架/换代，续费口径要能跟着调整。
	if req.UpstreamProductID != nil && !resaleInstance(row) {
		return nil, fmt.Errorf("%w: 该实例为自营链路，未对接上游镜像，「更换上游镜像」不适用；如需更换本地售出商品请只勾选该项", ErrInvalidAction)
	}
	if s.detail == nil {
		return nil, ErrCapabilityUnsupported
	}

	out := &dto.RebindProductResult{
		OldSellProductID:     row.SellProductID,
		OldUpstreamProductID: row.UpstreamProductID,
	}

	// 新本地商品：必须存在（续费算价、配置展示都以它为准）。
	if req.SellProductID != 0 && req.SellProductID != row.SellProductID {
		product, perr := s.detail.Product(ctx, req.SellProductID)
		if perr != nil || product == nil || product.Deleted {
			return nil, fmt.Errorf("%w: 目标本地商品 #%d 不存在或已删除", ErrInvalidAction, req.SellProductID)
		}
		out.SellProductName = product.Name
	}
	// 新上游镜像：必须存在且属于同一渠道。
	if req.UpstreamProductID != nil && *req.UpstreamProductID != row.UpstreamProductID {
		target := *req.UpstreamProductID
		if target != 0 {
			up, _, uerr := s.detail.Upstream(ctx, target, row.BillingMode)
			if uerr != nil || up == nil {
				return nil, fmt.Errorf("%w: 目标上游镜像 #%d 不存在或已删除", ErrInvalidAction, target)
			}
			if up.ProviderID != 0 && row.ProviderID != 0 && up.ProviderID != row.ProviderID {
				return nil, fmt.Errorf("%w: 目标镜像属于渠道 #%d（%s），与本实例渠道 #%d 不一致；跨渠道换绑会导致价格与配置口径错位",
					ErrInvalidAction, up.ProviderID, firstNonEmptyStr(up.ProviderName, "未命名"), row.ProviderID)
			}
			out.UpstreamProductName = up.Name
		}
		out.UpstreamChanged = true
	}

	before := row.Status
	params := map[string]any{
		"old_sell_product_id":     row.SellProductID,
		"new_sell_product_id":     req.SellProductID,
		"old_upstream_product_id": row.UpstreamProductID,
		"reason":                  req.Reason,
	}
	if req.UpstreamProductID != nil {
		params["new_upstream_product_id"] = *req.UpstreamProductID
	}
	if rerr := s.repo.RebindProduct(ctx, row.ID, req.SellProductID, req.UpstreamProductID); rerr != nil {
		s.record(ctx, row, op, model.ActionRebindProduct, params, before, before, rerr)
		return nil, rerr
	}
	// 顺带调续费价（与账期调整同一语义：绑定价压过商品调价与折扣）。
	if req.RenewalPrice != nil {
		price := money.Round2(*req.RenewalPrice)
		if price == 0 {
			s.record(ctx, row, op, model.ActionRebindProduct, params, before, before, ErrBillingAdjustInvalid)
			return nil, fmt.Errorf("%w: 续费价必须大于 0；要取消绑定请传 -1", ErrBillingAdjustInvalid)
		}
		params["renewal_price"] = *req.RenewalPrice
		if perr := s.repo.SetRenewalPrice(ctx, row.ID, req.RenewalPrice); perr != nil {
			s.record(ctx, row, op, model.ActionRebindProduct, params, before, before, perr)
			return nil, perr
		}
	}
	s.record(ctx, row, op, model.ActionRebindProduct, params, before, before, nil)

	// 回显最终绑定（未指定的那项保持原值）。
	out.NewSellProductID = row.SellProductID
	out.NewUpstreamProductID = row.UpstreamProductID
	if req.SellProductID != 0 {
		out.NewSellProductID = req.SellProductID
	}
	if req.UpstreamProductID != nil {
		out.NewUpstreamProductID = *req.UpstreamProductID
	}
	if out.SellProductName == "" && out.NewSellProductID != 0 {
		if product, perr := s.detail.Product(ctx, out.NewSellProductID); perr == nil && product != nil {
			out.SellProductName = product.Name
		}
	}
	if out.UpstreamProductName == "" && out.NewUpstreamProductID != 0 {
		if up, _, uerr := s.detail.Upstream(ctx, out.NewUpstreamProductID, row.BillingMode); uerr == nil && up != nil {
			out.UpstreamProductName = up.Name
		}
	}
	return out, nil
}

// RebindCandidates 读更换对接商品的候选项。
//
// 目的：把「先到商品列表查到 ID，再回来手填」的两步操作压成一次选择。
// 候选范围与 RebindProduct 的校验一致 —— 本地商品给全量（附上下架标记），
// 上游镜像只给同渠道（跨渠道必被服务层拒绝，列出来只会让运营白跑一趟）。
func (s *instanceService) RebindCandidates(ctx context.Context, id uint64) (*dto.RebindProductCandidates, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	if s.detail == nil {
		return nil, ErrCapabilityUnsupported
	}
	out, cerr := s.detail.RebindCandidates(ctx, row.ProviderID, resaleInstance(row))
	if cerr != nil {
		return nil, cerr
	}
	// 标出当前绑定项：运营在长列表里一眼看到"现在用的是哪个"。
	for i := range out.SellProducts {
		out.SellProducts[i].Current = out.SellProducts[i].ID == row.SellProductID
	}
	for i := range out.UpstreamProducts {
		out.UpstreamProducts[i].Current = out.UpstreamProducts[i].ID == row.UpstreamProductID
	}
	return out, nil
}

// RevealCredential 查看实例登录明文密码。
//
// 明文不随详情下发（详情高频加载、会被日志与浏览器缓存），只在此处按需返回；
// 无论有无密码都写一条流水——"谁在什么时候看过密码"本身就是必须留痕的信息。
func (s *instanceService) RevealCredential(ctx context.Context, op Operator, id uint64) (*dto.CredentialRevealResult, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	if s.detail == nil {
		return nil, ErrCapabilityUnsupported
	}
	username, password, port, rerr := s.detail.RevealCredential(ctx, row.ID)
	if rerr != nil {
		s.record(ctx, row, op, model.ActionRevealCredential, nil, row.Status, row.Status, rerr)
		return nil, rerr
	}
	s.record(ctx, row, op, model.ActionRevealCredential,
		map[string]any{"has_password": password != ""}, row.Status, row.Status, nil)
	if password == "" {
		return nil, fmt.Errorf("%w: 上游未保存该实例的明文密码，请用「重置密码」重新设置", ErrCredentialUnavailable)
	}
	return &dto.CredentialRevealResult{Username: username, Password: password, Port: port}, nil
}

// parseFlexibleTime 解析运维手填的到期时间：接受 RFC3339 与常见的空格分隔写法。
func parseFlexibleTime(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time: %s", raw)
}

// Destroy 销毁实例（T5.5 按能力分派：优先 InstanceTermination，退化 InstanceAdministration.Delete）。
// req.ConfirmMark 必须等于实例标识或记录 ID，否则拒绝执行。
func (s *instanceService) Destroy(ctx context.Context, op Operator, id uint64, req *dto.DestroyRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	if req == nil {
		return ErrPermissionDenied
	}
	mark := strings.TrimSpace(req.ConfirmMark)
	if mark != row.InstanceID && mark != strconv.FormatUint(row.ID, 10) {
		return fmt.Errorf("%w: 请准确输入实例标识 %s", ErrPermissionDenied, row.InstanceID)
	}
	if row.Status == string(pkgmodel.InstanceStatusDeleted) {
		return fmt.Errorf("%w: 实例已销毁", ErrStatusConflict)
	}

	params := map[string]any{"reason": req.Reason}
	before := row.Status
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionDestroy, params, before, before, err)
		return err
	}
	reason := req.Reason
	if reason == "" {
		reason = "管理员销毁"
	}
	if err := upstream.TerminateWithFallback(ctx, provider, row.InstanceMark(), reason); err != nil {
		if errors.Is(err, upstream.ErrCapabilityMissing) {
			err = fmt.Errorf("%w: %w", ErrCapabilityUnsupported, err)
		}
		s.record(ctx, row, op, model.ActionDestroy, params, before, before, err)
		return err
	}
	after := string(pkgmodel.InstanceStatusDeleted)
	_ = s.repo.UpdateStatus(ctx, row.ID, after)
	// 同步落阶段为终态：只写 status 会让「阶段=active/grace」与「状态=已销毁」长期共存，
	// 生命周期推进器还会按旧阶段把它当在跑机器重复处置。
	_ = s.repo.UpdateLifecycleStage(ctx, row.ID, lifecycleStageDestroyed)
	s.record(ctx, row, op, model.ActionDestroy, params, before, after, nil)
	return nil
}

// Suspend 暂停实例（T5.5）：优先平台暂停态，退化关机；缺能力显式报错。
func (s *instanceService) Suspend(ctx context.Context, op Operator, id uint64, reason string) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	before := row.Status
	params := map[string]any{"reason": reason}
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionSuspend, params, before, before, err)
		return err
	}
	if reason == "" {
		reason = "管理员暂停"
	}
	if err := upstream.SuspendWithFallback(ctx, provider, row.InstanceMark(), reason); err != nil {
		err = mapCapabilityErr(err)
		s.record(ctx, row, op, model.ActionSuspend, params, before, before, err)
		return err
	}
	after := string(pkgmodel.InstanceStatusStopped)
	_ = s.repo.UpdateStatus(ctx, row.ID, after)
	// 落阶段（T5.4 语义）：手动暂停同样写 lifecycle_stage，避免推进器重复动作。
	_ = s.repo.UpdateLifecycleStage(ctx, row.ID, lifecycleStageSuspended)
	s.record(ctx, row, op, model.ActionSuspend, params, before, after, nil)
	return nil
}

// Unsuspend 恢复实例（T5.5）：优先平台解除暂停，退化开机；缺能力显式报错。
func (s *instanceService) Unsuspend(ctx context.Context, op Operator, id uint64) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	// 待开通实例（占位行）没有上游实例号：上游侧操作一律拒绝并给出可操作提示
	// （能力位在 Detail 里已整体关闭，这里是接口层的兜底）。
	if err := s.ensureProvisioned(row); err != nil {
		return err
	}
	before := row.Status
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionUnsuspend, nil, before, before, err)
		return err
	}
	if err := upstream.UnsuspendWithFallback(ctx, provider, row.InstanceMark()); err != nil {
		err = mapCapabilityErr(err)
		s.record(ctx, row, op, model.ActionUnsuspend, nil, before, before, err)
		return err
	}
	after := string(pkgmodel.InstanceStatusRunning)
	_ = s.repo.UpdateStatus(ctx, row.ID, after)
	_ = s.repo.UpdateLifecycleStage(ctx, row.ID, lifecycleStageActive)
	s.record(ctx, row, op, model.ActionUnsuspend, nil, before, after, nil)
	return nil
}

// maxBatchActionIDs 单次批量操作的实例上限：太大既会让上游限流，也会让前端回显失控。
const maxBatchActionIDs = 50

// BatchAction 批量运维（doc61 P1）：对多台实例下发同一动作，逐台独立执行并返回结果。
//
// 设计取舍：
//   - **逐台独立**，单台失败/跳过不影响其余（批量操作不能因为一台关机了就整体中止）；
//   - **无需执行的状态记为 skipped 而非 failed**：已在运行中再点开机、上游缺能力，
//     都不该被当成事故；只有真实上游/网络错误才算 failed；
//   - 复用单实例方法，因此操作流水与事件回调自动具备（避免两套写审计路径）。
func (s *instanceService) BatchAction(ctx context.Context, op Operator, req *dto.BatchActionRequest) (*dto.BatchActionResponse, error) {
	if req == nil || len(req.IDs) == 0 {
		return nil, ErrInvalidAction
	}
	if len(req.IDs) > maxBatchActionIDs {
		return nil, fmt.Errorf("%w: 单次最多 %d 台", ErrInvalidAction, maxBatchActionIDs)
	}
	if _, _, err := normalizeBatchAction(req.Action); err != nil {
		return nil, err
	}

	// 去重但保持请求顺序，避免同台被下发两次。
	seen := make(map[uint64]bool, len(req.IDs))
	ids := make([]uint64, 0, len(req.IDs))
	for _, id := range req.IDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	resp := &dto.BatchActionResponse{Items: make([]dto.BatchActionItem, 0, len(ids))}
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		default:
		}
		item := s.batchOne(ctx, op, id, req)
		resp.Total++
		switch item.Status {
		case model.ResultSuccess:
			resp.Succeeded++
		case model.ResultSkipped:
			resp.Skipped++
		default:
			resp.Failed++
		}
		resp.Items = append(resp.Items, item)
	}
	return resp, nil
}

// batchOne 执行单台并归类结果。错误分类是这里唯一的判断逻辑：
// 无需执行/缺能力 → skipped；其余（上游报错等）→ failed。
func (s *instanceService) batchOne(ctx context.Context, op Operator, id uint64, req *dto.BatchActionRequest) dto.BatchActionItem {
	item := dto.BatchActionItem{ID: id, Status: model.ResultSuccess}

	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		item.Status = model.ResultFailed
		item.Message = "实例不存在"
		return item
	}
	item.InstanceMark = row.InstanceID
	item.Name = row.Name
	item.BeforeStatus = row.Status

	normalized, _, _ := normalizeBatchAction(req.Action)
	var opErr error
	switch normalized {
	case "on", "off", "hard_off", "reboot", "hard_reboot":
		opErr = s.Power(ctx, op, id, normalized)
	case "sync":
		_, opErr = s.Sync(ctx, op, id)
	case "suspend":
		opErr = s.Suspend(ctx, op, id, req.Reason)
	case "unsuspend":
		opErr = s.Unsuspend(ctx, op, id)
	}

	if after, ferr := s.repo.FindByID(ctx, id); ferr == nil {
		item.AfterStatus = after.Status
	} else {
		item.AfterStatus = row.Status
	}
	if opErr == nil {
		return item
	}
	item.Message = opErr.Error()
	// 状态守卫（已在运行中/已关机/创建中）与上游缺能力属"无需或不可能执行"，归 skipped。
	if errors.Is(opErr, ErrStatusConflict) || errors.Is(opErr, ErrCapabilityUnsupported) {
		item.Status = model.ResultSkipped
		item.AfterStatus = item.BeforeStatus
		return item
	}
	item.Status = model.ResultFailed
	return item
}

// normalizeBatchAction 校验批量动作名并返回归一值（与单实例接口同一套取值）。
func normalizeBatchAction(action string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "on", "start":
		return "on", model.ActionPowerOn, nil
	case "off", "stop":
		return "off", model.ActionPowerOff, nil
	case "hard_off", "hard-off":
		return "hard_off", model.ActionHardOff, nil
	case "reboot", "restart":
		return "reboot", model.ActionReboot, nil
	case "hard_reboot", "hard-reboot":
		return "hard_reboot", model.ActionHardReboot, nil
	case "sync":
		return "sync", model.ActionSync, nil
	case "suspend":
		return "suspend", model.ActionSuspend, nil
	case "unsuspend":
		return "unsuspend", model.ActionUnsuspend, nil
	default:
		return "", "", fmt.Errorf("%w: 不支持的批量动作 %s", ErrInvalidAction, action)
	}
}

// BatchDestroy 批量销毁实例（逐台独立执行并返回结果）。
//
// 与 BatchAction 的差别在于**确认粒度**：批量电源动作可以一次确认全部，
// 销毁是真删上游机器、不可逆，因此每台都必须带上自己的确认文本
// （ConfirmMarks 里缺一个就少销毁一台，不静默放过也不整体拒绝）。
//
// 其余取舍与 BatchAction 一致：逐台独立（一台失败不中止其余）、
// 无需执行/缺能力记为 skipped、复用单实例 Destroy 因而流水与事件自动具备。
func (s *instanceService) BatchDestroy(ctx context.Context, op Operator, req *dto.BatchDestroyRequest) (*dto.BatchDestroyResponse, error) {
	if req == nil || len(req.IDs) == 0 {
		return nil, fmt.Errorf("%w: 请先选择要销毁的实例", ErrBatchDestroyInvalid)
	}
	if len(req.IDs) > maxBatchActionIDs {
		return nil, fmt.Errorf("%w: 单次最多 %d 台", ErrBatchDestroyInvalid, maxBatchActionIDs)
	}

	// 去重但保持请求顺序，避免同台被下发两次。
	seen := make(map[uint64]bool, len(req.IDs))
	ids := make([]uint64, 0, len(req.IDs))
	for _, id := range req.IDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	resp := &dto.BatchDestroyResponse{Items: make([]dto.BatchDestroyItem, 0, len(ids))}
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		default:
		}
		item := s.batchDestroyOne(ctx, op, id, req)
		resp.Total++
		switch item.Status {
		case model.ResultSuccess:
			resp.Succeeded++
		case model.ResultSkipped:
			resp.Skipped++
		default:
			resp.Failed++
		}
		resp.Items = append(resp.Items, item)
	}
	return resp, nil
}

// batchDestroyOne 销毁单台并归类结果。
func (s *instanceService) batchDestroyOne(ctx context.Context, op Operator, id uint64, req *dto.BatchDestroyRequest) dto.BatchDestroyItem {
	item := dto.BatchDestroyItem{ID: id}

	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		item.Status = model.ResultFailed
		item.Message = "实例不存在"
		return item
	}
	item.InstanceMark = row.InstanceID
	item.Name = row.Name
	item.BeforeStatus = row.Status

	// 逐台确认：确认文本按记录 ID 查（前端逐行回填该行的确认值）。
	mark := strings.TrimSpace(req.ConfirmMarks[strconv.FormatUint(id, 10)])
	if mark == "" {
		// 未逐台确认的实例不执行——批量销毁的护栏，与单实例接口的 confirm_mark 同语义。
		item.Status = model.ResultSkipped
		item.AfterStatus = row.Status
		item.Message = "未逐台确认，已跳过（批量销毁需为每台回填实例标识）"
		return item
	}

	opErr := s.Destroy(ctx, op, id, &dto.DestroyRequest{ConfirmMark: mark, Reason: req.Reason})
	if after, ferr := s.repo.FindByID(ctx, id); ferr == nil {
		item.AfterStatus = after.Status
	} else {
		item.AfterStatus = row.Status
	}
	if opErr == nil {
		item.Status = model.ResultSuccess
		return item
	}
	item.Message = opErr.Error()
	// 状态守卫（已销毁）与缺能力属"无需/不可能执行"，归 skipped。
	if errors.Is(opErr, ErrStatusConflict) || errors.Is(opErr, ErrCapabilityUnsupported) {
		item.Status = model.ResultSkipped
		item.AfterStatus = item.BeforeStatus
		return item
	}
	item.Status = model.ResultFailed
	return item
}

// 生命周期模块只依赖本结构的基础类型（service 层桥接），避免模块间循环依赖。
type StageActionInput struct {
	InstanceID   uint64
	InstanceMark string
	UserID       uint64
	Action       string
	FromStage    string
	ToStage      string
	// Result 显式结果：success / failed / skipped；为空按 Err 推导。
	Result string
	// Message 补充说明（能力缺失/预演/仅标记等），落 params.message。
	Message string
	Err     error
}

// RecordStageAction 以系统操作人身份写入一条实例运维流水（T5.4）。
// 尽力而为：写入失败返回错误由调用方记日志，不回滚上游动作。
func (s *instanceService) RecordStageAction(ctx context.Context, in StageActionInput) error {
	action := in.Action
	if action == "" {
		action = model.ActionStage
	}
	if action == model.ActionStage && in.FromStage == "" && in.ToStage == "" {
		// 无阶段变化且无动作：无需落库。
		return nil
	}
	result := in.Result
	if result == "" {
		result = model.ResultSuccess
	}
	entry := &model.Operation{
		InstanceID:   in.InstanceID,
		InstanceMark: in.InstanceMark,
		UserID:       in.UserID,
		OperatorType: model.OperatorTypeSystem,
		Action:       action,
		BeforeStatus: in.FromStage,
		AfterStatus:  in.ToStage,
		Result:       result,
	}
	if in.Message != "" {
		if raw, err := json.Marshal(map[string]any{"message": in.Message}); err == nil {
			entry.Params = string(raw)
		}
	}
	if in.Err != nil {
		entry.Result = model.ResultFailed
		entry.ErrorMessage = truncate(in.Err.Error(), 500)
	}
	if err := s.opRepo.Create(ctx, entry); err != nil {
		if s.logger != nil {
			s.logger.Warn("写入生命周期阶段流水失败",
				zap.Uint64("instance_id", in.InstanceID),
				zap.String("action", action),
				zap.Error(err))
		}
		return err
	}
	s.notifyEvent(ctx, InstanceEvent{
		InstanceRowID: in.InstanceID,
		InstanceMark:  in.InstanceMark,
		UserID:        in.UserID,
		Action:        action,
		FromStatus:    in.FromStage,
		ToStatus:      in.ToStage,
		Err:           in.Err,
	})
	return nil
}

// mapCapabilityErr 把上游能力缺失错误映射为实例域错误（handler 统一提示 20004）。
func mapCapabilityErr(err error) error {
	if errors.Is(err, upstream.ErrCapabilityMissing) {
		return fmt.Errorf("%w: %w", ErrCapabilityUnsupported, err)
	}
	return err
}

// Operations 实例操作流水分页。
func (s *instanceService) Operations(ctx context.Context, id uint64, query *dto.OperationListQuery) (*dto.OperationListResponse, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, s.wrapNotFound(err)
	}
	page, pageSize := 1, 20
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}
	rows, total, err := s.opRepo.List(ctx, id, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]dto.OperationItem, 0, len(rows))
	for _, it := range rows {
		items = append(items, dto.OperationItem{
			ID:           it.ID,
			InstanceID:   it.InstanceID,
			InstanceMark: it.InstanceMark,
			UserID:       it.UserID,
			OperatorType: it.OperatorType,
			OperatorID:   it.OperatorID,
			OperatorName: it.OperatorName,
			Action:       it.Action,
			Params:       it.Params,
			BeforeStatus: it.BeforeStatus,
			AfterStatus:  it.AfterStatus,
			Result:       it.Result,
			ErrorMessage: it.ErrorMessage,
			CreatedAt:    it.CreatedAt.Format(time.RFC3339),
		})
	}
	return &dto.OperationListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

// OperationLogs 全局操作流水分页（跨实例审计视图）。
func (s *instanceService) OperationLogs(ctx context.Context, query *dto.OperationLogQuery) (*dto.OperationLogListResponse, error) {
	if query == nil {
		query = &dto.OperationLogQuery{}
	}
	rows, total, err := s.opRepo.ListGlobal(ctx, query)
	if err != nil {
		return nil, err
	}
	page, pageSize := query.Page, query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	items := make([]dto.OperationLogItem, 0, len(rows))
	for i := range rows {
		it := &rows[i]
		items = append(items, dto.OperationLogItem{
			OperationItem: dto.OperationItem{
				ID:           it.ID,
				InstanceID:   it.InstanceID,
				InstanceMark: it.InstanceMark,
				UserID:       it.UserID,
				OperatorType: it.OperatorType,
				OperatorID:   it.OperatorID,
				OperatorName: it.OperatorName,
				Action:       it.Action,
				Params:       it.Params,
				BeforeStatus: it.BeforeStatus,
				AfterStatus:  it.AfterStatus,
				Result:       it.Result,
				ErrorMessage: it.ErrorMessage,
				CreatedAt:    it.CreatedAt.Format(time.RFC3339),
			},
			Username: it.Username,
		})
	}
	return &dto.OperationLogListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}
func (s *instanceService) Related(ctx context.Context, id uint64) (*dto.RelatedInfo, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.wrapNotFound(err)
	}
	renewals, err := s.related.Renewals(ctx, id)
	if err != nil {
		return nil, err
	}
	tickets, err := s.related.Tickets(ctx, id)
	if err != nil {
		return nil, err
	}
	// 来源订单（instances.order_id）+ 续费订单（instance_renewals.order_id）合并去重。
	orderIDs := make([]uint64, 0, len(renewals)+1)
	seen := map[uint64]bool{}
	if row.OrderID > 0 {
		orderIDs = append(orderIDs, row.OrderID)
		seen[row.OrderID] = true
	}
	renewalOrderIDs, err := s.related.RenewalOrderIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, oid := range renewalOrderIDs {
		if oid > 0 && !seen[oid] {
			orderIDs = append(orderIDs, oid)
			seen[oid] = true
		}
	}
	orders, err := s.related.Orders(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	return &dto.RelatedInfo{Orders: orders, Renewals: renewals, Tickets: tickets}, nil
}

// ============ 内部辅助 ============

// wrapNotFound 归一化仓储的未找到错误。
func (s *instanceService) wrapNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrInstanceNotFound
	}
	return err
}

// toItem 实例行 → 列表项（含剩余天数与到期状态）。
func (s *instanceService) toItem(row *repository.InstanceRow, withinDays int) dto.InstanceItem {
	state, daysLeft := expireStateOf(row.ExpireAt, withinDays)
	return dto.InstanceItem{
		ID:                 row.ID,
		InstanceID:         row.InstanceID,
		ProviderID:         row.ProviderID,
		ProviderName:       row.ProviderName,
		ProviderType:       row.ProviderType,
		UserID:             row.UserID,
		Username:           row.Username,
		UserEmail:          row.UserEmail,
		UserPhone:          row.UserPhone,
		OrderID:            row.OrderID,
		SourceMode:         row.SourceMode,
		SellProductID:      row.SellProductID,
		UpstreamProductID:  row.UpstreamProductID,
		ProviderInstanceID: row.ProviderInstanceID,
		Pending:            row.ProviderInstanceID == "",
		Resale:             resaleInstance(row),
		LifecycleStage:     row.LifecycleStage,
		// 产品类型归一后再下发：存量空串（迁移 108 回填前）与未知值都走 other，
		// 与筛选口径一致 —— 前端拿到的每个值都能在导航里找到对应标签。
		InstanceType: pkgmodel.NormalizeProductKind(row.InstanceType),
		Name:         row.Name,
		CPU:          row.CPU,
		Memory:       row.Memory,
		Disk:         row.Disk,
		DiskType:     row.DiskType,
		Bandwidth:    row.Bandwidth,
		OS:           row.OS,
		Region:       row.Region,
		Zone:         row.Zone,
		Status:       row.Status,
		PowerStatus:  powerOf(row.Status),
		// 两个正交维度（本轮修复的根因）：电源态答"机器通没通电"，
		// 服务态答"这个服务还在不在有效期内/有没有被我方暂停"。
		// 此前前端只凭 status 分不清关机与暂停，把两者压成一句「已暂停」。
		PowerState:   powerStateOf(row.Status, row.TransitionState),
		ServiceState: serviceStateOf(row.ExpireAt, row.LifecycleStage, withinDays),
		// 过渡态：非 nil 表示有未确认的操作在进行（界面显示"重启中"+禁用冲突动作）。
		Transition:       transitionInfoOf(row.TransitionState, row.TransitionStartedAt, row.TransitionUntil),
		PrivateIP:        row.PrivateIP,
		PublicIP:         row.PublicIP,
		BillingMode:      row.BillingMode,
		ActorUserID:      row.ActorUserID,
		ActorName:        row.ActorName,
		Remark:           row.Remark,
		CustomerRemark:   row.CustomerRemark,
		ExpireAt:         fmtTimePtr(row.ExpireAt),
		DaysLeft:         daysLeft,
		ExpireState:      state,
		LastSyncedAt:     fmtTimePtr(row.LastSyncedAt),
		EnforceAttempts:  row.EnforceAttempts,
		EnforceNextAt:    fmtTimePtr(row.EnforceNextAt),
		LastEnforceError: row.LastEnforceError,
		RenewalPrice:     row.RenewalPrice,
		CreatedAt:        row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        row.UpdatedAt.Format(time.RFC3339),
	}
}

// record 写入操作流水。尽力而为：写入失败只记 warn，不影响主操作结果。
func (s *instanceService) record(ctx context.Context, row *repository.InstanceRow, op Operator, action string, params map[string]any, before, after string, opErr error) {
	if row == nil || action == "" {
		return
	}
	opType := op.Type
	if opType == "" {
		opType = model.OperatorTypeAdmin
	}
	entry := &model.Operation{
		InstanceID:   row.ID,
		InstanceMark: row.InstanceID,
		UserID:       row.UserID,
		OperatorType: opType,
		OperatorID:   op.ID,
		OperatorName: op.Name,
		Action:       action,
		BeforeStatus: before,
		AfterStatus:  after,
		Result:       model.ResultSuccess,
	}
	if params != nil {
		if raw, err := json.Marshal(params); err == nil {
			entry.Params = string(raw)
		}
	}
	if opErr != nil {
		entry.Result = model.ResultFailed
		entry.ErrorMessage = truncate(opErr.Error(), 500)
	}
	if err := s.opRepo.Create(ctx, entry); err != nil && s.logger != nil {
		s.logger.Warn("写入实例操作流水失败",
			zap.Uint64("instance_id", row.ID),
			zap.String("action", action),
			zap.Error(err))
	}
	s.notifyEvent(ctx, InstanceEvent{
		InstanceRowID: row.ID,
		InstanceMark:  row.InstanceID,
		UserID:        row.UserID,
		Action:        action,
		FromStatus:    before,
		ToStatus:      after,
		Err:           opErr,
	})
}

// capabilities 断言上游适配器能力，供前端置灰按钮。
//
// 两条信息来源互补：**类型断言**回答"这个实现有没有该能力"（对全部平台通用），
// **描述符**回答"这个渠道声明支持什么"（运营可读、可提前展示）。二者取或，
// 避免适配器实现了能力却因描述符漏登记而在界面上被置灰。
func (s *instanceService) capabilities(ctx context.Context, providerID uint64) (dto.Capabilities, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return dto.Capabilities{}, err
	}
	_, ctrl := provider.(upstream.InstanceControl)
	_, admin := provider.(upstream.InstanceAdministration)
	_, suspend := provider.(upstream.InstanceSuspension)
	_, terminate := provider.(upstream.InstanceTermination)
	_, reinstall := provider.(upstream.InstanceReinstall)
	_, pwdReset := provider.(upstream.InstancePasswordReset)
	_, rescue := provider.(upstream.InstanceRescue)
	_, snapshot := provider.(upstream.InstanceSnapshot)
	_, hardware := provider.(upstream.InstanceHardware)
	_, natReader := provider.(upstream.NatReader)
	desc := upstream.CapabilitiesOf(provider)
	supports := func(op string) bool { return upstream.SupportsOperation(desc, op) }
	// 暂停/销毁按能力分派（T5.5）：平台无对应接口时退化为电源/删除等价操作。
	return dto.Capabilities{
		Power:         ctrl || supports(upstream.OpStart) || supports(upstream.OpStop),
		Console:       ctrl || supports(upstream.OpVNC),
		Resize:        admin || supports(upstream.OpResize),
		Destroy:       admin || terminate || supports(upstream.OpDestroy),
		Suspend:       suspend || ctrl || supports(upstream.OpSuspend),
		Reinstall:     reinstall || supports(upstream.OpReinstall),
		ResetPassword: pwdReset || supports(upstream.OpResetPassword),
		Rescue:        rescue || supports(upstream.OpRescue),
		Snapshot:      snapshot || supports(upstream.OpSnapshot),
		Bandwidth:     hardware || supports(upstream.OpBandwidth),
		AddIP:         hardware || supports(upstream.OpAddIP),
		AttachDisk:    hardware || supports(upstream.OpAttachDisk),
		NatAccess:     natReader || supports(upstream.OpNatAccess),
	}, nil
}

// passwordResetter 解析上游并断言重置密码能力。
func (s *instanceService) passwordResetter(ctx context.Context, providerID uint64) (upstream.InstancePasswordReset, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	r, ok := provider.(upstream.InstancePasswordReset)
	if !ok {
		return nil, ErrCapabilityUnsupported
	}
	return r, nil
}

// rescuer 解析上游并断言救援系统能力。
func (s *instanceService) rescuer(ctx context.Context, providerID uint64) (upstream.InstanceRescue, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	r, ok := provider.(upstream.InstanceRescue)
	if !ok {
		return nil, ErrCapabilityUnsupported
	}
	return r, nil
}

// snapshotter 解析上游并断言快照能力。
func (s *instanceService) snapshotter(ctx context.Context, providerID uint64) (upstream.InstanceSnapshot, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	r, ok := provider.(upstream.InstanceSnapshot)
	if !ok {
		return nil, ErrCapabilityUnsupported
	}
	return r, nil
}

// hardware 解析上游并断言硬件变更能力。
func (s *instanceService) hardware(ctx context.Context, providerID uint64) (upstream.InstanceHardware, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	r, ok := provider.(upstream.InstanceHardware)
	if !ok {
		return nil, ErrCapabilityUnsupported
	}
	return r, nil
}

// buildProvider 解析并实例化上游适配器。
func (s *instanceService) buildProvider(ctx context.Context, providerID uint64) (upstream.Provider, error) {
	if s.resolve == nil {
		return nil, ErrProviderUnavailable
	}
	provider, err := s.resolve(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
	}
	if provider == nil {
		return nil, ErrProviderUnavailable
	}
	return provider, nil
}

// control 解析上游并断言电源/控制台能力。
func (s *instanceService) control(ctx context.Context, providerID uint64) (upstream.InstanceControl, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	ctrl, ok := provider.(upstream.InstanceControl)
	if !ok {
		return nil, ErrCapabilityUnsupported
	}
	return ctrl, nil
}

// administration 解析上游并断言管理与维护能力（删除/变配）。
func (s *instanceService) administration(ctx context.Context, providerID uint64) (upstream.InstanceAdministration, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	admin, ok := provider.(upstream.InstanceAdministration)
	if !ok {
		return nil, ErrCapabilityUnsupported
	}
	return admin, nil
}

// refreshUpstream 回源刷新单实例并回写本地；失败返回错误（调用方决定是否降级）。
func (s *instanceService) refreshUpstream(ctx context.Context, row *repository.InstanceRow) error {
	return s.refreshUpstreamExpecting(ctx, row, "")
}

// refreshUpstreamExpecting 回源刷新，并拒绝把状态"倒退"回 staleStatus。
//
// staleStatus 是刚被替换掉的旧状态（一次成功操作的前置状态）。上游的状态机
// 通常有几秒延迟：关机指令返回成功时，GET 到的可能仍是 running。若照抄，
// 本地刚写的 stopped 会被旧值覆盖，前端列表闪回"运行中"，紧接的开机请求
// 又因"已在运行中"被拒 —— 表现就是开关机偶发失败。传空串表示不做该保护。
func (s *instanceService) refreshUpstreamExpecting(ctx context.Context, row *repository.InstanceRow, staleStatus string) error {
	return s.refreshUpstreamFromTransition(ctx, row, staleStatus, "", time.Time{})
}

// refreshUpstreamFromTransition 回源刷新 + 过渡态收尾。
//
// transition 非空表示"本地刚下发过这个动作"；claimedAt 是那次占位的开始时间
// （判断最短占用窗口用）。回源拿到的新状态若与 staleStatus 不同，说明上游已经
// 执行完并转过来了 —— 此时顺手清掉过渡态，界面从「重启中」回到「运行中」。
// 仍等于旧状态则说明还在执行，过渡态保留。
//
// 只在"上游确实报出新状态"时清：拿不到数据（接口失败/降级）时保留过渡态，
// 让界面继续显示"进行中"而不是假装完成了。
func (s *instanceService) refreshUpstreamFromTransition(ctx context.Context, row *repository.InstanceRow, staleStatus, transition string, claimedAt time.Time) error {
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		return err
	}
	ctrl, ok := provider.(upstream.InstanceControl)
	if !ok {
		return ErrCapabilityUnsupported
	}
	inst, err := ctrl.GetInstance(ctx, row.InstanceID)
	if err != nil {
		return fmt.Errorf("回源刷新失败: %w", err)
	}
	raw := ""
	if len(inst.RawData) > 0 {
		if b, merr := json.Marshal(inst.RawData); merr == nil {
			raw = string(b)
		}
	}
	status := string(inst.Status)
	// 终态不复活：本地已判销毁的实例，上游软删未清时仍会返回 running/active，
	// 照抄会把列表里已销毁的机器显示成"运行中"。终态由本系统认定，回源只刷新其余字段。
	switch {
	case row.LifecycleStage == lifecycleStageDestroyed:
		status = row.Status
	case !upstream.SupportsPowerStateReporting(provider):
		// **上游不报电源状态**（实测：魔方财务只有 domainstatus=Active 这类服务状态，
		// 没有机器电源字段）。此时回源值不能用来改写本地 status —— 它答的不是
		// "机器开着吗"，照抄会把「已关机」冲成「运行中」，运营点完关机一同步
		// 就变回来（用户反馈"状态混乱"的另一根因）。
		//
		// 保留本地现值（那是我们下发电源指令后唯一可靠的知识），只刷新 IP / 规格 / 原始数据。
		// 代价：客户在上游面板自己关机时本地看不出来 —— 这在财务型链路上无法避免
		//（上游没给这个信息），需要时由运营用「同步上游」看原始数据判断。
		status = ""
	case staleStatus != "" && status == staleStatus:
		// 上游还没转到新状态：保留本地刚写入的前瞻值，仅刷新 IP / 原始数据。
		status = ""
	}
	if err := s.repo.ApplySync(ctx, row.ID, status, inst.PublicIP, inst.PrivateIP, raw, time.Now()); err != nil {
		return err
	}
	// 过渡态收尾：
	//   · 报电源状态的平台：只有上游确实给出新状态（status 非空）才算执行完；
	//     仍是旧值说明还在跑，保留过渡态让界面继续显示「重启中」。
	//   · 不报电源状态的平台：status 被我们置空（无从核对），但指令已受理 ——
	//     可以收尾，界面回到本地已知状态，而不是永远停在「重启中」。
	//
	// 但两者都受**最短占用窗口**约束（MinHoldSec）：重启是 运行中→运行中 的转场，
	// 上游"确认"了也看不出变化；若一确认就放行，用户几秒后再点一次同样会堆指令
	//（用户实测反馈"多次重启累积"）。窗口未满时保留过渡态，让拦截持续到动作
	// 确实已经跑起来为止。
	if transition != "" && (status != "" || !upstream.SupportsPowerStateReporting(provider)) {
		if !transitionWithinMinHold(transition, claimedAt) {
			_ = s.repo.ClearTransition(ctx, row.ID)
		}
	}
	return nil
}

// transitionWithinMinHold 判断过渡态是否仍在自己动作的最短占用窗口内。
//
// true = 还在窗口内，调用方应保留过渡态（不放行重复指令）。
// startedAt 为零值（未记录起点）时返回 false —— 没有起点就无从计算，
// 按"可以收尾"处理，避免把历史行永久锁住。
func transitionWithinMinHold(state string, startedAt time.Time) bool {
	if startedAt.IsZero() {
		return false
	}
	spec := pkgmodel.InstanceTransitionOf(state)
	if spec.MinHoldSec <= 0 {
		return false
	}
	return time.Since(startedAt) < time.Duration(spec.MinHoldSec)*time.Second
}

// derefTime 解引用可选时间（nil → 零值），供"没有起点就不计入窗口"的判断使用。
func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// transitionOfPowerAction 电源动作 → 过渡态值（迁移 112）。
//
// 硬关与普通关机归到同一个过渡态：界面文案都是"关机中"，
// 区分硬/软对"正在执行"这件事没有意义（区别只在指令语义）。
func transitionOfPowerAction(normalized string) string {
	switch normalized {
	case "on":
		return pkgmodel.InstanceTransitionStarting
	case "off", "hard_off":
		return pkgmodel.InstanceTransitionStopping
	case "reboot", "hard_reboot":
		return pkgmodel.InstanceTransitionRebooting
	default:
		return ""
	}
}

// claimTransition **原子抢占**过渡态（迁移 112）。
//
// 返回 claimed=true 表示抢到了这个动作的执行权，调用方应下发上游指令；
// claimed=false 表示已有生效中的动作，必须拒绝本次请求。
// claimedAt/until 是本次占位的起止时间，调用方据此判断"最短占用窗口"
// 是否已过（不能读 row —— 那是抢占前的快照，时间列还是旧值）。
//
// 为什么必须抢占：上游下发是异步的，界面 loading 只覆盖接口往返那几秒，
// 真正的执行窗口有几分钟。用户连点两下时，若采用"先查后写"，两个并发请求
// 会双双通过检查、双双下发 —— 上游堆两条同样指令（实测复现：连点两次开机
// 两条都成功）。把判定与写入压进同一条 UPDATE 的 WHERE，由数据库行锁保证
// 只有一个请求赢，这是跨会话、跨页面唯一可靠的闸门。
func (s *instanceService) claimTransition(ctx context.Context, id uint64, state string) (bool, time.Time, time.Time, error) {
	if state == "" {
		return true, time.Time{}, time.Time{}, nil
	}
	spec := pkgmodel.InstanceTransitionOf(state)
	now := time.Now()
	until := now.Add(time.Duration(spec.TerminalTimeoutSec) * time.Second)
	claimed, err := s.repo.ClaimTransition(ctx, id, state, now, until)
	return claimed, now, until, err
}

// releaseTransition 下发失败时回滚自己抢到的过渡态（仅当仍属于自己）。
func (s *instanceService) releaseTransition(ctx context.Context, id uint64, state string) {
	if state == "" {
		return
	}
	_ = s.repo.ClearTransitionIfOwner(ctx, id, state)
}

// ensureNoActiveTransition 过渡态闸门：有未确认的动作时拒绝新指令。
//
// 两个例外必须放行，否则一次异常会把实例永久锁死：
//   - 已超时（超过该动作的典型耗时上限）：视为结果未知，放行但由界面提示；
//   - 生命周期推进器的处置动作（暂停/销毁）：那是系统按账期自动做的，
//     不能因为一个没回执的电源动作就一直延后。
//
// 但"手动重复点同一个按钮"必须拦住 —— 那正是用户反馈的问题（多次重启叠加）。
//
// 注意这只是**快速失败的友好提示**（能带上"从几点开始"的上下文），
// 真正的互斥由 claimTransition 的原子抢占保证 —— 两者之间的窗口正是
// 连点两次能得手的原因，单靠本函数不够。
func (s *instanceService) ensureNoActiveTransition(row *repository.InstanceRow, logAction string) error {
	if row == nil || row.TransitionState == "" {
		return nil
	}
	// 超时判定：窗口过后不再拦截（结果未知，交给界面提示与回源纠正）。
	if row.TransitionUntil != nil && time.Now().After(*row.TransitionUntil) {
		return nil
	}
	return transitionBusyError(row.TransitionState, row.TransitionStartedAt)
}

// transitionConflict 抢占失败时的拒绝错误：为提示语带上"谁在占用"的上下文。
//
// 读一次最新行拿占用者与开始时间（读失败则用调用方给的 state 兜底）——
// 这样用户看到的仍是「实例正在关机中（14:07:33 开始）」，而不是一句泛泛的"忙"。
func (s *instanceService) transitionConflict(ctx context.Context, row *repository.InstanceRow) error {
	if row == nil {
		return fmt.Errorf("%w：实例上有正在执行的操作，请稍后重试", ErrStatusConflict)
	}
	if fresh, err := s.repo.FindByID(ctx, row.ID); err == nil && fresh != nil && fresh.TransitionState != "" {
		return transitionBusyError(fresh.TransitionState, fresh.TransitionStartedAt)
	}
	if row.TransitionState != "" {
		return transitionBusyError(row.TransitionState, row.TransitionStartedAt)
	}
	return fmt.Errorf("%w：实例上有正在执行的操作，请稍后重试", ErrStatusConflict)
}

// transitionBusyError 过渡态占用时的拒绝提示（与 ensureNoActiveTransition 同一措辞）。
func transitionBusyError(state string, startedAt *time.Time) error {
	spec := pkgmodel.InstanceTransitionOf(state)
	since := ""
	if startedAt != nil {
		since = fmt.Sprintf("（%s 开始）", startedAt.Format("15:04:05"))
	}
	return fmt.Errorf("%w：实例正在%s%s，请等待完成后再操作；若长时间无变化可点「同步上游」确认",
		ErrStatusConflict, spec.Label, since)
}

// powerStateOf 计算**机器电源维度**（与账期无关）。
//
// 这是本轮修复的核心：此前前端只拿 status 判状态，而上游把 suspended（到期暂停）
// 也映射成本地 stopped，手动关机同样是 stopped —— 两者无法区分，于是关机被显示成
// 「已暂停」。电源维度只回答"机器通没通电"，是否到期/是否被停服务见 serviceStateOf。
//
//   - 有进行中的电源类动作 → transitioning（界面显示"重启中"等）
//   - running → on；stopped → off
//   - 创建中/删除中/已销毁 → unknown（机器还没起来或已不存在，谈"开机"没意义）
func powerStateOf(status, transition string) string {
	if transition != "" {
		if spec := pkgmodel.InstanceTransitionOf(transition); spec.PowerHint != "" {
			return "transitioning"
		}
	}
	switch status {
	case string(pkgmodel.InstanceStatusRunning):
		return "on"
	case string(pkgmodel.InstanceStatusStopped):
		return "off"
	default:
		return "unknown"
	}
}

// serviceStateOf 计算**账期/服务维度**（与电源无关）。
//
// 优先级：终态 > 我方暂停 > 到期时间派生。
// 判据刻意不看 status：一台关机但未到期的实例，服务维度仍是 active ——
// 服务在有效期内，只是机器没开（这两件事分开正是本轮的目的）。
//
// 到期时间派生（与旧的 expire_state 同口径，保留以兼容存量消费方）：
//
//	now < expire_at 且距到期 > withinDays → active
//	now < expire_at 且距到期 <= withinDays → expiring
//	now >= expire_at                       → expired（宽限期内阶段为 grace 时归 grace）
func serviceStateOf(expireAt *time.Time, stage string, withinDays int) string {
	switch stage {
	case lifecycleStageDestroyed:
		return "destroyed"
	case lifecycleStageSuspended:
		return "suspended"
	}
	if expireAt == nil {
		return "active"
	}
	now := time.Now()
	if expireAt.After(now) {
		if !expireAt.After(now.AddDate(0, 0, normalizeDays(withinDays))) {
			return "expiring"
		}
		return "active"
	}
	// 已过期：宽限期内给更贴切的 stage 文案（仍可服务，但已进入处置流程）。
	if stage == "grace" {
		return "grace"
	}
	return "expired"
}

// transitionInfoOf 组装过渡态下发结构；无过渡态返回 nil。
func transitionInfoOf(state string, startedAt, until *time.Time) *dto.TransitionInfo {
	if strings.TrimSpace(state) == "" {
		return nil
	}
	spec := pkgmodel.InstanceTransitionOf(state)
	info := &dto.TransitionInfo{
		State:     state,
		Label:     spec.Label,
		Verb:      spec.Verb,
		StartedAt: fmtTimePtr(startedAt),
		ExpiresAt: fmtTimePtr(until),
	}
	if until != nil && time.Now().After(*until) {
		info.Overdue = true
	}
	return info
}

// normalizeAction 归一化电源动作并返回流水动作名。
func normalizeAction(action string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "on", "start":
		return "on", model.ActionPowerOn, nil
	case "off", "stop":
		return "off", model.ActionPowerOff, nil
	case "hard_off", "hard-off":
		return "hard_off", model.ActionHardOff, nil
	case "reboot", "restart":
		return "reboot", model.ActionReboot, nil
	case "hard_reboot", "hard-reboot":
		return "hard_reboot", model.ActionHardReboot, nil
	default:
		return "", "", ErrInvalidAction
	}
}

// ensurePowerAllowed 状态守卫：创建中/删除中/已销毁的实例不允许电源操作。
func ensurePowerAllowed(status, normalized string) error {
	switch status {
	case string(pkgmodel.InstanceStatusCreating), string(pkgmodel.InstanceStatusDeleting):
		return fmt.Errorf("%w: 实例正在%s", ErrStatusConflict, status)
	case string(pkgmodel.InstanceStatusDeleted):
		// 终态不可逆：已销毁实例上游记录可能仍在（软删），开机指令只会得到一个
		// 看起来成功但毫无意义的回执，还会把本地状态从 deleted 带回去。
		return fmt.Errorf("%w: 实例已销毁，无法执行电源操作", ErrStatusConflict)
	}
	switch normalized {
	case "on":
		if status == string(pkgmodel.InstanceStatusRunning) {
			return fmt.Errorf("%w: 实例已在运行中", ErrStatusConflict)
		}
	case "off", "hard_off":
		if status == string(pkgmodel.InstanceStatusStopped) {
			return fmt.Errorf("%w: 实例已关机", ErrStatusConflict)
		}
	case "reboot", "hard_reboot":
		if status != string(pkgmodel.InstanceStatusRunning) {
			return fmt.Errorf("%w: 仅运行中的实例可重启", ErrStatusConflict)
		}
	}
	return nil
}

// expectedStatus 依据动作推算本地状态（上游状态回源前的前瞻值）。
func expectedStatus(normalized, before string) string {
	switch normalized {
	case "on":
		return string(pkgmodel.InstanceStatusRunning)
	case "off", "hard_off":
		return string(pkgmodel.InstanceStatusStopped)
	case "reboot", "hard_reboot":
		return string(pkgmodel.InstanceStatusRunning)
	default:
		return before
	}
}

// normalizeDays 临期天数的兜底与上限（避免构造超长区间）。
func normalizeDays(days int) int {
	if days <= 0 {
		return defaultExpireWithinDays
	}
	if days > 365 {
		return 365
	}
	return days
}

// expireStateOf 计算到期状态与剩余天数（负数表示已过期天数）。
func expireStateOf(expireAt *time.Time, withinDays int) (string, int) {
	if expireAt == nil {
		return dto.ExpireStateNone, 0
	}
	now := time.Now()
	daysLeft := int(expireAt.Sub(now).Hours() / 24)
	if expireAt.Before(now) {
		return dto.ExpireStateExpired, daysLeft
	}
	if daysLeft <= normalizeDays(withinDays) {
		return dto.ExpireStateExpiring, daysLeft
	}
	return dto.ExpireStateNormal, daysLeft
}

// powerOf 服务状态 → 电源状态（on/off/unknown）。
func powerOf(status string) string {
	switch status {
	case string(pkgmodel.InstanceStatusRunning):
		return "on"
	case string(pkgmodel.InstanceStatusStopped):
		return "off"
	default:
		return "unknown"
	}
}

// fmtTimePtr 时间指针转 RFC3339，空值返回空串。
func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// pickInt 取非零值，否则回落默认值。
func pickInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

// pickString 取非空值，否则回落默认值。
func pickString(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// truncate 按字节上限截断错误文案（错误可能来自上游，长度不可控）。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
