/**
 * 渠道链路（双链路）展示口径，供同步运维控制台各处共用。
 *
 * 为什么单独抽出来：控制台顶部切换、渠道列标签、下拉分组都要用同一套文案与配色，
 * 散落各处会出现「上游转售」与「上游渠道」两种叫法混着显示。
 */

/** 链路取值：'' 表示不过滤（全部）。 */
export type ChainKind = '' | 'upstream' | 'compute'

/** 链路筛选项（控制台顶部切换用）。 */
export const chainKindOptions: { label: string; value: ChainKind }[] = [
  { label: '全部链路', value: '' },
  { label: '上游转售', value: 'upstream' },
  { label: '自营平台', value: 'compute' },
]

/** 链路中文名（未知值原样返回，避免界面出现空白标签）。 */
export function chainKindLabel(kind?: string): string {
  if (kind === 'upstream') return '上游转售'
  if (kind === 'compute') return '自营平台'
  return kind || ''
}

/** 链路标签配色：上游转售=蓝（外部依赖），自营平台=绿（自有执行器）。 */
export function chainKindTheme(kind?: string): 'primary' | 'success' | 'default' {
  if (kind === 'upstream') return 'primary'
  if (kind === 'compute') return 'success'
  return 'default'
}

/** 归一化 URL / 入参里的链路值。 */
export function normalizeChainKind(raw: string | null | undefined): ChainKind {
  if (raw === 'upstream' || raw === 'compute') return raw
  return ''
}

/**
 * 该链路是否具备「上游目录与定价」语义。
 *
 * 自营平台是我方执行器：商品与价格由本地决定，不存在"上游改价再拉取"这回事。
 * 因此待确认调价等入站专属视图在自营链路下不渲染（接口保留，仅前端不展示）。
 */
export function hasUpstreamCatalog(kind: string): boolean {
  return kind !== 'compute'
}
