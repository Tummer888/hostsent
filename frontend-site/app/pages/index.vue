<template>
  <div class="home-page">
    <!-- 1. 通屏 banner + 底部能力卡 -->
    <HomeHeroSection />

    <!-- 2. 热门产品（真实数据：后台推荐位） -->
    <HomeFeaturedProducts />

    <!-- 3. 为什么选择我们 -->
    <HomeFeatureSection />

    <!-- 4. 优惠活动货架 -->
    <HomePromoSection />

    <!-- 5. 上游与生态 -->
    <HomePartnerLogos />

    <!-- 6. 上手与支持（控制台入口 + 文档/API 宫格） -->
    <HomeFeaturedShowcase />

    <!-- 7. 资源与支持 -->
    <HomeResourceCards />

    <!-- 8. 最新公告（真实数据） -->
    <HomeAnnouncements />

    <!-- 9. 快捷入口 + 服务承诺 -->
    <HomeQuickLinks />

    <!-- 10. 底部通栏 CTA -->
    <HomeCtaSection />
  </div>
</template>

<script setup lang="ts">
const { content } = useSiteContent()

// 首页 SEO：标题用 Hero 主标题，由 app.vue 的 titleTemplate 拼接站点名
useSeoMeta({
  title: () => content.value.home.heroTitle,
  description: () => content.value.home.heroSubtitle || content.value.site.slogan,
  ogTitle: () => `${content.value.home.heroTitle} · ${content.value.site.name}`,
  ogDescription: () => content.value.home.heroSubtitle || content.value.site.slogan,
  ogType: 'website',
})
</script>
