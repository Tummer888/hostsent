<template>
  <div class="content-page">
    <section class="content-hero">
      <div class="site-container">
        <nav class="content-crumb" aria-label="面包屑">
          <NuxtLink to="/">首页</NuxtLink>
          <span>/</span>
          <span class="content-crumb__current">服务公告</span>
        </nav>
        <h1 class="content-hero__title">服务公告</h1>
        <p class="content-hero__desc">
          {{ site.name }} 的产品发布、维护窗口与故障说明。重要公告会置顶显示。
        </p>
      </div>
    </section>

    <section class="content-shell">
      <div class="site-container">
        <ul v-if="announcements.length" class="announce-list">
          <li v-for="item in announcements" :key="item.id" class="announce-row">
            <span v-if="item.pinned" class="announce-row__pin">置顶</span>
            <span class="announce-row__tag" :class="`is-${announcementLevelMeta(item.level).tone}`">
              {{ announcementLevelMeta(item.level).label }}
            </span>
            <div class="announce-row__body">
              <h2 class="announce-row__title">
                <NuxtLink :to="`/announcements/${item.id}`">{{ item.title }}</NuxtLink>
              </h2>
              <p v-if="excerptOf(item)" class="announce-row__excerpt">{{ excerptOf(item) }}</p>
            </div>
            <time class="announce-row__date">{{ formatAnnouncementDate(item.publishedAt) }}</time>
          </li>
        </ul>

        <div v-else class="content-empty">
          <p class="content-empty__title">暂无公告</p>
          <p class="content-empty__desc">目前没有需要通知大家的事项，一切运行正常。</p>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import {
  announcementExcerpt,
  announcementLevelMeta,
  formatAnnouncementDate,
  type Announcement,
} from '#shared/schemas/announcement'

/**
 * 公开公告接口只提供 limit（无分页），一次取到上限 50 条足够覆盖列表页；
 * 真有超过 50 条公告时更该做的是归档页，而不是在这一页无限翻页。
 */
const ANNOUNCEMENT_FETCH_LIMIT = 50

const { content } = useSiteContent()
const site = computed(() => content.value.site)
const { announcements } = useAnnouncements(ANNOUNCEMENT_FETCH_LIMIT)

/**
 * 富文本公告的摘要素材：`announcementExcerpt` 会剥掉 HTML 标签，
 * 因此 content 是 html 还是 text 都能得到可读文字。
 */
function excerptOf(item: Announcement): string {
  return announcementExcerpt(item.content, 100)
}

useSeoMeta({
  title: '服务公告',
  description: () => `${site.value.name} 服务公告：产品发布、维护窗口与故障说明。`,
  ogType: 'website',
})
</script>
