import {
  EMPTY_ARTICLE_LIST,
  type ArticleKind,
  type ArticleList,
} from '#shared/schemas/content'

export interface UseArticlesOptions {
  kind: ArticleKind
  /** 分类筛选（分类 id）；0 或省略表示不限 */
  categoryId?: number
  page?: number
  pageSize?: number
}

/**
 * 文章列表数据获取（规范 R2/R3：走 BFF，且必须有默认值兜底）。
 *
 * key 里带全部查询参数，保证「同一页面上同时展示两组列表」时不会互相覆盖缓存；
 * `watch` 里放原始参数（而不是 key 函数），参数变了自动重取。
 */
export function useArticles(options: UseArticlesOptions) {
  const kind = computed(() => options.kind)
  const categoryId = computed(() => options.categoryId ?? 0)
  const page = computed(() => options.page ?? 1)
  const pageSize = computed(() => options.pageSize ?? 10)

  const { data, pending, error } = useAsyncData<ArticleList>(
    () => `articles:${kind.value}:${categoryId.value}:${page.value}:${pageSize.value}`,
    () =>
      $fetch<ArticleList>('/api/articles', {
        query: {
          kind: kind.value,
          ...(categoryId.value ? { category_id: String(categoryId.value) } : {}),
          page: String(page.value),
          page_size: String(pageSize.value),
        },
      }),
    { default: () => EMPTY_ARTICLE_LIST, watch: [kind, categoryId, page, pageSize] },
  )

  return {
    list: computed<ArticleList>(() => data.value ?? EMPTY_ARTICLE_LIST),
    pending,
    error,
  }
}
