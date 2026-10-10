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
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// defaultExpireWithinDays 未指定时的临期判定天数。
const defaultExpireWithinDays = 7

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
	Detail(ctx context.Context, id uint64, live bool) (*dto.DetailInfo, error)
	Power(ctx context.Context, op Operator, id uint64, action string) error
	VNC(ctx context.Context, op Operator, id uint64) (*dto.VNCResult, error)
	Sync(ctx context.Context, op Operator, id uint64) (*dto.DetailInfo, error)
	Resize(ctx context.Context, op Operator, id uint64, req *dto.ResizeRequest) error
	SetRemark(ctx context.Context, op Operator, id uint64, remark string) error
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
	// DisposeOnRefund 退款审核通过后的实例侧处置（doc61 §8 第 2 条，系统操作人）。
	// 是否动作由 lifecyclpolicy.refund_action 决定（默认 none：不动作）。
	DisposeOnRefund(ctx context.Context, in *dto.RefundInstanceRequest) error
	// SetRefundPolicyReader 注入退款处置策略读取（装配层调用，返回 none/suspend/destroy）。
	SetRefundPolicyReader(fn func(ctx context.Context) string)
	// RecordStageAction 写入生命周期阶段推进审计（T5.4 推进器调用，系统操作人）。
	RecordStageAction(ctx context.Context, in StageActionInput) error
	// SetEventListener 注入实例操作事件监听（P6/T6.5 开放平台回调；装配层调用，可为 nil）。
	SetEventListener(fn func(ctx context.Context, evt InstanceEvent))
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
	// refundPolicy 退款处置策略读取（装配层注入 lifecycle_policies.refund_action）；为 nil 视为 none。
	refundPolicy func(ctx context.Context) string
	// eventListener 实例操作/阶段事件监听（P6/T6.5 开放平台回调经装配层注入，可为 nil）。
	eventListener func(ctx context.Context, evt InstanceEvent)
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

// Detail 实例详情；live=true 时先尝试回源刷新（失败降级为本地数据）。
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
	if capErr != nil {
		detail.CapabilityError = capErr.Error()
	}
	return detail, nil
}

// Power 电源操作：开机/关机/硬关机/重启/硬重启。
func (s *instanceService) Power(ctx context.Context, op Operator, id uint64, action string) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}

	normalized, logAction, err := normalizeAction(action)
	if err != nil {
		return err
	}
	if err := ensurePowerAllowed(row.Status, normalized); err != nil {
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, row.Status, row.Status, err)
		return err
	}

	before := row.Status
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, before, before, err)
		return err
	}
	ctrl, ok := provider.(upstream.InstanceControl)
	if !ok {
		err := ErrCapabilityUnsupported
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
		// 上游状态有延迟，回源一次让列表与上游对齐；失败不影响本次操作结果。
		_ = s.refreshUpstream(ctx, row)
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
	sErr := s.refreshUpstream(ctx, row)
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
	if req == nil || strings.TrimSpace(req.OS) == "" {
		return nil, fmt.Errorf("%w: 请选择目标镜像", ErrInvalidAction)
	}
	if row.Status == string(pkgmodel.InstanceStatusDeleted) {
		return nil, fmt.Errorf("%w: 实例已销毁，无法重装", ErrStatusConflict)
	}
	provider, err := s.buildProvider(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionReinstall, nil, row.Status, row.Status, err)
		return nil, err
	}
	reinstaller, ok := provider.(upstream.InstanceReinstall)
	if !ok {
		err := fmt.Errorf("%w: %s", ErrCapabilityUnsupported, "该服务商不支持重装系统")
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
		s.record(ctx, row, op, model.ActionReinstall, params, before, before, rerr)
		return nil, rerr
	}
	out := &dto.ReinstallResult{}
	if res != nil {
		out.Username = res.Username
		out.Password = res.Password
		if res.Password != "" {
			params["credential_issued"] = true
		}
	}
	s.record(ctx, row, op, model.ActionReinstall, params, before, before, nil)
	return out, nil
}

// ResetPassword 重置实例登录密码（不重装，系统与数据保留）。
func (s *instanceService) ResetPassword(ctx context.Context, op Operator, id uint64, req *dto.ResetPasswordRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
	}
	if req == nil || strings.TrimSpace(req.Password) == "" {
		return fmt.Errorf("%w: 新密码不能为空", ErrInvalidAction)
	}
	if row.Status == string(pkgmodel.InstanceStatusDeleted) {
		return fmt.Errorf("%w: 实例已销毁，无法重置密码", ErrStatusConflict)
	}
	resetter, err := s.passwordResetter(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, model.ActionResetPassword, nil, row.Status, row.Status, err)
		return err
	}
	before := row.Status
	// 流水里只记"已重置"，绝不落密码明文。
	params := map[string]any{"password_changed": true}
	if perr := resetter.ResetInstancePassword(ctx, row.InstanceMark(), req.Password); perr != nil {
		s.record(ctx, row, op, model.ActionResetPassword, params, before, before, perr)
		return perr
	}
	s.record(ctx, row, op, model.ActionResetPassword, params, before, before, nil)
	return nil
}

// Rescue 进入救援系统（system 为平台口径的救援系统类型，tempPassword 为临时密码）。
func (s *instanceService) Rescue(ctx context.Context, op Operator, id uint64, req *dto.RescueRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
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

// Destroy 销毁实例（T5.5 按能力分派：优先 InstanceTermination，退化 InstanceAdministration.Delete）。
// req.ConfirmMark 必须等于实例标识或记录 ID，否则拒绝执行。
func (s *instanceService) Destroy(ctx context.Context, op Operator, id uint64, req *dto.DestroyRequest) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
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
			err = fmt.Errorf("%w: %s", ErrCapabilityUnsupported, err.Error())
		}
		s.record(ctx, row, op, model.ActionDestroy, params, before, before, err)
		return err
	}
	after := string(pkgmodel.InstanceStatusDeleted)
	_ = s.repo.UpdateStatus(ctx, row.ID, after)
	s.record(ctx, row, op, model.ActionDestroy, params, before, after, nil)
	return nil
}

// Suspend 暂停实例（T5.5）：优先平台暂停态，退化关机；缺能力显式报错。
func (s *instanceService) Suspend(ctx context.Context, op Operator, id uint64, reason string) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
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
	_ = s.repo.UpdateLifecycleStage(ctx, row.ID, "suspended")
	s.record(ctx, row, op, model.ActionSuspend, params, before, after, nil)
	return nil
}

// Unsuspend 恢复实例（T5.5）：优先平台解除暂停，退化开机；缺能力显式报错。
func (s *instanceService) Unsuspend(ctx context.Context, op Operator, id uint64) error {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.wrapNotFound(err)
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
	_ = s.repo.UpdateLifecycleStage(ctx, row.ID, "active")
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

// StageActionInput 生命周期阶段推进审计入参（T5.4 推进器 → 实例运维流水）。
//
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
		return fmt.Errorf("%w: %s", ErrCapabilityUnsupported, err.Error())
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
		LifecycleStage:     row.LifecycleStage,
		Name:               row.Name,
		CPU:                row.CPU,
		Memory:             row.Memory,
		Disk:               row.Disk,
		DiskType:           row.DiskType,
		Bandwidth:          row.Bandwidth,
		OS:                 row.OS,
		Region:             row.Region,
		Zone:               row.Zone,
		Status:             row.Status,
		PowerStatus:        powerOf(row.Status),
		PrivateIP:          row.PrivateIP,
		PublicIP:           row.PublicIP,
		BillingMode:        row.BillingMode,
		ActorUserID:        row.ActorUserID,
		ActorName:          row.ActorName,
		Remark:             row.Remark,
		ExpireAt:           fmtTimePtr(row.ExpireAt),
		DaysLeft:           daysLeft,
		ExpireState:        state,
		LastSyncedAt:       fmtTimePtr(row.LastSyncedAt),
		EnforceAttempts:    row.EnforceAttempts,
		EnforceNextAt:      fmtTimePtr(row.EnforceNextAt),
		LastEnforceError:   row.LastEnforceError,
		CreatedAt:          row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          row.UpdatedAt.Format(time.RFC3339),
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
	}, nil
}

// reinstaller 解析上游并断言重装能力。
func (s *instanceService) reinstaller(ctx context.Context, providerID uint64) (upstream.InstanceReinstall, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	r, ok := provider.(upstream.InstanceReinstall)
	if !ok {
		return nil, ErrCapabilityUnsupported
	}
	return r, nil
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
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
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
	ctrl, err := s.control(ctx, row.ProviderID)
	if err != nil {
		return err
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
	return s.repo.ApplySync(ctx, row.ID, string(inst.Status), inst.PublicIP, inst.PrivateIP, raw, time.Now())
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

// ensurePowerAllowed 状态守卫：创建中/销毁中的实例不允许电源操作。
func ensurePowerAllowed(status, normalized string) error {
	switch status {
	case string(pkgmodel.InstanceStatusCreating), string(pkgmodel.InstanceStatusDeleting):
		return fmt.Errorf("%w: 实例正在%s", ErrStatusConflict, status)
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
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
