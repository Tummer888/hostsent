import cloneDeep from 'lodash/cloneDeep';
import { defineStore } from 'pinia';
import type { RouteRecordRaw } from 'vue-router';

import router, { fixedRouterList, homepageRouterList } from '@/router';
import { store } from '@/store';

export const usePermissionStore = defineStore('permission', {
  state: () => ({
    whiteListRouters: ['/login'],
    routers: [] as Array<RouteRecordRaw>,
    removeRoutes: [] as Array<RouteRecordRaw>,
    asyncRoutes: [] as Array<RouteRecordRaw>,
  }),
  actions: {
    async initRoutes() {
      const accessedRouters = this.asyncRoutes;
      this.routers = cloneDeep([...homepageRouterList, ...accessedRouters, ...fixedRouterList]);
    },
    async buildAsyncRoutes() {
      this.asyncRoutes = [];
      await this.initRoutes();
      return this.asyncRoutes;
    },
    async restoreRoutes() {
      this.asyncRoutes.forEach((item: RouteRecordRaw) => {
        if (item.name) {
          router.removeRoute(item.name);
        }
      });
      this.asyncRoutes = [];
      await this.initRoutes();
    },
  },
});

export function getPermissionStore() {
  return usePermissionStore(store);
}
