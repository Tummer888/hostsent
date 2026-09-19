package oauth

import (
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// StateTTL 授权 state 的有效期。
//
// 5 分钟：足够用户完成三方登录页的操作，又短到重放窗口可忽略。
const StateTTL = 5 * time.Minute

// AudienceOAuthState state 令牌的 audience。
//
// 与访问令牌、二次验证令牌的 audience 都不同，因此三类令牌互不可用
// （对齐 pkg/auth 里 AudienceOTPPending 的防越权手法）。
const AudienceOAuthState = "oauth_state"

// AudienceOAuthTicket 回调完成后回跳前端用的一次性票据 audience。
const AudienceOAuthTicket = "oauth_ticket"

// ErrInvalidState state 缺失、过期、被篡改或 audience 不符。
var ErrInvalidState = errors.New("第三方登录会话已失效，请重新发起")

// StateMode 本次流程的用途。
const (
	// ModeLogin 登录/自动注册。
	ModeLogin = "login"
	// ModeBind 已登录用户绑定新渠道。
	ModeBind = "bind"
)

// StateClaims 授权 state 的声明。
type StateClaims struct {
	Provider string `json:"provider"`
	// Mode login / bind。
	Mode string `json:"mode"`
	// UserID 仅 ModeBind 使用：绑定操作必须落到发起时的登录用户，
	// 防止攻击者用自己的 state 把别人的账号绑到自己的第三方账号上。
	UserID uint64 `json:"user_id,omitempty"`
	// InviteCode 仅 ModeLogin 使用：透传注册邀请码，使 OAuth 注册也能计入返现。
	InviteCode string `json:"invite_code,omitempty"`
	// Nonce 一次性随机串；Redis 可用时由缓存做一次性消费。
	Nonce string `json:"nonce"`
	jwt.RegisteredClaims
}

// TicketClaims 回调结果票据的声明。
//
// 回调是免登录入口，不能直接把访问令牌放进重定向 URL（会进浏览器历史、
// Referer、代理日志）。改为发一张 60 秒一次性票据，前端用它换正式令牌。
type TicketClaims struct {
	UserID   uint64 `json:"user_id"`
	Provider string `json:"provider"`
	// NeedBind 该第三方账号尚未绑定任何平台用户，且未开启自动注册。
	NeedBind bool `json:"need_bind,omitempty"`
	jwt.RegisteredClaims
}

// TicketTTL 结果票据有效期。
const TicketTTL = 60 * time.Second

// StateSigner 签发/校验 state 与 ticket。
//
// 为什么用签名 JWT 而不是「Redis 存 state」：
// pkg/cache 在 Redis 不可用时会降级为进程内缓存，多实例部署下状态不共享——
// 图形码可以「降级即放行」（pkg/captcha 的选择），但 CSRF 防护不能放行。
// 签名 JWT 自带完整性与过期，不依赖任何外部存储，是最低成本的正确解。
// Redis 可用时再叠加 nonce 一次性消费（见服务层），把重放窗口压到零。
type StateSigner struct {
	secret []byte
	issuer string
}

// NewStateSigner 创建 state 签发器。secret 复用 auth.jwt_secret。
func NewStateSigner(secret, issuer string) *StateSigner {
	return &StateSigner{secret: []byte(secret), issuer: issuer}
}

// IssueState 签发一次授权 state。
func (s *StateSigner) IssueState(provider, mode string, userID uint64, inviteCode, nonce string) (string, error) {
	now := time.Now()
	claims := StateClaims{
		Provider:   provider,
		Mode:       mode,
		UserID:     userID,
		InviteCode: inviteCode,
		Nonce:      nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{AudienceOAuthState},
			ID:        nonce,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(StateTTL)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// ParseState 解析并校验 state；audience 不符即拒（普通访问令牌在此被拦下）。
func (s *StateSigner) ParseState(tokenStr string) (*StateClaims, error) {
	if tokenStr == "" {
		return nil, ErrInvalidState
	}
	token, err := jwt.ParseWithClaims(tokenStr, &StateClaims{}, s.keyFunc)
	if err != nil {
		return nil, ErrInvalidState
	}
	claims, ok := token.Claims.(*StateClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidState
	}
	if !audienceHas(claims.Audience, AudienceOAuthState) {
		return nil, ErrInvalidState
	}
	if claims.Provider == "" || claims.Mode == "" {
		return nil, ErrInvalidState
	}
	return claims, nil
}

// IssueTicket 签发回调结果票据。
func (s *StateSigner) IssueTicket(userID uint64, provider string, needBind bool, nonce string) (string, error) {
	now := time.Now()
	claims := TicketClaims{
		UserID:   userID,
		Provider: provider,
		NeedBind: needBind,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{AudienceOAuthTicket},
			ID:        nonce,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TicketTTL)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// ParseTicket 解析并校验结果票据。
func (s *StateSigner) ParseTicket(tokenStr string) (*TicketClaims, error) {
	if tokenStr == "" {
		return nil, fmt.Errorf("%w: 票据为空", ErrInvalidState)
	}
	token, err := jwt.ParseWithClaims(tokenStr, &TicketClaims{}, s.keyFunc)
	if err != nil {
		return nil, fmt.Errorf("%w: 票据无效或已过期", ErrInvalidState)
	}
	claims, ok := token.Claims.(*TicketClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("%w: 票据无效", ErrInvalidState)
	}
	if !audienceHas(claims.Audience, AudienceOAuthTicket) {
		return nil, fmt.Errorf("%w: 票据 audience 不符", ErrInvalidState)
	}
	return claims, nil
}

func (s *StateSigner) keyFunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, errors.New("invalid token signing method")
	}
	return s.secret, nil
}

func audienceHas(list jwt.ClaimStrings, want string) bool {
	for _, a := range list {
		if a == want {
			return true
		}
	}
	return false
}
