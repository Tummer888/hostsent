package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appauth "hostsent/backend/internal/pkg/auth"
	apperrors "hostsent/backend/internal/pkg/errors"
)

const claimsContextKey = "claims"

func Auth(requiredToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		parts := strings.SplitN(authorization, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] != requiredToken {
			err := apperrors.New(10001, "unauthorized")
			c.JSON(http.StatusUnauthorized, gin.H{"code": err.Code, "message": err.Message, "timestamp": timestamp()})
			c.Abort()
			return
		}

		c.Set(claimsContextKey, appauth.BuildMockClaims())
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
