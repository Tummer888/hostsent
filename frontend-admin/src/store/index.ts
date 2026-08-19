import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import type { Pinia } from 'pinia'

export { useUserStore } from './modules/user'
export { usePermissionStore } from './modules/permission'
export { useMenuStore } from './modules/menu'

export const pinia: Pinia = createPinia()
pinia.use(piniaPluginPersistedstate)
