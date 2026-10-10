// Package service 提供用户中心主机管理业务编排：列表/详情/电源操作/VNC。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	admininstancedto "hostsent/backend/internal/modules/admin/instance/dto"
	admininstanceservice "hostsent/backend/internal/modules/admin/instance/service"
	"hostsent/backend/internal/modules/uc/instance/dto"
	"hostsent/backend/internal/modules/uc/instance/model"
	"hostsent/backend/internal/modules/uc/instance/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// 销毁相关错误（doc91 §6.4 验收断言 1/27）。
var (
	// ErrInstanceNotFoundOrDenied 实例不存在或不属于当前账号。
	// 两种情形共用一条提示，避免枚举他人实例 ID。
	ErrInstanceNotFoundOrDenied = errors.New("主机不存在或无权访问")
	// ErrDestroyUnsupported 销毁执行者未装配（部署裁剪时）。
	ErrDestroyUnsupported = errors.New("当前环境不支持自助销毁，请联系客服")
)

// ProviderResolver 按提供商 ID 解析上游适配器（装配层注入，避免依赖 provider 服务）。
type ProviderResolver func(ctx context.Context, providerID uint64) (upstream.Provider, error)

// Destructor 销毁执行者（装配层委派给管理端实例运维台，doc91 §6.4）。
//
// 接口定义在本包、实现是管理端的 instanceOpsService：uc/* 反向引用 admin/*
// 在本仓库已有先例（uc/finance、uc/payment、uc/order 均直引 admin 的 dto/model），
// 因此这里直接复用 admin 侧的 Operator 与 DestroyRequest，少一层结构体转换。
type Destructor interface {
	Destroy(ctx context.Context, op admininstanceservice.Operator, id uint64, req *admininstancedto.DestroyRequest) error
}

// InstanceOperator 实例维护执行者（装配层委派给管理端实例运维台）。
//
// 与 Destructor 分开声明是为了"装配裁剪"：只装配了 Destructor 的部署仍可销毁，
// 维护类操作则统一返回"当前环境不支持"，不需要在 UC 侧重写能力分派、流水与事件。
type InstanceOperator interface {
	Reinstall(ctx context.Context, op admininstanceservice.Operator, id uint64, req *admininstancedto.ReinstallRequest) (*admininstancedto.ReinstallResult, error)
	ResetPassword(ctx context.Context, op admininstanceservice.Operator, id uint64, req *admininstancedto.ResetPasswordRequest) error
	Rescue(ctx context.Context, op admininstanceservice.Operator, id uint64, req *admininstancedto.RescueRequest) error
	ExitRescue(ctx context.Context, op admininstanceservice.Operator, id uint64) error
	Snapshots(ctx context.Context, id uint64, snapshotType string) ([]admininstancedto.SnapshotInfo, error)
	CreateSnapshot(ctx context.Context, op admininstanceservice.Operator, id uint64, req *admininstancedto.SnapshotCreateRequest) error
	DeleteSnapshot(ctx context.Context, op admininstanceservice.Operator, id uint64, snapshotID string) error
	RestoreSnapshot(ctx context.Context, op admininstanceservice.Operator, id uint64, req *admininstancedto.SnapshotRestoreRequest) error
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
	// SetDestructor 注入销毁执行者（装配层调用；未注入时销毁返回明确错误）。
	SetDestructor(d Destructor)
	// SetOperator 注入维护执行者（装配层调用；未注入时维护类操作返回明确错误）。
	SetOperator(o InstanceOperator)
}

type instanceService struct {
	repo    repository.InstanceRepository
	resolve ProviderResolver
	// destructor 销毁执行者（管理端运维台），可为 nil。
	destructor Destructor
	// operator 维护类执行者（管理端运维台），可为 nil。
	operator InstanceOperator
}

// ErrMaintenanceUnsupported 维护执行者未装配（部署裁剪时）。
var ErrMaintenanceUnsupported = errors.New("当前环境不支持该操作，请联系客服")

// NewInstanceService 创建用户中心主机服务。
func NewInstanceService(repo repository.InstanceRepository, resolve ProviderResolver) InstanceService {
	return &instanceService{repo: repo, resolve: resolve}
}

// SetDestructor 注入销毁执行者。
func (s *instanceService) SetDestructor(d Destructor) { s.destructor = d }

// SetOperator 注入维护类执行者。
func (s *instanceService) SetOperator(o InstanceOperator) { s.operator = o }

// userOperator 组装委派给运维台的操作人（子账号记为真实操作人）。
func userOperator(userID, actorID uint64, actorName string) admininstanceservice.Operator {
	if actorID == 0 {
		actorID = userID
	}
	return admininstanceservice.Operator{Type: "user", ID: actorID, Name: actorName}
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
	res, err := s.operator.Reinstall(ctx, userOperator(userID, actorID, actorName), it.ID, &admininstancedto.ReinstallRequest{
		OS: req.OS, Port: req.Port, FormatDataDisk: req.FormatDataDisk,
		SystemDiskSize: req.SystemDiskSize, Reason: req.Reason,
	})
	if err != nil {
		// 归属正确但平台不支持时，把"不支持"翻译成用户能懂的话。
		if errors.Is(err, admininstanceservice.ErrCapabilityUnsupported) {
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
		&admininstancedto.ResetPasswordRequest{Password: req.Password})
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
		&admininstancedto.RescueRequest{System: req.System, TempPassword: req.TempPassword})
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
		&admininstancedto.SnapshotCreateRequest{Type: req.Type, Name: req.Name, DiskID: req.DiskID})
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
		&admininstancedto.SnapshotRestoreRequest{SnapshotID: req.SnapshotID, ConfirmMark: req.ConfirmMark})
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
	op := admininstanceservice.Operator{Type: "user", ID: actorID, Name: actorName}
	return s.destructor.Destroy(ctx, op, it.ID, &admininstancedto.DestroyRequest{
		ConfirmMark: req.ConfirmMark,
		Reason:      req.Reason,
	})
}

// List 我的主机列表。live=true 时逐个刷新上游运行状态。
func (s *instanceService) List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error) {
	items, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.InstanceInfo, 0, len(items))
	for _, it := range items {
		info := toInfo(&it)
		if query.Live && s.resolve != nil {
			if live, ok := s.refresh(ctx, &it); ok {
				info.Status = string(live.Status)
				info.PowerStatus = powerOf(live.Status)
			}
		} else {
			info.PowerStatus = powerOf(statusToModel(it.Status))
		}
		out = append(out, info)
	}
	return &dto.ListResponse{Items: out, Total: int64(len(out))}, nil
}

// Detail 我的主机详情。live=true 时刷新上游运行状态。
func (s *instanceService) Detail(ctx context.Context, userID, id uint64, live bool) (*dto.InstanceInfo, error) {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	info := toInfo(it)
	if live && s.resolve != nil {
		if liveInst, ok := s.refresh(ctx, it); ok {
			info.Status = string(liveInst.Status)
			info.PowerStatus = powerOf(liveInst.Status)
			if liveInst.PublicIP != "" {
				info.PublicIP = liveInst.PublicIP
			}
		}
	} else {
		info.PowerStatus = powerOf(statusToModel(it.Status))
	}
	return &info, nil
}

// Power 电源操作：开机/关机/重启/硬关机/硬重启。
// 与 UC 用户侧一致地优先使用平台的独立硬电源指令（客户端点「硬关机」不该只是"尽量硬"）。
func (s *instanceService) Power(ctx context.Context, userID, id uint64, action string) error {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return err
	}
	provider, err := s.buildProvider(ctx, it.ProviderID)
	if err != nil {
		return err
	}
	provider_ := provider
	control, ok := provider_.(upstream.InstanceControl)
	if !ok {
		return fmt.Errorf("上游不支持该操作（电源/控制台）")
	}
	hard, hasHard := provider_.(upstream.InstancePowerHard)
	switch strings.ToLower(action) {
	case "on":
		return control.StartInstance(ctx, it.InstanceID)
	case "off":
		return control.StopInstance(ctx, it.InstanceID, false)
	case "hard_off":
		if hasHard {
			return hard.HardStopInstance(ctx, it.InstanceID)
		}
		return control.StopInstance(ctx, it.InstanceID, true)
	case "reboot":
		return control.RestartInstance(ctx, it.InstanceID)
	case "hard_reboot":
		if hasHard {
			return hard.HardRestartInstance(ctx, it.InstanceID)
		}
		// 上游没有独立硬重启：退化为硬关再开。
		if err := control.StopInstance(ctx, it.InstanceID, true); err != nil {
			return err
		}
		return control.StartInstance(ctx, it.InstanceID)
	default:
		return fmt.Errorf("暂不支持的操作: %s", action)
	}
}

// VNC 获取远程控制台地址。
func (s *instanceService) VNC(ctx context.Context, userID, id uint64) (*dto.VNCResult, error) {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("上游提供商未配置")
	}
	provider, err := s.resolve(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("解析上游提供商失败: %w", err)
	}
	if provider == nil {
		return nil, fmt.Errorf("上游提供商不存在")
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
		return nil, fmt.Errorf("上游不支持该操作（电源/控制台）")
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
	return dto.InstanceInfo{
		ID:          it.ID,
		InstanceID:  it.InstanceID,
		ProviderID:  it.ProviderID,
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
		ExpireAt:    fmtTime(it.ExpireAt),
		CreatedAt:   it.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   it.UpdatedAt.Format(time.RFC3339),
	}
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
