package service

import (
	"context"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/open/dto"
	openrepo "hostsent/backend/internal/modules/open/repository"
)

// AuditDeps 对账服务依赖。
type AuditDeps struct {
	Repo openrepo.AuditRepository
}

// OpenAuditService 对账服务（T6.6）：/open/v1/audit/*。
type OpenAuditService struct {
	deps AuditDeps
}

// NewOpenAuditService 构造对账服务。
func NewOpenAuditService(deps AuditDeps) *OpenAuditService {
	return &OpenAuditService{deps: deps}
}

// ListRequests 幂等请求对账（含响应快照，供下游核对首次执行结果）。
func (s *OpenAuditService) ListRequests(ctx context.Context, appID uint64, page, pageSize int) (*dto.AuditListResponse, []dto.AuditRequestItem, error) {
	rows, total, err := s.deps.Repo.ListRequests(ctx, appID, page, pageSize)
	if err != nil {
		return nil, nil, err
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
	items := make([]dto.AuditRequestItem, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		item := dto.AuditRequestItem{
			ClientRequestID: r.ClientRequestID,
			Method:          r.Method,
			Path:            r.Path,
			Status:          r.Status,
			ResponseStatus:  r.ResponseStatus,
			CreatedAt:       formatTime(r.CreatedAt),
			UpdatedAt:       formatTime(r.UpdatedAt),
		}
		if r.ResponseBody != nil {
			item.ResponseBody = *r.ResponseBody
		}
		items = append(items, item)
	}
	return &dto.AuditListResponse{Total: total, Page: page, PageSize: pageSize}, items, nil
}

// ListOrders 代客下单订单对账。
func (s *OpenAuditService) ListOrders(ctx context.Context, appID uint64, page, pageSize int) (*dto.AuditListResponse, []dto.AuditOrderItem, error) {
	rows, total, err := s.deps.Repo.ListOrders(ctx, appID, page, pageSize)
	if err != nil {
		return nil, nil, err
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
	items := make([]dto.AuditOrderItem, 0, len(rows))
	for i := range rows {
		items = append(items, toAuditOrderItem(&rows[i]))
	}
	return &dto.AuditListResponse{Total: total, Page: page, PageSize: pageSize}, items, nil
}

func toAuditOrderItem(o *ordermodel.Order) dto.AuditOrderItem {
	return dto.AuditOrderItem{
		OrderNo:        o.OrderNo,
		ProductID:      o.ProductID,
		ProductName:    o.ProductName,
		Status:         o.Status,
		OriginalAmount: o.OriginalAmount,
		FinalAmount:    o.FinalAmount,
		PaidAmount:     o.PaidAmount,
		BillingCycle:   o.PriceModel,
		CustomerRef:    o.ChannelCustomerRef,
		CreatedAt:      formatTime(o.CreatedAt),
	}
}
