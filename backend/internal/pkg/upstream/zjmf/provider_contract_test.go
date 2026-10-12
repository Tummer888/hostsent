package zjmf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// newMockProvider 起一个 httptest 服务模拟魔方财务（财务型 zjmf_api），返回指向它的适配器。
func newMockProvider(t *testing.T, handler http.HandlerFunc) *ZJMFProvider {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return NewZJMFProvider(&upstream.ProviderConfig{
		APIEndpoint:  ts.URL,
		Secure:       false,
		UpstreamType: "zjmf_api",
		Timeout:      5,
	})
}

// TestContract_GetInstanceHostHeader 校验：登录走 POST /zjmf_api_login（返回 jwt），
// 业务调用 GET /host/header 携带 `Authorization: Bearer <jwt>`，且 assignedips/域名映射正确。
func TestContract_GetInstanceHostHeader(t *testing.T) {
	var authHeader string
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			if r.Method != http.MethodPost {
				t.Errorf("login method = %s, want POST", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/host/header":
			authHeader = r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{
					"host_data": map[string]interface{}{
						"id": 1934, "domain": "hs-abc", "dedicatedip": "1.2.3.4",
						"domainstatus": "Active", "assignedips": "1.2.3.4,5.6.7.8",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	inst, err := p.GetInstance(context.Background(), "1934")
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if authHeader != "Bearer TESTJWT" {
		t.Errorf("Authorization header = %q, want \"Bearer TESTJWT\"", authHeader)
	}
	if inst.UpstreamID != "1934" || inst.Name != "hs-abc" || inst.PublicIP != "1.2.3.4" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	if inst.Status != pkgmodel.InstanceStatusRunning {
		t.Errorf("status = %s, want running (Active)", inst.Status)
	}
	if got, ok := inst.RawData["assignedips"].(StringList); !ok || got != "1.2.3.4,5.6.7.8" {
		t.Errorf("assignedips = %#v, want \"1.2.3.4,5.6.7.8\"", inst.RawData["assignedips"])
	}
}

// TestContract_HardPowerAndMaintenance 校验电源与维护类指令都落到上游约定的 func，
// 且都以 POST /provision/default + is_api=1 下发（与源码 Host::hardOff/hardReboot/reinstall/crackPass
// 的 zjmf_api 分支一致）。
//
// 回归背景：适配器此前未实现 InstancePowerHard，服务层把"硬重启"退化成 hard_off + on 两次下发，
// 既与面板行为不一致又多一次失败面；重装/重置密码则完全缺失，界面按钮永远是灰的。
func TestContract_HardPowerAndMaintenance(t *testing.T) {
	type req struct {
		path string
		form map[string]string
	}
	var got []req
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/zjmf_api_login" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
			return
		}
		if r.URL.Path != "/provision/default" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("provision/default method = %s, want POST", r.Method)
		}
		_ = r.ParseForm()
		flat := map[string]string{}
		for k := range r.Form {
			flat[k] = r.Form.Get(k)
		}
		got = append(got, req{path: r.URL.Path, form: flat})
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
	})

	ctx := context.Background()
	checks := []struct {
		name string
		call func() error
		fn   string
	}{
		{"hard_off", func() error { return p.HardStopInstance(ctx, "1935") }, "hard_off"},
		{"hard_reboot", func() error { return p.HardRestartInstance(ctx, "1935") }, "hard_reboot"},
		{"off", func() error { return p.StopInstance(ctx, "1935", false) }, "off"},
		{"force_off", func() error { return p.StopInstance(ctx, "1935", true) }, "hard_off"},
		{"reboot", func() error { return p.RestartInstance(ctx, "1935") }, "reboot"},
		{"on", func() error { return p.StartInstance(ctx, "1935") }, "on"},
		{"reset_password", func() error { return p.ResetInstancePassword(ctx, "1935", "Abc123456!") }, "crack_pass"},
	}
	for _, c := range checks {
		before := len(got)
		if err := c.call(); err != nil {
			t.Fatalf("%s failed: %v", c.name, err)
		}
		sent := got[before:]
		if len(sent) != 1 {
			t.Fatalf("%s sent %d upstream calls, want 1", c.name, len(sent))
		}
		if sent[0].form["func"] != c.fn {
			t.Errorf("%s: func = %q, want %q", c.name, sent[0].form["func"], c.fn)
		}
		if sent[0].form["id"] != "1935" || sent[0].form["is_api"] != "1" {
			t.Errorf("%s: id/is_api = %q/%q, want 1935/1", c.name, sent[0].form["id"], sent[0].form["is_api"])
		}
	}

	// 重置密码必须带上新密码。
	if last := got[len(got)-1]; last.form["password"] != "Abc123456!" {
		t.Errorf("crack_pass password = %q, want Abc123456!", last.form["password"])
	}

	// 重装：os 为配置子项 id，另带 port / format_data_disk。
	before := len(got)
	if _, err := p.ReinstallInstance(ctx, &upstream.ReinstallRequest{
		ProviderInstanceID: "1935", OS: "4389493", Port: 2222, FormatDataDisk: true,
	}); err != nil {
		t.Fatalf("ReinstallInstance failed: %v", err)
	}
	reinstall := got[before:]
	if len(reinstall) != 1 {
		t.Fatalf("reinstall sent %d upstream calls, want 1", len(reinstall))
	}
	for k, want := range map[string]string{"func": "reinstall", "os": "4389493", "port": "2222", "format_data_disk": "1"} {
		if reinstall[0].form[k] != want {
			t.Errorf("reinstall %s = %q, want %q", k, reinstall[0].form[k], want)
		}
	}
}

// TestContract_PowerHardCapability 断言适配器实现了 InstancePowerHard，且描述符声明了相应操作
// ——服务层与前端都靠这两处判据决定"硬关机/硬重启"按钮是否可用。
func TestContract_PowerHardCapability(t *testing.T) {
	p := NewZJMFProvider(&upstream.ProviderConfig{APIEndpoint: "http://127.0.0.1:1", UpstreamType: "zjmf_api"})
	if _, ok := interface{}(p).(upstream.InstancePowerHard); !ok {
		t.Fatal("ZJMFProvider 未实现 InstancePowerHard")
	}
	if _, ok := interface{}(p).(upstream.InstanceReinstall); !ok {
		t.Fatal("ZJMFProvider 未实现 InstanceReinstall")
	}
	if _, ok := interface{}(p).(upstream.InstancePasswordReset); !ok {
		t.Fatal("ZJMFProvider 未实现 InstancePasswordReset")
	}
	d := p.Capabilities()
	for _, op := range []string{upstream.OpReinstall, upstream.OpResetPassword} {
		if !upstream.SupportsOperation(d, op) {
			t.Errorf("描述符未声明操作 %s", op)
		}
	}
}

// TestContract_CreateInstanceCurrencyAndCustomField 校验开通用上账号币种 id（而非固定 1），
// 且自定义字段随 add_to_shop / cart/settle 一并下发。
func TestContract_CreateInstanceCurrencyAndCustomField(t *testing.T) {
	type call struct {
		path string
		form map[string]string
	}
	var (
		addToShop call
		settle    call
		cleared   bool
	)
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/credit":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data":   map[string]interface{}{"credit": "199.00", "currency": map[string]interface{}{"id": 3, "code": "HKD"}},
			})
		case "/cart/clear":
			cleared = true
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
		case "/cart/add_to_shop":
			addToShop = call{path: r.URL.Path, form: flattenForm(r)}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
		case "/cart/settle":
			settle = call{path: r.URL.Path, form: flattenForm(r)}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "ok",
				"data": map[string]interface{}{"invoiceid": 7001, "hostid": []int64{9001}},
			})
		case "/host/header":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{"host_data": map[string]interface{}{
					"id": 9001, "domain": "hs-x", "domainstatus": "Active", "nextduedate": 1798761600,
				}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	inst, err := p.CreateInstance(context.Background(), &pkgmodel.CreateInstanceRequest{
		ProviderType: ProviderType,
		Name:         "hs-currency-check",
		BillingMode:  "monthly",
		Extra: map[string]interface{}{
			"upstream_pid": "308",
			"customfield":  map[string]interface{}{"12": "custom-value"},
		},
	})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	if inst == nil || inst.UpstreamID != "9001" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	if !cleared {
		t.Error("下单前未调用 cart/clear")
	}
	if addToShop.form["currencyid"] != "3" {
		t.Errorf("add_to_shop currencyid = %q, want 3 (账号币种)", addToShop.form["currencyid"])
	}
	if settle.form["cart_data[currencyid]"] != "3" {
		t.Errorf("settle currencyid = %q, want 3", settle.form["cart_data[currencyid]"])
	}
	if addToShop.form["customfield[12]"] != "custom-value" {
		t.Errorf("add_to_shop customfield[12] = %q, want custom-value", addToShop.form["customfield[12]"])
	}
	if settle.form["cart_data[customfield][12]"] != "custom-value" {
		t.Errorf("settle customfield[12] = %q, want custom-value", settle.form["cart_data[customfield][12]"])
	}
}

// flattenForm 把 ParseForm 后的值压成单值 map（测试断言用；同名多值只取第一个）。
func flattenForm(r *http.Request) map[string]string {
	out := map[string]string{}
	for k := range r.Form {
		out[k] = r.Form.Get(k)
	}
	return out
}

// TestContract_CartIndexWithoutDataEnvelope 回归：cart/index 的响应**不带 data 壳**
// （商品数组挂在顶层 products），只认 data 的解析路径会静默解出空结果 ——
// 活动款价格回填与在售判定都会因此失效，本用例守住这条协议差异。
func TestContract_CartIndexWithoutDataEnvelope(t *testing.T) {
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/index":
			if got := r.URL.Query().Get("gid"); got != "46" {
				t.Errorf("gid = %q, want 46", got)
			}
			// 关键：没有 data 字段，products 直接挂顶层。
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"msg":    "请求成功",
				"products": []map[string]interface{}{
					{"id": 589, "name": "海外年付活动——轻量型", "gid": 46, "billingcycle": "annually",
						"product_price": "259.00", "sale_price": "181.30", "bates": "77.70", "has_bates": 1},
					{"id": 592, "name": "海外年付活动——标准型", "gid": 46, "billingcycle": "annually",
						"product_price": "359.00", "sale_price": "251.30", "bates": "107.70", "has_bates": 1},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	row, ok := p.fetchCatalogRow(context.Background(), 46, "589")
	if !ok {
		t.Fatal("未取到 cart/index 行：顶层 products 解析失效（价格回填将全部落空）")
	}
	if float64(row.ProductPrice) != 259 || float64(row.SalePrice) != 181.3 || row.BillingCycle != "annually" || row.HasBates != 1 {
		t.Fatalf("目录行解析错误: %+v", row)
	}
	// 该商品的周期价全为 0/-1（价格只挂在配置项上）时，必须回填前台展示价。
	raw := []PricingRow{{"monthly": "-1.00", "annually": "0"}}
	rows, source := resolveListRows(raw, row, ok)
	if source != priceSourceCatalog {
		t.Fatalf("price_source = %q, want catalog", source)
	}
	if v, _ := rows[0].Num("annually"); v != 259 {
		t.Errorf("annually = %v, want 259", v)
	}
	// 折扣口径不变：259 × 70% = 181.30（与上游前台 sale_price 一致）。
	costs := scalePricingRows(rows, SaleFlag{Type: 1, Bates: 70})
	if v, _ := costs[0].Num("annually"); v != 181.3 {
		t.Errorf("代理价 annually = %v, want 181.3", v)
	}
	// data 壳存在的接口仍按原路径解析（callTopLevel 兼容两种形态）。
	var all CartAllResp
	if err := p.callTopLevel(context.Background(), "test", http.MethodGet, "cart/all", nil, &all); err == nil {
		t.Log("cart/all 在本 mock 中 404，符合预期（只验证解析路径不 panic）")
	}
}
