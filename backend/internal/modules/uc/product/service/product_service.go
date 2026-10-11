// Package service 提供用户中心商品模块业务编排（复用管理端商品目录，仅暴露上架商品）。
package service

import (
	"context"
	"errors"
	"strings"

	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"

	"hostsent/backend/internal/modules/uc/product/dto"
)

// ErrProductOffline 商品未上架，不可购买。
var ErrProductOffline = errors.New("商品已下架，不可购买")

// catalogReader 管理端商品目录读取能力的最小暴露接口（由装配层注入，避免 uc 依赖 admin service 层）。
type catalogReader interface {
	List(ctx context.Context, query catalogdto.ProductListQuery) (*catalogdto.ProductListResponse, error)
	FindByID(ctx context.Context, id uint64) (*catalogdto.ProductInfo, error)
	// ListSpecs 商品下的 SKU 列表（T4.1）：详情页展示可售规格。
	ListSpecs(ctx context.Context, productID uint64) ([]catalogdto.ProductSpecInfo, error)
	// CustomerOptions 商品对客户开放的选配项（T4.5）：详情页渲染选配控件。
	CustomerOptions(ctx context.Context, productID uint64) ([]catalogservice.CustomerOptionGroup, error)
}

// cycleReader 周期价格矩阵读取能力（doc25）：商品级与 SKU 级可售周期。
// 由装配层注入价格矩阵服务适配器；未注入时商品详情不下发周期（Skus[].Cycles 为空）。
type cycleReader interface {
	SellableCycles(ctx context.Context, productID uint64) (product []string, bySpec map[uint64][]string, err error)
}

// ProductService 用户中心商品业务能力。
type ProductService interface {
	// List 上架商品列表（status=published），仅返回用户可见字段。
	List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error)
	// Get 商品详情（仅上架商品可购）。
	Get(ctx context.Context, id uint64) (*dto.ProductInfo, error)
}

type productService struct {
	catalog catalogReader
	cycles  cycleReader
}

// NewProductService 创建用户中心商品服务；cycles 可为 nil（商品详情不含周期选择）。
func NewProductService(catalog catalogReader, cycles cycleReader) ProductService {
	return &productService{catalog: catalog, cycles: cycles}
}

func statusPtr(v int) *int { return &v }

func (s *productService) List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 12
	}
	resp, err := s.catalog.List(ctx, catalogdto.ProductListQuery{
		Keyword:    query.Keyword,
		CategoryID: query.Category,
		Status:     statusPtr(1), // 仅上架
		Featured:   query.Featured,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]dto.ProductInfo, 0, len(resp.Items))
	for _, it := range resp.Items {
		items = append(items, fromAdmin(it))
	}
	return &dto.ListResponse{Items: items, Page: page, PageSize: pageSize, Total: resp.Meta.Total}, nil
}

func (s *productService) Get(ctx context.Context, id uint64) (*dto.ProductInfo, error) {
	item, err := s.catalog.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 仅上架商品可购
	if item.Status != 1 {
		return nil, ErrProductOffline
	}
	info := fromAdmin(*item)
	// 对接链路信息（配置箱只读展示 + 可续费提示）：只透出展示口径，代理价/成本不外泄。
	info.Upstream = upstreamChainFor(item.Upstream)
	// 周期矩阵（doc25）：商品级可售周期 + 各 SKU 自有周期，供用户端渲染周期选择。
	var productCycles []string
	cyclesBySpec := map[uint64][]string{}
	if s.cycles != nil {
		if pc, bySpec, err := s.cycles.SellableCycles(ctx, id); err == nil {
			productCycles = pc
			cyclesBySpec = bySpec
		}
	}
	info.Cycles = productCycles
	// SKU 矩阵（T4.1）：详情页展示可售规格，供用户选择后按 spec_code 下单。
	if specs, err := s.catalog.ListSpecs(ctx, id); err == nil {
		for _, sp := range specs {
			if sp.Status != 1 {
				continue
			}
			// SKU 自有周期优先；没有自有行则回落商品级周期（矩阵按 SKU 建行时才区分）。
			cycles := cyclesBySpec[sp.ID]
			if len(cycles) == 0 {
				cycles = productCycles
			}
			info.Skus = append(info.Skus, dto.SkuInfo{
				SpecCode:   sp.SpecCode,
				Name:       sp.Name,
				Specs:      sp.Specs,
				Price:      sp.Price,
				PriceModel: sp.PriceModel,
				Stock:      sp.Stock,
				Cycles:     cycles,
			})
		}
	}
	// 客户可选配置项（T4.5）：档位之外可自选的参数，前端据此渲染控件并算加价。
	if groups, err := s.catalog.CustomerOptions(ctx, id); err == nil {
		for _, g := range groups {
			if len(g.Options) == 0 {
				continue
			}
			group := dto.OptionGroup{Options: make([]dto.Option, 0, len(g.Options))}
			for _, o := range g.Options {
				opt := dto.Option{
					OptionKey:    o.OptionKey,
					Name:         o.Name,
					Widget:       o.Widget,
					Required:     o.Required,
					DefaultValue: o.DefaultValue,
					Unit:         o.Unit,
					Help:         o.Help,
					MinValue:     o.MinValue,
					MaxValue:     o.MaxValue,
					UnitPrice:    o.UnitPrice,
				}
				for _, v := range o.Values {
					opt.Values = append(opt.Values, dto.OptionItem{
						Value:          v.Value,
						Label:          v.Label,
						GroupLabel:     v.Group,
						Help:           v.Help,
						Default:        v.Default,
						PriceMonthly:   v.PriceMonthly,
						PriceQuarterly: v.PriceQuarterly,
						PriceAnnually:  v.PriceAnnually,
						PriceOnetime:   v.PriceOnetime,
					})
				}
				group.Options = append(group.Options, opt)
			}
			info.OptionGroups = append(info.OptionGroups, group)
		}
	}
	return &info, nil
}

// fromAdmin 将管理端商品信息映射为用户可见字段（剔除成本价等）。
func fromAdmin(it catalogdto.ProductInfo) dto.ProductInfo {
	return dto.ProductInfo{
		ID:            it.ID,
		Name:          it.Name,
		Description:   it.Description,
		CoverImage:    it.CoverImage,
		CategoryID:    it.CategoryID,
		ProductType:   it.ProductType,
		Price:         it.Price,
		PriceModel:    it.PriceModel,
		Specs:         it.Specs,
		SourceMode:    it.SourceMode,
		ConfigOptions: it.ConfigOptions,
		Featured:      it.Featured,
		CreatedAt:     it.CreatedAt,
	}
}

// upstreamChainFor 把管理端的上游对接信息裁剪为客户可见版本：
// 保留「是否固定配置 / 是否可续费 / 各配置项锁定状态 / 下单说明」，
// 剔除上游标价、代理价、折扣与渠道等内部口径（客户看到的只有本平台的售价）。
func upstreamChainFor(up *catalogdto.ProductUpstreamInfo) *dto.UpstreamChain {
	if up == nil {
		return nil
	}
	chain := &dto.UpstreamChain{
		FixedConfig: up.FixedConfig,
		Renewable:   up.Renewable,
		RenewNote:   up.RenewNote,
		OrderNotes: []string{
			"主机名可自定义（留空由系统自动生成）",
			"初始密码由系统随机生成，开通后可在主机详情重置",
			"端口由上游随机分配，开通后随主机下发",
		},
	}
	for _, opt := range up.Options {
		chain.Options = append(chain.Options, dto.UpstreamChainOption{
			Key:    opt.Key,
			Name:   upstreamOptionLabel(opt.OptionName, opt.Key),
			Locked: opt.Locked,
			Value:  opt.Value,
		})
	}
	return chain
}

// upstreamOptionLabel 取配置项的中文名：上游下发形如「cpu|CPU」，取竖线后的说明部分。
func upstreamOptionLabel(optionName, key string) string {
	name := strings.TrimSpace(optionName)
	if idx := strings.Index(name, "|"); idx >= 0 {
		if label := strings.TrimSpace(name[idx+1:]); label != "" {
			return label
		}
	}
	if name != "" {
		return name
	}
	return key
}
