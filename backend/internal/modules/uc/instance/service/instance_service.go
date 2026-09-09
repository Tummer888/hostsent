// Package service 提供用户中心主机管理业务编排：列表/详情/电源操作/VNC。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hostsent/backend/internal/modules/uc/instance/dto"
	"hostsent/backend/internal/modules/uc/instance/model"
	"hostsent/backend/internal/modules/uc/instance/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// ProviderResolver 按提供商 ID 解析上游适配器（装配层注入，避免依赖 provider 服务）。
type ProviderResolver func(ctx context.Context, providerID uint64) (upstream.Provider, error)

// InstanceService 用户中心主机业务能力。
type InstanceService interface {
	List(ctx context.Context, userID uint64, query dto.ListQuery) (*dto.ListResponse, error)
	Detail(ctx context.Context, userID, id uint64, live bool) (*dto.InstanceInfo, error)
	Power(ctx context.Context, userID, id uint64, action string) error
	VNC(ctx context.Context, userID, id uint64) (*dto.VNCResult, error)
}

type instanceService struct {
	repo      repository.InstanceRepository
	resolve   ProviderResolver
}

// NewInstanceService 创建用户中心主机服务。
func NewInstanceService(repo repository.InstanceRepository, resolve ProviderResolver) InstanceService {
	return &instanceService{repo: repo, resolve: resolve}
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
func (s *instanceService) Power(ctx context.Context, userID, id uint64, action string) error {
	it, err := s.repo.FindByUser(ctx, userID, id)
	if err != nil {
		return err
	}
	provider, err := s.buildProvider(ctx, it.ProviderID)
	if err != nil {
		return err
	}
	switch strings.ToLower(action) {
	case "on":
		return provider.StartInstance(ctx, it.InstanceID)
	case "off":
		return provider.StopInstance(ctx, it.InstanceID, false)
	case "hard_off":
		return provider.StopInstance(ctx, it.InstanceID, true)
	case "reboot":
		return provider.RestartInstance(ctx, it.InstanceID)
	case "hard_reboot":
		// 上游多为软重启；硬重启映射为先硬关再开。
		if err := provider.StopInstance(ctx, it.InstanceID, true); err != nil {
			return err
		}
		return provider.StartInstance(ctx, it.InstanceID)
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
	provider, err := s.buildProvider(ctx, it.ProviderID)
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

// refresh 调用上游 GetInstance 刷新运行状态；失败返回 ok=false（降级为本地状态）。
func (s *instanceService) refresh(ctx context.Context, it *model.Instance) (*pkgmodel.StandardInstance, bool) {
	provider, err := s.resolve(ctx, it.ProviderID)
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
