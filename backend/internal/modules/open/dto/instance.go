package dto

// InstanceItem 标准实例视图（对外契约；内部异构信息不外泄）。
type InstanceItem struct {
	ID          uint64 `json:"id"`
	InstanceID  string `json:"instance_id"` // 平台实例标识
	Name        string `json:"name"`
	Status      string `json:"status"`
	SourceMode  string `json:"source_mode,omitempty"` // self=自营链路 / upstream=上游链路
	CPU         int    `json:"cpu"`
	Memory      int    `json:"memory"` // MB
	Disk        int    `json:"disk"`   // GB
	DiskType    string `json:"disk_type,omitempty"`
	Bandwidth   int    `json:"bandwidth,omitempty"`
	OS          string `json:"os,omitempty"`
	Region      string `json:"region,omitempty"`
	Zone        string `json:"zone,omitempty"`
	PublicIP    string `json:"public_ip,omitempty"`
	PrivateIP   string `json:"private_ip,omitempty"`
	BillingMode string `json:"billing_mode,omitempty"`
	ExpireAt    string `json:"expire_at,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	OrderID     uint64 `json:"order_id,omitempty"`
}

// InstanceListResponse 实例分页。
type InstanceListResponse struct {
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Items    []InstanceItem `json:"items"`
}

// RenewInstanceRequest 实例续费（POST /instances/{id}/renew）。
type RenewInstanceRequest struct {
	// PeriodCount 续费周期数，默认 1。
	PeriodCount int `json:"period_count"`
}

// PowerInstanceRequest 实例电源操作（POST /instances/{id}/power）。
type PowerInstanceRequest struct {
	// Action ∈ on / off / hard_off / reboot。
	Action string `json:"action" binding:"required"`
}

// SuspendInstanceRequest 实例暂停（POST /instances/{id}/suspend）。
type SuspendInstanceRequest struct {
	Reason string `json:"reason"`
}
