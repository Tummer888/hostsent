// Package cache 提供 Redis 的最小封装与进程内降级实现。
//
// 设计（doc89 §3 / doc91 §2）：
//   - Redis 是「加速器」而非「依赖」：连不通时整体降级为进程内 LRU + 业务侧 DB 兜底，
//     绝不让验证码/限流把登录、下单等主链路拖死。
//   - 未启用时 Get 返回 ("", false)，Incr 返回 (0, ErrDisabled)，业务侧据此走降级分支。
//   - 所有降级分叉收敛在本包，业务代码不得出现 `if redisEnabled` 判断，
//     以便 C10 能做一次「停掉 Redis 跑全套场景」的演练（doc91 §2.4 实现约束）。
package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"hostsent/backend/internal/pkg/config"
)

// ErrDisabled Redis 未启用（连接失败或未配置）时返回，调用方据此走 DB 降级路径。
var ErrDisabled = errors.New("cache: redis disabled")

// ErrNotAcquired 分布式锁未获取到。
var ErrNotAcquired = errors.New("cache: lock not acquired")

// releaseScript 释放锁：仅当值匹配（自己的锁）才删除，避免释放他人的锁。
var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
end
return 0
`)

// Client Redis 最小封装；rdb 为 nil 表示未启用（降级）。
type Client struct {
	rdb    redis.UniversalClient
	logger *zap.Logger
	local  *localStore
}

// New 构造缓存客户端。
//
// required=true 时连接失败返回错误（生产强制），否则返回降级客户端并打 Warn（不阻断启动）。
func New(cfg config.RedisConfig, required bool, logger *zap.Logger) (*Client, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	c := &Client{logger: logger, local: newLocalStore(4096)}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if cfg.Host == "" {
		if required {
			return nil, errors.New("cache: redis host not configured but required")
		}
		logger.Warn("cache: redis host not configured, running in degraded mode")
		return c, nil
	}

	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{addr},
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		if required {
			return nil, fmt.Errorf("cache: redis required but unavailable: %w", err)
		}
		logger.Warn("cache: redis unavailable, running in degraded mode",
			zap.String("addr", addr), zap.Error(err))
		return c, nil
	}
	c.rdb = rdb
	return c, nil
}

// NewDisabled 显式构造降级客户端（测试与「强制无 Redis」演练用）。
func NewDisabled(logger *zap.Logger) *Client {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Client{logger: logger, local: newLocalStore(4096)}
}

// Enabled 是否启用 Redis。
func (c *Client) Enabled() bool { return c != nil && c.rdb != nil }

// Ping 探活；未启用时返回 ErrDisabled。
func (c *Client) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	return c.rdb.Ping(ctx).Err()
}

// Set 写入键值，ttl<=0 表示不过期。
func (c *Client) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	if !c.Enabled() {
		c.local.set(key, toStr(val), ttl)
		return nil
	}
	return c.rdb.Set(ctx, key, toStr(val), ttl).Err()
}

// SetNX 仅当 key 不存在时写入，返回是否写入成功。
func (c *Client) SetNX(ctx context.Context, key string, val any, ttl time.Duration) (bool, error) {
	if !c.Enabled() {
		return c.local.setNX(key, toStr(val), ttl), nil
	}
	return c.rdb.SetNX(ctx, key, toStr(val), ttl).Result()
}

// Get 读取键值；不存在或未启用时返回 ("", false)。
func (c *Client) Get(ctx context.Context, key string) (string, bool) {
	if !c.Enabled() {
		return c.local.get(key)
	}
	v, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

// GetDel 读取并删除（验证码一次性校验的核心）；不存在或未启用时返回 ("", false)。
func (c *Client) GetDel(ctx context.Context, key string) (string, bool) {
	if !c.Enabled() {
		v, ok := c.local.get(key)
		if ok {
			c.local.del(key)
		}
		return v, ok
	}
	v, err := c.rdb.GetDel(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

// Incr 自增计数器（首次写入时设置 ttl）；未启用时走进程内计数。
func (c *Client) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if !c.Enabled() {
		return c.local.incr(key, ttl), nil
	}
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// Del 删除若干键。
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if !c.Enabled() {
		c.local.del(keys...)
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// Expire 设置过期时间。
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// TTL 查询剩余有效期；-1 表示无过期、-2 表示不存在。
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	if !c.Enabled() {
		return c.local.ttl(key), nil
	}
	return c.rdb.TTL(ctx, key).Result()
}

// Lock 尝试获取分布式锁。返回 release 释放函数与是否获取成功。
//
// Redis 不可用时降级为进程内 SETNX（单实例有效）；跨实例互斥由业务侧的
// DB 部分唯一索引兜底（doc92 §6.5）。
func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (func(), bool) {
	token := randomToken()
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if !c.Enabled() {
		if !c.local.setNX(key, token, ttl) {
			return func() {}, false
		}
		return func() { c.local.del(key) }, true
	}
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil || !ok {
		return func() {}, false
	}
	return func() {
		rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = releaseScript.Run(rctx, c.rdb, []string{key}, token).Err()
	}, true
}

// WarnDegraded 降级期告警（同一 scene 每分钟至多一条，避免刷屏；doc91 §2.5）。
var (
	degradedMu   sync.Mutex
	degradedSeen = map[string]time.Time{}
)

// WarnDegraded 在 Redis 不可用导致放行/降级时打一次 Warn，供事后取证。
func (c *Client) WarnDegraded(scene, path string) {
	if c == nil || c.Enabled() {
		return
	}
	degradedMu.Lock()
	last, ok := degradedSeen[scene]
	now := time.Now()
	if ok && now.Sub(last) < time.Minute {
		degradedMu.Unlock()
		return
	}
	degradedSeen[scene] = now
	degradedMu.Unlock()
	if c.logger != nil {
		c.logger.Warn("cache degraded: verification bypassed",
			zap.String("scene", scene), zap.String("path", path))
	}
}

// toStr 归一化键值（统一以字符串存储，便于跨语言排查）。
func toStr(val any) string {
	switch v := val.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case time.Duration:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
