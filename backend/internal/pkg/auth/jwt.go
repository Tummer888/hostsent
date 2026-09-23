package auth

import (
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// 用户类型常量，用于区分管理员与普通用户 token。
const (
	UserTypeAdmin = "admin"
	UserTypeUser  = "user"
)

// AudienceOTPPending 登录二次验证待验证令牌的 audience（doc91 §4.6）。
//
// 这一条是防越权的关键：普通访问令牌的 aud 不是这个值，因此不能拿来换登录，
// 待验证令牌也不能当访问令牌用（audience 不同、不含权限、TTL 5 分钟、只能用一次）。
const AudienceOTPPending = "otp_pending"

// OTPPendingClaims 二次验证待验证令牌声明。
type OTPPendingClaims struct {
	UserID  uint64 `json:"user_id"`
	Scene   string `json:"scene"`
	IsAdmin bool   `json:"is_admin,omitempty"`
	jwt.RegisteredClaims
}

type Claims struct {
	UserID   uint64   `json:"user_id"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Roles    []string `json:"roles"`
	UserType string   `json:"user_type"`
	jwt.RegisteredClaims
}

// AdminClaims 管理员专属声明。
type AdminClaims struct {
	AdminID  uint64 `json:"admin_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type UserClaims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Tier     string `json:"tier"`
	// OwnerUserID 子账号归属的主账号 ID；主账号为 0（旧 token 缺该字段即视为主账号，P4-03）。
	OwnerUserID uint64 `json:"owner_user_id,omitempty"`
	// IsSub 是否子账号。
	IsSub bool `json:"is_sub,omitempty"`
	// SessionID 该令牌对应的 user_sessions.session_id。
	//
	// 这是「强制下线」能真正生效的前提：JWT 本身无状态，签发后无法撤回，
	// 只能靠令牌里带一个可被服务端查证的句柄。中间件据此查会话是否仍有效
	// （存在 + status=active + 未过期），管理员踢人或用户登出后请求立即 401。
	//
	// 刻意用 omitempty 而不是必填：旧令牌没有该字段，中间件对缺字段的令牌
	// 一律拒绝（见 middleware.UserAuth），这样「升级前的令牌」不会变成绕过口。
	SessionID string `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

type JWTIssuer struct {
	secretKey []byte
	issuer    string
	expireIn  time.Duration
}

func NewJWTIssuer(secret string, issuer string, expireIn time.Duration) *JWTIssuer {
	return &JWTIssuer{
		secretKey: []byte(secret),
		issuer:    issuer,
		expireIn:  expireIn,
	}
}

// ExpireIn 返回访问令牌有效期。
//
// 开会话时要用它写 user_sessions.expired_at：会话的「有效」与令牌的「未过期」
// 必须同步，否则会出现「令牌还能用但会话已判过期」（用户被莫名踢下线）
// 或反过来的「令牌过期了会话仍算在线」（在线数虚高）这两种矛盾状态。
func (j *JWTIssuer) ExpireIn() time.Duration {
	return j.expireIn
}

// Generate 生成兼容旧接口的 Claims token（默认不设 UserType，视为管理员）。
func (j *JWTIssuer) Generate(claims *Claims) (string, error) {
	now := time.Now()
	payload := *claims
	payload.RegisteredClaims = j.buildRegisteredClaims(now)
	return jwt.NewWithClaims(jwt.SigningMethodHS256, payload).SignedString(j.secretKey)
}

// GenerateAdmin 生成管理员 token。
func (j *JWTIssuer) GenerateAdmin(username string, adminID uint64, role string) (string, error) {
	claims := AdminClaims{
		AdminID:          adminID,
		Username:         username,
		Role:             role,
		RegisteredClaims: j.buildRegisteredClaims(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secretKey)
}

// GenerateUserSession 生成绑定了会话句柄的普通用户 token（强制下线的前提）。
//
// 这是**唯一**的用户令牌签发入口。此前还有 GenerateUser / GenerateUserFull
// 两个不带会话句柄的版本，现在已删除：它们签出的令牌因 sid 为空会被中间件
// 一律拒绝，留着只会让调用方拿到一枚「必然 401」的令牌却看不到任何报错。
// 删掉后误用会直接编译失败，被逼着先去 user_sessions 开会话。
//
// sessionID 必须非空且对应的会话已在 user_sessions 里落库（存在 + active +
// 未过期），中间件每次请求都会校验。顺序不能反：先签令牌再补会话，
// 补写失败就会签出一张立刻不可用的令牌。
func (j *JWTIssuer) GenerateUserSession(username string, userID uint64, tier string, ownerUserID uint64, isSub bool, sessionID string) (string, error) {
	claims := UserClaims{
		UserID:           userID,
		Username:         username,
		Tier:             tier,
		OwnerUserID:      ownerUserID,
		IsSub:            isSub,
		SessionID:        sessionID,
		RegisteredClaims: j.buildRegisteredClaims(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secretKey)
}

func (j *JWTIssuer) buildRegisteredClaims(now time.Time) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    j.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.expireIn)),
		NotBefore: jwt.NewNumericDate(now),
	}
}

// OTPPendingTTL 待验证令牌有效期（固定 5 分钟，不随 jwt_expire_hours 走）。
const OTPPendingTTL = 5 * time.Minute

// GenerateOTPPending 生成登录二次验证待验证令牌。
//
// jti 由调用方生成并作为唯一标识写入缓存，校验时同时验证 aud 与缓存中的 jti，
// 保证令牌只能用一次。
func (j *JWTIssuer) GenerateOTPPending(userID uint64, scene string, isAdmin bool, jti string) (string, error) {
	now := time.Now()
	claims := OTPPendingClaims{
		UserID:  userID,
		Scene:   scene,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   u64str(userID),
			Audience:  jwt.ClaimStrings{AudienceOTPPending},
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(OTPPendingTTL)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secretKey)
}

// ParseOTPPending 解析待验证令牌，并强制校验 audience=otp_pending。
func (j *JWTIssuer) ParseOTPPending(tokenString string) (*OTPPendingClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &OTPPendingClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid token signing method")
		}
		return j.secretKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*OTPPendingClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	// 防越权关键点：audience 必须是 otp_pending，普通访问令牌在此被拒。
	if !audienceHas(claims.Audience, AudienceOTPPending) {
		return nil, errors.New("invalid token audience")
	}
	return claims, nil
}

// audienceHas 判断 audience 列表是否包含目标值。
func audienceHas(list jwt.ClaimStrings, want string) bool {
	for _, a := range list {
		if a == want {
			return true
		}
	}
	return false
}

func u64str(v uint64) string {
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

func (j *JWTIssuer) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid token signing method")
		}
		return j.secretKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (j *JWTIssuer) ParseAdmin(tokenString string) (*AdminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AdminClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid token signing method")
		}
		return j.secretKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AdminClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (j *JWTIssuer) ParseUser(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid token signing method")
		}
		return j.secretKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
