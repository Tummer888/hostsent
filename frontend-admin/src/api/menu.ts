import { request } from '@/utils/request'

export interface MenuNode {
  id: number
  parent_id: number
  platform: string
  name: string
  type: string
  path?: string
  component?: string
  icon?: string
  sort_order: number
  status: string
  children?: MenuNode[]
}

// 只读：菜单由后端 seed 定义（backend/internal/pkg/db/db.go 的 SeedMenus()），
// 管理端不再提供新增/编辑/删除接口（doc102 M0）。
export function getMenuTree(platform: string): Promise<MenuNode[]> {
  return request.get<MenuNode[]>({
    url: '/menus/tree',
    params: { platform },
  })
}
