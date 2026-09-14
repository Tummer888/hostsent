<template>
  <div class="content-page">
    <section class="content-hero">
      <div class="site-container">
        <nav class="content-crumb" aria-label="面包屑">
          <NuxtLink to="/">首页</NuxtLink>
          <span>/</span>
          <span class="content-crumb__current">{{ label }}</span>
        </nav>
        <h1 class="content-hero__title">{{ label }}</h1>
        <p class="content-hero__desc">{{ intro }}</p>
      </div>
    </section>

    <section class="content-shell">
      <div class="site-container">
        <ContentArticle
          v-if="article"
          :title="article.title"
          :body="article.body"
          back-to="/"
          back-label="返回首页"
          :updated-at="article.updatedAt || article.publishAt"
        >
          <template #meta>
            <!-- Q5：版本号由运营在管理端手工维护，门户只做展示 -->
            <span v-if="article.version">版本：{{ article.version }}</span>
            <time v-if="article.publishAt">生效日期：{{ formatArticleDate(article.publishAt) }}</time>
          </template>
        </ContentArticle>

        <!--
          未发布时不是 404：页面本身存在（注册页会链接过来），只是运营还没填正文。
          报「页面不存在」会让用户以为链接坏了；后端故障则已经在上面的组合式函数里
          抛成 503，不会走到这个占位分支。
        -->
        <div v-else class="content-placeholder">
          <p class="content-placeholder__title">{{ label }}尚未发布</p>
          <p class="content-placeholder__desc">
            该文档正在准备中。如对相关条款有疑问，可通过首页联系方式与我们沟通。
          </p>
          <NuxtLink to="/" class="site-btn site-btn--ghost">返回首页</NuxtLink>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { formatArticleDate, type ArticleKind } from '#shared/schemas/content'

/**
 * 条款 / 隐私政策共用页面组件。
 *
 * 两个页面的唯一差别是 kind 与文案，抽成一个组件避免同一份排版写两遍
 * （改一次样式要记得改两处，早晚会漏）。
 */
const props = defineProps<{
  kind: Extract<ArticleKind, 'terms' | 'privacy'>
  label: string
  intro: string
}>()

const { article } = await useSingletonArticle(props.kind)

useSeoMeta({
  title: props.label,
  description: props.intro,
  ogType: 'article',
  // 尚未发布时是一张空壳页面，不该被索引
  robots: () => (article.value ? 'index,follow' : 'noindex'),
})
</script>
