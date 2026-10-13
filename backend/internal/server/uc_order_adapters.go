// uc 订单模块的装配层适配器：把 admin 订单域仓储/投递器适配成 uc 订单服务的
// 本地行结构端口（CONTRIBUTING §3：uc service 不 import admin 域 model，
// 转换职责落在装配层）。

package server

import (
	"context"
	"errors"
	"time"

	admininstancedto "hostsent/backend/internal/modules/admin/instance/dto"
	admininstanceservice "hostsent/backend/internal/modules/admin/instance/service"
	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	adminmodel "hostsent/backend/internal/modules/admin/manager/model"
	adminrepo "hostsent/backend/internal/modules/admin/manager/repository"
	orderdto "hostsent/backend/internal/modules/admin/order/dto"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderrepo "hostsent/backend/internal/modules/admin/order/repository"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	ucinstancedto "hostsent/backend/internal/modules/uc/instance/dto"
	ucinstanceservice "hostsent/backend/internal/modules/uc/instance/service"
	membermodel "hostsent/backend/internal/modules/uc/member/model"
	memberrepo "hostsent/backend/internal/modules/uc/member/repository"
	ucorderdto "hostsent/backend/internal/modules/uc/order/dto"
	"hostsent/backend/internal/pkg/middleware"
)

// toAdminOrder 本地订单行 → admin 订单模型（投影出的新结构体，不共享引用）。
func toAdminOrder(row *ucorderdto.OrderRow) *ordermodel.Order {
	if row == nil {
		return nil
	}
	return &ordermodel.Order{
		ID:                 row.ID,
		OrderNo:            row.OrderNo,
		UserID:             row.UserID,
		ProductID:          row.ProductID,
		ProductName:        row.ProductName,
		Specs:              row.Specs,
		SpecCode:           row.SpecCode,
		Quantity:           row.Quantity,
		PriceModel:         row.PriceModel,
		Cycle:              row.Cycle,
		TotalAmount:        row.TotalAmount,
		PaidAmount:         row.PaidAmount,
		Status:             row.Status,
		PayMethod:          row.PayMethod,
		PayTime:            row.PayTime,
		ExpireTime:         row.ExpireTime,
		ConfigOptions:      row.ConfigOptions,
		OriginalAmount:     row.OriginalAmount,
		DiscountAmount:     row.DiscountAmount,
		FinalAmount:        row.FinalAmount,
		PricePolicyID:      row.PricePolicyID,
		DiscountSource:     row.DiscountSource,
		PriceSnapshot:      row.PriceSnapshot,
		OperatorID:         row.OperatorID,
		CustomerRemark:     row.CustomerRemark,
		Channel:            row.Channel,
		OpenAppID:          row.OpenAppID,
		ChannelCustomerRef: row.ChannelCustomerRef,
		SalesAdminID:       row.SalesAdminID,
		RenewalID:          row.RenewalID,
		RenewalPrice:       row.RenewalPrice,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

// fromAdminOrder 管理端订单模型 → 本地订单行。
func fromAdminOrder(o *ordermodel.Order) *ucorderdto.OrderRow {
	if o == nil {
		return nil
	}
	return &ucorderdto.OrderRow{
		ID:                 o.ID,
		OrderNo:            o.OrderNo,
		UserID:             o.UserID,
		ProductID:          o.ProductID,
		ProductName:        o.ProductName,
		Specs:              o.Specs,
		SpecCode:           o.SpecCode,
		Quantity:           o.Quantity,
		PriceModel:         o.PriceModel,
		Cycle:              o.Cycle,
		TotalAmount:        o.TotalAmount,
		PaidAmount:         o.PaidAmount,
		Status:             o.Status,
		PayMethod:          o.PayMethod,
		PayTime:            o.PayTime,
		ExpireTime:         o.ExpireTime,
		ConfigOptions:      o.ConfigOptions,
		OriginalAmount:     o.OriginalAmount,
		DiscountAmount:     o.DiscountAmount,
		FinalAmount:        o.FinalAmount,
		PricePolicyID:      o.PricePolicyID,
		DiscountSource:     o.DiscountSource,
		PriceSnapshot:      o.PriceSnapshot,
		OperatorID:         o.OperatorID,
		CustomerRemark:     o.CustomerRemark,
		Channel:            o.Channel,
		OpenAppID:          o.OpenAppID,
		ChannelCustomerRef: o.ChannelCustomerRef,
		SalesAdminID:       o.SalesAdminID,
		RenewalID:          o.RenewalID,
		RenewalPrice:       o.RenewalPrice,
		CreatedAt:          o.CreatedAt,
		UpdatedAt:          o.UpdatedAt,
	}
}

// ucOrderRepoAdapter 适配 admin 订单仓储为 uc 订单仓储端口（本地行结构进出）。
type ucOrderRepoAdapter struct{ repo orderrepo.OrderRepository }

// Create 写订单；成功后把 GORM 回填的自增 ID 写回本地行（uc 下单立即用 order.ID）。
func (a ucOrderRepoAdapter) Create(ctx context.Context, row *ucorderdto.OrderRow) error {
	o := toAdminOrder(row)
	if err := a.repo.Create(ctx, o); err != nil {
		return err
	}
	row.ID = o.ID
	return nil
}

func (a ucOrderRepoAdapter) List(ctx context.Context, query orderdto.OrderListQuery) ([]ucorderdto.OrderRow, int64, error) {
	items, total, err := a.repo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]ucorderdto.OrderRow, 0, len(items))
	for i := range items {
		rows = append(rows, *fromAdminOrder(&items[i]))
	}
	return rows, total, nil
}

func (a ucOrderRepoAdapter) FindByID(ctx context.Context, id uint64) (*ucorderdto.OrderRow, error) {
	o, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return fromAdminOrder(o), nil
}

func (a ucOrderRepoAdapter) TransitionStatus(ctx context.Context, id uint64, from, to string) (int64, error) {
	return a.repo.TransitionStatus(ctx, id, from, to)
}

func (a ucOrderRepoAdapter) ListExpiredPending(ctx context.Context, before time.Time, limit int) ([]ucorderdto.OrderRow, error) {
	items, err := a.repo.ListExpiredPending(ctx, before, limit)
	if err != nil {
		return nil, err
	}
	rows := make([]ucorderdto.OrderRow, 0, len(items))
	for i := range items {
		rows = append(rows, *fromAdminOrder(&items[i]))
	}
	return rows, nil
}

// ucOrderItemAdapter 适配 admin 订单项仓储为 uc 订单项端口。
type ucOrderItemAdapter struct{ repo orderrepo.OrderItemRepository }

func (a ucOrderItemAdapter) Create(ctx context.Context, item *ucorderdto.OrderItemRow) error {
	return a.repo.Create(ctx, &ordermodel.OrderItem{
		OrderID:        item.OrderID,
		ProductID:      item.ProductID,
		ProductName:    item.ProductName,
		SpecCode:       item.SpecCode,
		Specs:          item.Specs,
		ConfigOptions:  item.ConfigOptions,
		Cycle:          item.Cycle,
		Price:          item.Price,
		Quantity:       item.Quantity,
		Amount:         item.Amount,
		OriginalAmount: item.OriginalAmount,
		DiscountAmount: item.DiscountAmount,
		FinalAmount:    item.FinalAmount,
	})
}

// ucProvisionEnqueueAdapter 适配开通任务投递器为 uc 端口（本地行结构）。
type ucProvisionEnqueueAdapter struct {
	enqueuer *orderservice.ProvisionEnqueuer
}

func (a ucProvisionEnqueueAdapter) EnqueueProvision(ctx context.Context, row *ucorderdto.OrderRow, sourceMode string) error {
	return a.enqueuer.EnqueueProvision(ctx, toAdminOrder(row), sourceMode)
}

// ucProvisionTaskAdapter 适配开通任务查询仓储为 uc 端口（本地行结构）。
type ucProvisionTaskAdapter struct {
	repo orderrepo.ProvisionTaskRepository
}

func (a ucProvisionTaskAdapter) FindByOrderIDs(ctx context.Context, orderIDs []uint64) (map[uint64]*ucorderdto.ProvisionTaskRow, error) {
	tasks, err := a.repo.FindByOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]*ucorderdto.ProvisionTaskRow, len(tasks))
	for id, t := range tasks {
		out[id] = &ucorderdto.ProvisionTaskRow{OrderID: t.OrderID, Status: t.Status, LastError: t.LastError}
	}
	return out, nil
}

// adminAuditWriterAdapter 把 pkg/middleware 的审计条目翻译成管理端审计模型落库。
// pkg 禁止 import modules，因此模型依赖收敛在装配层（CONTRIBUTING §3）。
type adminAuditWriterAdapter struct {
	repo adminrepo.AdminAuditRepository
}

func (a adminAuditWriterAdapter) CreateAuditLog(ctx context.Context, entry *middleware.AdminAuditEntry) error {
	if entry == nil {
		return nil
	}
	return a.repo.CreateAuditLog(ctx, &adminmodel.AdminAuditLog{
		AdminID:       entry.AdminID,
		AdminName:     entry.AdminName,
		Action:        entry.Action,
		ResourceType:  entry.ResourceType,
		ResourceID:    entry.ResourceID,
		Detail:        entry.Detail,
		IP:            entry.IP,
		UserAgent:     entry.UserAgent,
		Module:        entry.Module,
		RequestMethod: entry.RequestMethod,
		RequestPath:   entry.RequestPath,
		ResponseCode:  entry.ResponseCode,
		TraceID:       entry.TraceID,
	})
}

// userOperationLogWriterAdapter 把 pkg/middleware 的用户操作日志条目翻译成
// uc/member 的操作日志模型落库。
type userOperationLogWriterAdapter struct{ repo memberrepo.MemberRepository }

func (a userOperationLogWriterAdapter) CreateOperationLog(ctx context.Context, entry *middleware.UserOperationLogEntry) error {
	if entry == nil {
		return nil
	}
	return a.repo.CreateOperationLog(ctx, &membermodel.OperationLog{
		ActorUserID:   entry.ActorUserID,
		ActorName:     entry.ActorName,
		AccountUserID: entry.AccountUserID,
		Module:        entry.Module,
		Action:        entry.Action,
		Target:        entry.Target,
		Detail:        entry.Detail,
		IP:            entry.IP,
	})
}

// ucInstanceOperatorAdapter 把 admin 实例运维台适配为 uc/instance 的维护端口。
//
// uc 侧端口使用本地 Operator 与本地 dto（CONTRIBUTING §3：uc service 不 import
// admin 域 service），字段同构的转换职责落在装配层。
type ucInstanceOperatorAdapter struct {
	ops instanceservice.InstanceService
}

func (a ucInstanceOperatorAdapter) Destroy(ctx context.Context, op ucinstanceservice.Operator, id uint64, req *ucinstancedto.DestroyRequest) error {
	return a.ops.Destroy(ctx, admininstanceservice.Operator{Type: op.Type, ID: op.ID, Name: op.Name}, id,
		&admininstancedto.DestroyRequest{ConfirmMark: req.ConfirmMark, Reason: req.Reason})
}

func (a ucInstanceOperatorAdapter) Reinstall(ctx context.Context, op ucinstanceservice.Operator, id uint64, req *ucinstancedto.ReinstallRequest) (*ucinstancedto.ReinstallResult, error) {
	res, err := a.ops.Reinstall(ctx, admininstanceservice.Operator{Type: op.Type, ID: op.ID, Name: op.Name}, id,
		&admininstancedto.ReinstallRequest{
			OS: req.OS, Port: req.Port, FormatDataDisk: req.FormatDataDisk,
			SystemDiskSize: req.SystemDiskSize, Reason: req.Reason,
		})
	if err != nil {
		if errors.Is(err, admininstanceservice.ErrCapabilityUnsupported) {
			return nil, ucinstanceservice.ErrCapabilityUnsupported
		}
		return nil, err
	}
	return &ucinstancedto.ReinstallResult{Username: res.Username, Password: res.Password}, nil
}

func (a ucInstanceOperatorAdapter) ResetPassword(ctx context.Context, op ucinstanceservice.Operator, id uint64, req *ucinstancedto.ResetPasswordRequest) error {
	err := a.ops.ResetPassword(ctx, admininstanceservice.Operator{Type: op.Type, ID: op.ID, Name: op.Name}, id,
		&admininstancedto.ResetPasswordRequest{Password: req.Password})
	if errors.Is(err, admininstanceservice.ErrCapabilityUnsupported) {
		return ucinstanceservice.ErrCapabilityUnsupported
	}
	return err
}

func (a ucInstanceOperatorAdapter) Rescue(ctx context.Context, op ucinstanceservice.Operator, id uint64, req *ucinstancedto.RescueRequest) error {
	err := a.ops.Rescue(ctx, admininstanceservice.Operator{Type: op.Type, ID: op.ID, Name: op.Name}, id,
		&admininstancedto.RescueRequest{System: req.System, TempPassword: req.TempPassword})
	if errors.Is(err, admininstanceservice.ErrCapabilityUnsupported) {
		return ucinstanceservice.ErrCapabilityUnsupported
	}
	return err
}

func (a ucInstanceOperatorAdapter) ExitRescue(ctx context.Context, op ucinstanceservice.Operator, id uint64) error {
	err := a.ops.ExitRescue(ctx, admininstanceservice.Operator{Type: op.Type, ID: op.ID, Name: op.Name}, id)
	if errors.Is(err, admininstanceservice.ErrCapabilityUnsupported) {
		return ucinstanceservice.ErrCapabilityUnsupported
	}
	return err
}

func (a ucInstanceOperatorAdapter) Snapshots(ctx context.Context, id uint64, snapshotType string) ([]ucinstancedto.SnapshotInfo, error) {
	items, err := a.ops.Snapshots(ctx, id, snapshotType)
	if err != nil {
		return nil, err
	}
	out := make([]ucinstancedto.SnapshotInfo, 0, len(items))
	for _, it := range items {
		out = append(out, ucinstancedto.SnapshotInfo{
			ID: it.ID, Name: it.Name, Type: it.Type, Size: it.Size, Status: it.Status,
			DiskID: it.DiskID, DiskName: it.DiskName, CreateTime: it.CreateTime, Remarks: it.Remarks,
		})
	}
	return out, nil
}

func (a ucInstanceOperatorAdapter) CreateSnapshot(ctx context.Context, op ucinstanceservice.Operator, id uint64, req *ucinstancedto.SnapshotCreateRequest) error {
	err := a.ops.CreateSnapshot(ctx, admininstanceservice.Operator{Type: op.Type, ID: op.ID, Name: op.Name}, id,
		&admininstancedto.SnapshotCreateRequest{Type: req.Type, Name: req.Name, DiskID: req.DiskID})
	if errors.Is(err, admininstanceservice.ErrCapabilityUnsupported) {
		return ucinstanceservice.ErrCapabilityUnsupported
	}
	return err
}

func (a ucInstanceOperatorAdapter) DeleteSnapshot(ctx context.Context, op ucinstanceservice.Operator, id uint64, snapshotID string) error {
	err := a.ops.DeleteSnapshot(ctx, admininstanceservice.Operator{Type: op.Type, ID: op.ID, Name: op.Name}, id, snapshotID)
	if errors.Is(err, admininstanceservice.ErrCapabilityUnsupported) {
		return ucinstanceservice.ErrCapabilityUnsupported
	}
	return err
}

func (a ucInstanceOperatorAdapter) RestoreSnapshot(ctx context.Context, op ucinstanceservice.Operator, id uint64, req *ucinstancedto.SnapshotRestoreRequest) error {
	err := a.ops.RestoreSnapshot(ctx, admininstanceservice.Operator{Type: op.Type, ID: op.ID, Name: op.Name}, id,
		&admininstancedto.SnapshotRestoreRequest{SnapshotID: req.SnapshotID, ConfirmMark: req.ConfirmMark})
	if errors.Is(err, admininstanceservice.ErrCapabilityUnsupported) {
		return ucinstanceservice.ErrCapabilityUnsupported
	}
	return err
}
