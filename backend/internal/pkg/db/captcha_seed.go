package db

import (
	"time"

	"gorm.io/gorm"

	systemmodel "hostsent/backend/internal/modules/admin/system/model"
	captchamodel "hostsent/backend/internal/modules/uc/captcha/model"
)

// captchaSystemConfigs 验证码体系相关开关默认值（doc91 §9.1/§9.2）。
//
// 全部取「默认安全侧」：captcha_enabled=false 时升级后登录行为与升级前一致，
// login_fail_lock=false 时失败不锁定；运营配好策略后再开总闸。
func captchaSystemConfigs() []systemmodel.SystemConfig {
	return []systemmodel.SystemConfig{
		{ConfigKey: "captcha_enabled", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "验证码总闸（关闭时所有场景策略不生效）", SortOrder: 20, Status: systemmodel.StatusActive},
		{ConfigKey: "captcha_provider", ConfigValue: "native", ValueType: systemmodel.ValueTypeString, Group: systemmodel.ConfigGroupSecurity, Description: "图形验证码兜底服务商类型", SortOrder: 21, Status: systemmodel.StatusActive},
		{ConfigKey: "captcha_image_ttl_seconds", ConfigValue: "120", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "图形验证码有效期（秒）", SortOrder: 22, Status: systemmodel.StatusActive},
		{ConfigKey: "verify_code_ttl_seconds", ConfigValue: "300", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "验证码有效期（秒）", SortOrder: 23, Status: systemmodel.StatusActive},
		{ConfigKey: "verify_code_send_interval_seconds", ConfigValue: "60", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "验证码发送间隔（秒）", SortOrder: 24, Status: systemmodel.StatusActive},
		{ConfigKey: "verify_code_daily_limit", ConfigValue: "10", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "单目标每日验证码发送上限", SortOrder: 25, Status: systemmodel.StatusActive},
		{ConfigKey: "verify_code_max_attempts", ConfigValue: "5", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "验证码单次校验次数上限", SortOrder: 26, Status: systemmodel.StatusActive},
		{ConfigKey: "captcha_send_ip_hourly_limit", ConfigValue: "20", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "单 IP 每小时验证码发送上限", SortOrder: 27, Status: systemmodel.StatusActive},
		{ConfigKey: "captcha_image_ip_minute_limit", ConfigValue: "30", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "单 IP 每分钟图形码获取上限", SortOrder: 28, Status: systemmodel.StatusActive},
		{ConfigKey: "login_fail_lock", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "登录失败是否锁定账号", SortOrder: 29, Status: systemmodel.StatusActive},
		{ConfigKey: "login_fail_threshold", ConfigValue: "5", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "登录失败锁定阈值（次）", SortOrder: 30, Status: systemmodel.StatusActive},
		{ConfigKey: "login_lock_minutes", ConfigValue: "15", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "登录锁定时长（分钟）", SortOrder: 31, Status: systemmodel.StatusActive},
		{ConfigKey: "mfa_required", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "是否强制所有场景二次验证（全局提升）", SortOrder: 32, Status: systemmodel.StatusActive},
		{ConfigKey: "register_enabled", ConfigValue: "true", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupFeature, Description: "是否开放用户注册", SortOrder: 2, Status: systemmodel.StatusActive},
		{ConfigKey: "api_rate_limit", ConfigValue: "0", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "API 每分钟请求上限（0=不限）", SortOrder: 33, Status: systemmodel.StatusActive},
		{ConfigKey: "password_min_length", ConfigValue: "6", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "密码最小长度", SortOrder: 34, Status: systemmodel.StatusActive},
		{ConfigKey: "password_max_length", ConfigValue: "64", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "密码最大长度", SortOrder: 35, Status: systemmodel.StatusActive},
		{ConfigKey: "password_require_upper", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "密码是否要求大写字母", SortOrder: 36, Status: systemmodel.StatusActive},
		{ConfigKey: "password_require_lower", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "密码是否要求小写字母", SortOrder: 37, Status: systemmodel.StatusActive},
		{ConfigKey: "password_require_digit", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "密码是否要求数字", SortOrder: 38, Status: systemmodel.StatusActive},
		{ConfigKey: "password_require_symbol", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "密码是否要求特殊字符", SortOrder: 39, Status: systemmodel.StatusActive},
	}
}

// seedCaptchaPolicies 写入 17 个场景的平台基线策略（doc91 §1.2.2）。
//
// 幂等：仅补缺失场景，绝不覆盖运营已改过的行（admin_login 建议开图形码等
// 属于「建议」，不能每次启动都推翻运营决定）。
func seedCaptchaPolicies(tx *gorm.DB) error {
	defaults := []captchamodel.CaptchaPolicy{
		{Scene: captchamodel.SceneAdminLogin, Name: "管理端登录", OTPChannel: captchamodel.ChannelSMS, Remark: "管理端登录图形码；默认关，建议开"},
		{Scene: captchamodel.SceneUserLogin, Name: "用户端登录", OTPChannel: captchamodel.ChannelEmail, Remark: "密码登录图形码"},
		{Scene: captchamodel.SceneUserLoginSMS, Name: "用户端短信登录", ImageRequired: true, OTPChannel: captchamodel.ChannelSMS, Remark: "短信登录必有图形码（防短信轰炸）"},
		{Scene: captchamodel.SceneUserLoginEmail, Name: "用户端邮箱登录", ImageRequired: true, OTPChannel: captchamodel.ChannelEmail, Remark: "邮箱登录必有图形码（防邮件轰炸）"},
		{Scene: captchamodel.SceneUserRegister, Name: "用户注册", ImageRequired: true, OTPRequired: true, OTPChannel: captchamodel.ChannelEmail, Remark: "注册需邮箱验证码"},
		{Scene: captchamodel.ScenePasswordReset, Name: "忘记密码", ImageRequired: true, OTPRequired: true, OTPChannel: captchamodel.ChannelEmail},
		{Scene: captchamodel.ScenePasswordChange, Name: "修改密码", OTPRequired: true, OTPChannel: captchamodel.ChannelSMS, Remark: "关键操作"},
		{Scene: captchamodel.ScenePhoneBind, Name: "绑定手机", OTPRequired: true, OTPChannel: captchamodel.ChannelSMS, Remark: "必须验短信（验的是要绑的号）"},
		{Scene: captchamodel.SceneEmailBind, Name: "绑定邮箱", OTPRequired: true, OTPChannel: captchamodel.ChannelEmail, Remark: "必须验邮箱"},
		{Scene: captchamodel.SceneWithdrawApply, Name: "发起提现", OTPRequired: true, OTPChannel: captchamodel.ChannelSMS, MinChannelLevel: captchamodel.ChannelLevelSMS, Remark: "资金类，最低通道等级 sms"},
		{Scene: captchamodel.ScenePayoutApply, Name: "销售提成提现申请", OTPRequired: true, OTPChannel: captchamodel.ChannelSMS, MinChannelLevel: captchamodel.ChannelLevelSMS, Remark: "资金类（doc86）"},
		{Scene: captchamodel.SceneAPIKeyCreate, Name: "创建 API 密钥", OTPRequired: true, OTPChannel: captchamodel.ChannelSMS, Remark: "开放平台凭据类"},
		{Scene: captchamodel.SceneAPIKeyView, Name: "查看 API 密钥明文", OTPRequired: true, OTPChannel: captchamodel.ChannelSMS, Remark: "凭据类"},
		{Scene: captchamodel.SceneInstanceDestroy, Name: "销毁实例", OTPRequired: true, OTPChannel: captchamodel.ChannelSMS, Remark: "不可逆"},
		{Scene: captchamodel.SceneInstanceResize, Name: "实例变配", OTPChannel: captchamodel.ChannelSMS, Remark: "可逆但影响服务"},
		{Scene: captchamodel.SceneAdminGrant, Name: "变更员工权限", OTPRequired: true, OTPChannel: captchamodel.ChannelSMS, Remark: "管理端高危"},
		{Scene: captchamodel.SceneRealnameSubmit, Name: "提交实名认证", ImageRequired: true, OTPChannel: captchamodel.ChannelEmail},
	}
	for i := range defaults {
		row := defaults[i]
		if row.Status == "" {
			row.Status = captchamodel.StatusActive
		}
		if row.MaxAttempts == 0 {
			row.MaxAttempts = 5
		}
		if row.ImageLevel == "" {
			row.ImageLevel = "normal"
		}
		var count int64
		if err := tx.Model(&captchamodel.CaptchaPolicy{}).Where("scene = ?", row.Scene).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		row.CreatedAt = time.Now()
		row.UpdatedAt = time.Now()
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
