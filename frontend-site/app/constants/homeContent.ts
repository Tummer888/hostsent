/**
 * 首页版式骨架文案。
 *
 * 这里只放两类内容：
 *  1. 指向站点内**真实存在**的页面或锚点（`/products`、`/#contact` 等）；
 *  2. 每一条说明都对应平台**已经在跑**的能力。
 *
 * 为什么强调这一点：本文件曾集中放了一批「搭结构」阶段自拟的营销文案
 *（注册送代金券、首单立减 30%、续费同价、开通失败自动退款、DDoS 防护…）。
 * 这些能力在代码与数据库里都不存在 —— `promotions` / `coupons` 表为空，
 * `pricing` 的 PromotionRule 依赖装配为空实现，开通失败链路上只有重试与转人工、
 * 没有退款分支。官网上写着做不到的承诺，比少写一句更伤可信度：
 * 用户拿着截图来对质时无法解释。
 *
 * 品牌名、联系方式、Hero 主副标题、产品优势四张卡等**可变文案**不在这里：
 * 它们由 `system_configs`（group=`site`）下发，经 `useSiteContent()` 读取，
 * 运营在「系统配置 → 站点品牌 / 首页配置」里改，不需要重新构建（白标要求，doc80 §5）。
 * 本文件保留的是版式骨架（几个标签、几张卡），改动频率低且与具体品牌无关。
 */

/*
 * 首页内的板块锚点。导航标签栏与 Hero 卡片里的快捷入口都从这里挑，
 * 每一条都能滚到首页对应的区块（或站内真实页面）。
 */
export interface SiteAnchor {
  /** 区块名，同时作为标签文案 */
  label: string
  /** 首页锚点（`/#features`）或站内页面（`/products`） */
  to: string
}

/** Hero 左栏标签：首项渲染为高亮态。 */
export const HERO_TABS: SiteAnchor[] = [
  { label: '云服务器', to: '/products' },
  { label: '自助开通', to: '/#features' },
  { label: '服务与保障', to: '/#partners' },
  { label: '联系与支持', to: '/#contact' },
]

export interface HeroHighlight {
  icon: string
  title: string
  desc: string
}

/** Banner 右栏三栏能力面板：只写客户关心的选购/开通/续费，不写内部链路。 */
export const HERO_HIGHLIGHTS: HeroHighlight[] = [
  {
    icon: 'bolt',
    title: '支付后自动处理开通',
    desc: '支付完成后按订单自动处理开通，进度可在订单详情查看。',
  },
  {
    icon: 'refresh',
    title: '订单状态可查询',
    desc: '下单、开通与续费进度都能在订单详情里查到。',
  },
  {
    icon: 'layers',
    title: '周期计费更灵活',
    desc: '支持月付、季付、年付等周期，可自助续费或开启自动续费。',
  },
]

/** Banner 下方左侧卡片的标题：accent 走品牌色，title 为深色主体。 */
export const HERO_SHELF_HEAD = {
  accent: '全流程',
  title: '在线完成选购与开通',
}

/** 左侧卡片内的两列快捷入口。 */
export const HERO_SHELF_LINKS: SiteAnchor[] = [
  { label: '云服务器', to: '/products' },
  { label: '周期计费', to: '/#pricing' },
  { label: '自助开通', to: '/#features' },
  { label: '服务与保障', to: '/#partners' },
  { label: '用户中心与文档', to: '/#console' },
  { label: '服务与支持', to: '/#contact' },
]

/**
 * 计费与优惠货架（首页 `#pricing` 区块）。
 *
 * 每张卡对应一个**平台真实提供**的机制：
 *  - 周期价格：`product_prices.cycle` 支持月付 / 季付 / 年付等周期，价格按周期定价；
 *  - 邀请返利：推广返现体系已上线（返现比例在后台「推广设置」里配，可提现或转入余额）；
 *  - 自动续费：实例可开启自动续费，到期前有提醒，开关在「用户中心 → 续费管理」；
 *  - 批量采购：走售后联系，不承诺固定折扣（折扣按用户组策略配置，不是面向所有人的活动）。
 *
 * 此前的五张卡（代金券 / 首单立减 / 续费同价 / 企业补贴 / 邀请返利）里，
 * 只有「邀请返利」有真实后端；其余四张都渲染成「即将上线」，等于货架一半是空的。
 */
export interface PricingCard {
  /** 1–5，对应 --site-art-N 渐变 */
  variant: number
  tag: string
  title: string
  desc: string
  /** 站内页面或锚点；与 consolePath 二选一 */
  to?: string
  /** 用户中心内的相对路径，配合 publicConfig.consoleUrl 拼接；未配控制台地址时卡片不可点 */
  consolePath?: string
}

export const PRICING_CARDS: PricingCard[] = [
  {
    variant: 1,
    tag: '计费',
    title: '周期计价',
    desc: '同一产品可选月付、季付、年付等周期，价格按周期展示。',
    to: '/products',
  },
  {
    variant: 2,
    tag: '推广',
    title: '邀请返利',
    desc: '邀请好友注册并消费可得返现，累计后可提现或转入余额，具体比例以活动页为准。',
    consolePath: '/referral/overview',
  },
  {
    variant: 4,
    tag: '续费',
    title: '到期提醒与自动续费',
    desc: '到期前提醒，可自助续费或开启自动续费，减少因到期导致的停机。',
    consolePath: '/cloud/renewals',
  },
  {
    variant: 3,
    tag: '商务',
    title: '批量采购咨询',
    desc: '批量采购与长期合约可联系商务沟通，具体方案与折扣以实际报价为准。',
    to: '/#contact',
  },
]

export interface PartnerItem {
  /** 占位标记的图形变体，1–5 */
  variant: number
  name: string
  /** 该主体与本平台的关系，一句话说明；避免让访客以为都是官方合作伙伴。 */
  note: string
}

/**
 * 服务保障清单（首页 `#partners` 区块）。
 *
 * 原为"上游对接与运行组件"：列的是供货渠道与内部技术栈（魔方云/魔方财务、
 * PostgreSQL/Redis/Docker/Go/Nuxt/Vue）。这些既不是客户关心的内容，
 * 也把供货结构与技术实现直接摊给了访客 —— 换成客户真正在意的服务点。
 *
 * 每项都是平台真实具备的能力，不做无依据承诺（广告法）。
 */
export const PARTNERS: PartnerItem[] = [
  { variant: 1, name: '在线选购开通', note: '自助选配下单' },
  { variant: 2, name: '周期计费', note: '月付 / 季付 / 年付' },
  { variant: 3, name: '自助续费', note: '支持自动续费' },
  { variant: 4, name: '到期提醒', note: '到期前通知' },
  { variant: 1, name: '余额支付', note: '明细可查询' },
  { variant: 2, name: '工单支持', note: '按优先级响应' },
  { variant: 3, name: '订单可查', note: '状态随时查询' },
  { variant: 4, name: '自助运维', note: '重启 / 重装 / 远程连接' },
]

export interface QuickEntry {
  icon: string
  label: string
  /** 控制台内的相对路径，配合 publicConfig.consoleUrl 拼接 */
  path: string
}

/** Featured 深色卡里的控制台快捷入口；未配置 consoleUrl 时整卡降级为说明文案。 */
export const CONSOLE_ENTRIES: QuickEntry[] = [
  { icon: 'server', label: '我的资源', path: '/cloud/instances' },
  { icon: 'cart', label: '订单与账单', path: '/order' },
  { icon: 'support', label: '工单支持', path: '/support/tickets' },
  { icon: 'user', label: '账号资料', path: '/profile' },
]

export interface LinkCard {
  icon: string
  title: string
  desc: string
  badge?: string
  /** 站内页面或锚点。四张卡都要能点得动 —— 不放没有目的地的入口。 */
  to: string
}

/** 「上手与支持」区块右侧的四张链接卡，全部指向门户上真实存在的页面。 */
export const FEATURED_LINKS: LinkCard[] = [
  { icon: 'book', title: '帮助文档', desc: '了解产品能力与开通流程', to: '/help' },
  { icon: 'sparkle', title: '产品动态', desc: '版本发布与功能变更记录', to: '/news' },
  { icon: 'time', title: '服务公告', desc: '维护窗口、故障说明与重要通知', to: '/announcements' },
  { icon: 'secured', title: '条款与隐私', desc: '服务条款、责任边界与信息处理规则', to: '/terms' },
]

export interface ResourceCard {
  variant: number
  icon: string
  artLabel: string
  title: string
  desc: string
  to: string
}

export const RESOURCES: ResourceCard[] = [
  {
    variant: 1,
    icon: 'sparkle',
    artLabel: '产品更新',
    title: '产品更新',
    desc: '新功能上线与版本更新速览。',
    to: '/news',
  },
  {
    variant: 3,
    icon: 'book',
    artLabel: '使用文档',
    title: '使用文档',
    desc: '从选购到开通、从计费到排障的完整指引。',
    to: '/help',
  },
  {
    variant: 4,
    icon: 'time',
    artLabel: '服务公告',
    title: '服务公告',
    desc: '维护窗口、故障说明与重要通知。',
    to: '/announcements',
  },
  {
    variant: 5,
    icon: 'secured',
    artLabel: '条款政策',
    title: '用户条款与隐私政策',
    desc: '服务条款、责任边界与个人信息处理规则。',
    to: '/terms',
  },
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
    title: '选购云服务器',
    desc: '浏览全部可开通的云服务器与各周期价格，在线下单。',
    actionLabel: '去选购',
    to: '/products',
  },
  {
    // 工单入口在登录后的用户中心里（`/support/tickets`），门户上没有免登录提单页，
    // 因此这里导向「联系与支持」区块（含客服电话与邮箱），而不是造一个假提单入口。
    icon: 'ticket',
    title: '联系与支持',
    desc: '售前咨询与售后问题都可联系客服；已注册用户可在用户中心提交工单并按优先级跟进。',
    actionLabel: '查看联系方式',
    to: '/#contact',
  },
]

/**
 * 快捷入口右侧的服务说明小卡片。
 *
 * 三条都能在用户中心里找到对应的事实：
 *  - 支付完成后按订单处理开通，进度在订单详情可见；
 *  - 开通与续费的记录可查询；
 *  - 工单按优先级响应。
 * 措辞用"服务说明"而不是"服务承诺"：后者容易被理解为对结果的保证
 * （广告法对绝对化承诺有限制），这里说的是可查证的服务方式。
 */
export const SERVICE_PROMISE = {
  title: '服务说明',
  desc: '支付完成后按订单处理开通，进度可在订单详情查看。',
  points: ['在线自助续费', '订单与实例记录可查询', '工单按优先级响应'],
}
