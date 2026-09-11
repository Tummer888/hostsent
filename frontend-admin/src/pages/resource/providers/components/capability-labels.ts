// 能力描述符展示文案：把后端 upstream 包的英文枚举翻成后台可读标签。
// 后端常量见 backend/internal/pkg/upstream/capability.go。

export const OPERATION_LABELS: Record<string, string> = {
  provision: '开通',
  renew: '续费',
  start: '开机',
  stop: '关机',
  restart: '重启',
  vnc: 'VNC 控制台',
  resize: '变配',
  reinstall: '重装系统',
  destroy: '销毁',
  snapshot: '快照',
}

export const SYNC_SCOPE_LABELS: Record<string, string> = {
  catalog: '商品目录',
  price: '价格',
  pool: '资源池',
  region: '区域',
  image: '镜像',
  instance: '实例',
  stock: '库存',
}

export const SIGNER_LABELS: Record<string, string> = {
  none: '无签名',
  bearer: 'Bearer Token',
  form: '表单密钥',
  token: '请求头令牌',
  ak_sk: 'AK/SK 签名',
  tc3: '腾讯云 TC3',
}

export const RENEW_MODE_LABELS: Record<string, string> = {
  order: '上游下单续费',
  direct: '直接延期',
  none: '不支持',
}

export const DESTROY_MODE_LABELS: Record<string, string> = {
  immediate: '立即销毁',
  delayed: '延迟/回收站',
  unsupported: '不支持',
}

export const BILLING_CYCLE_LABELS: Record<string, string> = {
  hour: '小时',
  day: '天',
  month: '月',
  year: '年',
}

export const KIND_LABELS: Record<string, string> = {
  upstream: '上游转售',
  compute: '自营算力',
}

export function labelOf(map: Record<string, string>, key: string): string {
  return map[key] || key || '—'
}
