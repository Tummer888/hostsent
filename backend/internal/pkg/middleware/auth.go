package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appauth "hostsent/backend/internal/pkg/auth"
	apperrors "hostsent/backend/internal/pkg/errors"
)

const claimsContextKey = "claims"

func Auth(jwtIssuer *appauth.JWTIssuer, bearerPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		parts := strings.SplitN(authorization, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], bearerPrefix) {
			err := apperrors.New(10001, "unauthorized")
			c.JSON(http.StatusUnauthorized, gin.H{"code": err.Code, "message": err.Message, "timestamp": timestamp()})
			c.Abort()
			return
		}

		claims, err := jwtIssuer.Parse(parts[1])
		if err != nil {
			appErr := apperrors.New(10001, "unauthorized")
			c.JSON(http.StatusUnauthorized, gin.H{"code": appErr.Code, "message": appErr.Message, "timestamp": timestamp()})
			c.Abort()
			return
		}

		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

func GetClaims(c *gin.Context) (*appauth.Claims, bool) {
	value, ok := c.Get(claimsContextKey)
	if !ok {
		return nil, false
	}
	claims, ok := value.(*appauth.Claims)
	return claims, ok
}
