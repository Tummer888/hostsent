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

// GenerateUser 生成普通用户 token。
func (j *JWTIssuer) GenerateUser(username string, userID uint64, tier string) (string, error) {
	claims := UserClaims{
		UserID:           userID,
		Username:         username,
		Tier:             tier,
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
