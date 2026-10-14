// 操作系统图标识别：把平台下发的 os 字符串归一到有限的图标家族。
//
// 为什么需要一层归一而不是直接匹配字符串：
//   上游的 os 取值形态很杂——财务型上游给的是配置子项原文（"12|CentOS^CentOS-7.6.1810-x64"，
//   带选项值前缀与 ^ 分隔的完整版本名），魔方云给的是镜像名（"CentOS-7.9.2111-x64.qcow2"，
//   可能带虚拟化后缀），导入的模板还可能是 `lin:30,win:50` 这种系统盘取值串。
//   在页面里写一堆 includes 会散落各处且总漏一种，因此集中到本文件，并配一张
//   「图标家族 → 关键字」的表：新增发行版只改这张表。
//
// 不做的两件事：
//   1. 不猜。识别不出就返回通用 linux/unknown 家族，绝不用某家的图标去冒充
//      （把 Alpine 显示成 Ubuntu 比显示成通用企鹅更误导运营）。
//   2. 不裁版本号。显示用的 label 保留原始字符串的信息量（如 CentOS 7.9），
//      图标只表达「哪一家」。

/** 图标家族：决定用官方图形还是首字母章、以及配色。 */
export interface OsIconInfo {
  /** 家族键，同时作为 CSS 类名后缀（os-icon--<key>）。 */
  key: string
  /** 家族展示名（hover 提示用），如 "CentOS"。 */
  label: string
  /** 首字母章文字（1~2 字符）。 */
  mark: string
}

// 关键字表：按「更具体的在前」排列，命中即返回。
// 注意 server/desktop 版本共用一个家族（图标按厂商给，不按发行版分支）。
const OS_FAMILIES: { key: string; label: string; mark: string; patterns: string[] }[] = [
  {
    key: 'windows',
    label: 'Windows',
    mark: 'W',
    patterns: ['windows', 'win32', 'win64', 'win-', 'win2', 'winserver', 'windows_server'],
  },
  { key: 'macos', label: 'macOS', mark: 'M', patterns: ['macos', 'mac os', 'osx', 'os x', 'darwin'] },
  { key: 'android', label: 'Android', mark: 'A', patterns: ['android'] },
  { key: 'almalinux', label: 'AlmaLinux', mark: 'AL', patterns: ['almalinux', 'alma'] },
  { key: 'rockylinux', label: 'Rocky Linux', mark: 'RL', patterns: ['rocky'] },
  { key: 'centos', label: 'CentOS', mark: 'CE', patterns: ['centos', 'cent os'] },
  { key: 'debian', label: 'Debian', mark: 'DE', patterns: ['debian'] },
  { key: 'ubuntu', label: 'Ubuntu', mark: 'UB', patterns: ['ubuntu'] },
  { key: 'fedora', label: 'Fedora', mark: 'F', patterns: ['fedora'] },
  { key: 'opensuse', label: 'openSUSE', mark: 'SUSE', patterns: ['opensuse', 'suse', 'sles'] },
  { key: 'arch', label: 'Arch Linux', mark: 'AR', patterns: ['archlinux', 'arch linux', 'manjaro'] },
  { key: 'alpine', label: 'Alpine Linux', mark: 'AP', patterns: ['alpine'] },
  { key: 'freebsd', label: 'FreeBSD', mark: 'FB', patterns: ['freebsd'] },
  { key: 'openwrt', label: 'OpenWrt', mark: 'OW', patterns: ['openwrt', 'lede'] },
  { key: 'kylin', label: '麒麟', mark: 'KY', patterns: ['kylin', '银河麒麟'] },
  { key: 'uos', label: '统信 UOS', mark: 'UOS', patterns: ['uos', '统信'] },
  { key: 'redhat', label: 'Red Hat', mark: 'RH', patterns: ['redhat', 'red hat', 'rhel'] },
  // 兜底：明确说了是 Linux 家族但认不出发行版。
  { key: 'linux', label: 'Linux', mark: 'LX', patterns: ['linux', 'lin:', 'lin|'] },
]

/** 未识别系统的图标信息（中性灰，不冒充任何厂商）。 */
const UNKNOWN: OsIconInfo = { key: 'unknown', label: '未知系统', mark: '?' }

/**
 * osIconInfo 把实例的 os 原文归一到图标家族。
 *
 * 先做一轮清洗再匹配：上游的 "12|CentOS^CentOS-7.6.1810-x64" 需要先去掉 "^" 之后的
 * 完整版本名与 "值|" 前缀，否则 "12|" 这种数字前缀会干扰个别关键字（如 "win2"）。
 */
export function osIconInfo(os?: string): OsIconInfo {
  const raw = normalizeOsForMatch(os)
  if (!raw) return UNKNOWN
  for (const family of OS_FAMILIES) {
    for (const pattern of family.patterns) {
      if (raw.includes(pattern)) {
        return { key: family.key, label: family.label, mark: family.mark }
      }
    }
  }
  return UNKNOWN
}

/** 归一化用于匹配的字符串：小写、去选项值前缀、去 ^ 后版本名、去镜像后缀。 */
export function normalizeOsForMatch(os?: string): string {
  let v = (os || '').trim().toLowerCase()
  if (!v) return ''
  // 财务型上游：去掉 "^" 之后的详细版本（"CentOS^CentOS-7.6.1810-x64" → "CentOS"）
  if (v.includes('^')) v = v.split('^')[0]
  // 选项值前缀（"12|CentOS" → "CentOS"）；只在竖线前是纯数字/短标识时才切，
  // 避免把 "lin:30,win:50" 这类取值串误伤。
  const pipe = v.indexOf('|')
  if (pipe >= 0 && pipe <= 4) v = v.slice(pipe + 1)
  // 镜像文件后缀（魔方云 os_image 可能带 .qcow2/.raw/.vhd/.img）
  v = v.replace(/\.(qcow2|raw|vhd|img|iso)$/g, '')
  return v.trim()
}

/**
 * osDisplayName 展示用的系统名：在原文基础上做最小清洗，**保留版本信息**。
 *
 * 与 osIconInfo 的分工：图标只表达厂商（哪一家），这里表达「哪个版本」——
 * 运营排查"客户报的兼容性问题"时要看的是 CentOS 7.9 而不是 CentOS。
 *
 * 上游的两种形态要分别处理（实测取值）：
 *   财务型上游："12|CentOS^CentOS-7.6.1810-x64"
 *     → `^` 前是选项展示名（CentOS）、后是完整主机系统名（CentOS-7.6.1810-x64）。
 *       这里取 **`^` 之后** 的那段：它才带版本号，前面那段只是商品选项的短名。
 *   魔方云："CentOS-7.9.2111-x64.qcow2"
 *     → 直接去掉镜像后缀即可（trimImageSuffix 的口径）。
 */
export function osDisplayName(os?: string): string {
  const raw = (os || '').trim()
  if (!raw) return '—'
  let v = raw
  const caret = v.indexOf('^')
  if (caret >= 0) {
    const detail = v.slice(caret + 1).trim()
    // 只有 `^` 后真的有内容时才用它；某些平台会给 "CentOS^" 这种空尾串。
    v = detail || v.slice(0, caret)
  }
  const pipe = v.indexOf('|')
  if (pipe >= 0 && pipe <= 4) v = v.slice(pipe + 1)
  v = v.replace(/\.(qcow2|raw|vhd|img|iso)$/gi, '')
  return v.trim() || raw
}
