// 商品目录域装配（批次四 4b，合并原 spec_template/option_catalog/price_matrix 三个适配器文件）。

package server

import (
	"context"
	"encoding/json"
	instancerepo "hostsent/backend/internal/modules/admin/instance/repository"
	cataloghandler "hostsent/backend/internal/modules/admin/product/catalog/handler"
	catalogrepo "hostsent/backend/internal/modules/admin/product/catalog/repository"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	categoryhandler "hostsent/backend/internal/modules/admin/product/category/handler"
	categoryrepo "hostsent/backend/internal/modules/admin/product/category/repository"
	categoryservice "hostsent/backend/internal/modules/admin/product/category/service"
	discounthandler "hostsent/backend/internal/modules/admin/product/discount/handler"
	discountrepo "hostsent/backend/internal/modules/admin/product/discount/repository"
	discountservice "hostsent/backend/internal/modules/admin/product/discount/service"
	flashdiscounthandler "hostsent/backend/internal/modules/admin/product/flashdiscount/handler"
	flashdiscountrepo "hostsent/backend/internal/modules/admin/product/flashdiscount/repository"
	flashdiscountservice "hostsent/backend/internal/modules/admin/product/flashdiscount/service"
	pricingdto "hostsent/backend/internal/modules/admin/product/pricing/dto"
	pricinghandler "hostsent/backend/internal/modules/admin/product/pricing/handler"
	pricingrepo "hostsent/backend/internal/modules/admin/product/pricing/repository"
	matrixservice "hostsent/backend/internal/modules/admin/product/pricing/service"
	promotionhandler "hostsent/backend/internal/modules/admin/product/promotion/handler"
	promotionrepo "hostsent/backend/internal/modules/admin/product/promotion/repository"
	promotionservice "hostsent/backend/internal/modules/admin/product/promotion/service"
	specdto "hostsent/backend/internal/modules/admin/product/spec/dto"
	spechandler "hostsent/backend/internal/modules/admin/product/spec/handler"
	specrepo "hostsent/backend/internal/modules/admin/product/spec/repository"
	specservice "hostsent/backend/internal/modules/admin/product/spec/service"
	productservice "hostsent/backend/internal/modules/admin/resource/product/service"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	ucproducthandler "hostsent/backend/internal/modules/uc/product/handler"
	ucproductservice "hostsent/backend/internal/modules/uc/product/service"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/pricing"
	"hostsent/backend/internal/pkg/upstream"
	"math"
	"strings"

	"go.uber.org/zap"
)

// buildCatalog 构造分组/目录/定价/矩阵/规格/促销/折扣/算价管线与用户中心商品。
func (a *asm) buildCatalog() {
	// 产品管理（面向终端售卖）
	a.prodCategoryRepo = categoryrepo.NewCategoryRepository(a.database)
	a.prodCategoryService = categoryservice.NewCategoryService(a.prodCategoryRepo)
	a.prodCategoryHandler = categoryhandler.NewCategoryHandler(a.prodCategoryService)
	// 商品管理（catalog 子域）
	a.prodCatalogRepo = catalogrepo.NewProductRepository(a.database)
	// 规格契约仓储/服务提前构造：catalog 服务需读取 SKU 平台绑定（T4.2/T4.6）
	// 并在克隆时登记上游规格快照（T4.3），故先于 prodCatalogService 装配。
	a.specContractRepo = specrepo.NewSpecContractRepository(a.database)
	a.specContractService = specservice.NewSpecContractService(a.specContractRepo)
	a.prodCatalogService = catalogservice.NewProductService(
		a.prodCatalogRepo, a.productRepo, a.providerRepo, a.specContractRepo,
		catalogservice.UpstreamSpecRegistrar(func(ctx context.Context, snap catalogservice.UpstreamSpecSnapshot) error {
			return a.specContractService.RegisterUpstreamSpec(ctx, specservice.UpstreamSpecSnapshot{
				ProviderID:   snap.ProviderID,
				ProviderType: snap.ProviderType,
				ExternalID:   snap.ExternalID,
				ExternalName: snap.ExternalName,
				ExternalKind: snap.ExternalKind,
				Raw:          snap.Raw,
				Normalized:   snap.Normalized,
			})
		}),
	)
	a.prodCatalogHandler = cataloghandler.NewProductHandler(a.prodCatalogService)
	// 上游商品页/导入弹窗标注「已导入」：商品域的批量引用查询注入资源商品服务（跨域端口）。
	a.productService.SetImportedReader(newImportedProductReader(a.prodCatalogRepo))
	// T3.4：把"已确认调价"落地到售出商品的实现注入同步引擎（写 product_history）。
	a.syncEngine.SetPriceApplier(a.prodCatalogService)
	// 上游库存随目录同步落地到售出商品（下单门禁读 products.stock）。
	a.syncEngine.SetStockApplier(a.prodCatalogService)
	// 定价与计费（pricing 子域）
	a.pricingRepo = pricingrepo.NewPricingRepository(a.database)
	a.pricingService = matrixservice.NewPricingService(a.pricingRepo)
	a.pricingHandler = pricinghandler.NewPricingHandler(a.pricingService)
	// 周期价格矩阵（doc25）：商品 × 规格 × 周期 的价格与启停，整表读写。
	a.priceMatrixRepo = pricingrepo.NewPriceMatrixRepository(a.database)
	a.priceMatrixService = matrixservice.NewPriceMatrixService(
		a.priceMatrixRepo,
		NewProductMetaReader(a.prodCatalogService),
		NewProviderCyclesReader(a.providerService),
	)
	a.priceMatrixHandler = pricinghandler.NewPriceMatrixHandler(a.priceMatrixService)
	// 上游调价确认 → 按周期重算售出商品的价格矩阵（doc25 §5）。
	a.prodCatalogService.SetCyclePriceWriter(NewCyclePriceWriter(a.priceMatrixService))
	// 续费价同步（商品改价 → 已绑定续费价的实例）的注入放在 buildWire：
	// 它依赖 a.instanceOpsRepo，而实例域（buildInstanceOps）在本函数之后构建。
	// 用户中心商品：复用管理端商品目录，仅暴露上架商品（用户可购）；
	// 详情页带周期价格矩阵的可售周期（doc25），用户端据此渲染周期选择。
	a.ucProductService = ucproductservice.NewProductService(a.prodCatalogService, NewUCCycleReader(a.priceMatrixService))
	a.ucProductHandler = ucproducthandler.NewProductHandler(a.ucProductService)
	// 折扣策略（discount 子域，P5-01/P5-06）：**已退役**，仅保留接口与页面外的兼容，
	// 算价链路不再读取它（折扣来源已迁到代理等级与限时活动折扣，见 doc108）。
	a.discountPolicyRepo = discountrepo.NewPricePolicyRepository(a.database)
	a.discountPolicyService = discountservice.NewPolicyService(a.discountPolicyRepo)
	a.discountPolicyHandler = discounthandler.NewPolicyHandler(a.discountPolicyService)
	// 限时活动折扣（doc108 §8J）：面向**普通用户**的营销折扣，按生效窗口实时命中。
	// 注入 AgentChecker（agentLevelService.IsAgent）以实现「代理不参与活动折扣」。
	// 必须构造在 pricePipeline 之前：PromotionRule 要引用它。
	a.flashDiscountRepo = flashdiscountrepo.NewRepository(a.database)
	a.flashDiscountService = flashdiscountservice.NewService(a.flashDiscountRepo, a.agentLevelService)
	a.flashDiscountHandler = flashdiscounthandler.NewHandler(a.flashDiscountService)
	// 统一算价管线（P5-03）：基础价读 product_pricing 回落 products.price；折扣来源按序注入。
	a.pricePipeline = pricing.NewService(pricing.Deps{
		BasePrice: func(ctx context.Context, productID uint64) (float64, uint64, error) {
			product, err := a.prodCatalogService.FindByID(ctx, productID)
			if err != nil {
				return 0, 0, err
			}
			unitPrice := product.Price
			// product_pricing = 这商品基础多少钱：配置了启用中的计费模板则优先采用其单价。
			if tmpl, err := a.pricingRepo.FindByProductID(ctx, productID); err == nil && tmpl != nil && tmpl.UnitPrice > 0 {
				unitPrice = tmpl.UnitPrice
			}
			return unitPrice, product.CategoryID, nil
		},
		// SKU 级基础价（T4.1）：请求带 spec_code 且该 SKU 有独立定价时覆盖商品级基础价；
		// 未定价 / 未选规格返回 found=false，回落商品级基础价（折扣算法不变）。
		SpecBasePrice: func(ctx context.Context, in pricing.ResolveInput) (float64, uint64, bool, error) {
			if in.SpecCode == "" {
				return 0, 0, false, nil
			}
			spec, err := a.prodCatalogService.FindSpec(ctx, in.ProductID, in.SpecCode)
			if err != nil || spec == nil || spec.Status != 1 || spec.Price <= 0 {
				return 0, 0, false, nil
			}
			return spec.Price, 0, true, nil
		},
		// 周期基础价（doc25）：矩阵 (商品, SKU, 周期) 有价则以它为基数；
		// 优先级最高（周期价就是"折后价"，再交给 price_policies 二次叠加）。
		// 矩阵无该规格行时回落 SpecBasePrice / BasePrice，存量行为不变。
		CycleBasePrice: func(ctx context.Context, in pricing.ResolveInput) (float64, uint64, bool, error) {
			if in.Cycle == "" {
				return 0, 0, false, nil
			}
			specID := uint64(0)
			if in.SpecCode != "" {
				if spec, err := a.prodCatalogService.FindSpec(ctx, in.ProductID, in.SpecCode); err == nil && spec != nil {
					specID = spec.ID
				}
			}
			price, hasMatrix, err := a.priceMatrixService.CycleBasePrice(ctx, in.ProductID, specID, in.Cycle)
			if err != nil {
				// 矩阵存在但该周期未开放：显式拒绝，避免按下单周期错价成交。
				return 0, 0, false, err
			}
			if !hasMatrix {
				return 0, 0, false, nil
			}
			return price, 0, true, nil
		},
		// 代理等级折扣 = 这代理打几折（doc108 唯一折扣来源）。
		// 用户组已不再参与算价（只做客户分类），因此此处不再注入 GroupRule。
		AgentRule: a.agentLevelService.RuleForUser,
		// 限时活动折扣（doc108 §8J）：面向**普通用户**的营销活动，按生效窗口实时命中。
		// 内部先判「是不是代理」——代理只走 AgentRule，不参与活动，两条线互不叠加。
		PromotionRule: func(ctx context.Context, in pricing.ResolveInput, _ float64) (*pricing.Rule, error) {
			return a.flashDiscountService.RuleForUser(ctx, in.UserID, in.ProductID)
		},
		// 客户选配项加价（T4.5）：按订单周期取选中取值的加价小计。
		// 走 catalog 服务的统一计价实现——与下单时校验、履约传参用的是同一套规则，
		// 保证「预结算 = 下单 = 开通」三处口径一致。
		OptionPrice: func(ctx context.Context, in pricing.ResolveInput) (float64, error) {
			result, err := a.prodCatalogService.ResolveOptionPricing(ctx, in.ProductID, in.Cycle, in.ConfigSelections)
			if err != nil {
				return 0, err
			}
			if result == nil {
				return 0, nil
			}
			return result.Amount, nil
		},
	}, a.cfg.Pricing.StackMode)
	// 规格管理（spec 子域）
	a.specTemplateRepo = specrepo.NewSpecTemplateRepository(a.database)
	a.specMappingRepo = specrepo.NewSpecMappingRepository(a.database)
	// specContractRepo 已在上方随 catalog 服务构造（见 prodCatalogService 处）。
	// 规格原子字典（T2.5/T2.6）：启动期从 spec_atoms 载入内存，供 Extra 校验与平台字段展示。
	if err := specrepo.LoadAtomDictionary(context.Background(), a.specContractRepo); err != nil {
		a.logger.Warn("加载规格原子字典失败", zap.Error(err))
	}
	a.specTemplateService = specservice.NewSpecTemplateService(a.specTemplateRepo)
	a.specMappingService = specservice.NewSpecMappingService(a.specMappingRepo)
	// specContractService 已在上方随 catalog 服务构造（见 prodCatalogService 处）。
	// 平台配置项目录（T4.5）：适配器声明 → 取值库 → 配置档。渠道类型/平台资源读取
	// 通过装配适配器注入，spec 子域不反向依赖 resource 子域。
	a.specOptionRepo = specrepo.NewSpecOptionRepository(a.database)
	a.specOptionService = specservice.NewSpecOptionService(a.specOptionRepo,
		NewProviderTypeReader(a.providerService), NewPlatformResourceReader(a.providerService))
	a.specHandler = spechandler.NewSpecHandler(a.specTemplateService, a.specMappingService, a.specContractService)
	a.specHandler.SetOptionService(a.specOptionService)
	// 配置档按平台校验勾选取值：目录服务作为校验器注入模板服务。
	a.specTemplateService.SetOptionCatalog(a.specOptionService)
	// 自营链路打通：catalog 服务按规格模板生成 SKU 并落平台绑定（装配顺序上模板/契约服务在本行之后才就绪）。
	a.prodCatalogService.SetSpecTemplateReader(NewSpecTemplateReader(a.specTemplateService))
	a.prodCatalogService.SetSpecBindingWriter(NewSpecBindingWriter(a.specContractService))
	// T4.5：建品按配置档生成客户选配项时，从平台配置项目录取控件元数据与可选值标签。
	a.prodCatalogService.SetSpecOptionCatalog(NewSpecOptionCatalogReader(a.specOptionService))
	// 促销管理（promotion 子域）
	a.couponRepo = promotionrepo.NewCouponRepository(a.database)
	a.couponGrantRepo = promotionrepo.NewCouponGrantRepository(a.database)
	a.promotionRepo = promotionrepo.NewPromotionRepository(a.database)
	a.couponService = promotionservice.NewCouponService(a.couponRepo, a.couponGrantRepo)
	a.couponGrantService = promotionservice.NewCouponGrantService(a.couponGrantRepo, a.couponRepo)
	a.promotionService = promotionservice.NewPromotionService(a.promotionRepo)
	a.promotionHandler = promotionhandler.NewPromotionHandler(a.couponService, a.couponGrantService, a.promotionService)
}

// 自营链路打通（规格模板 → SKU → 平台绑定）的装配适配层。
//
// catalog 服务不认识 spec 子域的模型：这里把 spec 模板服务与规格契约服务
// 适配成 catalog 侧声明的最小接口，避免两个子域互相依赖（与 price_matrix_adapters 同法）。

// specTemplateReaderAdapter 把 spec 的模板服务适配为 catalog 的 SpecTemplateReader。
type specTemplateReaderAdapter struct {
	templates specservice.SpecTemplateService
}

// NewSpecTemplateReader 构造 SpecTemplateReader（装配层调用）。
func NewSpecTemplateReader(templates specservice.SpecTemplateService) catalogservice.SpecTemplateReader {
	return &specTemplateReaderAdapter{templates: templates}
}

func (a *specTemplateReaderAdapter) SpecTemplateByID(ctx context.Context, id uint64) (*catalogservice.SpecTemplateSnapshot, error) {
	item, err := a.templates.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return &catalogservice.SpecTemplateSnapshot{
		ID:               item.ID,
		Name:             item.Name,
		SpecFamily:       item.SpecFamily,
		CPU:              item.CPU,
		Memory:           item.Memory,
		Disk:             item.Disk,
		Bandwidth:        item.Bandwidth,
		DiskType:         item.DiskType,
		OS:               item.OS,
		Price:            item.Price,
		SpecValues:       rawJSONString(item.SpecValues),
		PlatformParams:   rawJSONString(item.PlatformParams),
		ProviderType:     item.ProviderType,
		OptionSelections: rawJSONString(item.OptionSelections),
		NameTemplate:     item.NameTemplate,
	}, nil
}

// specBindingWriterAdapter 把规格契约服务适配为 catalog 的 SpecBindingWriter。
type specBindingWriterAdapter struct {
	contracts specservice.SpecContractService
}

// NewSpecBindingWriter 构造 SpecBindingWriter（装配层调用）。
func NewSpecBindingWriter(contracts specservice.SpecContractService) catalogservice.SpecBindingWriter {
	return &specBindingWriterAdapter{contracts: contracts}
}

// UpsertConfirmedProductSpecBinding 为 SKU 幂等写入出站绑定并置 confirmed（模板即人工确认过的映射）。
func (a *specBindingWriterAdapter) UpsertConfirmedProductSpecBinding(
	ctx context.Context, productSpecID, specTemplateID uint64, platformParams string, operatorID uint64, remark string,
) error {
	info, err := a.contracts.UpsertBinding(ctx, specdto.SpecBindingUpsertRequest{
		ProductSpecID:  productSpecID,
		SpecTemplateID: specTemplateID,
		Direction:      "outbound",
		PlatformParams: []byte(platformParams),
		MatchType:      "manual",
		Status:         "confirmed",
		Confidence:     100,
		Remark:         remark,
	})
	if err != nil {
		return err
	}
	if info == nil {
		return nil
	}
	// 复核确认，确保确认人/时间留痕（UpsertBinding 已置 confirmed，这里补齐审计字段）。
	_, err = a.contracts.ConfirmBinding(ctx, info.ID, specdto.SpecBindingConfirmRequest{Remark: remark}, operatorID)
	return err
}

// rawJSONString 把 DTO 的 json.RawMessage 转成字符串；空/null 返回空串。
func rawJSONString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	s := string(raw)
	if s == "null" {
		return ""
	}
	return s
}

// 平台配置项目录（T4.5）的装配适配层。
//
// spec 子域需要"渠道 ID → 平台类型"与"平台实时资源目录"两项能力，而这两项都在
// resource/provider 子域。按本项目既有的适配器隔离法（同 price_matrix_adapters、
// spec_template_adapters），这里把 ProviderService 适配成 spec 侧声明的最小接口，
// 避免 spec 反向依赖 resource。

// providerTypeReaderAdapter 把渠道服务适配为 spec 的 ProviderTypeByID。
type providerTypeReaderAdapter struct {
	providers providerservice.ProviderService
}

// NewProviderTypeReader 构造渠道类型读取器（装配层调用）。
func NewProviderTypeReader(providers providerservice.ProviderService) *providerTypeReaderAdapter {
	return &providerTypeReaderAdapter{providers: providers}
}

func (a *providerTypeReaderAdapter) ProviderTypeByID(ctx context.Context, id uint64) (string, error) {
	item, err := a.providers.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", nil
	}
	return item.ProviderType, nil
}

// platformResourceReaderAdapter 把渠道服务适配为 spec 的平台资源目录读取器。
type platformResourceReaderAdapter struct {
	providers providerservice.ProviderService
}

// NewPlatformResourceReader 构造平台资源读取器（装配层调用）。
func NewPlatformResourceReader(providers providerservice.ProviderService) *platformResourceReaderAdapter {
	return &platformResourceReaderAdapter{providers: providers}
}

func (a *platformResourceReaderAdapter) PlatformResources(ctx context.Context, providerID uint64) (*upstream.PlatformResources, error) {
	return a.providers.PlatformResources(ctx, providerID)
}

// specOptionCatalogAdapter 把 spec 的配置项目录服务适配为 catalog 的 SpecOptionCatalogReader。
// 两个子域各自声明最小接口，中间靠本适配器翻译快照结构。
type specOptionCatalogAdapter struct {
	options specservice.SpecOptionService
}

// NewSpecOptionCatalogReader 构造配置项目录读取器（装配层调用）。
func NewSpecOptionCatalogReader(options specservice.SpecOptionService) catalogservice.SpecOptionCatalogReader {
	return &specOptionCatalogAdapter{options: options}
}

func (a *specOptionCatalogAdapter) OptionCatalogByProvider(ctx context.Context, providerType string) ([]catalogservice.SpecOptionMeta, error) {
	infos, err := a.options.OptionCatalogByProvider(ctx, providerType)
	if err != nil {
		return nil, err
	}
	out := make([]catalogservice.SpecOptionMeta, 0, len(infos))
	for _, info := range infos {
		meta := catalogservice.SpecOptionMeta{
			OptionKey:    info.OptionKey,
			Label:        info.Label,
			Widget:       info.Widget,
			Required:     info.Required,
			Default:      info.DefaultValue,
			Unit:         info.Unit,
			GroupName:    info.GroupName,
			Help:         info.Help,
			OptionsHelp:  optionsHelpText(info.OptionsHelp),
			ProviderType: info.ProviderType,
			MinValue:     info.MinValue,
			MaxValue:     info.MaxValue,
			SortOrder:    info.SortOrder,
		}
		for _, v := range info.Values {
			meta.Values = append(meta.Values, catalogservice.SpecOptionValueMeta{
				Value: v.Value, Label: v.Label, GroupLabel: v.GroupLabel,
			})
		}
		out = append(out, meta)
	}
	return out, nil
}

// optionsHelpText 把目录项里的取值含义表（jsonb）转成纯文本，供 catalog 侧快照。
// nil / 空 / null 都归一为空串。
func optionsHelpText(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "{}" {
		return ""
	}
	return s
}

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
// newImportedProductReader 把商品域仓库适配成资源域的「已导入」查询端口：
// 资源商品服务只需知道「这个上游商品有没有被导入、对应哪个本地商品」，
// 不引入商品域模型（避免资源域反向依赖商品域）。
func newImportedProductReader(repo catalogrepo.ProductRepository) productservice.ImportedProductReader {
	return importedProductReader{repo: repo}
}

type importedProductReader struct {
	repo catalogrepo.ProductRepository
}

func (r importedProductReader) ListImportedRefs(ctx context.Context, resourceProductIDs []uint64) (map[uint64]productservice.ImportedProductRef, error) {
	refs, err := r.repo.ListImportedRefs(ctx, resourceProductIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]productservice.ImportedProductRef, len(refs))
	for rpID, item := range refs {
		out[rpID] = productservice.ImportedProductRef{ProductID: item.ID, Status: item.Status}
	}
	return out, nil
}

func NewCyclePriceWriter(matrix matrixservice.PriceMatrixService) catalogservice.CyclePriceWriter {
	return &cyclePriceWriterAdapter{matrix: matrix}
}

func (a *cyclePriceWriterAdapter) ApplyUpstreamPrices(ctx context.Context, productID uint64, rows []catalogservice.CyclePriceRow) error {
	converted := make([]pricingdto.CyclePriceRow, 0, len(rows))
	for _, r := range rows {
		converted = append(converted, pricingdto.CyclePriceRow{
			Cycle:         r.Cycle,
			Price:         r.Price,
			CostPrice:     r.CostPrice,
			PreservePrice: r.PreservePrice,
			SetupFee:      r.SetupFee,
			CostSetupFee:  r.CostSetupFee,
			Source:        r.Source,
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

// ============================================================================
// 续费价同步适配器（商品改价 → 已绑定续费价的实例）
// ============================================================================

// priceStatusEnabled 周期价格矩阵行的"启用"状态（与 pricing/repository 同值）。
// 这里用字面量而非 import：本文件只需要这一个常量，而 pricing 仓储包已通过
// matrixservice 间接引入，再显式依赖一层会让"谁依赖谁"更难读。
const priceStatusEnabled = 1

// roundMoney 金额按分取整（比较"是否已是目标价"时避免浮点尾差误判）。
func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

// renewalPriceSyncerAdapter 把「实例仓储 + 价格矩阵」适配成 catalog 的续费价同步端口。
//
// 为什么做成端口：instance 域与 catalog 域互不依赖，直接互调会形成循环
// （instance 已经依赖 catalog 的商品模型）。装配层用最小接口把两边接起来，
// 与 pendingInstanceReleaser / StockApplier 的既有做法一致。
type renewalPriceSyncerAdapter struct {
	instances instancerepo.InstanceRepository
	matrix    matrixservice.PriceMatrixService
	// logger 记录"某台实例取不到该档价、保持原值"这类逐台明细，便于运营核对。
	logger *zap.Logger
}

// NewRenewalPriceSyncer 创建续费价同步适配器（装配层调用）。
func NewRenewalPriceSyncer(instances instancerepo.InstanceRepository, matrix matrixservice.PriceMatrixService, logger *zap.Logger) catalogservice.RenewalPriceSyncer {
	return &renewalPriceSyncerAdapter{instances: instances, matrix: matrix, logger: logger}
}

// SyncBoundRenewalPrices 按各实例自己的付款周期，把商品当前该档价写回 renewal_price。
//
// 逐台取价而不是"取一个价写全部"：绑定价是**每周期单价**，而实例的付款周期各不相同
// （月付/年付混在一起）。全部写同一个数会让年付实例按"月价"续费、平台少收钱。
func (a *renewalPriceSyncerAdapter) SyncBoundRenewalPrices(ctx context.Context, productID uint64, dryRun bool) (int, error) {
	rows, err := a.instances.ListBoundRenewalByProduct(ctx, productID)
	if err != nil {
		return 0, err
	}
	// 商品级周期价（spec_id=0）映射：周期 → 售价。
	cyclePrices, err := a.productCyclePrices(ctx, productID)
	if err != nil {
		return 0, err
	}
	affected := 0
	for _, row := range rows {
		cycle := billingcycle.NormalizeOrDefault(row.BillingMode)
		price, ok := cyclePrices[cycle]
		if !ok || price <= 0 {
			// 该实例的周期在商品价格矩阵里没有可售行：保留原绑定值不动。
			// 清零会让续费直接算不出钱（续费守卫会拒绝 0 元单），比"没同步"更糟。
			if a.logger != nil {
				a.logger.Info("renewal price sync skipped: cycle has no sellable price",
					zap.Uint64("product_id", productID), zap.Uint64("instance_id", row.ID),
					zap.String("cycle", cycle))
			}
			continue
		}
		if row.RenewalPrice != nil && roundMoney(*row.RenewalPrice) == roundMoney(price) {
			continue // 已是目标价，不必写库（也让 dry_run 的台数更贴近真实改动量）
		}
		if dryRun {
			affected++
			continue
		}
		p := price
		if err := a.instances.SetRenewalPrice(ctx, row.ID, &p); err != nil {
			// 单台失败不中断整批：批量操作里中断会留下"改了一半"的状态，
			// 且运营无法知道改到哪一台；逐台尽力而为并记日志更可核对。
			if a.logger != nil {
				a.logger.Warn("set renewal price failed",
					zap.Uint64("product_id", productID), zap.Uint64("instance_id", row.ID), zap.Error(err))
			}
			continue
		}
		affected++
	}
	return affected, nil
}

// ClearBoundRenewalPrices 清除绑定，让实例回到"续费时实时读商品周期价"。
func (a *renewalPriceSyncerAdapter) ClearBoundRenewalPrices(ctx context.Context, productID uint64, dryRun bool) (int, error) {
	rows, err := a.instances.ListBoundRenewalByProduct(ctx, productID)
	if err != nil {
		return 0, err
	}
	if dryRun {
		return len(rows), nil
	}
	n, err := a.instances.SetRenewalPriceByProduct(ctx, productID, nil, true)
	return int(n), err
}

// productCyclePrices 读商品级周期价（spec_id=0 且启用的行），返回 周期 → 售价。
func (a *renewalPriceSyncerAdapter) productCyclePrices(ctx context.Context, productID uint64) (map[string]float64, error) {
	matrix, err := a.matrix.Matrix(ctx, pricingdto.PriceMatrixQuery{ProductID: productID})
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(matrix.Items))
	for _, item := range matrix.Items {
		if item.Status != priceStatusEnabled || item.Price <= 0 {
			continue
		}
		out[item.Cycle] = item.Price
	}
	return out, nil
}
