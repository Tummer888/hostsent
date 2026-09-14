import { isArticleKind } from '#shared/schemas/content'
import { fetchSingletonArticle } from '../../utils/content'

/**
 * 单篇文档 BFF 代理（用户条款 / 隐私政策）。
 *
 * 与详情接口的差别：内容尚未配置时**不返回 404**，而是 200 + `{ article: null }`。
 * 页面本身存在（注册页会链接过来），只是运营还没填正文；
 * 报「页面不存在」会让用户以为链接坏了，展示「内容尚未准备好」才符合事实。
 * 后端网络异常仍返回 503，与其它接口一致。
 *
 * 响应恒为对象而不是裸 `null`：h3 把 `null` 当「无内容」，会回 204 空响应，
 * 前端 `$fetch` 拿到 undefined，与「尚未发布」这个业务语义就没法区分了。
 */
export default defineEventHandler(async (event) => {
  const kind = String(getRouterParam(event, 'kind') ?? '').trim()
  if (!isArticleKind(kind) || (kind !== 'terms' && kind !== 'privacy')) {
    throw createError({ statusCode: 404, statusMessage: 'Document not found' })
  }

  let article: Awaited<ReturnType<typeof fetchSingletonArticle>>
  try {
    article = await fetchSingletonArticle(event, kind)
  } catch (error) {
    console.warn(`[content] 读取单篇文档失败 kind=${kind}:`, error)
    throw createError({ statusCode: 503, statusMessage: 'Content service unavailable' })
  }

  setHeader(event, 'cache-control', 'public, s-maxage=600, stale-while-revalidate=1800')
  return { article }
})
