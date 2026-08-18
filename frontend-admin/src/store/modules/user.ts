import { defineStore } from 'pinia';

import { getCurrentUser, login as loginApi } from '@/api/auth';
import { usePermissionStore } from '@/store';
import type { UserInfo } from '@/types/interface';

const initUserInfo: UserInfo = {
  id: 0,
  name: '',
  username: '',
  role: '',
  roles: [],
  email: '',
  phone: '',
  status: '',
};

export const useUserStore = defineStore('user', {
  state: () => ({
    token: '',
    userInfo: { ...initUserInfo },
  }),
  getters: {
    roles: (state) => state.userInfo.roles,
  },
  actions: {
    async login(userInfo: Record<string, unknown>) {
      const { account, password } = userInfo as { account: string; password: string };
      const res = await loginApi({
        username: account,
        password,
      });

      this.token = res.token;
      this.userInfo = {
        id: res.user_info.id,
        name: res.user_info.username,
        username: res.user_info.username,
        role: res.user_info.role,
        roles: res.user_info.roles?.length ? res.user_info.roles : [res.user_info.role],
        email: res.user_info.email,
        phone: res.user_info.phone,
        status: res.user_info.status,
      };
    },
    async getUserInfo() {
      const res = await getCurrentUser();
      this.userInfo = {
        id: res.id,
        name: res.username,
        username: res.username,
        role: res.role,
        roles: res.roles?.length ? res.roles : [res.role],
        email: res.email,
        phone: res.phone,
        status: res.status,
      };
      return this.userInfo;
    },
    async logout() {
      this.token = '';
      this.userInfo = { ...initUserInfo };
    },
  },
  persist: {
    afterHydrate: () => {
      const permissionStore = usePermissionStore();
      permissionStore.initRoutes();
    },
    key: 'user',
    pick: ['token'],
  },
});
