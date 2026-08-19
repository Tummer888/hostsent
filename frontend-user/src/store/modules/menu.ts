import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Component } from 'vue'
import {
  DashboardIcon,
  HomeIcon,
  LayersIcon,
  OrderIcon,
  ServiceIcon,
  UserIcon,
  WalletIcon,
} from 'tdesign-icons-vue-next'

import { getMenuTree, type MenuNode } from '@/api/menu'

const iconMap: Record<string, Component> = {
  dashboard: DashboardIcon,
  home: HomeIcon,
  user: UserIcon,
  cloud: LayersIcon,
  layers: LayersIcon,
  order: OrderIcon,
  wallet: WalletIcon,
  bill: WalletIcon,
  service: ServiceIcon,
  ticket: ServiceIcon,
}

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
  return iconMap[key] || DashboardIcon
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
