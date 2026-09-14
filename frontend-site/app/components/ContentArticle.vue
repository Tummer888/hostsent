<template>
  <article class="content-article">
    <nav class="content-crumb" aria-label="面包屑">
      <NuxtLink to="/">首页</NuxtLink>
      <span>/</span>
      <NuxtLink :to="backTo">{{ backLabel }}</NuxtLink>
      <span>/</span>
      <span class="content-crumb__current">{{ title }}</span>
    </nav>

    <header class="content-article__head">
      <h1 class="content-article__title">{{ title }}</h1>
      <div class="content-article__meta">
        <slot name="meta" />
      </div>
    </header>

    <div class="content-article__body">
      <RichContent :html="body" :format="format" />
    </div>

    <footer class="content-article__foot">
      <NuxtLink :to="backTo" class="site-btn site-btn--ghost">{{ backLabel }}</NuxtLink>
      <span v-if="updatedAt" class="content-article__updated">
        最后更新：{{ formatArticleDate(updatedAt) }}
      </span>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { formatArticleDate } from '#shared/schemas/content'

withDefaults(
  defineProps<{
    title: string
    body: string
    /** text 走纯文本输出（存量公告），html 直接渲染（后端已净化）。 */
    format?: 'html' | 'text'
    backTo: string
    backLabel: string
    updatedAt?: string
  }>(),
  { format: 'html', updatedAt: '' },
)
</script>

<style scoped>
.content-article {
  padding: 34px 0 72px;
}

.content-article__head {
  margin-top: 22px;
}

.content-article__updated {
  font-size: 13px;
  color: var(--site-text-subtle);
}
</style>
