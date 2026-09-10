package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	adminmodel "hostsent/backend/internal/modules/admin/manager/model"
)

// AdminAuditWriter 管理端操作审计落库能力（由 manager 仓储实现，避免 middleware 直接持有 *gorm.DB）。
type AdminAuditWriter interface {
	CreateAuditLog(ctx context.Context, log *adminmodel.AdminAuditLog) error
}

// auditExcludedPaths 不记录的高噪/敏感路径（前缀匹配）。
var auditExcludedPaths = []string{
	"/api/v1/admin/auth/login",
	"/health",
	"/ready",
	"/metrics",
}

// sensitiveKeys payload 脱敏键（子串匹配，命中即替换为 ***）。
var sensitiveKeys = []string{"password", "passwd", "token", "secret", "captcha"}

// maxAuditPayload 审计 payload 上限（字符），超出截断，避免大 body 撑爆日志表。
const maxAuditPayload = 2000

// AdminAudit 记录管理员写操作（POST/PUT/PATCH/DELETE）到 admin_audit_logs（P2-06）。
// 必须在 AdminAuth 之后注册：先由鉴权填充 claims，本中间件在 c.Next() 返回后落库。
// 落库失败不影响主流程；GET 与白名单路径直接放行。
func AdminAudit(writer AdminAuditWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if writer == nil || !isAuditMethod(c.Request.Method) || isAuditExcluded(c.Request.URL.Path) {
			c.Next()
			return
		}

		// 读取并回填 body，保证后续 handler 仍能正常解析
		var payload []byte
		if c.Request.Body != nil {
			payload, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(payload))
		}

		c.Next()

		claims, ok := GetAdminClaims(c)
		if !ok || claims.AdminID == 0 {
			return // 未通过鉴权的请求不记录
		}
		resourceType, resourceID, action := parseAuditPath(c.Request.URL.Path, c.Request.Method)
		log := &adminmodel.AdminAuditLog{
			AdminID:       claims.AdminID,
			AdminName:     claims.Username,
			Action:        action,
			ResourceType:  resourceType,
			ResourceID:    resourceID,
			IP:            c.ClientIP(),
			UserAgent:     truncate(c.Request.UserAgent(), 255),
			Module:        resourceType,
			RequestMethod: c.Request.Method,
			RequestPath:   truncate(c.Request.URL.Path, 255),
			ResponseCode:  c.Writer.Status(),
			Detail:        buildDetail(payload),
		}
		// 审计写入失败不影响响应，仅静默忽略（由仓储侧日志记录）
		_ = writer.CreateAuditLog(c.Request.Context(), log)
	}
}

func isAuditMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func isAuditExcluded(path string) bool {
	for _, prefix := range auditExcludedPaths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// parseAuditPath 从路径提取资源类型、资源 ID 与动作。
// 例：/api/v1/admin/users/5/reset-password → (users, 5, reset-password)
//
//	/api/v1/admin/tickets/9 → (tickets, 9, update|delete)
func parseAuditPath(path, method string) (resourceType, resourceID, action string) {
	trimmed := strings.TrimPrefix(path, "/api/v1/admin/")
	segments := make([]string, 0, 4)
	for _, seg := range strings.Split(trimmed, "/") {
		if seg != "" {
			segments = append(segments, seg)
		}
	}
	if len(segments) == 0 {
		return "", "", methodAction(method)
	}
	resourceType = segments[0]
	action = methodAction(method)
	for _, seg := range segments[1:] {
		if _, err := strconv.ParseUint(seg, 10, 64); err == nil {
			resourceID = seg
			continue
		}
		action = seg // 末尾动词段优先作为动作（approve / reset-password / claim ...）
	}
	return resourceType, resourceID, action
}

func methodAction(method string) string {
	switch method {
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	}
	return strings.ToLower(method)
}

// buildDetail 生成脱敏后的 payload JSON（空 body 返回 nil）。
func buildDetail(payload []byte) *string {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		// 非 JSON（如表单）：直接字符串化并脱敏
		s := maskText(string(payload))
		return &s
	}
	for key := range data {
		lower := strings.ToLower(key)
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(lower, sensitive) {
				data[key] = "***"
				break
			}
		}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		s := maskText(string(payload))
		return &s
	}
	s := truncate(string(encoded), maxAuditPayload)
	return &s
}

// maskText 纯文本脱敏：对 "key=value" / "key":"value" 形式的敏感键做替换。
func maskText(text string) string {
	result := text
	for _, key := range sensitiveKeys {
		result = maskKeyValue(result, key)
	}
	return truncate(result, maxAuditPayload)
}

// maskKeyValue 粗略替换 key 后的值（支持 = 与 : 两种分隔）。
func maskKeyValue(text, key string) string {
	lower := strings.ToLower(text)
	idx := 0
	for {
		pos := strings.Index(lower[idx:], key)
		if pos < 0 {
			return text
		}
		start := idx + pos + len(key)
		// 跳过键名后的引号与分隔符
		for start < len(text) && (text[start] == '"' || text[start] == '=' || text[start] == ':' || text[start] == ' ') {
			start++
		}
		end := start
		for end < len(text) && text[end] != '&' && text[end] != '"' && text[end] != ',' && text[end] != ' ' {
			end++
		}
		if end > start {
			text = text[:start] + "***" + text[end:]
			lower = strings.ToLower(text)
		}
		idx = start + 3
		if idx >= len(text) {
			return text
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
