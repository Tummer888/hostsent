// Package sessionguard 校验「令牌绑定的会话是否仍然有效」。
//
// 存在意义：JWT 是无状态的，签发后无法撤回。安全页的「强制下线」、用户改密后的
// 「撤销全部会话」、登出，这些动作此前只改 user_sessions.status，而请求链路上
// 没有任何地方读这个状态位 —— 于是撤销后同一个令牌照样能用，doc91 §9 的
// 「强制下线后会话立即不可用」实际上从未生效（实测确认）。
//
// 现在的链路：令牌里带 sid（user_sessions.session_id）→ 中间件每次请求调本包
// 的 IsActive → 命中缓存直接放行，未命中查库 → 撤销时显式失效缓存。
//
// 为什么用「缓存 + 显式失效」而不是「每次请求查库」或「纯 TTL 缓存」：
//   - 每请求查库：user_sessions 会成为全站最热的表，而绝大多数请求的答案是不变的；
//   - 纯 TTL 缓存（哪怕只有 10 秒）：撤销后最多 10 秒内令牌仍可用，
//     「立即失效」就变成了「稍后失效」，而这恰恰是这次要修的东西。
//
// 显式失效在撤销侧完成：撤销路径本来就先把会话行读出来（拿得到 session_id），
// 顺手删掉对应缓存键即可，成本近乎为零。
package sessionguard

import (
	"context"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/pkg/cache"
)

// Store 会话有效性的数据源（由装配层用 user_sessions 表实现）。
//
// 只暴露一个方法：本包不关心会话长什么样，只关心「这个 session_id 现在还能不能用」。
type Store interface {
	// IsActive 判断会话是否有效：存在、status=active、且未过期。
	// 会话不存在时返回 (false, nil) 而不是错误 —— 「查无此会话」是正常结论，
	// 不是故障；把它当错误会让中间件无法区分「令牌是伪造的」和「库挂了」。
	IsActive(ctx context.Context, sessionID string) (bool, error)
}

// 缓存键前缀与 TTL。
const (
	keyPrefix = "auth:session:"

	// positiveTTL 有效会话的缓存时长。
	//
	// 取 5 分钟而不是「永久 + 撤销时删除」：撤销路径有若干条（管理员踢人、
	// 改密、注销、风控处置、登出），将来还可能有新的。任何一条漏调失效，
	// 会话就会永久有效 —— TTL 是兜底，让「漏失效」最坏退化成 5 分钟的延迟，
	// 而不是永久的口子。
	positiveTTL = 5 * time.Minute

	// negativeTTL 无效/不存在会话的缓存时长。
	//
	// 比 positiveTTL 短得多：这类结果主要来自伪造或过期的令牌，缓存是为了
	// 挡住「拿一个坏令牌反复打接口」的噪音；但也不能太长，否则用户重新登录后
	// 复用同一个 session_id 的场景（实际不会发生，session_id 每次都新生成）
	// 会被误判。1 分钟足够挡噪音。
	negativeTTL = time.Minute
)

// Guard 带缓存的会话有效性校验器。
//
// 零值/未装配（store 为 nil）时 IsActive 一律返回 false：会话校验是安全原语，
// 「校验不了」必须等于「不通过」，否则未装配就成了绕过口。
// 这与 pkg/cache 对图形码「降级即放行」的取舍刻意相反 —— 放行图形码丢的是
// 一次人机校验，放行会话校验丢的是「撤销」这个能力本身。
type Guard struct {
	store  Store
	cache  *cache.Client
	logger *zap.Logger
}

// New 创建校验器。store 为 nil 时构造出的是「一律拒绝」的守卫（见类型注释）。
func New(store Store, c *cache.Client, logger *zap.Logger) *Guard {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Guard{store: store, cache: c, logger: logger}
}

// IsActive 判断会话是否有效。
//
// 查库出错时返回错误（而不是 false）：调用方（中间件）据此返回 401，
// 与「会话确实无效」在行为上一致，但日志里能区分开，便于排查库故障。
func (g *Guard) IsActive(ctx context.Context, sessionID string) (bool, error) {
	if g == nil || g.store == nil || sessionID == "" {
		return false, nil
	}
	key := keyPrefix + sessionID
	if g.cache != nil {
		if v, ok := g.cache.Get(ctx, key); ok {
			// 只认明确写过的两个值，其它内容（人为污染/旧格式）当未命中处理。
			switch v {
			case "1":
				return true, nil
			case "0":
				return false, nil
			}
		}
	}

	active, err := g.store.IsActive(ctx, sessionID)
	if err != nil {
		// 不缓存错误结果：库抖动时缓存 "0" 会让刚登录的用户被拒一分钟。
		g.logger.Warn("session validity lookup failed",
			zap.String("session_id", sessionID), zap.Error(err))
		return false, err
	}

	if g.cache != nil {
		ttl := negativeTTL
		val := "0"
		if active {
			ttl, val = positiveTTL, "1"
		}
		if err := g.cache.Set(ctx, key, val, ttl); err != nil {
			// 缓存写失败不影响本次判定结果，只影响下次是否还要查库。
			g.logger.Warn("cache session validity failed",
				zap.String("session_id", sessionID), zap.Error(err))
		}
	}
	return active, nil
}

// Invalidate 使若干会话的缓存立即失效（撤销/登出后调用）。
//
// 失败只告警：缓存删不掉时，最坏结果是该会话在 positiveTTL 内仍被判为有效 ——
// 比让「撤销」这个动作整体失败要轻。调用方不应因为清理缓存失败而回滚撤销。
func (g *Guard) Invalidate(ctx context.Context, sessionIDs ...string) {
	if g == nil || g.cache == nil || len(sessionIDs) == 0 {
		return
	}
	keys := make([]string, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		if id != "" {
			keys = append(keys, keyPrefix+id)
		}
	}
	if len(keys) == 0 {
		return
	}
	if err := g.cache.Del(ctx, keys...); err != nil {
		g.logger.Warn("invalidate session cache failed",
			zap.Strings("session_ids", sessionIDs), zap.Error(err))
	}
}
