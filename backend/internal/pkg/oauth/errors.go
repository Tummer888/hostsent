package oauth

import (
	"errors"
	"fmt"
)

// ErrAdapterNotImplemented 适配器已登记描述符但未提供真实实现。
var ErrAdapterNotImplemented = errors.New("该第三方登录渠道尚未接入")

// ErrProviderConfig 渠道配置缺失或不可用（未启用、凭证解密失败、凭证不全）。
var ErrProviderConfig = errors.New("第三方登录渠道配置不可用")

// ErrProviderDisabled 该渠道未启用。
var ErrProviderDisabled = errors.New("该第三方登录渠道未启用")

// ErrExchangeFailed 授权码换令牌失败（上游拒绝或网络故障）。
var ErrExchangeFailed = errors.New("第三方登录授权失败")

// ErrUserInfoFailed 拉取外部用户资料失败。
var ErrUserInfoFailed = errors.New("获取第三方账号资料失败")

// ErrOpenIDTaken 该第三方账号已绑定到另一个用户。
var ErrOpenIDTaken = errors.New("该第三方账号已绑定其他用户")

// ErrLastLoginMethod 解绑被拒：解绑后账号将没有任何可用的登录方式。
var ErrLastLoginMethod = errors.New("解绑后账号将无法登录，请先设置登录密码或绑定其他方式")

// ErrNeedBind 外部账号未绑定任何平台用户，且平台未开启自动注册。
var ErrNeedBind = errors.New("该第三方账号尚未注册，请先用账号密码登录后绑定")

func validateCredentials(d CapabilityDescriptor, creds map[string]string) error {
	for _, f := range d.CredentialSchema {
		if !f.Required {
			continue
		}
		if v, ok := creds[f.Key]; !ok || v == "" {
			label := f.Label
			if label == "" {
				label = f.Key
			}
			return fmt.Errorf("第三方登录凭证字段 %s 必填", label)
		}
	}
	return nil
}
