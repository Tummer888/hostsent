package notifier

// mandatoryEvents 强制送达事件白名单（doc89 §3 / doc91 §5.1）。
//
// 语义：这些事件属于「验证类消息」，不受用户通知偏好开关影响——
// 用户不能通过关掉「通知偏好」来关掉登录验证码。消息中心在偏好过滤前
// 必须先调 IsMandatoryEvent，命中则直接放行（doc90 §6 Bug④ 的修复点）。
var mandatoryEvents = map[string]bool{
	"login_otp":       true, // 登录二次验证
	"phone_bind":      true, // 绑定手机号
	"email_bind":      true, // 绑定邮箱
	"password_reset":  true, // 找回/重置密码
	"register_verify": true, // 注册验证
	"withdraw_verify": true, // 提现二次验证
	"apikey_verify":   true, // API 密钥操作二次验证
}

// IsMandatoryEvent 判断事件是否属于强制送达白名单（不受用户偏好影响）。
func IsMandatoryEvent(event string) bool {
	return mandatoryEvents[event]
}

// MandatoryEvents 返回白名单事件列表（升序，供后台展示与测试断言）。
func MandatoryEvents() []string {
	out := make([]string, 0, len(mandatoryEvents))
	for e := range mandatoryEvents {
		out = append(out, e)
	}
	// 数量固定且很小，插入排序足够；保持确定性输出便于断言。
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
