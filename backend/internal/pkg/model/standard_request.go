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
	BillingMode  string                 `json:"billing_mode"` // hourly/monthly
	Extra        map[string]interface{} `json:"extra,omitempty"` // 上游特有参数
}
