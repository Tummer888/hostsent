import type { Component } from 'vue'
import { defineStore } from 'pinia'

import {
  AppIcon,
  BillIcon,
  CloudIcon,
  DashboardIcon,
  HomeIcon,
  LayersIcon,
  MenuIcon,
  OrderIcon,
  ServiceIcon,
  SettingIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'

import { getMenuTree, type MenuNode } from '@/api/menu'

// 后端菜单 icon 字段是字符串名，这里映射到 TDesign 图标组件。
const iconMap: Record<string, Component> = {
  dashboard: DashboardIcon,
  home: HomeIcon,
  user: UserIcon,
  users: UserIcon,
  layers: LayersIcon,
  cloud: CloudIcon,
  bill: BillIcon,
  service: ServiceIcon,
  settings: SettingIcon,
  setting: SettingIcon,
  menu: MenuIcon,
  apps: AppIcon,
  order: OrderIcon,
  product: AppIcon,
  resource: LayersIcon,
  data: DashboardIcon,
}

function resolveIcon(name?: string): Component | undefined {
  if (!name) return undefined
  return iconMap[name] || iconMap[name.toLowerCase()]
}

export interface FlatMenu {
  id: number
  parentId: number
  name: string
  path?: string
  component?: string
  icon?: Component
  children: FlatMenu[]
}

function toFlatMenu(nodes: MenuNode[]): FlatMenu[] {
  return nodes.map((node) => ({
    id: node.id,
    parentId: node.parent_id,
    name: node.name,
    path: node.path,
    component: node.component,
    icon: resolveIcon(node.icon),
    children: node.children?.length ? toFlatMenu(node.children) : [],
  }))
}

export const useMenuStore = defineStore('menu', {
  state: () => ({
    menus: [] as FlatMenu[],
    loaded: false,
  }),
  getters: {
    sidebarMenus: (state): FlatMenu[] => state.menus,
    hasMenu: (state) => state.loaded && state.menus.length > 0,
  },
  actions: {
    async loadMenus(platform = 'admin') {
      const tree = await getMenuTree(platform)
      this.menus = toFlatMenu(tree)
      this.loaded = true
    },
    reset() {
      this.menus = []
      this.loaded = false
    },
  },
})
