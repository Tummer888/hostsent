package model

import "time"

// 员工类型（S1 员工体系）：决定该账号在组织内的职能，用于工单派单
// 与销售归属的候选范围筛选（doc86 §1）。staff_type 与角色(roles)正交：
// 角色管权限点，员工类型管业务身份。一个账号可以既是 sales 又持有运营角色。
const (
	StaffTypeAdmin   string = "admin"   // 平台管理（通用后台）
	StaffTypeSales   string = "sales"   // 销售（可被分配客户、产生提成）
	StaffTypeSupport string = "support" // 客服（工单受理）
	StaffTypeTech    string = "tech"    // 技术（工单处理）
	StaffTypeOps     string = "ops"     // 运维
	StaffTypeFinance string = "finance" // 财务
)

// StaffTypes 返回全部合法员工类型（前端下拉与后端校验共用）。
func StaffTypes() []string {
	return []string{StaffTypeAdmin, StaffTypeSales, StaffTypeSupport, StaffTypeTech, StaffTypeOps, StaffTypeFinance}
}

// IsValidStaffType 校验员工类型是否为已知枚举值。
func IsValidStaffType(t string) bool {
	for _, v := range StaffTypes() {
		if v == t {
			return true
		}
	}
	return false
}

// Admin 管理员账号，与普通用户(users)物理分表存储。
type Admin struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"column:username;size:64;not null;uniqueIndex"`
	Email        string `gorm:"column:email;size:128;not null;uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash;size:255;not null"`
	Avatar       string `gorm:"column:avatar;size:255"`
	Role         string `gorm:"column:role;size:32;not null;default:admin"` // 兼容/展示字段，鉴权改读 admin_roles
	// Department 历史自由文本部门（迁移期兼容保留）：新数据写 department_id，
	// 迁移 040 已按名称回填 departments 表并把 department_id 指向对应行（doc86 §1）。
	Department string `gorm:"column:department;size:64"`
	Position   string `gorm:"column:position;size:64"`
	// —— S1 员工体系扩展 ——
	RealName string `gorm:"column:real_name;size:64"` // 真实姓名（工单/销售展示用，区别于 username）
	Phone    string `gorm:"column:phone;size:32"`     // 联系电话
	// DepartmentID 归属部门（departments.id），0 表示未分配。
	DepartmentID uint64 `gorm:"column:department_id;index"`
	// StaffType 员工类型，见 StaffType* 常量；默认 admin 兼容存量账号。
	StaffType string `gorm:"column:staff_type;size:32;not null;default:admin"`
	// SalesEnabled 是否开启销售能力（可被分配客户）：与 staff_type=sales 解耦，
	// 允许「销售主管」等角色按需开关，关闭后不再出现在销售归属候选列表。
	SalesEnabled bool       `gorm:"column:sales_enabled;not null;default:false"`
	JoinedAt     *time.Time `gorm:"column:joined_at"`   // 入职时间
	ResignedAt   *time.Time `gorm:"column:resigned_at"` // 离职时间，非空表示已离职（不再派单/归属）
	// ServiceGroupID 工单归属客服组（P2 自动派单按组选择在岗员工）。
	// 迁移期兜底：departments 未配置时，派单仍可回落到 service_group_id 选择候选。
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
