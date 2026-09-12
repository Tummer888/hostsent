// Package service 提供资源同步模块的业务编排。
package service

import (
	"context"
	"time"

	"hostsent/backend/internal/modules/admin/resource/sync/dto"
	"hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/modules/admin/resource/sync/repository"
)

// SyncService 资源同步业务能力
type SyncService interface {
	ListTasks(ctx context.Context, query dto.SyncTaskListQuery) (*dto.SyncTaskListResponse, error)
	FindTask(ctx context.Context, id uint64) (*dto.SyncTaskInfo, error)
	CreateTask(ctx context.Context, req dto.CreateSyncTaskRequest) (*dto.SyncTaskInfo, error)
	ListLogs(ctx context.Context, query dto.SyncLogListQuery) (*dto.SyncLogListResponse, error)
	ListInstances(ctx context.Context, query dto.InstanceListQuery) (*dto.InstanceListResponse, error)
	FindInstance(ctx context.Context, id uint64) (*dto.InstanceInfo, error)
}

type syncService struct {
	repo   repository.SyncRepository
	engine *SyncEngine
}

// NewSyncService 创建资源同步业务服务
func NewSyncService(repo repository.SyncRepository, engine *SyncEngine) SyncService {
	return &syncService{repo: repo, engine: engine}
}

func (s *syncService) ListTasks(ctx context.Context, query dto.SyncTaskListQuery) (*dto.SyncTaskListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.repo.ListTasks(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.SyncTaskInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildTaskInfo(item))
	}
	return &dto.SyncTaskListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *syncService) FindTask(ctx context.Context, id uint64) (*dto.SyncTaskInfo, error) {
	item, err := s.repo.FindTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildTaskInfo(*item)
	return &info, nil
}

func (s *syncService) CreateTask(ctx context.Context, req dto.CreateSyncTaskRequest) (*dto.SyncTaskInfo, error) {
	item, err := s.engine.StartSync(ctx, req.ProviderID, req.TaskType)
	if err != nil {
		return nil, err
	}
	info := buildTaskInfo(*item)
	return &info, nil
}

func (s *syncService) ListLogs(ctx context.Context, query dto.SyncLogListQuery) (*dto.SyncLogListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.repo.ListLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.SyncLogInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildLogInfo(item))
	}
	return &dto.SyncLogListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *syncService) ListInstances(ctx context.Context, query dto.InstanceListQuery) (*dto.InstanceListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.repo.ListInstances(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.InstanceInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildInstanceInfo(item))
	}
	return &dto.InstanceListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *syncService) FindInstance(ctx context.Context, id uint64) (*dto.InstanceInfo, error) {
	item, err := s.repo.FindInstanceByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildInstanceInfo(*item)
	return &info, nil
}

func buildTaskInfo(item model.SyncTask) dto.SyncTaskInfo {
	info := dto.SyncTaskInfo{
		ID:           item.ID,
		ProviderID:   item.ProviderID,
		TaskType:     item.TaskType,
		Status:       item.Status,
		TotalCount:   item.TotalCount,
		SuccessCount: item.SuccessCount,
		ErrorMessage: item.ErrorMessage,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}
	if item.StartedAt != nil {
		v := item.StartedAt.Format(time.RFC3339)
		info.StartedAt = &v
	}
	if item.CompletedAt != nil {
		v := item.CompletedAt.Format(time.RFC3339)
		info.CompletedAt = &v
	}
	return info
}

func buildLogInfo(item model.SyncLog) dto.SyncLogInfo {
	return dto.SyncLogInfo{
		ID:           item.ID,
		TaskID:       item.TaskID,
		ProviderID:   item.ProviderID,
		SyncType:     item.SyncType,
		Status:       item.Status,
		TotalCount:   item.TotalCount,
		SuccessCount: item.SuccessCount,
		ErrorMessage: item.ErrorMessage,
		Details:      item.Details,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}
}

func buildInstanceInfo(item model.Instance) dto.InstanceInfo {
	info := dto.InstanceInfo{
		ID:          item.ID,
		InstanceID:  item.InstanceID,
		ProviderID:  item.ProviderID,
		UserID:      item.UserID,
		SellProductID:    item.SellProductID,
		UpstreamProductID: item.UpstreamProductID,
		SourceMode:  item.SourceMode,
		Name:        item.Name,
		CPU:         item.CPU,
		Memory:      item.Memory,
		Disk:        item.Disk,
		DiskType:    item.DiskType,
		Bandwidth:   item.Bandwidth,
		OS:          item.OS,
		Region:      item.Region,
		Zone:        item.Zone,
		Status:      item.Status,
		PrivateIP:   item.PrivateIP,
		PublicIP:    item.PublicIP,
		BillingMode: item.BillingMode,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
	}
	if item.ExpireAt != nil {
		v := item.ExpireAt.Format(time.RFC3339)
		info.ExpireAt = &v
	}
	return info
}
