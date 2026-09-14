<template>
  <div class="content-page">
    <section class="content-hero">
      <div class="site-container">
        <nav class="content-crumb" aria-label="面包屑">
          <NuxtLink to="/">首页</NuxtLink>
          <span>/</span>
          <span class="content-crumb__current">新闻资讯</span>
        </nav>
        <h1 class="content-hero__title">新闻资讯</h1>
        <p class="content-hero__desc">
          {{ site.name }} 的产品动态、能力迭代与运维公告。
        </p>

        <div v-if="categories.length" class="content-filters">
          <NuxtLink
            class="content-filter"
            :class="{ 'is-active': !activeCategory }"
            :to="categoryLink('')"
          >
            全部
          </NuxtLink>
          <NuxtLink
            v-for="entry in flatCategories"
            :key="entry.node.id"
            class="content-filter"
            :class="{ 'is-active': activeCategory === entry.node.slug }"
            :to="categoryLink(entry.node.slug)"
          >
            {{ entry.node.name }}
          </NuxtLink>
        </div>
      </div>
    </section>

    <section class="content-shell">
      <div class="site-container">
        <p v-if="pending && !items.length" class="content-empty__desc">正在加载内容…</p>

        <ul v-else-if="items.length" class="news-list">
          <li v-for="item in items" :key="item.id" class="news-item">
            <div class="news-item__body">
              <div class="news-item__meta">
                <span v-if="item.pinned" class="news-item__badge">置顶</span>
                <span v-if="item.categoryName">{{ item.categoryName }}</span>
                <time>{{ formatArticleDate(item.publishAt) }}</time>
              </div>
              <h2 class="news-item__title">
                <NuxtLink :to="`/news/${item.slug}`">{{ item.title }}</NuxtLink>
              </h2>
              <p v-if="excerptOf(item)" class="news-item__excerpt">{{ excerptOf(item) }}</p>
              <div v-if="item.tags.length" class="news-item__tags">
                <span v-for="tag in item.tags" :key="tag" class="news-item__tag">{{ tag }}</span>
              </div>
            </div>
            <img
              v-if="item.cover"
              class="news-item__cover"
              :src="item.cover"
              :alt="item.title"
              loading="lazy"
            >
          </li>
        </ul>

        <div v-else class="content-empty">
          <p class="content-empty__title">
            {{ activeCategory ? '该分类下暂无内容' : '暂无新闻内容' }}
          </p>
          <p class="content-empty__desc">
            {{ activeCategory ? '换个分类看看，或浏览全部新闻。' : '内容正在准备中，敬请期待。' }}
          </p>
        </div>

        <nav v-if="totalPages > 1" class="content-pager" aria-label="分页">
          <button type="button" :disabled="currentPage <= 1" @click="goPage(currentPage - 1)">
            上一页
          </button>
          <span class="content-pager__info">第 {{ currentPage }} / {{ totalPages }} 页</span>
          <button
            type="button"
            :disabled="currentPage >= totalPages"
            @click="goPage(currentPage + 1)"
          >
            下一页
          </button>
        </nav>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import {
  articleExcerpt,
  flattenCategoryTree,
  formatArticleDate,
  type ArticleSummary,
} from '#shared/schemas/content'

const PAGE_SIZE = 10

const route = useRoute()
const router = useRouter()
const { content } = useSiteContent()
const site = computed(() => content.value.site)

const { categories } = useArticleCategories('news')
const flatCategories = computed(() => flattenCategoryTree(categories.value))

/** 分类以 URL query 为单一真相（`/news?category=xxx`），便于分享与 SSR 直达。 */
const activeCategory = computed(() => {
  const raw = route.query.category
  return typeof raw === 'string' ? raw.trim() : ''
})

const currentPage = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})

/** slug → id：后端列表按 id 过滤，而 URL 上用 slug 更可读。 */
const categoryId = computed(() => {
  if (!activeCategory.value) return 0
  return flatCategories.value.find((e) => e.node.slug === activeCategory.value)?.node.id ?? 0
})

const { list, pending } = useArticles({
  kind: 'news',
  categoryId: categoryId.value,
  page: currentPage.value,
  pageSize: PAGE_SIZE,
})

const items = computed<ArticleSummary[]>(() => list.value.items)
const totalPages = computed(() => Math.max(1, Math.ceil(list.value.total / PAGE_SIZE)))

function excerptOf(item: ArticleSummary): string {
  return articleExcerpt(item.summary, '', 120)
}

function categoryLink(slug: string): string {
  return slug ? `/news?category=${encodeURIComponent(slug)}` : '/news'
}

function goPage(page: number) {
  router.push({
    path: '/news',
    query: {
      ...(activeCategory.value ? { category: activeCategory.value } : {}),
      ...(page > 1 ? { page: String(page) } : {}),
    },
  })
}

useSeoMeta({
  title: () => (activeCategory.value ? `新闻资讯 · ${activeCategory.value}` : '新闻资讯'),
  description: () => `${site.value.name} 新闻资讯：产品动态、能力迭代与运维公告。`,
  ogType: 'website',
})
</script>
