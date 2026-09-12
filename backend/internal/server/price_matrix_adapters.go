package server

// 本文件把「周期价格矩阵」服务所需的两项只读依赖适配为最小接口
// （pricing/service 侧不反向依赖 catalog/provider 模块，见 doc25 §3）。

import (
	"context"

	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	pricingdto "hostsent/backend/internal/modules/admin/product/pricing/dto"
	matrixservice "hostsent/backend/internal/modules/admin/product/pricing/service"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
)

// productMetaReader 用商品目录服务实现周期价格矩阵的商品元信息读取。
type productMetaReader struct {
	catalog catalogservice.ProductService
}

// NewProductMetaReader 创建商品元信息读取适配器。
func NewProductMetaReader(catalog catalogservice.ProductService) matrixservice.ProductMetaReader {
	return &productMetaReader{catalog: catalog}
}

func (r *productMetaReader) ProductMeta(ctx context.Context, productID uint64) (*matrixservice.MatrixProductMeta, error) {
	item, err := r.catalog.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return &matrixservice.MatrixProductMeta{
		ID:               item.ID,
		Name:             item.Name,
		SourceMode:       item.SourceMode,
		SourceProviderID: item.SourceProviderID,
		PriceModel:       item.PriceModel,
		Price:            item.Price,
		CostPrice:        item.CostPrice,
	}, nil
}

// cycleReaderAdapter 把价格矩阵服务适配为 uc 商品服务的周期读取接口。
type cycleReaderAdapter struct {
	matrix matrixservice.PriceMatrixService
}

// NewUCCycleReader 创建用户端商品详情的周期读取适配器。
func NewUCCycleReader(matrix matrixservice.PriceMatrixService) *cycleReaderAdapter {
	return &cycleReaderAdapter{matrix: matrix}
}

func (a *cycleReaderAdapter) SellableCycles(ctx context.Context, productID uint64) ([]string, map[uint64][]string, error) {
	return a.matrix.SellableCycles(ctx, productID)
}

// cyclePriceWriterAdapter 把价格矩阵服务适配为 catalog 的周期价格写入接口
// （上游调价确认后按周期重算矩阵，doc25 §5）。
type cyclePriceWriterAdapter struct {
	matrix matrixservice.PriceMatrixService
}

// NewCyclePriceWriter 创建周期价格矩阵写入适配器。
func NewCyclePriceWriter(matrix matrixservice.PriceMatrixService) catalogservice.CyclePriceWriter {
	return &cyclePriceWriterAdapter{matrix: matrix}
}

func (a *cyclePriceWriterAdapter) ApplyUpstreamPrices(ctx context.Context, productID uint64, rows []catalogservice.CyclePriceRow) error {
	converted := make([]pricingdto.CyclePriceRow, 0, len(rows))
	for _, r := range rows {
		converted = append(converted, pricingdto.CyclePriceRow{
			Cycle:        r.Cycle,
			Price:        r.Price,
			CostPrice:    r.CostPrice,
			SetupFee:     r.SetupFee,
			CostSetupFee: r.CostSetupFee,
			Source:       r.Source,
		})
	}
	return a.matrix.ApplyUpstreamPrices(ctx, productID, converted)
}

// providerCyclesReader 用渠道服务实现「渠道可提供的计费周期」读取。
type providerCyclesReader struct {
	provider providerservice.ProviderService
}

// NewProviderCyclesReader 创建渠道周期读取适配器。
func NewProviderCyclesReader(provider providerservice.ProviderService) matrixservice.ProviderCyclesReader {
	return &providerCyclesReader{provider: provider}
}

func (r *providerCyclesReader) BillingCycles(ctx context.Context, providerID uint64) ([]string, error) {
	info, err := r.provider.FindByID(ctx, providerID)
	if err != nil || info == nil {
		return nil, err
	}
	// 渠道可售周期以适配器能力描述符为准（provider_types 落库兜底）。
	return info.Capabilities.BillingCycles, nil
}
