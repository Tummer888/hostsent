package db

import (
	"encoding/json"

	"gorm.io/gorm"

	systemmodel "hostsent/backend/internal/modules/admin/system/model"
	verificationmodel "hostsent/backend/internal/modules/admin/user/verification/model"
	oauthmodel "hostsent/backend/internal/modules/uc/oauth/model"
	oauthpkg "hostsent/backend/internal/pkg/oauth"
	realnamepkg "hostsent/backend/internal/pkg/realname"
)

// oauthSystemConfigs 第三方登录的运行期配置（doc104 §6.4）。
//
// 回调地址与前端回跳地址都做成配置项而不是写死在代码里：部署域名各不相同，
// 写死意味着「换域名要改代码发版」。代码里给的兜底值（本机 8080 /oauth/callback）
// 只服务于纯后端联调，生产必须由运营在后台填写。
func oauthSystemConfigs() []systemmodel.SystemConfig {
	return []systemmodel.SystemConfig{
		{
			ConfigKey:   "oauth.callback_base",
			ConfigValue: "",
			ValueType:   systemmodel.ValueTypeString,
			Group:       systemmodel.ConfigGroupSecurity,
			Description: "第三方登录后端回调基地址，如 https://api.example.com/api/v1/uc/oauth（留空用进程内兜底）",
			SortOrder:   20,
			Status:      systemmodel.StatusActive,
		},
		{
			ConfigKey:   "oauth.frontend_callback",
			ConfigValue: "/oauth/callback",
			ValueType:   systemmodel.ValueTypeString,
			Group:       systemmodel.ConfigGroupSecurity,
			Description: "第三方登录完成后前端回跳路径，如 https://www.example.com/oauth/callback",
			SortOrder:   21,
			Status:      systemmodel.StatusActive,
		},
		{
			ConfigKey:   "oauth.auto_register",
			ConfigValue: "true",
			ValueType:   systemmodel.ValueTypeBool,
			Group:       systemmodel.ConfigGroupSecurity,
			Description: "第三方账号首次登录时自动创建平台账号（关闭则要求先绑定已有账号）",
			SortOrder:   22,
			Status:      systemmodel.StatusActive,
		},
	}
}

// seedDefaultOAuthProviders 预置三家第三方登录渠道（doc104 §7）。
//
// 幂等口径与 seedNotifyChannelConfigs 一致：按 provider 键存在即跳过，
// 绝不覆盖运营已编辑的凭证。默认 enabled=false —— 没有 AppID/AppSecret 的渠道
// 若默认启用，登录页会出现点了就报错的图标。
func seedDefaultOAuthProviders(tx *gorm.DB) error {
	for i, d := range oauthpkg.AllDescriptors() {
		var count int64
		if err := tx.Model(&oauthmodel.OAuthProvider{}).
			Where("provider = ? AND deleted_at IS NULL", d.Type).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		descriptor, err := json.Marshal(d)
		if err != nil {
			return err
		}
		row := oauthmodel.OAuthProvider{
			Provider: d.Type,
			Name:     d.Name,
			Enabled:  false,
			Mode:     d.Mode,
			Icon:     d.Icon,
			Scopes:   d.DefaultScopes,
			// 描述符快照落库：便于事后审计「当时运营看到的字段定义是什么」。
			Descriptor: string(descriptor),
			SortOrder:  int64(i + 1),
			// credentials 是 jsonb 列，空串不是合法 JSON（PG 报 22P02）。
			// 必须显式写 "{}"，否则一旦有新渠道登记进描述符注册表，
			// 这条 seed 就会把整个启动流程带崩。
			Credentials: "{}",
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedDefaultRealnameProvider 预置实名核验服务商（doc104 §7）。
//
// manual 为内置默认：它是「未接入三方核验」时的合法兜底路径，恒可用、不可删。
// alipay 只登记一条停用记录，让运营在配置页能看到「可以接哪家」，凭证留空。
func seedDefaultRealnameProvider(tx *gorm.DB) error {
	descriptors := realnamepkg.AllDescriptors()
	for _, d := range descriptors {
		var count int64
		if err := tx.Model(&verificationmodel.RealnameProvider{}).
			Where("provider_type = ? AND deleted_at IS NULL", d.Type).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		descriptor, err := json.Marshal(d)
		if err != nil {
			return err
		}
		row := verificationmodel.RealnameProvider{
			ProviderType: d.Type,
			Name:         d.Name,
			Mode:         d.Mode,
			Descriptor:   string(descriptor),
			// credentials 是 jsonb 列，空串会触发 22P02；与 oauth 侧同口径显式写 "{}"。
			Credentials: "{}",
			// 内置 manual 启用且为默认；三方核验默认停用，避免配置缺失时被选中。
			Status:    boolToStatus(d.Builtin),
			IsDefault: d.Builtin,
			Remark:    d.Name,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func boolToStatus(enabled bool) int {
	if enabled {
		return 1
	}
	return 0
}

// seedVerificationConfigs 写入实名配置默认值（doc104 §5.4）。
//
// 默认值取自 model.VerificationConfigDefaults()，与运行时回落同源，
// 避免「库里 24 小时、代码回落 48 小时」这类两套口径。
func seedVerificationConfigs(tx *gorm.DB) error {
	for _, def := range verificationmodel.VerificationConfigDefaults() {
		var count int64
		if err := tx.Model(&verificationmodel.VerificationConfig{}).
			Where("config_key = ?", def.Key).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		row := verificationmodel.VerificationConfig{
			ConfigKey:   def.Key,
			ConfigGroup: verificationmodel.ConfigGroupVerification,
			ConfigValue: def.Value,
			ValueType:   def.ValueType,
			Status:      systemmodel.StatusActive,
			Description: def.Description,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
