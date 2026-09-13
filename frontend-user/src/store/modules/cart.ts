import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'

/**
 * 购物车 store（前端暂存，localStorage 持久化）。
 *
 * 为什么不上后端：购物车是「登录用户的意图暂存」，跨设备同步的收益远小于
 * 新建 carts/cart_items 两张表的成本（见规划文档 87 §6.2 澄清 1）。
 * 后端没有购物车表，下单仍然是逐条走 POST /uc/orders，购物车只是下单前的清单。
 *
 * 存的是「下单所需的完整快照」（商品名/单价/规格名），不是只存 id：
 * 商品下架或改价后，购物车里仍能显示用户加入时的价格，结算时再以 quoteOrder
 * 的结果为准 —— 与后端「算价不信任前端」的口径一致。
 */

const STORAGE_KEY = 'console_cart'

export interface CartItem {
  /** 同一商品的不同规格是两行，key 由商品 + 规格 + 周期三者拼成 */
  key: string
  productId: number
  productName: string
  coverImage?: string
  specCode: string
  specName: string
  cycle: string
  quantity: number
  unitPrice: number
  priceModel: string
  addedAt: number
}

export function cartItemKey(productId: number, specCode: string, cycle: string): string {
  return `${productId}::${specCode || '-'}::${cycle || '-'}`
}

function load(): CartItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (it): it is CartItem =>
        !!it && typeof it.productId === 'number' && typeof it.key === 'string',
    )
  } catch {
    // 存储被手工改坏时按空车处理，不让整个控制台崩在解析上
    return []
  }
}

export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>(load())

  const count = computed(() => items.value.reduce((sum, it) => sum + it.quantity, 0))
  const isEmpty = computed(() => items.value.length === 0)
  /** 预估金额：仅为展示，真实应付以结算时的 quoteOrder 为准。 */
  const estimatedAmount = computed(() =>
    items.value.reduce((sum, it) => sum + it.unitPrice * it.quantity, 0),
  )

  function persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(items.value))
    } catch {
      // 隐私模式等场景下写入失败，购物车退化为「仅本次会话有效」
    }
  }

  watch(items, persist, { deep: true })

  /** 加入购物车：已存在同商品同规格同周期则累加数量。 */
  function add(item: Omit<CartItem, 'key' | 'addedAt'>) {
    const key = cartItemKey(item.productId, item.specCode, item.cycle)
    const existing = items.value.find((it) => it.key === key)
    if (existing) {
      existing.quantity += item.quantity
      existing.unitPrice = item.unitPrice
      existing.specName = item.specName
      return
    }
    items.value.push({ ...item, key, addedAt: Date.now() })
  }

  function updateQuantity(key: string, quantity: number) {
    const target = items.value.find((it) => it.key === key)
    if (!target) return
    target.quantity = Math.max(1, Math.floor(quantity) || 1)
  }

  function remove(key: string) {
    items.value = items.value.filter((it) => it.key !== key)
  }

  function clear() {
    items.value = []
  }

  return {
    items,
    count,
    isEmpty,
    estimatedAmount,
    add,
    updateQuantity,
    remove,
    clear,
  }
})
