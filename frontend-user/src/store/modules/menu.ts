import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Component } from 'vue'
import {
  CartIcon,
  CloudIcon,
  DashboardIcon,
  EditIcon,
  FilePasteIcon,
  GiftIcon,
  HomeIcon,
  ImageIcon,
  LayersIcon,
  LockOnIcon,
  MailIcon,
  MoneyIcon,
  OrderIcon,
  RefreshIcon,
  SecuredIcon,
  ServerIcon,
  ServiceIcon,
  SettingIcon,
  ShareIcon,
  UserCircleIcon,
  UserIcon,
  UsergroupIcon,
  WalletIcon,
} from 'tdesign-icons-vue-next'

import { getMenuTree, type MenuNode } from '@/api/menu'

// 后端菜单 icon 字段是字符串名，这里映射到 TDesign 图标组件（按需引入）。
//
// 注意：mapIcon 命中不到时返回 undefined，页面表现只是「这一项没图标」，不报错。
// 因此后端 seed 新增菜单项时必须同步在这里补映射，否则静默丢失图标。
const iconMap: Record<string, Component> = {
  dashboard: DashboardIcon,
  home: HomeIcon,
  user: UserIcon,
  'user-circle': UserCircleIcon,
  cloud: CloudIcon,
  server: ServerIcon,
  'file-paste': FilePasteIcon,
  image: ImageIcon,
  layers: LayersIcon,
  order: OrderIcon,
  cart: CartIcon,
  wallet: WalletIcon,
  bill: WalletIcon,
  money: MoneyIcon,
  refresh: RefreshIcon,
  'lock-on': LockOnIcon,
  // 后端菜单 icon 为 "safety"（doc91 /profile/security）；未映射会静默丢图标。
  safety: SecuredIcon,
  mail: MailIcon,
  setting: SettingIcon,
  edit: EditIcon,
  service: ServiceIcon,
  ticket: ServiceIcon,
  share: ShareIcon,
  usergroup: UsergroupIcon,
  gift: GiftIcon,
}

/**
 * 仅主账号可见的菜单路径。
 *
 * menus 表没有 ownerOnly 字段（不为一条菜单加列），这里按路径白名单在前端过滤，
 * 与 router meta.ownerOnly 是同一套语义、同一批路径，改一处要同时改两处。
 */
/**
 * 仅主账号可见的菜单路径（子账号侧边栏隐藏）。
 *
 * 与后端 uc/menu 服务的 ownerOnlyMenuPaths 必须一致：后端已按权限裁剪，
 * 前端这份是「同一次请求里就藏好」的兜底 —— 菜单数据可能来自缓存或旧接口，
 * 不能让子账号看到点进去必然 403 的入口。改动时两边同步。
 */
export const OWNER_ONLY_MENU_PATHS: ReadonlySet<string> = new Set([
  '/member',
  '/referral',
  '/referral/overview',
  '/referral/invitees',
  '/referral/cashbacks',
  '/referral/withdrawals',
  '/referral/materials',
])

export interface FlatMenu {
  id: number
  parentId: number
  name: string
  path?: string
  icon?: Component
  children?: FlatMenu[]
}

function mapIcon(iconName?: string): Component | undefined {
  if (!iconName) return undefined
  const key = iconName.toLowerCase()
  return iconMap[key]
}

function flattenMenu(node: MenuNode): FlatMenu {
  return {
    id: node.id,
    parentId: node.parentId,
    name: node.name,
    path: node.path,
    icon: mapIcon(node.icon),
    children: node.children?.map(flattenMenu),
  }
}

export const useMenuStore = defineStore('menu', () => {
  const menus = ref<FlatMenu[]>([])
  const loaded = ref(false)

  // 侧边栏渲染源。ownerOnly 过滤不在这里做：那需要读 memberStore（登录态），
  // 让菜单 store 依赖会员 store 会引入循环依赖。过滤放在 SideNav 组件内。
  const sidebarMenus = computed<FlatMenu[]>(() => menus.value)
  const hasMenu = computed(() => menus.value.length > 0)

  async function loadMenus(platform: string = 'user') {
    const { data } = await getMenuTree(platform)
    menus.value = data.map(flattenMenu)
    loaded.value = true
  }

  function reset() {
    menus.value = []
    loaded.value = false
  }

  return {
    menus,
    loaded,
    sidebarMenus,
    hasMenu,
    loadMenus,
    reset,
  }
})
