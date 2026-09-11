// 产品管理 · 常量与格式化工具

export const productStatusOptions = [
  { label: '草稿', value: 0 },
  { label: '上架', value: 1 },
  { label: '下架', value: 2 },
]

export const priceModelOptions = [
  { label: '固定价', value: 'fixed' },
  { label: '按小时', value: 'hourly' },
  { label: '按月', value: 'monthly' },
]

export const productTypeOptions = [
  { label: '云主机', value: 'cloud_host' },
  { label: 'IP', value: 'ip' },
  { label: '存储', value: 'storage' },
  { label: '带宽', value: 'bandwidth' },
]

// 商品供货模式
export const provisionModeOptions = [
  { label: '自营', value: 'self' },
  { label: '上游克隆', value: 'clone' },
]

export function provisionModeText(mode: string): string {
  return mode === 'clone' ? '上游克隆' : mode === 'self' ? '自营' : mode || '自营'
}

export function provisionModeTag(mode: string): { theme: 'primary' | 'warning' | 'default'; text: string } {
  if (mode === 'clone') return { theme: 'warning', text: '上游克隆' }
  if (mode === 'self') return { theme: 'primary', text: '自营' }
  return { theme: 'default', text: '自营' }
}

// 链路判据（双链路重构 D6）：source_mode 为唯一判据，供货模式仅保留展示一版。
export const sourceModeOptions = [
  { label: '自营', value: 'self' },
  { label: '上游转售', value: 'upstream' },
]

export function sourceModeTag(mode: string): { theme: 'primary' | 'warning' | 'default'; text: string } {
  if (mode === 'upstream') return { theme: 'warning', text: '上游转售' }
  if (mode === 'self') return { theme: 'primary', text: '自营' }
  return { theme: 'default', text: '未知' }
}

export const changeTypeOptions = [
  { label: '上架', value: 'publish' },
  { label: '下架', value: 'unpublish' },
  { label: '调价', value: 'price' },
  { label: '创建', value: 'create' },
  { label: '编辑', value: 'update' },
  { label: '删除', value: 'delete' },
]

export function statusTag(status: number): { theme: 'success' | 'danger' | 'default'; text: string } {
  switch (status) {
    case 1:
      return { theme: 'success', text: '上架' }
    case 2:
      return { theme: 'danger', text: '下架' }
    default:
      return { theme: 'default', text: '草稿' }
  }
}

export function priceModelLabel(model: string): string {
  const found = priceModelOptions.find((item) => item.value === model)
  return found ? found.label : model || '—'
}

export function changeTypeLabel(type: string): string {
  const found = changeTypeOptions.find((item) => item.value === type)
  return found ? found.label : type || '—'
}

export function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

export function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// ===== 上游加价规则与规格绑定（T4.2/T4.3/T4.4）=====

/** 上游加价规则类型选项：空串表示不配置（上游改价只更新成本、不改售价） */
export const markupTypeOptions = [
  { label: '不自动改价', value: '' },
  { label: '按成本百分比', value: 'percent' },
  { label: '成本加固定额', value: 'fixed' },
]

export function markupLabel(type: string, value: number): string {
  if (!type) return '未配置（上游改价只更新成本）'
  if (type === 'percent') return `成本 × ${value}%`
  if (type === 'fixed') return `成本 + ¥${Number(value || 0).toFixed(2)}`
  return type
}

/** SKU 出站平台绑定状态（spec_bindings.status）的展示标签 */
export function bindingStatusTag(status?: string): {
  theme: 'success' | 'warning' | 'danger' | 'default'
  text: string
} {
  switch (status) {
    case 'confirmed':
      return { theme: 'success', text: '已确认' }
    case 'auto_mapped':
      return { theme: 'warning', text: '自动映射' }
    case 'stale':
      return { theme: 'danger', text: '已失效' }
    case 'unmapped':
      return { theme: 'warning', text: '未映射' }
    default:
      return { theme: 'default', text: '未绑定' }
  }
}
