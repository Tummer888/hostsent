/**
 * 规格数值展示：统一 CPU / 内存 / 磁盘 / 带宽的单位与精度。
 *
 * 单位口径（与后端一致）：
 *   - 商品/实例内存：**MB** 存储（8192 = 8GB），见 pkg/model/standard_product.go；
 *   - 资源池容量：**GB** 存储（15 = 15GB），见 pkg/upstream/provider.go 的
 *     StandardPool 字段注释与 dcimcloud 的 parseSizeGB；
 *   - 磁盘：**GB** 存储（40 = 40GB）；
 *   - 带宽：**Mbps**。
 *
 * 两套内存口径必须分开实现（实测故障）：商品链路的内存是 MB，
 * 资源池链路是 GB。此前资源池页把 GB 值交给按 MB 实现的 formatMemory，
 * 15GB 的宿主机显示成「15 MB」，比真实值小三个数量级。
 *
 * 为什么集中一处（另一处实测故障）：资源域四个页面各自复制了一份 formatMemory，
 * 实现是"≥1024 就当 TB" —— 那是磁盘口径，用在内存上会把 8GB（8192MB）
 * 显示成「8.0T」。用户看到的就是"内存 8T"这种不可能存在的规格。
 */

/**
 * 内存展示：入参为 MB（**商品/实例**链路的存储口径）。
 * 1024MB 以上按 GB 显示（内存都是 2 的幂，取 1024 而非 1000）。
 */
export function formatMemory(mb: number): string {
  const value = Number(mb || 0)
  if (!Number.isFinite(value) || value <= 0) return '—'
  if (value >= 1024) {
    const gb = value / 1024
    return `${Number.isInteger(gb) ? gb : gb.toFixed(1)} GB`
  }
  return `${Math.round(value)} MB`
}

/**
 * 资源池内存展示：入参为 **GB**（资源池链路的存储口径，不是 MB）。
 *
 * 与 formatMemory 的区别只在口径，不在展示：池容量本来就是 GB 量级，
 * 不需要 MB→GB 的换算。1024GB 以上按 TB 显示（大宿主机/集群汇总会到 TB）。
 */
export function formatPoolMemory(gb: number): string {
  const value = Number(gb || 0)
  if (!Number.isFinite(value) || value <= 0) return '—'
  if (value >= 1024) {
    const tb = value / 1024
    return `${Number.isInteger(tb) ? tb : tb.toFixed(1)} TB`
  }
  return `${Math.round(value)} GB`
}

/**
 * 磁盘展示：入参为 GB（后端存储口径）。
 * 1024GB 以上按 TB 显示（磁盘容量确实会到 TB 级，与内存不同）。
 */
export function formatDisk(gb: number): string {
  const value = Number(gb || 0)
  if (!Number.isFinite(value) || value <= 0) return '—'
  if (value >= 1024) {
    const tb = value / 1024
    return `${Number.isInteger(tb) ? tb : tb.toFixed(1)} TB`
  }
  return `${Math.round(value)} GB`
}

/** 带宽展示：入参为 Mbps。 */
export function formatBandwidth(mbps: number): string {
  const value = Number(mbps || 0)
  if (!Number.isFinite(value) || value <= 0) return '—'
  return `${Math.round(value)} Mbps`
}

/**
 * 组合成一行规格文本：CPU / 内存 / 磁盘 / 带宽。
 * 缺失项自动跳过（上游未下发时不显示误导性的 0）。
 */
export function formatSpecLine(spec: { cpu?: number; memory?: number; disk?: number; bandwidth?: number }): string {
  const parts: string[] = []
  if (spec.cpu && spec.cpu > 0) parts.push(`${spec.cpu}核`)
  const mem = formatMemory(spec.memory || 0)
  if (mem !== '—') parts.push(mem)
  const disk = formatDisk(spec.disk || 0)
  if (disk !== '—') parts.push(disk)
  const bw = formatBandwidth(spec.bandwidth || 0)
  if (bw !== '—') parts.push(bw)
  return parts.length ? parts.join(' / ') : '规格未同步'
}
