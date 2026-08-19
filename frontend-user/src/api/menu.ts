import request from '@/utils/request'

export interface MenuNode {
  id: number
  parentId: number
  name: string
  path?: string
  icon?: string
  type?: string
  children?: MenuNode[]
}

export function getMenuTree(platform: string = 'user') {
  return request.get<any, { data: MenuNode[] }>('/menus/tree', {
    params: { platform },
  })
}
