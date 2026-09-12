// 支付渠道能力标签映射（驱动能力矩阵展示）
export const SCENE_LABELS: Record<string, string> = {
  native: 'PC 原生',
  scan: '主扫/被扫',
  h5: '手机网页',
  jsapi: '公众号内',
  mini: '小程序',
  app: 'APP SDK',
}

export const OPERATION_LABELS: Record<string, string> = {
  collect: '收款下单',
  query: '主动查单',
  close: '关单',
  refund: '渠道退款',
  payout: '打款/代付',
}

export const SIGNER_LABELS: Record<string, string> = {
  none: '无需签名',
  md5: 'MD5',
  rsa2: 'RSA2',
  v3: '微信 v3 证书',
}

export const CERT_MODE_LABELS: Record<string, string> = {
  '': '仅密钥',
  key: '仅密钥',
  cert: '需上传证书',
}

export const MODE_LABELS: Record<string, string> = {
  api: '接口自动',
  manual: '线下人工',
}

export function labelOf(map: Record<string, string>, key?: string): string {
  if (!key) return '—'
  return map[key] ?? key
}
