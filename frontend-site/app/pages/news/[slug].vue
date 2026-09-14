<template>
  <div class="content-shell">
    <div class="site-container">
      <ContentArticle
        :title="article!.title"
        :body="article!.body"
        back-to="/news"
        back-label="返回新闻列表"
        :updated-at="article!.updatedAt"
      >
        <template #meta>
          <span v-if="article!.categoryName">{{ article!.categoryName }}</span>
          <time>{{ formatArticleDate(article!.publishAt) }}</time>
          <span v-if="article!.tags.length">标签：{{ article!.tags.join('、') }}</span>
        </template>
      </ContentArticle>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatArticleDate } from '#shared/schemas/content'

const route = useRoute()
// 详情接口的 404/503 会在这里抛出并交给 error.vue：
// 走到模板时 article 必然有值，无需再做「内容不存在」分支（那会变成 soft 404）。
const { article } = await useArticleDetail('news', () => String(route.params.slug ?? ''))

useSeoMeta({
  title: () => article.value?.title ?? '新闻详情',
  description: () => article.value?.summary ?? '',
  ogType: 'article',
})
</script>
