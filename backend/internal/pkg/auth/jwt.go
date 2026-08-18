package auth

import (
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   uint64   `json:"user_id"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Roles    []string `json:"roles"`
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

func (j *JWTIssuer) Generate(claims *Claims) (string, error) {
	now := time.Now()
	payload := *claims
	payload.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    j.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.expireIn)),
		NotBefore: jwt.NewNumericDate(now),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, payload).SignedString(j.secretKey)
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
