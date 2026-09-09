// Package mofangfinance 定义魔方财务（JMF Finance）上游适配器的 API 类型。
//
// 魔方财务作为「下游」第三方对接系统时，入口为 `ZjmfFinanceApiController`：
//   - createApi：POST /admin/zjmf_finance_api （注册/创建第三方接口访问凭证）
//   - inputProduct：POST /admin/zjmf_finance_api/inputproduct （导入上游商品）
//
// 字段以魔方财务后台「接口」文档为准（见规范文档 app/admin/controller/ZjmfFinanceApiController.php）。
package mofangfinance

// MoFangFinanceResp 魔方财务统一响应外壳。
// 魔方财务接口通常返回 `{status, msg, data}` 形态；个别接口为 `{code, data}`。
type MoFangFinanceResp[T any] struct {
	Status int    `json:"status"`
	Code   int    `json:"code"` // 与 status 二选一，兼容不同接口
	Msg    string `json:"msg"`
	Data   T      `json:"data"`
}

// CreateAPIParams 创建魔方财务接口（createApi）请求参数。
type CreateAPIParams struct {
	Name             string `json:"name"`               // 名称
	Hostname         string `json:"hostname"`           // 地址(IP 或域名)
	Username         string `json:"username"`           // 用户名
	Password         string `json:"password"`           // 密码
	Des              string `json:"des"`                // 备注
	Type             string `json:"type"`               // 接口类型：zjmf_api|智简魔方, manual|手动, v10
	ContactWay       string `json:"contact_way"`        // 联系方式
	AutoReplySwitch  bool   `json:"auto_reply_switch"`  // 自动回复开关（非必填）
	AutoReplyAccount string `json:"auto_reply_account"` // 自动回复账号（非必填）
	SyncStock        int    `json:"sync_stock"`         // 前台订购实时更新库存和商品：1 开启，默认 0 关闭（非必填）
}

// InputProductParams 导入上游商品（inputProduct）请求参数。
type InputProductParams struct {
	GroupID          uint64            `json:"gid"`                  // 组 ID
	ProductNames     map[string]string `json:"productnames"`         // 键=上游产品ID，值=产品名称
	UpstreamPriceVal string            `json:"upstream_price_value"` // 利润百分比（非必填）
	Ptype            string            `json:"ptype"`                // 导航类型（非必填）
	ZjmfFinanceAPIID uint64            `json:"zjmf_finance_api_id"`  // 魔方财务 api id
	ModuleTypes      map[string]int    `json:"type"`                 // 键=上游产品ID，值=模块类型
	Rate             float64           `json:"rate,omitempty"`       // 汇率（非必填，上下游汇率不同时传入）
}

// MoFangFinanceApiInfo 创建/查询到的第三方接口访问凭证（createApi 结果）。
type MoFangFinanceApiInfo struct {
	ID         uint64 `json:"id"`
	Name       string `json:"name"`
	AppID      string `json:"app_id"`
	AppSecret  string `json:"app_secret"`
	AccessKey  string `json:"access_key"`
	Status     int    `json:"status"`
	LastCallAt string `json:"last_call_at"`
}

// MoFangFinanceProduct 魔方财务商品（上游可售产品）。
type MoFangFinanceProduct struct {
	ID        uint64  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"` // server/cloud/vhost 等
	GroupID   uint64  `json:"group_id"`
	Status    int     `json:"status"`
	CostPrice float64 `json:"cost_price"`
	SalePrice float64 `json:"sale_price"`
}

// MoFangFinanceInstance 魔方财务实例（下游订单关联的 VPS/云主机）。
type MoFangFinanceInstance struct {
	ID        uint64 `json:"id"`
	OrderID   string `json:"order_id"`
	HostID    uint64 `json:"host_id"`
	ProductID uint64 `json:"product_id"`
	Status    string `json:"status"`
	Region    string `json:"region"`
	Zone      string `json:"zone"`
}
