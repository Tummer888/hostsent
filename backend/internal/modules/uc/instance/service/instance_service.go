// Package service 提供用户中心主机管理业务编排：列表/详情/电源操作/VNC。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"hostsent/backend/internal/modules/uc/instance/dto"
	"hostsent/backend/internal/modules/uc/instance/model"
	"hostsent/backend/internal/modules/uc/instance/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/safego"
	"hostsent/backend/internal/pkg/upstream"

	"go.uber.org/zap"
)

// Operator 操作人标识（写入操作流水，委派运维台时带上）。
// 是 admin/instance/service.Operator 的本地面投影（CONTRIBUTING §3：
// uc service 不 import admin 域 service）。
type Operator struct {
	Type string // admin / user / system
	ID   uint64
	Name string
}

// 销毁相关错误（doc91 §6.4 验收断言 1/27）。
// ProviderResolver 按提供商 ID 解析上游适配器（装配层注入，避免依赖 provider 服务）。
type ProviderResolver func(ctx context.Context, providerID uint64) (upstream.Provider, error)

// Destructor 销毁执行者（装配层委派给管理端实例运维台，doc91 §6.4）。
//
// 接口定义在本包、实现是管理端的 instanceOpsService（装配层注入）；
// Operator 与请求/响应结构均为本包 dto 的本地面，装配层负责转换。
type Destructor interface {
	Destroy(ctx context.Context, op Operator, id uint64, req *dto.DestroyRequest) error
}

// InstanceOperator 实例维护执行者（装配层委派给管理端实例运维台）。
//
// 与 Destructor 分开声明是为了"装配裁剪"：只装配了 Destructor 的部署仍可销毁，
// 维护类操作则统一返回"当前环境不支持"，不需要在 UC 侧重写能力分派、流水与事件。
type InstanceOperator interface {
	Reinstall(ctx context.Context, op Operator, id uint64, req *dto.ReinstallRequest) (*dto.ReinstallResult, error)
	ResetPassword(ctx context.Context, op Operator, id uint64, req *dto.ResetPasswordRequest) error
	Rescue(ctx context.Context, op Operator, id uint64, req *dto.RescueRequest) error
	ExitRescue(ctx context.Context, op Operator, id uint64) error
	Snapshots(ctx context.Context, id uint64, snapshotType string) ([]dto.SnapshotInfo, error)
	CreateSnapshot(ctx context.Context, op Operator, id uint64, req *dto.SnapshotCreateRequest) error
	DeleteSnapshot(ctx context.Context, op Operator, id uint64, snapshotID string) error
	RestoreSnapshot(ctx context.Context, op Operator, id uint64, req *dto.SnapshotRestoreRequest) error
}

// InstanceService 用户中心主机业务能力。
type InstanceService interface {
	List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error)
	Detail(ctx context.Context, userID, id uint64, live bool) (*dto.InstanceInfo, error)
	Power(ctx context.Context, userID, id uint64, action string) error
	VNC(ctx context.Context, userID, id uint64) (*dto.VNCResult, error)
	// Destroy 用户自助销毁实例（归属校验 → 转操作人 → 委派运维台）。
	Destroy(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.DestroyRequest) error
	// 维护类自助操作（重装/重置密码/救援/快照）；归属校验后委派运维台。
	Reinstall(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.ReinstallRequest) (*dto.ReinstallResult, error)
	ResetPassword(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.ResetPasswordRequest) error
	Rescue(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.RescueRequest) error
	ExitRescue(ctx context.Context, userID, actorID uint64, actorName string, id uint64) error
	Snapshots(ctx context.Context, userID, id uint64, snapshotType string) ([]dto.SnapshotInfo, error)
	CreateSnapshot(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.SnapshotCreateRequest) error
	DeleteSnapshot(ctx context.Context, userID, actorID uint64, actorName string, id uint64, snapshotID string) error
	RestoreSnapshot(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.SnapshotRestoreRequest) error
	// Operations 实例操作流水（用户可见的只读审计）。
	Operations(ctx context.Context, userID, id uint64, limit int) ([]dto.OperationEntry, error)
	// MetricsCatalog 监控图表目录；Supported=false 表示平台不提供监控。
	MetricsCatalog(ctx context.Context, userID, id uint64) (*dto.MetricsCatalogResponse, error)
	// Metrics 查询单项监控序列。
	Metrics(ctx context.Context, userID, id uint64, query *dto.MetricsQuery) (*dto.MetricsSeries, error)
	// SetDestructor 注入销毁执行者（装配层调用；未注入时销毁返回明确错误）。
	SetDestructor(d Destructor)
	// SetOperator 注入维护执行者（装配层调用；未注入时维护类操作返回明确错误）。
	SetOperator(o InstanceOperator)
	// RetryProvision 重新开通待开通实例（占位行）：重投来源订单的履约任务。
	RetryProvision(ctx context.Context, userID, actorID uint64, actorName string, id uint64) (*dto.InstanceInfo, error)
	// SetProvisionRetry 注入「重新开通」执行者（装配层调用；未注入时该入口返回明确错误）。
	SetProvisionRetry(fn func(ctx context.Context, orderID uint64) error)
	// SetLogger 注入日志器（装配层调用，可选）：非致命异常记录用。
	SetLogger(l *zap.Logger)
}

type instanceService struct {
	repo    repository.InstanceRepository
	resolve ProviderResolver
	// destructor 销毁执行者（管理端运维台），可为 nil。
	destructor Destructor
	// operator 维护类执行者（管理端运维台），可为 nil。
	operator InstanceOperator
	// provisionRetry 「重新开通」执行者（装配层注入：重投来源订单的履约任务），可为 nil。
	provisionRetry func(ctx context.Context, orderID uint64) error
	// logger 可选：状态落库失败等非致命异常记日志（nil 时静默）。
	logger *zap.Logger
}

// ErrMaintenanceUnsupported 维护执行者未装配（部署裁剪时）。

// ErrInstanceNotProvisioned 实例尚未开通（待开通占位行）：侧端操作无从下手。

// errUnsupportedControl 该商品/渠道不支持电源与控制台操作。
// 措辞对客户只讲"不支持"，不暴露平台对接细节。

// errProviderUnavailable 底层渠道不可用。面向客户统一为一句话，
// 具体原因（哪个渠道、什么错误）只进日志，不给客户看对接细节。

// customerProvisionError 把开通失败的原始错误翻译成客户可读的原因。
//
// 为什么需要这层：provision_tasks.last_error 存的是上游 API 原文
// （形如 `upstream POST /index.php?m=api&a=...: http=500 code=123 msg=...`），
// 里面含上游接口地址、HTTP 状态与上游业务码，属于纯内部信息。
// 这里按已知失败类型映射为一句人话；认不出的统一归到"系统处理中"，
// 原文仍保留在库与服务端日志，运营排障不受影响。
func customerProvisionError(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return ""
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(msg, "余额不足") || strings.Contains(msg, "账单未结清") ||
		strings.Contains(msg, "未完成支付") || strings.Contains(lower, "insufficient"):
		return "开通排队中：资源正在补货，请稍候；如长时间未开通可联系客服加急。"
	case strings.Contains(msg, "主机名") && strings.Contains(msg, "存在"):
		return "主机名已被占用，请修改主机名后重新提交。"
	case strings.Contains(msg, "密码"):
		return "密码不符合要求，请调整后重新提交。"
	case strings.Contains(msg, "参数") || strings.Contains(msg, "不合法"):
		return "所选配置暂不可用，请返回商品页重新选择配置。"
	case strings.Contains(msg, "库存") || strings.Contains(msg, "售罄") || strings.Contains(msg, "不足"):
		return "所选配置库存不足，请稍后重试或联系客服。"
	case strings.Contains(lower, "timeout") || strings.Contains(msg, "超时"):
		return "服务响应超时，系统正在自动重试，请稍后刷新查看。"
	default:
		// 认不出的错误一律给中性结论：不把上游原文抛给客户。
		return "系统正在处理中，请稍后刷新查看；如长时间未开通请联系客服。"
	}
}

// ErrProvisionRetryUnavailable 重新开通入口未装配。

// ErrAlreadyProvisioned 实例已开通，无需重新开通。

// ErrMetricsUnsupported 当前服务商不提供性能监控。

// ErrMetricsTypeRequired 监控查询缺少 type。

// 生命周期阶段（与 upstream/管理端同口径的字符串值；只用于读取判断，不落库）。
const (
	lifecycleStageDestroyed = "destroyed"
)

// 到期状态（读时计算，与管理端 admin instance dto 同口径）。
const (
	expireStateExpiring = "expiring"
	expireStateExpired  = "expired"
	expireStateNormal   = "normal"
	expireStateNone     = "none"
)

// expireWithinDays 用户侧「即将到期」判定天数：到期前 7 天内提示续费。
const expireWithinDays = 7

// NewInstanceService 创建用户中心主机服务。
func NewInstanceService(repo repository.InstanceRepository, resolve ProviderResolver) InstanceService {
	return &instanceService{repo: repo, resolve: resolve}
}

// SetDestructor 注入销毁执行者。
func (s *instanceService) SetDestructor(d Destructor) { s.destructor = d }

// SetOperator 注入维护类执行者。
func (s *instanceService) SetOperator(o InstanceOperator) { s.operator = o }

// SetLogger 注入日志器（可选）：状态落库失败等非致命异常记录用。
func (s *instanceService) SetLogger(l *zap.Logger) { s.logger = l }

// SetProvisionRetry 注入「重新开通」执行者（装配层调用）。
func (s *instanceService) SetProvisionRetry(fn func(ctx context.Context, orderID uint64) error) {
	s.provisionRetry = fn
}

// RetryProvision 重新开通：待开通实例（占位行）重投来源订单的开通履约任务。
// 归属校验用带 user_id 的查询完成；已开通的实例拒绝（避免误触重复开通）。
func (s *instanceService) RetryProvision(ctx context.Context, userID, actorID uint64, actorName string, id uint64) (*dto.InstanceInfo, error) {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if it.ProviderInstanceID != "" {
		return nil, ErrAlreadyProvisioned
	}
	if s.provisionRetry == nil {
		return nil, ErrProvisionRetryUnavailable
	}
	if err := s.provisionRetry(ctx, it.OrderID); err != nil {
		return nil, err
	}
	_ = actorID
	_ = actorName
	return s.Detail(ctx, userID, id, false)
}

// userOperator 组装委派给运维台的操作人（子账号记为真实操作人）。
func userOperator(userID, actorID uint64, actorName string) Operator {
	if actorID == 0 {
		actorID = userID
	}
	return Operator{Type: "user", ID: actorID, Name: actorName}
}

// ownedInstance 归属校验 + 取出实例行；失败统一返回"不存在或无权访问"。
func (s *instanceService) ownedInstance(ctx context.Context, userID, id uint64) (*model.Instance, error) {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return nil, ErrInstanceNotFoundOrDenied
	}
	return it, nil
}

// Reinstall 用户自助重装系统（可换镜像）。平台新签发的初始凭据一次性回传。
func (s *instanceService) Reinstall(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.ReinstallRequest) (*dto.ReinstallResult, error) {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if s.operator == nil {
		return nil, ErrMaintenanceUnsupported
	}
	res, err := s.operator.Reinstall(ctx, userOperator(userID, actorID, actorName), it.ID, &dto.ReinstallRequest{
		OS: req.OS, Port: req.Port, FormatDataDisk: req.FormatDataDisk,
		SystemDiskSize: req.SystemDiskSize, Reason: req.Reason,
	})
	if err != nil {
		// 归属正确但平台不支持时，把"不支持"翻译成用户能懂的话。
		if errors.Is(err, ErrCapabilityUnsupported) {
			return nil, fmt.Errorf("当前服务商不支持重装系统")
		}
		return nil, err
	}
	out := &dto.ReinstallResult{}
	if res != nil {
		out.Username, out.Password = res.Username, res.Password
	}
	return out, nil
}

// ResetPassword 用户自助重置登录密码。
func (s *instanceService) ResetPassword(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.ResetPasswordRequest) error {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return err
	}
	if s.operator == nil {
		return ErrMaintenanceUnsupported
	}
	return s.operator.ResetPassword(ctx, userOperator(userID, actorID, actorName), it.ID,
		&dto.ResetPasswordRequest{Password: req.Password})
}

// Rescue 用户自助进入救援系统。
func (s *instanceService) Rescue(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.RescueRequest) error {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return err
	}
	if s.operator == nil {
		return ErrMaintenanceUnsupported
	}
	return s.operator.Rescue(ctx, userOperator(userID, actorID, actorName), it.ID,
		&dto.RescueRequest{System: req.System, TempPassword: req.TempPassword})
}

// ExitRescue 用户自助退出救援系统。
func (s *instanceService) ExitRescue(ctx context.Context, userID, actorID uint64, actorName string, id uint64) error {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return err
	}
	if s.operator == nil {
		return ErrMaintenanceUnsupported
	}
	return s.operator.ExitRescue(ctx, userOperator(userID, actorID, actorName), it.ID)
}

// Snapshots 列出自有实例的快照/备份。
func (s *instanceService) Snapshots(ctx context.Context, userID, id uint64, snapshotType string) ([]dto.SnapshotInfo, error) {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if s.operator == nil {
		return nil, ErrMaintenanceUnsupported
	}
	rows, err := s.operator.Snapshots(ctx, it.ID, snapshotType)
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

// CreateSnapshot 用户自助创建快照/备份。
func (s *instanceService) CreateSnapshot(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.SnapshotCreateRequest) error {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return err
	}
	if s.operator == nil {
		return ErrMaintenanceUnsupported
	}
	return s.operator.CreateSnapshot(ctx, userOperator(userID, actorID, actorName), it.ID,
		&dto.SnapshotCreateRequest{Type: req.Type, Name: req.Name, DiskID: req.DiskID})
}

// DeleteSnapshot 用户自助删除自有快照/备份。
func (s *instanceService) DeleteSnapshot(ctx context.Context, userID, actorID uint64, actorName string, id uint64, snapshotID string) error {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return err
	}
	if s.operator == nil {
		return ErrMaintenanceUnsupported
	}
	return s.operator.DeleteSnapshot(ctx, userOperator(userID, actorID, actorName), it.ID, snapshotID)
}

// RestoreSnapshot 用户自助用快照恢复（高危，需二次确认标识）。
func (s *instanceService) RestoreSnapshot(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.SnapshotRestoreRequest) error {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return err
	}
	if s.operator == nil {
		return ErrMaintenanceUnsupported
	}
	return s.operator.RestoreSnapshot(ctx, userOperator(userID, actorID, actorName), it.ID,
		&dto.SnapshotRestoreRequest{SnapshotID: req.SnapshotID, ConfirmMark: req.ConfirmMark})
}

// Destroy 用户自助销毁实例（doc91 §6.4）。
//
// 关键行为约束：
//  1. 归属校验必须在委派之前——用带 user_id 过滤的仓储查询，绝不能只依赖
//     管理端的 FindByID（那是跨用户的）。
//  2. 操作人记为 user，ID 取真实操作人（子账号）而不是归属主账号。
//  3. 不触发退款/余额返还：销毁只终止上游实例并把本地状态置 deleted，
//     未到期余额需按业务规则另行处理（本轮不做自动退款）。
func (s *instanceService) Destroy(ctx context.Context, userID, actorID uint64, actorName string, id uint64, req *dto.DestroyRequest) error {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return ErrInstanceNotFoundOrDenied
	}
	if s.destructor == nil {
		return ErrDestroyUnsupported
	}
	if actorID == 0 {
		actorID = userID
	}
	op := Operator{Type: "user", ID: actorID, Name: actorName}
	return s.destructor.Destroy(ctx, op, it.ID, &dto.DestroyRequest{
		ConfirmMark: req.ConfirmMark,
		Reason:      req.Reason,
	})
}

// List 我的主机列表。live=true 时逐个刷新上游运行状态。
//
// 汇总数字与 items 在同一循环里累加：列表页顶部卡片（共 N 台 / 运行中 / 即将到期）
// 若前端自己数，要么再遍历一次、要么与后端统计口径分叉（比如"即将到期"的天数阈值）。
func (s *instanceService) List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error) {
	items, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.InstanceInfo, 0, len(items))
	summary := dto.ListSummary{Total: len(items)}
	// 能力位按渠道解析一次即复用：一个账号下的机器通常只属于一两个渠道，
	// 逐台解析会为同一渠道重复查库；但**不能干脆不填**——前端拿到全 false 的
	// 能力位会把所有按钮置灰（"有值但不支持"比"没有值"更危险）。
	//
	// 能力位只做"该渠道是否实现某接口"的静态判定（无上游 HTTP），因此放在并发段之前
	// 串行完成，避免给下面的并发回源引入一张需要加锁的共享 map。
	capsCache := make(map[uint64]dto.Capabilities, 4)
	for i := range items {
		out = append(out, toInfo(&items[i]))
		out[i].Capabilities = s.capabilitiesCached(ctx, &items[i], capsCache)
	}
	if !query.Live || s.resolve == nil {
		for i := range out {
			out[i].PowerStatus = powerOf(statusToModel(items[i].Status))
			countSummary(&summary, out[i])
		}
		return &dto.ListResponse{Items: out, Total: int64(len(out)), Summary: summary}, nil
	}

	// 实时刷新：逐台回源上游取运行状态。
	//
	// 必须并发且带总预算。原来是串行 for 循环里一台一台调 GetInstance，
	// 每台一次上游 HTTP（实测约 2~3s）；账号下 13 台机器就是 30s+，
	// 直接顶穿网关超时——前端表现为"列表报错、数据全空"（用户实测反馈）。
	// 并发后总耗时约等于最慢的一台；总预算再兜一层，防止个别上游 hang 住拖死请求。
	liveStatus := s.refreshStatuses(ctx, items, refreshConcurrency, refreshBudget)
	for i := range out {
		if st, ok := liveStatus[items[i].ID]; ok {
			out[i].Status = string(st.Status)
			out[i].PowerStatus = powerOf(st.Status)
		} else {
			// 未取到（不支持该操作/上游报错/超预算）：降级为本地状态，
			// 不因单台失败影响整页（宁可显示可能略旧的本地状态，也不要空白页）。
			out[i].PowerStatus = powerOf(statusToModel(items[i].Status))
		}
		countSummary(&summary, out[i])
	}
	return &dto.ListResponse{Items: out, Total: int64(len(out)), Summary: summary}, nil
}

// 实时回源的并发度与总预算。
//
// 并发度取 8：一次列表最多打出 8 个上游请求，既能把 13 台机器压到"一轮多"，
// 又不至于对上游形成压测式突发。
// 总预算 8s：网关通常 30s 左右断连，留足余量；即使部分上游 hang 住，
// 也只是这些机器回落本地状态，页面照常出得来。
const (
	refreshConcurrency = 8
	refreshBudget      = 8 * time.Second
)

// refreshStatuses 并发回源刷新各实例的运行状态，按实例 ID 返回成功结果。
// 失败与超预算的实例不出现在返回值里，由调用方回落本地状态。
//
// **不报电源状态的渠道（如魔方财务）一律不收进返回值**：那种上游只返回
// "服务是否激活"（domainstatus），没有机器电源字段，照抄会把客户刚关的机
// 显示回"运行中"（用户反馈"状态混乱"的根因之一）。让调用方回落到本地值，
// 正是我们要保留的"我们下发指令后唯一可靠的知识"。
func (s *instanceService) refreshStatuses(ctx context.Context, items []model.Instance, concurrency int, budget time.Duration) map[uint64]*pkgmodel.StandardInstance {
	targets := make([]model.Instance, 0, len(items))
	for _, it := range items {
		// 待开通占位（provider_instance_id 为空）没有上游实体，回源必然失败，直接跳过。
		if it.ProviderInstanceID != "" {
			targets = append(targets, it)
		}
	}
	if len(targets) == 0 {
		return nil
	}
	if concurrency < 1 {
		concurrency = 1
	}
	// 超预算即取消在途请求：返回的 map 只含已完成的部分，其余由调用方降级。
	budgetCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	out := make(map[uint64]*pkgmodel.StandardInstance, len(targets))
	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := range targets {
		it := targets[i]
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			// 并发入口按项目约定兜 panic：单台异常不应击穿整个列表请求。
			defer safego.Recover(nil)
			// 不报电源状态的渠道：回源值答的不是"机器开着吗"，不参与状态覆盖。
			if provider, perr := s.buildProvider(budgetCtx, it.ProviderID); perr != nil ||
				!upstream.SupportsPowerStateReporting(provider) {
				return
			}
			res, ok := s.refresh(budgetCtx, &it)
			if !ok {
				return
			}
			mu.Lock()
			out[it.ID] = res
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

// capabilitiesCached 单次请求内的能力位缓存（按渠道 ID）。
func (s *instanceService) capabilitiesCached(ctx context.Context, it *model.Instance, cache map[uint64]dto.Capabilities) dto.Capabilities {
	if it == nil || it.ProviderInstanceID == "" {
		return dto.Capabilities{}
	}
	if caps, ok := cache[it.ProviderID]; ok {
		return caps
	}
	caps := s.capabilities(ctx, it)
	cache[it.ProviderID] = caps
	return caps
}

// countSummary 把一台机器的可见状态累加进汇总（与前端徽标同口径）。
func countSummary(s *dto.ListSummary, info dto.InstanceInfo) {
	if info.Pending {
		s.Creating++
		return
	}
	switch {
	case info.LifecycleStage == lifecycleStageDestroyed || info.Status == "deleted":
		s.Destroyed++
	case info.ExpireState == expireStateExpired:
		s.Expired++
	case info.ExpireState == expireStateExpiring:
		s.Expiring++
	case info.Status == "running":
		s.Running++
	case info.Status == "stopped":
		s.Stopped++
	default:
		s.Creating++
	}
}

// Detail 我的主机详情。live=true 时刷新上游运行状态。
func (s *instanceService) Detail(ctx context.Context, userID, id uint64, live bool) (*dto.InstanceInfo, error) {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	info := toInfo(it)
	// 待开通实例：带上履约状态与失败原因，用户能直接看到"为什么还没开通"。
	if it.ProviderInstanceID == "" {
		if status, lastErr, perr := s.repo.Provision(ctx, it.OrderID); perr == nil {
			info.ProvisionStatus = status
			// 失败原因必须过一遍面向客户的翻译：原始 last_error 是上游 API 原文
			// （含上游地址、HTTP 状态、上游业务码与原始留言），直接展示等于把
			// 对接链路摊给客户看。原文仍留在库与服务端日志，运营排障不受影响。
			info.ProvisionError = customerProvisionError(lastErr)
		}
	}
	// 待开通占位（无上游实例号）：跳过回源（上游还没有这台机器），保持本地信息。
	if live && s.resolve != nil && it.ProviderInstanceID != "" {
		if liveInst, ok := s.refresh(ctx, it); ok {
			s.applyLiveStatus(ctx, it, liveInst, &info)
			if liveInst.PublicIP != "" {
				info.PublicIP = liveInst.PublicIP
			}
		}
	} else {
		info.PowerStatus = powerOf(statusToModel(it.Status))
	}
	// 回源可能刚清掉过渡态（上游已确认）：重新取一次，避免界面继续显示「重启中」。
	if it.TransitionState == "" {
		info.Transition = nil
	}
	info.Capabilities = s.capabilities(ctx, it)
	// NAT 机型补齐访问地址（doc122）：NAT 没有独立 IP，public_ip 为空，
	// 不给转发地址客户就无从连上机器。
	s.attachNatAccess(ctx, it, &info)
	return &info, nil
}

// attachNatAccess 为用户中心详情补齐 NAT 访问地址（doc122）。
//
// 与用户无关的判据：先按机型判断是不是 NAT（IsNatInstance 认 raw_data），
// 非 NAT 直接返回，不发起上游请求。
//
// 容错：读取失败只填 Error 不返回错误——NAT 这个结论本身（is_nat=true）
// 对客户仍然有用（至少知道这机器没独立 IP），不该因为上游抖动让详情页打不开。
func (s *instanceService) attachNatAccess(ctx context.Context, it *model.Instance, info *dto.InstanceInfo) {
	if it == nil || info == nil {
		return
	}
	if !upstream.IsNatInstance(it.RawData) {
		return
	}
	nat := &dto.NatAccessInfo{IsNat: true}
	info.NatAccess = nat
	if it.ProviderInstanceID == "" {
		// 待开通：还没有上游实例号，读不到规则属正常。
		return
	}
	provider, err := s.buildProvider(ctx, it.ProviderID)
	if err != nil {
		nat.Error = "服务暂时不可达，请稍后刷新重试"
		return
	}
	reader, ok := provider.(upstream.NatReader)
	if !ok {
		nat.Error = "该主机暂不支持查看转发地址"
		return
	}
	forwards, ferr := reader.ListNatForwards(ctx, it.InstanceID)
	if ferr != nil {
		nat.Error = "转发地址读取失败，请稍后刷新重试"
		return
	}
	for _, f := range forwards {
		if addr := upstream.FormatNatForwardAddr(f.HostIP, f.ExternalPort); addr != "" {
			nat.Forwards = append(nat.Forwards, addr)
		}
	}
	webs, werr := reader.ListNatWebs(ctx, it.InstanceID)
	if werr != nil {
		if len(nat.Forwards) == 0 {
			nat.Error = "读取 NAT 建站地址失败，请稍后刷新重试"
		}
		return
	}
	for _, w := range webs {
		if w.Domain != "" {
			nat.Domains = append(nat.Domains, w.Domain)
		}
	}
}

// capabilities 推导该实例可用的操作（用户侧按钮门禁）。
//
// 与管理端的差别：这里只暴露**用户确实能自助触发**的项，且待开通实例一律全关
// （没有上游实例号，任何动作都必然失败）。
func (s *instanceService) capabilities(ctx context.Context, it *model.Instance) dto.Capabilities {
	if it == nil || it.ProviderInstanceID == "" {
		return dto.Capabilities{}
	}
	provider, err := s.buildProvider(ctx, it.ProviderID)
	if err != nil {
		return dto.Capabilities{}
	}
	_, ctrl := provider.(upstream.InstanceControl)
	_, admin := provider.(upstream.InstanceAdministration)
	_, terminate := provider.(upstream.InstanceTermination)
	_, reinstall := provider.(upstream.InstanceReinstall)
	_, pwdReset := provider.(upstream.InstancePasswordReset)
	_, rescue := provider.(upstream.InstanceRescue)
	_, snapshot := provider.(upstream.InstanceSnapshot)
	_, metrics := provider.(upstream.InstanceMetrics)
	desc := upstream.CapabilitiesOf(provider)
	supports := func(op string) bool { return upstream.SupportsOperation(desc, op) }
	return dto.Capabilities{
		Power:         ctrl || supports(upstream.OpStart) || supports(upstream.OpStop),
		Console:       ctrl || supports(upstream.OpVNC),
		Destroy:       admin || terminate || supports(upstream.OpDestroy),
		Reinstall:     reinstall || supports(upstream.OpReinstall),
		ResetPassword: pwdReset || supports(upstream.OpResetPassword),
		Rescue:        rescue || supports(upstream.OpRescue),
		Snapshot:      snapshot || supports(upstream.OpSnapshot),
		Metrics:       metrics || supports(upstream.OpMetrics),
		NatAccess:     supports(upstream.OpNatAccess),
	}
}

// Operations 实例操作流水（只读）。
func (s *instanceService) Operations(ctx context.Context, userID, id uint64, limit int) ([]dto.OperationEntry, error) {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.Operations(ctx, it.ID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.OperationEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.OperationEntry{
			ID:           r.ID,
			Action:       r.Action,
			OperatorType: r.OperatorType,
			OperatorName: r.OperatorName,
			Result:       r.Result,
			ErrorMessage: r.ErrorMessage,
			CreatedAt:    r.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// MetricsCatalog 监控图表目录。平台不提供时返回 Supported=false + 原因文案，
// 而不是报错：前端据此显示说明区块，页面其余部分照常可用。
func (s *instanceService) MetricsCatalog(ctx context.Context, userID, id uint64) (*dto.MetricsCatalogResponse, error) {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if it.ProviderInstanceID == "" {
		return &dto.MetricsCatalogResponse{Supported: false, Reason: "实例尚未开通，暂无可监控的机器"}, nil
	}
	provider, err := s.buildProvider(ctx, it.ProviderID)
	if err != nil {
		return &dto.MetricsCatalogResponse{Supported: false, Reason: errProviderUnavailable.Error()}, nil
	}
	metrics, ok := provider.(upstream.InstanceMetrics)
	if !ok {
		return &dto.MetricsCatalogResponse{Supported: false, Reason: "该主机暂不提供性能监控"}, nil
	}
	charts, err := metrics.ListMetricCharts(ctx, &upstream.MetricsCatalogRequest{
		ProviderInstanceID: it.InstanceID,
		InstanceRecordID:   it.ID,
	})
	if err != nil {
		return nil, err
	}
	out := &dto.MetricsCatalogResponse{Supported: len(charts) > 0, Charts: make([]dto.MetricsChartInfo, 0, len(charts))}
	if len(charts) == 0 {
		out.Reason = "该实例类型暂不提供性能监控（如 Hyper-V 面板无统计数据）"
		return out, nil
	}
	for _, c := range charts {
		item := dto.MetricsChartInfo{
			Type: c.Type, Kind: c.Kind, Title: c.Title,
			Select: make([]dto.MetricsChartOption, 0, len(c.Select)),
		}
		for _, opt := range c.Select {
			item.Select = append(item.Select, dto.MetricsChartOption{Label: opt.Label, Value: opt.Value})
		}
		out.Charts = append(out.Charts, item)
	}
	return out, nil
}

// Metrics 查询单项监控序列。
func (s *instanceService) Metrics(ctx context.Context, userID, id uint64, query *dto.MetricsQuery) (*dto.MetricsSeries, error) {
	it, err := s.ownedInstance(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if it.ProviderInstanceID == "" {
		return nil, ErrInstanceNotProvisioned
	}
	if query == nil || query.Type == "" {
		return nil, ErrMetricsTypeRequired
	}
	provider, err := s.buildProvider(ctx, it.ProviderID)
	if err != nil {
		return nil, err
	}
	metrics, ok := provider.(upstream.InstanceMetrics)
	if !ok {
		return nil, ErrMetricsUnsupported
	}
	start, end := resolveMetricWindow(query)
	series, err := metrics.QueryMetrics(ctx, &upstream.MetricsQuery{
		ProviderInstanceID: it.InstanceID,
		InstanceRecordID:   it.ID,
		Type:               query.Type,
		Select:             query.Select,
		StartMillis:        start,
		EndMillis:          end,
	})
	if err != nil {
		return nil, err
	}
	return toMetricSeries(series), nil
}

// resolveMetricWindow 归一化时间窗：显式 start/end 优先，其次 range 预设，最后默认 6 小时。
//
// 为什么由后端定默认窗口而不是像源码那样固定 7 天：面板统计接口按点返回，
// 7 天 × 5 分钟粒度是两千多个点，用户端画一张图没必要，也拖慢上游。
func resolveMetricWindow(q *dto.MetricsQuery) (int64, int64) {
	now := time.Now()
	end := q.End
	if end <= 0 {
		end = now.UnixMilli()
	}
	if q.Start > 0 && q.Start < end {
		return q.Start, end
	}
	durations := map[string]time.Duration{
		"1h":  time.Hour,
		"6h":  6 * time.Hour,
		"24h": 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
	}
	d, ok := durations[q.Range]
	if !ok {
		d = 6 * time.Hour
	}
	return end - d.Milliseconds(), end
}

// toMetricSeries 上游序列 → 用户侧 DTO（只搬字段，不做换算）。
func toMetricSeries(s *upstream.MetricSeries) *dto.MetricsSeries {
	out := &dto.MetricsSeries{Label: []string{}, List: [][]dto.MetricsPoint{}}
	if s == nil {
		return out
	}
	out.Type, out.Unit, out.ChartType = s.Type, s.Unit, s.ChartType
	out.Label = s.Label
	if out.Label == nil {
		out.Label = []string{}
	}
	for _, points := range s.List {
		row := make([]dto.MetricsPoint, 0, len(points))
		for _, p := range points {
			row = append(row, dto.MetricsPoint{Time: p.Time, Value: p.Value})
		}
		out.List = append(out.List, row)
	}
	return out
}

// Power 电源操作：开机/关机/重启/硬关机/硬重启。
// 与 UC 用户侧一致地优先使用平台的独立硬电源指令（客户端点「硬关机」不该只是"尽量硬"）。
func (s *instanceService) Power(ctx context.Context, userID, id uint64, action string) error {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return err
	}
	// 待开通实例（占位行）：没有上游实例号，电源操作必然失败，给出可操作提示。
	if it.ProviderInstanceID == "" {
		return ErrInstanceNotProvisioned
	}
	provider, err := s.buildProvider(ctx, it.ProviderID)
	if err != nil {
		return err
	}
	providerImpl := provider
	control, ok := providerImpl.(upstream.InstanceControl)
	if !ok {
		return errUnsupportedControl
	}

	// ---- 过渡态闸门 + 原子抢占（迁移 112，与运维台同一口径）----
	//
	// 客户侧此前完全没有这层：下发后只有一句"指令已发送"，机器要几十秒才变，
	// 期间再点一次就会堆一条指令（用户实测反馈"连续点两次开机，两条都成功"）。
	//
	// 顺序很重要：先做友好的占用检查（能说明"在做什么、从几点开始"），
	// 再原子抢占（真正的互斥，防并发连点），最后才下发。
	normalized := strings.ToLower(action)
	if err := ensureNoActiveTransition(it, normalized); err != nil {
		return err
	}
	transition := transitionOfPowerAction(normalized)
	claimed, claimedAt, cerr := s.claimTransition(ctx, it.ID, transition)
	if cerr != nil {
		return errProviderUnavailable
	}
	if !claimed {
		return s.transitionBusyError(ctx, it)
	}

	hard, hasHard := providerImpl.(upstream.InstancePowerHard)
	var opErr error
	switch normalized {
	case "on":
		opErr = control.StartInstance(ctx, it.InstanceID)
	case "off":
		opErr = control.StopInstance(ctx, it.InstanceID, false)
	case "hard_off":
		if hasHard {
			opErr = hard.HardStopInstance(ctx, it.InstanceID)
		} else {
			opErr = control.StopInstance(ctx, it.InstanceID, true)
		}
	case "reboot":
		opErr = control.RestartInstance(ctx, it.InstanceID)
	case "hard_reboot":
		if hasHard {
			opErr = hard.HardRestartInstance(ctx, it.InstanceID)
		} else {
			// 上游没有独立硬重启：退化为硬关再开。
			if opErr = control.StopInstance(ctx, it.InstanceID, true); opErr == nil {
				opErr = control.StartInstance(ctx, it.InstanceID)
			}
		}
	default:
		s.releaseTransition(ctx, it.ID, transition)
		return fmt.Errorf("暂不支持的操作: %s", action)
	}
	if opErr != nil {
		// 下发失败：把抢占的占位还回去（仅当仍属于自己），
		// 否则实例会被一个从未执行的动作锁到窗口超时。
		s.releaseTransition(ctx, it.ID, transition)
		return opErr
	}
	// 指令下发成功就立刻把预期状态落到本地，用户不必等下一轮回源才能看到变化。
	// 上游状态有延迟（关机通常要几秒），落库后前端立刻能反映"正在关机/已关机"，
	// 否则界面上什么都没发生 —— 用户会以为按钮没生效而反复点。
	if expected := expectedPowerStatus(strings.ToLower(action), it.Status); expected != "" && expected != it.Status {
		if uerr := s.repo.UpdateStatus(ctx, it.ID, expected); uerr != nil {
			// 落库失败不影响操作结果（上游已收到指令），仅本地状态短暂偏旧。
			s.logUpdateStatusFailure(it, expected, uerr)
		}
	}
	// 尽力回源对齐一次真实状态：拿不到就保留上面落的预期状态。
	//
	// 回源同时承担过渡态收尾：上游给出新状态（或该渠道不报电源状态、
	// 指令已受理即算完成）且已过最短占用窗口时，清掉过渡态。
	// 窗口内保留 —— 重启是 运行中→运行中 的转场，上游"确认"了也看不出变化，
	// 一确认就放行会让客户几秒后再点一次同样堆指令。
	if live, ok := s.refresh(ctx, it); ok && live.Status != "" {
		if !upstream.SupportsPowerStateReporting(providerImpl) {
			// 上游不报电源状态：回源值答的不是"机器开着吗"，不能改写本地。
			// 过渡态按"指令已受理"收尾（见下）。
		} else if uerr := s.repo.UpdateStatus(ctx, it.ID, string(live.Status)); uerr != nil {
			s.logUpdateStatusFailure(it, string(live.Status), uerr)
		}
		if !transitionWithinMinHold(transition, claimedAt) {
			if serr := s.repo.ClearTransitionIfOwner(ctx, it.ID, transition); serr != nil && s.logger != nil {
				s.logger.Warn("清除过渡态失败", zap.Uint64("instance_id", it.ID), zap.Error(serr))
			}
		}
	}
	return nil
}

// ensureNoActiveTransition 过渡态闸门：有未确认的动作时拒绝新指令（客户侧）。
//
// 两个例外必须放行，否则一次异常会把实例永久锁死：
//   - 已超时（超过该动作的典型耗时上限）：视为结果未知，放行；
//   - 开关机方向相反的动作：机器正在关机时点"开机"是常见误操作纠正，
//     但这里同样按占用拦截 —— 中间态叠加会让最终状态无法判断。
//
// 这只是快速失败的友好提示；真正的互斥由 claimTransition 的原子抢占保证。
func ensureNoActiveTransition(it *model.Instance, action string) error {
	if it == nil || strings.TrimSpace(it.TransitionState) == "" {
		return nil
	}
	if it.TransitionUntil != nil && time.Now().After(*it.TransitionUntil) {
		return nil
	}
	return transitionBusyErrorFor(it.TransitionState, it.TransitionStartedAt, action)
}

// claimTransition 原子抢占过渡态。返回 claimed=false 表示已有生效中的动作。
func (s *instanceService) claimTransition(ctx context.Context, id uint64, state string) (bool, time.Time, error) {
	if state == "" {
		return true, time.Time{}, nil
	}
	spec := pkgmodel.InstanceTransitionOf(state)
	now := time.Now()
	until := now.Add(time.Duration(spec.TerminalTimeoutSec) * time.Second)
	claimed, err := s.repo.ClaimTransition(ctx, id, state, now, until)
	if err != nil {
		return false, now, err
	}
	return claimed, now, nil
}

// transitionOfPowerAction 电源动作 → 过渡态值（与运维台同一映射）。
//
// 硬关与普通关机归到同一个过渡态：客户看到的文案都是"关机中"，
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

// releaseTransition 下发失败时回滚自己抢到的过渡态（仅当仍属于自己）。
func (s *instanceService) releaseTransition(ctx context.Context, id uint64, state string) {
	if state == "" {
		return
	}
	if err := s.repo.ClearTransitionIfOwner(ctx, id, state); err != nil && s.logger != nil {
		s.logger.Warn("回滚过渡态失败", zap.Uint64("instance_id", id), zap.Error(err))
	}
}

// transitionBusyError 抢占失败时的拒绝提示：读最新行拿占用者与开始时间。
func (s *instanceService) transitionBusyError(ctx context.Context, it *model.Instance) error {
	if it == nil {
		return ErrOperationInProgress
	}
	if fresh, err := s.repo.FindByUser(ctx, it.UserID, it.ID); err == nil && fresh != nil && fresh.TransitionState != "" {
		return transitionBusyErrorFor(fresh.TransitionState, fresh.TransitionStartedAt, "")
	}
	return transitionBusyErrorFor(it.TransitionState, it.TransitionStartedAt, "")
}

// transitionBusyErrorFor 组装面向客户的占用提示。
//
// 措辞让客户能自己回答三个问题：在做什么、从什么时候开始、还要等多久。
// 不写"过渡态""锁"这类内部词汇 —— 客户只关心"能不能点、要等多久"。
func transitionBusyErrorFor(state string, startedAt *time.Time, action string) error {
	spec := pkgmodel.InstanceTransitionOf(state)
	_ = action
	since := ""
	if startedAt != nil {
		since = fmt.Sprintf("（%s 开始）", startedAt.Format("15:04:05"))
	}
	return fmt.Errorf("%w：正在%s%s，通常需要 1–3 分钟，请等待完成后再操作。若长时间无变化可点「刷新状态」",
		ErrOperationInProgress, spec.Verb, since)
}

// transitionWithinMinHold 判断过渡态是否仍在最短占用窗口内（客户侧副本，
// 与运维台同一规则；uc 包不 import admin 域服务）。
//
// true = 还在窗口内，回源后应保留过渡态（不放行重复指令）。
// 重启是 运行中→运行中 的转场，上游"确认"了也看不出变化；一确认就放行会让
// 客户几秒后再点一次、同样堆指令（用户实测反馈"任务累积"）。
func transitionWithinMinHold(state string, claimedAt time.Time) bool {
	if claimedAt.IsZero() {
		return false
	}
	spec := pkgmodel.InstanceTransitionOf(state)
	if spec.MinHoldSec <= 0 {
		return false
	}
	return time.Since(claimedAt) < time.Duration(spec.MinHoldSec)*time.Second
}

// expectedPowerStatus 由动作与当前状态推出落库的预期状态。
//
// 重启类不在此列：重启是「运行中 → 运行中」的转场，落库没有可写的中间态
// （真正的中间态由上游侧短暂体现），回源对齐会把它拉正。
func expectedPowerStatus(action, current string) string {
	switch action {
	case "on":
		return string(pkgmodel.InstanceStatusRunning)
	case "off", "hard_off":
		return string(pkgmodel.InstanceStatusStopped)
	default:
		return ""
	}
}

// logUpdateStatusFailure 记录状态落库失败（无 logger 时静默：不影响主流程）。
func (s *instanceService) logUpdateStatusFailure(it *model.Instance, status string, err error) {
	if s.logger == nil {
		return
	}
	s.logger.Warn("更新实例状态失败（上游操作已完成，本地状态将短暂偏旧）",
		zap.Uint64("instance_id", it.ID), zap.String("instance_mark", it.InstanceID),
		zap.String("target_status", status), zap.Error(err))
}

// VNC 获取远程控制台地址。
func (s *instanceService) VNC(ctx context.Context, userID, id uint64) (*dto.VNCResult, error) {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if it.ProviderInstanceID == "" {
		return nil, ErrInstanceNotProvisioned
	}
	provider, err := s.buildControl(ctx, it.ProviderID)
	if err != nil {
		return nil, err
	}
	res, err := provider.VNC(ctx, it.InstanceID)
	if err != nil {
		return nil, err
	}
	return &dto.VNCResult{URL: res.URL, External: res.External}, nil
}

// buildProvider 解析提供商并实例化上游适配器。
func (s *instanceService) buildProvider(ctx context.Context, providerID uint64) (upstream.Provider, error) {
	if s.resolve == nil {
		return nil, errProviderUnavailable
	}
	provider, err := s.resolve(ctx, providerID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("解析服务渠道失败", zap.Uint64("provider_id", providerID), zap.Error(err))
		}
		return nil, errProviderUnavailable
	}
	if provider == nil {
		return nil, errProviderUnavailable
	}
	return provider, nil
}

// buildControl 解析提供商并断言为「电源/查询」能力接口；失败返回明确错误。
func (s *instanceService) buildControl(ctx context.Context, providerID uint64) (upstream.InstanceControl, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	control, ok := provider.(upstream.InstanceControl)
	if !ok {
		return nil, errUnsupportedControl
	}
	return control, nil
}

// refresh 调用上游 GetInstance 刷新运行状态；失败返回 ok=false（降级为本地状态）。
func (s *instanceService) refresh(ctx context.Context, it *model.Instance) (*pkgmodel.StandardInstance, bool) {
	provider, err := s.buildControl(ctx, it.ProviderID)
	if err != nil {
		return nil, false
	}
	inst, err := provider.GetInstance(ctx, it.InstanceID)
	if err != nil {
		return nil, false
	}
	return inst, true
}

// toInfo 将本地主机映射为用户可见信息。
func toInfo(it *model.Instance) dto.InstanceInfo {
	expireState, daysLeft := expireStateOf(it.ExpireAt)
	info := dto.InstanceInfo{
		ID:          it.ID,
		InstanceID:  it.InstanceID,
		Name:        it.Name,
		Status:      it.Status,
		PowerStatus: powerOf(statusToModel(it.Status)),
		OS:          it.OS,
		CPU:         it.CPU,
		Memory:      it.Memory,
		Disk:        it.Disk,
		DiskType:    it.DiskType,
		Bandwidth:   it.Bandwidth,
		Region:      it.Region,
		Zone:        it.Zone,
		PublicIP:    it.PublicIP,
		PrivateIP:   it.PrivateIP,
		BillingMode: it.BillingMode,
		ActorID:     it.ActorUserID,
		ActorName:   it.ActorName,
		// 待开通占位（provider_instance_id 为空）：前端显示「待开通」且只开放重新开通。
		// 注意：Pending 由内部字段推导，不下发上游实例号（DTO 里已无该字段）。
		OrderID:        it.OrderID,
		Pending:        it.ProviderInstanceID == "",
		ExpireAt:       fmtTime(it.ExpireAt),
		LifecycleStage: it.LifecycleStage,
		ExpireState:    expireState,
		DaysLeft:       daysLeft,
		CreatedAt:      it.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      it.UpdatedAt.Format(time.RFC3339),
		Transition:     transitionInfoOf(it),
	}
	// 两个正交维度分别下发（与运维台同口径）：电源维度回答"机器通电了吗"，
	// 服务维度回答"账期还有效吗"。客户端据它们展示两个独立标签，
	// 不再需要从 status 反推 —— 那是"关机显示成已暂停"的根源。
	info.PowerState = powerStateOf(it)
	info.ServiceState = serviceStateOf(it, expireState, daysLeft)
	// 列表只标记机型、不拉转发地址：地址要逐台请求上游，列表页几十台会把响应拖垮，
	// 且列表里展示首条地址的价值不大（详情页才需要复制）。只给 is_nat 让前端
	// 区分"未分配 IP（NAT 机型）"与"未分配 IP（异常）"。
	if upstream.IsNatInstance(it.RawData) {
		info.NatAccess = &dto.NatAccessInfo{IsNat: true}
	}
	return info
}

// applyLiveStatus 把回源结果落到下发结构上，并处理过渡态收尾。
//
// 与运维台的 refreshUpstreamFromTransition 同口径，两件事必须一起做：
//
//  1. **不报电源状态的渠道不得改写本地状态**（实测：魔方财务只返回 domainstatus
//     "服务是否激活"，没有机器电源字段）。照抄会把客户刚关的机显示回"运行中"，
//     客户以为关机没生效（用户反馈"状态混乱"的根因之一）。
//  2. **过渡态收尾**：上游确实给出新状态、且已过最短占用窗口时才清。窗口内保留，
//     让"重启中"的动画与操作禁用持续到动作确实跑起来为止 —— 否则客户等几秒
//     看动画停了再点一次，又堆一条重启。
func (s *instanceService) applyLiveStatus(ctx context.Context, it *model.Instance, live *pkgmodel.StandardInstance, info *dto.InstanceInfo) {
	if it == nil || live == nil || info == nil {
		return
	}
	reportsPower := true
	if provider, err := s.buildProvider(ctx, it.ProviderID); err == nil {
		reportsPower = upstream.SupportsPowerStateReporting(provider)
	}
	if reportsPower {
		info.Status = string(live.Status)
		info.PowerStatus = powerOf(live.Status)
	} else {
		// 保留本地现值（那是我们下发指令后唯一可靠的知识）。
		info.Status = it.Status
		info.PowerStatus = powerOf(statusToModel(it.Status))
	}

	if strings.TrimSpace(it.TransitionState) == "" {
		return
	}
	claimedAt := time.Time{}
	if it.TransitionStartedAt != nil {
		claimedAt = *it.TransitionStartedAt
	}
	// 与运维台一致：报电源状态的渠道要"上游确实报出新状态"才算完成；
	// 不报的渠道指令已受理即算完成（无从核对）。两者都受最短窗口约束。
	confirmed := reportsPower || it.TransitionUntil != nil
	if confirmed && !transitionWithinMinHold(it.TransitionState, claimedAt) {
		if err := s.repo.ClearTransitionIfOwner(ctx, it.ID, it.TransitionState); err != nil {
			if s.logger != nil {
				s.logger.Warn("清除过渡态失败", zap.Uint64("instance_id", it.ID), zap.Error(err))
			}
			return
		}
		it.TransitionState = ""
		it.TransitionStartedAt = nil
		it.TransitionUntil = nil
		info.Transition = nil
	}
}

// transitionInfoOf 组装过渡态下发结构；无过渡态返回 nil（客户侧副本）。
//
// 超时（Overdue）不是失败：机器可能还在执行，只是回执丢了。前端据此把
// "转圈中"换成"结果未确认，请刷新"，并允许重试 —— 但与运维台一致，
// 这个"允许"由服务端 ensureNoActiveTransition 的超时放行兜底。
func transitionInfoOf(it *model.Instance) *dto.TransitionInfo {
	if it == nil || strings.TrimSpace(it.TransitionState) == "" {
		return nil
	}
	spec := pkgmodel.InstanceTransitionOf(it.TransitionState)
	info := &dto.TransitionInfo{
		State: spec.Value,
		Label: spec.Label,
		Verb:  spec.Verb,
	}
	if it.TransitionStartedAt != nil {
		info.StartedAt = it.TransitionStartedAt.Format(time.RFC3339)
	}
	if it.TransitionUntil != nil {
		info.ExpiresAt = it.TransitionUntil.Format(time.RFC3339)
		info.Overdue = time.Now().After(*it.TransitionUntil)
	}
	return info
}

// expireStateOf 计算到期状态与剩余天数（负数表示已过期天数）。
//
// 与「服务状态」正交：一台 status=running 的机器可以同时是 expired（还没到处置
// 时点），前端要把两者都展示出来才能解释"为什么它还在运行但提示已过期"。
func expireStateOf(expireAt *time.Time) (string, int) {
	if expireAt == nil {
		return expireStateNone, 0
	}
	now := time.Now()
	daysLeft := int(expireAt.Sub(now).Hours() / 24)
	if expireAt.Before(now) {
		return expireStateExpired, daysLeft
	}
	if daysLeft <= expireWithinDays {
		return expireStateExpiring, daysLeft
	}
	return expireStateNormal, daysLeft
}

// powerStateOf 计算**机器电源维度**（与账期无关，客户侧副本）。
//
// 判据与运维台 powerStateOf 一致：
//   - 有进行中的电源类动作 → transitioning（界面显示"重启中"等）
//   - running → on；stopped → off；其余（创建中/删除中）→ unknown
//
// 刻意不看 lifecycle_stage：到期暂停是"服务被停"，不是"机器被关"，
// 两者在客户端必须能分开说（这正是客户反馈的"状态显示混乱"）。
func powerStateOf(it *model.Instance) string {
	if it == nil {
		return "unknown"
	}
	if s := strings.TrimSpace(it.TransitionState); s != "" {
		if spec := pkgmodel.InstanceTransitionOf(s); spec.PowerHint != "" {
			// 窗口已过的过渡态不再算"进行中"：结果未知，回落到真实状态判断。
			if it.TransitionUntil == nil || time.Now().Before(*it.TransitionUntil) {
				return "transitioning"
			}
		}
	}
	switch it.Status {
	case string(pkgmodel.InstanceStatusRunning):
		return "on"
	case string(pkgmodel.InstanceStatusStopped):
		return "off"
	default:
		return "unknown"
	}
}

// serviceStateOf 计算**账期/服务维度**（与电源无关，客户侧副本）。
//
// 优先级：终态 > 我方暂停 > 到期时间派生。判据刻意不看 status ——
// 一台关机但未到期的实例，服务维度仍是 active（服务在有效期内，只是机器没开）。
func serviceStateOf(it *model.Instance, expireState string, daysLeft int) string {
	if it == nil {
		return "active"
	}
	switch it.LifecycleStage {
	case "destroyed":
		return "destroyed"
	case "suspended":
		return "suspended"
	}
	switch expireState {
	case expireStateExpired:
		if it.LifecycleStage == "grace" {
			return "grace"
		}
		return "expired"
	case expireStateExpiring:
		return "expiring"
	}
	_ = daysLeft
	return "active"
}

// statusToModel 将实例表状态映射为上游状态枚举（辅助电源展示）。
func statusToModel(status string) pkgmodel.StandardInstanceStatus {
	switch status {
	case "running":
		return pkgmodel.InstanceStatusRunning
	case "stopped":
		return pkgmodel.InstanceStatusStopped
	default:
		return pkgmodel.InstanceStatusCreating
	}
}

// powerOf 服务状态 → 电源状态（on/off/unknown）。
func powerOf(status pkgmodel.StandardInstanceStatus) string {
	switch status {
	case pkgmodel.InstanceStatusRunning:
		return "on"
	case pkgmodel.InstanceStatusStopped:
		return "off"
	default:
		return "unknown"
	}
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
