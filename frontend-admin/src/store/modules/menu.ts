import { markRaw, type Component } from 'vue'
import { defineStore } from 'pinia'

import {
  AiToolIcon,
  AppIcon,
  BillIcon,
  CalendarIcon,
  ChartBarIcon,
  CheckCircleIcon,
  CloudDownloadIcon,
  CloudIcon,
  ControlPlatformIcon,
  DashboardIcon,
  DataCheckedIcon,
  DiscountIcon,
  DownloadIcon,
  ErrorCircleIcon,
  FileIcon,
  FilePasteIcon,
  FolderIcon,
  HistoryIcon,
  HomeIcon,
  ImageIcon,
  InternetIcon,
  KeyIcon,
  LayersIcon,
  LinkIcon,
  LockOnIcon,
  MailIcon,
  MenuIcon,
  MoneyIcon,
  OrderIcon,
  RefreshIcon,
  ServerIcon,
  ServiceIcon,
  SettingIcon,
  ShareIcon,
  StopIcon,
  TagIcon,
  TicketIcon,
  UploadIcon,
  UserIcon,
  UserCircleIcon,
  UserListIcon,
  UsergroupIcon,
  VerifyIcon,
  WalletIcon,
} from 'tdesign-icons-vue-next'

import { getMenuTree, type MenuNode } from '@/api/menu'

// 后端菜单 icon 字段是字符串名，这里映射到 TDesign 图标组件（按需引入）。
const iconMap: Record<string, Component> = {
  dashboard: DashboardIcon,
  home: HomeIcon,
  user: UserIcon,
  users: UserIcon,
  'user-list': UserListIcon,
  usergroup: UsergroupIcon,
  'user-circle': UserCircleIcon,
  'control-platform': ControlPlatformIcon,
  layers: LayersIcon,
  cloud: CloudIcon,
  server: ServerIcon,
  'file-paste': FilePasteIcon,
  file: FileIcon,
  folder: FolderIcon,
  image: ImageIcon,
  internet: InternetIcon,
  app: AppIcon,
  apps: AppIcon,
  tag: TagIcon,
  bill: BillIcon,
  money: MoneyIcon,
  discount: DiscountIcon,
  order: OrderIcon,
  setting: SettingIcon,
  settings: SettingIcon,
  menu: MenuIcon,
  history: HistoryIcon,
  service: ServiceIcon,
  ticket: TicketIcon,
  'chart-bar': ChartBarIcon,
  calendar: CalendarIcon,
  'lock-on': LockOnIcon,
  key: KeyIcon,
  link: LinkIcon,
  mail: MailIcon,
  email: MailIcon,
  share: ShareIcon,
  promote: ShareIcon,
  'check-circle': CheckCircleIcon,
  'error-circle': ErrorCircleIcon,
  stop: StopIcon,
  refresh: RefreshIcon,
  verify: VerifyIcon,
  product: AppIcon,
  resource: LayersIcon,
  data: DashboardIcon,
  catalog: LayersIcon,
  wallet: WalletIcon,
  download: DownloadIcon,
  upload: UploadIcon,
  'cloud-download': CloudDownloadIcon,
  'data-checked': DataCheckedIcon,
  'ai-tool': AiToolIcon,
}

function resolveIcon(name?: string): Component | undefined {
  if (!name) return undefined
  const comp = iconMap[name] || iconMap[name.toLowerCase()]
  // 仅当映射到真实组件时才 markRaw，避免对 undefined 调用 Vue.markRaw(undefined) 抛错
  // （该 Vue 版本的 markRaw 先执行 Object.hasOwn(value,...) 再判空，传入 undefined 会抛 TypeError）。
  return comp ? markRaw(comp) : undefined
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

// 递归转换菜单树为扁平结构；仅保留启用（active）节点，禁用菜单不在侧边栏下发展示
function toFlatMenu(nodes: MenuNode[]): FlatMenu[] {
  return nodes
    .filter((node) => node.status === 'active')
    .map((node) => ({
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
