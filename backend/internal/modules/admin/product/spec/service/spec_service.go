package service

import (
	"context"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/model"
	"hostsent/backend/internal/modules/admin/product/spec/repository"
)

// SpecTemplateService 定义规格模板业务能力。
type SpecTemplateService interface {
	List(ctx context.Context, query dto.SpecTemplateQuery) (*dto.SpecTemplateListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.SpecTemplateInfo, error)
	Create(ctx context.Context, req dto.SpecTemplateRequest) (*dto.SpecTemplateInfo, error)
	Update(ctx context.Context, id uint64, req dto.SpecTemplateRequest) (*dto.SpecTemplateInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type specTemplateService struct {
	repo repository.SpecTemplateRepository
}

func NewSpecTemplateService(repo repository.SpecTemplateRepository) SpecTemplateService {
	return &specTemplateService{repo: repo}
}

func (s *specTemplateService) List(ctx context.Context, query dto.SpecTemplateQuery) (*dto.SpecTemplateListResponse, error) {
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.SpecTemplateInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildSpecTemplateInfo(item))
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	return &dto.SpecTemplateListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *specTemplateService) FindByID(ctx context.Context, id uint64) (*dto.SpecTemplateInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildSpecTemplateInfo(*item)
	return &info, nil
}

func (s *specTemplateService) Create(ctx context.Context, req dto.SpecTemplateRequest) (*dto.SpecTemplateInfo, error) {
	item := &model.SpecTemplate{
		Name:        strings.TrimSpace(req.Name),
		SpecFamily:  defaultString(req.SpecFamily, model.SpecFamilyGeneral),
		CPU:         req.CPU,
		Memory:      req.Memory,
		Disk:        req.Disk,
		DiskType:    defaultString(req.DiskType, "ssd"),
		Bandwidth:   req.Bandwidth,
		OS:          req.OS,
		Description: req.Description,
		Price:       req.Price,
		SortOrder:   req.SortOrder,
		Status:      req.Status,
	}
	if item.CPU < 1 {
		item.CPU = 1
	}
	if item.Memory < 1 {
		item.Memory = 1
	}
	if item.Disk < 1 {
		item.Disk = 40
	}
	if item.Status == 0 {
		item.Status = model.SpecTemplateEnabled
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *specTemplateService) Update(ctx context.Context, id uint64, req dto.SpecTemplateRequest) (*dto.SpecTemplateInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = strings.TrimSpace(req.Name)
	item.SpecFamily = defaultString(req.SpecFamily, model.SpecFamilyGeneral)
	item.CPU = req.CPU
	item.Memory = req.Memory
	item.Disk = req.Disk
	item.DiskType = defaultString(req.DiskType, "ssd")
	item.Bandwidth = req.Bandwidth
	item.OS = req.OS
	item.Description = req.Description
	item.Price = req.Price
	item.SortOrder = req.SortOrder
	item.Status = req.Status
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *specTemplateService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

// SpecMappingService 定义规格映射业务能力。
type SpecMappingService interface {
	List(ctx context.Context, query dto.SpecMappingQuery) (*dto.SpecMappingListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.SpecMappingInfo, error)
	Create(ctx context.Context, req dto.SpecMappingRequest) (*dto.SpecMappingInfo, error)
	Update(ctx context.Context, id uint64, req dto.SpecMappingRequest) (*dto.SpecMappingInfo, error)
	Bind(ctx context.Context, id uint64, req dto.SpecMappingBindRequest) (*dto.SpecMappingInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type specMappingService struct {
	repo repository.SpecMappingRepository
}

func NewSpecMappingService(repo repository.SpecMappingRepository) SpecMappingService {
	return &specMappingService{repo: repo}
}

func (s *specMappingService) List(ctx context.Context, query dto.SpecMappingQuery) (*dto.SpecMappingListResponse, error) {
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.SpecMappingInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildSpecMappingInfo(item))
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	return &dto.SpecMappingListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *specMappingService) FindByID(ctx context.Context, id uint64) (*dto.SpecMappingInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildSpecMappingInfo(*item)
	return &info, nil
}

func (s *specMappingService) Create(ctx context.Context, req dto.SpecMappingRequest) (*dto.SpecMappingInfo, error) {
	item := &model.SpecMapping{
		ProviderType:   req.ProviderType,
		UpstreamSpecID: strings.TrimSpace(req.UpstreamSpecID),
		UpstreamName:   req.UpstreamName,
		PlatformSpecID: req.PlatformSpecID,
		PlatformName:   req.PlatformName,
		CPU:            req.CPU,
		Memory:         req.Memory,
		Disk:           req.Disk,
		Status:         req.Status,
	}
	if item.Status == 0 && item.PlatformSpecID > 0 {
		item.Status = model.SpecMappingMapped
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *specMappingService) Update(ctx context.Context, id uint64, req dto.SpecMappingRequest) (*dto.SpecMappingInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.ProviderType = req.ProviderType
	item.UpstreamSpecID = strings.TrimSpace(req.UpstreamSpecID)
	item.UpstreamName = req.UpstreamName
	item.PlatformSpecID = req.PlatformSpecID
	item.PlatformName = req.PlatformName
	item.CPU = req.CPU
	item.Memory = req.Memory
	item.Disk = req.Disk
	item.Status = req.Status
	if item.Status == 0 && item.PlatformSpecID > 0 {
		item.Status = model.SpecMappingMapped
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

// Bind 将上游规格一次性映射到平台规格模板并保持状态为已映射。
func (s *specMappingService) Bind(ctx context.Context, id uint64, req dto.SpecMappingBindRequest) (*dto.SpecMappingInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.PlatformSpecID = req.PlatformSpecID
	item.PlatformName = req.PlatformName
	item.Status = model.SpecMappingMapped
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *specMappingService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

// ===== 辅助 =====

func buildSpecTemplateInfo(item model.SpecTemplate) dto.SpecTemplateInfo {
	return dto.SpecTemplateInfo{
		ID:          item.ID,
		Name:        item.Name,
		SpecFamily:  item.SpecFamily,
		CPU:         item.CPU,
		Memory:      item.Memory,
		Disk:        item.Disk,
		DiskType:    item.DiskType,
		Bandwidth:   item.Bandwidth,
		OS:          item.OS,
		Description: item.Description,
		Price:       item.Price,
		SortOrder:   item.SortOrder,
		Status:      item.Status,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.Format(time.RFC3339),
	}
}

func buildSpecMappingInfo(item model.SpecMapping) dto.SpecMappingInfo {
	return dto.SpecMappingInfo{
		ID:             item.ID,
		ProviderType:   item.ProviderType,
		UpstreamSpecID: item.UpstreamSpecID,
		UpstreamName:   item.UpstreamName,
		PlatformSpecID: item.PlatformSpecID,
		PlatformName:   item.PlatformName,
		CPU:            item.CPU,
		Memory:         item.Memory,
		Disk:           item.Disk,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      item.UpdatedAt.Format(time.RFC3339),
	}
}

func defaultString(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}
