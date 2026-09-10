<template>
  <div class="site-root" :style="themeVars">
    <NuxtRouteAnnouncer />
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
  </div>
</template>

<script setup lang="ts">
const { content, themeVars } = useSiteContent()

// 全站默认 SEO 与站点图标，均来自配置（规范 R1：不硬编码品牌信息）
useHead(() => ({
  titleTemplate: (title?: string) =>
    title ? `${title} · ${content.value.site.name}` : content.value.site.name,
  link: [{ rel: 'icon', href: content.value.site.favicon }],
  meta: [
    { name: 'description', content: content.value.site.slogan },
    { name: 'theme-color', content: content.value.theme.primaryColor },
  ],
}))
</script>

<style>
.site-root {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}
</style>
