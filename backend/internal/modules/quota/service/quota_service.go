package service

import (
	"context"
	"fmt"
	"time"

	"hostsent/backend/internal/modules/quota/dto"
	"hostsent/backend/internal/modules/quota/model"
	"hostsent/backend/internal/modules/quota/repository"
)

// ResourceQuotaService defines the quota operations exposed to handlers.
type ResourceQuotaService interface {
	List(ctx context.Context, query dto.QuotaListQuery) (*dto.QuotaListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.QuotaInfo, error)
	FindByUserID(ctx context.Context, userID uint64) ([]dto.QuotaInfo, error)
	Adjust(ctx context.Context, id uint64, req dto.QuotaAdjustRequest) (*dto.QuotaInfo, error)
}

type QuotaTemplateService interface {
	List(ctx context.Context, query dto.QuotaTemplateListQuery) (*dto.QuotaTemplateListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.QuotaTemplateInfo, error)
	Create(ctx context.Context, req dto.QuotaTemplateCreateRequest) (*dto.QuotaTemplateInfo, error)
	Update(ctx context.Context, id uint64, req dto.QuotaTemplateUpdateRequest) (*dto.QuotaTemplateInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type UserLevelService interface {
	List(ctx context.Context, query dto.UserLevelListQuery) (*dto.UserLevelListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.UserLevelInfo, error)
	Create(ctx context.Context, req dto.UserLevelCreateRequest) (*dto.UserLevelInfo, error)
	Update(ctx context.Context, id uint64, req dto.UserLevelUpdateRequest) (*dto.UserLevelInfo, error)
	Delete(ctx context.Context, id uint64) error
	BindTemplate(ctx context.Context, id uint64, req dto.UserLevelBindTemplateRequest) (*dto.UserLevelInfo, error)
}

// QuotaAdjustmentService 定义配额调整记录相关的业务能力。
type QuotaAdjustmentService interface {
	List(ctx context.Context, query dto.QuotaAdjustmentListQuery) (*dto.QuotaAdjustmentListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.QuotaAdjustmentInfo, error)
}

type resourceQuotaService struct {
	quotaRepo repository.ResourceQuotaRepository
	logRepo   repository.QuotaAdjustmentRepository
}

type quotaTemplateService struct {
	repo repository.QuotaTemplateRepository
}

type userLevelService struct {
	repo repository.UserLevelRepository
}

type quotaAdjustmentService struct {
	repo repository.QuotaAdjustmentRepository
}

func NewResourceQuotaService(quotaRepo repository.ResourceQuotaRepository, logRepo repository.QuotaAdjustmentRepository) ResourceQuotaService {
	return &resourceQuotaService{quotaRepo: quotaRepo, logRepo: logRepo}
}

func NewQuotaTemplateService(repo repository.QuotaTemplateRepository) QuotaTemplateService {
	return &quotaTemplateService{repo: repo}
}

func NewUserLevelService(repo repository.UserLevelRepository) UserLevelService {
	return &userLevelService{repo: repo}
}

func NewQuotaAdjustmentService(repo repository.QuotaAdjustmentRepository) QuotaAdjustmentService {
	return &quotaAdjustmentService{repo: repo}
}

func (s *resourceQuotaService) List(ctx context.Context, query dto.QuotaListQuery) (*dto.QuotaListResponse, error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.quotaRepo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.QuotaInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toQuotaInfo(item))
	}
	return &dto.QuotaListResponse{
		Items: respItems,
		Meta:  dto.QuotaListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *resourceQuotaService) FindByID(ctx context.Context, id uint64) (*dto.QuotaInfo, error) {
	item, err := s.quotaRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toQuotaInfo(*item)
	return &resp, nil
}

func (s *resourceQuotaService) FindByUserID(ctx context.Context, userID uint64) ([]dto.QuotaInfo, error) {
	items, err := s.quotaRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.QuotaInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toQuotaInfo(item))
	}
	return respItems, nil
}

func (s *resourceQuotaService) Adjust(ctx context.Context, id uint64, req dto.QuotaAdjustRequest) (*dto.QuotaInfo, error) {
	item, err := s.quotaRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	beforeValue := item.LimitValue
	item.LimitValue = req.LimitValue
	item.AvailableValue = req.LimitValue - item.UsedValue
	item.IsOverallocated = item.AvailableValue < 0
	item.Source = "manual"
	item.LastAdjustedAt = time.Now()
	if err := s.quotaRepo.Update(ctx, &item.ResourceQuota); err != nil {
		return nil, err
	}
	_ = s.logRepo.Create(ctx, &model.QuotaAdjustmentLog{
		UserID:         item.UserID,
		Username:       item.Username,
		QuotaCode:      item.QuotaCode,
		QuotaName:      item.QuotaName,
		BeforeValue:    beforeValue,
		AfterValue:     req.LimitValue,
		DeltaValue:     req.LimitValue - beforeValue,
		AdjustmentType: "manual",
		Source:         "manual",
		TemplateID:     item.TemplateID,
		LevelID:        item.LevelID,
		OperatorID:     0,
		OperatorName:   "system",
		Reason:         req.Reason,
		TicketNo:       req.TicketNo,
		BatchNo:        fmt.Sprintf("quota-%d", time.Now().Unix()),
	})
	updated, err := s.quotaRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toQuotaInfo(*updated)
	return &resp, nil
}

func (s *quotaTemplateService) List(ctx context.Context, query dto.QuotaTemplateListQuery) (*dto.QuotaTemplateListResponse, error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.QuotaTemplateInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, dto.QuotaTemplateInfo{
			ID:          item.ID,
			Name:        item.Name,
			Code:        item.Code,
			Scope:       item.Scope,
			Status:      item.Status,
			Description: item.Description,
			Version:     item.Version,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return &dto.QuotaTemplateListResponse{Items: respItems, Meta: dto.QuotaListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *quotaTemplateService) FindByID(ctx context.Context, id uint64) (*dto.QuotaTemplateInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	children, err := s.repo.FindItems(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toQuotaTemplateInfo(*item, children)
	return &resp, nil
}

func (s *quotaTemplateService) Create(ctx context.Context, req dto.QuotaTemplateCreateRequest) (*dto.QuotaTemplateInfo, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	scope := req.Scope
	if scope == "" {
		scope = "default"
	}
	item := &model.QuotaTemplate{Name: req.Name, Code: req.Code, Scope: scope, Status: status, Description: req.Description, Version: 1}
	children := toQuotaTemplateItems(req.Items)
	if err := s.repo.Create(ctx, item, children); err != nil {
		return nil, err
	}
	resp := toQuotaTemplateInfo(*item, children)
	return &resp, nil
}

func (s *quotaTemplateService) Update(ctx context.Context, id uint64, req dto.QuotaTemplateUpdateRequest) (*dto.QuotaTemplateInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = req.Name
	item.Code = req.Code
	item.Scope = req.Scope
	item.Status = req.Status
	item.Description = req.Description
	item.Version += 1
	children := toQuotaTemplateItems(req.Items)
	if err := s.repo.Update(ctx, item, children); err != nil {
		return nil, err
	}
	resp := toQuotaTemplateInfo(*item, children)
	return &resp, nil
}

func (s *quotaTemplateService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *userLevelService) List(ctx context.Context, query dto.UserLevelListQuery) (*dto.UserLevelListResponse, error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.UserLevelInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toUserLevelInfo(item))
	}
	return &dto.UserLevelListResponse{Items: respItems, Meta: dto.QuotaListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *userLevelService) FindByID(ctx context.Context, id uint64) (*dto.UserLevelInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toUserLevelInfo(*item)
	return &resp, nil
}

func (s *userLevelService) Create(ctx context.Context, req dto.UserLevelCreateRequest) (*dto.UserLevelInfo, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	item := &model.UserLevel{
		Name: req.Name, Code: req.Code, Weight: req.Weight, Status: status, DefaultTemplateID: req.DefaultTemplateID,
		MaxInstanceCount: req.MaxInstanceCount, MaxCPUCores: req.MaxCPUCores, MaxMemoryGB: req.MaxMemoryGB, MaxDiskGB: req.MaxDiskGB,
		FeatureFlags: req.FeatureFlags, UpgradeCondition: req.UpgradeCondition, Description: req.Description,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	created, err := s.repo.FindByID(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	resp := toUserLevelInfo(*created)
	return &resp, nil
}

func (s *userLevelService) Update(ctx context.Context, id uint64, req dto.UserLevelUpdateRequest) (*dto.UserLevelInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = req.Name
	item.Code = req.Code
	item.Weight = req.Weight
	item.Status = req.Status
	item.DefaultTemplateID = req.DefaultTemplateID
	item.MaxInstanceCount = req.MaxInstanceCount
	item.MaxCPUCores = req.MaxCPUCores
	item.MaxMemoryGB = req.MaxMemoryGB
	item.MaxDiskGB = req.MaxDiskGB
	item.FeatureFlags = req.FeatureFlags
	item.UpgradeCondition = req.UpgradeCondition
	item.Description = req.Description
	if err := s.repo.Update(ctx, &item.UserLevel); err != nil {
		return nil, err
	}
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toUserLevelInfo(*updated)
	return &resp, nil
}

func (s *userLevelService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *userLevelService) BindTemplate(ctx context.Context, id uint64, req dto.UserLevelBindTemplateRequest) (*dto.UserLevelInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.DefaultTemplateID = &req.DefaultTemplateID
	if err := s.repo.Update(ctx, &item.UserLevel); err != nil {
		return nil, err
	}
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toUserLevelInfo(*updated)
	return &resp, nil
}

func (s *quotaAdjustmentService) List(ctx context.Context, query dto.QuotaAdjustmentListQuery) (*dto.QuotaAdjustmentListResponse, error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.QuotaAdjustmentInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toQuotaAdjustmentInfo(item))
	}
	return &dto.QuotaAdjustmentListResponse{Items: respItems, Meta: dto.QuotaListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *quotaAdjustmentService) FindByID(ctx context.Context, id uint64) (*dto.QuotaAdjustmentInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toQuotaAdjustmentInfo(*item)
	return &resp, nil
}

func normalizeMeta(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func toQuotaInfo(item repository.ResourceQuotaWithUser) dto.QuotaInfo {
	return dto.QuotaInfo{ID: item.ID, UserID: item.UserID, Username: item.Username, QuotaCode: item.QuotaCode, QuotaName: item.QuotaName, QuotaType: item.QuotaType, LimitValue: item.LimitValue, UsedValue: item.UsedValue, AvailableValue: item.AvailableValue, Unit: item.Unit, Status: item.Status, Source: item.Source, TemplateID: item.TemplateID, LevelID: item.LevelID, IsOverallocated: item.IsOverallocated, UpdatedBy: item.UpdatedBy, LastAdjustedAt: item.LastAdjustedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func toQuotaTemplateItems(items []dto.QuotaTemplateItemPayload) []model.QuotaTemplateItem {
	resp := make([]model.QuotaTemplateItem, 0, len(items))
	for _, item := range items {
		resp = append(resp, model.QuotaTemplateItem{QuotaCode: item.QuotaCode, QuotaName: item.QuotaName, QuotaType: item.QuotaType, LimitValue: item.LimitValue, Unit: item.Unit, Sort: item.Sort})
	}
	return resp
}

func toQuotaTemplateInfo(item model.QuotaTemplate, children []model.QuotaTemplateItem) dto.QuotaTemplateInfo {
	payloads := make([]dto.QuotaTemplateItemPayload, 0, len(children))
	for _, child := range children {
		payloads = append(payloads, dto.QuotaTemplateItemPayload{QuotaCode: child.QuotaCode, QuotaName: child.QuotaName, QuotaType: child.QuotaType, LimitValue: child.LimitValue, Unit: child.Unit, Sort: child.Sort})
	}
	return dto.QuotaTemplateInfo{ID: item.ID, Name: item.Name, Code: item.Code, Scope: item.Scope, Status: item.Status, Description: item.Description, Version: item.Version, CreatedBy: item.CreatedBy, UpdatedBy: item.UpdatedBy, Items: payloads, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func toUserLevelInfo(item repository.UserLevelWithTemplate) dto.UserLevelInfo {
	return dto.UserLevelInfo{ID: item.ID, Name: item.Name, Code: item.Code, Weight: item.Weight, Status: item.Status, DefaultTemplateID: item.DefaultTemplateID, DefaultTemplateName: item.DefaultTemplateName, MaxInstanceCount: item.MaxInstanceCount, MaxCPUCores: item.MaxCPUCores, MaxMemoryGB: item.MaxMemoryGB, MaxDiskGB: item.MaxDiskGB, FeatureFlags: item.FeatureFlags, UpgradeCondition: item.UpgradeCondition, Description: item.Description, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func toQuotaAdjustmentInfo(item model.QuotaAdjustmentLog) dto.QuotaAdjustmentInfo {
	return dto.QuotaAdjustmentInfo{ID: item.ID, UserID: item.UserID, Username: item.Username, QuotaCode: item.QuotaCode, QuotaName: item.QuotaName, BeforeValue: item.BeforeValue, AfterValue: item.AfterValue, DeltaValue: item.DeltaValue, AdjustmentType: item.AdjustmentType, Source: item.Source, TemplateID: item.TemplateID, LevelID: item.LevelID, OperatorID: item.OperatorID, OperatorName: item.OperatorName, Reason: item.Reason, TicketNo: item.TicketNo, BatchNo: item.BatchNo, CreatedAt: item.CreatedAt}
}
