// Package dto 定义用户中心主机模块数据结构。
package dto

// InstanceInfo 用户可见主机信息。
type InstanceInfo struct {
	ID           uint64 `json:"id"`
	InstanceID   string `json:"instance_id"`
	ProviderType string `json:"provider_type"`
	ProviderID   uint64 `json:"provider_id"`
	Name         string `json:"name"`
	Status       string `json:"status"`       // 服务状态：running/stopped/creating/...
	PowerStatus  string `json:"power_status"` // 电源状态：on/off/unknown
	OS           string `json:"os"`
	CPU          int    `json:"cpu"`
	Memory       int    `json:"memory"`
	Disk         int    `json:"disk"`
	DiskType     string `json:"disk_type"`
	Bandwidth    int    `json:"bandwidth"`
	Region       string `json:"region"`
	Zone         string `json:"zone"`
	PublicIP     string `json:"public_ip"`
	PrivateIP    string `json:"private_ip"`
	BillingMode  string `json:"billing_mode"`
	ExpireAt     string `json:"expire_at"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ListQuery 我的主机查询。
type ListQuery struct {
	Live bool `form:"live" json:"live"` // 是否刷新上游实时状态
}

// ListResponse 我的主机列表响应。
type ListResponse struct {
	Items []InstanceInfo `json:"items"`
	Total int64          `json:"total"`
}

// PowerRequest 电源操作请求。
type PowerRequest struct {
	Action string `json:"action" binding:"required,oneof=on off reboot hard_off hard_reboot"`
}

// VNCResult 远程控制台结果。
type VNCResult struct {
	URL      string `json:"url"`
	External bool   `json:"external"`
}
