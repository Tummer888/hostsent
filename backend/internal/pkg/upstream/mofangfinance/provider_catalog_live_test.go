package mofangfinance

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"hostsent/backend/internal/pkg/upstream"
)

// liveCatalogProvider 只读真机上游（haika）适配器：用于验证商品目录/分类/代理折扣抓取。
// 与 provider_live_test.go 的区别：这里**只发 GET 读接口**，不下单、不扣余额，
// 故用独立开关 LIVE_ZJMF_CATALOG=1（不会因误设 LIVE_ZJMF 而触发扣费链路）。
func liveCatalogProvider(t *testing.T) *MoFangFinanceProvider {
	if os.Getenv("LIVE_ZJMF_CATALOG") == "" {
		t.Skip("set LIVE_ZJMF_CATALOG=1 to run read-only mofangfinance catalog probe")
	}
	endpoint := os.Getenv("ZJMF_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://www.haikayun.com/"
	}
	return NewMoFangFinanceProvider(&upstream.ProviderConfig{
		Type:         "mofangfinance",
		APIEndpoint:  endpoint,
		APIKey:       os.Getenv("ZJMF_USER"),
		APISecret:    os.Getenv("ZJMF_PASS"),
		UpstreamType: "zjmf_api",
		Secure:       true,
		Timeout:      40,
	})
}

// TestListProductsLiveReadOnly 只读验证商品抓取的三个关键口径：
//  1. 分类名（cart/all 的分组）落到 RawSpecs.group_name —— 导入弹窗的分类标题来源；
//  2. 代理折扣价（cart/get_product_config 的 flag）落到 product_pricings 且 cost < list；
//  3. 开通参数（config_groups 的 option.id / sub.id）齐备 —— 向上游下单的 configoption 键。
func TestListProductsLiveReadOnly(t *testing.T) {
	p := liveCatalogProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// 直连两个关键接口，先确认协议层可用（失败时能立刻分辨是协议还是聚合逻辑）。
	var all CartAllResp
	if err := p.call(ctx, "live", http.MethodGet, "cart/all", nil, &all); err != nil {
		t.Fatalf("cart/all 失败: %v", err)
	}
	if len(all.Products) == 0 {
		t.Fatalf("cart/all 未返回分组（分类名将全部缺失）")
	}
	t.Logf("cart/all: count=%d currency=%s groups=%d 首个分组=%q(%d 个商品)",
		all.Count, all.Currency, len(all.Products), all.Products[0].Name, len(all.Products[0].Products))

	products, err := p.ListProducts(ctx)
	if err != nil {
		t.Fatalf("ListProducts 失败: %v", err)
	}
	if len(products) == 0 {
		t.Fatalf("ListProducts 返回空")
	}

	named, discounted, lockedCfg, rawPricing := 0, 0, 0, 0
	for _, it := range products {
		if name, _ := it.RawSpecs["group_name"].(string); name != "" {
			named++
		}
		if it.SalePrice > 0 && it.CostPrice > 0 && it.CostPrice < it.SalePrice {
			discounted++
		}
		if _, ok := it.RawSpecs["config_groups"]; ok {
			lockedCfg++
		}
		if _, ok := it.RawSpecs["product_pricings_list"]; ok {
			rawPricing++
		}
	}
	t.Logf("商品 %d 个：有分类名 %d、有代理折扣（成本<标价）%d、带开通参数 %d、双价口径 %d",
		len(products), named, discounted, lockedCfg, rawPricing)
	if named == 0 {
		t.Error("没有任何商品拿到分类名（分组解析失效）")
	}
	if rawPricing == 0 {
		t.Error("没有任何商品带 product_pricings_list（标价/代理价双口径失效）")
	}
	if discounted == 0 {
		t.Error("没有任何商品命中代理折扣（flag 解析或折算失效）")
	}
	// 抽样打印一个带折扣的商品，便于人工核对金额。
	for _, it := range products {
		if it.CostPrice > 0 && it.CostPrice < it.SalePrice {
			t.Logf("样例：%s 标价 ¥%.2f → 代理价 ¥%.2f（分组=%v）",
				it.Name, it.SalePrice, it.CostPrice, it.RawSpecs["group_name"])
			break
		}
	}
}

// TestListProductsLiveVisibilityAndCatalogPrice 只读验证本轮新增的三条口径：
//  1. 在售判定：隐藏分组/隐藏商品不再一律标 active（此前 245 条全部"上架"）；
//  2. 分类名派生：上游隐藏的分类（香港|二区 等）能给出派生标签而非"未分类"；
//  3. 活动款价格回填：product_pricings 全 0/-1 的商品用上游前台展示价补齐（代理价为正）。
func TestListProductsLiveVisibilityAndCatalogPrice(t *testing.T) {
	p := liveCatalogProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	products, err := p.ListProducts(ctx)
	if err != nil {
		t.Fatalf("ListProducts 失败: %v", err)
	}
	derived, groupHidden, onSale, catalogPrice, unpriced := 0, 0, 0, 0, 0
	groupFlagSeen := map[int64]map[float64]bool{}
	samples := []string{}
	for _, it := range products {
		visibility, _ := it.RawSpecs["visibility"].(string)
		source, _ := it.RawSpecs["group_name_source"].(string)
		priceSource, _ := it.RawSpecs["price_source"].(string)
		if source == "derived" {
			derived++
		}
		if visibility == "group_hidden" {
			groupHidden++
		}
		if it.Status == "active" {
			onSale++
		}
		if priceSource == "catalog" {
			catalogPrice++
			if it.CostPrice <= 0 || it.SalePrice <= 0 {
				samples = append(samples, it.Name+"（回填后仍无正价）")
			}
		}
		if it.CostPrice <= 0 && it.SalePrice <= 0 {
			unpriced++
		}
		gid, _ := it.RawSpecs["gid"].(int64)
		disc, _ := it.RawSpecs["upstream_discount"].(map[string]interface{})
		rate, _ := disc["bates"].(float64)
		if groupFlagSeen[gid] == nil {
			groupFlagSeen[gid] = map[float64]bool{}
		}
		groupFlagSeen[gid][rate] = true
	}
	t.Logf("商品 %d 个：在售 %d、隐藏分组 %d、派生分类 %d、前台价回填 %d、未定价 %d",
		len(products), onSale, groupHidden, derived, catalogPrice, unpriced)
	if onSale == 0 || onSale == len(products) {
		t.Errorf("在售判定异常：active=%d/%d（隐藏分组商品不应显示上架）", onSale, len(products))
	}
	if groupHidden == 0 {
		t.Error("没有任何商品被判为隐藏分组（visibility 判定失效）")
	}
	if derived == 0 {
		t.Error("没有任何商品拿到派生分类名（隐藏分组将显示为未分类）")
	}
	if catalogPrice == 0 {
		t.Error("没有任何商品命中前台价回填（活动款价格会显示 0）")
	}
	// 同一分组内折扣应可能出现不同值（按商品命中），这是本轮修正的核心：
	// 旧实现按分组采样，会把整组都算成同一个折扣。
	mixed := 0
	for gid, rates := range groupFlagSeen {
		if len(rates) > 1 {
			mixed++
			t.Logf("分组 %d 内出现多种折扣：%v", gid, rates)
		}
	}
	if mixed == 0 {
		t.Log("提示：本次未观察到同组多折扣（上游折扣数据可能刚好一致）")
	}
	for _, s := range samples {
		t.Errorf("活动款回填后仍无正价：%s", s)
	}
}
