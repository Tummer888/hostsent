import type { Directive } from 'vue'

import { useUserStore } from '@/store'

/**
 * v-permission="'ticket:reply'" 或 v-permission="['a','b']"（任一命中即保留元素）。
 * 仅作体验优化（隐藏无权入口），真正的越权防护由后端 RequirePermission 承担。
 */
export const permission: Directive<HTMLElement, string | string[]> = {
  mounted(el, binding) {
    const codes = Array.isArray(binding.value) ? binding.value : [binding.value]
    if (!codes.length) return
    const userStore = useUserStore()
    if (!userStore.hasPermission(...codes)) {
      el.parentNode?.removeChild(el)
    }
  },
}

export default permission
