package model

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// StandardProductSpec 统一商品规格
type StandardProductSpec struct {
	CPU       int                    `json:"cpu"`             // CPU核数
	Memory    int                    `json:"memory"`          // 内存大小(MB)
	Disk      int                    `json:"disk"`            // 系统盘大小(GB)
	DiskType  string                 `json:"disk_type"`       // ssd/premium/hdd
	Bandwidth int                    `json:"bandwidth"`       // 带宽(Mbps)
	OS        string                 `json:"os"`              // 操作系统
	Region    string                 `json:"region"`          // 区域
	Zone      string                 `json:"zone"`            // 可用区
	Extra     map[string]interface{} `json:"extra,omitempty"` // 扩展字段
}

// StandardProduct 统一商品
type StandardProduct struct {
	ID           uint                   `json:"id"`
	ProviderID   uint                   `json:"provider_id"`
	ProviderType string                 `json:"provider_type"`
	UpstreamID   string                 `json:"upstream_id"`
	Name         string                 `json:"name"`
	Specs        StandardProductSpec    `json:"specs"` // 上游标准化规格（StandardProductSpec）；wire 层商品 DTO 的 `specs` 为该结构的 JSON 字符串
	RawSpecs     map[string]interface{} `json:"raw_specs"`
	CostPrice    float64                `json:"cost_price"`
	SalePrice    float64                `json:"sale_price"`
	Status       string                 `json:"status"` // active/inactive
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// StockValue 上游可售库存：-1=不限量（上游未开启库存控制），>=0=剩余可售数量。
//
// 口径集中在适配器里：上游各家表达库存的字段不同（zjmf 是 stock_control + qty），
// 消费方只认这一个方法，避免每个渠道各写一份判断。
func (p *StandardProduct) StockValue() int {
	if p == nil || p.RawSpecs == nil {
		return -1
	}
	v, ok := p.RawSpecs[stockControlKey]
	if !ok {
		return -1
	}
	if toInt(v) != 1 {
		return -1
	}
	qty := toInt(p.RawSpecs[stockQtyKey])
	if qty < 0 {
		return 0
	}
	return qty
}

// stockControlKey / stockQtyKey 上游原始规格里的库存字段名（zjmf 商品详情语义）。
// 各适配器在 RawSpecs 里统一用这两个键表达库存，StockValue 才能跨渠道通用。
const (
	stockControlKey = "stock_control"
	stockQtyKey     = "qty"
)

// toInt 宽松转 int：上游 JSON 里同一个数值字段可能是 float64/json.Number/string。
func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case float32:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0
		}
		return int(i)
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}
