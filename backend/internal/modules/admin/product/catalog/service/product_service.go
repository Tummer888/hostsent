package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
	"hostsent/backend/internal/modules/admin/product/catalog/repository"
	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
)

// 业务错误
var (
	// ErrProductInvalidStatus 非法状态流转
	ErrProductInvalidStatus = errors.New("非法的产品状态流转")
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
	// CloneFromUpstream 从上游资源商品克隆创建销售商品（对接魔方财务商品导入）
	CloneFromUpstream(ctx context.Context, req dto.ProductCloneRequest, operatorID uint64, operatorName string) (*dto.ProductInfo, error)
	// CloneFromUpstreamBatch 批量从上游商品克隆创建销售商品（按百分比定价）
	CloneFromUpstreamBatch(ctx context.Context, req dto.ProductBatchCloneRequest, operatorID uint64, operatorName string) ([]dto.ProductInfo, error)
	// BuildProvisionRequest 按商品供货模式构建上游开通请求（供订单履约联动用）
	BuildProvisionRequest(ctx context.Context, productID uint64, name string) (*ProvisionRequest, error)
}

// ProvisionRequest 订单履约时构建的上游开通请求（由订单模块消费）。
type ProvisionRequest struct {
	ProductID     uint64                 `json:"product_id"`
	ProviderID    uint64                 `json:"provider_id"`
	ProviderType  string                 `json:"provider_type"`
	ProvisionMode string                 `json:"provision_mode"`
	Name          string                 `json:"name"`
	ConfigOptions map[string]interface{} `json:"config_options"`
	BillingMode   string                 `json:"billing_mode"`
}

// ResourceProductReader 上游资源商品读取接口（由 resource/product 仓储实现，避免包循环）。
type ResourceProductReader interface {
	FindByID(ctx context.Context, id uint64) (*resourceproductmodel.ResourceProduct, error)
}

type productService struct {
	repo repository.ProductRepository
	// resourceReader 用于克隆商品开通时读取上游商品规格
	resourceReader ResourceProductReader
}

// NewProductService 创建商品业务服务。
func NewProductService(repo repository.ProductRepository, readers ...ResourceProductReader) ProductService {
	s := &productService{repo: repo}
	if len(readers) > 0 {
		s.resourceReader = readers[0]
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
	item := &model.Product{
		Code:             strings.TrimSpace(req.Code),
		Name:             strings.TrimSpace(req.Name),
		CategoryID:       req.CategoryID,
		ProductType:      req.ProductType,
		Description:      req.Description,
		Specs:            req.Specs,
		PriceModel:       req.PriceModel,
		Price:            req.Price,
		CostPrice:        req.CostPrice,
		SourceProductID:  req.SourceProductID,
		SourceProviderID: req.SourceProviderID,
		ProvisionMode:    req.ProvisionMode,
		ConfigOptions:    req.ConfigOptions,
		Stock:            req.Stock,
		SortOrder:        req.SortOrder,
		Status:           req.Status,
	}
	if item.Stock == 0 {
		item.Stock = -1
	}
	if item.Status == 0 {
		item.Status = model.ProductStatusDraft
	}
	if item.ProvisionMode == "" {
		item.ProvisionMode = model.ProvisionModeSelf
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
	item.Specs = req.Specs
	item.PriceModel = req.PriceModel
	item.Price = req.Price
	item.CostPrice = req.CostPrice
	item.ConfigOptions = req.ConfigOptions
	item.Stock = req.Stock
	item.SortOrder = req.SortOrder
	item.Status = req.Status
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

// Publish 上架产品：草稿/下架 → 上架，并记录操作历史。
func (s *productService) Publish(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Status == model.ProductStatusPublished {
		return s.FindByID(ctx, id)
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
		resp = append(resp, dto.ProductSpecInfo{
			ID:         item.ID,
			ProductID:  item.ProductID,
			SpecCode:   item.SpecCode,
			Name:       item.Name,
			Specs:      item.Specs,
			PriceModel: item.PriceModel,
			Price:      item.Price,
			CostPrice:  item.CostPrice,
			Stock:      item.Stock,
			SortOrder:  item.SortOrder,
			Status:     item.Status,
		})
	}
	return resp, nil
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
		ProvisionMode:    model.ProvisionModeClone,
		ConfigOptions:    req.ConfigOptions,
		Price:            req.Price,
		CostPrice:        req.CostPrice,
		Stock:            req.Stock,
		Status:           req.Status,
	}
	// 若未传入名称，尝试从上游资源商品补全名称与规格快照（用于列表/详情展示）。
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
		if s.resourceReader != nil {
			if rp, err := s.resourceReader.FindByID(ctx, pid); err == nil {
				name = rp.Name
				cost = rp.SalePrice // 上游售价作为成本基线
				specJSON, _ := json.Marshal(buildCloneBaseOptions(rp))
				specs = string(specJSON)
			}
		}
		item := &model.Product{
			Code:             fmt.Sprintf("cpy-%d-%d", req.SourceProviderID, pid),
			Name:             firstNonEmptyStr(name, ""),
			CategoryID:       req.CategoryID,
			ProductType:      "cloud_host",
			SourceProductID:  pid,
			SourceProviderID: req.SourceProviderID,
			ProvisionMode:    model.ProvisionModeClone,
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

// round2 金额保留两位小数。
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
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
// self 模式：直接使用商品自身 ConfigOptions；
// clone 模式：读取 source_product_id 对应的上游资源商品规格作为基础参数
// （cpu/memory/system_disk_size/os/area 等），再用本商品 ConfigOptions 覆盖补全。
func (s *productService) BuildProvisionRequest(ctx context.Context, productID uint64, name string) (*ProvisionRequest, error) {
	item, err := s.repo.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if item.ProvisionMode == "" {
		item.ProvisionMode = model.ProvisionModeSelf
	}
	req := &ProvisionRequest{
		ProductID:     item.ID,
		ProviderID:    item.SourceProviderID,
		ProvisionMode: item.ProvisionMode,
		Name:          name,
		BillingMode:   item.PriceModel,
	}
	// 自定义可配置项作基础（可能为空）
	opts := map[string]interface{}{}
	if item.ProvisionMode == model.ProvisionModeClone {
		// 克隆模式：从上游资源商品取规格。source_product_id 缺失回退为空，交由适配器兜底。
		if item.SourceProductID > 0 && s.resourceReader != nil {
			rp, err := s.resourceReader.FindByID(ctx, item.SourceProductID)
			if err == nil {
				opts = buildCloneBaseOptions(rp)
			}
		}
	}
	// 商品自身可配置项覆盖（补全/改写上游规格键）
	for k, v := range parseConfigOptions(item.ConfigOptions) {
		opts[k] = v
	}
	req.ConfigOptions = opts
	return req, nil
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

func buildProductInfo(item model.Product) dto.ProductInfo {
	return dto.ProductInfo{
		ID:               item.ID,
		Code:             item.Code,
		Name:             item.Name,
		CategoryID:       item.CategoryID,
		ProductType:      item.ProductType,
		Description:      item.Description,
		Specs:            item.Specs,
		PriceModel:       item.PriceModel,
		Price:            item.Price,
		CostPrice:        item.CostPrice,
		SourceProductID:  item.SourceProductID,
		SourceProviderID: item.SourceProviderID,
		ProvisionMode:    item.ProvisionMode,
		ConfigOptions:    item.ConfigOptions,
		Featured:         item.Featured,
		Stock:            item.Stock,
		SortOrder:        item.SortOrder,
		Status:           item.Status,
		CreatedAt:        item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.Format(time.RFC3339),
	}
}
