import { computed, reactive, ref } from 'vue'

import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

export interface SecurityPageConfig<TQuery extends Record<string, any>, TItem> {
  title: string
  subtitle: string
  tableTitle: string
  tableDesc: string
  emptyText: string
  columns: PrimaryTableCol<TItem>[]
  createQuery: () => TQuery
  fetcher: (query: TQuery) => Promise<{ items: TItem[]; meta: { page: number; page_size: number; total: number } }>
}

export function useSecurityListPage<TQuery extends { page?: number; page_size?: number }, TItem>(config: SecurityPageConfig<TQuery, TItem>) {
  const loading = ref(false)
  const errorMessage = ref('')
  const items = ref<TItem[]>([])
  const filters = reactive<TQuery>(config.createQuery())
  const pagination = reactive({
    current: filters.page || 1,
    pageSize: filters.page_size || 10,
    total: 0,
    showJumper: true,
    showPageSize: true,
    pageSizeOptions: [10, 20, 50, 100],
  })

  const pageTitle = computed(() => config.title)
  const pageSubtitle = computed(() => config.subtitle)
  const tableTitle = computed(() => config.tableTitle)
  const tableDesc = computed(() => config.tableDesc)

  async function load() {
    loading.value = true
    errorMessage.value = ''
    try {
      filters.page = pagination.current
      filters.page_size = pagination.pageSize
      const data = await config.fetcher({ ...filters })
      items.value = data.items || []
      pagination.current = data.meta.page
      pagination.pageSize = data.meta.page_size
      pagination.total = data.meta.total
    } catch (error) {
      items.value = []
      pagination.total = 0
      errorMessage.value = (error as Error)?.message || `${config.title}加载失败`
    } finally {
      loading.value = false
    }
  }

  async function handleSearch() {
    pagination.current = 1
    filters.page = 1
    await load()
  }

  async function handleReset() {
    const next = config.createQuery()
    Object.assign(filters, next)
    pagination.current = next.page || 1
    pagination.pageSize = next.page_size || 10
    await load()
  }

  async function handlePageChange(pageInfo: PageInfo) {
    pagination.current = pageInfo.current
    pagination.pageSize = pageInfo.pageSize
    await load()
  }

  return {
    loading,
    errorMessage,
    items,
    filters,
    pagination,
    pageTitle,
    pageSubtitle,
    tableTitle,
    tableDesc,
    columns: config.columns,
    emptyText: config.emptyText,
    load,
    handleSearch,
    handleReset,
    handlePageChange,
  }
}
