import { createPinia } from 'pinia'

export const pinia = createPinia()

export { useUserStore } from './modules/user'
export { useMenuStore } from './modules/menu'
export { useSettingsStore } from './modules/settings'
export { useCartStore, type CartItem } from './modules/cart'
export { useBrandStore } from './modules/brand'
