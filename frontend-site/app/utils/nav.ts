/**
 * 同页锚点链接（`/#features` 这类）的 aria-current 修正。
 *
 * Vue Router 把 `/#features` 判定为命中 `/` 路由，于是 NuxtLink 会给它加上
 * `aria-current="page"`：读屏器会把导航里所有锚点都读成「当前页面」，
 * 而它们其实只是本页内部的段落锚点。传空串是 ARIA 规范里表示「非当前项」的合法值，
 * 普通页面链接仍用默认的 `page`。
 */
export function ariaCurrentFor(to: string): '' | 'page' {
  return to.startsWith('/#') ? '' : 'page'
}
