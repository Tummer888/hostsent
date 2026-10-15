package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/credentials"
)

// 本文件提供实例详情页的附加只读数据：来源订单财务口径、售出商品与对接关系、
// 上游商品价格、实例生效配置项、登录连接信息。
//
// 与 related_repo.go 同样的取舍：跨表只读查询直接在本模块内完成，
// 不为「一屏看全」给每个关联域引入模块依赖（否则 instance 包会依赖 order/product/resource 三个包）。

// InstanceDetailRepository 实例详情附加数据只读仓储。
type InstanceDetailRepository interface {
	// SourceOrder 读来源订单（含按实例当前周期算出的续费价）。
	// instanceID 用于读取绑定价（instances.renewal_price 压过商品周期价）；传 0 则只按商品价。
	SourceOrder(ctx context.Context, orderID uint64, instanceID uint64, billingMode string, quantity int) (*dto.SourceOrderInfo, error)
	// Product 读售出商品与对接关系（含渠道名）。
	Product(ctx context.Context, productID uint64) (*dto.ProductInfo, error)
	// Upstream 读对接商品的上游信息（非对接商品返回 nil）。
	Upstream(ctx context.Context, sourceProductID uint64, billingMode string) (*dto.UpstreamInfo, float64, error)
	// ProductConfigs 读商品配置项并与订单选配/实例取值合并。
	ProductConfigs(ctx context.Context, productID uint64, orderConfigOptions string) ([]dto.ConfigItem, error)
	// Provision 读来源订单的开通履约状态（provision_tasks）：status + 最近失败原因。
	// 待开通实例的详情/列表据此展示"为什么还没开通"（manual=需人工，如上游余额不足）。
	Provision(ctx context.Context, orderID uint64) (status, lastError string, err error)
	// Credentials 读实例登录连接信息（不含明文密码）。
	Credentials(ctx context.Context, instanceID uint64) (*dto.CredentialInfo, error)
	// RevealCredential 读明文密码（供 reveal 接口；未存返回空）。
	RevealCredential(ctx context.Context, instanceID uint64) (username, password, port string, err error)
	// ProductCyclePrices 读商品价格矩阵的各周期售价/成本（product_prices）。
	// 账期调整对话框据此展示"我方该周期卖多少钱"，与上游价并排比价。
	ProductCyclePrices(ctx context.Context, productID uint64) (map[string]ProductCyclePrice, error)
	// ReinstallOSOptions 读某商品可用的操作系统镜像（重装对话框的下拉数据源）。
	// productID 为**本地售出商品**（products.id）：自营取其平台配置项、
	// 对接取其上游镜像（resource_products.raw_specs）的 os 配置子项。
	ReinstallOSOptions(ctx context.Context, productID uint64) ([]dto.ReinstallImage, bool, error)
	// RebindCandidates 读更换对接商品的候选项（本地商品 + 同渠道上游镜像）。
	//
	// resale=false（自营链路）时不返回上游镜像候选：自营实例不对接上游镜像商品，
	// 列出来只会让运营误以为"这里也能换"（用户实测反馈）。本地商品候选照给。
	// providerID 用于把上游镜像候选限定在本实例渠道内 —— 跨渠道换绑后端必拒。
	RebindCandidates(ctx context.Context, providerID uint64, resale bool) (*dto.RebindProductCandidates, error)
}

// ProductCyclePrice 商品某周期的售价与成本（product_prices 一行）。
type ProductCyclePrice struct {
	Sale float64
	Cost float64
}

type instanceDetailRepository struct {
	db *gorm.DB
	// secretKey 应用加密密钥：解密 instances.credential_password（迁移 100 的 enc:v1: 密文）。
	// 为空时视作不具备解密能力——读取显式报错，绝不把密文当明文返回（地雷 L8）。
	secretKey string
}

// NewInstanceDetailRepository 创建实例详情附加数据仓储。
func NewInstanceDetailRepository(db *gorm.DB, secretKey string) InstanceDetailRepository {
	return &instanceDetailRepository{db: db, secretKey: secretKey}
}

// ============================ 来源订单 ============================

// orderDetailRow orders 读取行。
type orderDetailRow struct {
	ID             uint64
	OrderNo        string
	Status         string
	ProductID      uint64
	ProductName    string
	Cycle          string
	Quantity       int
	TotalAmount    float64
	PaidAmount     float64
	DiscountAmount float64
	PayMethod      string
	PayTime        *time.Time
	CreatedAt      time.Time
	RenewalID      uint64
	Remark         string
	CustomerRemark string
}

// SourceOrder 读来源订单，并按其「当前付款周期」补出下次续费金额。
//
// 为什么续费金额要现算而不是读订单里的金额：订单金额是**历史成交**（可能带折扣、
// 可能是首月促销价），而运营关心的是"这台机器下次续费要多少钱"。后者由商品当前
// 周期价（product_prices）决定。
func (r *instanceDetailRepository) SourceOrder(ctx context.Context, orderID uint64, instanceID uint64, billingMode string, quantity int) (*dto.SourceOrderInfo, error) {
	if orderID == 0 {
		return nil, nil
	}
	var row orderDetailRow
	err := r.db.WithContext(ctx).Table("orders").
		Select("id, order_no, status, product_id, product_name, cycle, quantity, total_amount, paid_amount, discount_amount, pay_method, pay_time, created_at, renewal_id, remark, COALESCE(customer_remark, '') AS customer_remark").
		Where("id = ?", orderID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	out := &dto.SourceOrderInfo{
		ID:             row.ID,
		OrderNo:        row.OrderNo,
		Status:         row.Status,
		ProductID:      row.ProductID,
		ProductName:    row.ProductName,
		Cycle:          row.Cycle,
		CycleName:      billingcycle.Name(row.Cycle),
		Quantity:       row.Quantity,
		TotalAmount:    row.TotalAmount,
		PaidAmount:     row.PaidAmount,
		DiscountAmt:    row.DiscountAmount,
		FirstPaidAmt:   row.PaidAmount,
		PayMethod:      row.PayMethod,
		PayTime:        fmtTimeOpt(row.PayTime),
		OrderedAt:      row.CreatedAt.Format(time.RFC3339),
		RenewalID:      row.RenewalID,
		Remark:         row.Remark,
		CustomerRemark: row.CustomerRemark,
	}
	// 续费金额：按实例当前付款周期（可能已被调整过）取商品周期价。
	cycle := billingcycle.NormalizeOrDefault(billingMode)
	if cycle == "" {
		cycle = billingcycle.NormalizeOrDefault(row.Cycle)
	}
	if row.ProductID > 0 && cycle != "" {
		sale, cost, ok := r.cyclePrice(ctx, row.ProductID, cycle)
		if !ok {
			// 该周期没建价格矩阵：回落商品基础价，与续费算价管线的降级口径一致
			// （pricing.Resolve 在 CycleBasePrice found=false 时也用基础价 × 周期倍率）。
			// 此前这里直接留 0，详情页"续费金额"显示成 ¥0.00 且无任何说明（实测反馈）。
			sale, cost = r.productBasePrice(ctx, row.ProductID)
			ok = sale > 0
		}
		if ok {
			n := quantity
			if n <= 0 {
				n = 1
			}
			out.RenewUnitCost = cost
			out.RenewCycle = cycle
			out.RenewAmount = money2(sale * float64(n))
			out.RenewCost = money2(cost * float64(n))
		}
	}
	// 绑定续费价（instances.renewal_price）压过商品周期价 —— 与续费实际计费口径保持一致。
	//
	// 为什么必须在这里也生效：绑定价是"下次续费按这个价"的承诺（续费服务
	// renewal_service.boundRenewalPrice 优先用它）。详情页若仍显示商品价，
	// 运营看到的"续费金额"与实际扣款不是同一个数，调完价也看不出效果（实测困惑）。
	// 读不到实例行（order 存在但实例缺失）时保持商品价，不猜测。
	if bound, berr := r.boundRenewalPrice(ctx, instanceID); berr == nil && bound != nil {
		n := quantity
		if n <= 0 {
			n = 1
		}
		out.RenewAmount = money2(*bound * float64(n))
		out.RenewBound = true
		if out.RenewCycle == "" {
			out.RenewCycle = cycle
		}
	}
	return out, nil
}

// boundRenewalPrice 读实例绑定的续费单价（未绑定/非正数返回 nil）。
// 与续费服务同口径：0 与负数视为未绑定（0 元续费会被续费守卫拒绝）。
func (r *instanceDetailRepository) boundRenewalPrice(ctx context.Context, instanceID uint64) (*float64, error) {
	if instanceID == 0 {
		return nil, nil
	}
	var row struct {
		RenewalPrice *float64
	}
	err := r.db.WithContext(ctx).Table("instances").
		Select("renewal_price").Where("id = ?", instanceID).Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	if row.RenewalPrice == nil || *row.RenewalPrice <= 0 {
		return nil, nil
	}
	v := money2(*row.RenewalPrice)
	return &v, nil
}

// cyclePrice 读商品某周期的售价与成本价（product_prices）。
func (r *instanceDetailRepository) cyclePrice(ctx context.Context, productID uint64, cycle string) (sale, cost float64, ok bool) {
	var row struct {
		Price     float64
		CostPrice float64
	}
	err := r.db.WithContext(ctx).Table("product_prices").
		Select("price, cost_price").
		Where("product_id = ? AND cycle = ? AND product_spec_id = 0 AND status = 1", productID, cycle).
		Take(&row).Error
	if err != nil {
		return 0, 0, false
	}
	return row.Price, row.CostPrice, true
}

// productBasePrice 读商品基础价与成本（products.price / cost_price）。
// 仅用作周期矩阵缺行时的降级口径：没有矩阵意味着这个商品没按周期定价，
// 基础价就是唯一已知的价格。
func (r *instanceDetailRepository) productBasePrice(ctx context.Context, productID uint64) (sale, cost float64) {
	var row struct {
		Price     float64
		CostPrice float64
	}
	err := r.db.WithContext(ctx).Table("products").
		Select("price, cost_price").Where("id = ?", productID).Take(&row).Error
	if err != nil {
		return 0, 0
	}
	return row.Price, row.CostPrice
}

// ProductCyclePrices 读商品价格矩阵的全部周期价（账期调整对话框比价用）。
//
// 只取 product_spec_id = 0（商品级矩阵）与 status = 1（启用中）的行：
// SKU 级矩阵（spec_id > 0）是特定规格的价，套到实例上会给出错误的比价基准。
func (r *instanceDetailRepository) ProductCyclePrices(ctx context.Context, productID uint64) (map[string]ProductCyclePrice, error) {
	out := map[string]ProductCyclePrice{}
	if productID == 0 {
		return out, nil
	}
	var rows []struct {
		Cycle     string
		Price     float64
		CostPrice float64
	}
	err := r.db.WithContext(ctx).Table("product_prices").
		Select("cycle, price, cost_price").
		Where("product_id = ? AND product_spec_id = 0 AND status = 1", productID).
		Scan(&rows).Error
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		cycle := billingcycle.Normalize(row.Cycle)
		if cycle == "" {
			continue
		}
		out[cycle] = ProductCyclePrice{Sale: row.Price, Cost: row.CostPrice}
	}
	return out, nil
}

// ReinstallOSOptions 读商品可用的操作系统镜像。
//
// 两个来源按链路取（第二个返回值报告是否命中）：
//   - 对接商品（source_mode=upstream）：上游镜像 raw_specs 里 os 配置项的 sub 列表，
//     其 option_name 形如 "12|CentOS^CentOS-7.6.1810-x64"，取竖线前的数字作为
//     下发给上游的配置子项 ID（与开通时用同一套取值，保证重装与开通口径一致）；
//   - 自营商品：product_config_options 里 option_key=os 的子项（运营在商品里配的镜像档）。
//
// 两个来源都为空返回 (nil,false,nil)：调用方据此回落渠道镜像目录或提示手填。
func (r *instanceDetailRepository) ReinstallOSOptions(ctx context.Context, productID uint64) ([]dto.ReinstallImage, bool, error) {
	if productID == 0 {
		return nil, false, nil
	}
	var row struct {
		SourceMode      string
		SourceProductID uint64
	}
	err := r.db.WithContext(ctx).Table("products").
		Select("source_mode, source_product_id").
		Where("id = ?", productID).Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	// 对接链路：上游镜像的 os 配置子项。
	if row.SourceMode == "upstream" && row.SourceProductID != 0 {
		var raw struct {
			RawSpecs string
		}
		if e := r.db.WithContext(ctx).Table("resource_products").
			Select("raw_specs").Where("id = ?", row.SourceProductID).Take(&raw).Error; e == nil {
			if imgs := osImagesFromRawSpecs(raw.RawSpecs); len(imgs) > 0 {
				return imgs, true, nil
			}
		}
	}
	// 自营链路：商品自身的 os 配置项子项。
	images, oerr := r.osImagesFromProductOptions(ctx, productID)
	if oerr != nil {
		return nil, false, oerr
	}
	if len(images) > 0 {
		return images, true, nil
	}
	return nil, false, nil
}

// osImagesFromProductOptions 读商品配置项里 option_key=os 的子项（自营链路镜像档）。
func (r *instanceDetailRepository) osImagesFromProductOptions(ctx context.Context, productID uint64) ([]dto.ReinstallImage, error) {
	var rows []struct {
		SubName string
		SubKey  string
	}
	err := r.db.WithContext(ctx).Table("product_config_options AS o").
		Select("s.option_name AS sub_name, s.source_key AS sub_key").
		Joins("JOIN product_config_options_sub AS s ON s.option_id = o.id").
		Where("o.product_id = ? AND (o.option_key = ? OR lower(o.option_name) LIKE ?)", productID, "os", "%os%").
		Order("o.sort_order ASC, s.sort_order ASC, s.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.ReinstallImage, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		value := strings.TrimSpace(row.SubKey)
		if value == "" {
			// 子项没登记 source_key 时退回用名称里的数字前缀（上游形态 "62|CentOS-7.9"）。
			if idx := strings.Index(row.SubName, "|"); idx > 0 {
				value = strings.TrimSpace(row.SubName[:idx])
			}
		}
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, dto.ReinstallImage{Value: value, Label: trimOptionValue(row.SubName)})
	}
	return out, nil
}

// ============================ 更换对接商品候选 ============================

// RebindCandidates 读更换对接商品的候选项。
//
// 本地商品：全部未删除商品（跨链路也能换，但前端会按渠道/链路给出提示）；
// 上游镜像：**仅本实例同渠道**的 resource_products —— 跨渠道换绑会被服务层拒绝
// （价格与配置口径会错位），所以这里干脆不列出来，运营不会选到必失败的项。
func (r *instanceDetailRepository) RebindCandidates(ctx context.Context, providerID uint64, resale bool) (*dto.RebindProductCandidates, error) {
	out := &dto.RebindProductCandidates{
		SellProducts:     []dto.RebindProductCandidate{},
		UpstreamProducts: []dto.RebindProductCandidate{},
		ProviderID:       providerID,
	}

	if providerID != 0 {
		var providerName string
		if err := r.db.WithContext(ctx).Table("resource_providers").
			Select("name").Where("id = ?", providerID).Scan(&providerName).Error; err == nil {
			out.ProviderName = providerName
		}
	}

	// 本地售出商品候选。
	var sellRows []struct {
		ID               uint64
		Code             string
		Name             string
		Price            float64
		CostPrice        float64
		Status           int
		SourceMode       string
		SourceProviderID uint64
	}
	err := r.db.WithContext(ctx).Table("products").
		Select("id, code, name, price, cost_price, status, source_mode, source_provider_id").
		Where("deleted_at IS NULL").
		Order("id DESC").
		Scan(&sellRows).Error
	if err != nil {
		return nil, err
	}
	if len(sellRows) > 0 {
		ids := make([]uint64, 0, len(sellRows))
		for _, row := range sellRows {
			ids = append(ids, row.ID)
		}
		cycles, cerr := r.cycleListByProduct(ctx, ids)
		if cerr != nil {
			return nil, cerr
		}
		for _, row := range sellRows {
			cand := dto.RebindProductCandidate{
				ID:        row.ID,
				Name:      row.Name,
				Code:      row.Code,
				Price:     row.Price,
				CostPrice: row.CostPrice,
				Cycles:    cycles[row.ID],
			}
			if cand.Cycles == nil {
				cand.Cycles = []string{}
			}
			if row.Status != 1 {
				cand.Disabled = true
				cand.DisabledReason = "已下架：换上后续费无法按该商品算价"
			}
			out.SellProducts = append(out.SellProducts, cand)
		}
	}

	// 上游镜像候选：仅同渠道，且仅对接链路。
	if !resale {
		out.Hint = "该实例为自营链路（机器由本平台开通），不存在对接的上游镜像；这里只列出可更换的本地售出商品。"
		return out, nil
	}
	if providerID != 0 {
		var upRows []struct {
			ID         uint64
			Name       string
			SalePrice  float64
			CostPrice  float64
			ProviderID uint64
		}
		uerr := r.db.WithContext(ctx).Table("resource_products").
			Select("id, name, sale_price, cost_price, provider_id").
			Where("provider_id = ?", providerID).
			Order("id DESC").
			Scan(&upRows).Error
		if uerr != nil {
			return nil, uerr
		}
		for _, row := range upRows {
			out.UpstreamProducts = append(out.UpstreamProducts, dto.RebindProductCandidate{
				ID:           row.ID,
				Name:         row.Name,
				ProviderName: out.ProviderName,
				Price:        row.SalePrice,
				CostPrice:    row.CostPrice,
			})
		}
		out.Hint = "上游镜像只列出与本实例同渠道的商品；跨渠道换绑会导致价格与配置口径错位，系统会拒绝。"
	} else {
		out.Hint = "该实例未绑定渠道，无上游镜像候选。"
	}
	return out, nil
}

// cycleListByProduct 批量读商品已配置价格的周期档（product_prices.status=1）。
func (r *instanceDetailRepository) cycleListByProduct(ctx context.Context, productIDs []uint64) (map[uint64][]string, error) {
	out := map[uint64][]string{}
	if len(productIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ProductID uint64
		Cycle     string
	}
	err := r.db.WithContext(ctx).Table("product_prices").
		Select("product_id, cycle").
		Where("product_id IN ? AND product_spec_id = 0 AND status = 1", productIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		cycle := billingcycle.Normalize(row.Cycle)
		if cycle == "" {
			continue
		}
		out[row.ProductID] = append(out[row.ProductID], cycle)
	}
	return out, nil
}

// osImagesFromRawSpecs 从上游镜像快照提取 os 配置子项作为可重装镜像。
//
// 取值口径与开通链路（buildCartConfigOption）严格一致：下发给上游的是
// **配置子项自身的 id 字段**（sub.id，如 565664），不是 option_name 里的数字前缀。
//
// 实测更正（本轮踩到）：option_name 形态是 "75|Debian^Debian-12.0_x64"，其中
// 75 只是上游的面板展示序号，**不是**下发给 API 的值。用 75 重装会被上游拒绝
// （code=406 操作系统错误），因为真实子项 id 是 565664 —— 开通订单的
// configoption 里存的就是这个值（{"163751":"565664"}）。此前按名称前缀取值，
// 列表看起来正常（名称能洗出来），一点提交就报错。
func osImagesFromRawSpecs(rawSpecs string) []dto.ReinstallImage {
	if strings.TrimSpace(rawSpecs) == "" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(rawSpecs), &m); err != nil {
		return nil
	}
	groups, _ := m["config_groups"].([]interface{})
	if len(groups) == 0 {
		return nil
	}
	out := make([]dto.ReinstallImage, 0, 16)
	seen := map[string]bool{}
	for _, g := range groups {
		gm, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		options, _ := gm["options"].([]interface{})
		for _, o := range options {
			om, ok := o.(map[string]interface{})
			if !ok {
				continue
			}
			name := strings.TrimSpace(fmt.Sprintf("%v", om["option_name"]))
			// 只认 os 选项（上游形如 "os|操作系统"）；其余配置项不是可重装镜像。
			if !isOSOption(name) {
				continue
			}
			subs, _ := om["sub"].([]interface{})
			for _, s := range subs {
				sm, ok := s.(map[string]interface{})
				if !ok {
					continue
				}
				if hidden := sm["hidden"]; hidden != nil {
					if h, ok := toFloat(hidden); ok && h != 0 {
						continue // 上游隐藏项不下发（与开通一致）
					}
				}
				subName := strings.TrimSpace(fmt.Sprintf("%v", sm["option_name"]))
				value := subItemID(sm)
				if value == "" || value == "0" || seen[value] {
					continue
				}
				seen[value] = true
				out = append(out, dto.ReinstallImage{
					Value: value,
					Label: osImageLabel(subName),
					Group: osGroupOf(subName),
				})
			}
		}
	}
	return out
}

// subItemID 取配置子项下发给上游的 ID。
//
// 优先用 sub.id（开通链路用的就是它）；缺失时才退化为 option_name 的数字前缀
// （部分手工维护的配置只有名称前缀可辨认）。两种形态的差别见 osImagesFromRawSpecs 注释。
func subItemID(sub map[string]interface{}) string {
	if raw, ok := sub["id"]; ok && raw != nil {
		if v, ok := toFloat(raw); ok && v > 0 {
			return strconv.FormatInt(int64(v), 10)
		}
	}
	name := strings.TrimSpace(fmt.Sprintf("%v", sub["option_name"]))
	if idx := strings.Index(name, "|"); idx > 0 {
		if prefix := strings.TrimSpace(name[:idx]); prefix != "" {
			return prefix
		}
	}
	return ""
}

// isOSOption 判断配置项是否为操作系统项（"os|操作系统" / "os" / "操作系统"）。
func isOSOption(name string) bool {
	key := name
	if idx := strings.Index(name, "|"); idx > 0 {
		key = name[:idx]
	}
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "os" || key == "os_image" || key == "image" {
		return true
	}
	label := name
	if idx := strings.Index(name, "|"); idx > 0 && idx+1 < len(name) {
		label = name[idx+1:]
	}
	return strings.Contains(label, "操作系统") || strings.Contains(label, "镜像")
}

// osGroupOf 从镜像名提取系列分组（"12|CentOS^CentOS-7.6.1810-x64" → "CentOS"）。
func osGroupOf(subName string) string {
	v := osImageLabel(subName)
	if idx := strings.IndexAny(v, " -_"); idx > 0 {
		return v[:idx]
	}
	return v
}

// osImageLabel 清洗上游镜像名用于展示。
//
// 上游形态是 "12|CentOS^CentOS-7.9.2111-x64"：
//
//	竖线前 = 配置子项 ID（下发用，不展示）
//	^ 前   = 系列名（CentOS）
//	^ 后   = 完整镜像名（CentOS-7.9.2111-x64）—— 这才是运营要看的
//
// 通用的 trimOptionValue 会把两段拼在一起（"CentOSCentOS-7.9.2111-x64"，
// 实测重装下拉里就是这么显示的），故这里单独收口：优先取 ^ 后的完整名。
func osImageLabel(subName string) string {
	s := strings.TrimSpace(subName)
	if i := strings.Index(s, "|"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, "^"); i >= 0 {
		if tail := strings.TrimSpace(s[i+1:]); tail != "" {
			return tail
		}
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// ============================ 售出商品 ============================

// Product 读售出商品（含渠道名/类型，便于判断能否改配置）。
func (r *instanceDetailRepository) Product(ctx context.Context, productID uint64) (*dto.ProductInfo, error) {
	if productID == 0 {
		return nil, nil
	}
	var row struct {
		ID                 uint64
		Code               string
		Name               string
		SourceMode         string
		SourceProviderID   uint64
		SourceProductID    uint64
		PriceModel         string
		Price              float64
		CostPrice          float64
		UpstreamMarkupType string
		UpstreamMarkupVal  float64
		Status             int
		ProviderName       string
		ProviderType       string
	}
	err := r.db.WithContext(ctx).Table("products AS p").
		Select(`p.id, p.code, p.name, p.source_mode, p.source_provider_id, p.source_product_id,
			p.price_model, p.price, p.cost_price, p.upstream_markup_type, p.upstream_markup_value, p.status,
			pr.name AS provider_name, pr.provider_type AS provider_type`).
		Joins("LEFT JOIN resource_providers AS pr ON pr.id = p.source_provider_id").
		Where("p.id = ?", productID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &dto.ProductInfo{
		ID:               row.ID,
		Code:             row.Code,
		Name:             row.Name,
		SourceMode:       row.SourceMode,
		SourceProviderID: row.SourceProviderID,
		SourceProvider:   row.ProviderName,
		ProviderType:     row.ProviderType,
		SourceProductID:  row.SourceProductID,
		PriceModel:       row.PriceModel,
		Price:            row.Price,
		CostPrice:        row.CostPrice,
		MarkupType:       row.UpstreamMarkupType,
		MarkupValue:      row.UpstreamMarkupVal,
		Status:           row.Status,
	}, nil
}

// ============================ 上游商品 ============================

// upstreamRaw 上游镜像快照（resource_products.raw_specs）中本页需要的子集。
type upstreamRaw struct {
	GID          int64                    `json:"gid"`
	GroupName    string                   `json:"group_name"`
	PayType      payTypeField             `json:"pay_type"`
	ProviderID   uint64                   `json:"provider_id"`
	ListPrice    float64                  `json:"list_price"`
	CostMonthly  float64                  `json:"cost_price_monthly"`
	PricingsCost []map[string]interface{} `json:"product_pricings"`
	PricingsList []map[string]interface{} `json:"product_pricings_list"`
	Discount     map[string]interface{}   `json:"upstream_discount"`
}

// payTypeField 上游 pay_type 的两种落库形态：
//
//	对象：{"pay_type":"recurring","pay_hour_cycle":"720",...}
//	字符串：上面对象被再序列化成字符串（历史同步按 text 存过）
//
// 只当 string 解析会把整段 JSON 原样塞进详情页（实测「上游商品」卡片把 pay_type
// 渲染成一串 {"pay_type":"recurring"...}），而 renewable 判定跟着失真（显示不可续费）。
type payTypeField struct {
	Code string // recurring / onetime ...
}

// UnmarshalJSON 兼容对象、JSON 字符串（含再序列化）、普通字符串三种输入。
func (p *payTypeField) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		var inner string
		if err := json.Unmarshal(data, &inner); err != nil {
			return nil // 非法字符串按缺失处理，不让整段 raw 解析失败
		}
		inner = strings.TrimSpace(inner)
		if strings.HasPrefix(inner, "{") {
			// 再序列化形态：递归解析出真正的 code。
			var nested struct {
				PayType string `json:"pay_type"`
			}
			if err := json.Unmarshal([]byte(inner), &nested); err == nil {
				p.Code = nested.PayType
			}
			return nil
		}
		p.Code = inner
		return nil
	}
	if trimmed[0] == '{' {
		var nested struct {
			PayType string `json:"pay_type"`
		}
		if err := json.Unmarshal(data, &nested); err == nil {
			p.Code = nested.PayType
		}
		return nil
	}
	return nil
}

// Upstream 读对接商品的上游信息；同时返回该实例当前周期的上游续费金额。
//
// 第二个返回值是"上游续费金额"（上游标价口径，即我们向上游下单时上游收的价），
// 非对接商品或无该周期价时为 0。
func (r *instanceDetailRepository) Upstream(ctx context.Context, sourceProductID uint64, billingMode string) (*dto.UpstreamInfo, float64, error) {
	if sourceProductID == 0 {
		return nil, 0, nil
	}
	var row struct {
		ID           uint64
		ProviderID   uint64
		ProviderName string
		UpstreamID   string
		Name         string
		GroupID      int64
		GroupName    string
		RawSpecs     string
		SalePrice    float64
		CostPrice    float64
	}
	err := r.db.WithContext(ctx).Table("resource_products AS rp").
		Select(`rp.id, rp.provider_id, rp.upstream_id, rp.name, rp.group_id, rp.group_name, rp.raw_specs,
			rp.sale_price, rp.cost_price, pr.name AS provider_name`).
		Joins("LEFT JOIN resource_providers AS pr ON pr.id = rp.provider_id").
		Where("rp.id = ?", sourceProductID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	raw := parseUpstreamRaw(row.RawSpecs)
	out := &dto.UpstreamInfo{
		ResourceProductID: row.ID,
		ProviderID:        row.ProviderID,
		ProviderName:      row.ProviderName,
		UpstreamID:        row.UpstreamID,
		Name:              row.Name,
		GroupID:           firstNonZeroInt64(raw.GID, row.GroupID),
		GroupName:         firstNonEmpty(raw.GroupName, row.GroupName),
		ListPrice:         firstNonZeroFloat(raw.ListPrice, row.SalePrice),
		CostPrice:         firstNonZeroFloat(raw.CostMonthly, row.CostPrice),
		PayType:           raw.PayType.Code,
		// 周期型才可续费；未标注（老镜像没有 pay_type）按可续费处理，避免误挡续费。
		Renewable:  raw.PayType.Code == "" || raw.PayType.Code == "recurring",
		ListCycles: cycleMap(raw.PricingsList),
		CostCycles: cycleMap(raw.PricingsCost),
	}
	if d, ok := raw.Discount["desc"].(string); ok && d != "" {
		out.DiscountDesc = d
	}
	// 上游续费金额：优先取上游标价（上给我们看的价），没有则取采购价。
	cycle := billingcycle.NormalizeOrDefault(billingMode)
	upstreamRenew := out.ListCycles[cycle]
	if upstreamRenew == 0 {
		upstreamRenew = out.CostCycles[cycle]
	}
	return out, money2(upstreamRenew), nil
}

// cycleNameToKey 上游 pricing 行里的周期字段名（与 billingcycle 规范值的映射）。
var cycleKeys = []struct {
	key   string
	cycle string
}{
	{"hour", billingcycle.Hourly},
	{"day", billingcycle.Daily},
	{"monthly", billingcycle.Monthly},
	{"quarterly", billingcycle.Quarterly},
	{"semiannually", billingcycle.Semiannually},
	{"annually", billingcycle.Annually},
	{"biennially", billingcycle.Biennially},
	{"triennially", billingcycle.Triennially},
	{"onetime", billingcycle.Onetime},
}

// cycleMap 把上游 pricing 行（可能多行、多币种）压成「周期 → 金额」。
// 只取第一行（默认币种行），负值（上游用 -1 表示"该周期不卖"）一律丢弃。
func cycleMap(rows []map[string]interface{}) map[string]float64 {
	out := map[string]float64{}
	if len(rows) == 0 {
		return out
	}
	row := rows[0]
	for _, ck := range cycleKeys {
		if v, ok := row[ck.key]; ok {
			if f, ok2 := toFloat(v); ok2 && f >= 0 {
				out[ck.cycle] = f
			}
		}
	}
	return out
}

// parseUpstreamRaw 解析上游镜像快照（失败返回零值，调用方按"无上游信息"降级）。
func parseUpstreamRaw(raw string) upstreamRaw {
	var out upstreamRaw
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// ============================ 开通履约状态 ============================

// Provision 读来源订单的开通任务状态（未创建任务/订单为 0 时返回空串）。
func (r *instanceDetailRepository) Provision(ctx context.Context, orderID uint64) (string, string, error) {
	if orderID == 0 {
		return "", "", nil
	}
	var row struct {
		Status    string
		LastError string
	}
	err := r.db.WithContext(ctx).Table("provision_tasks").
		Select("status, COALESCE(last_error, '') AS last_error").
		Where("order_id = ?", orderID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", "", nil
		}
		return "", "", err
	}
	return row.Status, row.LastError, nil
}

// ============================ 配置项 ============================

// ProductConfigs 读商品配置项（子表）并与「订单选配」「实例取值」合并。
//
// 三层优先级（后者覆盖前者）：
//
//	商品默认（product_config_options 首个可见子项）
//	  → 订单选配（orders.config_options，客户实际所选）
//	    → 实例表字段（cpu/memory/disk/bandwidth/os/region/zone，回源刷新过，最准）
//
// 上游锁定项（对接商品里只有一个可见取值的项）标记 Locked=true，提示运营不可改。
func (r *instanceDetailRepository) ProductConfigs(ctx context.Context, productID uint64, orderConfigOptions string) ([]dto.ConfigItem, error) {
	var rows []struct {
		OptionName string
		OptionType int
		Source     string
		SourceKey  string
		OptionKey  string
		Unit       string
		SubName    string
		SubKey     string
		SubHidden  int
	}
	err := r.db.WithContext(ctx).Table("product_config_options AS o").
		Select(`o.option_name, o.option_type, o.source, o.source_key, o.option_key, o.unit,
			s.option_name AS sub_name, s.source_key AS sub_key, s.hidden AS sub_hidden`).
		Joins("LEFT JOIN product_config_options_sub AS s ON s.option_id = o.id").
		Where("o.product_id = ?", productID).
		Order("o.sort_order ASC, o.id ASC, s.sort_order ASC, s.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	selections := parseSelections(orderConfigOptions)
	// 子项取值字典：选项 source_key → { 子项 source_key: 子项展示名 }。
	// 订单选配里存的是子项 source_key（如 os=62），直接展示只是裸数字；
	// 用本字典翻成中文名（CentOS-7.9），运营才看得懂。
	subLabels := map[string]map[string]string{}
	for _, row := range rows {
		if row.SubKey == "" {
			continue
		}
		optKey := row.SourceKey
		if optKey == "" {
			optKey = row.OptionKey
		}
		if subLabels[optKey] == nil {
			subLabels[optKey] = map[string]string{}
		}
		subLabels[optKey][row.SubKey] = trimOptionValue(row.SubName)
	}
	out := make([]dto.ConfigItem, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		key := configCanonicalKey(row.OptionName, row.SourceKey)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		item := dto.ConfigItem{
			Key:    key,
			Label:  optionLabel(row.OptionName),
			Unit:   row.Unit,
			Source: "product",
			Value:  trimOptionValue(row.SubName),
		}
		// 订单选配覆盖：orders.config_options 用「上游选项 id」或「平台参数名」作键。
		if v, ok := selections[row.SourceKey]; ok && v != "" {
			item.Value, item.Source = resolveSelection(v, subLabels[row.SourceKey]), "order"
		} else if v, ok := selections[key]; ok && v != "" {
			item.Value, item.Source = resolveSelection(v, subLabels[row.SourceKey]), "order"
		} else if v, ok := selections[row.OptionKey]; ok && v != "" {
			item.Value, item.Source = resolveSelection(v, subLabels[row.OptionKey]), "order"
		}
		if row.Source == "upstream" {
			item.Locked = true
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

// resolveSelection 把订单选配里的子项键翻成展示名；查不到则原样返回。
func resolveSelection(value string, labels map[string]string) string {
	if labels == nil {
		return value
	}
	if label, ok := labels[strings.TrimSpace(value)]; ok && label != "" {
		return label
	}
	return value
}

// configKeyAliases 配置项名/上游键 → 机器键。自营商品的 option_name 是中文短名
// （"CPU"/"内存"/"数据盘"），上游商品是 "cpu|CPU" 形式，两者必须落到同一个机器键，
// 否则「实例字段覆盖取值」与前端去重都匹配不上，配置区会出现重复行。
var configKeyAliases = map[string]string{
	"cpu":              "cpu",
	"内存":               "memory",
	"memory":           "memory",
	"数据盘":              "data_disk_size",
	"data_disk_size":   "data_disk_size",
	"datadisk":         "data_disk_size",
	"操作系统":             "os",
	"os":               "os",
	"带宽":               "bw",
	"bw":               "bw",
	"bandwidth":        "bw",
	"ip 数量":            "ip_num",
	"ip数量":             "ip_num",
	"ip_num":           "ip_num",
	"系统盘":              "system_disk_size",
	"system_disk_size": "system_disk_size",
	"区域":               "area",
	"area":             "area",
	"region":           "area",
	"可用区":              "zone",
	"zone":             "zone",
	"node":             "zone",
	"网络类型":             "network_type",
	"network_type":     "network_type",
}

// configCanonicalKey 归一化配置项键：先看 option_name 的中文/英文名，再看 source_key，
// 都不认识时退回归一化后的 option_name（保证任何配置项都能展示，只是覆盖不到实例值）。
func configCanonicalKey(optionName, sourceKey string) string {
	if c, ok := configKeyAliases[strings.ToLower(strings.TrimSpace(normalizeOptionName(optionName)))]; ok {
		return c
	}
	sk := strings.ToLower(strings.TrimSpace(sourceKey))
	if c, ok := configKeyAliases[sk]; ok {
		return c
	}
	if name := strings.ToLower(strings.TrimSpace(normalizeOptionName(optionName))); name != "" {
		return name
	}
	return sk
}

// parseSelections 解析订单落库的客户选配（参数名 → 取值）。
func parseSelections(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// normalizeOptionName 归一化配置项名：上游形如 "cpu|CPU"/"os|操作系统"，取竖线前的机器键。
func normalizeOptionName(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "|"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// optionLabel 取配置项展示名：优先竖线后的中文名，没有则用原名。
func optionLabel(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "|"); i >= 0 && i+1 < len(s) {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

// trimOptionValue 去掉子项名前缀（上游形如 "16|16核"）并清理版本后缀。
func trimOptionValue(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "|"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, "^"); i >= 0 {
		s = s[:i] + s[i+1:]
	}
	return strings.TrimSpace(s)
}

// ============================ 登录连接信息 ============================

// Credentials 读实例登录连接信息（不含明文密码）。
//
// 用户名/端口取自 raw_data（上游详情的原样快照，字段名随平台不同：魔方云
// osuser/port，魔方财务 username/port）。密码**不看 raw_data** ——
// 上游从不回传密码（dcimcloud 该字段实测为空、zjmf 无读密码接口），
// 判定依据是本系统开通信留存 `credential_password`（迁移 100），
// 这里只回答"有没有"，绝不把明文带回详情响应。
func (r *instanceDetailRepository) Credentials(ctx context.Context, instanceID uint64) (*dto.CredentialInfo, error) {
	var row struct {
		RawData            string
		CredentialPassword string
		ProviderType       string
	}
	err := r.db.WithContext(ctx).Table("instances AS i").
		Select("i.raw_data, i.credential_password, pr.provider_type AS provider_type").
		Joins("LEFT JOIN resource_providers AS pr ON pr.id = i.provider_id").
		Where("i.id = ?", instanceID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	var raw map[string]interface{}
	if strings.TrimSpace(row.RawData) != "" {
		_ = json.Unmarshal([]byte(row.RawData), &raw)
	}
	// 密码存在性：本系统留存优先；存量行（迁移前开通）回退看 raw_data 里有没有上游下发的密码。
	hasStored := strings.TrimSpace(row.CredentialPassword) != ""
	if !hasStored {
		hasStored = firstNonEmpty(rawStr(raw, "rootpassword"), rawStr(raw, "password")) != ""
	}
	out := &dto.CredentialInfo{
		Username:    firstNonEmpty(rawStr(raw, "osuser"), rawStr(raw, "username")),
		Port:        firstNonEmpty(rawStr(raw, "port"), defaultPort(row.ProviderType)),
		HasPassword: hasStored,
	}
	if p := rawStr(raw, "panel_pass"); p != "" {
		out.HasPanelPassword = true
	}
	switch {
	case strings.TrimSpace(row.CredentialPassword) != "":
		out.PasswordHint = "开通时由本系统生成并留存，可点「查看密码」获取"
	case out.HasPassword:
		out.PasswordHint = "上游已下发初始密码，可点「查看密码」获取"
	case row.ProviderType == "zjmf":
		out.PasswordHint = "上游财务不回传密码，请用「重置密码」或到上游面板查看"
	default:
		out.PasswordHint = "上游未返回密码，请用「重置密码」设置"
	}
	return out, nil
}

// RevealCredential 读明文密码（供 reveal 接口）。未存返回空串，由调用方提示改用重置密码。
//
// 优先解密本系统开通信留的 credential_password（迁移 100，开通时由我方生成并下发上游，
// 是唯一权威来源）；存量行回退 raw_data 里上游可能下发的字段。
// 解密失败按地雷 L8 显式报错：绝不把 enc:v1: 密文当明文交出去。
func (r *instanceDetailRepository) RevealCredential(ctx context.Context, instanceID uint64) (string, string, string, error) {
	var row struct {
		RawData            string
		CredentialPassword string
	}
	err := r.db.WithContext(ctx).Table("instances").
		Select("raw_data, credential_password").Where("id = ?", instanceID).Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", "", "", nil
		}
		return "", "", "", err
	}
	var raw map[string]interface{}
	if strings.TrimSpace(row.RawData) != "" {
		_ = json.Unmarshal([]byte(row.RawData), &raw)
	}
	username := firstNonEmpty(rawStr(raw, "osuser"), rawStr(raw, "username"))
	port := rawStr(raw, "port")
	if strings.TrimSpace(row.CredentialPassword) != "" {
		plain, derr := credentials.DecryptSecret(row.CredentialPassword, r.secretKey)
		if derr != nil {
			return username, "", port, derr
		}
		return username, plain, port, nil
	}
	return username, firstNonEmpty(rawStr(raw, "rootpassword"), rawStr(raw, "password")), port, nil
}

// defaultPort 平台默认 SSH 端口（仅用于展示兜底）。
func defaultPort(providerType string) string {
	switch providerType {
	case "dcimcloud", "zjmf":
		return "22"
	default:
		return ""
	}
}

// ============================ 小工具 ============================

func rawStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// fmtTimeOpt 时间指针转 RFC3339；空值返回空串。
func fmtTimeOpt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZeroFloat(vals ...float64) float64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func firstNonZeroInt64(vals ...int64) int64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func money2(v float64) float64 {
	return float64(int64(v*100+copySign(0.5, v))) / 100
}

func copySign(mag, sign float64) float64 {
	if sign < 0 {
		return -mag
	}
	return mag
}
