// 商品描述文本化（用户侧安全渲染）
//
// 上游（魔方财务）商品描述是富文本 HTML（卖点图文），会随商品导入落到本地商品描述里。
// 用户侧不绑定 HTML（避免把上游内容当可信 HTML 直接渲染 —— 一旦上游被篡改就是 XSS），
// 而是先把标签剥掉、实体解码、空白归一，再按纯文本展示。
// 需要在后台预览富文本原文时用首页签里的富文本预览（仅内部可见）。

const BLOCK_TAGS = [
  'div',
  'p',
  'br',
  'li',
  'tr',
  'h1',
  'h2',
  'h3',
  'h4',
  'section',
  'article',
  'table',
]

const NAMED_ENTITIES: Record<string, string> = {
  amp: '&',
  lt: '<',
  gt: '>',
  quot: '"',
  apos: "'",
  nbsp: ' ',
  ldquo: '“',
  rdquo: '”',
  mdash: '—',
  hellip: '…',
  times: '×',
}

/** decodeEntities 解码命名/数字实体（未知实体原样保留，避免丢内容）。 */
function decodeEntities(input: string): string {
  return input.replace(/&(#x?[0-9a-fA-F]+|[a-zA-Z]+);/g, (match, body: string) => {
    if (body.startsWith('#x') || body.startsWith('#X')) {
      const code = Number.parseInt(body.slice(2), 16)
      return Number.isFinite(code) ? String.fromCodePoint(code) : match
    }
    if (body.startsWith('#')) {
      const code = Number.parseInt(body.slice(1), 10)
      return Number.isFinite(code) ? String.fromCodePoint(code) : match
    }
    return NAMED_ENTITIES[body.toLowerCase()] ?? match
  })
}

/**
 * htmlToPlainText 把富文本描述转成可读纯文本。
 *
 * 处理顺序很重要：上游下发的描述是**已转义**的 HTML（`&lt;div style=&quot;...&quot;&gt;`，
 * 上游页面自己会再做一次 htmlspecialchars_decode），所以要先解实体、再剥标签；
 * 顺序反了会把 `&lt;div&gt;` 变成可见的 `<div>` 文本（离线验证时踩过）。
 *
 * 步骤：解实体 → 丢弃 style/script → 标签换行/删除 → 再解一次实体（文本内的 &amp; 等）→ 压缩空行。
 * 传入空值返回空串（调用方据此决定是否展示）。
 */
export function htmlToPlainText(html?: string | null): string {
  if (!html) return ''
  let text = decodeEntities(String(html))
  // <br> 与块级标签边界 → 换行，保证"16核 / 16G / 50Mbps"这类分行卖点不糊成一团。
  const blockPattern = new RegExp(`</?(?:${BLOCK_TAGS.join('|')})\\b[^>]*>`, 'gi')
  text = text.replace(blockPattern, '\n')
  // <style>/<script> 连同内容一起丢弃（样式与脚本不是商品描述内容）。
  text = text.replace(/<(script|style)\b[^>]*>[\s\S]*?<\/\1>/gi, '')
  // 其余标签直接删除（属性里的内容也不保留，避免 style="..." 之类的噪音）。
  text = text.replace(/<[^>]*>/g, '')
  text = decodeEntities(text)
  const lines = text
    .split('\n')
    .map((line) => line.replace(/[ \t\u00a0]+/g, ' ').trim())
    .filter((line) => line !== '')
  return compactLines(lines).trim()
}

/** 短行合并阈值：上游把「值」「名称」放进相邻的 div（16核 / CPU），逐行展示会占掉六行。 */
const SHORT_LINE_LIMIT = 8

/**
 * compactLines 收敛上游排版：丢弃只由换行/缩进产生的空行，把连续的短行合并成一行
 * （空格连接），长行视为小标题、前面留一个空行。只改排版，不改文字内容。
 */
function compactLines(lines: string[]): string {
  const out: string[] = []
  let bucket: string[] = []
  const flushBucket = () => {
    if (bucket.length === 0) return
    if (out.length > 0) out.push('')
    out.push(bucket.join(' '))
    bucket = []
  }
  for (const line of lines) {
    if (line.length <= SHORT_LINE_LIMIT) {
      bucket.push(line)
      continue
    }
    flushBucket()
    if (out.length > 0) out.push('')
    out.push(line)
  }
  flushBucket()
  return out.join('\n')
}
