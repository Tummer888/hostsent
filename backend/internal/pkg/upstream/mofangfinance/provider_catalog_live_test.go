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
