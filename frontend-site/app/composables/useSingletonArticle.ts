import type { ArticleDetail, ArticleKind } from '#shared/schemas/content'

/**
 * 「每类型一篇」文档获取（用户条款 / 隐私政策）。
 *
 * 三种情况，语义各不相同：
 *  - 后端返回 `{ article: {...} }` → 正常渲染；
 *  - 后端返回 `{ article: null }` → 内容尚未配置，页面展示「尚未发布」占位。
 *    这不是 404：页面本身存在（注册页会链接过来），只是运营还没填正文；
 *  - 后端故障 → BFF 抛 503，这里同样抛出交给 error.vue。
 *    不能也降级成「尚未发布」—— 那会把一次临时故障说成「运营没写」，
 *    而且响应的 HTTP 状态码会变成 200，搜索引擎会据此缓存一个空页面。
 */
export async function useSingletonArticle(kind: Extract<ArticleKind, 'terms' | 'privacy'>) {
  const { data, error } = await useAsyncData<ArticleDetail | null>(
    `singleton:${kind}`,
    async () => {
      const payload = await $fetch<{ article: ArticleDetail | null }>(
        `/api/article-singletons/${kind}`,
      )
      return payload?.article ?? null
    },
    { default: () => null },
  )

  throwIfFetchFailed(error.value)

  return { article: data }
}
