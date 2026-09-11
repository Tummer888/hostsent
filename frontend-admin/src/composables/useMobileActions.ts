import type { PrimaryTableCol } from 'tdesign-vue-next'

/**
 * 移动端操作列辅助：把「行操作定义（content/value/hidden/disabled）」映射为
 * TDesign Dropdown 的 options。桌面端仍渲染 t-link 列表，移动端用
 * <MobileAction>（省略号按钮 + 下拉）收纳全部操作。
 */
export interface MobileActionItem {
  content: string
  value: string
  /** 返回 false 时该项不出现（如 v-if 条件） */
  hidden?: () => boolean
  disabled?: () => boolean
  theme?: 'default' | 'success' | 'warning' | 'error'
}

export function buildMobileActionOptions(
  items: Array<MobileActionItem | null | undefined>,
): Array<{ content: string; value: string; disabled?: boolean; theme?: 'default' | 'success' | 'warning' | 'error' }> {
  return items
    .filter((item): item is MobileActionItem => !!item && item.hidden?.() !== true)
    .map((item) => ({
      content: item.content,
      value: item.value,
      disabled: item.disabled?.() ?? false,
      theme: item.theme,
    }))
}

/** 列定义用：移动端窄列宽 */
export function actionColumnWidth(isMobile: boolean, desktop = 260): number {
  return isMobile ? 70 : desktop
}

export type { PrimaryTableCol }
