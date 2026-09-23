package server

// 代登录（Impersonate）审计写入装配。
//
// account 模块的服务层只声明一个函数端口（service.LoginRecorder），不知道
// login_logs / user_sessions 的存在；这里用 security 模块的模型把端口实现出来。
// 这样 account 不必 import security（避免同层反向依赖），security 也不必知道
// 「代登录」这个业务动作。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	accountservice "hostsent/backend/internal/modules/admin/user/account/service"
	securitymodel "hostsent/backend/internal/modules/admin/user/security/model"
)

// impersonatePlatform 代登录会话与日志的 platform 标记。
//
// 刻意不用 "web"：会话列表里必须能一眼区分「用户自己登的」与「管理员代登的」，
// 否则运营排查「这个用户现在有几个登录态」时会把自己的代登录算进用户的正常登录。
const impersonatePlatform = "admin"

// impersonateLoginType 代登录在 login_logs.login_type 上的取值。
// 与 password / sms / email / 各 oauth provider 并列，便于按登录方式筛选。
const impersonateLoginType = "impersonate"

// newImpersonationRecorder 构造代登录审计写入器，返回新建会话的 session_id。
//
// 用单条事务写 login_logs + user_sessions：两条记录描述的是同一件事，
// 只落一半（比如日志写了、会话没写）会让安全页出现「有登录记录但强制下线
// 找不到会话」的矛盾状态，比完全不写更难排查。
//
// sessionTTL 用于写 expired_at，与 JWT 有效期对齐 —— 代登录令牌到期后，
// 会话必须跟着失效，否则在线用户里会一直挂着一条早已不可用的会话。
func newImpersonationRecorder(db *gorm.DB, sessionTTL time.Duration) accountservice.LoginRecorder {
	return func(ctx context.Context, in accountservice.ImpersonationRecord) (string, error) {
		if db == nil {
			return "", errors.New("代登录审计存储不可用")
		}
		now := time.Now()
		// 发起人信息塞进 failure_reason：该列在成功记录里本来是空的，
		// 复用它承载「代登录：admin（#1）」不需要新增迁移，且安全页的
		// 「失败原因」列会直接显示出来，无需改前端列定义。
		reason := ""
		if in.AdminName != "" || in.AdminID > 0 {
			reason = "代登录：" + in.AdminName
			if in.AdminID > 0 {
				reason = reason + " (#" + strconv.FormatUint(in.AdminID, 10) + ")"
			}
		}

		sessionID, err := impersonateSessionID()
		if err != nil {
			return "", err
		}
		var expiredAt *time.Time
		if sessionTTL > 0 {
			exp := now.Add(sessionTTL)
			expiredAt = &exp
		}

		err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&securitymodel.LoginLog{
				UserID:        in.UserID,
				Username:      in.Username,
				LoginType:     impersonateLoginType,
				Result:        "success",
				FailureReason: truncateUTF8(reason, 255),
				IP:            in.IP,
				UserAgent:     truncateUTF8(in.UserAgent, 255),
				Platform:      impersonatePlatform,
				RiskFlag:      "normal",
				CreatedAt:     now,
			}).Error; err != nil {
				return err
			}

			return tx.Create(&securitymodel.Session{
				SessionID:    sessionID,
				UserID:       in.UserID,
				Username:     in.Username,
				Platform:     impersonatePlatform,
				IP:           in.IP,
				UserAgent:    truncateUTF8(in.UserAgent, 255),
				LoginAt:      now,
				LastActiveAt: now,
				ExpiredAt:    expiredAt,
				Status:       "active",
				RiskFlag:     "normal",
				CreatedAt:    now,
				UpdatedAt:    now,
			}).Error
		})
		if err != nil {
			return "", err
		}
		return sessionID, nil
	}
}

// truncateUTF8 按字节截断到 UTF-8 字符边界（DB 列有长度上限，超长会整条 INSERT 失败）。
// 与 uc/auth、uc/oauth 的同名工具同语义，各自持有是因为它们分属不同模块。
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

// impersonateSessionID 生成代登录会话标识。
//
// session_id 是 uniqueIndex，16 字节随机串足够避免碰撞；前缀 "impersonate_"
// 让它在会话表里可被直接识别（与 oauth 的 "oauth_<provider>_" 同思路）。
func impersonateSessionID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "impersonate_" + hex.EncodeToString(buf), nil
}
