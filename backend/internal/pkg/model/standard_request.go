package model

// CreateInstanceRequest 统一创建实例请求
type CreateInstanceRequest struct {
	ProviderType string                 `json:"provider_type"`
	ProductID    uint                   `json:"product_id"`
	Name         string                 `json:"name"`
	Password     string                 `json:"password"`
	Region       string                 `json:"region"`
	Zone         string                 `json:"zone"`
	Count        int                    `json:"count"`
	BillingMode  string                 `json:"billing_mode"`    // hourly/monthly
	Extra        map[string]interface{} `json:"extra,omitempty"` // 上游特有参数

	// UpstreamInvoiceID 已存在的上游账单 id（开通幂等）。
	//
	// 上游（魔方财务类）开通是"下单 → 结算生成账单 → 余额支付"三步：账单一旦生成，
	// 重试就应**复用同一张账单**再支付，而不是重新下单。此前每次重试都新建账单 ——
	// 上游余额不足时连试 5 次就留下 5 张待支付账单（实测：一个订单在上游生成多张账单）。
	UpstreamInvoiceID int64 `json:"upstream_invoice_id,omitempty"`

	// OnInvoiceCreated 账单创建后的即时回调：调用方在此把账单 id 落库，供后续重试复用。
	// 必须在**支付之前**调用 —— 支付失败正是最需要复用账单的场景。
	// 非账单型上游（资源池等）可忽略。
	OnInvoiceCreated func(invoiceID int64) `json:"-"`

	// DownstreamToken/URL/ID 「下游身份三件套」（魔方财务上游协议的幂等键）。
	//
	// 上游为下游预留了按 token 反查既有开通的握手：下单前调 cart/clear，带上这三项，
	// 上游会回答「这个下游订单在上游有没有已生成的账单/主机」——
	//   返回 invoiceid → 上一轮已生成账单未支付，直接支付同一张（就是"重新开通"）；
	//   返回 400 + hostid → 上游主机已在，不可重复开通；
	//   两者皆无 → 全新订单，正常下单。
	// Token 必须**按订单稳定**（生成一次后落库，重试复用），否则握手查不到既有订单，
	// 每次重试都会在上游新建账单与主机（实测事故：一个订单生成 5 张账单）。
	DownstreamToken string `json:"downstream_token,omitempty"`
	DownstreamURL   string `json:"downstream_url,omitempty"`
	DownstreamID    uint64 `json:"downstream_id,omitempty"`
}
