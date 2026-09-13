/**
 * 员工体系常量（S1，doc86 §4.1）
 *
 * 后端枚举的单一镜像：员工类型 staff_type 与部门类型 kind。
 * 与权限点(roles)正交：员工类型只决定业务身份（是否可被派单/分配客户），
 * 角色才决定能操作什么。
 */

/** 员工类型选项（对应 model.StaffType*） */
export const staffTypeOptions = [
  { label: '平台管理', value: 'admin' },
  { label: '销售', value: 'sales' },
  { label: '客服', value: 'support' },
  { label: '技术', value: 'tech' },
  { label: '运维', value: 'ops' },
  { label: '财务', value: 'finance' },
]

const staffTypeLabelMap: Record<string, string> = Object.fromEntries(
  staffTypeOptions.map((item) => [item.value, item.label]),
)

/** 员工类型中文名，未知值原样返回（避免新枚举上线后前端显示空白） */
export function staffTypeLabel(value?: string): string {
  if (!value) return '—'
  return staffTypeLabelMap[value] || value
}

/** 员工类型标签主题：销售/客服/技术等等按语义区分，便于列表快速识别 */
export function staffTypeTheme(value?: string): 'primary' | 'success' | 'warning' | 'danger' | 'default' {
  switch (value) {
    case 'sales':
      return 'primary'
    case 'support':
      return 'success'
    case 'tech':
      return 'warning'
    case 'finance':
      return 'danger'
    default:
      return 'default'
  }
}

/** 部门类型选项（对应 model.DepartmentKind*） */
export const departmentKindOptions = [
  { label: '综合', value: 'general' },
  { label: '销售', value: 'sales' },
  { label: '客服', value: 'support' },
  { label: '技术', value: 'tech' },
  { label: '运维', value: 'ops' },
  { label: '财务', value: 'finance' },
]

const departmentKindLabelMap: Record<string, string> = Object.fromEntries(
  departmentKindOptions.map((item) => [item.value, item.label]),
)

export function departmentKindLabel(value?: string): string {
  if (!value) return '综合'
  return departmentKindLabelMap[value] || value
}

export function departmentKindTheme(value?: string): 'primary' | 'success' | 'warning' | 'danger' | 'default' {
  switch (value) {
    case 'sales':
      return 'primary'
    case 'support':
      return 'success'
    case 'tech':
      return 'warning'
    case 'finance':
      return 'danger'
    default:
      return 'default'
  }
}

export const activeStatusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]
