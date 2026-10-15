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
 *
 * 关于「未分类」：分类为空（0）与分类指向已删除的分类，在界面上都只能落成
 * "没有有效归属"这一种事实，所以用同一个词表达。原先空值显示 "—"，
 * 看起来像某处没加载出来，而不是"这里真的没选分类、需要去补"。
 */
export function useCategoryOptions() {
  const categoryOptions = ref<{ label: string; value: number }[]>([])
  const categoryIdMap = ref<Record<number, string>>({})

  /** 未分类商品数（后端单独给一个桶；接口未返回时为 undefined） */
  const uncategorizedCount = ref<number | undefined>(undefined)
  /** 分类导航计数：分类 ID → 该分类自身直接挂的商品数 */
  const categoryCounts = ref<Record<number, number>>({})

  async function loadCategories() {
    try {
      const data = await getProductCategoryList()
      const options: { label: string; value: number }[] = []
      const map: Record<number, string> = {}
      const counts: Record<number, number> = {}
      const flatten = (nodes: SaleProductCategoryInfo[]) => {
        for (const node of nodes) {
          options.push({ label: node.name, value: node.id })
          map[node.id] = node.name
          if (typeof node.product_count === 'number') counts[node.id] = node.product_count
          if (node.children?.length) flatten(node.children)
        }
      }
      flatten(data.items || [])
      categoryOptions.value = options
      categoryIdMap.value = map
      categoryCounts.value = counts
      uncategorizedCount.value = data.uncategorized_count
    } catch {
      /* 分类加载失败不阻塞列表渲染 */
    }
  }

  /**
   * 取分类展示名：分类为空或指向已删除分类时一律给「未分类」。
   *
   * 不给空串/破折号：那两种取值都是"分类没落地"，显示 "—" 会让运营误以为是
   * 加载失败或字段缺失，看不出真正该动作的方向（去补分类）。
   */
  function categoryName(id?: number): string {
    if (!id) return '未分类'
    return categoryIdMap.value[id] || '未分类'
  }

  return { categoryOptions, categoryIdMap, categoryName, categoryCounts, uncategorizedCount, loadCategories }
}
