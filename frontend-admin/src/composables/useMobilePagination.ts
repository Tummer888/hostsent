import { reactive } from 'vue'

import type { PageInfo } from 'tdesign-vue-next'

/**
 * 列表页分页状态机：统一维护桌面端（t-table 内置分页）与移动端（MobilePagination）
 * 两套状态，避免各页重复实现「翻页三件套 + 双状态同步」。
 *
 * 用法：
 *   const { pagination, mobilePage, handlePageChange, goMobilePage,
 *           handleMobilePageSizeChange, resetPage, applyTotal } =
 *     useMobilePagination(loadList)
 *   // loadList 里把后端 total 交给 applyTotal(data.meta.total)
 *
 * 约定：`load()` 由调用方负责请求与渲染，本 composable 只维护页码/每页条数与同步。
 */
export function useMobilePagination(
  load: () => void | Promise<void>,
  options: { desktopPageSize?: number; mobilePageSize?: number } = {},
) {
  const { desktopPageSize = 20, mobilePageSize = 10 } = options

  const pagination = reactive({
    current: 1,
    pageSize: desktopPageSize,
    total: 0,
    showJumper: true,
  })

  const mobilePage = reactive({
    current: 1,
    pageSize: mobilePageSize,
    total: 0,
  })

  /** 同步后端返回的总数到两套状态。 */
  function applyTotal(total: number) {
    pagination.total = total
    mobilePage.total = total
  }

  function handlePageChange(pageInfo: Pick<PageInfo, 'current' | 'pageSize'>) {
    pagination.current = pageInfo.current
    pagination.pageSize = pageInfo.pageSize
    void load()
    mobilePage.current = pageInfo.current ?? pagination.current
    mobilePage.pageSize = pageInfo.pageSize ?? pagination.pageSize
    mobilePage.total = pagination.total
  }

  function goMobilePage(target: number) {
    const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
    if (clamped === mobilePage.current) return
    applyMobilePage(clamped, mobilePage.pageSize)
  }

  function applyMobilePage(current: number, pageSize: number) {
    pagination.current = current
    pagination.pageSize = pageSize
    mobilePage.current = current
    mobilePage.pageSize = pageSize
    handlePageChange({ current, pageSize })
  }

  function handleMobilePageSizeChange(pageSize: number) {
    mobilePage.pageSize = pageSize
    applyMobilePage(1, pageSize)
  }

  /** 回到第一页并重新加载（查询/重置筛选用）。 */
  function resetPage() {
    pagination.current = 1
    mobilePage.current = 1
    void load()
  }

  return {
    pagination,
    mobilePage,
    applyTotal,
    handlePageChange,
    goMobilePage,
    handleMobilePageSizeChange,
    resetPage,
  }
}
