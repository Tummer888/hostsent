// Package mofangfinance 定义魔方财务上游适配器的 API 类型。
//
// 协议参考魔方财务源码 `app/zjmf.php`（zjmfCurl / zjmfApiLogin）与
// `app/home/controller/LoginController.php`（/zjmf_api_login）、
// `app/home/controller/CartController.php`（cart/all、cart/get_product_config）、
// `app/home/controller/DcimController.php`（/dcim/on|off|reboot）、
// `app/home/controller/HostController.php`（host/header）。
package mofangfinance

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// ZjmfResp 魔方财务统一响应外壳：{status, msg, data}；个别接口返回 {code, data}；
// 登录接口额外返回 jwt 字段。
type ZjmfResp[T any] struct {
	Status int    `json:"status"`
	Code   int    `json:"code"` // 与 status 二选一，兼容个别接口
	Msg    string `json:"msg"`
	JWT    string `json:"jwt"`
	Data   T      `json:"data"`
}

// StatusOK 魔方财务业务成功码
const StatusOK = 200

// StatusTokenExpired JWT 失效码：收到后需强制重新登录重试一次（zjmfCurl 逻辑）。
const StatusTokenExpired = 405

// StatusPaidSuccess 上游余额支付成功码（apply_credit 返回 1001=支付完成并开通）。
const StatusPaidSuccess = 1001

// HostHeader 下游拉取上游云主机信息（host/header 的 data.host_data）
type HostHeader struct {
	HostData struct {
		ID           int64      `json:"id"`
		Domain       string     `json:"domain"`      // 主机名
		Username     string     `json:"username"`    // 系统用户
		DedicatedIP  string     `json:"dedicatedip"` // 主 IP
		AssignedIPs  StringList `json:"assignedips"` // 附加 IP（可能为逗号字符串或数组）
		Port         string     `json:"port"`
		OS           string     `json:"os"`
		DomainStatus string     `json:"domainstatus"` // Active/Pending/Suspended/...
		NextDueDate  int64      `json:"nextduedate"`
		ProductID    int64      `json:"productid"`
		ProductName  string     `json:"productname"`
		UpstreamCost string     `json:"upstream_cost"`
		DcimID       int64      `json:"dcimid"`
		InvoiceID    int64      `json:"invoice_id"`
	} `json:"host_data"`
}

// FlexFloat 兼容字符串或数字的浮点解析（上游 pricing 字段常为字符串如 "0.00"）。
type FlexFloat float64

// UnmarshalJSON 接受 json 字符串或数字。
func (f *FlexFloat) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = 0
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, _ := strconv.ParseFloat(s, 64)
		*f = FlexFloat(v)
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = FlexFloat(n)
		return nil
	}
	*f = 0
	return nil
}

// StringList 兼容字符串或数组的字段（上游 assignedips 可能是 "ip1,ip2" 或 ["ip1","ip2"]）。
type StringList string

// UnmarshalJSON 接受 json 字符串或数组，统一归一为逗号分隔字符串。
func (s *StringList) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*s = ""
		return nil
	}
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		*s = StringList(strings.Join(arr, ","))
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		*s = StringList(str)
		return nil
	}
	*s = StringList(strings.Trim(string(b), `"`))
	return nil
}

// ProductPricing 上游商品价格（pricing 表，月付取 monthly）。字段可能为字符串或数字。
type ProductPricing struct {
	Monthly   FlexFloat `json:"monthly"`   // 月付价格（通常为上游售价）
	Annually  FlexFloat `json:"annually"`  // 年付价格
	Quarterly FlexFloat `json:"quarterly"` // 季付价格
	Onetime   FlexFloat `json:"onetime"`   // 一次性价格
	MSetupFee FlexFloat `json:"msetupfee"` // 月付初装费
}

// PricingRow 上游 pricing 行的无损载体：字段集随上游版本演进
// （hour/day/ontrial/monthly/quarterly/semiannually/annually/biennially/triennially/
// fourly…tenly 及各自 setupfee，见上游 config('price_type')），且同一字段可能以
// 字符串或数字下发。按原始键值对承载，折扣换算逐字段处理，不丢未建模字段。
type PricingRow map[string]interface{}

// priceFieldPairs 上游周期价格字段对（价格字段, 初装费字段），口径来自上游 config('price_type')。
var priceFieldPairs = [][2]string{
	{"onetime", "osetupfee"},
	{"hour", "hsetupfee"},
	{"day", "dsetupfee"},
	{"ontrial", "ontrialfee"},
	{"monthly", "msetupfee"},
	{"quarterly", "qsetupfee"},
	{"semiannually", "ssetupfee"},
	{"annually", "asetupfee"},
	{"biennially", "bsetupfee"},
	{"triennially", "tsetupfee"},
	{"fourly", "foursetupfee"},
	{"fively", "fivesetupfee"},
	{"sixly", "sixsetupfee"},
	{"sevenly", "sevensetupfee"},
	{"eightly", "eightsetupfee"},
	{"ninely", "ninesetupfee"},
	{"tenly", "tensetupfee"},
}

// priceFieldNames 全部可换算的价格/初装费字段名。
func priceFieldNames() []string {
	names := make([]string, 0, len(priceFieldPairs)*2)
	for _, p := range priceFieldPairs {
		names = append(names, p[0], p[1])
	}
	return names
}

// Num 读取某价格字段（兼容字符串/数字）。
func (r PricingRow) Num(key string) (float64, bool) {
	v, ok := r[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
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

// HeadlinePrice 行的代表价：monthly > quarterly > semiannually > annually > onetime（取首个 > 0）。
// 上游用 -1 表示"该周期未配置"，0 表示免费，二者都不作为代表价。
func (r PricingRow) HeadlinePrice() float64 {
	for _, key := range []string{"monthly", "quarterly", "semiannually", "annually", "onetime"} {
		if v, ok := r.Num(key); ok && v > 0 {
			return v
		}
	}
	return 0
}

// Scale 返回按倍率换算后的价格行副本：只换算已知的价格/初装费字段，
// 负值（上游约定 -1 = 该周期未配置）与不可解析值原样保留。
func (r PricingRow) Scale(factor float64) PricingRow {
	out := make(PricingRow, len(r))
	for k, v := range r {
		out[k] = v
	}
	for _, key := range priceFieldNames() {
		cur, ok := r.Num(key)
		if !ok || cur < 0 {
			continue
		}
		out[key] = roundPrice(cur * factor)
	}
	return out
}

// roundPrice 价格保留两位小数（与上游 bcscale(2) 同口径）。
func roundPrice(v float64) float64 {
	return math.Round(v*100) / 100
}

// SaleFlag 上游对我方账号的销售折扣（cart/get_product_config 的 data.flag）。
// 上游无折扣时该字段下发 false，故需容忍布尔形态。
//
// 语义来自上游源码（common.php::getSaleProductUser + zjmf.php::upstreamBates）：
//   - type=1：bates 为「应付百分比」，折后价 = 标价 × bates/100（如 bates=70 → 打七折）；
//   - 其它：bates 为「固定减免额」，折后价 = 标价 − bates（不为负）。
type SaleFlag struct {
	Type  int       `json:"type"`
	Bates FlexFloat `json:"bates"`
}

// UnmarshalJSON 容忍 false/true/{}/null 与对象四种形态：无折扣时上游返回 false。
func (f *SaleFlag) UnmarshalJSON(b []byte) error {
	*f = SaleFlag{}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" || trimmed == "null" || trimmed == "false" || trimmed == "true" ||
		trimmed == "{}" || trimmed == "[]" {
		return nil
	}
	var raw struct {
		Type  int       `json:"type"`
		Bates FlexFloat `json:"bates"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	f.Type, f.Bates = raw.Type, raw.Bates
	return nil
}

// Enabled 是否有可用折扣（bates<=0 视为无折扣：上游对无折扣商品下发 false 或 bates=0）。
func (f SaleFlag) Enabled() bool { return float64(f.Bates) > 0 }

// Factor 折扣倍率：type=1 时 bates/100（应付比例），其它情况返回 0（需走 Apply 的固定减免口径）。
func (f SaleFlag) Factor() float64 {
	if !f.Enabled() {
		return 1
	}
	if f.Type == 1 {
		return float64(f.Bates) / 100
	}
	return 1
}

// Apply 按本折扣换算标价：type=1 乘百分比，其它按固定额减免（减到 0 为止）。
func (f SaleFlag) Apply(price float64) float64 {
	if !f.Enabled() || price < 0 {
		return price
	}
	if f.Type == 1 {
		return roundPrice(price * float64(f.Bates) / 100)
	}
	if price < float64(f.Bates) {
		return 0
	}
	return roundPrice(price - float64(f.Bates))
}

// DiscountDesc 折扣的中文描述（写入资源商品 raw_specs，供详情页展示）。
func (f SaleFlag) DiscountDesc() string {
	if !f.Enabled() {
		return "无代理折扣（按上游标价结算）"
	}
	if f.Type == 1 {
		return "代理折扣 " + trimFloat(float64(f.Bates)) + "%（应付标价的 " + trimFloat(float64(f.Bates)) + "%）"
	}
	return "代理折扣 直减 " + trimFloat(float64(f.Bates))
}

// trimFloat 去掉浮点尾零（70.00 → 70）。
func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

// CartGroup 上游商品分组（cart/all 的 data.products 元素）——即商品分类。
type CartGroup struct {
	ID       int64        `json:"id"`
	Name     string       `json:"name"`
	Products []CartMember `json:"products"`
}

// CartMember 分组下的商品（只含列表级字段，价格需另行取详情）。
type CartMember struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// CartAllResp GET cart/all 的 data（分组列表 + 商品总数 + 币种）。
//
// 注意：这里描述的是**响应壳 data 字段内部**的结构 —— p.call 已经把
// {status,msg,data} 外壳解掉了，out 直接收到 data 对象本身，切勿再包一层 Data
// （包了不会报错，只会静默解出零值，分类名与折扣价就这么丢过一轮）。
type CartAllResp struct {
	Count    int         `json:"count"`
	Currency string      `json:"currency"`
	Products []CartGroup `json:"products"`
}

// CartProductConfigResp GET cart/get_product_config 的 data（同样是壳内结构）。
// 与 prodetail 的差别：多出 flag（本客户折扣）与 customfields/advanced（下单与展示素材）。
type CartProductConfigResp struct {
	Flag            SaleFlag                 `json:"flag"`
	Products        json.RawMessage          `json:"products"` // 完整商品行（pay_type/host/api_type/upstream_* 等）
	ProductPricings []PricingRow             `json:"product_pricings"`
	ConfigGroups    []ConfigGroup            `json:"config_groups"`
	CustomFields    []CustomField            `json:"customfields"`
	Advanced        []map[string]interface{} `json:"advanced"`
	ConfigLinks     []int64                  `json:"config_links"`
}

// CustomField 上游商品自定义字段（下单时以 customfield[fieldname]=value 透传）。
type CustomField struct {
	ID          int64  `json:"id"`
	FieldName   string `json:"fieldname"`
	Description string `json:"description"`
	FieldType   string `json:"fieldtype"`
	FieldOption string `json:"fieldoptions"`
	RegExpr     string `json:"regexpr"`
	Required    int    `json:"required"`
	ShowOrder   int    `json:"showorder"`
	ShowDetail  int    `json:"showdetail"`
	SortOrder   int    `json:"sortorder"`
}

// ConfigOption 商品配置项（option_name：cpu/memory/os/area...）。
type ConfigOption struct {
	ID         int64       `json:"id"`
	OptionName string      `json:"option_name"`
	OptionType int         `json:"option_type"` // 1 下拉 2 单选 3 开关 4 数量
	QtyMinimum int         `json:"qty_minimum"`
	QtyMaximum int         `json:"qty_maximum"`
	UpstreamID int64       `json:"upstream_id"` // 上游 configoption 键（cart 下单用）
	Hidden     int         `json:"hidden"`
	Sub        []ConfigSub `json:"sub"`
}

// ConfigSub 配置项可选值（option_name 为值，如 "2"/"8192"/"CentOS"）。
type ConfigSub struct {
	ID         int64            `json:"id"`
	OptionName string           `json:"option_name"`
	QtyMinimum int              `json:"qty_minimum"`
	QtyMaximum int              `json:"qty_maximum"`
	UpstreamID int64            `json:"upstream_id"` // 上游 configoption 值（cart 下单用）
	Hidden     int              `json:"hidden"`
	Pricings   []ProductPricing `json:"pricings"`
}

// ConfigGroup 商品配置项分组（prodetail 的 config_groups 元素）。
type ConfigGroup struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Options     []ConfigOption `json:"options"`
}

// UpstreamProductDetail 上游商品完整详情（api/product/prodetail 的 data.detail[pid]）。
// 来自上游 getProducts()：全量商品（不过滤 hidden/retired）+ product_pricings + config_groups，
// 另含 customfields（下单透传字段）、advanced/config_links（配置项联动）与商品原始行字段。
type UpstreamProductDetail struct {
	ID              int64                    `json:"id"`
	Type            string                   `json:"type"` // dcim/dcimcloud/vhost...
	GID             int64                    `json:"gid"`  // 分组 id（cart/all 的组）
	Name            string                   `json:"name"`
	Description     string                   `json:"description"`      // 上游富文本描述（商品详情页展示）
	ProductPricings []PricingRow             `json:"product_pricings"` // 上游标价（默认币种）；我方采购价需再套代理折扣
	ConfigGroups    []ConfigGroup            `json:"config_groups"`    // 商品配置项（开通规格，含 CPU/内存/系统盘等）
	CustomFields    []CustomField            `json:"customfields"`
	Advanced        []map[string]interface{} `json:"advanced"`
	ConfigLinks     []int64                  `json:"config_links"`
	// PayType 上游付费类型 JSON 字符串（pay_type=recurring/onetime/free + 试用配置），
	// 决定可售周期与是否可续费（onetime 不可续费）。
	PayType string `json:"pay_type"`
	// 上游链路标识：api_type=zjmf_api 表示该商品本身也是上游转售的，价格已含其加价
	// （upstream_price_type=percent 时口径为「上游原价 × upstream_price_value/100」）。
	APIType            string    `json:"api_type"`
	UpstreamPID        int64     `json:"upstream_pid"`
	UpstreamPriceType  string    `json:"upstream_price_type"`
	UpstreamPriceValue FlexFloat `json:"upstream_price_value"`
	Hidden             int       `json:"hidden"`
	Retired            int       `json:"retired"`
	StockControl       int       `json:"stock_control"`
	Qty                int       `json:"qty"`
}

// OpenapiProinfoResp GET api/product/proinfo 响应 data（全量商品 id/name/location_version）。
type OpenapiProinfoResp struct {
	Info []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"info"`
	Currency string `json:"currency"`
}

// OpenapiProdetailResp GET api/product/prodetail 响应 data（detail 为 pid → 完整详情）。
type OpenapiProdetailResp struct {
	Detail map[string]UpstreamProductDetail `json:"detail"`
}

// 说明：v1/products 与 v1/productsconfig 这一整套旧版 "openapi v1" 对外商品接口
// 响应结构（OpenapiProductsResp/OpenapiFirstGroup/OpenapiGroup/OpenapiProduct/
// OpenapiProductsConfigResp/OpenapiProductConfig）已被 api/product/proinfo 与
// api/product/prodetail 取代且无任何消费方引用，故删除。见 00 规划 Phase 0 T0.1。

// CreditRecord 余额支付流水（GET /credit_record → home/user_invoice/creditRecord）。
//
// 这是上游对「我方账户消费」的权威记录：每笔余额支付的开通/续费一行，
// relid 是上游账单号，create_time 是秒级时间戳。@refund 为关联的退款行
// （描述形如 "Credit Removed from Invoice #x"，金额为负）。
type CreditRecord struct {
	ID          int64      `json:"id"`
	RelID       int64      `json:"relid"`       // 上游账单号
	CreateTime  int64      `json:"create_time"` // 秒级时间戳
	Description string     `json:"description"` // 上游归一后的中文（余额支付）
	Type        string     `json:"type"`        // 订购产品 / 续费
	Amount      FlexFloat  `json:"amount"`      // 正数
	Refund      RefundList `json:"refund"`
}

// RefundLine 与某笔消费关联的退款行（金额为负）。
type RefundLine struct {
	ID     int64     `json:"id"`
	Amount FlexFloat `json:"amount"`
}

// RefundList 退款行集合：上游在「无退款」时返回空对象 `{}`、有退款时返回数组，
// 两种形态都要能吃下（实测 haika 返回的是 {}，写成 []RefundLine 会整页同步失败）。
type RefundList []RefundLine

// UnmarshalJSON 兼容 []、{}、null 与单个对象四种形态。
func (r *RefundList) UnmarshalJSON(b []byte) error {
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" || trimmed == "[]" {
		*r = nil
		return nil
	}
	var arr []RefundLine
	if err := json.Unmarshal(b, &arr); err == nil {
		*r = arr
		return nil
	}
	var one RefundLine
	if err := json.Unmarshal(b, &one); err == nil {
		*r = RefundList{one}
		return nil
	}
	// 未知形态不阻塞整页同步：退款只是消费的补充信息，缺了不影响成本口径。
	*r = nil
	return nil
}

// TopupRecord 充值/入账流水（GET /v1/transactions/funds → openapi/Invoices/accountsRecord）。
type TopupRecord struct {
	ID          int64     `json:"id"`
	InvoiceID   int64     `json:"invoice_id"`
	PayTime     FlexFloat `json:"pay_time"` // 秒级时间戳（上游偶尔以字符串返回）
	PaymentZh   string    `json:"payment_zh"`
	Description string    `json:"description"` // 用户充值 / 人工入账
	Type        string    `json:"type"`        // recharge 等
	AmountIn    FlexFloat `json:"amount_in"`
	TransID     string    `json:"trans_id"`
}

// UpstreamHost 上游主机（GET /v1/hosts → openapi/Host/getHosts）。
// 只取余额水位告警需要的字段：续费金额与到期日。
type UpstreamHost struct {
	ID           int64     `json:"id"`
	Domain       string    `json:"domain"`
	ProductName  string    `json:"product_name"`
	Amount       FlexFloat `json:"amount"`       // 下一期续费金额
	NextDueDate  FlexFloat `json:"nextduedate"`  // 到期时间（秒级时间戳）
	DomainStatus string    `json:"domainstatus"` // Active / Suspended
}
