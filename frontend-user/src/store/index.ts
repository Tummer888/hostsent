import { createPinia } from 'pinia'

export const pinia = createPinia()

export { useUserStore } from './modules/user'
export { useMenuStore } from './modules/menu'
