package zjmf

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// newMockProvider 起一个 httptest 服务模拟魔方财务（财务型 zjmf_api），返回指向它的适配器。
func newMockProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return NewProvider(&upstream.ProviderConfig{
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
	p := NewProvider(&upstream.ProviderConfig{APIEndpoint: "http://127.0.0.1:1", UpstreamType: "zjmf_api"})
	if _, ok := interface{}(p).(upstream.InstancePowerHard); !ok {
		t.Fatal("Provider 未实现 InstancePowerHard")
	}
	if _, ok := interface{}(p).(upstream.InstanceReinstall); !ok {
		t.Fatal("Provider 未实现 InstanceReinstall")
	}
	if _, ok := interface{}(p).(upstream.InstancePasswordReset); !ok {
		t.Fatal("Provider 未实现 InstancePasswordReset")
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
		paidInv   string
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
			// 上游真实形态：下游请求（is_downstream）时 data 同时给 invoiceid 与 hostid，
			// 而 hostid 对应的主机此刻是 Pending（未支付不开通）——见 CartController::settle。
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "ok",
				"data": map[string]interface{}{"invoiceid": 7001, "hostid": []int64{9001}},
			})
		case "/apply_credit":
			paidInv = r.Form.Get("invoiceid")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 1001, "msg": "支付完成", "data": map[string]interface{}{"hostid": []int64{9001}},
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
	// 结算返回的 hostid 是"单据已建"（Pending），不等于开好：有账单就必须付清。
	if paidInv != "7001" {
		t.Errorf("支付账单 = %q, want 7001（有账单就必须支付，hostid 不替代支付）", paidInv)
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

// TestContract_CreateInstancePersistInvoiceBeforePay 回归（实测事故：一个订单在上游生成 5 张账单）：
// 结算生成账单后，账单 id 必须在**支付之前**回调给调用方落库；余额不足支付失败时，
// 该账单 id 是后续重试复用的唯一凭据 —— 落库晚了（放到支付之后）就等于没有。
func TestContract_CreateInstancePersistInvoiceBeforePay(t *testing.T) {
	var order []string
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/credit":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data":   map[string]interface{}{"credit": "5.08", "currency": map[string]interface{}{"id": 1, "code": "CNY"}},
			})
		case "/cart/clear":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
		case "/cart/add_to_shop":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
		case "/cart/settle":
			order = append(order, "settle")
			// 待支付单：只有 invoiceid，没有 hostid。
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "ok",
				"data": map[string]interface{}{"invoiceid": 7001},
			})
		case "/cart/update_invoice_customfield":
			order = append(order, "customfield")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
		case "/apply_credit":
			order = append(order, "pay")
			// 上游余额不足：status 200（非 1001），账单未清、主机不会开通。
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "余额不足：账单需18.00，现余额5.08",
			})
		default:
			http.NotFound(w, r)
		}
	})

	var persisted []int64
	_, err := p.CreateInstance(context.Background(), &pkgmodel.CreateInstanceRequest{
		ProviderType: ProviderType,
		Name:         "hs-balance-check",
		BillingMode:  "monthly",
		Extra:        map[string]interface{}{"upstream_pid": "308"},
		OnInvoiceCreated: func(invoiceID int64) {
			order = append(order, "persist")
			persisted = append(persisted, invoiceID)
		},
	})
	if err == nil {
		t.Fatal("余额不足必须报错，而不是把未开通当成功")
	}
	var perr *upstream.ProviderError
	if !errors.As(err, &perr) || perr.Code != 200 {
		t.Fatalf("err = %#v, want ProviderError code=200（余额不足，供 worker 判终止性）", err)
	}
	if len(persisted) != 1 || persisted[0] != 7001 {
		t.Fatalf("落库账单 id = %v, want [7001]", persisted)
	}
	// 顺序：结算生成账单 → 回调落库 → 支付。落库必须在支付前。
	want := []string{"settle", "persist", "pay"}
	if len(order) != len(want) {
		t.Fatalf("调用顺序 = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("调用顺序 = %v, want %v", order, want)
		}
	}
}

// TestContract_CreateInstanceReusesExistingInvoice 回归（同上事故的修复验证）：
// 请求带上已记录的上游账单 id 时，适配器必须跳过「清购物车 + 加购 + 结算」，
// 直接补录自定义字段并支付该账单 —— 重试绝不能在上游再生成一张新账单。
func TestContract_CreateInstanceReusesExistingInvoice(t *testing.T) {
	var (
		cleared, addedToShop, settled bool
		paidInvoice                   string
		paidUseCredit                 string
	)
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/clear":
			cleared = true
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
		case "/cart/add_to_shop":
			addedToShop = true
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
		case "/cart/settle":
			settled = true
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "ok", "data": map[string]interface{}{"invoiceid": 8888},
			})
		case "/cart/update_invoice_customfield":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
		case "/apply_credit":
			paidInvoice = r.Form.Get("invoiceid")
			paidUseCredit = r.Form.Get("use_credit")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 1001, "msg": "支付成功",
				"data": map[string]interface{}{"hostid": []int64{9001}},
			})
		case "/host/header":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{"host_data": map[string]interface{}{
					"id": 9001, "domain": "hs-retry", "domainstatus": "Active", "nextduedate": 1798761600,
				}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	inst, err := p.CreateInstance(context.Background(), &pkgmodel.CreateInstanceRequest{
		ProviderType:      ProviderType,
		Name:              "hs-retry",
		BillingMode:       "monthly",
		UpstreamInvoiceID: 8887, // 上次结算已生成、未支付的账单
		Extra: map[string]interface{}{
			"upstream_pid": "308",
			"customfield":  map[string]interface{}{"12": "custom-value"},
		},
	})
	if err != nil {
		t.Fatalf("复用账单开通失败: %v", err)
	}
	if inst == nil || inst.UpstreamID != "9001" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	if cleared || addedToShop || settled {
		t.Fatalf("重试路径不得重新下单：clear=%v add_to_shop=%v settle=%v", cleared, addedToShop, settled)
	}
	if paidInvoice != "8887" {
		t.Errorf("支付的账单 id = %q, want 8887（复用已记录账单）", paidInvoice)
	}
	if paidUseCredit != "1" {
		t.Errorf("use_credit = %q, want 1", paidUseCredit)
	}
}

// TestContract_DownstreamHandshake 回归（实测事故：一个订单在上游生成 5 张账单）：
// 带上 downstream 三件套时，下单前必须先做 cart/clear 幂等握手，并按上游的回答分流：
//   - 已有待支付账单 → 支付同一张，绝不重新结算；
//   - 已有 Active 主机 → 幂等返回该主机，绝不重新开通；
//   - 什么都没有   → 才走"加购 + 结算 + 支付"新单流程。
func TestContract_DownstreamHandshake(t *testing.T) {
	req := func() *pkgmodel.CreateInstanceRequest {
		return &pkgmodel.CreateInstanceRequest{
			ProviderType:    ProviderType,
			Name:            "hs-hs",
			BillingMode:     "monthly",
			DownstreamToken: "0123456789abcdef0123456789abcdef",
			DownstreamURL:   "https://hostsent.local",
			DownstreamID:    832,
			Extra:           map[string]interface{}{"upstream_pid": "308"},
		}
	}
	baseRoutes := func(w http.ResponseWriter, r *http.Request, calls *[]string) bool {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/credit":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "data": map[string]interface{}{"credit": "999.00", "currency": map[string]interface{}{"id": 1, "code": "CNY"}},
			})
		case "/host/header":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{"host_data": map[string]interface{}{
					"id": 9001, "domain": "hs-hs", "domainstatus": "Active", "nextduedate": 1798761600,
				}},
			})
		default:
			return false
		}
		_ = calls
		return true
	}

	t.Run("待支付账单：支付同一张不重新结算", func(t *testing.T) {
		var calls []string
		var clearForm map[string]string
		var paidInvoice string
		p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
			if baseRoutes(w, r, &calls) {
				return
			}
			_ = r.ParseForm()
			switch r.URL.Path {
			case "/cart/clear":
				clearForm = flattenForm(r)
				calls = append(calls, "clear")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 200, "msg": "请求成功", "invoiceid": 808026, "hostid": 7788,
				})
			case "/cart/add_to_shop":
				calls = append(calls, "add_to_shop")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "添加成功"})
			case "/cart/settle":
				calls = append(calls, "settle")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "结算成功", "data": map[string]interface{}{"invoiceid": 999999}})
			case "/apply_credit":
				calls = append(calls, "pay")
				paidInvoice = r.Form.Get("invoiceid")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 1001, "msg": "支付完成", "data": map[string]interface{}{"hostid": []int64{9001}},
				})
			default:
				http.NotFound(w, r)
			}
		})

		inst, err := p.CreateInstance(context.Background(), req())
		if err != nil {
			t.Fatalf("握手复用账单开通失败: %v", err)
		}
		if inst == nil || inst.UpstreamID != "9001" {
			t.Fatalf("unexpected instance: %+v", inst)
		}
		if len(calls) != 2 || calls[0] != "clear" || calls[1] != "pay" {
			t.Fatalf("调用序列 = %v, want [clear pay]（不得重新结算）", calls)
		}
		if paidInvoice != "808026" {
			t.Errorf("支付的账单 = %q, want 808026（握手返回的既有账单）", paidInvoice)
		}
		if clearForm["downstream_token"] != "0123456789abcdef0123456789abcdef" || clearForm["downstream_id"] != "832" {
			t.Errorf("clear 未带下游三件套: %v", clearForm)
		}
	})

	t.Run("已有 Active 主机：幂等返回不重复开通", func(t *testing.T) {
		var calls []string
		p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
			if baseRoutes(w, r, &calls) {
				return
			}
			switch r.URL.Path {
			case "/cart/clear":
				calls = append(calls, "clear")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 400, "msg": "该订单已开通,请勿重新开通", "hostid": 9001, "domainstatus": "Active",
				})
			case "/cart/add_to_shop", "/cart/settle", "/apply_credit":
				calls = append(calls, strings.TrimPrefix(r.URL.Path, "/cart/"))
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok"})
			default:
				http.NotFound(w, r)
			}
		})

		inst, err := p.CreateInstance(context.Background(), req())
		if err != nil {
			t.Fatalf("已开通握手应幂等成功: %v", err)
		}
		if inst == nil || inst.UpstreamID != "9001" {
			t.Fatalf("unexpected instance: %+v", inst)
		}
		if len(calls) != 1 || calls[0] != "clear" {
			t.Fatalf("调用序列 = %v, want [clear]（不得重新下单）", calls)
		}
	})

	t.Run("主机已存在但非 Active：终止性错误，交人工", func(t *testing.T) {
		p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
			var calls []string
			if baseRoutes(w, r, &calls) {
				return
			}
			switch r.URL.Path {
			case "/cart/clear":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 400, "msg": "该订单已开通,请勿重新开通", "hostid": 105, "domainstatus": "Deleted",
				})
			default:
				http.NotFound(w, r)
			}
		})

		_, err := p.CreateInstance(context.Background(), req())
		if err == nil {
			t.Fatal("上游主机状态异常时必须报错（不得盲目重新下单）")
		}
		var perr *upstream.ProviderError
		if !errors.As(err, &perr) || perr.Code != 400 {
			t.Fatalf("err = %#v, want ProviderError code=400（供 worker 判终止性转人工）", err)
		}
	})

	t.Run("无既有记录：正常新单流程", func(t *testing.T) {
		var calls []string
		p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
			if baseRoutes(w, r, &calls) {
				return
			}
			_ = r.ParseForm()
			switch r.URL.Path {
			case "/cart/clear":
				calls = append(calls, "clear")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "请求成功"})
			case "/cart/add_to_shop":
				calls = append(calls, "add_to_shop")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "添加成功"})
			case "/cart/settle":
				calls = append(calls, "settle")
				if r.Form.Get("downstream_token") == "" {
					t.Error("settle 未带 downstream_token：上游不会按下游处理（重复下单的直接根因）")
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "结算成功", "data": map[string]interface{}{"invoiceid": 660001}})
			case "/apply_credit":
				calls = append(calls, "pay")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 1001, "msg": "支付完成", "data": map[string]interface{}{"hostid": []int64{9001}},
				})
			default:
				http.NotFound(w, r)
			}
		})

		var recorded []int64
		r := req()
		r.OnInvoiceCreated = func(id int64) { recorded = append(recorded, id) }
		inst, err := p.CreateInstance(context.Background(), r)
		if err != nil {
			t.Fatalf("新单开通失败: %v", err)
		}
		if inst == nil || inst.UpstreamID != "9001" {
			t.Fatalf("unexpected instance: %+v", inst)
		}
		want := []string{"clear", "add_to_shop", "settle", "pay"}
		if len(calls) != len(want) {
			t.Fatalf("调用序列 = %v, want %v", calls, want)
		}
		for i := range want {
			if calls[i] != want[i] {
				t.Fatalf("调用序列 = %v, want %v", calls, want)
			}
		}
		if len(recorded) != 1 || recorded[0] != 660001 {
			t.Fatalf("账单落库回调 = %v, want [660001]", recorded)
		}
	})
}

// TestContract_SettleHostIDEncodings 回归（实测故障：开通失败，报
// `json: cannot unmarshal string into Go struct field .hostid of type int64`）。
//
// 上游 cart/settle 与 apply_credit 对 hostid 的下发形态**不一致**且各自多变：
//
//	cart/settle   → {"data":{"hostid":["2056"],"invoiceid":808041}}  字符串数组
//	apply_credit  → {"data":{"hostid":[2049],"url":"..."}}            数字数组
//	cart/clear    → {"hostid":2056,"invoiceid":808041}                平铺数字
//
// 任一处类型不符都会让整段 data 解析失败：账单 id 丢失 → 开通链路断在
// "结算成功但未返回账单"（订单 841 实测）。本用例把三种形态钉死为回归。
func TestContract_SettleHostIDEncodings(t *testing.T) {
	// 形态一：cart/settle（字符串数组 + 数字账单）
	var settle struct {
		InvoiceID FlexID     `json:"invoiceid"`
		HostID    FlexIDList `json:"hostid"`
	}
	if err := json.Unmarshal([]byte(`{"hostid":["2056"],"invoiceid":808041}`), &settle); err != nil {
		t.Fatalf("settle 响应解析失败（整段报废，账单 id 丢失）: %v", err)
	}
	if settle.InvoiceID.Int64() != 808041 {
		t.Fatalf("invoiceid = %d, want 808041", settle.InvoiceID.Int64())
	}
	if settle.HostID.First() != 2056 {
		t.Fatalf("hostid = %d, want 2056（字符串数组）", settle.HostID.First())
	}

	// 形态二：apply_credit（数字数组）
	var pay struct {
		HostID FlexIDList `json:"hostid"`
	}
	if err := json.Unmarshal([]byte(`{"hostid":[2049],"url":"servicedetail?id=2049"}`), &pay); err != nil {
		t.Fatalf("apply_credit 响应解析失败: %v", err)
	}
	if pay.HostID.First() != 2049 {
		t.Fatalf("hostid = %d, want 2049（数字数组）", pay.HostID.First())
	}

	// 形态三：cart/clear（平铺数字；个别版本字符串）
	var clearRes clearResp
	if err := json.Unmarshal([]byte(`{"hostid":2056,"invoiceid":808041,"status":200,"msg":"请求成功"}`), &clearRes); err != nil {
		t.Fatalf("clear 响应解析失败: %v", err)
	}
	if clearRes.HostID.Int64() != 2056 || clearRes.InvoiceID.Int64() != 808041 {
		t.Fatalf("clear 解析错误: hostid=%d invoiceid=%d", clearRes.HostID.Int64(), clearRes.InvoiceID.Int64())
	}
	var clearStr clearResp
	if err := json.Unmarshal([]byte(`{"hostid":"2056","invoiceid":"808041","status":200}`), &clearStr); err != nil {
		t.Fatalf("clear 字符串形态解析失败: %v", err)
	}
	if clearStr.HostID.Int64() != 2056 || clearStr.InvoiceID.Int64() != 808041 {
		t.Fatalf("clear 字符串形态解析错误: hostid=%d invoiceid=%d", clearStr.HostID.Int64(), clearStr.InvoiceID.Int64())
	}

	// 0 元单：结算只给 hostid 不给 invoiceid，仍要能取出主机号。
	var free struct {
		InvoiceID FlexID     `json:"invoiceid"`
		HostID    FlexIDList `json:"hostid"`
	}
	if err := json.Unmarshal([]byte(`{"hostid":["9001"]}`), &free); err != nil {
		t.Fatalf("0 元单响应解析失败: %v", err)
	}
	if free.InvoiceID != 0 || free.HostID.First() != 9001 {
		t.Fatalf("0 元单解析错误: invoiceid=%d hostid=%d", free.InvoiceID.Int64(), free.HostID.First())
	}
}

// TestContract_CreateInstanceSettleStringHostID 端到端回归：上游按真实形态
// （settle 回字符串数组 hostid）返回时，整条开通链路必须走通并留下账单 id。
func TestContract_CreateInstanceSettleStringHostID(t *testing.T) {
	var settledInvoiceID int64
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/credit":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data":   map[string]interface{}{"credit": "69.08", "currency": map[string]interface{}{"id": 1, "code": "CNY"}},
			})
		case "/cart/clear":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "请求成功"})
		case "/cart/add_to_shop":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "添加成功"})
		case "/cart/settle":
			// 真实形态：hostid 是字符串数组（订单 841 实测原样）。
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "结算成功",
				"data": map[string]interface{}{"hostid": []string{"2056"}, "invoiceid": 808041},
			})
		case "/apply_credit":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 1001, "msg": "支付完成",
				"data": map[string]interface{}{"hostid": []int64{2056}, "url": "servicedetail?id=2056"},
			})
		case "/host/header":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{"host_data": map[string]interface{}{
					"id": 2056, "domain": "hs-six-zone", "domainstatus": "Active",
					"port": 0, "nextduedate": 1794529855,
				}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	inst, err := p.CreateInstance(context.Background(), &pkgmodel.CreateInstanceRequest{
		ProviderType:    ProviderType,
		Name:            "hs-six-zone",
		BillingMode:     "monthly",
		Extra:           map[string]interface{}{"upstream_pid": "528"},
		DownstreamToken: "0123456789abcdef0123456789abcdef",
		DownstreamURL:   "https://hostsent.local",
		DownstreamID:    841,
		OnInvoiceCreated: func(invoiceID int64) {
			settledInvoiceID = invoiceID
		},
	})
	if err != nil {
		t.Fatalf("开通失败（字符串数组 hostid 未兼容）: %v", err)
	}
	if inst == nil || inst.UpstreamID != "2056" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	if settledInvoiceID != 808041 {
		t.Fatalf("账单落库回调 = %d, want 808041（解析失败会让幂等修复失效）", settledInvoiceID)
	}
	if inst.Status != pkgmodel.InstanceStatusRunning {
		t.Errorf("status = %s, want running（Active 应映射为运行中）", inst.Status)
	}
}

// TestContract_RecoverAfterUpstreamPaid 回归（实测用户场景）：
//
//	本地下单 → 上游余额不足 → 开通失败（账单已建、主机 Pending）
//	→ 用户/运营在**上游侧**充值并把账单付掉、机器开通
//	→ 本地点「重新开通」/同步刷新，必须能拉回"已开通"的结果，而不是一直显示创建中。
//
// 旧行为（三个断点，本用例逐个钉死）：
//  1. 重试只认 apply_credit 的 1001；账单已被上游付掉时返回 406「账单已支付」，
//     旧代码当成失败 → 本地永远停在创建中。
//  2. 账单已支付回执不带 hostid，必须用下游令牌握手反查主机。
//  3. 上游账单被清理时返回「账单未找到」（**与"已支付"同为 406**），不能死等，
//     要继续握手分流（无主机则重新下单）。
func TestContract_RecoverAfterUpstreamPaid(t *testing.T) {
	newReq := func() *pkgmodel.CreateInstanceRequest {
		return &pkgmodel.CreateInstanceRequest{
			ProviderType:      ProviderType,
			Name:              "hs-recover",
			BillingMode:       "monthly",
			UpstreamInvoiceID: 808065,
			DownstreamToken:   "0123456789abcdef0123456789abcdef",
			DownstreamURL:     "https://hostsent.local",
			DownstreamID:      849,
			Extra:             map[string]interface{}{"upstream_pid": "529"},
		}
	}

	t.Run("账单已在上游付掉：握手反查主机并按 Active 返回", func(t *testing.T) {
		var paid, cleared bool
		p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			switch r.URL.Path {
			case "/zjmf_api_login":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
			case "/apply_credit":
				paid = true
				// 上游原文：status 是**字符串**，msg 是中文提示。
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "406", "msg": "账单已支付", "data": map[string]interface{}{"PayStatus": "Paid"},
				})
			case "/cart/clear":
				cleared = true
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 400, "msg": "该订单已开通,请勿重新开通", "hostid": 2068, "domainstatus": "Active",
				})
			case "/host/header":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 200,
					"data": map[string]interface{}{"host_data": map[string]interface{}{
						"id": 2068, "domain": "hs-2-4-10mbps-gxxreu", "domainstatus": "Active",
						"dedicatedip": "38.76.195.100", "nextduedate": 1794545644,
					}},
				})
			default:
				http.NotFound(w, r)
			}
		})

		inst, err := p.CreateInstance(context.Background(), newReq())
		if err != nil {
			t.Fatalf("账单已被上游支付时必须视为成功（回源接管主机）: %v", err)
		}
		if !paid || !cleared {
			t.Fatalf("应先支付账单再握手反查（paid=%v cleared=%v）", paid, cleared)
		}
		if inst == nil || inst.UpstreamID != "2068" {
			t.Fatalf("unexpected instance: %+v", inst)
		}
		if inst.Status != pkgmodel.InstanceStatusRunning {
			t.Errorf("status = %s, want running（上游 Active）", inst.Status)
		}
	})

	t.Run("上游仍创建中（Pending）：接管并返回 creating，本地继续跟进", func(t *testing.T) {
		p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			switch r.URL.Path {
			case "/zjmf_api_login":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
			case "/apply_credit":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "406", "msg": "账单已支付", "data": map[string]interface{}{"PayStatus": "Paid"},
				})
			case "/cart/clear":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 400, "msg": "该订单已开通,请勿重新开通", "hostid": 2066, "domainstatus": "Pending",
				})
			case "/host/header":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 200,
					"data": map[string]interface{}{"host_data": map[string]interface{}{
						"id": 2066, "domain": "hs-2-4-10mbps-j4t9o5", "domainstatus": "Pending", "dcimid": 0,
					}},
				})
			default:
				http.NotFound(w, r)
			}
		})

		inst, err := p.CreateInstance(context.Background(), newReq())
		if err != nil {
			t.Fatalf("上游主机已存在（仅未激活）时不得报错，应交回调用方按开通中跟进: %v", err)
		}
		if inst == nil || inst.UpstreamID != "2066" {
			t.Fatalf("unexpected instance: %+v", inst)
		}
		if inst.Status != pkgmodel.InstanceStatusCreating {
			t.Errorf("status = %s, want creating（上游 Pending）", inst.Status)
		}
	})

	t.Run("上游账单已被清理：继续握手，无主机则重新下单", func(t *testing.T) {
		var settled bool
		p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			switch r.URL.Path {
			case "/zjmf_api_login":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
			case "/cart/credit":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 200, "data": map[string]interface{}{"credit": "999.00", "currency": map[string]interface{}{"id": 1, "code": "CNY"}},
				})
			case "/apply_credit":
				// 旧账单（808065）已被上游清理；新账单（808099）正常支付成功。
				if r.Form.Get("invoiceid") == "808065" {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "406", "msg": "账单未找到"})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 1001, "msg": "支付完成", "data": map[string]interface{}{"hostid": []string{"2070"}},
				})
			case "/cart/clear":
				// 无任何既有记录。
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "请求成功"})
			case "/cart/add_to_shop":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "添加成功"})
			case "/cart/settle":
				settled = true
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 200, "msg": "结算成功", "data": map[string]interface{}{"hostid": []string{"2070"}, "invoiceid": 808099},
				})
			case "/host/header":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 200,
					"data": map[string]interface{}{"host_data": map[string]interface{}{
						"id": 2070, "domain": "hs-recover-zzz", "domainstatus": "Active", "nextduedate": 1794545644,
					}},
				})
			default:
				http.NotFound(w, r)
			}
		})

		var invoices []int64
		r := newReq()
		r.OnInvoiceCreated = func(id int64) { invoices = append(invoices, id) }
		inst, err := p.CreateInstance(context.Background(), r)
		if err != nil {
			t.Fatalf("账单已被清理时应重新下单而不是死等: %v", err)
		}
		if !settled {
			t.Fatal("账单不存在时必须走新的下单/结算流程")
		}
		if inst == nil || inst.UpstreamID != "2070" {
			t.Fatalf("unexpected instance: %+v", inst)
		}
		// 回调序列：先告知"旧账单作废(0)"，再落新账单 id。
		if len(invoices) != 2 || invoices[0] != 0 || invoices[1] != 808099 {
			t.Fatalf("账单回调 = %v, want [0 808099]（旧账单作废 + 新账单落库）", invoices)
		}
	})

	t.Run("余额仍不足：保持可重试的终止性错误", func(t *testing.T) {
		p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			switch r.URL.Path {
			case "/zjmf_api_login":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
			case "/apply_credit":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": 400, "msg": "余额不足：账单需15.00，现余额6.08",
				})
			default:
				http.NotFound(w, r)
			}
		})

		_, err := p.CreateInstance(context.Background(), newReq())
		if err == nil {
			t.Fatal("余额仍然不足时必须报错")
		}
		var perr *upstream.ProviderError
		if !errors.As(err, &perr) || perr.Code != 400 {
			t.Fatalf("err = %#v, want ProviderError code=400", err)
		}
	})
}
