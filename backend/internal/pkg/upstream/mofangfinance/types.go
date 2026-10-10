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
// 来自上游 getProducts()：全量商品（不过滤 hidden/retired）+ product_pricings + config_groups。
type UpstreamProductDetail struct {
	ID              int64            `json:"id"`
	Type            string           `json:"type"` // dcim/dcimcloud/vhost...
	GID             int64            `json:"gid"`  // 分组 id
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	ProductPricings []ProductPricing `json:"product_pricings"` // 上游售价（默认币种）
	ConfigGroups    []ConfigGroup    `json:"config_groups"`    // 商品配置项（开通规格，含 CPU/内存/系统盘等）
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
