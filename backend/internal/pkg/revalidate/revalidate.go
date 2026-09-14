// Package revalidate 向官网门户（frontend-site，Nuxt/Nitro）发起主动缓存失效。
//
// 存在的原因：门户的公开页面走 SWR（品牌 60s、公告/新闻 300s、帮助 600s），运营在
// 后台改完站点名称或发布公告后，最长要等一个 TTL 才在前台可见。doc80 §10.1 与
// doc100 §9.1 都承诺「保存/发布时主动 purge」，但此前只有声明没有实现 ——
// 门户 nuxt.config.ts 里那个 `internalToken` 没有任何消费方。
//
// 用包级单例（与 internal/pkg/jobrun、internal/pkg/upstream.Recorder 同款做法）而不是
// 逐层注入：内容服务的构造函数已经有一串依赖，为一个「可选的旁路通知」再加一个参数，
// 会让所有测试夹具都要改。未注入时 Notify 是零开销的空操作。
//
// 失败绝不影响业务写入：缓存没清掉最坏是「晚一个 TTL 才可见」，而让一次公告发布
// 因为门户进程没起而报错，是把可降级问题升级成不可用。
package revalidate

import (
	"context"
	"sync/atomic"
)

// 失效键。门户侧当前按「有内容变更就整体清空 Nitro 缓存」处理（见
// frontend-site/server/api/internal/revalidate.post.ts），键仅用于日志与
// 将来做细粒度清理时的区分，不参与鉴权。
const (
	KeySiteContent  = "site-content"
	KeyAnnouncement = "announcements"
	KeyArticle      = "articles"
	KeyFriendlyLink = "friendly-links"
)

// Notifier 主动失效通知器。
type Notifier interface {
	// Notify 通知门户清理 keys 对应的缓存；实现必须自行吞掉错误并记日志。
	Notify(ctx context.Context, keys ...string)
}

var global atomic.Value // Notifier

// SetNotifier 注入通知器（装配层调用一次）。
func SetNotifier(n Notifier) { global.Store(n) }

// Notify 向门户发起失效通知；未注入通知器时为空操作。
func Notify(ctx context.Context, keys ...string) {
	v := global.Load()
	if v == nil {
		return
	}
	n, _ := v.(Notifier)
	if n == nil {
		return
	}
	n.Notify(ctx, keys...)
}
