import { timingSafeEqual } from 'node:crypto'
import type { H3Event } from 'h3'

/**
 * 门户内部缓存失效接口（doc80 §10.1 / doc100 Q10）。
 *
 * 调用方是后端（Go）：运营在管理端保存站点配置、发布/下线公告或内容后，
 * 后端回调本接口清理 Nitro 的 SWR 缓存，让「发布即见」不必等 TTL。
 * 反向回调而不是共享缓存的原因：被清理的是 Node 进程内存里的 SWR 条目，
 * Go 进程碰不到它。
 *
 * 鉴权：`X-Internal-Token` 必须与 `NUXT_INTERNAL_TOKEN` 一致。
 * 未配置 token 时**拒绝一切请求**（503）而不是放行 —— 这个接口能在全站范围内
 * 清缓存，是个「无凭据即拒绝」的端点，绝不能因为「部署时忘配了」就变成公开的。
 *
 * 清缓存粒度：当前按 keys 只做**整表清空**。理由是本门户的缓存条目数量级很小
 *（首页 + 产品/内容页若干），精确到 key 需要知道 Nitro 内部对 SWR 条目的命名规则
 *（`cache:nitro:routes:...`），那条规则不是公开契约、升级 Nuxt 可能变，
 * 依赖它反而更脆。keys 仅记录进日志，便于回查是谁触发的清理。
 *
 * 响应形状与后端统一信封对齐（HTTP 200 + code），这样后端 HTTPNotifier 只需判状态码，
 * 且运维用 curl 联调时两个方向的输出形状一致。
 */
export default defineEventHandler(async (event: H3Event) => {
  const { internalToken } = useRuntimeConfig(event)

  if (!internalToken) {
    // 未配置密钥 = 未启用该能力。回 503 让后端在日志里 Warn 出来（HTTPNotifier 会识别），
    // 而不是静默 200 让人以为成功了。
    setResponseStatus(event, 503)
    return { code: 50301, message: 'internal revalidate not configured', timestamp: Date.now() }
  }

  const presented = String(getHeader(event, 'x-internal-token') || '')
  if (!secretEqual(presented, String(internalToken))) {
    setResponseStatus(event, 401)
    return { code: 40101, message: 'invalid internal token', timestamp: Date.now() }
  }

  const body = await readBody<{ keys?: unknown }>(event).catch(() => null)
  const keys = Array.isArray(body?.keys) ? body.keys.map((k) => String(k)).slice(0, 20) : []

  let cleared = 0
  try {
    const storage = useStorage('cache')
    const all = await storage.getKeys()
    for (const key of all) {
      await storage.removeItem(key)
      cleared += 1
    }
  } catch (error) {
    // 清理失败如实回 500：后端会 Warn，运维看到「后台保存了但前台没变」时能对上这条日志。
    console.error('[revalidate] 清理 Nitro 缓存失败:', error)
    setResponseStatus(event, 500)
    return { code: 50001, message: 'failed to clear portal cache', timestamp: Date.now() }
  }

  console.info(`[revalidate] 已清理门户缓存 ${cleared} 条 keys=${keys.join(',') || '(none)'}`)
  return {
    code: 0,
    message: 'success',
    data: { cleared, keys },
    timestamp: Date.now(),
  }
})

/** 定长比较，避免按字符提前返回泄露 token 前缀。 */
function secretEqual(a: string, b: string): boolean {
  const bufA = Buffer.from(a)
  const bufB = Buffer.from(b)
  if (bufA.length !== bufB.length) return false
  return timingSafeEqual(bufA, bufB)
}
