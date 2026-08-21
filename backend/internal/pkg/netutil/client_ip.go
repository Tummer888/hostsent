package netutil

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

func ClientIP(c *gin.Context) string {
	candidates := []string{
		c.GetHeader("CF-Connecting-IP"),
		c.GetHeader("X-Forwarded-For"),
		c.GetHeader("X-Real-IP"),
		c.ClientIP(),
	}
	for _, candidate := range candidates {
		if ip := firstIP(candidate); ip != "" {
			return ip
		}
	}
	return ""
}

func firstIP(raw string) string {
	for _, part := range strings.Split(raw, ",") {
		ip := strings.TrimSpace(part)
		if ip == "" {
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil {
			continue
		}
		return parsed.String()
	}
	return ""
}
