package netutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

type IPRegionResolver interface {
	Resolve(ctx context.Context, ip string) string
}

type ipAPIResponse struct {
	Status  string `json:"status"`
	Country string `json:"country"`
	Region  string `json:"regionName"`
	City    string `json:"city"`
}

type HTTPIPRegionResolver struct {
	client *http.Client
}

func NewHTTPIPRegionResolver() *HTTPIPRegionResolver {
	return &HTTPIPRegionResolver{
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

func (r *HTTPIPRegionResolver) Resolve(ctx context.Context, ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	if parsed.IsLoopback() || parsed.IsPrivate() {
		return "本地/内网"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN", ip), nil)
	if err != nil {
		return ""
	}
	response, err := r.client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	var payload ipAPIResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return ""
	}
	if payload.Status != "success" {
		return ""
	}
	parts := make([]string, 0, 3)
	if payload.Country != "" {
		parts = append(parts, payload.Country)
	}
	if payload.Region != "" {
		parts = append(parts, payload.Region)
	}
	if payload.City != "" {
		parts = append(parts, payload.City)
	}
	if len(parts) == 0 {
		return ""
	}
	return joinParts(parts)
}

func joinParts(parts []string) string {
	result := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if result == "" {
			result = part
			continue
		}
		result += " / " + part
	}
	return result
}
