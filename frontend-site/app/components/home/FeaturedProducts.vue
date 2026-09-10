<template>
  <section v-if="visible.length" id="products" class="site-section featured">
    <div class="site-container">
      <SectionHeading
        :title="home.featuredTitle"
        :subtitle="subtitle"
        action-label="查看全部产品"
        action-to="/products"
      />

      <div class="featured__grid">
        <ProductCard v-for="product in visible" :key="product.id" :product="product" />
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
const { content } = useSiteContent()
const home = computed(() => content.value.home)

/**
 * 只取运营在后台勾选「推荐」的商品（后端 featured 过滤）。
 *
 * 拉取条数用固定的上限值而非 home.featuredLimit：站点配置是异步取回的，
 * setup 阶段拿到的还只是代码默认值，用它当请求参数会导致后台改的数量不生效。
 * 因此固定多取一点、再按配置响应式截断，配置改动无需重新请求。
 */
const HOMEPAGE_FETCH_LIMIT = 12
const { items } = useProducts({ featured: true, pageSize: HOMEPAGE_FETCH_LIMIT })

const visible = computed(() => items.value.slice(0, home.value.featuredLimit))

const subtitle = computed(() => `${content.value.site.name} 精选的高性价比产品，按需开通、弹性计费`)
</script>

<style scoped>
.featured {
  background: var(--site-bg-muted);
}

.featured__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
  margin-top: 34px;
}

@media (max-width: 1024px) {
  .featured__grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 860px) {
  .featured__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .featured__grid {
    grid-template-columns: minmax(0, 1fr);
    margin-top: 30px;
  }
}
</style>
