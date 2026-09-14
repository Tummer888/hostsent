/**
 * 公告弹窗的「已关闭」去重记录。
 *
 * 公告的 `popup` 字段表示「发布后需要在用户中心弹一次」。这是**一次性提醒**，
 * 不是常驻状态：用户关掉之后不该每次进控制台又被同一个弹窗拦住。后端
 * `announcements` 表没有 per-user 的已读表（也不该为这件事建表 —— 公告是
 * 全员广播，逐用户落库既没必要也会随用户数×公告数膨胀），因此去重放在本地。
 *
 * 代价是换设备/清缓存后会再弹一次，对「重要通知别漏看」这个目标来说是
 * 可以接受的偏差 —— 宁可偶尔多提醒一次，也不要因为同步失败而永远不提醒。
 *
 * 与 `recent.ts` 同一套取舍：localStorage 不可写（隐私模式）时静默失败。
 */

const STORAGE_KEY = 'user_seen_announcement_popups'

/** 只保留最近 N 条 id，避免长期使用后键无限增长。 */
const MAX_ITEMS = 100

function read(): number[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((id): id is number => typeof id === 'number' && Number.isFinite(id))
  } catch {
    return []
  }
}

/** 该公告是否已被用户关闭过。 */
export function hasSeenAnnouncement(id: number): boolean {
  return read().includes(id)
}

/** 记录公告已关闭（可一次记多条：本轮一起弹出来的都算看过）。 */
export function markAnnouncementsSeen(ids: number[]): void {
  const valid = ids.filter((id) => typeof id === 'number' && Number.isFinite(id))
  if (!valid.length) return
  try {
    const merged = [...valid, ...read().filter((id) => !valid.includes(id))]
    localStorage.setItem(STORAGE_KEY, JSON.stringify(merged.slice(0, MAX_ITEMS)))
  } catch {
    // 隐私模式：下次仍会弹，可接受
  }
}
