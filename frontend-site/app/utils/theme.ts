import type { ThemeSection } from '#shared/schemas/siteContent'

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

/** 解析 #RRGGBB；非法输入返回 null。 */
function parseHex(hex: string): { r: number; g: number; b: number } | null {
  const match = /^#([0-9a-fA-F]{6})$/.exec(hex.trim())
  if (!match) return null
  const int = Number.parseInt(match[1] as string, 16)
  return { r: (int >> 16) & 255, g: (int >> 8) & 255, b: int & 255 }
}

function toHex(channel: number): string {
  return clamp(Math.round(channel), 0, 255).toString(16).padStart(2, '0')
}

/** 与目标色按比例混合，ratio 为目标色占比 0~1。 */
function mixHex(hex: string, target: string, ratio: number): string {
  const a = parseHex(hex)
  const b = parseHex(target)
  if (!a || !b) return hex
  const t = clamp(ratio, 0, 1)
  return `#${toHex(a.r + (b.r - a.r) * t)}${toHex(a.g + (b.g - a.g) * t)}${toHex(a.b + (b.b - a.b) * t)}`
}

function hexToRgba(hex: string, alpha: number): string {
  const rgb = parseHex(hex)
  if (!rgb) return `rgba(43, 92, 255, ${alpha})`
  return `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${clamp(alpha, 0, 1)})`
}

/** 主色兜底值，须与 shared/schemas/siteContent.ts 的 theme.primaryColor 默认值一致。 */
const DEFAULT_PRIMARY = '#2b5cff'

/**
 * 由主题配置生成 CSS 变量。
 * 挂在根节点上，后续所有组件只用变量、不写死颜色，换主题色即为改一个配置项。
 */
export function buildThemeVars(theme: ThemeSection): Record<string, string> {
  const primary = parseHex(theme.primaryColor) ? theme.primaryColor : DEFAULT_PRIMARY
  const rgb = parseHex(primary) ?? { r: 43, g: 92, b: 255 }
  const radius = Number.parseInt(theme.radius, 10)
  const baseRadius = Number.isFinite(radius) ? radius : 10

  return {
    '--site-primary': primary,
    '--site-primary-strong': mixHex(primary, '#000000', 0.16),
    // --site-art-* 的 rgba() 需要 RGB 通道三元组，无法从 --site-primary 反拆，故单独注入
    '--site-primary-rgb': `${rgb.r}, ${rgb.g}, ${rgb.b}`,
    '--site-primary-soft': hexToRgba(primary, 0.08),
    '--site-primary-border': hexToRgba(primary, 0.26),
    '--site-radius': `${baseRadius}px`,
    '--site-radius-sm': `${Math.max(2, baseRadius - 4)}px`,
    '--site-radius-lg': `${baseRadius + 4}px`,
  }
}
