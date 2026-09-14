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
  { label: '云主机与轻量应用', to: '/products' },
  { label: '自动化交付', to: '/#features' },
  { label: '上游与生态', to: '/#partners' },
  { label: '联系与支持', to: '/#contact' },
]

export interface HeroHighlight {
  icon: string
  title: string
  desc: string
}

/** Banner 右栏三栏能力面板。每条都对应代码里真实存在的链路。 */
export const HERO_HIGHLIGHTS: HeroHighlight[] = [
  {
    icon: 'bolt',
    title: '支付后自动开通',
    desc: '订单支付成功即投递开通任务，由上游接口创建实例，不需要人工介入。',
  },
  {
    icon: 'refresh',
    title: '订单即交付',
    desc: '下单、开通、续费全程留痕；上游接口异常时自动重试并转入人工处理队列。',
  },
  {
    icon: 'layers',
    title: '一套系统多品牌',
    desc: '品牌名、主题色、联系方式与页脚栏目均可后台配置，同一份构建产物换配置即换品牌。',
  },
]

/** Banner 下方左侧卡片的标题：accent 走品牌色，title 为深色主体。 */
export const HERO_SHELF_HEAD = {
  accent: '全流程',
  title: '从选购到自动开通',
}

/** 左侧卡片内的两列快捷入口。 */
export const HERO_SHELF_LINKS: SiteAnchor[] = [
  { label: '弹性云主机', to: '/products' },
  { label: '轻量应用服务器', to: '/products' },
  { label: '自动化交付', to: '/#features' },
  { label: '上游与生态', to: '/#partners' },
  { label: '控制台与文档', to: '/#console' },
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
    desc: '同一产品可选月付、季付、年付等周期，各周期价格在后台单独维护。',
    to: '/products',
  },
  {
    variant: 2,
    tag: '推广',
    title: '邀请返利',
    desc: '邀请好友注册并消费可得返现，返现比例由后台配置，累计后可提现或转入余额。',
    consolePath: '/referral/overview',
  },
  {
    variant: 4,
    tag: '续费',
    title: '到期提醒与自动续费',
    desc: '到期前提醒，可自助续费或开启自动续费从余额扣款，避免业务中断。',
    consolePath: '/cloud/renewals',
  },
  {
    variant: 3,
    tag: '商务',
    title: '批量采购咨询',
    desc: '批量采购与长期合约可联系商务沟通；大客户折扣按用户组策略单独配置。',
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
 * 上游对接与运行组件清单（首页 `#partners` 区块）。
 *
 * 无真实素材，用「文字 + 几何占位标记」表示，**不使用任何第三方商标图形**；
 * 每项都标出它与平台的关系（上游供应商 / 平台自身使用的技术组件），
 * 避免访客把「平台自己用了 Redis」误读成「Redis 是我们的合作客户」。
 *
 * 此前这里混排了 Kubernetes / Prometheus / Grafana / MySQL / Nginx / 对象存储 ——
 * 平台既没接这些上游、运行时也没用（go.mod 里没有 mysql 驱动，指标是自研的
 * 文本格式计数器，没有 Prometheus server），属于无中生有的合作方。
 */
export const PARTNERS: PartnerItem[] = [
  { variant: 1, name: '魔方财务', note: '上游转售渠道（已对接）' },
  { variant: 2, name: '魔方云', note: '上游资源池（已对接）' },
  { variant: 3, name: 'PostgreSQL', note: '业务数据库' },
  { variant: 4, name: 'Redis', note: '缓存与队列' },
  { variant: 1, name: 'Docker', note: '容器化部署' },
  { variant: 2, name: 'Go', note: '服务端语言' },
  { variant: 3, name: 'Nuxt', note: '官网 SSR 框架' },
  { variant: 4, name: 'Vue', note: '控制台前端框架' },
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
  { variant: 1, icon: 'sparkle', artLabel: '产品更新', title: '产品更新', desc: '新功能上线与能力迭代速览。', to: '/news' },
  { variant: 3, icon: 'book', artLabel: '使用文档', title: '使用文档', desc: '从选购到开通、从计费到排障的完整指引。', to: '/help' },
  { variant: 4, icon: 'time', artLabel: '服务公告', title: '服务公告', desc: '维护窗口、故障说明与重要通知。', to: '/announcements' },
  { variant: 5, icon: 'secured', artLabel: '条款政策', title: '用户条款与隐私政策', desc: '服务条款、责任边界与个人信息处理规则。', to: '/terms' },
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
    desc: '浏览全部可开通的云产品与各周期价格，在线下单后自动交付。',
    actionLabel: '去选购',
    to: '/products',
  },
  {
    // 工单入口在登录后的用户中心里（`/support/tickets`），门户上没有免登录提单页，
    // 因此这里导向「联系与支持」区块（含客服电话与邮箱），而不是造一个假提单入口。
    icon: 'ticket',
    title: '联系与支持',
    desc: '售前咨询与售后问题都可联系客服；已注册用户可在控制台提交工单并按优先级跟进。',
    actionLabel: '查看联系方式',
    to: '/#contact',
  },
]

/**
 * 快捷入口右侧的服务承诺小卡片。
 *
 * 三条都能在用户中心里找到对应的事实：
 *  - 支付成功后由后台开通任务创建实例（订单详情可看到开通状态与失败原因）；
 *  - 开通与续费的每一步都留痕，异常时转人工队列处理（不是「自动退款」，
 *    退款属于后台按单审核的流程，不作为对用户的服务承诺）；
 *  - 工单分低/中/高/紧急四档优先级，按优先级响应。
 */
export const SERVICE_PROMISE = {
  title: '服务承诺',
  desc: '订单支付后自动开通；上游同步异常时进入人工处理队列，并全程记录操作留痕。',
  points: ['开通与续费全程留痕', '异常转入人工处理', '工单按优先级响应'],
}
