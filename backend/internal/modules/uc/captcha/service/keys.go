package service

import (
	"context"
	"crypto/subtle"
	"net/mail"
	"regexp"
	"strings"

	"hostsent/backend/internal/modules/uc/captcha/model"
)

// otpKey OTP 验证码 key（doc91 §2.3）。
func otpKey(scene, targetHash string) string { return "cap:otp:" + scene + ":" + targetHash }

// otpTryKey OTP 校验次数 key。
func otpTryKey(scene, targetHash string) string { return "cap:otp:try:" + scene + ":" + targetHash }

// sendGapKey 发送间隔锁 key。
func sendGapKey(scene, targetHash string) string { return "cap:send:gap:" + scene + ":" + targetHash }

// sendDayKey 每日发送计数 key。
func sendDayKey(scene, targetHash string) string { return "cap:send:day:" + scene + ":" + targetHash }

// ticketKey 关键操作验证票据 key。
func ticketKey(subject Subject, scene string) string {
	if subject.IsAdmin {
		return "auth:verify_ticket:admin:" + u64(subject.ID) + ":" + scene
	}
	return "auth:verify_ticket:" + u64(subject.ID) + ":" + scene
}

// otpPendingKey 登录二次验证待验证令牌 key。
func OTPPendingKey(jti string) string { return "auth:otp_pending:" + jti }

// failAcctKey 登录失败计数（账号维度）。
func failAcctKey(account string) string {
	return "auth:fail:acct:" + strings.ToLower(strings.TrimSpace(account))
}

// failIPKey 登录失败计数（IP 维度）。
func failIPKey(ip string) string { return "auth:fail:ip:" + ip }

// constantTimeEqual 常量时间比较（避免时序侧信道）。
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// validEmail 邮箱格式校验（net/mail 解析 + 必须含 @ 与域名点）。
func validEmail(s string) bool {
	if strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	at := strings.LastIndex(s, "@")
	return at > 0 && strings.Contains(s[at+1:], ".")
}

var cnMobileRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

// validCNMobile 中国大陆手机号校验。
func validCNMobile(s string) bool { return cnMobileRe.MatchString(s) }

// normalizePhone 去掉手机号里的空格、连字符与国际区号前缀。
func normalizePhone(raw string) string {
	s := strings.TrimSpace(raw)
	replacer := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", "+86", "", "+", "")
	s = replacer.Replace(s)
	return s
}

// channelsForPolicy 返回某场景允许的通道选项（用户端通道选择列表用）。
func channelsForPolicy(base *model.CaptchaPolicy) []string {
	all := []string{model.ChannelEmail, model.ChannelSMS, model.ChannelTOTP}
	if base == nil || base.MinChannelLevel <= 0 {
		return all
	}
	out := make([]string, 0, len(all))
	for _, ch := range all {
		if model.ChannelLevel(ch) >= base.MinChannelLevel {
			out = append(out, ch)
		}
	}
	return out
}

// u64 无依赖的十进制格式化（避免 fmt 在热路径的反射开销）。
func u64(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

var _ = context.Background
