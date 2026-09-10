/**
 * 首页占位文案。
 *
 * ⚠️ 这些是「搭结构」阶段自拟的文案，**不是最终内容**。
 *
 * 按架构设计 §5（品牌与站点配置层），站点文案应由 `system_configs`（group=`site`）下发、
 * 组件经 `useSiteContent()` 读取。Phase 3 落地后台「品牌与站点」配置页后，
 * 本文件整体并入配置默认值。现在集中在这里，是为了迁移时只动一个文件，
 * 而不是把几十段文案散在组件里。
 *
 * `pending: true` 表示该入口对应的页面/接口尚不存在：组件会渲染为带「即将上线」标记的
 * 非可点卡片，而不是造一个点进去 404 的假链接。
 */

export interface HeroTab {
  label: string
  /** 真实可跳转的目的地。标签栏不放未实现的入口——点了就该到得了。 */
  to: string
}

/**
 * Banner 左侧竖向标签栏（对齐参考图左侧那一列）。
 * 只放真实存在的页面/锚点；首项渲染为高亮态。
 */
export const HERO_TABS: HeroTab[] = [
  { label: '云主机与轻量应用', to: '/products' },
  { label: '自动化交付', to: '/#features' },
  { label: '白标转售', to: '/#features' },
  { label: '优惠活动', to: '/#promos' },
  { label: '控制台与文档', to: '/#console' },
]

export interface HeroHighlight {
  icon: string
  title: string
  desc: string
}

/** Banner 下方右侧的三栏能力面板。 */
export const HERO_HIGHLIGHTS: HeroHighlight[] = [
  {
    icon: 'bolt',
    title: '分钟级上线',
    desc: '支付后由上游接口自动开通，不需要人工介入。',
  },
  {
    icon: 'refresh',
    title: '订单即交付',
    desc: '下单、开通、续费全程留痕，开通失败自动退款。',
  },
  {
    icon: 'layers',
    title: '一套系统多品牌',
    desc: '品牌名、主题色、联系方式可配，支持白标对外销售。',
  },
]

/** Banner 下方左侧卡片的标题：accent 走品牌色，title 为深色主体。 */
export const HERO_SHELF_HEAD = {
  accent: '全流程',
  title: '从选购到自动开通',
}

export interface HeroShelfLink {
  label: string
  to: string
}

/** 左侧卡片内的两列快捷入口。 */
export const HERO_SHELF_LINKS: HeroShelfLink[] = [
  { label: '弹性云主机', to: '/products' },
  { label: '轻量应用服务器', to: '/products' },
  { label: '自动化交付', to: '/#features' },
  { label: '白标转售', to: '/#features' },
  { label: '优惠活动', to: '/#promos' },
  { label: '服务与支持', to: '/#contact' },
]

export interface PromoItem {
  /** 1–5，对应 --site-art-N 渐变 */
  variant: number
  tag: string
  title: string
  desc: string
  to?: string
  pending?: boolean
}

export const PROMOS: PromoItem[] = [
  { variant: 1, tag: '新用户', title: '注册即送代金券', desc: '完成实名认证后自动发放，可直接抵扣首单。', pending: true },
  { variant: 2, tag: '首购', title: '首单立减 30%', desc: '云主机与轻量应用服务器首购享折扣。', pending: true },
  { variant: 4, tag: '续费', title: '续费同价不涨价', desc: '长期续费保持原价，预算更好规划。', pending: true },
  { variant: 3, tag: '企业', title: '企业上云补贴', desc: '批量采购与年度合约可申请专属报价。', to: '/#contact' },
  { variant: 5, tag: '邀请', title: '邀请返利', desc: '邀请好友注册并消费，双方均可获得返利。', pending: true },
]

export interface PartnerItem {
  /** 占位标记的图形变体，1–5 */
  variant: number
  name: string
}

/**
 * 上游与生态占位清单。
 *
 * 注：这里用「文字 + 几何占位标记」表示，不使用任何第三方商标图形。
 * 魔方财务 / 魔方云 是本项目真实对接的上游；其余为通用技术栈标签，
 * 上线前应按实际接入情况核对替换，避免做成不实宣传。
 */
export const PARTNERS: PartnerItem[] = [
  { variant: 1, name: '魔方财务' },
  { variant: 2, name: '魔方云' },
  { variant: 3, name: 'Kubernetes' },
  { variant: 4, name: 'Docker' },
  { variant: 1, name: 'Prometheus' },
  { variant: 2, name: 'Grafana' },
  { variant: 3, name: 'MySQL' },
  { variant: 4, name: 'Redis' },
  { variant: 1, name: 'Nginx' },
  { variant: 2, name: '对象存储' },
]

export interface QuickEntry {
  icon: string
  label: string
  /** 控制台内的相对路径，配合 publicConfig.consoleUrl 拼接 */
  path: string
}

/** Featured 深色卡里的控制台快捷入口；未配置 consoleUrl 时整卡降级为说明文案。 */
export const CONSOLE_ENTRIES: QuickEntry[] = [
  { icon: 'server', label: '我的资源', path: '/instances' },
  { icon: 'cart', label: '订单与账单', path: '/orders' },
  { icon: 'support', label: '工单支持', path: '/tickets' },
  { icon: 'code', label: 'API 密钥', path: '/settings/api' },
]

export interface LinkCard {
  icon: string
  title: string
  desc: string
  badge?: string
  to?: string
  pending?: boolean
}

export const FEATURED_LINKS: LinkCard[] = [
  { icon: 'book', title: '产品文档', desc: '了解产品能力与开通流程', badge: '新', pending: true },
  { icon: 'code', title: '开发者 API', desc: '用接口自动化你的业务流程', pending: true },
  { icon: 'pulse', title: '服务状态', desc: '查看各区域与服务可用性', pending: true },
  { icon: 'sparkle', title: '更新日志', desc: '版本发布与功能变更记录', pending: true },
]

export interface ResourceCard {
  variant: number
  icon: string
  artLabel: string
  title: string
  desc: string
  to?: string
  pending?: boolean
}

export const RESOURCES: ResourceCard[] = [
  { variant: 1, icon: 'sparkle', artLabel: '产品更新', title: '产品更新', desc: '新功能上线与能力迭代速览。', pending: true },
  { variant: 3, icon: 'book', artLabel: '技术分享', title: '技术分享', desc: '上云实践、性能调优与踩坑记录。', pending: true },
  { variant: 4, icon: 'layers', artLabel: '解决方案', title: '解决方案', desc: '建站、电商、企业应用的推荐架构。', pending: true },
  { variant: 5, icon: 'users', artLabel: '客户案例', title: '客户案例', desc: '看其他团队如何在本平台落地。', pending: true },
]

export interface QuickAction {
  icon: string
  title: string
  desc: string
  actionLabel: string
  to: string
}

export const QUICK_ACTIONS: QuickAction[] = [
  {
    icon: 'cart',
    title: '立即选购',
    desc: '浏览全部可开通的云产品与规格，在线下单后自动交付。',
    actionLabel: '去选购',
    to: '/products',
  },
  {
    icon: 'ticket',
    title: '提交工单',
    desc: '7×24 技术支持，遇到问题随时提单，按优先级响应。',
    actionLabel: '联系支持',
    to: '/#contact',
  },
]

/** 快捷入口右侧的服务承诺小卡片 */
export const SERVICE_PROMISE = {
  title: '服务承诺',
  desc: '订单支付后自动开通；上游同步异常时人工兜底，并全程记录审计日志。',
  points: ['7×24 工单响应', '开通失败自动退款', '操作留痕可追溯'],
}
