// Package service 提供上游商品管理模块的业务编排。
//
// 命名澄清：本模块管理「上游资源商品」（resource_products 表，来自供应商
// 同步镜像，含成本价/同步状态），与 admin/product/catalog 的「终端销售商品」
// （products 表，含定价/规格/SKU）是两个域——后者才是面向客户售卖的商品目录。
// 两者的关联经 catalog.Product.SourceProductID/SourceProviderID 建立引用。
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/resource/product/dto"
	"hostsent/backend/internal/modules/admin/resource/product/model"
	"hostsent/backend/internal/modules/admin/resource/product/repository"
)

// ProductService 上游商品业务能力
type ProductService interface {
	List(ctx context.Context, query dto.ProductListQuery) (*dto.ProductListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.ProductInfo, error)
	UpdatePrice(ctx context.Context, id uint64, req dto.ProductPriceRequest) (*dto.ProductInfo, error)
	// Sync 拉取上游商品（骨架占位，具体同步逻辑在同步引擎阶段实现）
	Sync(ctx context.Context, providerID uint64) (*dto.SyncResult, error)
	// SetImportedReader 注入「已导入商品」查询端口（装配层调用，标注已导入用）。
	SetImportedReader(reader ImportedProductReader)
}

// ImportedProductRef 已导入的本地商品引用（标注「已导入」用，不依赖商品域模型）。
type ImportedProductRef struct {
	ProductID uint64
	Status    int
}

// ImportedProductReader 已导入引用读取端口：由商品域（catalog 仓库）实现并在装配层注入，
// 避免资源域反向依赖商品域。
type ImportedProductReader interface {
	ListImportedRefs(ctx context.Context, resourceProductIDs []uint64) (map[uint64]ImportedProductRef, error)
}

type productService struct {
	repo repository.ProductRepository
	// importedReader 可为空（未装配时前端按"未导入"展示，不阻塞列表）。
	importedReader ImportedProductReader
}

// NewProductService 创建上游商品业务服务
func NewProductService(repo repository.ProductRepository) ProductService {
	return &productService{repo: repo}
}

// SetImportedReader 注入「已导入商品」查询端口（装配层调用）。
func (s *productService) SetImportedReader(reader ImportedProductReader) {
	s.importedReader = reader
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
	// 一次性批量查「已导入」引用：上游商品列表动辄几百条，逐条查商品域会打成 N+1。
	imported := map[uint64]ImportedProductRef{}
	if s.importedReader != nil && len(items) > 0 {
		ids := make([]uint64, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		if refs, err := s.importedReader.ListImportedRefs(ctx, ids); err == nil {
			imported = refs
		}
	}
	respItems := make([]dto.ProductInfo, 0, len(items))
	for _, item := range items {
		// 列表不带 raw_specs：上游商品是非结构化正文（config_groups/描述/双价周期），
		// 单条 10KB 量级，全量列表（245 条）会白白放大 3MB+；详情接口按需取。
		// 但列表要看清分类/价格/在售状态，先把 raw_specs 里的元信息摘出来再清空。
		info := buildProductInfo(item)
		info.RawSpecs = ""
		if ref, ok := imported[item.ID]; ok {
			info.Imported = true
			info.ImportedProductID = ref.ProductID
			info.ImportedStatus = ref.Status
		}
		respItems = append(respItems, info)
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

func (s *productService) UpdatePrice(ctx context.Context, id uint64, req dto.ProductPriceRequest) (*dto.ProductInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.CostPrice = req.CostPrice
	item.SalePrice = req.SalePrice
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *productService) Sync(ctx context.Context, providerID uint64) (*dto.SyncResult, error) {
	return &dto.SyncResult{TaskID: 0, Status: "skipped", Message: "商品同步将在同步引擎阶段实现"}, nil
}

// upstreamSpecMeta raw_specs 中供列表页使用的元信息（分类来源/在售判定/价格来源/前台目录价）。
type upstreamSpecMeta struct {
	GroupNameSource string `json:"group_name_source"`
	Visibility      string `json:"visibility"`
	PriceSource     string `json:"price_source"`
	Catalog         struct {
		Price        flexNum `json:"price"`
		SalePrice    flexNum `json:"sale_price"`
		BillingCycle string  `json:"billing_cycle"`
	} `json:"catalog"`
}

// flexNum 兼容上游把价格回成字符串或数字。
type flexNum float64

// UnmarshalJSON 数字/字符串/空值一律解析为 float64；非法值按 0 处理。
func (f *flexNum) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		*f = 0
		return nil
	}
	if err := json.Unmarshal([]byte(raw), (*float64)(f)); err != nil {
		*f = 0
		return nil
	}
	return nil
}

// parseUpstreamSpecMeta 解析 raw_specs 的列表页元信息；解析失败返回零值（前端按未知处理）。
func parseUpstreamSpecMeta(rawSpecs string) upstreamSpecMeta {
	var meta upstreamSpecMeta
	if strings.TrimSpace(rawSpecs) == "" {
		return meta
	}
	_ = json.Unmarshal([]byte(rawSpecs), &meta)
	return meta
}

// buildProductInfo 组装商品 DTO：raw_specs 原样带上（详情接口展示），
// 同时把其中的元信息摘成独立字段（列表接口随后会清空 raw_specs）。
func buildProductInfo(item model.ResourceProduct) dto.ProductInfo {
	meta := parseUpstreamSpecMeta(item.RawSpecs)
	return dto.ProductInfo{
		ID:               item.ID,
		ProviderID:       item.ProviderID,
		UpstreamID:       item.UpstreamID,
		Name:             item.Name,
		GroupID:          item.GroupID,
		GroupName:        item.GroupName,
		GroupNameDerived: meta.GroupNameSource == "derived",
		CPU:              item.CPU,
		Memory:           item.Memory,
		Disk:             item.Disk,
		DiskType:         item.DiskType,
		Bandwidth:        item.Bandwidth,
		OS:               item.OS,
		Region:           item.Region,
		Zone:             item.Zone,
		Specs:            item.Specs,
		RawSpecs:         item.RawSpecs,
		CostPrice:        item.CostPrice,
		SalePrice:        item.SalePrice,
		Status:           item.Status,
		Visibility:       meta.Visibility,
		PriceSource:      meta.PriceSource,
		CatalogPrice:     float64(meta.Catalog.Price),
		CatalogSalePrice: float64(meta.Catalog.SalePrice),
		CatalogCycle:     meta.Catalog.BillingCycle,
		CreatedAt:        item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.Format(time.RFC3339),
	}
}
