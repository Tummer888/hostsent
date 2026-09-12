// 支付中心 · 常量与格式化工具
// 枚举口径与后端 model/payment.go、pkg/payment/contract.go 保持一致。

// ===== 支付场景（pkg/payment Scene*） =====
export const sceneOptions = [
  { label: 'PC 原生（跳转/二维码）', value: 'native' },
  { label: '主扫/被扫（二维码）', value: 'scan' },
  { label: '手机网页 H5', value: 'h5' },
  { label: '公众号内 JSAPI', value: 'jsapi' },
  { label: '小程序', value: 'mini' },
  { label: 'APP SDK', value: 'app' },
]

export function sceneLabel(scene: string): string {
  const found = sceneOptions.find((item) => item.value === scene)
  return found ? found.label : scene || '—'
}

// ===== 渠道模式 =====
export const modeOptions = [
  { label: '接口自动', value: 'api' },
  { label: '线下人工', value: 'manual' },
]

export function modeLabel(mode: string): string {
  const found = modeOptions.find((item) => item.value === mode)
  return found ? found.label : mode || '—'
}

export function modeTheme(mode: string): string {
  switch (mode) {
    case 'api':
      return 'primary'
    case 'manual':
      return 'warning'
    default:
      return 'default'
  }
}

// ===== 渠道状态 =====
export const channelStatusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

export function channelStatusLabel(status: number): string {
  return status === 1 ? '启用' : '停用'
}

export function channelStatusTheme(status: number): string {
  return status === 1 ? 'success' : 'default'
}

// ===== 渠道健康状态 =====
export function healthLabel(status: string): string {
  switch (status) {
    case 'healthy':
      return '正常'
    case 'down':
      return '异常'
    default:
      return '未检测'
  }
}

export function healthTheme(status: string): string {
  switch (status) {
    case 'healthy':
      return 'success'
    case 'down':
      return 'danger'
    default:
      return 'default'
  }
}

// ===== 能力操作 =====
export const operationOptions = [
  { label: '收款下单', value: 'collect' },
  { label: '主动查单', value: 'query' },
  { label: '关单', value: 'close' },
  { label: '渠道退款', value: 'refund' },
  { label: '打款/代付', value: 'payout' },
]

export function operationLabel(op: string): string {
  const found = operationOptions.find((item) => item.value === op)
  return found ? found.label : op
}

// ===== 签名类型 =====
export function signerLabel(signer: string): string {
  switch (signer) {
    case 'none':
      return '无需签名'
    case 'md5':
      return 'MD5'
    case 'rsa2':
      return 'RSA2'
    case 'v3':
      return '微信 v3 证书'
    default:
      return signer || '—'
  }
}

// ===== 支付单状态（model.OrderStatus*） =====
export const orderStatusOptions = [
  { label: '待支付', value: 'pending' },
  { label: '支付中', value: 'paying' },
  { label: '已支付', value: 'paid' },
  { label: '支付失败', value: 'failed' },
  { label: '已关闭', value: 'closed' },
  { label: '退款中', value: 'refunding' },
  { label: '已退款', value: 'refunded' },
]

export function orderStatusLabel(status: string): string {
  const found = orderStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function orderStatusTheme(status: string): string {
  switch (status) {
    case 'paid':
      return 'success'
    case 'paying':
    case 'pending':
      return 'warning'
    case 'failed':
      return 'danger'
    case 'refunding':
    case 'refunded':
      return 'primary'
    default:
      return 'default'
  }
}

// ===== 业务类型（model.BizType*） =====
export const bizTypeOptions = [
  { label: '余额充值', value: 'recharge' },
  { label: '账单支付', value: 'bill' },
  { label: '订单支付', value: 'order' },
]

export function bizTypeLabel(bizType: string): string {
  const found = bizTypeOptions.find((item) => item.value === bizType)
  return found ? found.label : bizType || '—'
}

// ===== 退款状态（model.RefundStatus*） =====
export const refundStatusOptions = [
  { label: '退款中', value: 'pending' },
  { label: '退款成功', value: 'success' },
  { label: '退款失败', value: 'failed' },
  { label: '已驳回', value: 'rejected' },
]

export function refundStatusLabel(status: string): string {
  const found = refundStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function refundStatusTheme(status: string): string {
  switch (status) {
    case 'success':
      return 'success'
    case 'pending':
      return 'warning'
    case 'failed':
    case 'rejected':
      return 'danger'
    default:
      return 'default'
  }
}

// ===== 打款状态（model.PayoutStatus*） =====
export const payoutStatusOptions = [
  { label: '待打款', value: 'pending' },
  { label: '打款中', value: 'paying' },
  { label: '已打款', value: 'paid' },
  { label: '打款失败', value: 'failed' },
]

export function payoutStatusLabel(status: string): string {
  const found = payoutStatusOptions.find((item) => item.value === status)
  return found ? found.label : status || '—'
}

export function payoutStatusTheme(status: string): string {
  switch (status) {
    case 'paid':
      return 'success'
    case 'pending':
    case 'paying':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'default'
  }
}

export function payoutModeLabel(mode: string): string {
  switch (mode) {
    case 'api':
      return '接口打款'
    case 'manual':
      return '人工打款'
    default:
      return mode || '—'
  }
}

// ===== 回调处理状态 =====
export function callbackHandleLabel(status: string): string {
  switch (status) {
    case 'ok':
      return '已处理'
    case 'ignored':
      return '已忽略'
    case 'duplicate':
      return '重复回调'
    case 'error':
      return '处理失败'
    default:
      return status || '—'
  }
}

export function callbackHandleTheme(status: string): string {
  switch (status) {
    case 'ok':
      return 'success'
    case 'duplicate':
    case 'ignored':
      return 'warning'
    case 'error':
      return 'danger'
    default:
      return 'default'
  }
}

// ===== 对账状态（后端 recon_service: matched/suspicious） =====
export function reconStatusLabel(status: string): string {
  switch (status) {
    case 'matched':
      return '账实一致'
    case 'diff':
      return '存在差异'
    case 'suspicious':
      return '疑似异常'
    default:
      return status || '—'
  }
}

export function reconStatusTheme(status: string): string {
  switch (status) {
    case 'matched':
      return 'success'
    case 'diff':
    case 'suspicious':
      return 'danger'
    default:
      return 'default'
  }
}

// ===== 金额 =====
// 支付中心金额一律「分」进「元」出：列表展示用 formatFenYuan。
export function formatFenYuan(fen: number): string {
  return (Number(fen || 0) / 100).toFixed(2)
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

export function toDateString(value: unknown): string {
  if (!value) return ''
  if (value instanceof Date && !Number.isNaN(value.getTime())) {
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}`
  }
  const s = String(value)
  const matched = s.match(/^\d{4}-\d{2}-\d{2}/)
  return matched ? matched[0] : s
}
