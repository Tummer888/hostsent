package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"hostsent/backend/internal/modules/admin/product/pricing/dto"
	"hostsent/backend/internal/modules/admin/product/pricing/model"
	"hostsent/backend/internal/modules/admin/product/pricing/repository"
	"hostsent/backend/internal/pkg/billingcycle"
)

// ============================================================================
// 周期价格矩阵（doc25）：商品 × 规格 × 周期 的价格与启停。
// 与 PriceMatrixRepository 的区别：本层负责周期合法性、上游周期约束、
// 上游派生行只读等业务规则；仓储只管读写。
// ============================================================================

// MatrixProductMeta 矩阵所需的最小商品元信息（由 catalog 侧适配注入，避免包循环）。
type MatrixProductMeta struct {
	ID               uint64
	Name             string
	SourceMode       string // self / upstream
	SourceProviderID uint64
	PriceModel       string
	Price            float64
	CostPrice        float64
}

// ProductMetaReader 商品元信息读取（实现方：catalog 仓储适配）。
type ProductMetaReader interface {
	ProductMeta(ctx context.Context, productID uint64) (*MatrixProductMeta, error)
}

// ProviderCyclesReader 渠道可提供的计费周期（实现方：provider 服务，读 CapabilityDescriptor.BillingCycles）。
type ProviderCyclesReader interface {
	BillingCycles(ctx context.Context, providerID uint64) ([]string, error)
}

// PriceMatrixService 周期价格矩阵业务能力。
type PriceMatrixService interface {
	// Matrix 取商品（或指定 SKU）的周期价格矩阵；上游商品附带渠道可用周期。
	Matrix(ctx context.Context, query dto.PriceMatrixQuery) (*dto.PriceMatrixResponse, error)
	// SaveMatrix 整表保存：upsert 提交项 + 删除未提交档位。
	SaveMatrix(ctx context.Context, req dto.PriceMatrixSaveRequest) (*dto.PriceMatrixResponse, error)
	// CycleBasePrice 取指定周期的启停与售价（下单校验 + 算价基数）。
	// hasMatrix=false 表示该商品（或 SKU）尚未建立矩阵，调用方应回落旧单价口径。
	CycleBasePrice(ctx context.Context, productID, specID uint64, cycle string) (price float64, hasMatrix bool, err error)
	// ApplyUpstreamPrices 按上游周期成本写入矩阵（同步确认 / 克隆导入调用），
	// 会保留人工覆盖行（source=manual）不改写。
	ApplyUpstreamPrices(ctx context.Context, productID uint64, rows []dto.CyclePriceRow) error
	// EnabledCycles 返回商品可售周期（供前台/开放平台展示），按规范顺序。
	EnabledCycles(ctx context.Context, productID uint64) ([]string, error)
	// SellableCycles 返回商品级与 SKU 级可售周期。product 为商品级（spec_id=0）行；
	// bySpec 为各 SKU 自有行（无自有行的 SKU 不出现，由调用方回落商品级）。
	SellableCycles(ctx context.Context, productID uint64) (product []string, bySpec map[uint64][]string, err error)
}

type priceMatrixService struct {
	repo     repository.PriceMatrixRepository
	products ProductMetaReader
	channels ProviderCyclesReader
}

// NewPriceMatrixService 创建周期价格矩阵服务；products/channels 可为 nil（降级为不校验上游周期）。
func NewPriceMatrixService(repo repository.PriceMatrixRepository, products ProductMetaReader, channels ProviderCyclesReader) PriceMatrixService {
	return &priceMatrixService{repo: repo, products: products, channels: channels}
}

// defaultCurrency 平台当前仅支持人民币计价。
const defaultCurrency = "CNY"

func normalizeCurrency(raw string) string {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if c == "" {
		return defaultCurrency
	}
	return c
}

// Matrix 组装矩阵：以"该商品可用的全部周期"为行，缺行的档位补零行（status=0）。
func (s *priceMatrixService) Matrix(ctx context.Context, query dto.PriceMatrixQuery) (*dto.PriceMatrixResponse, error) {
	if query.ProductID == 0 {
		return nil, errors.New("product_id 必填")
	}
	currency := normalizeCurrency(query.Currency)
	resp := &dto.PriceMatrixResponse{
		ProductID: query.ProductID,
		SpecID:    query.SpecID,
		Currency:  currency,
		Items:     []dto.PriceMatrixItem{},
	}
	meta, err := s.productMeta(ctx, query.ProductID)
	if err != nil {
		return nil, err
	}
	if meta != nil {
		resp.ProductName = meta.Name
		resp.SourceMode = meta.SourceMode
	}
	cycles, err := s.availableCycles(ctx, meta)
	if err != nil {
		return nil, err
	}
	resp.UpstreamCycles = cycles

	rows, err := s.repo.List(ctx, query.ProductID, query.SpecID, currency)
	if err != nil {
		return nil, err
	}
	byCycle := make(map[string]model.ProductPrice, len(rows))
	for _, r := range rows {
		byCycle[r.Cycle] = r
	}
	for _, cycle := range cycles {
		item := dto.PriceMatrixItem{
			Cycle:     cycle,
			CycleName: billingcycle.Name(cycle),
			// 上游商品未同步到的档位不可人工填价（价格须来自上游同步 + 加价规则）。
			Editable: !isUpstream(meta),
		}
		if row, ok := byCycle[cycle]; ok {
			item.Price = row.Price
			item.CostPrice = row.CostPrice
			item.SetupFee = row.SetupFee
			item.CostSetupFee = row.CostSetupFee
			item.Source = row.Source
			item.Status = row.Status
			item.Remark = row.Remark
			// 上游派生行只读；人工行（运营显式覆盖过）仍可改。
			item.Editable = !isUpstream(meta) || row.Source == model.PriceSourceManual
		}
		resp.Items = append(resp.Items, item)
	}
	return resp, nil
}

// availableCycles 可售周期集合：
//   - 上游商品：渠道能力声明的周期 ∩ 平台规范周期（运营不能自造周期）；
//     渠道未声明时回落"已有矩阵行"，再退到平台全集，避免渠道描述符缺失时页面空白。
//   - 自营商品：平台规范周期全集（账期权威在平台）。
func (s *priceMatrixService) availableCycles(ctx context.Context, meta *MatrixProductMeta) ([]string, error) {
	if isUpstream(meta) && s.channels != nil && meta.SourceProviderID > 0 {
		raw, err := s.channels.BillingCycles(ctx, meta.SourceProviderID)
		if err == nil && len(raw) > 0 {
			return billingcycle.NormalizeList(raw), nil
		}
	}
	return billingcycle.AllCodes(), nil
}

// isUpstream 商品是否为上游转售链路（价格权威在上游）。
func isUpstream(meta *MatrixProductMeta) bool {
	return meta != nil && meta.SourceMode == "upstream"
}

// isUpstreamDerived 上游派生行（source=upstream/markup）不允许人工改价：
// 价格由上游成本 + 加价规则推导，要改必须改上游或加价规则。
func isUpstreamDerived(meta *MatrixProductMeta, source string) bool {
	if !isUpstream(meta) {
		return false
	}
	switch source {
	case model.PriceSourceUpstream, model.PriceSourceMarkup:
		return true
	default:
		return false
	}
}

// SaveMatrix 整表保存。上游商品的派生行只接受"启停"变更，价格/成本保持库中原值。
func (s *priceMatrixService) SaveMatrix(ctx context.Context, req dto.PriceMatrixSaveRequest) (*dto.PriceMatrixResponse, error) {
	if req.ProductID == 0 {
		return nil, errors.New("product_id 必填")
	}
	currency := normalizeCurrency(req.Currency)
	meta, err := s.productMeta(ctx, req.ProductID)
	if err != nil {
		return nil, err
	}
	allowed, err := s.availableCycles(ctx, meta)
	if err != nil {
		return nil, err
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, c := range allowed {
		allowedSet[c] = struct{}{}
	}
	existing, err := s.repo.List(ctx, req.ProductID, req.SpecID, currency)
	if err != nil {
		return nil, err
	}
	existingByCycle := make(map[string]model.ProductPrice, len(existing))
	for _, r := range existing {
		existingByCycle[r.Cycle] = r
	}

	keep := make([]string, 0, len(req.Items))
	seen := map[string]struct{}{}
	for _, in := range req.Items {
		cycle := billingcycle.Normalize(in.Cycle)
		if cycle == "" {
			return nil, fmt.Errorf("不支持的计费周期：%s", in.Cycle)
		}
		if _, ok := seen[cycle]; ok {
			return nil, fmt.Errorf("计费周期重复：%s", billingcycle.Name(cycle))
		}
		seen[cycle] = struct{}{}
		if _, ok := allowedSet[cycle]; !ok {
			return nil, fmt.Errorf("上游渠道不提供该计费周期：%s", billingcycle.Name(cycle))
		}
		if in.Price < 0 || in.CostPrice < 0 || in.SetupFee < 0 || in.CostSetupFee < 0 {
			return nil, errors.New("价格与成本不能为负")
		}
		row := model.ProductPrice{
			ProductID:     req.ProductID,
			ProductSpecID: req.SpecID,
			Cycle:         cycle,
			Currency:      currency,
			Price:         in.Price,
			CostPrice:     in.CostPrice,
			SetupFee:      in.SetupFee,
			CostSetupFee:  in.CostSetupFee,
			Source:        model.PriceSourceManual,
			Status:        normalizeStatus(in.Status),
			Remark:        req.Remark,
		}
		if prev, ok := existingByCycle[cycle]; ok {
			// 上游派生行：价格由同步推导，人工只能启停。
			if isUpstreamDerived(meta, prev.Source) {
				row.Price = prev.Price
				row.CostPrice = prev.CostPrice
				row.SetupFee = prev.SetupFee
				row.CostSetupFee = prev.CostSetupFee
				row.Source = prev.Source
			}
		}
		if err := s.repo.Upsert(ctx, &row); err != nil {
			return nil, err
		}
		keep = append(keep, cycle)
	}
	if _, err := s.repo.DeleteMissing(ctx, req.ProductID, req.SpecID, currency, keep); err != nil {
		return nil, err
	}
	return s.Matrix(ctx, dto.PriceMatrixQuery{ProductID: req.ProductID, SpecID: req.SpecID, Currency: currency})
}

func normalizeStatus(v int) int {
	if v == repository.PriceStatusEnabled {
		return repository.PriceStatusEnabled
	}
	return repository.PriceStatusDisabled
}

// CycleBasePrice 取周期售价。命中优先级：SKU 自有行 → 商品级行。
// hasMatrix=false 表示该商品与 SKU 都没有矩阵行，调用方应回落
// products.price / product_pricing 旧口径（存量兼容）。
func (s *priceMatrixService) CycleBasePrice(ctx context.Context, productID, specID uint64, cycle string) (float64, bool, error) {
	code := billingcycle.Normalize(cycle)
	if code == "" {
		return 0, false, fmt.Errorf("不支持的计费周期：%s", cycle)
	}
	// SKU 有自有周期行时以 SKU 为权威；无自有行则回落商品级矩阵（矩阵可按商品建、按 SKU 覆盖）。
	if specID != 0 {
		rows, err := s.repo.List(ctx, productID, specID, "")
		if err != nil {
			return 0, false, err
		}
		if len(rows) > 0 {
			return s.pickCyclePrice(ctx, productID, specID, code, true)
		}
	}
	rows, err := s.repo.List(ctx, productID, 0, "")
	if err != nil {
		return 0, false, err
	}
	if len(rows) == 0 {
		return 0, false, nil
	}
	return s.pickCyclePrice(ctx, productID, 0, code, true)
}

// pickCyclePrice 在指定 (商品, SKU) 下取某周期启用行；hasMatrix 由调用方给定。
func (s *priceMatrixService) pickCyclePrice(ctx context.Context, productID, specID uint64, code string, hasMatrix bool) (float64, bool, error) {
	row, err := s.repo.FindActive(ctx, productID, specID, code, defaultCurrency)
	if err != nil {
		return 0, hasMatrix, err
	}
	if row == nil {
		return 0, hasMatrix, fmt.Errorf("该商品不提供%s，请选择其它计费周期", billingcycle.Name(code))
	}
	return row.Price, hasMatrix, nil
}

// ApplyUpstreamPrices 上游周期成本落库：人工行（manual）不被覆盖，只按需新增/更新派生行。
func (s *priceMatrixService) ApplyUpstreamPrices(ctx context.Context, productID uint64, rows []dto.CyclePriceRow) error {
	if productID == 0 || len(rows) == 0 {
		return nil
	}
	existing, err := s.repo.List(ctx, productID, 0, defaultCurrency)
	if err != nil {
		return err
	}
	manual := map[string]struct{}{}
	for _, r := range existing {
		if r.Source == model.PriceSourceManual {
			manual[r.Cycle] = struct{}{}
		}
	}
	for _, in := range rows {
		cycle := billingcycle.Normalize(in.Cycle)
		if cycle == "" {
			continue
		}
		if _, ok := manual[cycle]; ok {
			continue
		}
		row := model.ProductPrice{
			ProductID: productID,
			Cycle:     cycle,
			Currency:  defaultCurrency,
			Price:     in.Price,
			CostPrice: in.CostPrice,
			// 初装费：上游 msetupfee 挂在哪一档就记在该档。
			SetupFee:     in.SetupFee,
			CostSetupFee: in.CostSetupFee,
			Source:       in.Source,
			Status:       repository.PriceStatusEnabled,
		}
		if row.Source == "" {
			row.Source = model.PriceSourceUpstream
		}
		if err := s.repo.Upsert(ctx, &row); err != nil {
			return err
		}
	}
	return nil
}

// EnabledCycles 商品级可售周期（status=1 且 price>0），按规范顺序返回。
func (s *priceMatrixService) EnabledCycles(ctx context.Context, productID uint64) ([]string, error) {
	rows, err := s.repo.List(ctx, productID, 0, defaultCurrency)
	if err != nil {
		return nil, err
	}
	return sellableFromRows(rows), nil
}

// SellableCycles 商品级 + 各 SKU 可售周期，供用户端/开放平台展示周期选择。
func (s *priceMatrixService) SellableCycles(ctx context.Context, productID uint64) ([]string, map[uint64][]string, error) {
	rows, err := s.repo.List(ctx, productID, 0, defaultCurrency)
	if err != nil {
		return nil, nil, err
	}
	bySpecRows := map[uint64][]model.ProductPrice{}
	for _, r := range rows {
		if r.ProductSpecID == 0 {
			continue
		}
		bySpecRows[r.ProductSpecID] = append(bySpecRows[r.ProductSpecID], r)
	}
	bySpec := make(map[uint64][]string, len(bySpecRows))
	for specID, list := range bySpecRows {
		if cycles := sellableFromRows(list); len(cycles) > 0 {
			bySpec[specID] = cycles
		}
	}
	return sellableFromRows(rows), bySpec, nil
}

// sellableFromRows 从价格行筛出可售周期（启用且售价 > 0），按规范顺序。
func sellableFromRows(rows []model.ProductPrice) []string {
	codes := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Status == repository.PriceStatusEnabled && r.Price > 0 {
			codes = append(codes, r.Cycle)
		}
	}
	return billingcycle.NormalizeList(codes)
}

func (s *priceMatrixService) productMeta(ctx context.Context, productID uint64) (*MatrixProductMeta, error) {
	if s.products == nil {
		return nil, nil
	}
	return s.products.ProductMeta(ctx, productID)
}
