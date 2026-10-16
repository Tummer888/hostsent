// 能力描述符展示文案：把后端 upstream 包的英文枚举翻成后台可读标签。
// 后端常量见 backend/internal/pkg/upstream/capability.go。
//
// 覆盖范围必须与后端常量表逐一对齐：漏掉一个取值，能力矩阵就会把英文键名
// 原样渲染给运营（实测 suspend / reset_password / bandwidth / monthly 等
// 九个操作与六个计费周期都曾以英文裸奔）。

export const OPERATION_LABELS: Record<string, string> = {
  provision: '开通',
  renew: '续费',
  start: '开机',
  stop: '关机',
  restart: '重启',
  vnc: 'VNC 控制台',
  resize: '变配',
  reinstall: '重装系统',
  reset_password: '重置密码',
  rescue: '救援系统',
  destroy: '销毁',
  snapshot: '快照',
  suspend: '暂停服务',
  unsuspend: '恢复服务',
  bandwidth: '调整带宽',
  add_ip: '增加 IP',
  attach_disk: '挂载数据盘',
  metrics: '性能监控',
  nat_access: 'NAT 访问地址',
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

// 计费周期：适配器各自用自己的命名（魔方云 month/year/hour/day，
// 魔方财务 monthly/quarterly/... —— 后者是上游订单周期的原词）。
// 两套都要覆盖，否则财务型渠道的周期会原样显示英文。
export const BILLING_CYCLE_LABELS: Record<string, string> = {
  hour: '小时',
  day: '天',
  week: '周',
  month: '月',
  year: '年',
  monthly: '月付',
  quarterly: '季付',
  semiannually: '半年付',
  annually: '年付',
  biennially: '两年付',
  triennially: '三年付',
}

export const KIND_LABELS: Record<string, string> = {
  upstream: '上游转售',
  compute: '自营算力',
}

export function labelOf(map: Record<string, string>, key: string): string {
  return map[key] || key || '—'
}
