package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
	"hostsent/backend/internal/modules/admin/product/catalog/repository"
	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/specatom"
)

// 业务错误
var (
	// ErrProductInvalidStatus 非法状态流转
	ErrProductInvalidStatus = errors.New("非法的产品状态流转")
	// ErrSpecNotFound SKU 不存在或不属于该商品
	ErrSpecNotFound = errors.New("规格不存在")
	// ErrSpecCodeExists 同一商品下规格编码重复
	ErrSpecCodeExists = errors.New("规格编码已存在")
	// ErrSpecInvalid 规格取值不符合原子字典约束
	ErrSpecInvalid = errors.New("规格不符合原子字典约束")
)

// ProductService 定义商品业务能力。
type ProductService interface {
	List(ctx context.Context, query dto.ProductListQuery) (*dto.ProductListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.ProductInfo, error)
	Create(ctx context.Context, req dto.ProductCreateRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	Update(ctx context.Context, id uint64, req dto.ProductUpdateRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	Delete(ctx context.Context, id uint64, operatorID uint64, operatorName string) error
	Publish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	Unpublish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	UpdatePrice(ctx context.Context, id uint64, req dto.ProductPriceRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	SetFeatured(ctx context.Context, id uint64, featured bool, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	ListHistory(ctx context.Context, id uint64) ([]dto.ProductHistoryInfo, error)
	ListSpecs(ctx context.Context, id uint64) ([]dto.ProductSpecInfo, error)
	// CreateSpec / UpdateSpec / DeleteSpec 维护商品下的 SKU 矩阵（T4.1）。
	CreateSpec(ctx context.Context, productID uint64, req dto.ProductSpecRequest, operatorID uint64, operatorName string) (*dto.ProductSpecInfo, error)
	UpdateSpec(ctx context.Context, productID, specID uint64, req dto.ProductSpecRequest, operatorID uint64, operatorName string) (*dto.ProductSpecInfo, error)
	DeleteSpec(ctx context.Context, productID, specID uint64, operatorID uint64, operatorName string) error
	// CloneFromUpstream 从上游资源商品克隆创建销售商品（对接魔方财务商品导入）
	CloneFromUpstream(ctx context.Context, req dto.ProductCloneRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	// CloneFromUpstreamBatch 批量从上游商品克隆创建销售商品（按百分比定价）
	CloneFromUpstreamBatch(ctx context.Context, req dto.ProductBatchCloneRequest, operatorID uint64, operatorName string) ([]dto.ProductInfo, error)
	// BuildProvisionRequest 按商品供货模式构建上游开通请求（供订单履约联动用）。
	// specSnapshot 为所选 SKU 的规格快照（原子取值 JSON，T4.1）；specCode 为所选 SKU 编码
	// （T4.2），用于回查该 SKU 已确认的平台绑定参数。两者均可为空。
	BuildProvisionRequest(ctx context.Context, productID uint64, name, specSnapshot, specCode string) (*ProvisionRequest, error)
	// FindSpec 按商品 + 规格编码取启用中的 SKU（下单/算价用，T4.1）。
	FindSpec(ctx context.Context, productID uint64, specCode string) (*dto.ProductSpecInfo, error)
	// DecrementSpecStock / IncrementSpecStock SKU 库存原子增减（下单扣减、失败回补，T4.1）。
	DecrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error)
	IncrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error)
	// ApplyConfirmedPrice 把已确认的上游成本价写入绑定该上游资源商品的售出商品（T3.4）。
	ApplyConfirmedPrice(ctx context.Context, resourceProductID uint64, costPrice float64, operatorName, remark string) (int, error)
	// ListConfigOptions 读取商品全部配置项（含 source=self，T4.4）。
	ListConfigOptions(ctx context.Context, productID uint64) ([]interface{}, error)
	// SaveConfigOptions 覆盖保存商品配置项（T4.4）：配置项可标注 source/source_key，
	// 自营项（source=self）在开通时作为平台写参数直接下发。
	SaveConfigOptions(ctx context.Context, productID uint64, groups []interface{}, operatorID uint64, operatorName string) error
}

// ProvisionRequest 订单履约时构建的上游开通请求（由订单模块消费）。
type ProvisionRequest struct {
	ProductID     uint64                 `json:"product_id"`
	ProviderID    uint64                 `json:"provider_id"`
	ProviderType  string                 `json:"provider_type"`
	Name          string                 `json:"name"`
	ConfigOptions map[string]interface{} `json:"config_options"`
	BillingMode   string                 `json:"billing_mode"`
}

// ResourceProductReader 上游资源商品读取接口（由 resource/product 仓储实现，避免包循环）。
type ResourceProductReader interface {
	FindByID(ctx context.Context, id uint64) (*resourceproductmodel.ResourceProduct, error)
}

// ProviderReader 渠道读取接口（由 resource/provider 仓储实现，避免包循环）。
// 仅用于取 provider_type，把 SKU 规格原子翻译成目标平台的写参数（T4.1）。
type ProviderReader interface {
	FindByID(ctx context.Context, id uint64) (*providermodel.ResourceProvider, error)
}

// specbindingConfirmed / specbindingAutoMapped 是 spec_bindings.status 的取值（与 spec 模型同口径）。
const (
	specbindingConfirmed  = "confirmed"
	specbindingAutoMapped = "auto_mapped"
)

// SpecBindingReader 读取 SKU 平台绑定（由 spec 契约仓储实现，T4.2）。
// 只返回基础类型，避免 catalog 包反向依赖 spec 模型。
type SpecBindingReader interface {
	// ConfirmedPlatformParamsByProductSpec 取某 SKU 已确认的出站平台参数 JSON；无绑定返回空串。
	ConfirmedPlatformParamsByProductSpec(ctx context.Context, productSpecID uint64) (string, error)
	// BindingStatusByProductSpecs 批量取 SKU 的出站绑定状态：product_spec_id → status。
	BindingStatusByProductSpecs(ctx context.Context, productSpecIDs []uint64) (map[uint64]string, error)
}

// SpecExternalBindingReader 读取上游规格绑定（代理链路上架门禁用，T4.6）。
// 由 spec 契约仓储额外实现；未装配时门禁按"未确认"处理。
type SpecExternalBindingReader interface {
	// HasConfirmedBindingForExternal 判断某上游规格（provider_type + external_id）是否已有 confirmed 绑定。
	HasConfirmedBindingForExternal(ctx context.Context, providerType, externalID string) (bool, error)
}

// UpstreamSpecRegistrar 登记上游规格快照（代理链路，T4.3）。
// 用函数类型而非接口：调用方（装配层）负责把 spec 契约服务适配成该签名，
// 避免 catalog 反向依赖 spec/dto 的快照结构。未注入时克隆流程跳过登记（不阻断）。
type UpstreamSpecRegistrar func(ctx context.Context, snap UpstreamSpecSnapshot) error

// UpstreamSpecSnapshot 上游规格快照（代理链路）：raw 原样、normalized 归一原子取值。
type UpstreamSpecSnapshot struct {
	ProviderID   uint64
	ProviderType string
	ExternalID   string
	ExternalName string
	ExternalKind string
	Raw          string
	Normalized   string
}

type productService struct {
	repo repository.ProductRepository
	// resourceReader 用于克隆商品开通时读取上游商品规格
	resourceReader ResourceProductReader
	// providerReader 用于开通时解析目标平台（SKU 规格原子→平台写字段）
	providerReader ProviderReader
	// bindingReader 用于开通时读取 SKU 已确认的平台绑定参数（T4.2）
	bindingReader SpecBindingReader
	// specRegistrar 用于克隆时登记上游规格快照（T4.3）
	specRegistrar UpstreamSpecRegistrar
}

// NewProductService 创建商品业务服务。
// readers 依次可为上游资源商品读取（resource/product 仓储）、渠道读取（resource/provider 仓储）、
// SKU 绑定读取（spec 契约仓储）、上游规格登记（spec 契约服务）；均可省略（对应能力降级为空实现）。
func NewProductService(repo repository.ProductRepository, readers ...interface{}) ProductService {
	s := &productService{repo: repo}
	for _, r := range readers {
		switch v := r.(type) {
		case ResourceProductReader:
			s.resourceReader = v
		case ProviderReader:
			s.providerReader = v
		case SpecBindingReader:
			s.bindingReader = v
		case UpstreamSpecRegistrar:
			s.specRegistrar = v
		}
	}
	return s
}

func (s *productService) List(ctx context.Context, query dto.ProductListQuery) (*dto.ProductListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.ProductInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildProductInfo(item))
	}
	return &dto.ProductListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *productService) FindByID(ctx context.Context, id uint64) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildProductInfo(*item)
	return &info, nil
}

func (s *productService) Create(ctx context.Context, req dto.ProductCreateRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	markupType, err := normalizeMarkupType(req.UpstreamMarkupType)
	if err != nil {
		return nil, err
	}
	item := &model.Product{
		Code:             strings.TrimSpace(req.Code),
		Name:             strings.TrimSpace(req.Name),
		CategoryID:       req.CategoryID,
		ProductType:      req.ProductType,
		Description:      req.Description,
		CoverImage:       req.CoverImage,
		Specs:            req.Specs,
		PriceModel:       req.PriceModel,
		Price:            req.Price,
		CostPrice:        req.CostPrice,
		SourceProductID:  req.SourceProductID,
		SourceProviderID: req.SourceProviderID,
		ConfigOptions:    req.ConfigOptions,
		Stock:            req.Stock,
		SortOrder:        req.SortOrder,
		Status:           req.Status,
		// 代理链路加价规则（T4.3）：非负；仅 clone 模式生效（上游改价时按规则重算售价）。
		UpstreamMarkupType:  markupType,
		UpstreamMarkupValue: req.UpstreamMarkupValue,
		SpecPassthrough:     req.SpecPassthrough,
	}
	if item.Stock == 0 {
		item.Stock = -1
	}
	if item.Status == 0 {
		item.Status = model.ProductStatusDraft
	}
	// 链路判据单一化（D6）：source_mode 即链路，空值归一为自营（P8 起 provision_mode 列已删除）。
	if item.SourceMode == "" {
		item.SourceMode = model.SourceModeSelf
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	// 记录创建历史
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID:    item.ID,
		ChangeType:   model.ChangeTypeCreate,
		NewValue:     item.Name,
		OperatorID:   operatorID,
		OperatorName: operatorName,
	})
	return s.FindByID(ctx, item.ID)
}

func (s *productService) Update(ctx context.Context, id uint64, req dto.ProductUpdateRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = strings.TrimSpace(req.Name)
	item.CategoryID = req.CategoryID
	item.ProductType = req.ProductType
	item.Description = req.Description
	item.CoverImage = req.CoverImage
	item.Specs = req.Specs
	item.PriceModel = req.PriceModel
	item.Price = req.Price
	item.CostPrice = req.CostPrice
	item.ConfigOptions = req.ConfigOptions
	item.Stock = req.Stock
	item.SortOrder = req.SortOrder
	item.Status = req.Status
	// 上游加价规则与透传标记（T4.3）：指针为 nil 表示未传，保持原值。
	if req.UpstreamMarkupType != nil {
		markupType, err := normalizeMarkupType(*req.UpstreamMarkupType)
		if err != nil {
			return nil, err
		}
		item.UpstreamMarkupType = markupType
	}
	if req.UpstreamMarkupValue != nil {
		if *req.UpstreamMarkupValue < 0 {
			return nil, errors.New("加价数值不能为负")
		}
		item.UpstreamMarkupValue = *req.UpstreamMarkupValue
	}
	if req.SpecPassthrough != nil {
		item.SpecPassthrough = *req.SpecPassthrough
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID:    id,
		ChangeType:   model.ChangeTypeUpdate,
		NewValue:     item.Name,
		OperatorID:   operatorID,
		OperatorName: operatorName,
	})
	return s.FindByID(ctx, id)
}

func (s *productService) Delete(ctx context.Context, id uint64, operatorID uint64, operatorName string) error {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID:    id,
		ChangeType:   model.ChangeTypeDelete,
		OldValue:     item.Name,
		OperatorID:   operatorID,
		OperatorName: operatorName,
	})
}

// Publish 上架产品：先过映射完整性门禁（T4.6），再 草稿/下架 → 上架，并记录操作历史。
func (s *productService) Publish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Status == model.ProductStatusPublished {
		return s.FindByID(ctx, id)
	}
	// 上架门禁（T4.6 / 16 §7.6）：映射不全禁止上架，避免"死功能 + 静默失败"。
	if err := s.validatePublish(ctx, item); err != nil {
		return nil, err
	}
	oldStatus := item.Status
	item.Status = model.ProductStatusPublished
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: id, ChangeType: model.ChangeTypePublish,
		OldValue: strconv.Itoa(oldStatus), NewValue: strconv.Itoa(item.Status),
		OperatorID: operatorID, OperatorName: operatorName,
	})
	return s.FindByID(ctx, id)
}

// validatePublish 上架门禁（T4.6 / 16 §7.6）：
//   - 代理链路（upstream）：必须绑定上游商品；且其上游规格已确认绑定，或显式标记"仅透传"。
//   - 自营链路且绑定了渠道：必须有可售 SKU，每个启用 SKU 都满足必填原子，且有 confirmed 平台绑定。
//   - 未绑定渠道的商品（虚拟/纯本地）不设门禁，保持存量可上架。
func (s *productService) validatePublish(ctx context.Context, item *model.Product) error {
	if s.isUpstreamChain(item) {
		if item.SourceProductID == 0 {
			return errors.New("上架失败：代理商品未绑定上游商品（source_product_id）")
		}
		if item.SpecPassthrough {
			return nil // 显式声明仅透传上游参数，放行。
		}
		if s.bindingReader == nil || !s.externalSpecConfirmed(ctx, item) {
			return errors.New("上架失败：上游规格未完成平台绑定（spec_bindings 需为 confirmed）；如确为纯透传，请在商品上开启「仅透传」")
		}
		return nil
	}
	if item.SourceProviderID == 0 {
		return nil // 无渠道绑定的自营/虚拟商品不设门禁。
	}
	specs, err := s.repo.ListSpecs(ctx, item.ID)
	if err != nil {
		return err
	}
	enabled := make([]model.ProductSpec, 0, len(specs))
	for _, sp := range specs {
		if sp.Status == model.ProductSpecEnabled {
			enabled = append(enabled, sp)
		}
	}
	if len(enabled) == 0 {
		return errors.New("上架失败：自营商品至少需要一个启用的规格（SKU）")
	}
	// 必填原子校验（spec_atoms.required）：按目标平台过滤——billing.cycle 这类
	// 由订单账期决定、在该平台无写字段的原子不参与 SKU 完整性校验；字典为空时不阻断（预埋期）。
	platform := s.provisionPlatform(ctx, item)
	for _, sp := range enabled {
		if missing := specatom.RequiredMissingFor(specFromJSONMap(sp.Specs), model.SourceModeSelf, platform); len(missing) > 0 {
			reasons := make([]string, 0, len(missing))
			for _, m := range missing {
				reasons = append(reasons, m.Error())
			}
			return fmt.Errorf("上架失败：规格 %s 缺少必填项（%s）", sp.SpecCode, strings.Join(reasons, "；"))
		}
	}
	if s.bindingReader == nil {
		return errors.New("上架失败：规格绑定能力未装配，无法校验 SKU 平台绑定")
	}
	ids := make([]uint64, 0, len(enabled))
	for _, sp := range enabled {
		ids = append(ids, sp.ID)
	}
	status, err := s.bindingReader.BindingStatusByProductSpecs(ctx, ids)
	if err != nil {
		return err
	}
	missing := make([]string, 0)
	for _, sp := range enabled {
		if status[sp.ID] != specbindingConfirmed {
			missing = append(missing, sp.SpecCode)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("上架失败：以下规格尚未确认平台绑定：%s", strings.Join(missing, "、"))
	}
	return nil
}

// isUpstreamChain 判定商品是否走代理（上游转售）链路。
// 以 source_mode 为单一判据（D6，P8 起 provision_mode 兼容列已删除）。
func (s *productService) isUpstreamChain(item *model.Product) bool {
	if item.SourceMode == "" {
		return false
	}
	return item.SourceMode == model.SourceModeUpstream
}

// externalSpecConfirmed 判断代理商品对应的上游规格是否已有 confirmed 绑定。
// 以 (provider_type, 上游商品 upstream_id) 定位 external_specs 快照。
func (s *productService) externalSpecConfirmed(ctx context.Context, item *model.Product) bool {
	if item.SourceProviderID == 0 || s.providerReader == nil {
		return false
	}
	provider, err := s.providerReader.FindByID(ctx, item.SourceProviderID)
	if err != nil {
		return false
	}
	resolver, ok := s.bindingReader.(SpecExternalBindingReader)
	if !ok {
		return false
	}
	externalID := s.sourceUpstreamID(ctx, item)
	if externalID == "" {
		return false
	}
	ok2, err := resolver.HasConfirmedBindingForExternal(ctx, provider.ProviderType, externalID)
	return err == nil && ok2
}

// sourceUpstreamID 取商品绑定上游资源商品的 upstream_id（external_specs.external_id 口径）。
func (s *productService) sourceUpstreamID(ctx context.Context, item *model.Product) string {
	if item.SourceProductID == 0 || s.resourceReader == nil {
		return ""
	}
	rp, err := s.resourceReader.FindByID(ctx, item.SourceProductID)
	if err != nil {
		return ""
	}
	return rp.UpstreamID
}

// Unpublish 下架产品：→ 下架，并记录操作历史。
func (s *productService) Unpublish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Status == model.ProductStatusOffline {
		return s.FindByID(ctx, id)
	}
	oldStatus := item.Status
	item.Status = model.ProductStatusOffline
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: id, ChangeType: model.ChangeTypeUnpublish,
		OldValue: strconv.Itoa(oldStatus), NewValue: strconv.Itoa(item.Status),
		OperatorID: operatorID, OperatorName: operatorName,
	})
	return s.FindByID(ctx, id)
}

func (s *productService) UpdatePrice(ctx context.Context, id uint64, req dto.ProductPriceRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	oldPrice := strconv.FormatFloat(item.Price, 'f', -1, 64)
	item.Price = req.Price
	item.CostPrice = req.CostPrice
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: id, ChangeType: model.ChangeTypePrice,
		OldValue: oldPrice, NewValue: strconv.FormatFloat(item.Price, 'f', -1, 64),
		OperatorID: operatorID, OperatorName: operatorName, Remark: req.Remark,
	})
	return s.FindByID(ctx, id)
}

// SetFeatured 设置/取消前台推荐位。
func (s *productService) SetFeatured(ctx context.Context, id uint64, featured bool, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Featured = featured
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: id, ChangeType: model.ChangeTypeUpdate,
		NewValue:   strconv.FormatBool(featured),
		OperatorID: operatorID, OperatorName: operatorName, Remark: "设置前台推荐位",
	})
	return s.FindByID(ctx, id)
}

func (s *productService) ListHistory(ctx context.Context, id uint64) ([]dto.ProductHistoryInfo, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.ProductHistoryInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, dto.ProductHistoryInfo{
			ID:           item.ID,
			ProductID:    item.ProductID,
			ChangeType:   item.ChangeType,
			OldValue:     item.OldValue,
			NewValue:     item.NewValue,
			OperatorName: item.OperatorName,
			Remark:       item.Remark,
			CreatedAt:    item.CreatedAt.Format(time.RFC3339),
		})
	}
	return resp, nil
}

func (s *productService) ListSpecs(ctx context.Context, id uint64) ([]dto.ProductSpecInfo, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListSpecs(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.ProductSpecInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildProductSpecInfo(item))
	}
	s.fillBindingStatus(ctx, resp)
	return resp, nil
}

// FindSpec 按商品 + 规格编码取启用中的 SKU；无 SKU 能力的商品返回 (nil, nil) 由调用方兜底。
func (s *productService) FindSpec(ctx context.Context, productID uint64, specCode string) (*dto.ProductSpecInfo, error) {
	item, err := s.repo.FindSpecByCode(ctx, productID, specCode)
	if err != nil {
		return nil, err
	}
	info := buildProductSpecInfo(*item)
	s.fillBindingStatus(ctx, []dto.ProductSpecInfo{info})
	return &info, nil
}

// fillBindingStatus 批量回填 SKU 的出站绑定状态（T4.2）；
// 读取失败时保持空串（列表仍可用，仅门禁侧会视为未绑定）。
func (s *productService) fillBindingStatus(ctx context.Context, items []dto.ProductSpecInfo) {
	if s.bindingReader == nil || len(items) == 0 {
		return
	}
	ids := make([]uint64, 0, len(items))
	for i := range items {
		if items[i].ID > 0 {
			ids = append(ids, items[i].ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	status, err := s.bindingReader.BindingStatusByProductSpecs(ctx, ids)
	if err != nil {
		return
	}
	for i := range items {
		items[i].BindingStatus = status[items[i].ID]
	}
}

// DecrementSpecStock SKU 库存原子扣减（下单用）。
func (s *productService) DecrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error) {
	return s.repo.DecrementSpecStock(ctx, specID, qty)
}

// IncrementSpecStock SKU 库存回补（下单/开通失败补偿用）。
func (s *productService) IncrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error) {
	return s.repo.IncrementSpecStock(ctx, specID, qty)
}

// buildProductSpecInfo 转换 SKU 行 → DTO。
func buildProductSpecInfo(item model.ProductSpec) dto.ProductSpecInfo {
	return dto.ProductSpecInfo{
		ID:             item.ID,
		ProductID:      item.ProductID,
		SpecCode:       item.SpecCode,
		Name:           item.Name,
		Specs:          item.Specs,
		PriceModel:     item.PriceModel,
		Price:          item.Price,
		CostPrice:      item.CostPrice,
		Stock:          item.Stock,
		SpecTemplateID: item.SpecTemplateID,
		SortOrder:      item.SortOrder,
		Status:         item.Status,
	}
}

// validateSpecJSON 校验 SKU 的 Specs JSON：必须是"原子 key → 取值"对象，
// 且取值满足 spec_atoms 字典约束（T2.6/T4.1）。字典为空时不阻断（预埋期）。
func validateSpecJSON(specs string) error {
	raw := strings.TrimSpace(specs)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return fmt.Errorf("%w：specs 必须是 JSON 对象", ErrSpecInvalid)
	}
	if issues := specatom.ValidateMap(m); len(issues) > 0 {
		parts := make([]string, 0, len(issues))
		for _, issue := range issues {
			parts = append(parts, issue.Error())
		}
		return fmt.Errorf("%w：%s", ErrSpecInvalid, strings.Join(parts, "；"))
	}
	return nil
}

// CreateSpec 新增 SKU：校验规格 JSON 与编码唯一性，并写变更历史（T4.1）。
func (s *productService) CreateSpec(ctx context.Context, productID uint64, req dto.ProductSpecRequest, operatorID uint64, operatorName string) (*dto.ProductSpecInfo, error) {
	if _, err := s.repo.FindByID(ctx, productID); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(req.SpecCode)
	if code == "" {
		return nil, errors.New("规格编码不能为空")
	}
	if _, err := s.repo.FindSpecByCode(ctx, productID, code); err == nil {
		return nil, ErrSpecCodeExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := validateSpecJSON(req.Specs); err != nil {
		return nil, err
	}
	item := &model.ProductSpec{
		ProductID:      productID,
		SpecCode:       code,
		Name:           strings.TrimSpace(req.Name),
		Specs:          req.Specs,
		PriceModel:     firstNonEmptyStr(req.PriceModel, model.PriceModelFixed),
		Price:          req.Price,
		CostPrice:      req.CostPrice,
		Stock:          req.Stock,
		SpecTemplateID: req.SpecTemplateID,
		SortOrder:      req.SortOrder,
		Status:         req.Status,
	}
	if item.Stock == 0 {
		item.Stock = -1
	}
	if item.Status == 0 {
		item.Status = model.ProductSpecEnabled
	}
	if err := s.repo.CreateSpec(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: productID, ChangeType: model.ChangeTypeUpdate,
		NewValue: item.SpecCode, OperatorID: operatorID, OperatorName: operatorName,
		Remark: "新增规格变体：" + item.Name,
	})
	info := buildProductSpecInfo(*item)
	return &info, nil
}

// UpdateSpec 更新 SKU：归属校验 → 规格 JSON 校验 → 落库（T4.1）。
func (s *productService) UpdateSpec(ctx context.Context, productID, specID uint64, req dto.ProductSpecRequest, operatorID uint64, operatorName string) (*dto.ProductSpecInfo, error) {
	item, err := s.repo.FindSpecByID(ctx, specID)
	if err != nil {
		return nil, ErrSpecNotFound
	}
	if item.ProductID != productID {
		return nil, ErrSpecNotFound
	}
	code := strings.TrimSpace(req.SpecCode)
	if code != "" && code != item.SpecCode {
		if existing, err := s.repo.FindSpecByCode(ctx, productID, code); err == nil && existing.ID != item.ID {
			return nil, ErrSpecCodeExists
		}
		item.SpecCode = code
	}
	if err := validateSpecJSON(req.Specs); err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		item.Name = name
	}
	item.Specs = req.Specs
	item.PriceModel = firstNonEmptyStr(req.PriceModel, item.PriceModel, model.PriceModelFixed)
	item.Price = req.Price
	item.CostPrice = req.CostPrice
	item.Stock = req.Stock
	item.SpecTemplateID = req.SpecTemplateID
	item.SortOrder = req.SortOrder
	item.Status = req.Status
	if err := s.repo.UpdateSpec(ctx, item); err != nil {
		return nil, err
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: productID, ChangeType: model.ChangeTypeUpdate,
		NewValue: item.SpecCode, OperatorID: operatorID, OperatorName: operatorName,
		Remark: "更新规格变体：" + item.Name,
	})
	info := buildProductSpecInfo(*item)
	return &info, nil
}

// DeleteSpec 删除 SKU（归属校验）。
func (s *productService) DeleteSpec(ctx context.Context, productID, specID uint64, operatorID uint64, operatorName string) error {
	item, err := s.repo.FindSpecByID(ctx, specID)
	if err != nil {
		return ErrSpecNotFound
	}
	if item.ProductID != productID {
		return ErrSpecNotFound
	}
	if err := s.repo.DeleteSpec(ctx, specID); err != nil {
		return err
	}
	return s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: productID, ChangeType: model.ChangeTypeUpdate,
		OldValue: item.SpecCode, OperatorID: operatorID, OperatorName: operatorName,
		Remark: "删除规格变体：" + item.Name,
	})
}

// CloneFromUpstream 从上游资源商品克隆创建销售商品（对接魔方财务商品导入）。
// 直接以 clone 模式落库，source_product_id/source_provider_id 绑定上游商品与提供商，
// 售卖时将按上游商品的规格/构型调上游开通。
func (s *productService) CloneFromUpstream(ctx context.Context, req dto.ProductCloneRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.Code)
	}
	item := &model.Product{
		Code:             strings.TrimSpace(req.Code),
		Name:             name,
		CategoryID:       req.CategoryID,
		ProductType:      "cloud_host",
		SourceProductID:  req.SourceProductID,
		SourceProviderID: req.SourceProviderID,
		SourceMode:       model.SourceModeUpstream,
		ConfigOptions:    req.ConfigOptions,
		Price:            req.Price,
		CostPrice:        req.CostPrice,
		Stock:            req.Stock,
		Status:           req.Status,
	}
	// 若未传入名称，尝试从上游资源商品补全名称与规格快照（用于列表/详情展示）。
	var cloneGroups []interface{}
	if s.resourceReader != nil && req.SourceProductID > 0 {
		if rp, err := s.resourceReader.FindByID(ctx, req.SourceProductID); err == nil {
			if item.Name == "" || item.Name == item.Code {
				item.Name = rp.Name
			}
			if item.CostPrice == 0 {
				item.CostPrice = rp.CostPrice
			}
			if item.Price == 0 {
				item.Price = rp.SalePrice
			}
			if item.Specs == "" {
				specJSON, _ := json.Marshal(buildCloneBaseOptions(rp))
				item.Specs = string(specJSON)
			}
			// 落上游 config_groups 到子表，供开通时优先读取（替代惰性解析 raw_specs）。
			cloneGroups = extractConfigGroups(rp.RawSpecs)
			// 登记上游规格快照（T4.3）：代理商品上架门禁按 (provider_type, upstream_id)
			// 回查 external_specs 是否已有 confirmed 绑定；未登记则永远无法通过门禁。
			s.registerUpstreamSpec(ctx, req.SourceProviderID, rp)
		}
	}
	if item.Stock == 0 {
		item.Stock = -1
	}
	if item.Status == 0 {
		item.Status = model.ProductStatusDraft
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	if len(cloneGroups) > 0 {
		_ = s.repo.SaveConfigOptions(ctx, item.ID, cloneGroups)
	}
	s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: item.ID, ChangeType: model.ChangeTypeCreate,
		NewValue: item.Name, OperatorID: operatorID, OperatorName: operatorName,
		Remark: "从上游商品克隆导入",
	})
	return s.FindByID(ctx, item.ID)
}

// CloneFromUpstreamBatch 批量从上游商品克隆创建销售商品（按百分比定价）。
// 每个上游商品的售价（resource_products.sale_price，即上游结算价）作为成本基线，
// 销售价 = 成本 × price_percent/100（price_percent 小于等于 0 时按 100 处理，即原价）。
func (s *productService) CloneFromUpstreamBatch(ctx context.Context, req dto.ProductBatchCloneRequest, operatorID uint64, operatorName string) ([]dto.ProductInfo, error) {
	pct := req.PricePercent
	if pct <= 0 {
		pct = 100
	}
	status := req.Status
	if status == 0 {
		status = model.ProductStatusDraft
	}
	results := make([]dto.ProductInfo, 0, len(req.SourceProductIDs))
	for _, pid := range req.SourceProductIDs {
		// 读取上游商品，取名称/规格/售价
		name := ""
		specs := ""
		cost := 0.0
		var groups []interface{}
		if s.resourceReader != nil {
			if rp, err := s.resourceReader.FindByID(ctx, pid); err == nil {
				name = rp.Name
				cost = rp.SalePrice // 上游售价作为成本基线
				specJSON, _ := json.Marshal(buildCloneBaseOptions(rp))
				specs = string(specJSON)
				groups = extractConfigGroups(rp.RawSpecs)
				// 登记上游规格快照（T4.3）：批量导入同样供上架门禁回查。
				s.registerUpstreamSpec(ctx, req.SourceProviderID, rp)
			}
		}
		item := &model.Product{
			Code:             fmt.Sprintf("cpy-%d-%d", req.SourceProviderID, pid),
			Name:             firstNonEmptyStr(name, ""),
			CategoryID:       req.CategoryID,
			ProductType:      "cloud_host",
			SourceProductID:  pid,
			SourceProviderID: req.SourceProviderID,
			SourceMode:       model.SourceModeUpstream,
			Specs:            specs,
			Price:            round2(cost * pct / 100),
			CostPrice:        round2(cost),
			Stock:            -1,
			Status:           status,
		}
		if err := s.repo.Create(ctx, item); err != nil {
			// 单个失败不阻塞整批：记录后继续
			continue
		}
		if len(groups) > 0 {
			_ = s.repo.SaveConfigOptions(ctx, item.ID, groups)
		}
		s.repo.AddHistory(ctx, &model.ProductHistory{
			ProductID: item.ID, ChangeType: model.ChangeTypeCreate,
			NewValue: item.Name, OperatorID: operatorID, OperatorName: operatorName,
			Remark: fmt.Sprintf("批量导入上游商品（定价 %.0f%%）", pct),
		})
		if info, err := s.FindByID(ctx, item.ID); err == nil {
			results = append(results, *info)
		}
	}
	return results, nil
}

// buildUpstreamSpecSnapshot 组装上游规格快照（T4.3）：
// raw 用上游原始字段快照（resource_products.raw_specs），normalized 用我方标准规格词汇
// （resource_products 已归一的 cpu/memory/... 字段 → spec_atoms 原子 key），
// 使绑定 UI 与上架门禁能直接按原子字典对齐。
func buildUpstreamSpecSnapshot(providerID uint64, providerType string, rp *resourceproductmodel.ResourceProduct) *UpstreamSpecSnapshot {
	normalized, _ := json.Marshal(specatom.ToMap(pkgmodel.StandardProductSpec{
		CPU:       rp.CPU,
		Memory:    rp.Memory,
		Disk:      rp.Disk,
		DiskType:  rp.DiskType,
		Bandwidth: rp.Bandwidth,
		OS:        rp.OS,
		Region:    rp.Region,
		Zone:      rp.Zone,
	}))
	raw := strings.TrimSpace(rp.RawSpecs)
	if raw == "" {
		raw = strings.TrimSpace(rp.Specs)
	}
	if raw == "" {
		raw = "{}"
	}
	return &UpstreamSpecSnapshot{
		ProviderID:   providerID,
		ProviderType: strings.TrimSpace(providerType),
		ExternalID:   strings.TrimSpace(rp.UpstreamID),
		ExternalName: rp.Name,
		ExternalKind: "flavor",
		Raw:          raw,
		Normalized:   string(normalized),
	}
}

// registerUpstreamSpec 登记上游规格快照（T4.3）；失败只告警不阻断克隆流程
// （登记缺失由 T4.6 上架门禁兜底拦截，不影响导入本身）。
func (s *productService) registerUpstreamSpec(ctx context.Context, providerID uint64, rp *resourceproductmodel.ResourceProduct) {
	if s.specRegistrar == nil || rp == nil || strings.TrimSpace(rp.UpstreamID) == "" {
		return
	}
	platform := ""
	if s.providerReader != nil && providerID > 0 {
		if p, err := s.providerReader.FindByID(ctx, providerID); err == nil {
			platform = p.ProviderType
		}
	}
	if platform == "" {
		return
	}
	snap := buildUpstreamSpecSnapshot(providerID, platform, rp)
	_ = s.specRegistrar(ctx, *snap)
}

// ListConfigOptions 读取商品全部配置项（含 source=self，T4.4）。
func (s *productService) ListConfigOptions(ctx context.Context, productID uint64) ([]interface{}, error) {
	if _, err := s.repo.FindByID(ctx, productID); err != nil {
		return nil, err
	}
	groups, err := s.repo.AllConfigGroupsByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []interface{}{}
	}
	return groups, nil
}

// SaveConfigOptions 覆盖保存商品配置项（T4.4）：先校验来源标注，再整表重建并留痕。
// 校验规则：source 仅接受 upstream/self；source=self 时 source_key 与 option_name 至少有一个，
// 否则该配置项在开通侧无从映射（静默失效——正是本次要消灭的问题）。
func (s *productService) SaveConfigOptions(ctx context.Context, productID uint64, groups []interface{}, operatorID uint64, operatorName string) error {
	if _, err := s.repo.FindByID(ctx, productID); err != nil {
		return err
	}
	if err := validateConfigGroupSources(groups); err != nil {
		return err
	}
	if err := s.repo.SaveConfigOptions(ctx, productID, groups); err != nil {
		return err
	}
	return s.repo.AddHistory(ctx, &model.ProductHistory{
		ProductID: productID, ChangeType: model.ChangeTypeUpdate,
		NewValue: strconv.Itoa(len(groups)), OperatorID: operatorID, OperatorName: operatorName,
		Remark: fmt.Sprintf("保存可配置项（%d 组）", len(groups)),
	})
}

// validateConfigGroupSources 校验配置项来源标注（T4.4）。
func validateConfigGroupSources(groups []interface{}) error {
	for _, raw := range groups {
		b, err := json.Marshal(raw)
		if err != nil {
			return errors.New("可配置项结构非法")
		}
		var g struct {
			Options []struct {
				OptionName string `json:"option_name"`
				Source     string `json:"source"`
				SourceKey  string `json:"source_key"`
				Sub        []struct {
					OptionName string `json:"option_name"`
					Source     string `json:"source"`
					SourceKey  string `json:"source_key"`
				} `json:"sub"`
			} `json:"options"`
		}
		if err := json.Unmarshal(b, &g); err != nil {
			return errors.New("可配置项结构非法")
		}
		for _, opt := range g.Options {
			switch strings.TrimSpace(opt.Source) {
			case "", model.ConfigSourceUpstream, model.ConfigSourceSelf:
			default:
				return fmt.Errorf("配置项 %q 的来源非法（仅支持 upstream/self）", opt.OptionName)
			}
			if opt.Source == model.ConfigSourceSelf &&
				strings.TrimSpace(opt.SourceKey) == "" && strings.TrimSpace(opt.OptionName) == "" {
				return errors.New("自营配置项必须提供平台参数名（source_key 或 option_name）")
			}
			for _, sub := range opt.Sub {
				switch strings.TrimSpace(sub.Source) {
				case "", model.ConfigSourceUpstream, model.ConfigSourceSelf:
				default:
					return fmt.Errorf("配置项 %q 的子项 %q 来源非法（仅支持 upstream/self）", opt.OptionName, sub.OptionName)
				}
			}
		}
	}
	return nil
}

// round2 金额保留两位小数。
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// normalizeMarkupType 归一上游加价规则类型（T4.3）：percent 按成本百分比、fixed 加固定额；
// 空串表示不配置规则（上游改价只更新成本、不改售价）。
func normalizeMarkupType(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "none":
		return "", nil
	case model.MarkupTypePercent, model.MarkupTypeFixed:
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", errors.New("不支持的加价类型，仅支持 percent / fixed")
	}
}

// firstNonEmptyStr 返回第一个非空字符串。
func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// BuildProvisionRequest 按商品供货模式构建上游开通请求（供订单履约联动用）。
// self 模式：SKU 已确认的平台绑定 + 商品 ConfigOptions + SKU 规格原子（按平台字段字典翻译）作基础；
// clone 模式：读取 source_product_id 对应的上游资源商品规格作为基础参数
// （cpu/memory/system_disk_size/os/area 等），再用本商品 ConfigOptions 覆盖补全。
// specCode 非空时读取该 SKU 的 confirmed 平台绑定（T4.2），作为最高优先级的写参数。
func (s *productService) BuildProvisionRequest(ctx context.Context, productID uint64, name, specSnapshot, specCode string) (*ProvisionRequest, error) {
	item, err := s.repo.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	req := &ProvisionRequest{
		ProductID:   item.ID,
		ProviderID:  item.SourceProviderID,
		Name:        name,
		BillingMode: item.PriceModel,
	}
	// 自定义可配置项作基础（可能为空）
	opts := map[string]interface{}{}
	if item.SourceMode == model.SourceModeUpstream {
		// 代理商品：从上游资源商品取规格。source_product_id 缺失回退为空，交由适配器兜底。
		if item.SourceProductID > 0 && s.resourceReader != nil {
			rp, err := s.resourceReader.FindByID(ctx, item.SourceProductID)
			if err == nil {
				opts = buildCloneBaseOptions(rp)
				// 财务型上游下单需要：上游商品ID(upstream_pid) 与 上游配置组(config_groups 含选项/子项 upstream_id)
				if rp.UpstreamID != "" {
					opts["upstream_pid"] = rp.UpstreamID
				}
				// 优先读取子表（克隆时已落库的 config_groups），缺失回退惰性解析 raw_specs（Strangler-fig）。
				if groups, gerr := s.repo.ConfigGroupsByProductID(ctx, item.ID); gerr == nil && len(groups) > 0 {
					opts["config_groups"] = groups
				} else if groups := extractConfigGroups(rp.RawSpecs); len(groups) > 0 {
					opts["config_groups"] = groups
				}
			}
		}
	}
	// SKU 规格原子基线（T4.1）：自营商品按目标平台字段字典翻译成写参数，
	// 作为开通基础参数；上方 clone 基线与此互斥（source_mode 单一判据）。
	// 优先级：商品 ConfigOptions > SKU 原子 > 上游基线，故原子在 ConfigOptions 之前合并。
	if specSnapshot = strings.TrimSpace(specSnapshot); specSnapshot != "" {
		if platform := s.provisionPlatform(ctx, item); platform != "" {
			for k, v := range specatom.WriteParams(platform, specFromJSON(specSnapshot)) {
				opts[k] = v
			}
		}
	}
	// 商品自身可配置项覆盖（补全/改写上游规格键）
	for k, v := range parseConfigOptions(item.ConfigOptions) {
		opts[k] = v
	}
	// 自营配置项（source=self，T4.4）：平台参数名 → 选中值直接透传。
	for k, v := range s.selfConfigParams(ctx, item.ID) {
		opts[k] = v
	}
	// SKU 已确认平台绑定（T4.2）：schema 与 area/node/store/flavor 等最终参数，
	// 优先级最高（人工确认过，覆盖上面所有推导值）。
	if code := strings.TrimSpace(specCode); code != "" && s.bindingReader != nil {
		if spec, serr := s.repo.FindSpecByCode(ctx, item.ID, code); serr == nil && spec != nil {
			if raw, berr := s.bindingReader.ConfirmedPlatformParamsByProductSpec(ctx, spec.ID); berr == nil && raw != "" {
				for k, v := range parseConfigOptions(raw) {
					opts[k] = v
				}
			}
		}
	}
	req.ConfigOptions = opts
	return req, nil
}

// selfConfigParams 读取商品的自营配置项（source=self）；失败返回 nil（不阻断开通）。
func (s *productService) selfConfigParams(ctx context.Context, productID uint64) map[string]string {
	params, err := s.repo.SelfConfigParams(ctx, productID)
	if err != nil {
		return nil
	}
	return params
}

// provisionPlatform 返回商品开通目标平台的 provider_type（用于规格原子→平台写字段翻译）。
func (s *productService) provisionPlatform(ctx context.Context, item *model.Product) string {
	if item.SourceProviderID == 0 || s.providerReader == nil {
		return ""
	}
	p, err := s.providerReader.FindByID(ctx, item.SourceProviderID)
	if err != nil {
		return ""
	}
	return p.ProviderType
}

// specFromJSON 把 SKU 的规格快照 JSON 解析为标准规格；解析失败返回空规格（不阻断开通）。
func specFromJSON(raw string) pkgmodel.StandardProductSpec {
	return specatom.FromMap(specFromJSONMap(raw))
}

// specFromJSONMap 把 SKU 的规格快照 JSON 解析为"原子 key → 取值"映射（校验/门禁用）。
func specFromJSONMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

// buildCloneBaseOptions 将上游资源商品规格映射为开通参数（适配 mofangyun /clouds 及通用开通）。
// 字段语义对齐魔方云配置项：cpu/memory(MB)/system_disk_size(GB)/os/area(区域)/bw(带宽)。
func buildCloneBaseOptions(rp *resourceproductmodel.ResourceProduct) map[string]interface{} {
	opts := map[string]interface{}{}
	if rp.CPU > 0 {
		opts["cpu"] = rp.CPU
	}
	if rp.Memory > 0 {
		opts["memory"] = rp.Memory
	}
	if rp.Disk > 0 {
		opts["system_disk_size"] = rp.Disk
	}
	if rp.DiskType != "" {
		opts["disk_type"] = rp.DiskType
	}
	if rp.Bandwidth > 0 {
		opts["bw"] = rp.Bandwidth
	}
	if rp.OS != "" {
		opts["os"] = rp.OS
	}
	if rp.Region != "" {
		opts["area"] = rp.Region
	}
	if rp.Zone != "" {
		opts["node"] = rp.Zone
	}
	// 上游标准化规格 JSON 若有更多细节，合并进去（不覆盖上面的显式键）
	mergeSpecJSON(opts, rp.Specs)
	return opts
}

// mergeSpecJSON 将规格 JSON 中尚未在 opts 出现的键合并进 opts。
func mergeSpecJSON(opts map[string]interface{}, specJSON string) {
	if strings.TrimSpace(specJSON) == "" {
		return
	}
	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		return
	}
	for k, v := range spec {
		if _, exists := opts[k]; !exists && v != nil {
			opts[k] = v
		}
	}
}

// parseConfigOptions 解析商品可配置项 JSON；失败时返回空 map（由适配器兜底）。
func parseConfigOptions(s string) map[string]interface{} {
	if strings.TrimSpace(s) == "" {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

// extractConfigGroups 从上游资源商品的 RawSpecs（JSON）中提取 config_groups 数组，
// 供财务型上游下单时解析出配置项/子项的 upstream_id。
func extractConfigGroups(rawSpecs string) []interface{} {
	if strings.TrimSpace(rawSpecs) == "" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(rawSpecs), &m); err != nil {
		return nil
	}
	raw, ok := m["config_groups"]
	if !ok || raw == nil {
		return nil
	}
	if arr, ok := raw.([]interface{}); ok {
		return arr
	}
	return nil
}

func buildProductInfo(item model.Product) dto.ProductInfo {
	return dto.ProductInfo{
		ID:                  item.ID,
		Code:                item.Code,
		Name:                item.Name,
		CategoryID:          item.CategoryID,
		ProductType:         item.ProductType,
		Description:         item.Description,
		CoverImage:          item.CoverImage,
		Specs:               item.Specs,
		PriceModel:          item.PriceModel,
		Price:               item.Price,
		CostPrice:           item.CostPrice,
		SourceProductID:     item.SourceProductID,
		SourceProviderID:    item.SourceProviderID,
		SourceMode:          item.SourceMode,
		ConfigOptions:       item.ConfigOptions,
		UpstreamMarkupType:  item.UpstreamMarkupType,
		UpstreamMarkupValue: item.UpstreamMarkupValue,
		SpecPassthrough:     item.SpecPassthrough,
		Featured:            item.Featured,
		Stock:               item.Stock,
		SortOrder:           item.SortOrder,
		Status:              item.Status,
		CreatedAt:           item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           item.UpdatedAt.Format(time.RFC3339),
	}
}
