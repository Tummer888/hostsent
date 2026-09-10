<template>
  <header class="hd" :class="{ 'hd--scrolled': scrolled }">
    <div class="site-container hd__inner">
      <NuxtLink to="/" class="hd__brand">
        <BrandLogo :site="site" :size="30" />
        <span class="hd__name">{{ site.name }}</span>
      </NuxtLink>

      <nav class="hd__nav">
        <NuxtLink
          v-for="item in NAV_ITEMS"
          :key="item.to"
          class="hd__link"
          :to="item.to"
          :aria-current-value="ariaCurrentFor(item.to)"
        >
          {{ item.label }}
        </NuxtLink>
      </nav>

      <div class="hd__actions">
        <form class="hd__search" role="search" @submit.prevent="submitSearch">
          <SiteIcon name="search" class="hd__search-icon" aria-hidden="true" />
          <input
            v-model="keyword"
            class="hd__search-input"
            type="search"
            name="keyword"
            placeholder="搜索云产品"
            aria-label="搜索云产品"
          />
        </form>

        <a
          v-if="consoleBase"
          class="hd__console"
          :href="consoleBase"
          rel="noopener"
        >
          控制台
        </a>

        <NuxtLink class="site-btn site-btn--primary hd__cta" :to="ctaLink">
          {{ ctaLabel }}
        </NuxtLink>

        <button
          class="hd__menu-btn"
          type="button"
          :aria-expanded="menuOpen"
          aria-label="展开菜单"
          @click="menuOpen = !menuOpen"
        >
          <SiteIcon :name="menuOpen ? 'close' : 'menu'" />
        </button>
      </div>
    </div>

    <div v-if="menuOpen" class="hd__drawer">
      <div class="site-container">
        <form class="hd__search hd__search--drawer" role="search" @submit.prevent="submitSearch">
          <SiteIcon name="search" class="hd__search-icon" aria-hidden="true" />
          <input
            v-model="keyword"
            class="hd__search-input"
            type="search"
            placeholder="搜索云产品"
            aria-label="搜索云产品"
          />
        </form>

        <NuxtLink
          v-for="item in NAV_ITEMS"
          :key="item.to"
          class="hd__drawer-link"
          :to="item.to"
          :aria-current-value="ariaCurrentFor(item.to)"
          @click="menuOpen = false"
        >
          {{ item.label }}
        </NuxtLink>

        <a v-if="consoleBase" class="hd__drawer-link" :href="consoleBase" rel="noopener">
          控制台
        </a>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ariaCurrentFor } from '~/utils/nav'

const { content } = useSiteContent()
const { public: publicConfig } = useRuntimeConfig()
const router = useRouter()

const site = computed(() => content.value.site)

/** 只放真实存在的目的地，避免做出点进去 404 的导航。 */
const NAV_ITEMS = [
  { label: '全部产品', to: '/products' },
  { label: '产品优势', to: '/#features' },
  { label: '优惠活动', to: '/#promos' },
  { label: '最新公告', to: '/#announcements' },
  { label: '联系与支持', to: '/#contact' },
]

const ctaLabel = computed(() => content.value.home.heroPrimaryCta)
const ctaLink = computed(() => content.value.home.heroPrimaryLink || '/products')

/** 控制台地址未配置时隐藏入口，而不是留一个点不动的死链。 */
const consoleBase = computed(() => String(publicConfig.consoleUrl || '').replace(/\/+$/, ''))

const menuOpen = ref(false)
const keyword = ref('')

/**
 * 搜索直接落到产品列表页 —— 列表页以 URL query 为单一真相（`/products?keyword=`），
 * 所以这里只需要把词推进地址栏，其余交给列表页处理。
 */
function submitSearch() {
  const query = keyword.value.trim()
  menuOpen.value = false
  router.push(query ? { path: '/products', query: { keyword: query } } : { path: '/products' })
}

/** 首页首屏 banner 是浅色的，页头保持浅色半透明即可；滚动后加分隔线与阴影。 */
const scrolled = ref(false)

function onScroll() {
  scrolled.value = window.scrollY > 8
}

onMounted(() => {
  onScroll()
  window.addEventListener('scroll', onScroll, { passive: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll)
})

watch(() => router.currentRoute.value.fullPath, () => {
  menuOpen.value = false
})
</script>

<style scoped>
.hd {
  position: sticky;
  top: 0;
  z-index: 30;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(14px);
  border-bottom: 1px solid transparent;
  transition: border-color 0.22s ease, box-shadow 0.22s ease, background-color 0.22s ease;
}

.hd--scrolled {
  background: rgba(255, 255, 255, 0.96);
  border-bottom-color: var(--site-border);
  box-shadow: 0 6px 24px rgba(15, 23, 42, 0.05);
}

.hd__inner {
  height: var(--site-header-height);
  display: flex;
  align-items: center;
  gap: 28px;
}

.hd__brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.hd__name {
  font-size: 17px;
  font-weight: 700;
  letter-spacing: 0.01em;
  color: var(--site-text);
}

.hd__nav {
  display: flex;
  align-items: center;
  gap: 24px;
  margin-right: auto;
}

.hd__link {
  font-size: 14.5px;
  color: #334155;
  white-space: nowrap;
  transition: color 0.18s ease;
}

.hd__link:hover {
  color: var(--site-primary);
}

.hd__actions {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
}

/* 搜索框：参考图页头右侧的圆角输入框 */
.hd__search {
  position: relative;
  flex: 0 1 232px;
  min-width: 132px;
}

.hd__search-icon {
  position: absolute;
  left: 13px;
  top: 50%;
  width: 16px;
  height: 16px;
  transform: translateY(-50%);
  color: var(--site-text-subtle);
  pointer-events: none;
}

.hd__search-input {
  width: 100%;
  height: 38px;
  padding: 0 14px 0 37px;
  border: 1px solid var(--site-border);
  border-radius: 999px;
  background: #fff;
  color: var(--site-text);
  font-size: 13.5px;
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.hd__search-input::placeholder {
  color: var(--site-text-subtle);
}

.hd__search-input:focus {
  outline: none;
  border-color: var(--site-primary-border);
  box-shadow: 0 0 0 3px var(--site-primary-soft);
}

.hd__console {
  font-size: 14.5px;
  color: #334155;
  white-space: nowrap;
  transition: color 0.18s ease;
}

.hd__console:hover {
  color: var(--site-primary);
}

.hd__cta {
  height: 38px;
  padding: 0 20px;
  font-size: 14px;
  border-radius: 999px;
}

.hd__menu-btn {
  display: none;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border: 1px solid var(--site-border);
  border-radius: 50%;
  background: #fff;
  color: var(--site-text);
  cursor: pointer;
}

.hd__menu-btn:hover {
  border-color: var(--site-primary-border);
  color: var(--site-primary);
}

.hd__menu-btn :deep(.site-icon) {
  width: 18px;
  height: 18px;
}

.hd__drawer {
  padding: 6px 0 16px;
  background: #fff;
  border-bottom: 1px solid var(--site-border);
}

.hd__search--drawer {
  flex: none;
  margin: 10px 0 6px;
}

.hd__drawer-link {
  display: block;
  padding: 13px 2px;
  border-bottom: 1px solid var(--site-border-soft);
  color: var(--site-text);
  font-size: 15px;
}

.hd__drawer-link:hover {
  color: var(--site-primary);
}

/* 导航与搜索一起切换到抽屉，保证抽屉打开时能搜到东西 */
@media (max-width: 1024px) {
  .hd__nav,
  .hd__search {
    display: none;
  }

  .hd__search--drawer {
    display: block;
  }

  .hd__actions {
    margin-left: auto;
  }

  .hd__menu-btn {
    display: inline-flex;
  }
}

@media (max-width: 560px) {
  .hd__cta,
  .hd__console {
    display: none;
  }
}
</style>
