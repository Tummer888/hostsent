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

// GenerateUser 生成普通用户 token（主账号场景，等价于 GenerateUserFull 的子账号参数为空）。
func (j *JWTIssuer) GenerateUser(username string, userID uint64, tier string) (string, error) {
	return j.GenerateUserFull(username, userID, tier, 0, false)
}

// GenerateUserFull 生成带归属信息的普通用户 token（P4-03）。
// ownerUserID/isSub 为子账号信息：主账号传 0/false，子账号传主账号 ID 与 true。
func (j *JWTIssuer) GenerateUserFull(username string, userID uint64, tier string, ownerUserID uint64, isSub bool) (string, error) {
	claims := UserClaims{
		UserID:           userID,
		Username:         username,
		Tier:             tier,
		OwnerUserID:      ownerUserID,
		IsSub:            isSub,
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
