package service

import (
	"context"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/repository"
)

// CallbackService 回调日志查询能力。
type CallbackService interface {
	List(ctx context.Context, q dto.CallbackListQuery) (*dto.CallbackListResponse, error)
}

type callbackService struct {
	repo repository.CallbackRepository
}

// NewCallbackService 创建回调日志服务。
func NewCallbackService(repo repository.CallbackRepository) CallbackService {
	return &callbackService{repo: repo}
}

func (s *callbackService) List(ctx context.Context, q dto.CallbackListQuery) (*dto.CallbackListResponse, error) {
	items, total, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.CallbackInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildCallbackInfo(item))
	}
	return &dto.CallbackListResponse{
		Items: resp,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}
