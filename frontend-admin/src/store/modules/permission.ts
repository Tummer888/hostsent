import { defineStore } from 'pinia'

export const usePermissionStore = defineStore('permission', {
  state: () => ({
    whiteListRouters: ['/login'],
  }),
  actions: {
    restoreRoutes() {},
  },
})
