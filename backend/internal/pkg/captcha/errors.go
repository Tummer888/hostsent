// Package captcha 提供图形验证码生成与第三方验证码服务商适配。
//
// 设计（doc91 §3 / §7）：
//   - 图形码必须服务端生成、服务端校验：答案只写进缓存，响应体不含答案、
//     不含 SVG 文本、不含按字符拆分的图元坐标（那等于把答案卖了）。
//   - 校验用「取一次即销毁」（GetDel）语义：同一 key 第二次校验必然失败。
//   - native 是内置兜底实现在任何情况下都可用；第三方不可用时回落 native。
package captcha

import "errors"

// ErrInvalid 验证码错误或已过期。对外不区分两者，避免给攻击者信息（doc91 §3.4）。
var ErrInvalid = errors.New("验证码错误或已过期")

// ErrTooManyAttempts 单 key 校验次数超限，key 已被销毁。
var ErrTooManyAttempts = errors.New("验证码错误次数过多，请重新获取")

// ErrAdapterNotImplemented 服务商适配器尚未接入（占位 provider）。
var ErrAdapterNotImplemented = errors.New("captcha: 该验证码服务商适配器尚未接入")

// ErrProviderDisabled 服务商被停用或凭证不可用。
var ErrProviderDisabled = errors.New("captcha: 验证码服务商未启用或配置无效")
