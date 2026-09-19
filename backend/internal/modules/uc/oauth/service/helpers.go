package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// randomNonce 生成 16 字节随机串（state/ticket 的 jti）。
func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// randomSuffix 生成 4 字节随机后缀（用户名/邮箱去重）。
func randomSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		// 随机源不可用是环境级故障：退化成时间戳后缀，保证不 panic。
		return hex.EncodeToString([]byte(time.Now().Format("150405")))
	}
	return hex.EncodeToString(buf)
}

// randomPasswordHash 生成「不可用密码」的 bcrypt 哈希。
//
// 自动注册用户的密码是 32 字节随机值的哈希 —— 没有任何人（包括我们自己）
// 知道原文，因此该账号**永远无法用密码登录**，只能走第三方登录。
// 这比写空串或固定值更安全：空串哈希在个别实现里会退化成「任意密码都能通过」。
func randomPasswordHash() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// bcrypt 只取前 72 字节，32 字节随机已远超实际熵需求。
	hash, err := bcrypt.GenerateFromPassword(buf, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// recordLogin 写一条第三方登录成功日志（login_logs.login_type='oauth'）。
//
// 直接 INSERT 而不是复用 uc/auth 的 security.Port：端口层的 Log 需要构造
// security.LoginLogEntry，而 oauth 包不该反向依赖 uc/captcha 的端口类型。
// 字段与 login_logs 的 NOT NULL 列一一对应（user_id/username/login_type/result/ip/platform）。
func (s *service) recordLogin(ctx context.Context, userID uint64, username, provider, ip, userAgent string) error {
	if s.db == nil {
		return nil
	}
	return s.db.WithContext(ctx).Exec(`INSERT INTO login_logs
		(user_id, username, login_type, result, ip, user_agent, platform, created_at)
		VALUES (?, ?, ?, 'success', ?, ?, 'web', NOW())`,
		userID, username, provider, ip, truncate(userAgent, 255)).Error
}

// openSession 为第三方登录开一条会话记录（doc104 §6.5，F15）。
//
// 不写会话的话，「登录日志有记录但会话列表是空的」——运营在安全页排查
// 「这个用户现在有哪些登录态」时会看到空白，且「强制下线」对该会话无效。
func (s *service) openSession(ctx context.Context, userID uint64, username, provider, ip, userAgent string) error {
	if s.db == nil {
		return nil
	}
	sessionID, err := randomNonce()
	if err != nil {
		return err
	}
	now := time.Now()
	return s.db.WithContext(ctx).Exec(`INSERT INTO user_sessions
		(session_id, user_id, username, platform, ip, user_agent, login_at, last_active_at, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', NOW(), NOW())`,
		"oauth_"+provider+"_"+sessionID, userID, username, provider, ip, truncate(userAgent, 255), now, now).Error
}

// truncate 按字节截断（DB 列有长度上限，超长会整条 INSERT 失败）。
// 中文用户代理按字节截断可能切出半个字符，因此末尾做一次 UTF-8 边界回退。
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && !utf8ValidLast(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

// utf8ValidLast 判断字节串是否为合法 UTF-8（用于回退到字符边界）。
func utf8ValidLast(s string) bool {
	for i := len(s) - 1; i >= 0 && i >= len(s)-4; i-- {
		b := s[i]
		if b < 0x80 {
			return true
		}
		if b&0xC0 == 0xC0 {
			// 多字节首字节：检查后续续字节数量是否完整。
			want := 0
			switch {
			case b&0xE0 == 0xC0:
				want = 1
			case b&0xF0 == 0xE0:
				want = 2
			case b&0xF8 == 0xF0:
				want = 3
			default:
				return false
			}
			return len(s)-i-1 == want
		}
	}
	return false
}

// warnIf 统一的服务层告警入口，避免各处重复 zap 字段拼接。
func (s *service) warnIf(err error, msg string, fields ...zap.Field) {
	if err == nil {
		return
	}
	s.logger.Warn(msg, append(fields, zap.Error(err))...)
}

// mustJSON 序列化描述符快照；失败返回空串（描述符只是审计快照，不该阻断配置保存）。
func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}
