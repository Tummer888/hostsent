import {
  EMPTY_ARTICLE_CATEGORIES,
  type ArticleCategory,
  type ArticleKind,
} from '#shared/schemas/content'

/**
 * 内容分类树数据获取（新闻分栏 / 帮助目录）。
 * 后端不可用时返回空数组，侧栏退化为只有「全部」，页面其余部分不受影响。
 */
export function useArticleCategories(kind: ArticleKind) {
  const { data, pending } = useAsyncData<ArticleCategory[]>(
    `article-categories:${kind}`,
    () => $fetch<ArticleCategory[]>('/api/article-categories', { query: { kind } }),
    { default: () => EMPTY_ARTICLE_CATEGORIES },
  )

  return {
    categories: computed<ArticleCategory[]>(() => data.value ?? EMPTY_ARTICLE_CATEGORIES),
    pending,
  }
}
