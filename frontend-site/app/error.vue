<template>
  <div class="site-root">
    <AppHeader />
    <main class="site-main">
      <section class="content-shell">
        <div class="site-container">
          <div class="err">
            <p class="err__code">{{ statusCode }}</p>
            <h1 class="err__title">{{ title }}</h1>
            <p class="err__desc">{{ description }}</p>
            <div class="err__actions">
              <NuxtLink to="/" class="site-btn site-btn--primary">返回首页</NuxtLink>
              <NuxtLink to="/products" class="site-btn site-btn--ghost">浏览全部产品</NuxtLink>
            </div>
            <p v-if="statusCode === 503" class="err__hint">
              这属于临时故障，稍后刷新即可恢复正常，页面内容并未删除。
            </p>
          </div>
        </div>
      </section>
    </main>
    <AppFooter />
  </div>
</template>

<script setup lang="ts">
/**
 * 门户错误页。
 *
 * 存在的意义是把「404 内容不存在」与「503 服务不可用」讲清楚（doc80 R12）：
 * 内容页会因后端故障返回 503，如果统一渲染成「页面不存在」，用户会以为
 * 文档被删了，搜索引擎也会据此把整批内容页从索引里移除 —— 两者后果都不小。
 */
const props = defineProps<{
  error: {
    statusCode?: number
    statusMessage?: string
    message?: string
  }
}>()

const statusCode = computed(() => props.error?.statusCode ?? 500)

const title = computed(() => {
  if (statusCode.value === 404) return '页面不存在'
  if (statusCode.value === 503) return '服务暂时不可用'
  return '页面出错了'
})

const description = computed(() => {
  if (statusCode.value === 404) {
    return '这个地址没有对应的内容，可能是链接过期或内容已下线。'
  }
  if (statusCode.value === 503) {
    return '我们暂时没能取到页面数据，请稍后再试。'
  }
  return '页面渲染过程中出现了问题，请稍后再试。'
})

const { content } = useSiteContent()

useSeoMeta({
  title,
  description,
  robots: 'noindex',
})

// 顶栏与页脚都依赖站点配置：错误页也要带上，避免用户在这里失去导航能力。
useHead(() => ({ titleTemplate: (t?: string) => (t ? `${t} · ${content.value.site.name}` : content.value.site.name) }))
</script>

<style scoped>
.err {
  padding: 72px 0;
  max-width: 560px;
}

.err__code {
  font-size: 52px;
  font-weight: 700;
  line-height: 1;
  color: var(--site-primary);
}

.err__title {
  margin-top: 18px;
  font-size: 26px;
  font-weight: 700;
}

.err__desc {
  margin-top: 14px;
  font-size: 15px;
  line-height: 1.85;
  color: var(--site-text-muted);
}

.err__actions {
  display: flex;
  gap: 12px;
  margin-top: 28px;
  flex-wrap: wrap;
}

.err__hint {
  margin-top: 20px;
  font-size: 13px;
  color: var(--site-text-subtle);
}

@media (max-width: 560px) {
  .err {
    padding: 44px 0;
  }

  .err__code {
    font-size: 40px;
  }

  .err__title {
    font-size: 21px;
  }
}
</style>
