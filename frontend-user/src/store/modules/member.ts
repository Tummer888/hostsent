import { defineStore } from 'pinia'
import { computed } from 'vue'

import { useUserStore } from './user'

/**
 * 成员（子账号）权限 store（P4-09）。
 *
 * 数据来源：/uc/auth/userinfo 返回的 is_sub_account / owner 信息。
 * 权限码由后端在接口层强制校验，前端只负责隐藏入口与提示，不作为安全边界。
 */
export const useMemberStore = defineStore('member', () => {
  const userStore = useUserStore()

  const isSub = computed(() => Boolean(userStore.userInfo?.is_sub_account))
  const isOwner = computed(() => !isSub.value)
  const ownerName = computed(() => userStore.userInfo?.owner_name || '')
  const permissions = computed<string[]>(() => userStore.permissions)
  const isSuperAccount = computed(() => !isSub.value)

  /** has 判断是否持有某客户侧权限码；主账号恒为 true（拥有全部客户侧权限）。 */
  function has(code: string): boolean {
    if (!isSub.value) return true
    return permissions.value.includes(code)
  }

  return {
    isSub,
    isOwner,
    isSuperAccount,
    ownerName,
    permissions,
    has,
  }
})
