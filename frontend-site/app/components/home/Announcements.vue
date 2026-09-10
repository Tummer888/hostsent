<template>
  <section v-if="visible.length" id="announcements" class="site-section site-section--tight announce">
    <div class="site-container">
      <SectionHeading :title="home.announceTitle" :subtitle="subtitle" />

      <ul class="announce__list">
        <li v-for="item in visible" :key="item.id" class="announce__item">
          <span class="announce__tag" :class="`is-${announcementLevelMeta(item.level).tone}`">
            {{ announcementLevelMeta(item.level).label }}
          </span>
          <div class="announce__body">
            <h3 class="announce__title">{{ item.title }}</h3>
            <p v-if="excerpt(item.content)" class="announce__excerpt">{{ excerpt(item.content) }}</p>
          </div>
          <time class="announce__date">{{ formatAnnouncementDate(item.publishedAt) }}</time>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  announcementExcerpt,
  announcementLevelMeta,
  formatAnnouncementDate,
} from '#shared/schemas/announcement'

const { content } = useSiteContent()
const home = computed(() => content.value.home)

/**
 * 同 FeaturedProducts：固定上限拉取 + 按配置响应式截断，
 * 避免 setup 阶段用默认值当请求参数导致后台配置不生效。
 */
const ANNOUNCEMENT_FETCH_LIMIT = 10
const { announcements } = useAnnouncements(ANNOUNCEMENT_FETCH_LIMIT)

const visible = computed(() => announcements.value.slice(0, home.value.announceLimit))

const subtitle = computed(() => `${content.value.site.name} 的产品与运维动态`)

const excerpt = (text: string) => announcementExcerpt(text, 90)
</script>

<style scoped>
.announce {
  background: #fff;
}

.announce__list {
  margin: 30px 0 0;
  padding: 0;
  list-style: none;
  border-top: 1px solid var(--site-border-soft);
}

.announce__item {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 20px 4px;
  border-bottom: 1px solid var(--site-border-soft);
  transition: background-color 0.18s ease;
}

.announce__item:hover {
  background: var(--site-bg-muted);
}

.announce__tag {
  flex-shrink: 0;
  margin-top: 2px;
  padding: 2px 9px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 1.7;
  background: var(--site-bg-muted);
  color: var(--site-text-muted);
}

.announce__tag.is-warning {
  background: rgba(217, 119, 6, 0.1);
  color: #b45309;
}

.announce__tag.is-critical {
  background: rgba(220, 38, 38, 0.1);
  color: #b91c1c;
}

.announce__body {
  flex: 1;
  min-width: 0;
}

.announce__title {
  font-size: 15px;
  font-weight: 500;
}

.announce__excerpt {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--site-text-muted);
}

.announce__date {
  flex-shrink: 0;
  font-size: 13px;
  color: var(--site-text-subtle);
}

@media (max-width: 640px) {
  .announce__list {
    margin-top: 28px;
  }

  .announce__item {
    flex-wrap: wrap;
    gap: 8px 10px;
  }

  .announce__date {
    width: 100%;
    padding-left: 0;
  }
}
</style>
