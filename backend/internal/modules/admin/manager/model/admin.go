package model

import "time"

// Admin 管理员账号，与普通用户(users)物理分表存储。
type Admin struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"column:username;size:64;not null;uniqueIndex"`
	Email        string `gorm:"column:email;size:128;not null;uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash;size:255;not null"`
	Avatar       string `gorm:"column:avatar;size:255"`
	Role         string `gorm:"column:role;size:32;not null;default:admin"` // 兼容/展示字段，鉴权改读 admin_roles
	Department   string `gorm:"column:department;size:64"`
	Position     string `gorm:"column:position;size:64"`
	// ServiceGroupID 工单归属客服组（P2 自动派单按组选择在岗员工）。
	ServiceGroupID *uint64 `gorm:"column:service_group_id"`
	// MustChangePassword 首次登录/重置密码后置 true，强制管理员改密（P1-08）。
	MustChangePassword bool       `gorm:"column:must_change_password;not null;default:false"`
	IPWhitelist        *string    `gorm:"column:ip_whitelist;type:jsonb"`
	MFAEnabled         bool       `gorm:"column:mfa_enabled;not null;default:false"`
	MFASecret          string     `gorm:"column:mfa_secret;size:255"`
	Status             string     `gorm:"column:status;size:32;not null;default:active"`
	LastLoginAt        *time.Time `gorm:"column:last_login_at"`
	LastLoginIP        string     `gorm:"column:last_login_ip;size:64"`
	LastOperateAt      *time.Time `gorm:"column:last_operate_at"`
	CreatedAt          time.Time  `gorm:"autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime"`
}

func (Admin) TableName() string {
	return "admins"
}

// AdminAuditLog 管理员操作审计日志，独立于普通用户日志。
type AdminAuditLog struct {
	ID           uint64  `gorm:"primaryKey;autoIncrement"`
	AdminID      uint64  `gorm:"column:admin_id;not null;index"`
	AdminName    string  `gorm:"column:admin_name;size:64;not null"`
	Action       string  `gorm:"column:action;size:64;not null;index"`
	ResourceType string  `gorm:"column:resource_type;size:64;not null;index"`
	ResourceID   string  `gorm:"column:resource_id;size:64;index"`
	Detail       *string `gorm:"column:detail;type:jsonb"`
	IP           string  `gorm:"column:ip;size:64;index"`
	UserAgent    string  `gorm:"column:user_agent;size:255"`
	// —— P2-06 审计中间件上报字段 ——
	Module        string    `gorm:"column:module;size:64;index"`
	RequestMethod string    `gorm:"column:request_method;size:16"`
	RequestPath   string    `gorm:"column:request_path;size:255"`
	ResponseCode  int       `gorm:"column:response_code;not null;default:0"`
	TraceID       string    `gorm:"column:trace_id;size:64"`
	CreatedAt     time.Time `gorm:"autoCreateTime;index"`
}

func (AdminAuditLog) TableName() string {
	return "admin_audit_logs"
}
