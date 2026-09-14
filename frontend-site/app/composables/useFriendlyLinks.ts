import { EMPTY_FRIENDLY_LINKS, type FriendlyLink } from '#shared/schemas/content'

/**
 * 友情链接数据获取（页脚用）。
 * 后端不可用时返回空数组，页脚「友情链接」栏目整体不渲染，不留空标题。
 */
export function useFriendlyLinks() {
  const { data } = useAsyncData<FriendlyLink[]>(
    'friendly-links',
    () => $fetch<FriendlyLink[]>('/api/friendly-links'),
    { default: () => EMPTY_FRIENDLY_LINKS },
  )

  return {
    links: computed<FriendlyLink[]>(() => data.value ?? EMPTY_FRIENDLY_LINKS),
  }
}
