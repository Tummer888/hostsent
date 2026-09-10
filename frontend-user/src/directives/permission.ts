import type { Directive } from 'vue'

import { useMemberStore } from '@/store/modules/member'

/**
 * v-permission="'order:create'" 或 v-permission="['a','b']"（任一命中即保留元素）。
 * 仅作体验优化（隐藏无权入口），真正的越权防护由后端 RequireUserPermission 承担。
 */
export const permission: Directive<HTMLElement, string | string[]> = {
  mounted(el, binding) {
    const codes = Array.isArray(binding.value) ? binding.value : [binding.value]
    if (!codes.length) return
    const memberStore = useMemberStore()
    if (!codes.some((code) => memberStore.has(code))) {
      el.parentNode?.removeChild(el)
    }
  },
}

export default permission
