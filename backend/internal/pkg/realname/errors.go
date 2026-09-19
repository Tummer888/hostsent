package realname

import (
	"errors"
	"fmt"
)

// ErrAdapterNotImplemented 适配器已登记描述符但未提供真实实现。
//
// 占位 provider 用它把「渠道在 UI 上完整可见」与「功能尚未接入」两件事分开：
// 后台可以创建、保存、测试，调用时得到明确提示，而不是 500（doc104 §1.5）。
var ErrAdapterNotImplemented = errors.New("该实名核验服务商尚未接入")

// ErrVerifyFailed 核验未通过（业务判定，不是系统故障）。
var ErrVerifyFailed = errors.New("实名核验未通过")

// ErrProviderConfig 服务商配置缺失或不可用（如未启用、凭证解密失败）。
var ErrProviderConfig = errors.New("实名核验服务商配置不可用")

// ErrNoProvider 当前没有任何可用的核验服务商。
var ErrNoProvider = errors.New("未配置可用的实名核验服务商")

// ValidateCredentials 按描述符校验必填凭证字段（与 captcha/notifier 口径一致）。
func ValidateCredentials(d CapabilityDescriptor, creds map[string]string) error {
	for _, f := range d.CredentialSchema {
		if !f.Required {
			continue
		}
		if v, ok := creds[f.Key]; !ok || v == "" {
			label := f.Label
			if label == "" {
				label = f.Key
			}
			return fmt.Errorf("实名核验凭证字段 %s 必填", label)
		}
	}
	return nil
}
