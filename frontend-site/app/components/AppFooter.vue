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

          <div class="site-footer__hotline">
            <h4 class="site-footer__column-title">售前咨询热线</h4>
            <p class="site-footer__hotline-number">{{ site.contactPhone || '400-800-1234' }}</p>
            <a class="site-footer__link" href="/#contact">技术服务咨询</a>
            <a class="site-footer__link" href="/#contact">备案服务</a>
            <a class="site-footer__link" href="/products">云商店咨询</a>
          </div>

          <div class="site-footer__social">
            <h4 class="site-footer__column-title">关注宿派云控</h4>
            <div class="site-footer__social-row">
              <button
                v-for="s in socials"
                :key="s.label"
                class="site-footer__social-btn"
                :aria-label="s.label"
                :title="s.label"
                type="button"
                @click="onSocial(s.label)"
              >
                <SiteIcon :name="s.icon" :size="18" />
              </button>
              <button class="site-footer__social-app" type="button" @click="onSocial('App')">
                <SiteIcon name="mobile" :size="14" />
                App
              </button>
            </div>
          </div>
        </div>

        <div class="site-footer__columns">
          <div v-for="column in columns" :key="column.title" class="site-footer__column">
            <h4 class="site-footer__column-title">{{ column.title }}</h4>
            <a
              v-for="link in column.links"
              :key="link.label"
              class="site-footer__link"
              :href="link.to"
            >
              {{ link.label }}
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
        <div class="site-footer__contact">
          <span>增值电信业务经营许可证：B1-20260000 | 代理域名注册服务机构：示例机构</span>
          <span>© 2026 Hostsent.com 版权所有</span>
        </div>
        <div class="site-footer__policy">
          <a class="site-footer__link" href="/#contact">法律条文</a>
          <span class="site-footer__sep">|</span>
          <a class="site-footer__link" href="/#contact">隐私政策</a>
        </div>
      </div>

      <div class="site-footer__badges">
        <span class="site-footer__badge">
          <SiteIcon name="certificate" :size="14" />
          电子营业执照
        </span>
        <span class="site-footer__badge">
          <SiteIcon name="secured" :size="14" />
          公网安备 0000000000000号
        </span>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
const { content } = useSiteContent()

const site = computed(() => content.value.site)

const promises = [
  { title: '7×24', desc: '多渠道服务支持', icon: 'time' },
  { title: '备案', desc: '提供免费备案服务', icon: 'secured' },
  { title: '专业服务', desc: '云业务全流程支持', icon: 'service' },
  { title: '退订', desc: '享无忧退订服务', icon: 'rollback' },
  { title: '建议反馈', desc: '优化改进建议', icon: 'edit' },
]

const socials = [
  { label: '微信', icon: 'wechat' },
  { label: 'QQ', icon: 'qq' },
  { label: '开源社区', icon: 'github' },
  { label: '视频号', icon: 'video' },
]

const columns = [
  {
    title: '关于宿派云控',
    links: [
      { label: '了解宿派云控', to: '/#contact' },
      { label: '云计算概念', to: '/#contact' },
      { label: '客户案例', to: '/#contact' },
      { label: '信任中心', to: '/#contact' },
      { label: '新闻资讯', to: '/#contact' },
      { label: '视频中心', to: '/#contact' },
    ],
  },
  {
    title: '热门产品',
    links: [
      { label: '云主机 CVM', to: '/products' },
      { label: '轻量云主机', to: '/products' },
      { label: '对象存储', to: '/products' },
      { label: '云数据库', to: '/products' },
      { label: '私有网络', to: '/products' },
      { label: '负载均衡', to: '/products' },
    ],
  },
  {
    title: '支持与服务',
    links: [
      { label: '自助服务', to: '/#contact' },
      { label: '服务公告', to: '/#contact' },
      { label: '支持计划', to: '/#contact' },
      { label: '联系我们', to: '/#contact' },
      { label: '举报中心', to: '/#contact' },
    ],
  },
  {
    title: '实用工具',
    links: [
      { label: '价格计算器', to: '/products' },
      { label: '云助手', to: '/#contact' },
      { label: '服务健康看板', to: '/#contact' },
      { label: 'API 密钥', to: '/#contact' },
    ],
  },
  {
    title: '友情链接',
    links: [
      { label: '宿派云官网', to: '/' },
      { label: '开发者联盟', to: '/#contact' },
      { label: '企业业务', to: '/products' },
      { label: '云商城', to: '/products' },
    ],
  },
]

function onSocial(label: string) {
  const notice = document.createElement('div')
  notice.textContent = `${label}开发中`
  notice.className = 'site-footer__notice'
  document.body.appendChild(notice)
  window.setTimeout(() => notice.remove(), 1600)
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

.site-footer__notice {
  position: fixed;
  left: 50%;
  bottom: 32px;
  transform: translateX(-50%);
  z-index: 999;
  padding: 10px 18px;
  border-radius: 8px;
  background: rgba(17, 24, 39, 0.92);
  color: #fff;
  font-size: 13px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.16);
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
