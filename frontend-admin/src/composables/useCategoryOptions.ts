import { ref } from 'vue'

import { getProductCategoryList } from '@/api/product'
import type { SaleProductCategoryInfo } from '@/types/interface'

/**
 * 商品分类下拉选项：把分类树拍平成 {options, idMap}，并暴露按 id 取名的 helper。
 * 产品域此前在 5 个页面各写一份递归拍平，统一收敛到这里。
 *
 * 用法：
 *   const { categoryOptions, categoryName, loadCategories } = useCategoryOptions()
 *   onMounted(loadCategories)
 */
export function useCategoryOptions() {
  const categoryOptions = ref<{ label: string; value: number }[]>([])
  const categoryIdMap = ref<Record<number, string>>({})

  async function loadCategories() {
    try {
      const data = await getProductCategoryList()
      const options: { label: string; value: number }[] = []
      const map: Record<number, string> = {}
      const flatten = (nodes: SaleProductCategoryInfo[]) => {
        for (const node of nodes) {
          options.push({ label: node.name, value: node.id })
          map[node.id] = node.name
          if (node.children?.length) flatten(node.children)
        }
      }
      flatten(data.items || [])
      categoryOptions.value = options
      categoryIdMap.value = map
    } catch {
      /* 分类加载失败不阻塞列表渲染 */
    }
  }

  function categoryName(id?: number): string {
    if (!id) return '—'
    return categoryIdMap.value[id] || '—'
  }

  return { categoryOptions, categoryIdMap, categoryName, loadCategories }
}
