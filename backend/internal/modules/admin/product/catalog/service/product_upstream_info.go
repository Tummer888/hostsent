package service

// 对接商品（source_mode=upstream）的上游对接信息与配置锁定（2026-10-10）。
//
// 上游商品有两种形态，本地必须区别对待：
//
//	自营商品（source_mode=self）：配置项由本系统定义，客户/运营怎么改都由本系统开通；
//	对接商品（source_mode=upstream）：配置项来自上游商品镜像，上游商品本身是"固定规格"的
//	  （如四核8G、带宽 50Mbps），只有部分项（操作系统、网络类型）有多个取值可选。
//
// 于是对接商品的配置项分两类：
//
//	锁定项：上游只有一个可见取值 —— 本地改它必然与上游商品不符，开通即报错。
//	  前端只读展示，履约侧（BuildProvisionRequest）拒绝任何覆盖；
//	可选值项：上游有多个可见取值 —— 允许在下单时选择，随 cart configoption 下发。
//
// 另外上游一次性/免费商品（pay_type=onetime/free）不支持续费，本地据此隐藏续费入口。

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
)

// upstreamPayType 上游 pay_type 文本（recurring/onetime/free + 试用配置）。
type upstreamPayType struct {
	PayType string `json:"pay_type"`
}

// upstreamRawSpecs 上游资源商品镜像中我们消费的字段（其余字段保持原样透传）。
type upstreamRawSpecs struct {
	GID            int64                    `json:"gid"`
	GroupName      string                   `json:"group_name"`
	Description    string                   `json:"description"`
	PayType        string                   `json:"pay_type"`
	ListPrice      float64                  `json:"list_price"`
	CostPriceMonth float64                  `json:"cost_price_monthly"`
	PricingsCost   []map[string]interface{} `json:"product_pricings"`
	PricingsList   []map[string]interface{} `json:"product_pricings_list"`
	ConfigGroups   []upstreamConfigGroup    `json:"config_groups"`
	UpstreamDisc   map[string]interface{}   `json:"upstream_discount"`
	// PriceSource 价格来源：pricings=商品周期价；catalog=上游前台展示价回填；none=上游未定价。
	PriceSource string `json:"price_source"`
	// Catalog 上游前台展示价（cart/index）：价格来源为 catalog 时与之上同源，供运营核对。
	Catalog struct {
		Price        flexNumber `json:"price"`
		SalePrice    flexNumber `json:"sale_price"`
		BillingCycle string     `json:"billing_cycle"`
	} `json:"catalog"`
}

// upstreamConfigGroup / upstreamConfigOption / upstreamConfigSub 为上游 config_groups 的最小视图。
type upstreamConfigGroup struct {
	Name    string                 `json:"name"`
	Options []upstreamConfigOption `json:"options"`
}

type upstreamConfigOption struct {
	OptionName string              `json:"option_name"`
	Source     string              `json:"source"`
	SourceKey  string              `json:"source_key"`
	Sub        []upstreamConfigSub `json:"sub"`
}

type upstreamConfigSub struct {
	OptionName string `json:"option_name"`
	Hidden     int    `json:"hidden"`
}

// parseUpstreamRawSpecs 解析上游资源商品镜像（失败返回零值，调用方按"无上游信息"降级）。
func parseUpstreamRawSpecs(raw string) upstreamRawSpecs {
	var out upstreamRawSpecs
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// upstreamRenewable 判断上游商品是否可续费：只有周期性计费（recurring）可续费；
// onetime（一次性交付）与 free（免费）没有续费语义，本地不应提供续费入口。
func upstreamRenewable(payTypeRaw string) (bool, string) {
	var pt upstreamPayType
	if err := json.Unmarshal([]byte(strings.TrimSpace(payTypeRaw)), &pt); err != nil {
		// 拿不到 pay_type 时按可续费处理（宁可让上游在续费时回绝，也不要误禁正常商品）。
		return true, ""
	}
	switch strings.TrimSpace(pt.PayType) {
	case "onetime":
		return false, "上游为一次性交付商品，不支持续费"
	case "free":
		return false, "上游为免费商品，无需续费"
	default:
		return true, ""
	}
}

// upstreamOptionKey 归一化上游配置项键：取「键|说明」中的键部分并小写
// （"cpu|CPU" → "cpu"、"system_disk_size|系统盘" → "system_disk_size"）。
func upstreamOptionKey(optionName string) string {
	key := strings.TrimSpace(optionName)
	if idx := strings.Index(key, "|"); idx >= 0 {
		key = key[:idx]
	}
	return strings.ToLower(strings.TrimSpace(key))
}

// visibleUpstreamSubs 返回上游配置项的可见取值（hidden=0）。
func visibleUpstreamSubs(opt upstreamConfigOption) []upstreamConfigSub {
	out := make([]upstreamConfigSub, 0, len(opt.Sub))
	for _, sub := range opt.Sub {
		if sub.Hidden != 0 {
			continue
		}
		out = append(out, sub)
	}
	return out
}

// upstreamOptionLocksFromRaw 从上游 config_groups 镜像推导配置项锁定清单。
// 仅统计 source 为空或 upstream 的项（self 项是运营自加的，不受上游约束）。
func upstreamOptionLocksFromRaw(groups []upstreamConfigGroup) []dto.ProductUpstreamOption {
	seen := map[string]bool{}
	out := make([]dto.ProductUpstreamOption, 0, len(groups))
	for _, g := range groups {
		for _, opt := range g.Options {
			if strings.TrimSpace(opt.Source) == string(model.ConfigSourceSelf) {
				continue
			}
			key := upstreamOptionKey(opt.OptionName)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			subs := visibleUpstreamSubs(opt)
			item := dto.ProductUpstreamOption{
				OptionName: strings.TrimSpace(opt.OptionName),
				Key:        key,
				Locked:     len(subs) <= 1,
				SubCount:   len(subs),
			}
			if len(subs) > 0 {
				item.Value = strings.TrimSpace(subs[0].OptionName)
			}
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Locked != out[j].Locked {
			return out[i].Locked
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// upstreamOptionLocks 读取某对接商品的上游配置项锁定清单（商品非对接链路时返回 nil）。
// 用商品配置子表（已落库的上游镜像）判定，避免每次详情都去回查上游。
func (s *productService) upstreamOptionLocks(ctx context.Context, item *model.Product) []dto.ProductUpstreamOption {
	if item == nil || item.SourceMode != model.SourceModeUpstream {
		return nil
	}
	groups, err := s.repo.AllConfigGroupsByProductID(ctx, item.ID)
	if err != nil || len(groups) == 0 {
		// 子表为空时回退读上游镜像 raw_specs（历史克隆商品可能没落子表）。
		if raw := s.upstreamRawFor(ctx, item); raw != nil {
			return upstreamOptionLocksFromRaw(raw.ConfigGroups)
		}
		return nil
	}
	b, err := json.Marshal(groups)
	if err != nil {
		return nil
	}
	var parsed []upstreamConfigGroup
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil
	}
	return upstreamOptionLocksFromRaw(parsed)
}

// upstreamRawFor 读取商品绑定的上游资源商品镜像解析结果；无绑定或读取失败返回 nil。
func (s *productService) upstreamRawFor(ctx context.Context, item *model.Product) *upstreamRawSpecs {
	if item == nil || item.SourceProductID == 0 || s.resourceReader == nil {
		return nil
	}
	rp, err := s.resourceReader.FindByID(ctx, item.SourceProductID)
	if err != nil {
		return nil
	}
	raw := parseUpstreamRawSpecs(rp.RawSpecs)
	return &raw
}

// lockedUpstreamAliases 锁定项在本地开通参数里的所有别名键：
// 上游配置项键 + 本地规格词汇（buildCloneBaseOptions/Specs JSON 用的键）。
// 履约守卫据此把这些键从开通参数里剔除，保证锁定配置不会被本地改动带偏。
func lockedUpstreamAliases(key string) []string {
	switch key {
	case "cpu":
		return []string{"cpu"}
	case "memory", "mem":
		return []string{"memory", "mem"}
	case "system_disk_size", "data_disk_size", "disk":
		return []string{"system_disk_size", "data_disk_size", "disk", "disk_size"}
	case "bw", "bandwidth", "in_bw":
		return []string{"bw", "bandwidth", "in_bw"}
	case "os":
		return []string{"os", "image", "template"}
	case "area", "region":
		return []string{"area", "region"}
	case "node", "zone":
		return []string{"node", "zone"}
	case "network_type", "ip_num":
		return []string{key}
	}
	if key == "" {
		return nil
	}
	return []string{key}
}

// stripLockedUpstreamOptions 剔除被锁定上游配置项的本地覆盖值（履约守卫，原地修改 opts）。
// 返回被剔除的键，供日志留痕（说明本地哪次改动被忽略）。
func stripLockedUpstreamOptions(opts map[string]interface{}, locks []dto.ProductUpstreamOption) []string {
	if len(opts) == 0 || len(locks) == 0 {
		return nil
	}
	var stripped []string
	for _, lock := range locks {
		if !lock.Locked {
			continue
		}
		for _, alias := range lockedUpstreamAliases(lock.Key) {
			if _, ok := opts[alias]; !ok {
				continue
			}
			delete(opts, alias)
			stripped = append(stripped, alias)
		}
	}
	sort.Strings(stripped)
	return stripped
}

// upstreamInfoFor 组装对接商品的上游信息（商品详情展示）；非对接链路或读不到上游镜像返回 nil。
func (s *productService) upstreamInfoFor(ctx context.Context, item *model.Product) *dto.ProductUpstreamInfo {
	if item == nil || item.SourceMode != model.SourceModeUpstream {
		return nil
	}
	raw := s.upstreamRawFor(ctx, item)
	info := &dto.ProductUpstreamInfo{
		ProviderID:  item.SourceProviderID,
		GroupID:     raw.GID,
		GroupName:   raw.GroupName,
		Description: raw.Description,
		PayType:     raw.PayType,
		Renewable:   true,
	}
	if s.resourceReader != nil && item.SourceProductID > 0 {
		if rp, err := s.resourceReader.FindByID(ctx, item.SourceProductID); err == nil {
			info.UpstreamID = rp.UpstreamID
			info.ListPrice = rp.SalePrice
			info.CostPrice = rp.CostPrice
			if info.Description == "" {
				info.Description = parseUpstreamRawSpecs(rp.RawSpecs).Description
			}
		}
	}
	if s.providerReader != nil && item.SourceProviderID > 0 {
		if p, err := s.providerReader.FindByID(ctx, item.SourceProviderID); err == nil {
			info.ProviderName = p.Name
			info.ProviderType = p.ProviderType
		}
	}
	info.DiscountDesc = upstreamDiscountDesc(raw.UpstreamDisc)
	info.PriceSource = raw.PriceSource
	info.CatalogPrice = float64(raw.Catalog.Price)
	info.CatalogSalePrice = float64(raw.Catalog.SalePrice)
	info.CatalogCycle = raw.Catalog.BillingCycle
	info.ListCycles = upstreamCycles(raw.PricingsList)
	info.CostCycles = upstreamCycles(raw.PricingsCost)
	info.Renewable, info.RenewNote = upstreamRenewable(raw.PayType)
	info.Options = s.upstreamOptionLocks(ctx, item)
	fixed := len(info.Options) > 0
	for _, opt := range info.Options {
		if !opt.Locked {
			fixed = false
			break
		}
	}
	info.FixedConfig = fixed
	if info.ProviderID == 0 && info.UpstreamID == "" && info.GroupName == "" && len(info.ListCycles) == 0 {
		// 完全读不到上游镜像（如上游商品已删除）：不展示半截信息，避免误导运营。
		return nil
	}
	return info
}

// defaultUpstreamSelfOptions 对接商品导入时默认注入的客户选配项（source=self）。
// 只注入"上游没有、但下单必须给"的开通选项，避免上游配置项被改动：
//   - 主机名：客户可自定义，留空由系统按规则生成（上游 cart 的 host 参数）。
//     随机密码与端口由上游分配（密码在开通后可见、端口随主机下发），故不做成输入项，
//     只在商品详情页说明，避免给客户"能选其实不能选"的错觉。
func defaultUpstreamSelfOptions() []*model.ProductConfigOption {
	return []*model.ProductConfigOption{
		{
			Source:     model.ConfigSourceSelf,
			SourceKey:  "hostname",
			OptionName: "主机名",
			OptionKey:  "hostname",
			Widget:     model.ConfigWidgetText,
			Required:   false,
			Help:       "留空自动生成（hs- 前缀）；须以字母开头，仅字母/数字/连字符，最长 30 位",
			SortOrder:  10,
		},
	}
}

// upstreamDiscountDesc 折扣描述：优先取镜像里上游侧已写好的中文说明。
func upstreamDiscountDesc(disc map[string]interface{}) string {
	if disc == nil {
		return ""
	}
	if desc, ok := disc["desc"].(string); ok && strings.TrimSpace(desc) != "" {
		return strings.TrimSpace(desc)
	}
	return ""
}

// upstreamCycles 把上游 pricing 行换算为「周期 → 金额」。
// 只取 > 0 的周期（-1=未配置，0=免费，都不作为可售周期展示）。
func upstreamCycles(rows []map[string]interface{}) map[string]float64 {
	if len(rows) == 0 {
		return nil
	}
	out := map[string]float64{}
	for _, pair := range cyclePriceFields() {
		if v, ok := rowFloat(rows[0], pair.field); ok && v > 0 {
			out[pair.cycle] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// cyclePriceFields 周期 → 上游 pricing 价格字段（与上游 config('price_type') 同口径）。
func cyclePriceFields() []struct {
	cycle string
	field string
} {
	return []struct {
		cycle string
		field string
	}{
		{"hourly", "hour"},
		{"daily", "day"},
		{"monthly", "monthly"},
		{"quarterly", "quarterly"},
		{"semiannually", "semiannually"},
		{"annually", "annually"},
		{"biennially", "biennially"},
		{"triennially", "triennially"},
		{"onetime", "onetime"},
	}
}

// rowFloat 读取 pricing 行里的数值（兼容字符串/数字两种下发形态）。
func rowFloat(row map[string]interface{}, key string) (float64, bool) {
	v, ok := row[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		var f float64
		if err := json.Unmarshal([]byte(t), &f); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}
