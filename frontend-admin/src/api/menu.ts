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

export interface MenuCreateRequest {
  parent_id?: number
  platform: string
  name: string
  type?: string
  path?: string
  component?: string
  icon?: string
  sort_order?: number
  status?: string
}

export type MenuUpdateRequest = MenuCreateRequest & { status: string }

export function getMenuTree(platform: string): Promise<MenuNode[]> {
  return request.get<MenuNode[]>({
    url: '/menus/tree',
    params: { platform },
  })
}

export function createMenu(data: MenuCreateRequest): Promise<MenuNode> {
  return request.post<MenuNode>({
    url: '/menus',
    data,
  })
}

export function updateMenu(id: number, data: MenuUpdateRequest): Promise<MenuNode> {
  return request.put<MenuNode>({
    url: `/menus/${id}`,
    data,
  })
}

export function deleteMenu(id: number): Promise<string> {
  return request.delete<string>({
    url: `/menus/${id}`,
  })
}
