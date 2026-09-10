// 类型定义（拆分自 interface.d.ts：auth 域）
export interface UserInfo {
  id: number
  name: string
  username: string
  role: string
  roles: string[]
  email: string
  phone: string
  status: string
  avatar?: string
  department?: string
  position?: string
}
