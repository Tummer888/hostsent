<template>
  <div class="content-shell">
    <div class="site-container">
      <ContentArticle
        :title="announcement!.title"
        :body="announcement!.content"
        :format="announcement!.isHtml ? 'html' : 'text'"
        back-to="/announcements"
        back-label="返回公告列表"
      >
        <template #meta>
          <span
            class="announce-row__tag"
            :class="`is-${announcementLevelMeta(announcement!.level).tone}`"
          >
            {{ announcementLevelMeta(announcement!.level).label }}
          </span>
          <span v-if="announcement!.pinned" class="announce-row__pin">置顶</span>
          <time>{{ formatAnnouncementDate(announcement!.publishedAt) }}</time>
        </template>
      </ContentArticle>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  announcementLevelMeta,
  formatAnnouncementDate,
} from '#shared/schemas/announcement'

const route = useRoute()
// 404/503 由组合式函数直接抛出（error.vue 呈现），模板里不再需要空态分支。
const { announcement } = await useAnnouncementDetail(() => String(route.params.id ?? ''))

useSeoMeta({
  title: () => announcement.value?.title ?? '服务公告',
  description: () => announcement.value?.title ?? '',
  ogType: 'article',
})
</script>
