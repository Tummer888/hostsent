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

// HostHeader 下游拉取上游云主机信息（host/header 的 data.host_data）
type HostHeader struct {
	HostData struct {
		ID           int64  `json:"id"`
		Domain       string `json:"domain"`      // 主机名
		Username     string `json:"username"`    // 系统用户
		DedicatedIP  string `json:"dedicatedip"` // 主 IP
		AssignedIPs  string `json:"assignedips"` // 附加 IP（逗号分隔）
		Port         string `json:"port"`
		OS           string `json:"os"`
		DomainStatus string `json:"domainstatus"` // Active/Suspended/...
		NextDueDate  int64  `json:"nextduedate"`
		ProductID    int64  `json:"productid"`
		UpstreamCost string `json:"upstream_cost"`
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
	QtyMaximum int         `json:"qty_maximum"`
	Sub        []ConfigSub `json:"sub"`
}

// ConfigSub 配置项可选值（option_name 为值，如 "2"/"8192"/"CentOS"）。
type ConfigSub struct {
	ID         int64            `json:"id"`
	OptionName string           `json:"option_name"`
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

// ===== openapi（v1）对外商品接口响应结构 =====
// 下游对接上游财务的正式开放接口：
//   - GET v1/products            商品列表（含售价 product_price + 一级/二级分类）
//   - GET v1/productsconfig      商品配置（含 configoptions 开通规格），可按 first_group_id/group_id/product_id 过滤

// OpenapiProductsResp GET v1/products 响应 data。
type OpenapiProductsResp struct {
	FirstGroup []OpenapiFirstGroup `json:"first_group"`
	Currency   json.RawMessage     `json:"currency"` // 货币信息（对象/数组，暂不消费）
}

// OpenapiFirstGroup 一级商品分类（first_group）。
type OpenapiFirstGroup struct {
	ID    int64          `json:"id"`
	Name  string         `json:"name"`
	Group []OpenapiGroup `json:"group"`
}

// OpenapiGroup 二级商品分类（product_groups）。
type OpenapiGroup struct {
	ID       int64            `json:"id"`
	Name     string           `json:"name"`
	Products []OpenapiProduct `json:"products"`
}

// OpenapiProduct 可购商品条目（getList() 字段，含上游售价 product_price）。
type OpenapiProduct struct {
	ID                 int64           `json:"id"`
	Name               string          `json:"name"`
	Type               string          `json:"type"`
	Description        string          `json:"description"`
	ProductPrice       json.RawMessage `json:"product_price"` // 上游售价（当前计费周期，可能为字符串）
	SetupFee           json.RawMessage `json:"setup_fee"`     // 初装费（可能为字符串）
	BillingCycle       string          `json:"billingcycle"`  // monthly/quarterly/onetime/free...
	UpstreamPriceType  string          `json:"upstream_price_type"`
	UpstreamPriceValue float64         `json:"upstream_price_value"`
	PayType            json.RawMessage `json:"pay_type"`
	Qty                int             `json:"qty"`
	StockControl       int             `json:"stock_control"`
}

// OpenapiProductsConfigResp GET v1/productsconfig 响应 data。
type OpenapiProductsConfigResp struct {
	FirstGroup []struct {
		Group []struct {
			ID       int64                  `json:"id"`
			Name     string                 `json:"name"`
			Products []OpenapiProductConfig `json:"products"`
		} `json:"group"`
	} `json:"first_group"`
}

// OpenapiProductConfig 商品配置（含 configoptions 规格）。
type OpenapiProductConfig struct {
	ID            int64                    `json:"id"`
	Name          string                   `json:"name"`
	ConfigOptions []ConfigOption           `json:"configoptions"`
	CustomFields  []map[string]interface{} `json:"custom_fields"`
	Cycle         []map[string]interface{} `json:"cycle"`
}
