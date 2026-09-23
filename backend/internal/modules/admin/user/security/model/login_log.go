package model

import "time"

// 登录主体域：区分「客户登录」与「员工后台登录」。
//
// login_logs / user_sessions 的 user_id 列承载两个 ID 空间的值 —— 客户登录写
// users.id，员工后台登录写 admins.id。两张表各自自增，ID 天然会撞号（实测 7 组），
// 仅靠 platform 区分不够：代登录的 platform='admin' 却属于**客户域**（它描述的是
// 某个客户账号的登录态，运营要能在客户详情里看到并强制下线）。
//
// 因此必须有独立的域判别列，且默认落在客户域：客户侧登录（密码/短信/邮箱/OAuth）
// 是绝大多数写入方，新增写入路径不关心本列时天然正确。
const (
	SubjectTypeUser  = "user"
	SubjectTypeAdmin = "admin"
)

type LoginLog struct {
	ID                uint64 `gorm:"primaryKey;autoIncrement"`
	UserID            uint64 `gorm:"column:user_id;not null;index;index:idx_login_logs_subject_user,priority:2"`
	Username          string `gorm:"size:64;not null;index"`
	LoginType         string `gorm:"column:login_type;size:32;not null;index"`
	Result            string `gorm:"size:32;not null;index"`
	FailureReason     string `gorm:"column:failure_reason;size:255"`
	IP                string `gorm:"column:ip;size:64;not null;index"`
	UserAgent         string `gorm:"column:user_agent;size:255"`
	DeviceFingerprint string `gorm:"column:device_fingerprint;size:255"`
	Platform          string `gorm:"size:32;not null;index"`
	// SubjectType 登录主体域（user/admin），见上方常量说明。
	// default:user 让数据库兜底零值：Go 侧 struct Create 省略该列时不会写成空串。
	SubjectType string    `gorm:"column:subject_type;size:16;not null;default:user;index:idx_login_logs_subject_user,priority:1"`
	RiskFlag    string    `gorm:"column:risk_flag;size:32;index"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (LoginLog) TableName() string {
	return "login_logs"
}
