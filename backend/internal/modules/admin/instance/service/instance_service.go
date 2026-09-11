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
	SetRemark(ctx context.Context, id uint64, remark string) error
	Destroy(ctx context.Context, op Operator, id uint64, req *dto.DestroyRequest) error
	// Suspend / Unsuspend 暂停/恢复实例（T5.5 按能力分派）。
	Suspend(ctx context.Context, op Operator, id uint64, reason string) error
	Unsuspend(ctx context.Context, op Operator, id uint64) error
	// RecordStageAction 写入生命周期阶段推进审计（T5.4 推进器调用，系统操作人）。
	RecordStageAction(ctx context.Context, in StageActionInput) error
	Operations(ctx context.Context, id uint64, query *dto.OperationListQuery) (*dto.OperationListResponse, error)
	Related(ctx context.Context, id uint64) (*dto.RelatedInfo, error)
}

type instanceService struct {
	repo    repository.InstanceRepository
	opRepo  repository.OperationRepository
	related repository.RelatedRepository
	resolve ProviderResolver
	logger  *zap.Logger
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
	ctrl, err := s.control(ctx, row.ProviderID)
	if err != nil {
		s.record(ctx, row, op, logAction, map[string]any{"action": action}, before, before, err)
		return err
	}

	var opErr error
	switch normalized {
	case "on":
		opErr = ctrl.StartInstance(ctx, row.InstanceID)
	case "off":
		opErr = ctrl.StopInstance(ctx, row.InstanceID, false)
	case "hard_off":
		opErr = ctrl.StopInstance(ctx, row.InstanceID, true)
	case "reboot":
		opErr = ctrl.RestartInstance(ctx, row.InstanceID)
	case "hard_reboot":
		// 上游多为软重启；硬重启映射为先硬关再开（与用户中心一致）。
		if opErr = ctrl.StopInstance(ctx, row.InstanceID, true); opErr == nil {
			opErr = ctrl.StartInstance(ctx, row.InstanceID)
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

// SetRemark 写管理员内部备注（同步流程不会覆盖该字段）。
func (s *instanceService) SetRemark(ctx context.Context, id uint64, remark string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return s.wrapNotFound(err)
	}
	remark = strings.TrimSpace(remark)
	if len(remark) > 255 {
		remark = remark[:255]
	}
	return s.repo.UpdateRemark(ctx, id, remark)
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
	Err          error
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
	entry := &model.Operation{
		InstanceID:   in.InstanceID,
		InstanceMark: in.InstanceMark,
		UserID:       in.UserID,
		OperatorType: model.OperatorTypeSystem,
		Action:       action,
		BeforeStatus: in.FromStage,
		AfterStatus:  in.ToStage,
		Result:       model.ResultSuccess,
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

// Related 实例关联的订单/续费/工单。
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
		ProductID:          row.ProductID,
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
}

// capabilities 断言上游适配器能力，供前端置灰按钮。
func (s *instanceService) capabilities(ctx context.Context, providerID uint64) (dto.Capabilities, error) {
	provider, err := s.buildProvider(ctx, providerID)
	if err != nil {
		return dto.Capabilities{}, err
	}
	_, ctrl := provider.(upstream.InstanceControl)
	_, admin := provider.(upstream.InstanceAdministration)
	_, suspend := provider.(upstream.InstanceSuspension)
	_, terminate := provider.(upstream.InstanceTermination)
	// 暂停/销毁按能力分派（T5.5）：平台无对应接口时退化为电源/删除等价操作。
	return dto.Capabilities{
		Power:   ctrl,
		Console: ctrl,
		Resize:  admin,
		Destroy: admin || terminate,
		Suspend: suspend || ctrl,
		// 上游尚未提供重装系统能力（见 61 实施计划 §1.3），保留字段便于后续开启。
		Reinstall: false,
	}, nil
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
