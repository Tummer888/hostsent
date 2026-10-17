//go:build live

package server

// SPU/SKU 分离与建品定价入口收口的联调用例（doc128）。
//
// 这些用例打真实库（与容器内 backend 同一条代码路径），守的是本轮的三件事：
//  1. 逐规格逐周期定价真的落进 product_prices，且**每个规格都覆盖全部可售周期**
//     （某规格只要有任意一行，它就是该规格所有周期的权威，缺周期客户就买不了）；
//  2. SellableCycles 能返回各 SKU 的自有周期（改前它只取 spec_id=0 的行，
//     bySpec 恒为空，客户选规格后看到的是商品级周期，下单才被拒）；
//  3. 商品展示价（products.price）由最低规格价回填，而不是留 0。

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"testing"
	"time"
)

func nowUnixNano() int64 { return time.Now().UnixNano() }

// liveSelfTemplateID 取当前库里带 SKU 轴的自营规格模板。
//
// 不写死 ID：联调/演示环境常重建模板，写死的 ID 会随库清理而失效
// （实测：模板 21 被删后用例直接报"规格模板不存在"）。
// liveAxis 模板里一个参与正交的轴的取值与默认值。
type liveAxis struct {
	key      string
	values   []string
	baseline string
}

// liveTemplate 用例需要的模板信息（轴 + 默认平台参数）。
type liveTemplate struct {
	id     uint64
	axes   []liveAxis
	params map[string]any
}

// loadLiveTemplate 读取首个带 SKU 轴的模板，并把它的轴取值解析出来。
//
// 不写死模板 ID 与取值：联调/演示环境的模板常被重建（实测模板 21 被删、
// 模板 22 的取值档位也不一样）。用例按**库里现有模板**推导期望值，
// 才不会被数据变动误判成功能回归。
func loadLiveTemplate(t *testing.T, h *liveHarness, admin string) liveTemplate {
	t.Helper()
	status, body := h.do(http.MethodGet,
		"/api/v1/admin/product/spec/templates?page=1&page_size=100&status=1", admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("拉取规格模板失败 status=%d body=%v", status, body)
	}
	items, _ := body["data"].(map[string]any)["items"].([]any)
	for _, raw := range items {
		tpl, _ := raw.(map[string]any)
		selections, _ := tpl["option_selections"].(map[string]any)
		var axes []liveAxis
		for key, cfg := range selections {
			opt, _ := cfg.(map[string]any)
			if opt["sku_axis"] != true {
				continue
			}
			rawValues, _ := opt["values"].([]any)
			if len(rawValues) == 0 {
				continue
			}
			axis := liveAxis{key: key}
			for _, v := range rawValues {
				if s, ok := v.(string); ok {
					axis.values = append(axis.values, s)
				}
			}
			if len(axis.values) < 2 {
				continue // 单值不构成维度
			}
			// 基线取模板默认值，非法则首值（与后端 resolveAxisBaseline 同口径）。
			def, _ := opt["default"].(string)
			axis.baseline = axis.values[0]
			for _, v := range axis.values {
				if v == def {
					axis.baseline = v
				}
			}
			axes = append(axes, axis)
		}
		if len(axes) == 0 {
			continue
		}
		sort.Slice(axes, func(i, j int) bool { return axes[i].key < axes[j].key })
		id, _ := tpl["id"].(float64)
		params, _ := tpl["platform_params"].(map[string]any)
		return liveTemplate{id: uint64(id), axes: axes, params: params}
	}
	t.Skip("库里没有带 SKU 轴的规格模板，跳过")
	return liveTemplate{}
}

// comboCount 该模板正交展开的规格数。
func (tpl liveTemplate) comboCount() int {
	n := 1
	for _, a := range tpl.axes {
		n *= len(a.values)
	}
	return n
}

// combos 展开全部取值组合（与前端 rowAxisSpecs / 后端 cartesian 同序）。
func (tpl liveTemplate) combos() []map[string]string {
	out := []map[string]string{{}}
	for _, axis := range tpl.axes {
		next := make([]map[string]string, 0, len(out)*len(axis.values))
		for _, base := range out {
			for _, v := range axis.values {
				combo := map[string]string{}
				for k, val := range base {
					combo[k] = val
				}
				combo[axis.key] = v
				next = append(next, combo)
			}
		}
		out = next
	}
	return out
}

// specCodeOf 由取值组合推导规格编码：CPU 与内存合成 NcNG 段（与后端
// deriveAxisSpecCode 同口径），其它轴按「键+值」追加。用例只用 cpu/memory 两轴，
// 该口径与生产一致；遇到别的轴组合时用例会因找不到规格而失败，那时再扩。
func (tpl liveTemplate) specCodeOf(combo map[string]string) string {
	cpu, mem := 0, 0
	for key, value := range combo {
		switch key {
		case "cpu":
			cpu, _ = strconv.Atoi(value)
		case "memory":
			mb, _ := strconv.Atoi(value)
			mem = mb / 1024
		}
	}
	return fmt.Sprintf("general-%dc%dg", cpu, mem)
}

// specValues 补上模板可能缺的必填原子取值（系统盘），否则上架门禁会拦。
//
// 只补原子取值，不能把 platform_params 混进来：那是"平台写参数"（ip_num / nat_*），
// 键名不是原子 key，塞进 specs 会直接被原子字典校验拒掉。
func (tpl liveTemplate) specValues() map[string]any {
	return map[string]any{"storage.system.size": 60}
}

func liveSelfTemplateID(t *testing.T, h *liveHarness, admin string) uint64 {
	t.Helper()
	status, body := h.do(http.MethodGet,
		"/api/v1/admin/product/spec/templates?page=1&page_size=100&status=1", admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("拉取规格模板失败 status=%d body=%v", status, body)
	}
	data, _ := body["data"].(map[string]any)
	items, _ := data["items"].([]any)
	for _, raw := range items {
		tpl, _ := raw.(map[string]any)
		selections, _ := tpl["option_selections"].(map[string]any)
		for _, cfg := range selections {
			opt, _ := cfg.(map[string]any)
			if opt["sku_axis"] == true {
				if id, _ := tpl["id"].(float64); id > 0 {
					return uint64(id)
				}
			}
		}
	}
	t.Skip("库里没有带 SKU 轴的规格模板，跳过")
	return 0
}

func toStrings(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// liveAxisPrice 一组逐规格定价的期望值，供提交与断言共用。
type liveAxisPrice struct {
	cpu     string
	mem     string
	monthly float64
	cycles  map[string]float64
}

func (p liveAxisPrice) specCode() string {
	memGB, _ := strconv.Atoi(p.mem)
	return fmt.Sprintf("general-%sc%dg", p.cpu, memGB/1024)
}

// buildLiveAxisPrices 造 4核×4内存 的逐规格定价：月付 = 核价 + 内存加价，
// 季付 = 月付×3、年付 = 月付×10（各规格各自算，正是"逐规格定价"要表达的东西）。
func buildLiveAxisPrices() []liveAxisPrice {
	base := map[string]float64{"4": 100, "8": 180, "12": 260, "16": 340}
	add := map[string]float64{"1024": 0, "2048": 20, "4096": 40, "8192": 80}
	out := make([]liveAxisPrice, 0, 16)
	for _, cpu := range []string{"4", "8", "12", "16"} {
		for _, mem := range []string{"1024", "2048", "4096", "8192"} {
			monthly := base[cpu] + add[mem]
			out = append(out, liveAxisPrice{
				cpu: cpu, mem: mem, monthly: monthly,
				cycles: map[string]float64{
					"monthly":   monthly,
					"quarterly": monthly * 3,
					"annually":  monthly * 10,
				},
			})
		}
	}
	return out
}

// TestLiveAxisSpecPricingEndToEnd 逐规格单独定价：16 个（按模板实际档位）SKU 各自逐周期定价。
func TestLiveAxisSpecPricingEndToEnd(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	tpl := loadLiveTemplate(t, h, admin)

	// 按模板实际取值造价：月付 = 100 + 该组合的档位序号 × 10，
	// 季付 ×3、年付 ×10。这样每个规格价都不同，能证明"逐规格"确实生效。
	combos := tpl.combos()
	axisPrices := make([]map[string]any, 0, len(combos))
	want := map[string]map[string]float64{}
	for i, combo := range combos {
		monthly := float64(100 + i*10)
		cycles := map[string]float64{"monthly": monthly, "quarterly": monthly * 3, "annually": monthly * 10}
		axisPrices = append(axisPrices, map[string]any{
			"values": combo, "price": monthly, "cost_price": math.Round(monthly * 0.6),
			"cycles": cycles,
		})
		want[tpl.specCodeOf(combo)] = cycles
	}

	code := fmt.Sprintf("live-spu-%d", nowUnixNano()%1_000_000)
	payload := map[string]any{
		"code": code, "name": "联调 逐规格定价验证", "category_id": 1,
		"product_type": "cloud_host", "price_model": "monthly", "price": 0,
		"source_mode": "self", "source_provider_id": 61, "stock": -1, "status": 0,
		"spec_templates": []map[string]any{{
			"spec_template_id": tpl.id, "price": 0, "cost_price": 0, "stock": -1,
			"axis_prices":     axisPrices,
			"sellable_cycles": []string{"monthly", "quarterly", "annually"},
			"spec_values":     tpl.specValues(),
		}},
	}
	status, body := h.do(http.MethodPost, "/api/v1/admin/product/products", admin, payload)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("建品失败 status=%d body=%v", status, body)
	}
	productID := uint64(digNumber(body, "data", "id"))
	t.Cleanup(func() { h.deleteProduct(productID) })

	if n := len(body["data"].(map[string]any)["generated_specs"].([]any)); n != len(combos) {
		t.Fatalf("应生成 %d 个规格，实际 %d", len(combos), n)
	}
	// 展示价 = 最低规格价 100。
	if got := digNumber(body, "data", "price"); math.Abs(got-100) > 0.005 {
		t.Fatalf("展示价应为 100，实际 %v", got)
	}

	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/users/order-config?product_id=%d", productID), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("取代下单配置失败 status=%d body=%v", status, body)
	}
	specs, _ := body["data"].(map[string]any)["specs"].([]any)
	if len(specs) != len(combos) {
		t.Fatalf("代下单配置应有 %d 个规格，实际 %d", len(combos), len(specs))
	}
	for _, raw := range specs {
		spec, _ := raw.(map[string]any)
		specCode, _ := spec["spec_code"].(string)
		wantCycles, ok := want[specCode]
		if !ok {
			t.Fatalf("出现预期外的规格编码 %s", specCode)
		}
		if price, _ := spec["price"].(float64); math.Abs(price-wantCycles["monthly"]) > 0.005 {
			t.Fatalf("%s 基准价应为 %v，实际 %v", specCode, wantCycles["monthly"], price)
		}
		if cycles := toStrings(spec["cycles"]); len(cycles) != 3 {
			t.Fatalf("%s 应有 3 个可售周期，实际 %v", specCode, cycles)
		}
	}

	// 算价核对：每个规格 × 每个周期逐项验。
	if status, pbody := h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/product/products/%d/publish", productID), admin, nil); status != http.StatusOK || digNumber(pbody, "code") != 0 {
		t.Fatalf("上架失败 status=%d body=%v", status, pbody)
	}
	uid, username := h.registerUser("spuprice")
	user := h.userToken(uid, username)
	for specCode, cycles := range want {
		for cycle, amount := range cycles {
			status, qbody := h.do(http.MethodPost, "/api/v1/uc/orders/quote", user, map[string]any{
				"product_id": productID, "spec_code": specCode, "cycle": cycle, "quantity": 1,
			})
			if status != http.StatusOK || digNumber(qbody, "code") != 0 {
				t.Fatalf("%s/%s 预结算失败 status=%d body=%v", specCode, cycle, status, qbody)
			}
			if got := digNumber(qbody, "data", "original_amount"); math.Abs(got-amount) > 0.005 {
				t.Fatalf("%s/%s 算价应为 %v，实际 %v", specCode, cycle, amount, got)
			}
		}
	}
}

// TestLiveMonthlyBaselineCyclePricing 新表单口径：月付即基准价，长周期由它推导后提交。
//
// 这是建品页现在实际发的东西：base_price = 月付价，base_cycles = 各周期价
// （月付以外的由「一键生成周期价」按优惠月数算出），加价叠在月付之上。
// 断言每月都按"该周期基准 + 加价 × 该周期倍率"落库。
func TestLiveMonthlyBaselineCyclePricing(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	tpl := loadLiveTemplate(t, h, admin)

	// 找到 cpu 轴作为加价轴；基线取模板默认值。
	var cpuAxis *liveAxis
	for i := range tpl.axes {
		if tpl.axes[i].key == "cpu" {
			cpuAxis = &tpl.axes[i]
		}
	}
	if cpuAxis == nil {
		t.Skip("模板没有 cpu 轴，跳过")
	}
	// 选一个非基线的 cpu 取值加价 20。
	var chargedCPU string
	for _, v := range cpuAxis.values {
		if v != cpuAxis.baseline {
			chargedCPU = v
			break
		}
	}
	if chargedCPU == "" {
		t.Skip("cpu 轴没有可加价的非基线取值，跳过")
	}

	baseValues := map[string]string{}
	for _, axis := range tpl.axes {
		baseValues[axis.key] = axis.baseline
	}
	charges := map[string]any{
		"base_values": baseValues,
		"base_price":  100.0,
		"base_cost":   60.0,
		"base_cycles": map[string]float64{"monthly": 100, "quarterly": 300, "annually": 1000},
		"values":      map[string]map[string]float64{"cpu": {chargedCPU: 20}},
	}
	code := fmt.Sprintf("live-base-%d", nowUnixNano()%1_000_000)
	payload := map[string]any{
		"code": code, "name": "联调 月付基准价验证", "category_id": 1,
		"product_type": "cloud_host", "price_model": "monthly", "price": 0,
		"source_mode": "self", "source_provider_id": 61, "stock": -1, "status": 0,
		"spec_templates": []map[string]any{{
			"spec_template_id": tpl.id, "price": 0, "cost_price": 0, "stock": -1,
			"axis_charges":    charges,
			"sellable_cycles": []string{"monthly", "quarterly", "annually"},
			"spec_values":     tpl.specValues(),
		}},
	}
	status, body := h.do(http.MethodPost, "/api/v1/admin/product/products", admin, payload)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("建品失败 status=%d body=%v", status, body)
	}
	productID := uint64(digNumber(body, "data", "id"))
	t.Cleanup(func() { h.deleteProduct(productID) })

	if got := digNumber(body, "data", "price"); math.Abs(got-100) > 0.005 {
		t.Fatalf("展示价应为基准 100，实际 %v", got)
	}
	if status, pbody := h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/product/products/%d/publish", productID), admin, nil); status != http.StatusOK || digNumber(pbody, "code") != 0 {
		t.Fatalf("上架失败 status=%d body=%v", status, pbody)
	}

	// 基线组合 = 100；把 cpu 换成加价档 = 120，长周期按基准倍率放大
	// （季 300/100=3 → +60；年 1000/100=10 → +200）。
	baseCombo := map[string]string{}
	chargedCombo := map[string]string{}
	for _, axis := range tpl.axes {
		baseCombo[axis.key] = axis.baseline
		chargedCombo[axis.key] = axis.baseline
	}
	chargedCombo["cpu"] = chargedCPU

	uid, username := h.registerUser("base")
	user := h.userToken(uid, username)
	cases := []struct {
		combo map[string]string
		cycle string
		want  float64
	}{
		{baseCombo, "monthly", 100},
		{baseCombo, "quarterly", 300},
		{baseCombo, "annually", 1000},
		{chargedCombo, "monthly", 120},
		{chargedCombo, "quarterly", 360},
		{chargedCombo, "annually", 1200},
	}
	for _, c := range cases {
		specCode := tpl.specCodeOf(c.combo)
		status, qbody := h.do(http.MethodPost, "/api/v1/uc/orders/quote", user, map[string]any{
			"product_id": productID, "spec_code": specCode, "cycle": c.cycle, "quantity": 1,
		})
		if status != http.StatusOK || digNumber(qbody, "code") != 0 {
			t.Fatalf("%s/%s 预结算失败 status=%d body=%v", specCode, c.cycle, status, qbody)
		}
		if got := digNumber(qbody, "data", "original_amount"); math.Abs(got-c.want) > 0.005 {
			t.Fatalf("%s/%s 应为 %v，实际 %v", specCode, c.cycle, c.want, got)
		}
	}
}

// deleteProduct 清理联调用例建出的商品及其规格/价格行。
//
// 商品删除接口是软删（products.deleted_at）且**不级联** product_specs / product_prices
// （见 doc128 §六，属存量行为），所以这里显式按表清理，避免联调库越跑越脏。
func (h *liveHarness) deleteProduct(productID uint64) {
	if productID == 0 || h.db == nil {
		return
	}
	h.db.Exec("DELETE FROM product_prices WHERE product_id = ?", productID)
	h.db.Exec("DELETE FROM product_specs WHERE product_id = ?", productID)
	h.db.Exec("DELETE FROM product_history WHERE product_id = ?", productID)
	h.db.Exec("DELETE FROM product_config_options_sub WHERE option_id IN (SELECT id FROM product_config_options WHERE product_id = ?)", productID)
	h.db.Exec("DELETE FROM product_config_options WHERE product_id = ?", productID)
	h.db.Exec("DELETE FROM products WHERE id = ?", productID)
}

// TestLiveCompressedAxisPricing 压缩定价：只填基准价 + 取值加价，后端展开成各 SKU。
func TestLiveCompressedAxisPricing(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	tpl := loadLiveTemplate(t, h, admin)

	var cpuAxis *liveAxis
	for i := range tpl.axes {
		if tpl.axes[i].key == "cpu" {
			cpuAxis = &tpl.axes[i]
		}
	}
	if cpuAxis == nil {
		t.Skip("模板没有 cpu 轴，跳过")
	}
	baseValues := map[string]string{}
	for _, axis := range tpl.axes {
		baseValues[axis.key] = axis.baseline
	}
	// 所有非基线 cpu 取值都加价：序号越大加价越多，逐项可验。
	cpuCharges := map[string]float64{}
	step := 10.0
	for _, v := range cpuAxis.values {
		if v == cpuAxis.baseline {
			continue
		}
		cpuCharges[v] = step
		step += 10
	}
	if len(cpuCharges) == 0 {
		t.Skip("cpu 轴没有可加价的非基线取值，跳过")
	}
	charges := map[string]any{
		"base_values": baseValues,
		"base_price":  100.0,
		"values":      map[string]map[string]float64{"cpu": cpuCharges},
	}
	code := fmt.Sprintf("live-cmp-%d", nowUnixNano()%1_000_000)
	payload := map[string]any{
		"code": code, "name": "联调 压缩定价验证", "category_id": 1,
		"product_type": "cloud_host", "price_model": "monthly", "price": 0,
		"source_mode": "self", "source_provider_id": 61, "stock": -1, "status": 0,
		"spec_templates": []map[string]any{{
			"spec_template_id": tpl.id, "price": 0, "cost_price": 0, "stock": -1,
			"axis_charges":    charges,
			"sellable_cycles": []string{"monthly", "quarterly", "annually"},
			"spec_values":     tpl.specValues(),
		}},
	}
	status, body := h.do(http.MethodPost, "/api/v1/admin/product/products", admin, payload)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("建品失败 status=%d body=%v", status, body)
	}
	productID := uint64(digNumber(body, "data", "id"))
	t.Cleanup(func() { h.deleteProduct(productID) })

	combos := tpl.combos()
	if n := len(body["data"].(map[string]any)["generated_specs"].([]any)); n != len(combos) {
		t.Fatalf("应生成 %d 个规格，实际 %d", len(combos), n)
	}
	// 期望价 = 100 + 该组合 cpu 取值的加价（其余轴不加价）。
	want := map[string]float64{}
	for _, combo := range combos {
		price := 100.0
		if add, ok := cpuCharges[combo["cpu"]]; ok {
			price += add
		}
		want[tpl.specCodeOf(combo)] = price
	}

	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/users/order-config?product_id=%d", productID), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("取代下单配置失败 status=%d body=%v", status, body)
	}
	specs, _ := body["data"].(map[string]any)["specs"].([]any)
	if len(specs) != len(combos) {
		t.Fatalf("应有 %d 个规格，实际 %d", len(combos), len(specs))
	}
	for _, raw := range specs {
		spec, _ := raw.(map[string]any)
		specCode, _ := spec["spec_code"].(string)
		expect, ok := want[specCode]
		if !ok {
			t.Fatalf("出现预期外的规格编码 %s", specCode)
		}
		if price, _ := spec["price"].(float64); math.Abs(price-expect) > 0.005 {
			t.Fatalf("%s 价应为 %v，实际 %v", specCode, expect, price)
		}
		if cycles := toStrings(spec["cycles"]); len(cycles) != 3 {
			t.Fatalf("%s 应有 3 个可售周期，实际 %v", specCode, cycles)
		}
	}
}

// TestLiveOverrideBeatsCompressedCharge 单独定价优先于压缩展开。
func TestLiveOverrideBeatsCompressedCharge(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	tpl := loadLiveTemplate(t, h, admin)

	var cpuAxis *liveAxis
	for i := range tpl.axes {
		if tpl.axes[i].key == "cpu" {
			cpuAxis = &tpl.axes[i]
		}
	}
	if cpuAxis == nil {
		t.Skip("模板没有 cpu 轴，跳过")
	}
	var chargedCPU string
	for _, v := range cpuAxis.values {
		if v != cpuAxis.baseline {
			chargedCPU = v
			break
		}
	}
	if chargedCPU == "" {
		t.Skip("cpu 轴没有非基线取值，跳过")
	}
	baseValues := map[string]string{}
	for _, axis := range tpl.axes {
		baseValues[axis.key] = axis.baseline
	}
	charges := map[string]any{
		"base_values": baseValues,
		"base_price":  200.0,
		"values":      map[string]map[string]float64{"cpu": {chargedCPU: 50}},
	}
	// 把"cpu 用加价档、其余轴取基线"的那个组合单独定价 1 元。
	overrideCombo := map[string]string{}
	for _, axis := range tpl.axes {
		overrideCombo[axis.key] = axis.baseline
	}
	overrideCombo["cpu"] = chargedCPU
	overrideCode := tpl.specCodeOf(overrideCombo)

	code := fmt.Sprintf("live-ovr-%d", nowUnixNano()%1_000_000)
	payload := map[string]any{
		"code": code, "name": "联调 单独定价优先验证", "category_id": 1,
		"product_type": "cloud_host", "price_model": "monthly", "price": 0,
		"source_mode": "self", "source_provider_id": 61, "stock": -1, "status": 0,
		"spec_templates": []map[string]any{{
			"spec_template_id": tpl.id, "price": 0, "cost_price": 0, "stock": -1,
			"axis_charges":    charges,
			"sellable_cycles": []string{"monthly", "quarterly", "annually"},
			"spec_values":     tpl.specValues(),
			"axis_prices": []map[string]any{{
				"values": overrideCombo, "price": 1.0,
				"cycles": map[string]float64{"monthly": 1, "quarterly": 3, "annually": 12},
			}},
		}},
	}
	status, body := h.do(http.MethodPost, "/api/v1/admin/product/products", admin, payload)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("建品失败 status=%d body=%v", status, body)
	}
	productID := uint64(digNumber(body, "data", "id"))
	t.Cleanup(func() { h.deleteProduct(productID) })

	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/users/order-config?product_id=%d", productID), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("取代下单配置失败 status=%d body=%v", status, body)
	}
	got := map[string]float64{}
	for _, raw := range body["data"].(map[string]any)["specs"].([]any) {
		spec, _ := raw.(map[string]any)
		c, _ := spec["spec_code"].(string)
		p, _ := spec["price"].(float64)
		got[c] = p
	}
	if p := got[overrideCode]; math.Abs(p-1) > 0.005 {
		t.Fatalf("单独定价的 %s 应为 1 元，实际 %v（被展开价覆盖）", overrideCode, p)
	}
	// 基线组合没被单独定价，应仍按基准价 200。
	baseCombo := map[string]string{}
	for _, axis := range tpl.axes {
		baseCombo[axis.key] = axis.baseline
	}
	baseCode := tpl.specCodeOf(baseCombo)
	if p := got[baseCode]; math.Abs(p-200) > 0.005 {
		t.Fatalf("基线规格 %s 应为 200，实际 %v", baseCode, p)
	}
}
