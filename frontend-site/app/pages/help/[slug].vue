<template>
  <div class="content-page">
    <section class="content-hero">
      <div class="site-container">
        <nav class="content-crumb" aria-label="面包屑">
          <NuxtLink to="/">首页</NuxtLink>
          <span>/</span>
          <NuxtLink to="/help">帮助中心</NuxtLink>
          <span>/</span>
          <span class="content-crumb__current">{{ article!.title }}</span>
        </nav>
      </div>
    </section>

    <section class="content-shell">
      <div class="site-container">
        <div class="content-layout">
          <HelpTree :categories="categories" :active-slug="article!.categorySlug" />

          <ContentArticle
            :title="article!.title"
            :body="article!.body"
            back-to="/help"
            back-label="返回帮助中心"
            :updated-at="article!.updatedAt"
          >
            <template #meta>
              <span v-if="article!.categoryName">{{ article!.categoryName }}</span>
              <time>{{ formatArticleDate(article!.updatedAt || article!.publishAt) }}</time>
            </template>
          </ContentArticle>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { formatArticleDate } from '#shared/schemas/content'

const route = useRoute()

const { categories } = useArticleCategories('help')
// 404/503 由 useArticleDetail 直接抛出（error.vue 呈现），模板里不再需要空态分支。
const { article } = await useArticleDetail('help', () => String(route.params.slug ?? ''))

useSeoMeta({
  title: () => article.value?.title ?? '帮助文档',
  description: () => article.value?.summary ?? '',
  ogType: 'article',
})
</script>
