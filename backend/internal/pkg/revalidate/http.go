package revalidate

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// HTTPNotifier 通过门户的内部接口清理 Nitro 缓存。
//
// 为什么是「后端调用门户」而不是「后端自己清缓存」：被清理的是门户 Node 进程里的
// 内存缓存（Nitro SWR），Go 进程碰不到它，只能请求门户来清。因此这里的 BaseURL
// 指向门户服务（frontend-site），Token 与门户的 NUXT_INTERNAL_TOKEN 必须一致。
type HTTPNotifier struct {
	baseURL string // 门户基地址，如 http://frontend-site:3003；空值表示未配置，Notify 空转
	token   string // 内部接口密钥；门户侧未配置时接口返回 503
	client  *http.Client
	logger  *zap.Logger
}

// NewHTTPNotifier 创建通知器。baseURL 为空时返回的通知器所有调用都是空操作 ——
// 未部署门户的环境（纯后端联调）不该因此报错或刷日志。
func NewHTTPNotifier(baseURL, token string, logger *zap.Logger) *HTTPNotifier {
	return &HTTPNotifier{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		// 超时必须短：这是在业务写路径上发起的旁路请求，门户卡住不能拖慢后台保存。
		client: &http.Client{Timeout: 3 * time.Second},
		logger: logger,
	}
}

// Enabled 报告门户地址是否已配置。
func (n *HTTPNotifier) Enabled() bool {
	return n != nil && n.baseURL != ""
}

// Notify 请求门户清理缓存。失败只记日志，绝不向调用方返回错误。
func (n *HTTPNotifier) Notify(ctx context.Context, keys ...string) {
	if !n.Enabled() {
		return
	}
	body, err := json.Marshal(map[string]any{"keys": keys})
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.baseURL+"/api/internal/revalidate", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", n.token)

	resp, err := n.client.Do(req)
	if err != nil {
		n.logger.Warn("revalidate: 门户未响应，缓存将在 TTL 到期后自然刷新",
			zap.Strings("keys", keys), zap.Error(err))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// 401/503 是配置问题（密钥不一致 / 门户未配 token），值得 Warn 出来，
		// 否则运维只会看到「后台保存了但前台没变」，无从定位。
		n.logger.Warn("revalidate: 门户拒绝失效请求",
			zap.Strings("keys", keys), zap.Int("status", resp.StatusCode))
		return
	}
	n.logger.Info("revalidate: 门户缓存已失效", zap.Strings("keys", keys))
}
