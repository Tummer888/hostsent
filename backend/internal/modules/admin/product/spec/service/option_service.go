package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/model"
	"hostsent/backend/internal/modules/admin/product/spec/repository"
	"hostsent/backend/internal/pkg/upstream"
)

// ErrOptionSpecNotFound 配置项不存在。
var ErrOptionSpecNotFound = errors.New("平台配置项不存在")

// ErrProviderTypeUnknown 无法确定平台类型（渠道不存在 / 未声明配置项目录）。
var ErrProviderTypeUnknown = errors.New("无法确定平台类型：请提供 provider_type，或选择已接入的渠道")

// SpecOptionService 平台配置项目录与平台取值库业务能力（T4.5 规格配置化）。
type SpecOptionService interface {
	// ListCatalog 查某平台的配置项目录（适配器声明 ∪ 数据库覆盖），并挂上取值库中的可选值。
	ListCatalog(ctx context.Context, q dto.OptionCatalogQuery, includeHidden bool) ([]dto.OptionSpecInfo, error)
	// OptionCatalogByProvider 按平台类型取目录（含取值库可选值），供建品生成客户选配项用。
	// 与 ListCatalog 的区别：不需要渠道 ID，不触发平台实时拉取（生成 SKU 是离线动作）。
	OptionCatalogByProvider(ctx context.Context, providerType string) ([]dto.OptionSpecInfo, error)
	// SyncCatalog 把适配器声明的目录幂等导入数据库（不覆盖运营改过的行）。
	SyncCatalog(ctx context.Context, providerType string) (*dto.OptionCatalogSyncResult, error)
	// CreateSpec 新增自定义配置项（source=custom）。
	CreateSpec(ctx context.Context, req dto.OptionSpecRequest) (*dto.OptionSpecInfo, error)
	// UpdateSpec 修改配置项（标签/默认值/必选/控件/枚举/排序/隐藏）。
	UpdateSpec(ctx context.Context, id uint64, req dto.OptionSpecRequest) (*dto.OptionSpecInfo, error)
	// DeleteSpec 删除配置项及其取值。
	DeleteSpec(ctx context.Context, id uint64) error

	// ListValues 查取值库。
	ListValues(ctx context.Context, q dto.OptionValueQuery) ([]dto.OptionValueItem, error)
	// UpsertValue 新增/更新一条取值。
	UpsertValue(ctx context.Context, req dto.OptionValueUpsertRequest) (*dto.OptionValueItem, error)
	// ImportValues 批量导入取值（镜像类型直接入库的入口）。
	ImportValues(ctx context.Context, req dto.OptionValueImportRequest) (*dto.OptionValueImportResult, error)
	// RefreshFromPlatform 把平台实时资源（区域/节点/存储/镜像）灌入取值库。
	RefreshFromPlatform(ctx context.Context, providerType string, providerID uint64, optionKeys []string) (*dto.OptionValueImportResult, error)
	// DeleteValue 删除一条取值。
	DeleteValue(ctx context.Context, id uint64) error
}

// providerTypeReader 由渠道 ID 反查平台类型（装配层注入，避免 spec 依赖 resource 子域）。
type providerTypeReader interface {
	ProviderTypeByID(ctx context.Context, id uint64) (string, error)
}

// platformResourceReader 读取平台实时资源目录（取值库刷新用）。
type platformResourceReader interface {
	PlatformResources(ctx context.Context, providerID uint64) (*upstream.PlatformResources, error)
}

type specOptionService struct {
	repo     repository.SpecOptionRepository
	types    providerTypeReader
	platform platformResourceReader
}

// NewSpecOptionService 创建平台配置项目录服务。types/platform 可为 nil（则不支持下线刷新）。
func NewSpecOptionService(repo repository.SpecOptionRepository, types providerTypeReader, platform platformResourceReader) SpecOptionService {
	return &specOptionService{repo: repo, types: types, platform: platform}
}

// resolveProviderType 归一平台类型：显式 provider_type 优先，其次用 provider_id 反查。
func (s *specOptionService) resolveProviderType(ctx context.Context, providerType string, providerID uint64) (string, error) {
	if t := strings.TrimSpace(providerType); t != "" {
		return t, nil
	}
	if providerID != 0 && s.types != nil {
		if t, err := s.types.ProviderTypeByID(ctx, providerID); err == nil && strings.TrimSpace(t) != "" {
			return t, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", ErrProviderTypeUnknown
}

// OptionCatalogByProvider 按平台类型取目录（无需渠道 ID，不触发平台实时拉取）。
func (s *specOptionService) OptionCatalogByProvider(ctx context.Context, providerType string) ([]dto.OptionSpecInfo, error) {
	providerType = strings.TrimSpace(providerType)
	if providerType == "" {
		return nil, nil
	}
	return s.catalogByProvider(ctx, providerType, false)
}

func (s *specOptionService) ListCatalog(ctx context.Context, q dto.OptionCatalogQuery, includeHidden bool) ([]dto.OptionSpecInfo, error) {
	providerType, err := s.resolveProviderType(ctx, q.ProviderType, q.ProviderID)
	if err != nil {
		return nil, err
	}
	infos, err := s.catalogByProvider(ctx, providerType, includeHidden)
	if err != nil {
		return nil, err
	}
	// 平台来源的配置项（镜像/区域/节点/存储）在库为空时补一次实时读取，
	// 否则运营第一次打开页面看不到任何镜像可勾。
	if q.ProviderID != 0 && s.platform != nil {
		if err := s.fillLiveValues(ctx, q.ProviderID, infos); err != nil {
			// 平台不可达不该让整个目录读取失败：库里已有的取值继续返回。
			_ = err
		}
	}
	return infos, nil
}

// catalogByProvider 目录读取的公共实现：库内行优先，库空时回退适配器声明。
func (s *specOptionService) catalogByProvider(ctx context.Context, providerType string, includeHidden bool) ([]dto.OptionSpecInfo, error) {
	specs, err := s.repo.ListSpecs(ctx, providerType, includeHidden)
	if err != nil {
		return nil, err
	}
	// 尚未同步过：直接回适配器声明的目录（后台页面不会一片空白，可一键同步入库）。
	if len(specs) == 0 {
		return fromAdapterCatalog(providerType, upstream.OptionCatalogOf(providerType), includeHidden), nil
	}
	infos := make([]dto.OptionSpecInfo, 0, len(specs))
	keys := make([]string, 0, len(specs))
	for _, sp := range specs {
		info := buildOptionSpecInfo(sp)
		// 静态枚举：options 列存着候选值，直接当可选值下发给前端（不落取值库）。
		if info.ValueSource == upstream.ValueSourceStatic && len(info.Values) == 0 {
			info.Values = parseStaticOptions(info.Options)
		}
		infos = append(infos, info)
		keys = append(keys, sp.OptionKey)
	}
	// 挂上取值库中的取值（静态枚举行不需要取库）。
	if values, err := s.repo.ListValuesByKeys(ctx, providerType, keys, false); err == nil {
		for i := range infos {
			if dbValues := values[infos[i].OptionKey]; len(dbValues) > 0 {
				infos[i].Values = toValueItems(dbValues)
			}
		}
	}
	return infos, nil
}

// parseStaticOptions 把 options 列（[{label,value}]）解析为可选值列表。
func parseStaticOptions(raw json.RawMessage) []dto.OptionValueItem {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var opts []upstream.FieldOption
	if err := json.Unmarshal(raw, &opts); err != nil {
		return nil
	}
	out := make([]dto.OptionValueItem, 0, len(opts))
	for _, o := range opts {
		out = append(out, dto.OptionValueItem{
			Value: o.Value, Label: o.Label,
			Status: model.OptionValueActive, Origin: model.OptionSpecSourceAdapter,
		})
	}
	return out
}

// fillLiveValues 为平台来源的配置项补实时取值（库里已有的按 value 去重合并）。
func (s *specOptionService) fillLiveValues(ctx context.Context, providerID uint64, infos []dto.OptionSpecInfo) error {
	need := false
	for _, info := range infos {
		if isPlatformValueSource(info.ValueSource) {
			need = true
			break
		}
	}
	if !need {
		return nil
	}
	res, err := s.platform.PlatformResources(ctx, providerID)
	if err != nil {
		return err
	}
	for i := range infos {
		live := liveItemsFor(res, infos[i].ValueSource)
		if len(live) == 0 {
			continue
		}
		seen := map[string]bool{}
		for _, v := range infos[i].Values {
			seen[v.Value] = true
		}
		for _, item := range live {
			if seen[item.Value] {
				// 库里已有该取值：以库里的标签/状态为准（运营可能改过），只补分组。
				for j := range infos[i].Values {
					if infos[i].Values[j].Value == item.Value && infos[i].Values[j].GroupLabel == "" {
						infos[i].Values[j].GroupLabel = item.GroupLabel
					}
				}
				continue
			}
			infos[i].Values = append(infos[i].Values, item)
		}
	}
	return nil
}

// isPlatformValueSource 判断取值来源是否来自平台接口。
func isPlatformValueSource(vs string) bool {
	switch vs {
	case upstream.ValueSourceAreas, upstream.ValueSourceNodes,
		upstream.ValueSourceStores, upstream.ValueSourceImages:
		return true
	}
	return false
}

// liveItemsFor 把平台资源目录按取值来源映射成可选值。
func liveItemsFor(res *upstream.PlatformResources, valueSource string) []dto.OptionValueItem {
	if res == nil {
		return nil
	}
	var src []upstream.PlatformResourceItem
	switch valueSource {
	case upstream.ValueSourceAreas:
		src = res.Areas
	case upstream.ValueSourceNodes:
		src = res.Nodes
	case upstream.ValueSourceStores:
		src = res.Stores
	case upstream.ValueSourceImages:
		src = res.Images
	default:
		return nil
	}
	out := make([]dto.OptionValueItem, 0, len(src))
	// 同一取值可能因挂在多个节点下出现多次（镜像 × 节点），按 value 去重保留首个。
	seen := map[string]bool{}
	for _, it := range src {
		if seen[it.Value] {
			continue
		}
		seen[it.Value] = true
		out = append(out, dto.OptionValueItem{
			Label:       it.Label,
			Value:       it.Value,
			GroupLabel:  it.Group,
			ParentValue: it.ParentID,
			Status:      firstNonEmptyStr(it.Status, model.OptionValueActive),
			Origin:      model.OptionValueOriginPlatform,
		})
	}
	return out
}

func (s *specOptionService) SyncCatalog(ctx context.Context, providerType string) (*dto.OptionCatalogSyncResult, error) {
	providerType = strings.TrimSpace(providerType)
	if providerType == "" {
		return nil, ErrProviderTypeUnknown
	}
	declared := upstream.OptionCatalogOf(providerType)
	if len(declared) == 0 {
		return nil, fmt.Errorf("%w：%s 未声明配置项目录", ErrProviderTypeUnknown, providerType)
	}
	res := &dto.OptionCatalogSyncResult{ProviderType: providerType, Declared: len(declared)}
	for _, spec := range declared {
		item := specToModel(providerType, spec)
		created, err := s.repo.UpsertSpecIfAbsent(ctx, item)
		if err != nil {
			return nil, err
		}
		if created {
			res.Created++
		} else {
			res.Skipped++
		}
	}
	return res, nil
}

func (s *specOptionService) CreateSpec(ctx context.Context, req dto.OptionSpecRequest) (*dto.OptionSpecInfo, error) {
	providerType, err := s.resolveProviderType(ctx, req.ProviderType, 0)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(req.OptionKey)
	if key == "" {
		return nil, errors.New("参数名不能为空")
	}
	if existing, err := s.repo.FindSpec(ctx, providerType, key); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, fmt.Errorf("配置项 %s 已存在", key)
	}
	item := &model.ProviderOptionSpec{
		ProviderType: providerType,
		OptionKey:    key,
		Label:        firstNonEmptyStr(strings.TrimSpace(req.Label), key),
		GroupName:    req.GroupName,
		Required:     req.Required,
		DefaultValue: req.DefaultValue,
		Widget:       firstNonEmptyStr(req.Widget, upstream.ConfigWidgetSelect),
		ValueSource:  firstNonEmptyStr(req.ValueSource, upstream.ValueSourceStatic),
		Options:      rawToStringPtr(req.Options),
		MinValue:     req.MinValue,
		MaxValue:     req.MaxValue,
		StepValue:    req.StepValue,
		Unit:         req.Unit,
		Help:         req.Help,
		MultiValue:   req.MultiValue,
		Hidden:       req.Hidden,
		SortOrder:    req.SortOrder,
		Source:       model.OptionSpecSourceCustom,
	}
	if err := s.repo.CreateSpec(ctx, item); err != nil {
		return nil, err
	}
	info := buildOptionSpecInfo(*item)
	return &info, nil
}

func (s *specOptionService) UpdateSpec(ctx context.Context, id uint64, req dto.OptionSpecRequest) (*dto.OptionSpecInfo, error) {
	item, err := s.findSpecByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if label := strings.TrimSpace(req.Label); label != "" {
		item.Label = label
	}
	item.GroupName = req.GroupName
	item.Required = req.Required
	item.DefaultValue = req.DefaultValue
	if req.Widget != "" {
		item.Widget = req.Widget
	}
	if req.ValueSource != "" {
		item.ValueSource = req.ValueSource
	}
	if len(req.Options) > 0 {
		item.Options = rawToStringPtr(req.Options)
	}
	item.MinValue = req.MinValue
	item.MaxValue = req.MaxValue
	item.StepValue = req.StepValue
	item.Unit = req.Unit
	item.Help = req.Help
	item.MultiValue = req.MultiValue
	item.Hidden = req.Hidden
	item.SortOrder = req.SortOrder
	if err := s.repo.UpdateSpec(ctx, item); err != nil {
		return nil, err
	}
	info := buildOptionSpecInfo(*item)
	return &info, nil
}

func (s *specOptionService) DeleteSpec(ctx context.Context, id uint64) error {
	item, err := s.findSpecByID(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.DeleteSpec(ctx, item.ProviderType, item.OptionKey)
}

func (s *specOptionService) findSpecByID(ctx context.Context, id uint64) (*model.ProviderOptionSpec, error) {
	item, err := s.repo.FindSpecByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrOptionSpecNotFound
	}
	return item, nil
}

func (s *specOptionService) ListValues(ctx context.Context, q dto.OptionValueQuery) ([]dto.OptionValueItem, error) {
	providerType, err := s.resolveProviderType(ctx, q.ProviderType, q.ProviderID)
	if err != nil {
		return nil, err
	}
	q.ProviderType = providerType
	items, err := s.repo.ListValues(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && q.ProviderID != 0 && s.platform != nil && strings.TrimSpace(q.OptionKey) != "" {
		// 库为空：尝试从平台实时拉一次对应配置项的取值。
		if _, err := s.RefreshFromPlatformForKeys(ctx, providerType, q.ProviderID, []string{q.OptionKey}); err == nil {
			items, err = s.repo.ListValues(ctx, q)
			if err != nil {
				return nil, err
			}
		}
	}
	return toValueItems(items), nil
}

func (s *specOptionService) UpsertValue(ctx context.Context, req dto.OptionValueUpsertRequest) (*dto.OptionValueItem, error) {
	providerType, err := s.resolveProviderType(ctx, req.ProviderType, 0)
	if err != nil {
		return nil, err
	}
	item := &model.PlatformOptionValue{
		ProviderType: providerType,
		OptionKey:    strings.TrimSpace(req.OptionKey),
		Value:        strings.TrimSpace(req.Value),
		Label:        req.Label,
		ParentValue:  req.ParentValue,
		GroupLabel:   req.GroupLabel,
		Status:       firstNonEmptyStr(req.Status, model.OptionValueActive),
		Origin:       model.OptionValueOriginManual,
		SortOrder:    req.SortOrder,
	}
	if err := s.repo.UpsertValue(ctx, item); err != nil {
		return nil, err
	}
	return &dto.OptionValueItem{
		Label: item.Label, Value: item.Value, GroupLabel: item.GroupLabel,
		ParentValue: item.ParentValue, Status: item.Status, Origin: item.Origin,
	}, nil
}

func (s *specOptionService) ImportValues(ctx context.Context, req dto.OptionValueImportRequest) (*dto.OptionValueImportResult, error) {
	providerType, err := s.resolveProviderType(ctx, req.ProviderType, 0)
	if err != nil {
		return nil, err
	}
	optionKey := strings.TrimSpace(req.OptionKey)
	if optionKey == "" {
		return nil, errors.New("请指定要导入的配置项")
	}
	res := &dto.OptionValueImportResult{OptionKey: optionKey}
	// 替换模式：先把该配置项下平台来源的旧取值停用，再灌入新的一批
	// （镜像在平台上下架后，旧值应自动消失而不是继续可勾）。
	if req.Replace {
		if n, err := s.repo.OfflineValues(ctx, providerType, optionKey, model.OptionValueOriginPlatform); err == nil {
			res.Offlined = int(n)
		}
	}
	for _, it := range req.Items {
		value := strings.TrimSpace(it.Value)
		if value == "" {
			continue
		}
		origin := firstNonEmptyStr(it.Origin, model.OptionValueOriginManual)
		item := &model.PlatformOptionValue{
			ProviderType: providerType,
			OptionKey:    optionKey,
			Value:        value,
			Label:        firstNonEmptyStr(it.Label, value),
			ParentValue:  it.ParentValue,
			GroupLabel:   it.GroupLabel,
			Status:       firstNonEmptyStr(it.Status, model.OptionValueActive),
			Origin:       origin,
		}
		before, _ := s.repo.ListValues(ctx, dto.OptionValueQuery{
			ProviderType: providerType, OptionKey: optionKey, IncludeOffline: true,
		})
		exists := false
		for _, b := range before {
			if b.Value == value && b.ParentValue == item.ParentValue {
				exists = true
				break
			}
		}
		if err := s.repo.UpsertValue(ctx, item); err != nil {
			return nil, err
		}
		if exists {
			res.Updated++
		} else {
			res.Created++
		}
	}
	return res, nil
}

func (s *specOptionService) RefreshFromPlatform(ctx context.Context, providerType string, providerID uint64, optionKeys []string) (*dto.OptionValueImportResult, error) {
	t, err := s.resolveProviderType(ctx, providerType, providerID)
	if err != nil {
		return nil, err
	}
	return s.RefreshFromPlatformForKeys(ctx, t, providerID, optionKeys)
}

// RefreshFromPlatformForKeys 把平台实时资源灌入取值库。
// optionKeys 支持文档口径的配置项名（os/area/node/store），也接受原生来源名
// （images/areas/nodes/stores），方便运营在配置项改名后仍能刷新。
// 目录里声明了平台来源的配置项都会被刷新（optionKeys 为空时按目录自动推导）。
func (s *specOptionService) RefreshFromPlatformForKeys(ctx context.Context, providerType string, providerID uint64, optionKeys []string) (*dto.OptionValueImportResult, error) {
	if s.platform == nil || providerID == 0 {
		return nil, errors.New("未装配平台资源读取能力，无法刷新取值库")
	}
	if len(optionKeys) == 0 {
		// 未显式指定：按目录里所有平台来源的配置项刷新（镜像/区域/节点/存储一把刷完）。
		specs, err := s.repo.ListSpecs(ctx, providerType, true)
		if err != nil {
			return nil, err
		}
		if len(specs) == 0 {
			for _, spec := range upstream.OptionCatalogOf(providerType) {
				if isPlatformValueSource(spec.ValueSource) {
					optionKeys = append(optionKeys, spec.Key)
				}
			}
		} else {
			for _, spec := range specs {
				if isPlatformValueSource(spec.ValueSource) {
					optionKeys = append(optionKeys, spec.OptionKey)
				}
			}
		}
	}
	res, err := s.platform.PlatformResources(ctx, providerID)
	if err != nil {
		return nil, err
	}
	out := &dto.OptionValueImportResult{}
	for _, raw := range optionKeys {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		source := valueSourceForOptionKey(key)
		if source == "" {
			// 库里的配置项可能已改名：用目录声明反查它的取值来源。
			if spec, ferr := s.repo.FindSpec(ctx, providerType, key); ferr == nil && spec != nil {
				source = spec.ValueSource
			}
		}
		items := liveItemsFor(res, source)
		if len(items) == 0 {
			continue
		}
		imported, err := s.ImportValues(ctx, dto.OptionValueImportRequest{
			ProviderType: providerType, OptionKey: key, Replace: true, Items: items,
		})
		if err != nil {
			return nil, err
		}
		out.OptionKey = key
		out.Created += imported.Created
		out.Updated += imported.Updated
		out.Offlined += imported.Offlined
	}
	return out, nil
}

// valueSourceForOptionKey 平台配置项名 → 取值来源。
// 魔方云口径：area→区域、node→节点、store→存储、os→镜像。
func valueSourceForOptionKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "area", "areas":
		return upstream.ValueSourceAreas
	case "node", "nodes":
		return upstream.ValueSourceNodes
	case "store", "stores":
		return upstream.ValueSourceStores
	case "os", "image", "images":
		return upstream.ValueSourceImages
	}
	return ""
}

func (s *specOptionService) DeleteValue(ctx context.Context, id uint64) error {
	return s.repo.DeleteValue(ctx, id)
}

// ===== 映射辅助 =====

// specToModel 适配器声明 → 目录表行。
func specToModel(providerType string, spec upstream.ConfigOptionSpec) *model.ProviderOptionSpec {
	return &model.ProviderOptionSpec{
		ProviderType: providerType,
		OptionKey:    spec.Key,
		Label:        spec.Label,
		GroupName:    spec.Group,
		Required:     spec.Required,
		DefaultValue: spec.Default,
		Widget:       firstNonEmptyStr(spec.Widget, upstream.ConfigWidgetSelect),
		ValueSource:  firstNonEmptyStr(spec.ValueSource, upstream.ValueSourceStatic),
		Options:      optionsToJSONPtr(spec.Options),
		MinValue:     spec.MinValue,
		MaxValue:     spec.MaxValue,
		StepValue:    spec.Step,
		Unit:         spec.Unit,
		Help:         spec.Help,
		MultiValue:   spec.MultiValue,
		SortOrder:    spec.SortOrder,
		Source:       model.OptionSpecSourceAdapter,
	}
}

// fromAdapterCatalog 未落库时直接展示适配器声明（页面可读，一键同步后转为库内行）。
func fromAdapterCatalog(providerType string, catalog []upstream.ConfigOptionSpec, includeHidden bool) []dto.OptionSpecInfo {
	out := make([]dto.OptionSpecInfo, 0, len(catalog))
	for _, spec := range catalog {
		item := specToModel(providerType, spec)
		info := buildOptionSpecInfo(*item)
		if info.ValueSource == upstream.ValueSourceStatic {
			info.Values = optionsToItems(spec.Options)
		}
		if !includeHidden && info.Hidden {
			continue
		}
		out = append(out, info)
	}
	return out
}

func buildOptionSpecInfo(item model.ProviderOptionSpec) dto.OptionSpecInfo {
	return dto.OptionSpecInfo{
		ID:           item.ID,
		ProviderType: item.ProviderType,
		OptionKey:    item.OptionKey,
		Label:        item.Label,
		GroupName:    item.GroupName,
		Required:     item.Required,
		DefaultValue: item.DefaultValue,
		Widget:       item.Widget,
		ValueSource:  item.ValueSource,
		Options:      nullableJSONPtr(item.Options),
		MinValue:     item.MinValue,
		MaxValue:     item.MaxValue,
		StepValue:    item.StepValue,
		Unit:         item.Unit,
		Help:         item.Help,
		MultiValue:   item.MultiValue,
		Hidden:       item.Hidden,
		SortOrder:    item.SortOrder,
		Source:       firstNonEmptyStr(item.Source, model.OptionSpecSourceAdapter),
	}
}

func toValueItems(items []model.PlatformOptionValue) []dto.OptionValueItem {
	out := make([]dto.OptionValueItem, 0, len(items))
	for _, it := range items {
		out = append(out, dto.OptionValueItem{
			Label: it.Label, Value: it.Value, GroupLabel: it.GroupLabel,
			ParentValue: it.ParentValue, Status: it.Status, Origin: it.Origin,
		})
	}
	return out
}

func optionsToItems(options []upstream.FieldOption) []dto.OptionValueItem {
	out := make([]dto.OptionValueItem, 0, len(options))
	for _, o := range options {
		out = append(out, dto.OptionValueItem{
			Label: o.Label, Value: o.Value, Status: model.OptionValueActive,
			Origin: model.OptionSpecSourceAdapter,
		})
	}
	return out
}

func optionsToJSON(options []upstream.FieldOption) string {
	if len(options) == 0 {
		return ""
	}
	b, err := json.Marshal(options)
	if err != nil {
		return ""
	}
	return string(b)
}

func optionsToJSONPtr(options []upstream.FieldOption) *string {
	s := optionsToJSON(options)
	if s == "" {
		return nil
	}
	return &s
}

// rawToStringPtr 把前端回传的 JSON 片段转成 *string：空/null 返回 nil。
// 库里 options 是 jsonb 列，空串不是合法 jsonb（会报 22P02），必须落 NULL。
func rawToStringPtr(raw json.RawMessage) *string {
	s := rawToString(raw)
	if s == "" {
		return nil
	}
	return &s
}

// nullableJSONPtr 库里的 jsonb 文本转 RawMessage：nil/空输出 JSON null。
func nullableJSONPtr(v *string) json.RawMessage {
	if v == nil {
		return json.RawMessage("null")
	}
	return nullableRawJSON(*v)
}

func rawToString(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	return s
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// nullableRawJSON 空串输出 JSON null（前端 JSON.parse 友好）。
func nullableRawJSON(v string) json.RawMessage {
	s := strings.TrimSpace(v)
	if s == "" || s == "null" {
		return json.RawMessage("null")
	}
	return json.RawMessage(s)
}
