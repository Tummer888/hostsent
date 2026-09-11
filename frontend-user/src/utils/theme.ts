// 主题工具：主题色色阶生成与 CSS 变量注入（与 frontend-admin/src/utils/theme.ts 同构，
// 用户端默认主题色为蓝色系，见 styles/index.css 的 :root token）。

export interface ThemePreset {
  name: string
  value: string
}

// 预设主题色（首个为用户端默认蓝）
export const PRESET_THEME_COLORS: ThemePreset[] = [
  { name: '默认蓝', value: '#2563eb' },
  { name: '靛蓝', value: '#4f46e5' },
  { name: '青', value: '#0891b2' },
  { name: '绿', value: '#16a34a' },
  { name: '金', value: '#ca8a04' },
  { name: '橙', value: '#ea580c' },
  { name: '品红', value: '#db2777' },
  { name: '紫', value: '#7c3aed' },
]

export const DEFAULT_THEME_COLOR = PRESET_THEME_COLORS[0].value

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

// 以主题色为 7 号生成 1~10 品牌色阶（梯度对齐 TDesign 品牌色用法）
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

function setVar(name: string, value: string) {
  document.documentElement.style.setProperty(name, value)
}

// 应用主题色：项目 token + TDesign 品牌变量一次性注入
export function applyThemeColor(hexInput: string) {
  const hex = normalizeHex(hexInput)
  const scale = buildBrandScale(hex)
  setVar('--color-primary', hex)
  setVar('--color-primary-hover', scale[7])
  setVar('--color-primary-light', scale[0])
  setVar('--color-secondary', hex)
  setVar('--color-accent', hex)
  setVar('--color-ring', hex)
  scale.forEach((c, idx) => setVar(`--td-brand-color-${idx + 1}`, c))
  setVar('--td-brand-color', hex)
  setVar('--td-brand-color-hover', scale[7])
  setVar('--td-brand-color-active', scale[8])
  setVar('--td-brand-color-focus', scale[2])
  setVar('--td-brand-color-disabled', scale[3])
  setVar('--td-brand-color-light', scale[0])
  setVar('--td-brand-color-light-hover', scale[1])
  setVar('--td-brand-color-light-active', scale[2])
}

// 应用圆角档位：scalePercent 为百分比（100 = 默认 6/8/12/16px，对应用户端 styles/index.css）
export function applyRadius(scalePercent: number) {
  const f = Math.min(200, Math.max(50, scalePercent || 100)) / 100
  setVar('--radius-sm', `${Math.round(6 * f)}px`)
  setVar('--radius-md', `${Math.round(8 * f)}px`)
  setVar('--radius-lg', `${Math.round(12 * f)}px`)
  setVar('--radius-xl', `${Math.round(16 * f)}px`)
}
