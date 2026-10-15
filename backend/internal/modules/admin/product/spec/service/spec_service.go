package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
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
	machineKind, err := normalizeAndValidateMachineKind(req.MachineKind, req.PlatformParams, req.OptionSelections)
	if err != nil {
		return nil, err
	}
	item.MachineKind = machineKind
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
	machineKind, err := normalizeAndValidateMachineKind(req.MachineKind, req.PlatformParams, req.OptionSelections)
	if err != nil {
		return nil, err
	}
	item.MachineKind = machineKind
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
			Values  []string  `json:"values"`
			Range   []float64 `json:"range"`
			Default string    `json:"default"`
		}
		if err := json.Unmarshal(payload, &sel); err != nil {
			return fmt.Errorf("配置项「%s」的勾选结构非法", key)
		}
		if len(sel.Range) == 2 {
			// 数量型：区间内的取值由平台约束，但**默认值必须落在区间内**。
			//
			// 这个校验是补一个真实故障（用户反馈）：模板里 带宽 区间填 [10,100]、
			// 默认值却留 0（编辑器默认值初值），保存时无人拦截，于是生成的下单表单
			// 预填出 0 —— 客户看到的带宽是 0Mbps，而下限其实是 10。
			// 默认值出界时下游只会静默取到无效值，校验必须在这里拦住。
			if def := strings.TrimSpace(sel.Default); def != "" {
				defNum, perr := strconv.ParseFloat(def, 64)
				if perr != nil {
					return fmt.Errorf("配置项「%s」的默认值「%s」不是数字", key, def)
				}
				if defNum < sel.Range[0] || defNum > sel.Range[1] {
					return fmt.Errorf("配置项「%s」的默认值 %v 不在区间 [%v, %v] 内，请改正后再保存",
						key, defNum, sel.Range[0], sel.Range[1])
				}
			}
			continue
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
	return validateSKUAxisSelections(parsed)
}

// SKU 正交轴数量上限（与 catalog 服务的 buildAxisPlan 同口径，此处只做早期拦截）。
const (
	skuAxisMaxCombinations = 64
	skuAxisMaxPerOption    = 32
)

// validateSKUAxisSelections 校验标了「参与 SKU 正交」的参数可用于展开规格。
//
// 为什么在模板保存时就要拦：轴参数在**建品时**才展开成 SKU，若等到那时才发现
// "区间型参数不能做轴"，运营已经填完整张表才被打回。这里提前说清约束。
// 判定规则与 catalog 侧 buildAxisPlan 保持一致（两处都用同一个上限常量）。
func validateSKUAxisSelections(parsed map[string]json.RawMessage) error {
	axisCount := 0
	total := 1
	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var sel struct {
			Values  []string  `json:"values"`
			Range   []float64 `json:"range"`
			SKUAxis bool      `json:"sku_axis"`
		}
		if err := json.Unmarshal(parsed[key], &sel); err != nil {
			continue // 结构非法由上方主流程报错，这里不重复
		}
		if !sel.SKUAxis {
			continue
		}
		if len(sel.Range) == 2 {
			return fmt.Errorf(
				"配置项「%s」标了「参与 SKU 正交」，但它配的是取值范围（动态区间）。"+
					"参与正交的参数必须是离散取值（如内存 4G/8G/16G）；"+
					"带宽、数据盘这类连续量请改为不参与正交，作为客户下单时可调的加配项", key)
		}
		values := make([]string, 0, len(sel.Values))
		seen := map[string]bool{}
		for _, v := range sel.Values {
			v = strings.TrimSpace(v)
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			values = append(values, v)
		}
		if len(values) == 0 {
			return fmt.Errorf("配置项「%s」标了「参与 SKU 正交」，但没有勾选任何取值；请至少勾一个取值，或取消该项的正交标记", key)
		}
		if len(values) > skuAxisMaxPerOption {
			return fmt.Errorf("配置项「%s」勾了 %d 个取值，超出单个正交参数的上限 %d 个", key, len(values), skuAxisMaxPerOption)
		}
		axisCount++
		total *= len(values)
		if total > skuAxisMaxCombinations {
			return fmt.Errorf(
				"参与 SKU 正交的参数组合出 %d 个规格，超出上限 %d 个；"+
					"请减少参与正交的参数或取值 —— 组合数会随参数个数指数增长",
				total, skuAxisMaxCombinations)
		}
	}
	return nil
}

// normalizeJSON 归一前端回传的 JSON 片段为可写入 jsonb 列的值：
// 空/null 落 nil（列值 NULL）。不能返回空串——空串不是合法 jsonb，落库直接报 22P02。
func normalizeJSON(raw json.RawMessage) *string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return nil
	}
	return &s
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
		MachineKind:         item.MachineKind,
		OptionSelections:    json.RawMessage(nullableJSON(item.OptionSelections)),
		NameTemplate:        item.NameTemplate,
		DescriptionTemplate: item.DescriptionTemplate,
		Source:              defaultString(item.Source, model.SpecTemplateSourceSelf),
		CreatedAt:           item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           item.UpdatedAt.Format(time.RFC3339),
	}
}

// nullableJSON 把库里的 JSONB 文本（可空）转成可序列化的 RawMessage 取值：nil/空输出 null。
func nullableJSON(v *string) string {
	if v == nil {
		return "null"
	}
	s := strings.TrimSpace(*v)
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
