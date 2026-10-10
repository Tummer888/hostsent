package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/model"
	"hostsent/backend/internal/modules/admin/product/spec/repository"
)

// SpecTemplateService 定义规格模板（配置档）业务能力。
type SpecTemplateService interface {
	List(ctx context.Context, query dto.SpecTemplateQuery) (*dto.SpecTemplateListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.SpecTemplateInfo, error)
	Create(ctx context.Context, req dto.SpecTemplateRequest) (*dto.SpecTemplateInfo, error)
	Update(ctx context.Context, id uint64, req dto.SpecTemplateRequest) (*dto.SpecTemplateInfo, error)
	Delete(ctx context.Context, id uint64) error
	// SetOptionCatalog 注入平台配置项目录服务（配置档勾选取值校验用；可为 nil）。
	SetOptionCatalog(svc SpecOptionService)
}

type specTemplateService struct {
	repo repository.SpecTemplateRepository
	// options 平台配置项目录读取能力：用于校验配置档勾选的参数与取值确实存在于该平台。
	// 未装配时跳过校验（存量调用方与老数据不受影响）。
	options SpecOptionService
}

func NewSpecTemplateService(repo repository.SpecTemplateRepository) SpecTemplateService {
	return &specTemplateService{repo: repo}
}

// SetOptionCatalog 注入平台配置项目录服务（装配层调用）：
// 配置档保存时校验 option_selections 的每个参数名与取值都在目录/取值库中存在。
func (s *specTemplateService) SetOptionCatalog(svc SpecOptionService) {
	s.options = svc
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
		SpecValues:  normalizeJSON(req.SpecValues),
		// 配置档改造：平台归属 + 每参数勾选的可选值 + 名称/描述模板。
		ProviderType:        strings.TrimSpace(req.ProviderType),
		OptionSelections:    normalizeJSON(req.OptionSelections),
		NameTemplate:        strings.TrimSpace(req.NameTemplate),
		DescriptionTemplate: strings.TrimSpace(req.DescriptionTemplate),
		PlatformParams:      normalizeJSON(req.PlatformParams),
		// spec_family 不再写入：保留列为默认值 general，仅作旧数据兼容。
		SpecFamily: model.SpecFamilyGeneral,
		Source:     model.SpecTemplateSourceSelf,
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
	if err := s.validateOptionSelections(ctx, item.ProviderType, req.OptionSelections); err != nil {
		return nil, err
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
	// 原子取值与平台参数：空值即"清空"（运营显式删除映射），与指针语义无关，
	// 因为模板编辑表单始终回传完整 JSON。
	item.SpecValues = normalizeJSON(req.SpecValues)
	item.PlatformParams = normalizeJSON(req.PlatformParams)
	item.ProviderType = strings.TrimSpace(req.ProviderType)
	item.OptionSelections = normalizeJSON(req.OptionSelections)
	item.NameTemplate = strings.TrimSpace(req.NameTemplate)
	item.DescriptionTemplate = strings.TrimSpace(req.DescriptionTemplate)
	if err := s.validateOptionSelections(ctx, item.ProviderType, req.OptionSelections); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

// validateOptionSelections 校验配置档勾选的参数与取值确实存在于该平台的配置项目录。
//
// 为什么必须在保存时校验：配置档是建品时生成客户选配项与平台参数的来源，键名写错
// （如把 system_disk_size 写成 disk）不会立刻报错，而是一路带到面板才被拒；取值写错
// （勾了一个平台不存在的镜像 ID）则客户能选中但开不了机。宁可在保存时拦住。
// 目录服务未装配 / 平台未声明目录时跳过校验（老模板与自定义平台不受影响）。
func (s *specTemplateService) validateOptionSelections(ctx context.Context, providerType string, raw json.RawMessage) error {
	selections := strings.TrimSpace(string(raw))
	if selections == "" || selections == "null" {
		return nil
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(selections), &parsed); err != nil {
		return errors.New("选配项 JSON 非法（应为 参数名 → 勾选取值 的对象）")
	}
	if len(parsed) == 0 {
		return nil
	}
	if s.options == nil || strings.TrimSpace(providerType) == "" {
		return nil
	}
	infos, err := s.options.OptionCatalogByProvider(ctx, providerType)
	if err != nil || len(infos) == 0 {
		// 目录不可读时放行：不能因为目录服务抖动而阻止运营保存档位。
		return nil
	}
	allowed := make(map[string]map[string]bool, len(infos))
	for _, info := range infos {
		values := make(map[string]bool, len(info.Values))
		for _, v := range info.Values {
			values[v.Value] = true
		}
		allowed[info.OptionKey] = values
	}
	for key, payload := range parsed {
		valueSet, ok := allowed[key]
		if !ok {
			return fmt.Errorf("配置档含平台 %s 未声明的配置项「%s」", providerType, key)
		}
		var sel struct {
			Values []string  `json:"values"`
			Range  []float64 `json:"range"`
		}
		if err := json.Unmarshal(payload, &sel); err != nil {
			return fmt.Errorf("配置项「%s」的勾选结构非法", key)
		}
		if len(sel.Range) == 2 {
			continue // 数量型只给区间，取值在区间内由平台约束
		}
		if len(valueSet) == 0 {
			continue // 目录该参数无受控取值（手工填 ID 类型）：不逐个校验
		}
		for _, v := range sel.Values {
			if !valueSet[v] {
				return fmt.Errorf("配置项「%s」的取值「%s」不在平台取值库中，请先在「平台配置项」页导入", key, v)
			}
		}
	}
	return nil
}

// normalizeJSON 归一前端回传的 JSON 片段：空/null 落为空串（列值 NULL），非法 JSON 原样保留由仓储报错。
func normalizeJSON(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	return s
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
		ID:                  item.ID,
		Name:                item.Name,
		SpecFamily:          item.SpecFamily,
		CPU:                 item.CPU,
		Memory:              item.Memory,
		Disk:                item.Disk,
		DiskType:            item.DiskType,
		Bandwidth:           item.Bandwidth,
		OS:                  item.OS,
		Description:         item.Description,
		Price:               item.Price,
		SortOrder:           item.SortOrder,
		Status:              item.Status,
		SpecValues:          json.RawMessage(nullableJSON(item.SpecValues)),
		PlatformParams:      json.RawMessage(nullableJSON(item.PlatformParams)),
		ProviderType:        item.ProviderType,
		OptionSelections:    json.RawMessage(nullableJSON(item.OptionSelections)),
		NameTemplate:        item.NameTemplate,
		DescriptionTemplate: item.DescriptionTemplate,
		Source:              defaultString(item.Source, model.SpecTemplateSourceSelf),
		CreatedAt:           item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           item.UpdatedAt.Format(time.RFC3339),
	}
}

// nullableJSON 把库里的 JSONB 文本转成可序列化的 RawMessage 取值：空串输出 null。
func nullableJSON(v string) string {
	s := strings.TrimSpace(v)
	if s == "" || s == "null" {
		return "null"
	}
	return s
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
