import type { ArticleDetail, ArticleKind } from '#shared/schemas/content'

/**
 * 文章详情数据获取。
 *
 * 详情接口区分 404（内容不存在/已下线）与 503（后端故障），这里**必须把状态码原样抛出**：
 * `useAsyncData` 默认只把错误放进 `error`，页面会带着 HTTP 200 渲染成「内容不存在」——
 * 这就是 soft 404：搜索引擎拿到的是一次成功响应，于是一边保留那条已被删除的 URL，
 * 一边在结果里展示空页面。抛出 `createError` 才会真正返回 404/503。
 *
 * 因此本函数是 async 的，调用处需要 `await`：只有等 fetch 落地才知道是哪种失败。
 */
export async function useArticleDetail(kind: ArticleKind, slug: MaybeRefOrGetter<string>) {
  const resolvedSlug = computed(() => toValue(slug))

  const { data, error } = await useAsyncData<ArticleDetail>(
    () => `article:${kind}:${resolvedSlug.value}`,
    () => $fetch<ArticleDetail>(`/api/articles/${kind}/${encodeURIComponent(resolvedSlug.value)}`),
    { watch: [resolvedSlug] },
  )

  throwIfFetchFailed(error.value)

  // 没报错却也没数据属于契约异常（BFF 只会返回对象或抛错）：按 500 处理，
  // 而不是让页面进入「内容不存在」分支 —— 那又会变成一次 soft 404。
  if (!data.value) {
    throw createError({ statusCode: 500, statusMessage: 'Content payload missing', fatal: true })
  }

  return { article: data as Ref<ArticleDetail> }
}

/**
 * 网络/HTTP 层失败时抛出对应状态码；无错误则什么都不做。
 *
 * 拿不到 statusCode 时按 500 处理：宁可报「服务出错」也不能回落成 404 ——
 * 把故障说成「内容不存在」会诱导搜索引擎删除仍然有效的页面。
 */
export function throwIfFetchFailed(error: unknown): void {
  if (!error) return
  const candidate = error as { statusCode?: number; statusMessage?: string; message?: string }
  const statusCode =
    typeof candidate.statusCode === 'number' && candidate.statusCode >= 400
      ? candidate.statusCode
      : 500
  throw createError({
    statusCode,
    statusMessage: candidate.statusMessage || candidate.message || 'Request failed',
    fatal: true,
  })
}
