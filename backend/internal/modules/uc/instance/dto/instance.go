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
	// ActorID/ActorName 开通该实例的真实操作人（子账号下单可见，P4-09）
	ActorID   uint64 `json:"actor_user_id"`
	ActorName string `json:"actor_name"`
	ExpireAt  string `json:"expire_at"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
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

// DestroyRequest 用户侧销毁请求（doc91 §6.4）。
// ConfirmMark 必须与实例标识（instance_id）完全一致才算二次确认，
// 语义与管理端 dto.DestroyRequest 一致。
type DestroyRequest struct {
	ConfirmMark string `json:"confirm_mark" binding:"required"`
	Reason      string `json:"reason"`
}

// ReinstallRequest 用户侧重装系统请求。
type ReinstallRequest struct {
	OS string `json:"os" binding:"required"`
	// Port 重装后的自定义端口（>0 生效）。
	Port int `json:"port"`
	// FormatDataDisk 是否同时格式化数据盘（数据将丢失，默认 false）。
	FormatDataDisk bool `json:"format_data_disk"`
	// SystemDiskSize 目标系统盘大小（>0 生效）。
	SystemDiskSize int    `json:"system_disk_size"`
	Reason         string `json:"reason"`
}

// ReinstallResult 重装结果：平台新签发的初始凭据（一次性展示，不落库）。
type ReinstallResult struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ResetPasswordRequest 用户侧重置登录密码请求。
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// RescueRequest 用户侧进入救援系统请求。
type RescueRequest struct {
	System       int    `json:"system" binding:"required"`
	TempPassword string `json:"temp_password" binding:"required"`
}

// SnapshotCreateRequest 用户侧创建快照/备份请求。
type SnapshotCreateRequest struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	DiskID string `json:"disk_id"`
}

// SnapshotRestoreRequest 用户侧快照恢复请求（高危，需二次确认）。
type SnapshotRestoreRequest struct {
	SnapshotID  string `json:"snapshot_id" binding:"required"`
	ConfirmMark string `json:"confirm_mark" binding:"required"`
}

// SnapshotInfo 快照/备份条目。
type SnapshotInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       string `json:"size"`
	Status     int    `json:"status"`
	DiskID     string `json:"disk_id"`
	DiskName   string `json:"disk_name"`
	CreateTime string `json:"create_time"`
	Remarks    string `json:"remarks"`
}
