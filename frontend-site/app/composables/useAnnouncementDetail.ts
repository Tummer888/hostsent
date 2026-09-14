import type { Announcement } from '#shared/schemas/announcement'

/**
 * 公告详情数据获取（门户 `/announcements/:id`）。
 *
 * BFF 已区分 404（公告不存在/已下线）与 503（服务故障），这里把状态码**原样抛出**：
 * 让 error.vue 呈现正确的语义，同时保证 HTTP 响应码本身也是 404/503 ——
 * 若只渲染一个「公告不存在」的页面而返回 200，就是 soft 404，搜索引擎会继续
 * 保留一条已经撤下的公告地址。
 */
export async function useAnnouncementDetail(id: MaybeRefOrGetter<string | number>) {
  const resolvedId = computed(() => String(toValue(id) ?? ''))

  const { data, error } = await useAsyncData<Announcement>(
    () => `announcement:${resolvedId.value}`,
    () => $fetch<Announcement>(`/api/announcements/${encodeURIComponent(resolvedId.value)}`),
    { watch: [resolvedId] },
  )

  throwIfFetchFailed(error.value)

  if (!data.value) {
    throw createError({ statusCode: 500, statusMessage: 'Announcement payload missing', fatal: true })
  }

  return { announcement: data as Ref<Announcement> }
}
