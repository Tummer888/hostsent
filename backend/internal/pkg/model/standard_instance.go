package model

import "time"

// StandardInstanceStatus 统一实例状态
type StandardInstanceStatus string

const (
	InstanceStatusCreating   StandardInstanceStatus = "creating"
	InstanceStatusRunning    StandardInstanceStatus = "running"
	InstanceStatusStopped    StandardInstanceStatus = "stopped"
	InstanceStatusRestarting StandardInstanceStatus = "restarting"
	InstanceStatusError      StandardInstanceStatus = "error"
	InstanceStatusDeleting   StandardInstanceStatus = "deleting"
	InstanceStatusDeleted    StandardInstanceStatus = "deleted"
)

// StandardInstance 统一实例
type StandardInstance struct {
	ID           uint                   `json:"id"`
	ProviderID   uint                   `json:"provider_id"`
	ProviderType string                 `json:"provider_type"`
	UpstreamID   string                 `json:"upstream_id"`
	Name         string                 `json:"name"`
	UserID       uint                   `json:"user_id"`
	ProductID    uint                   `json:"product_id"`
	Specs        StandardProductSpec    `json:"specs"`
	Status       StandardInstanceStatus `json:"status"`
	PrivateIP    string                 `json:"private_ip"`
	PublicIP     string                 `json:"public_ip"`
	Region       string                 `json:"region"`
	Zone         string                 `json:"zone"`
	CreatedAt    time.Time              `json:"created_at"`
	ExpireAt     time.Time              `json:"expire_at"`
	RawData      map[string]interface{} `json:"raw_data"`
}
