<template>
  <footer class="site-footer">
    <div class="site-container">
      <div class="site-footer__top">
        <div class="site-footer__brand">
          <NuxtLink to="/" class="site-footer__logo">
            <BrandLogo :site="site" :size="28" />
            <span class="site-footer__name">{{ site.name }}</span>
          </NuxtLink>
          <p class="site-footer__slogan">{{ site.slogan }}</p>
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
          <span v-if="site.contactPhone">售前咨询：{{ site.contactPhone }}</span>
          <span v-if="site.contactEmail">{{ site.contactEmail }}</span>
        </div>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
const { content } = useSiteContent()

const site = computed(() => content.value.site)

const columns = [
  {
    title: '产品服务',
    links: [
      { label: '云主机', to: '#features' },
      { label: '轻量云主机', to: '#features' },
      { label: '对象存储', to: '#features' },
      { label: '负载均衡', to: '#features' },
    ],
  },
  {
    title: '支持与帮助',
    links: [
      { label: '帮助文档', to: '#contact' },
      { label: '提交工单', to: '#contact' },
      { label: '常见问题', to: '#contact' },
      { label: '联系我们', to: '#contact' },
    ],
  },
  {
    title: '关于我们',
    links: [
      { label: '公司介绍', to: '#contact' },
      { label: '新闻资讯', to: '#contact' },
      { label: '服务协议', to: '#contact' },
      { label: '隐私政策', to: '#contact' },
    ],
  },
]
</script>

<style scoped>
.site-footer {
  background: var(--site-bg-muted);
  border-top: 1px solid var(--site-border-soft);
  padding: 56px 0 28px;
}

.site-footer__top {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  gap: 48px;
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
  margin-top: 14px;
  font-size: 13.5px;
  line-height: 1.8;
  color: var(--site-text-muted);
}

.site-footer__columns {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 32px;
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
  margin-top: 48px;
  padding-top: 22px;
  border-top: 1px solid var(--site-border);
  font-size: 13px;
  color: var(--site-text-subtle);
}

.site-footer__legal,
.site-footer__contact {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.site-footer__sep {
  color: var(--site-border);
}

@media (max-width: 900px) {
  .site-footer__top {
    grid-template-columns: minmax(0, 1fr);
    gap: 32px;
  }
}

@media (max-width: 768px) {
  .site-footer {
    padding: 40px 0 24px;
  }

  .site-footer__columns {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 24px 16px;
  }

  .site-footer__bottom {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    margin-top: 32px;
  }
}
</style>
