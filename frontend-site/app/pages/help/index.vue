<template>
  <div class="content-page">
    <section class="content-hero">
      <div class="site-container">
        <nav class="content-crumb" aria-label="面包屑">
          <NuxtLink to="/">首页</NuxtLink>
          <span>/</span>
          <span class="content-crumb__current">帮助中心</span>
        </nav>
        <h1 class="content-hero__title">帮助中心</h1>
        <p class="content-hero__desc">
          从选购到开通、从计费到排障，{{ site.name }} 的使用问题在这里都能找到答案。
        </p>
      </div>
    </section>

    <section class="content-shell">
      <div class="site-container">
        <div class="content-layout">
          <HelpTree :categories="categories" :active-slug="activeCategory" />

          <div>
            <ul v-if="items.length" class="news-list">
              <li v-for="item in items" :key="item.id" class="news-item">
                <div class="news-item__body">
                  <div class="news-item__meta">
                    <span v-if="item.pinned" class="news-item__badge">置顶</span>
                    <span v-if="item.categoryName">{{ item.categoryName }}</span>
                    <time>{{ formatArticleDate(item.updatedAt || item.publishAt) }}</time>
                  </div>
                  <h2 class="news-item__title">
                    <NuxtLink :to="`/help/${item.slug}`">{{ item.title }}</NuxtLink>
                  </h2>
                  <p v-if="item.summary" class="news-item__excerpt">{{ item.summary }}</p>
                </div>
              </li>
            </ul>

            <div v-else class="content-empty">
              <p class="content-empty__title">
                {{ activeCategory ? '该目录下暂无文档' : '帮助文档正在整理中' }}
              </p>
              <p class="content-empty__desc">
                遇到紧急问题可直接在用户中心提交工单，我们会尽快响应。
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
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import {
  findCategoryBySlug,
  formatArticleDate,
  type ArticleSummary,
} from '#shared/schemas/content'

const PAGE_SIZE = 12

const route = useRoute()
const router = useRouter()
const { content } = useSiteContent()
const site = computed(() => content.value.site)

const { categories } = useArticleCategories('help')

const activeCategory = computed(() => {
  const raw = route.query.category
  return typeof raw === 'string' ? raw.trim() : ''
})

const currentPage = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})

const categoryId = computed(() => {
  if (!activeCategory.value) return 0
  return findCategoryBySlug(categories.value, activeCategory.value)?.id ?? 0
})

const { list } = useArticles({
  kind: 'help',
  categoryId: categoryId.value,
  page: currentPage.value,
  pageSize: PAGE_SIZE,
})

const items = computed<ArticleSummary[]>(() => list.value.items)
const totalPages = computed(() => Math.max(1, Math.ceil(list.value.total / PAGE_SIZE)))

function goPage(page: number) {
  router.push({
    path: '/help',
    query: {
      ...(activeCategory.value ? { category: activeCategory.value } : {}),
      ...(page > 1 ? { page: String(page) } : {}),
    },
  })
}

useSeoMeta({
  title: '帮助中心',
  description: () => `${site.value.name} 帮助中心：产品选购、开通交付、计费与排障文档。`,
  ogType: 'website',
})
</script>
