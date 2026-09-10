import { computed } from 'vue'

import { useUserStore } from '@/store'

/**
 * 权限判断组合式函数，用于 v-if / :disabled 等场景：
 *   const { has, permissions, isSuper } = usePermission()
 *   has('ticket:reply')            // 单码
 *   has('ticket:reply', 'ticket:close') // 任一命中
 */
export function usePermission() {
  const userStore = useUserStore()

  const has = (...codes: string[]): boolean => userStore.hasPermission(...codes)

  const permissions = computed(() => userStore.permissions)
  const isSuper = computed(() => userStore.permissions.includes('*'))

  return { has, permissions, isSuper }
}

export default usePermission
