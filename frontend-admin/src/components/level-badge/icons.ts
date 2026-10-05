// 等级图标目录（与 frontend-user/src/components/level-badge/icons.ts 保持一致）。
//
// 为什么存 key 而不是图片 URL：等级配色要随主题走（暗色模式、品牌色替换），
// 组件图标能直接继承当前前景色，而写死的图片链接换主题就失效。
//
// key 的命名与 tdesign-icons-vue-next 的组件名同源：`star-filled` → `StarFilledIcon`。
// 这里显式列成表而不是动态 import：一是打包器可以摇树，二是运营填了目录外的 key
// 时能明确回落到默认图标，而不是在运行时静默渲染空白。
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

export interface LevelIconOption {
  /** 落库的 icon 值（图标目录 key）。 */
  value: string
  /** 下拉里的中文名，便于运营按语义挑。 */
  label: string
  component: Component
}

/** 可选图标：按「由低到高」的语感排序，方便运营顺手挑一组递进的。 */
export const levelIconOptions: LevelIconOption[] = [
  { value: 'star-filled', label: '星星', component: StarFilledIcon },
  { value: 'star-1-filled', label: '星标', component: Star1FilledIcon },
  { value: 'bookmark-filled', label: '书签', component: BookmarkFilledIcon },
  { value: 'flag-1-filled', label: '旗帜一', component: Flag1FilledIcon },
  { value: 'flag-2-filled', label: '旗帜二', component: Flag2FilledIcon },
  { value: 'flag-3-filled', label: '旗帜三', component: Flag3FilledIcon },
  { value: 'secured-filled', label: '盾牌', component: SecuredFilledIcon },
  { value: 'certificate-filled', label: '证书', component: CertificateFilledIcon },
  { value: 'thumb-up-filled', label: '点赞', component: ThumbUpFilledIcon },
  { value: 'gift-filled', label: '礼盒', component: GiftFilledIcon },
  { value: 'money-filled', label: '金币', component: MoneyFilledIcon },
  { value: 'rocket-filled', label: '火箭', component: RocketFilledIcon },
  { value: 'high-level-filled', label: '高级', component: HighLevelFilledIcon },
  { value: 'sunny-filled', label: '太阳', component: SunnyFilledIcon },
  { value: 'sun-rising-filled', label: '旭日', component: SunRisingFilledIcon },
  { value: 'earth-filled', label: '地球', component: EarthFilledIcon },
  { value: 'tower-filled', label: '高塔', component: TowerFilledIcon },
  { value: 'castle-filled', label: '城堡', component: CastleFilledIcon },
  { value: 'palace-filled', label: '宫殿', component: PalaceFilledIcon },
  { value: 'user-vip-filled', label: 'VIP', component: UserVipFilledIcon },
  { value: 'heart-filled', label: '爱心', component: HeartFilledIcon },
]

const iconMap = new Map(levelIconOptions.map((item) => [item.value, item.component]))

/** 默认图标：等级未配图标时用星星，语义中性且不会让人误以为配过。 */
export const defaultLevelIcon = StarFilledIcon

/** 按 key 取图标组件；目录外的 key 返回 undefined，由调用方回落到默认图标。 */
export function resolveLevelIcon(key?: string): Component | undefined {
  if (!key) return undefined
  return iconMap.get(key)
}

/** 预设色板：运营不想调色时直接点，覆盖常见的递进语义。 */
export const levelColorSwatches = [
  '#8C9AAF',
  '#E6A23C',
  '#3BA9C4',
  '#7B61FF',
  '#FF7A45',
  '#E63946',
  '#2BA471',
  '#0052D9',
  '#B37FEB',
  '#D4A017',
]

// 未配颜色时的兜底色阶：按权重分档，保证不同等级颜色不同。
// 权重是运营可改的，因此这里按「档位」而不是精确值匹配 —— 运营把白银从 10 调到 15
// 不该让它换一个颜色。
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
