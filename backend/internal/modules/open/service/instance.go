package service

import (
	"context"
	"errors"
	"time"

	apperrors "hostsent/backend/internal/pkg/errors"

	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	lifecycledto "hostsent/backend/internal/modules/admin/lifecycle/dto"
	lifecycleservice "hostsent/backend/internal/modules/admin/lifecycle/service"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/modules/open/dto"
	openrepo "hostsent/backend/internal/modules/open/repository"
)

// OpsController 实例电源/暂停控制（复用实例运维台，含运维审计与上游能力分派）。
type OpsController interface {
	Power(ctx context.Context, op instanceservice.Operator, id uint64, action string) error
	Suspend(ctx context.Context, op instanceservice.Operator, id uint64, reason string) error
	Unsuspend(ctx context.Context, op instanceservice.Operator, id uint64) error
}

// Renewals 续费能力（复用生命周期续费服务：双链路上游续费 + owner 组折扣 + owner 余额）。
type Renewals interface {
	UserRenew(ctx context.Context, userID, instanceID uint64, req *lifecycledto.UserRenewRequest) (*lifecycledto.RenewalCreatedResponse, error)
}

// InstanceDeps 开放实例服务依赖。
type InstanceDeps struct {
	Repo     openrepo.InstanceRepository
	Ops      OpsController
	Renewals Renewals
}

// OpenInstanceService 开放实例服务（T6.4）。
type OpenInstanceService struct {
	deps InstanceDeps
}

// NewOpenInstanceService 构造开放实例服务。
func NewOpenInstanceService(deps InstanceDeps) *OpenInstanceService {
	return &OpenInstanceService{deps: deps}
}

// openOperator 实例运维台操作人视图：以应用身份记录，审计可回溯到具体应用。
func openOperator(app *openrepo.ResolvedApp) instanceservice.Operator {
	return instanceservice.Operator{
		Type: "system",
		ID:   app.App.ID,
		Name: "open-app:" + app.App.AppID,
	}
}

// List 归属账号实例分页。
func (s *OpenInstanceService) List(ctx context.Context, ownerUserID uint64, status string, page, pageSize int) (*dto.InstanceListResponse, error) {
	items, total, err := s.deps.Repo.ListByUser(ctx, ownerUserID, status, page, pageSize)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	list := make([]dto.InstanceItem, 0, len(items))
	for i := range items {
		list = append(list, toInstanceItem(&items[i]))
	}
	return &dto.InstanceListResponse{Total: total, Page: page, PageSize: pageSize, Items: list}, nil
}

// Get 实例详情（校验归属）。
func (s *OpenInstanceService) Get(ctx context.Context, ownerUserID, id uint64) (*dto.InstanceItem, error) {
	row, err := s.deps.Repo.FindByIDForUser(ctx, id, ownerUserID)
	if err != nil {
		return nil, mapInstanceErr(err)
	}
	item := toInstanceItem(row)
	return &item, nil
}

// Renew 实例续费：归属校验后走生命周期续费（上游账期权威，T5.2）。
func (s *OpenInstanceService) Renew(ctx context.Context, app *openrepo.ResolvedApp, instanceID uint64, req dto.RenewInstanceRequest) (*lifecycledto.RenewalCreatedResponse, error) {
	if _, err := s.deps.Repo.FindByIDForUser(ctx, instanceID, app.App.OwnerUserID); err != nil {
		return nil, mapInstanceErr(err)
	}
	resp, err := s.deps.Renewals.UserRenew(ctx, app.App.OwnerUserID, instanceID, &lifecycledto.UserRenewRequest{
		PeriodCount: req.PeriodCount,
	})
	if err != nil {
		return nil, mapRenewErr(err)
	}
	return resp, nil
}

// Power 电源操作（on/off/hard_off/reboot）。
func (s *OpenInstanceService) Power(ctx context.Context, app *openrepo.ResolvedApp, instanceID uint64, action string) error {
	if _, err := s.deps.Repo.FindByIDForUser(ctx, instanceID, app.App.OwnerUserID); err != nil {
		return mapInstanceErr(err)
	}
	switch action {
	case "on", "off", "hard_off", "reboot":
	default:
		return apperrors.New(CodeOpenParam, "action 仅支持 on/off/hard_off/reboot")
	}
	return s.deps.Ops.Power(ctx, openOperator(app), instanceID, action)
}

// Suspend 暂停实例（D2：开放接口提供的最重处置，无销毁）。
func (s *OpenInstanceService) Suspend(ctx context.Context, app *openrepo.ResolvedApp, instanceID uint64, reason string) error {
	if _, err := s.deps.Repo.FindByIDForUser(ctx, instanceID, app.App.OwnerUserID); err != nil {
		return mapInstanceErr(err)
	}
	return s.deps.Ops.Suspend(ctx, openOperator(app), instanceID, reason)
}

// Unsuspend 恢复实例。
func (s *OpenInstanceService) Unsuspend(ctx context.Context, app *openrepo.ResolvedApp, instanceID uint64) error {
	if _, err := s.deps.Repo.FindByIDForUser(ctx, instanceID, app.App.OwnerUserID); err != nil {
		return mapInstanceErr(err)
	}
	return s.deps.Ops.Unsuspend(ctx, openOperator(app), instanceID)
}

// Destroy 明确不支持（D2）：销毁仅限我方后台人工操作。
func (s *OpenInstanceService) Destroy() error {
	return apperrors.New(CodeOpenUnsupported, "销毁不支持：实例销毁仅限平台侧人工操作")
}

func mapInstanceErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, openrepo.ErrInstanceNotFound) || errors.Is(err, lifecycleservice.ErrInstanceNotFound) {
		return apperrors.New(CodeOpenNotFound, "实例不存在")
	}
	if ae, ok := err.(*apperrors.AppError); ok {
		return ae
	}
	return err
}

// mapRenewErr 续费业务错误归一（余额不足/记录不存在）。
func mapRenewErr(err error) error {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*apperrors.AppError); ok {
		return ae
	}
	switch {
	case errors.Is(err, lifecycleservice.ErrInsufficientBalance):
		return apperrors.New(CodeOpenInsufficientBalance, "归属账户余额不足")
	case errors.Is(err, lifecycleservice.ErrInstanceNotFound), errors.Is(err, openrepo.ErrInstanceNotFound):
		return apperrors.New(CodeOpenNotFound, "实例不存在")
	default:
		return apperrors.New(CodeOpenInternal, err.Error())
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func toInstanceItem(row *syncmodel.Instance) dto.InstanceItem {
	item := dto.InstanceItem{
		ID:          row.ID,
		InstanceID:  row.InstanceID,
		Name:        row.Name,
		Status:      row.Status,
		SourceMode:  row.SourceMode,
		CPU:         row.CPU,
		Memory:      row.Memory,
		Disk:        row.Disk,
		DiskType:    row.DiskType,
		Bandwidth:   row.Bandwidth,
		OS:          row.OS,
		Region:      row.Region,
		Zone:        row.Zone,
		PublicIP:    row.PublicIP,
		PrivateIP:   row.PrivateIP,
		BillingMode: row.BillingMode,
		OrderID:     row.OrderID,
		CreatedAt:   formatTime(row.CreatedAt),
	}
	if row.ExpireAt != nil {
		item.ExpireAt = formatTime(*row.ExpireAt)
	}
	return item
}
