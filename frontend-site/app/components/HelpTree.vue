<template>
  <aside class="help-tree">
    <h2 class="help-tree__title">文档目录</h2>
    <ul class="help-tree__list">
      <li>
        <NuxtLink
          class="help-tree__link"
          :class="{ 'is-active': !activeSlug }"
          to="/help"
        >
          全部文档
        </NuxtLink>
      </li>
      <li v-for="entry in flatCategories" :key="entry.node.id">
        <NuxtLink
          class="help-tree__link"
          :class="[
            { 'is-active': activeSlug === entry.node.slug },
            { 'help-tree__link--child': entry.depth > 0 },
          ]"
          :to="categoryLink(entry.node.slug)"
        >
          {{ entry.node.name }}
        </NuxtLink>
      </li>
    </ul>
  </aside>
</template>

<script setup lang="ts">
import { flattenCategoryTree, type ArticleCategory } from '#shared/schemas/content'

const props = defineProps<{
  categories: ArticleCategory[]
  /** 当前选中的分类 slug；空串表示「全部文档」。 */
  activeSlug: string
}>()

const flatCategories = computed(() => flattenCategoryTree(props.categories))

function categoryLink(slug: string): string {
  return slug ? `/help?category=${encodeURIComponent(slug)}` : '/help'
}
</script>
