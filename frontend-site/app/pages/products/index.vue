<template>
  <div class="products-page">
    <section class="products-hero">
      <div class="site-container">
        <h1 class="products-hero__title">全部产品</h1>
        <p class="products-hero__desc">
          {{ site.slogan }}。按需开通、弹性计费，挑选适合你业务的产品。
        </p>

        <form class="products-search" @submit.prevent="applySearch">
          <input
            v-model="keywordInput"
            class="products-search__input"
            type="search"
            placeholder="搜索产品名称或编码"
            aria-label="搜索产品"
          >
          <button class="site-btn site-btn--primary" type="submit">搜索</button>
          <button
            v-if="activeKeyword"
            class="site-btn site-btn--ghost"
            type="button"
            @click="clearSearch"
          >
            清除
          </button>
        </form>
      </div>
    </section>

    <section class="site-section products-list">
      <div class="site-container">
        <p v-if="pending && !items.length" class="products-state">正在加载产品…</p>

        <template v-else-if="items.length">
          <p class="products-count">
            共 <strong>{{ total }}</strong> 个产品
          </p>
          <div class="products-grid">
            <ProductCard v-for="product in items" :key="product.id" :product="product" />
          </div>
        </template>

        <div v-else class="products-empty">
          <p class="products-empty__title">
            {{ activeKeyword ? '没有找到匹配的产品' : '暂无可售产品' }}
          </p>
          <p class="products-empty__desc">
            {{
              activeKeyword
                ? '换个关键词试试，或浏览全部产品。'
                : '产品正在上架中，欢迎联系我们了解可开通的规格。'
            }}
          </p>
          <button v-if="activeKeyword" class="site-btn site-btn--ghost" type="button" @click="clearSearch">
            查看全部产品
          </button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const router = useRouter()
const { content } = useSiteContent()
const site = computed(() => content.value.site)

const PAGE_SIZE = 12

/** 关键词以 URL 查询参数为单一真相，便于分享与 SSR 直接命中。 */
const activeKeyword = computed(() => {
  const raw = route.query.keyword
  return typeof raw === 'string' ? raw.trim() : ''
})

const keywordInput = ref(activeKeyword.value)

const { items, total, pending } = useProducts({
  pageSize: PAGE_SIZE,
  keyword: activeKeyword.value || undefined,
})

watch(activeKeyword, (value) => {
  keywordInput.value = value
})

function applySearch() {
  const keyword = keywordInput.value.trim()
  router.push({ path: '/products', query: keyword ? { keyword } : {} })
}

function clearSearch() {
  keywordInput.value = ''
  router.push({ path: '/products' })
}

useSeoMeta({
  title: () => (activeKeyword.value ? `搜索「${activeKeyword.value}」` : '全部产品'),
  description: () =>
    `${site.value.name} 产品列表：${site.value.slogan}。查看云主机与算力产品规格与价格。`,
  ogType: 'website',
})
</script>

<style scoped>
.products-hero {
  padding: 56px 0 40px;
  background: var(--site-bg-muted);
  border-bottom: 1px solid var(--site-border-soft);
}

.products-hero__title {
  font-size: 32px;
  font-weight: 700;
}

.products-hero__desc {
  margin-top: 12px;
  max-width: 620px;
  font-size: 15px;
  color: var(--site-text-muted);
}

.products-search {
  display: flex;
  gap: 10px;
  margin-top: 26px;
  max-width: 560px;
}

.products-search__input {
  flex: 1;
  height: 44px;
  padding: 0 16px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius);
  background: #fff;
  font: inherit;
  font-size: 14px;
  color: var(--site-text);
  outline: none;
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.products-search__input:focus {
  border-color: var(--site-primary);
  box-shadow: 0 0 0 3px var(--site-primary-soft);
}

.products-list {
  padding-top: 44px;
}

.products-count {
  font-size: 13.5px;
  color: var(--site-text-muted);
}

.products-count strong {
  color: var(--site-text);
}

.products-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
  margin-top: 20px;
}

.products-state {
  padding: 40px 0;
  color: var(--site-text-muted);
}

.products-empty {
  padding: 60px 0;
  text-align: center;
}

.products-empty__title {
  font-size: 17px;
  font-weight: 600;
}

.products-empty__desc {
  margin: 10px 0 22px;
  font-size: 14px;
  color: var(--site-text-muted);
}

@media (max-width: 1024px) {
  .products-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 860px) {
  .products-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .products-hero {
    padding: 40px 0 30px;
  }

  .products-hero__title {
    font-size: 25px;
  }

  .products-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
