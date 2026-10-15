package model

import "strings"

// 产品类型（ProductKind）：商品与实例共用的产品线判据。
//
// 为什么需要一层独立词汇：运营视角按「卖的是什么」分产品线（云主机 / CDN / 物理机…），
// 这既不是链路判据（source_mode：自营 vs 转售），也不是上游适配器类型
// （provider_type：dcimcloud / zjmf），三者互相正交 ——
// 同一产品线可以自营也可以转售，同一适配器将来也可能开出多种产品线。
// 混用会让「实例列表按产品线筛选」这类需求无处落地。
//
// 取值与 products.product_type 同命名空间（迁移 108 起 instances.instance_type 也存这一套），
// 因此商品建好后开通出的实例天然落在同一产品线下，无需人工映射。
//
// 预埋 + 已上线（README）：
//   - cloud_host：云主机 / 云服务器（当前唯一在售，历史商品全部落这一档）
//   - cdn：CDN 内容分发
//   - ip：独立 IP / 弹性 IP
//   - storage：对象存储 / 云盘
//   - bandwidth：独享带宽 / 流量包
//   - physical：物理机 / 裸金属
//   - other：其它（兜底，避免未知类型在筛选器里凭空多出一项）
//
// 「预埋」的含义：类型本身已在后端登记（可被商品选中、可被实例继承、可在筛选器出现），
// 但**是否显示在导航**由实际有无实例决定 —— 没有该类型实例时导航不出现空标签页。
const (
	ProductKindCloudHost = "cloud_host"
	ProductKindCDN       = "cdn"
	ProductKindIP        = "ip"
	ProductKindStorage   = "storage"
	ProductKindBandwidth = "bandwidth"
	ProductKindPhysical  = "physical"
	ProductKindOther     = "other"
)

// ProductKind 一条产品类型定义（供列表筛选器与商品表单共用）。
type ProductKind struct {
	// Value 落库值（products.product_type / instances.instance_type）。
	Value string `json:"value"`
	// Label 中文展示名。
	Label string `json:"label"`
	// SortOrder 展示顺序（小的在前）。
	SortOrder int `json:"sort_order"`
}

// productKindRegistry 产品类型登记表（唯一权威定义）。
//
// 新增产品线时**只改这里**：后端筛选器、商品表单下拉、实例导航标签都从本表派生，
// 不会出现「加了一处忘了一处」的漂移。
var productKindRegistry = []ProductKind{
	{Value: ProductKindCloudHost, Label: "云主机", SortOrder: 1},
	{Value: ProductKindCDN, Label: "CDN", SortOrder: 2},
	{Value: ProductKindPhysical, Label: "物理机", SortOrder: 3},
	{Value: ProductKindIP, Label: "IP 地址", SortOrder: 4},
	{Value: ProductKindStorage, Label: "存储", SortOrder: 5},
	{Value: ProductKindBandwidth, Label: "带宽", SortOrder: 6},
	{Value: ProductKindOther, Label: "其它", SortOrder: 99},
}

// ProductKinds 返回登记的全部产品类型（副本，调用方可安全排序/裁剪）。
func ProductKinds() []ProductKind {
	out := make([]ProductKind, len(productKindRegistry))
	copy(out, productKindRegistry)
	return out
}

// ProductKindLabel 取产品类型展示名；未登记的类型返回原值（不丢信息）。
func ProductKindLabel(value string) string {
	v := strings.TrimSpace(value)
	for _, kind := range productKindRegistry {
		if kind.Value == v {
			return kind.Label
		}
	}
	return v
}

// IsKnownProductKind 判断是否为登记在册的产品类型。
//
// 用途：列表筛选与表单校验只接受登记值 —— 否则前端传任意字符串会得到一个
// 恒为空的筛选结果，看起来像"没有数据"而不是"参数不对"。
func IsKnownProductKind(value string) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}
	for _, kind := range productKindRegistry {
		if kind.Value == v {
			return true
		}
	}
	return false
}

// NormalizeProductKind 归一化产品类型：空值与未登记值一律落到 other。
//
// 为什么兜到 other 而不是置空：置空会让实例在「按类型筛选」里彻底消失
// （既不在任何标签页，也可能被"全部"以外的查询漏掉）。other 至少保证可见。
func NormalizeProductKind(value string) string {
	v := strings.TrimSpace(value)
	if IsKnownProductKind(v) {
		return v
	}
	return ProductKindOther
}
