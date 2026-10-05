// 会员等级图标目录。
//
// ⚠️ 与 frontend-admin/src/components/level-badge/icons.ts 必须保持一致：
// 管理员在后台挑的图标 key（user_levels.icon）要在这里解析得出同一个图标，
// 两边的兜底色阶也要一致，否则同一个等级在管理端与用户端会是两个颜色。
//
// 为什么存 key 而不存图片 URL：等级配色要随主题走（暗色模式、品牌色替换），
// 组件图标能直接继承当前前景色，写死的图片链接换主题就失效。
import {
  BookmarkFilledIcon,
  CastleFilledIcon,
  CertificateFilledIcon,
  EarthFilledIcon,
  Flag1FilledIcon,
  Flag2FilledIcon,
  Flag3FilledIcon,
  GiftFilledIcon,
  HeartFilledIcon,
  HighLevelFilledIcon,
  MoneyFilledIcon,
  PalaceFilledIcon,
  RocketFilledIcon,
  SecuredFilledIcon,
  Star1FilledIcon,
  StarFilledIcon,
  SunRisingFilledIcon,
  SunnyFilledIcon,
  ThumbUpFilledIcon,
  TowerFilledIcon,
  UserVipFilledIcon,
} from 'tdesign-icons-vue-next'
import type { Component } from 'vue'

const iconMap: Record<string, Component> = {
  'star-filled': StarFilledIcon,
  'star-1-filled': Star1FilledIcon,
  'bookmark-filled': BookmarkFilledIcon,
  'flag-1-filled': Flag1FilledIcon,
  'flag-2-filled': Flag2FilledIcon,
  'flag-3-filled': Flag3FilledIcon,
  'secured-filled': SecuredFilledIcon,
  'certificate-filled': CertificateFilledIcon,
  'thumb-up-filled': ThumbUpFilledIcon,
  'gift-filled': GiftFilledIcon,
  'money-filled': MoneyFilledIcon,
  'rocket-filled': RocketFilledIcon,
  'high-level-filled': HighLevelFilledIcon,
  'sunny-filled': SunnyFilledIcon,
  'sun-rising-filled': SunRisingFilledIcon,
  'earth-filled': EarthFilledIcon,
  'tower-filled': TowerFilledIcon,
  'castle-filled': CastleFilledIcon,
  'palace-filled': PalaceFilledIcon,
  'user-vip-filled': UserVipFilledIcon,
  'heart-filled': HeartFilledIcon,
}

/** 默认图标：等级未配图标时用星星。 */
export const defaultLevelIcon = StarFilledIcon

/** 按 key 取图标组件；目录外的 key 返回 undefined，由调用方回落到默认图标。 */
export function resolveLevelIcon(key?: string): Component | undefined {
  if (!key) return undefined
  return iconMap[key]
}

// 未配颜色时的兜底色阶：与 admin 侧同表，按权重分档。
const fallbackBands: { maxWeight: number; color: string }[] = [
  { maxWeight: 10, color: '#8C9AAF' },
  { maxWeight: 20, color: '#E6A23C' },
  { maxWeight: 30, color: '#3BA9C4' },
  { maxWeight: 40, color: '#7B61FF' },
  { maxWeight: 50, color: '#FF7A45' },
]

/** 取等级的展示色：运营配了合法十六进制色就用它，否则按权重兜底。 */
export function resolveLevelColor(color?: string, weight = 0): string {
  const normalized = (color || '').trim()
  if (/^#[0-9a-fA-F]{6}$/.test(normalized)) return normalized.toUpperCase()
  if (/^#[0-9a-fA-F]{3}$/.test(normalized)) {
    const [, r, g, b] = normalized
    return `#${r}${r}${g}${g}${b}${b}`.toUpperCase()
  }
  for (const band of fallbackBands) {
    if (weight <= band.maxWeight) return band.color
  }
  return '#E63946'
}
