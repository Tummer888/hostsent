import { EMPTY_ANNOUNCEMENT_LIST, type Announcement } from '#shared/schemas/announcement'

/**
 * 公告数据获取（规范 R2/R3：走 BFF，且必须有默认值兜底）。
 * 后端不可用时返回空数组，首页公告区块整体隐藏而不是展示空壳。
 */
export function useAnnouncements(limit = 5) {
  const { data, pending, error } = useAsyncData<Announcement[]>(
    `announcements:${limit}`,
    () => $fetch<Announcement[]>(`/api/announcements?limit=${limit}`),
    { default: () => EMPTY_ANNOUNCEMENT_LIST },
  )

  return {
    announcements: computed<Announcement[]>(() => data.value ?? EMPTY_ANNOUNCEMENT_LIST),
    pending,
    error,
  }
}
