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
