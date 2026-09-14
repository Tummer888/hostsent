<template>
  <footer class="site-footer">
    <div class="site-container">
      <!-- 服务保障条 -->
      <div class="site-footer__promise">
        <div v-for="p in promises" :key="p.title" class="site-footer__promise-item">
          <span class="site-footer__promise-icon">
            <SiteIcon :name="p.icon" :size="26" />
          </span>
          <div class="site-footer__promise-text">
            <strong>{{ p.title }}</strong>
            <span>{{ p.desc }}</span>
          </div>
        </div>
      </div>

      <div class="site-footer__top">
        <div class="site-footer__brand">
          <NuxtLink to="/" class="site-footer__logo">
            <BrandLogo :site="site" :size="28" />
            <span class="site-footer__name">{{ site.name }}</span>
          </NuxtLink>
          <p class="site-footer__slogan">{{ site.slogan }}</p>

          <!--
            联系方式区块：电话/邮箱都没配时不渲染「热线」标题 ——
            只留一个标题、下面空一行，比整块没有更让人困惑。
          -->
          <div class="site-footer__hotline">
            <template v-if="site.contactPhone">
              <h4 class="site-footer__column-title">售前咨询热线</h4>
              <p class="site-footer__hotline-number">{{ site.contactPhone }}</p>
            </template>
            <template v-else-if="site.contactEmail">
              <h4 class="site-footer__column-title">售前咨询邮箱</h4>
              <p class="site-footer__hotline-number">{{ site.contactEmail }}</p>
            </template>
            <a class="site-footer__link" href="/#contact">技术服务咨询</a>
            <NuxtLink class="site-footer__link" to="/announcements">服务公告</NuxtLink>
            <NuxtLink class="site-footer__link" to="/products">产品咨询</NuxtLink>
          </div>

          <!--
            关注入口：只渲染运营配了 url 的社交项 + 配了公众号名称时的文字展示。
            改造前这里是四个按钮（微信/QQ/开源社区/视频号）点了弹「XX开发中」——
            没有任何真实账号可跳，属于典型的假入口。没有可跳转地址就整块不渲染。
          -->
          <div v-if="reachableSocials.length || site.wechat" class="site-footer__social">
            <h4 class="site-footer__column-title">关注{{ site.name }}</h4>
            <p v-if="site.wechat" class="site-footer__wechat">
              公众号：{{ site.wechat }}
            </p>
            <div v-if="reachableSocials.length" class="site-footer__social-row">
              <a
                v-for="s in reachableSocials"
                :key="s.label"
                class="site-footer__social-btn"
                :href="s.url"
                :aria-label="s.label"
                :title="s.label"
                target="_blank"
                rel="noopener nofollow"
              >
                <SiteIcon :name="s.icon" :size="18" />
                <span>{{ s.label }}</span>
              </a>
            </div>
          </div>
        </div>

        <div class="site-footer__columns">
          <div v-for="column in footerColumns" :key="column.title" class="site-footer__column">
            <h4 class="site-footer__column-title">{{ column.title }}</h4>
            <!--
              站内路径用 NuxtLink 走客户端路由，外链用 <a>。
              外链一律 nofollow：友情链接是「被链方付费换取曝光」的典型位置，
              不加 nofollow 会被搜索引擎判为链接农场，连带拖累自己的权重。
            -->
            <template v-for="link in column.links" :key="link.label">
              <NuxtLink v-if="isInternalPath(link.to)" class="site-footer__link" :to="link.to">
                {{ link.label }}
              </NuxtLink>
              <a
                v-else
                class="site-footer__link"
                :href="link.to"
                target="_blank"
                rel="noopener nofollow"
              >
                {{ link.label }}
              </a>
            </template>
          </div>

          <!-- 友情链接来自 friendly_links 表，按 sort_order 排序；无数据时整栏不渲染 -->
          <div v-if="links.length" class="site-footer__column">
            <h4 class="site-footer__column-title">友情链接</h4>
            <a
              v-for="link in links"
              :key="link.id"
              class="site-footer__link"
              :href="link.url"
              :target="link.openInNew ? '_blank' : undefined"
              :rel="link.openInNew ? 'noopener nofollow' : 'nofollow'"
              :title="link.description || link.name"
            >
              {{ link.name }}
            </a>
          </div>
        </div>
      </div>

      <div class="site-footer__bottom">
        <div class="site-footer__legal">
          <span>{{ site.copyright }}</span>
          <template v-if="site.icp">
            <span class="site-footer__sep">|</span>
            <span>{{ site.icp }}</span>
          </template>
        </div>
        <div v-if="legalLine" class="site-footer__contact">
          <span>{{ legalLine }}</span>
        </div>
        <div class="site-footer__policy">
          <NuxtLink class="site-footer__link" to="/terms">用户条款</NuxtLink>
          <span class="site-footer__sep">|</span>
          <NuxtLink class="site-footer__link" to="/privacy">隐私政策</NuxtLink>
        </div>
      </div>

      <div v-if="site.publicSecurity" class="site-footer__badges">
        <span class="site-footer__badge">
          <SiteIcon name="secured" :size="14" />
          {{ site.publicSecurity }}
        </span>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
const { content } = useSiteContent()
const { links } = useFriendlyLinks()

const site = computed(() => content.value.site)

/**
 * 法务信息行：许可证号、代理机构与补充文案都来自管理端配置，三者都没有时不渲染整行。
 * 之前这里是写死的示例证号，属于"看起来合规、实际是假信息"，比留空更危险。
 */
const legalLine = computed(() => {
  const parts: string[] = []
  if (site.value.licenseNo) {
    parts.push(`增值电信业务经营许可证：${site.value.licenseNo}`)
  }
  if (site.value.licenseOrg) {
    parts.push(`代理域名注册服务机构：${site.value.licenseOrg}`)
  }
  if (site.value.footerLegalLine) {
    parts.push(site.value.footerLegalLine)
  }
  return parts.join(' | ')
})

/**
 * 页脚区块（doc100 §8.3）：运营在系统配置里填 JSON 即覆盖，留空或填坏时回落代码默认值。
 * 回落值就是本次改造前硬编码在组件里的那一套，因此「没配置」与「改造前」表现一致。
 */
const promises = computed(() => site.value.footerPromises)

/**
 * 只保留配了跳转地址的社交项。
 *
 * 为什么按 url 过滤而不是「配了就渲染」：页脚上的关注按钮必须点得动，
 * 缺地址的项会变成死按钮（改造前是弹「XX开发中」的提示）。运营只有拿到真实
 * 公众号/群/仓库地址时才应该填这一项，因此在渲染层强制这个约束。
 */
const reachableSocials = computed(() =>
  site.value.footerSocials.filter((s) => /^https?:\/\//i.test(s.url.trim())),
)

/** 默认栏目：文案与品牌名、真实产品线耦合，写死进配置默认值会在品牌改名后对不上。 */
function defaultColumns(name: string) {
  return [
    {
      title: `关于${name}`,
      links: [
        { label: '新闻资讯', to: '/news' },
        { label: '帮助中心', to: '/help' },
        { label: '服务公告', to: '/announcements' },
        { label: '全部产品', to: '/products' },
      ],
    },
    {
      title: '产品与计费',
      links: [
        // 只列门户上真实存在的页面。此前写的是对象存储/云数据库/私有网络/负载均衡、
        // 以及「轻量云主机」「GPU 云主机」——平台并不售卖这些商品线，点进去在
        // /products 也是同一份列表，属于用栏目名承诺不存在的品类。
        { label: '全部产品', to: '/products' },
        { label: '计费方式', to: '/#pricing' },
        { label: '产品优势', to: '/#features' },
      ],
    },
    {
      title: '支持与服务',
      links: [
        { label: '帮助中心', to: '/help' },
        { label: '服务公告', to: '/announcements' },
        { label: '联系咨询', to: '/#contact' },
      ],
    },
    {
      title: '法律条款',
      links: [
        { label: '用户条款', to: '/terms' },
        { label: '隐私政策', to: '/privacy' },
      ],
    },
  ]
}

const footerColumns = computed(() =>
  site.value.footerColumns.length ? site.value.footerColumns : defaultColumns(site.value.name),
)

/** 站内路径才走 NuxtLink；`http(s)://` 与协议相对地址按外链处理。 */
function isInternalPath(to: string): boolean {
  return to.startsWith('/') && !to.startsWith('//')
}
</script>

<style scoped>
.site-footer {
  background: var(--site-bg-muted);
  border-top: 1px solid var(--site-border-soft);
  padding: 40px 0 24px;
}

.site-footer__promise {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 24px;
  padding: 20px 0 28px;
  border-bottom: 1px solid var(--site-border-soft);
}

.site-footer__promise-item {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.site-footer__promise-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 8px;
  background: var(--site-bg-soft);
  color: var(--site-primary);
  flex-shrink: 0;
}

.site-footer__promise-text {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.site-footer__promise-text strong {
  font-size: 14.5px;
  color: var(--site-text-strong);
}

.site-footer__promise-text span {
  font-size: 12.5px;
  color: var(--site-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.site-footer__top {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 48px;
  padding: 32px 0;
}

.site-footer__brand {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.site-footer__logo {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.site-footer__name {
  font-size: 16px;
  font-weight: 700;
}

.site-footer__slogan {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.8;
  color: var(--site-text-muted);
}

.site-footer__hotline h4,
.site-footer__social h4 {
  margin-bottom: 6px;
}

.site-footer__hotline-number {
  margin: 0 0 8px;
  font-size: 18px;
  font-weight: 700;
  color: var(--site-text-strong);
}

.site-footer__social-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.site-footer__social-btn,
.site-footer__social-app {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--site-border);
  border-radius: 6px;
  background: var(--site-bg-soft);
  color: var(--site-text-muted);
  cursor: pointer;
  font-size: 12.5px;
  transition: border-color 0.18s ease, color 0.18s ease, background 0.18s ease;
}

.site-footer__social-btn:hover,
.site-footer__social-app:hover {
  border-color: var(--site-primary);
  color: var(--site-primary);
  background: var(--site-bg);
}

.site-footer__columns {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 24px;
}

.site-footer__column {
  display: flex;
  flex-direction: column;
  gap: 11px;
}

.site-footer__column-title {
  margin-bottom: 3px;
  font-size: 14px;
  font-weight: 600;
}

.site-footer__link {
  font-size: 13.5px;
  color: var(--site-text-muted);
  transition: color 0.18s ease;
}

.site-footer__link:hover {
  color: var(--site-primary);
}

.site-footer__bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-top: 8px;
  padding-top: 22px;
  border-top: 1px solid var(--site-border);
  font-size: 13px;
  color: var(--site-text-subtle);
  flex-wrap: wrap;
}

.site-footer__legal,
.site-footer__contact,
.site-footer__policy {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.site-footer__sep {
  color: var(--site-border);
}

.site-footer__badges {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--site-border-soft);
  font-size: 12.5px;
  color: var(--site-text-muted);
}

.site-footer__badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

/* 公众号名称是纯文本展示（扫码/搜索关注），不是可点按钮 */
.site-footer__wechat {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--site-text-muted);
}

@media (max-width: 1100px) {
  .site-footer__columns {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .site-footer__top {
    grid-template-columns: minmax(0, 1fr);
    gap: 32px;
  }

  .site-footer__promise {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .site-footer {
    padding: 28px 0 20px;
  }

  .site-footer__promise {
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
    padding: 16px 0 20px;
  }

  .site-footer__columns {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 20px 16px;
  }

  .site-footer__bottom {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    margin-top: 4px;
  }
}
</style>
