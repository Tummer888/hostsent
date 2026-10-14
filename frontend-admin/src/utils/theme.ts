// 主题工具：主题色色阶生成与 CSS 变量注入（主题设置中心 P1，见 docs/实施计划 00 主题方案）
// 设计：--color-primary 为主色单一来源；--td-brand-color-1~10 为 TDesign 品牌梯度；
// 默认值定义在 styles/index.css 的 :root，本模块在运行时以同名变量内联覆盖 documentElement。
//
// 配色基调：主色为石墨灰（黑），强调色固定为橙；绿色自本版起只作为
// 「成功/运行中」等语义色存在，不再承担品牌色。

export interface ThemePreset {
  name: string
  value: string
}

// 预设主题色（首个为系统默认：石墨灰；橙为强调色）
export const PRESET_THEME_COLORS: ThemePreset[] = [
  { name: '石墨灰', value: '#2b2f36' },
  { name: '橙', value: '#f97316' },
  { name: '蓝', value: '#2981ff' },
  { name: '靛蓝', value: '#4f46e5' },
  { name: '紫', value: '#7c3aed' },
  { name: '品红', value: '#db2777' },
  { name: '青', value: '#0891b2' },
  { name: '金', value: '#ca8a04' },
]

export const DEFAULT_THEME_COLOR = PRESET_THEME_COLORS[0].value

// 强调色（橙）：不随主题色变化，供 CTA/高亮场景固定使用。
export const ACCENT_COLOR = '#f97316'

// 预设换色后的存量迁移：老用户持久化里存的是旧值，读出来时映射到新预设，
// 否则「蓝」的色板高亮态会失配、已选用户停留在旧色。
// 绿色（#16a34a 及其近邻）曾是默认主色，现改为石墨灰：不迁移的话，
// 升级后这些用户整站仍停在绿色，与新设计语言冲突。
export const LEGACY_THEME_COLOR_MAP: Record<string, string> = {
  '#2563eb': '#2981ff',
  '#16a34a': DEFAULT_THEME_COLOR,
  '#22c55e': DEFAULT_THEME_COLOR,
  '#15803d': DEFAULT_THEME_COLOR,
  '#166534': DEFAULT_THEME_COLOR,
  '#4ade80': DEFAULT_THEME_COLOR,
}

export function migrateThemeColor(hex: string): string {
  return LEGACY_THEME_COLOR_MAP[hex.toLowerCase()] ?? hex
}

// 归一化 hex：支持 #abc / #aabbcc；非法时返回 fallback
export function normalizeHex(input: string, fallback = DEFAULT_THEME_COLOR): string {
  let hex = (input || '').trim().replace(/^#/, '')
  if (/^[0-9a-fA-F]{3}$/.test(hex)) {
    hex = hex
      .split('')
      .map((c) => c + c)
      .join('')
  }
  if (!/^[0-9a-fA-F]{6}$/.test(hex)) return fallback
  return `#${hex.toLowerCase()}`
}

// 颜色混合：ratio 为 other 的权重（0~1）
function mix(hex: string, other: string, ratio: number): string {
  const a = parseInt(hex.slice(1), 16)
  const b = parseInt(other.slice(1), 16)
  const ch = (shift: number) => {
    const x = (a >> shift) & 0xff
    const y = (b >> shift) & 0xff
    return Math.round(x + (y - x) * ratio)
  }
  const to2 = (n: number) => n.toString(16).padStart(2, '0')
  return `#${to2(ch(16))}${to2(ch(8))}${to2(ch(0))}`
}

const mixWhite = (hex: string, r: number) => mix(hex, '#ffffff', r)
const mixBlack = (hex: string, r: number) => mix(hex, '#000000', r)

// 相对亮度（WCAG 2.x），用于判断某色在纯黑/纯白底上是否还看得见
function luminance(hex: string): number {
  const channel = (v: number) => {
    const c = v / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  }
  const r = channel(parseInt(hex.slice(1, 3), 16))
  const g = channel(parseInt(hex.slice(3, 5), 16))
  const b = channel(parseInt(hex.slice(5, 7), 16))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

// 以主题色为 7 号生成 1~10 品牌色阶（梯度对齐 TDesign 亮色主题用法：1 最浅 → 10 最深）
export function buildBrandScale(hex: string): string[] {
  return [
    mixWhite(hex, 0.94), // 1 最浅
    mixWhite(hex, 0.86), // 2
    mixWhite(hex, 0.72), // 3
    mixWhite(hex, 0.55), // 4
    mixWhite(hex, 0.36), // 5
    mixWhite(hex, 0.14), // 6
    hex, // 7 主色
    mixBlack(hex, 0.12), // 8 hover
    mixBlack(hex, 0.28), // 9 active
    mixBlack(hex, 0.44), // 10 最深
  ]
}

// 深色主题：主色在纯黑底上亮度不足时逐步提亮。
// 石墨灰（#2b2f36）本是为白底设计的，直接放到黑底上，实心主按钮、
// 链接与选中态会糊成一片；提到 0.18 亮度（约 #787b82）后可读性达标。
export function liftForDark(hex: string): string {
  let out = hex
  for (let i = 0; i < 12 && luminance(out) < 0.18; i++) out = mixWhite(out, 0.1)
  return out
}

// 深色主题色阶：方向与亮色相反（1 最深 → 10 最亮，对齐 TDesign 深色主题用法：
// 主色取 8 号、hover 取 7 号）。亮色色阶的 1 号是近白，直接沿用到深色会把
// 选中行、浅色按钮等「品牌浅底」刷成刺眼的白块。
export function buildDarkBrandScale(hex: string): string[] {
  return [
    mixBlack(hex, 0.8), // 1 最深（深色下的品牌浅底）
    mixBlack(hex, 0.7), // 2
    mixBlack(hex, 0.6), // 3
    mixBlack(hex, 0.48), // 4
    mixBlack(hex, 0.36), // 5
    mixBlack(hex, 0.24), // 6
    mixBlack(hex, 0.14), // 7 hover
    hex, // 8 主色
    mixWhite(hex, 0.14), // 9 active
    mixWhite(hex, 0.28), // 10 最亮
  ]
}

function setVar(name: string, value: string) {
  document.documentElement.style.setProperty(name, value)
}

function rgbTriple(hex: string): string {
  return `${parseInt(hex.slice(1, 3), 16)}, ${parseInt(hex.slice(3, 5), 16)}, ${parseInt(hex.slice(5, 7), 16)}`
}

// 应用主题色：项目 token + TDesign 品牌变量一次性注入。
// isDark 决定色阶方向与是否提亮（深浅色切换后必须重跑，否则深色下主色过暗）。
export function applyThemeColor(hexInput: string, isDark = false) {
  const raw = normalizeHex(hexInput)
  const hex = isDark ? liftForDark(raw) : raw
  const scale = isDark ? buildDarkBrandScale(hex) : buildBrandScale(hex)
  setVar('--color-primary', hex)
  setVar('--color-accent', ACCENT_COLOR)
  setVar('--color-ring', hex)
  // RGB 三元组：CSS 里用 rgba(var(--color-primary-rgb), α) 派生半透明品牌色
  // （表头底色、行悬停、聚焦光圈），切主题时随之联动。
  setVar('--color-primary-rgb', rgbTriple(hex))
  setVar('--color-accent-rgb', rgbTriple(ACCENT_COLOR))
  scale.forEach((c, idx) => setVar(`--td-brand-color-${idx + 1}`, c))
  setVar('--td-brand-color', hex)
  if (isDark) {
    // TDesign 深色约定：hover/active 比主色更亮，浅底取最暗的 1 号
    setVar('--td-brand-color-hover', scale[6])
    setVar('--td-brand-color-active', scale[8])
    setVar('--td-brand-color-focus', scale[1])
    setVar('--td-brand-color-disabled', scale[2])
  } else {
    setVar('--td-brand-color-hover', scale[7])
    setVar('--td-brand-color-active', scale[8])
    setVar('--td-brand-color-focus', scale[2])
    setVar('--td-brand-color-disabled', scale[3])
  }
  setVar('--td-brand-color-light', scale[0])
  setVar('--td-brand-color-light-hover', scale[1])
  setVar('--td-brand-color-light-active', scale[2])
}

// 应用圆角档位：scalePercent 为百分比（100 = 默认 4/6/8/10px）
export function applyRadius(scalePercent: number) {
  const f = Math.min(200, Math.max(50, scalePercent || 100)) / 100
  setVar('--hs-radius-sm', `${Math.round(4 * f)}px`)
  setVar('--hs-radius-md', `${Math.round(6 * f)}px`)
  setVar('--hs-radius-lg', `${Math.round(8 * f)}px`)
  setVar('--hs-radius-xl', `${Math.round(10 * f)}px`)
}
