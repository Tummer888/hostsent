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

// 链路判据（双链路重构 D6）：source_mode 为唯一判据（P8 起 provision_mode 列已删除）。
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

// ---------- 计费周期标签（上游对接商品的付费周期展示） ----------

const billingCycleLabels: Record<string, string> = {
  hourly: '按小时',
  daily: '按天',
  monthly: '月付',
  quarterly: '季付',
  semiannually: '半年付',
  annually: '年付',
  biennially: '两年付',
  triennially: '三年付',
  onetime: '一次性',
  free: '免费',
  ontrial: '试用',
}

export function billingCycleLabel(cycle: string): string {
  return billingCycleLabels[cycle] || cycle || '—'
}

/** 周期顺序（展示用：短周期在前，一次性在最后） */
export const billingCycleOrder = [
  'monthly',
  'quarterly',
  'semiannually',
  'annually',
  'biennially',
  'triennially',
  'onetime',
]

/**
 * 平台参数中文名（魔方云 /clouds 写参数口径）。
 *
 * 为什么需要它：配置项与平台映射里只带英文字段名（cpu / ip_num / nat_acl_limit…），
 * 运营看到的是裸 key，不知道是什么。列表与建品页统一从这里取中文名，
 * 避免两处各写一份而互相漂移。
 */
export const platformFieldLabels: Record<string, string> = {
  // 基础配置
  cpu: 'CPU（核）',
  memory: '内存（MB）',
  os: '操作系统镜像',
  store: '系统盘存储',
  system_disk_size: '系统盘容量（GB）',
  type: '虚拟化类型',
  // 位置与调度
  area: '数据中心',
  node: '节点',
  node_group: '节点分组',
  node_priority: '节点优先级',
  ip_group: 'IP 组',
  // 网络
  bw: '带宽（Mbps）',
  in_bw: '上行带宽（Mbps）',
  out_bw: '下行带宽（Mbps）',
  network_type: '网络类型',
  vpc: 'VPC 网络',
  vpc_name: 'VPC 名称',
  flow_way: '流量方向',
  flow_limit: '流量上限（GB）',
  traffic_type: '流量计费方式',
  traffic_quota: '流量配额',
  traffic_bill_type: '流量结算周期',
  reset_flow_day: '流量重置日',
  ip_num: '独立 IP 数量',
  ipv6_num: 'IPv6 数量',
  port: '端口',
  niccard: '网卡型号',
  IP_MACBond: 'IP-MAC 绑定',
  link_clone: '链路克隆',
  // NAT（机型参数）
  nat_acl_limit: 'NAT 端口转发数',
  nat_web_limit: '共享建站数',
  // 高级
  snap_num: '快照数量',
  backup_num: '备份数量',
  gpu_num: 'GPU 数量',
  cpu_limit: 'CPU 限制（%）',
  advanced_cpu: '高级 CPU 型号',
  advanced_bw: '高级带宽',
  cpu_model: 'CPU 型号',
  rid: '资源包',
  resource_package: '资源包',
  system_disk_io_limit: '系统盘性能',
  data_disk_io_limit: '数据盘性能',
  data_disk_size: '数据盘容量',
  other_data_disk: '附加数据盘',
}

/** 平台参数中文名；未知键返回原 key（不隐藏信息，运营仍能对上平台文档）。 */
export function platformFieldLabel(key: string): string {
  return platformFieldLabels[key] || key
}

/** 把「周期 → 金额」映射按固定顺序拍平成数组（未知周期排在末尾，按字典序） */
export function cycleEntries(map?: Record<string, number> | null): { cycle: string; label: string; amount: number }[] {
  if (!map) return []
  const keys = Object.keys(map)
  const known = billingCycleOrder.filter((c) => keys.includes(c))
  const rest = keys.filter((c) => !billingCycleOrder.includes(c)).sort()
  return [...known, ...rest].map((cycle) => ({ cycle, label: billingCycleLabel(cycle), amount: map[cycle] }))
}
