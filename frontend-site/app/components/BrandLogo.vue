<template>
  <img
    v-if="site.logo"
    class="brand-logo__image"
    :src="site.logo"
    :alt="site.name"
    :height="size"
  />
  <span v-else class="brand-logo__mark" :style="{ width: `${size}px`, height: `${size}px` }">
    {{ site.name.slice(0, 1) }}
  </span>
</template>

<script setup lang="ts">
import type { SiteSection } from '#shared/schemas/siteContent'

withDefaults(defineProps<{ site: SiteSection; size?: number }>(), { size: 30 })
</script>

<style scoped>
.brand-logo__image {
  width: auto;
  object-fit: contain;
}

.brand-logo__mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--site-radius-sm);
  background: linear-gradient(135deg, var(--site-primary) 0%, var(--site-primary-strong) 100%);
  color: #fff;
  font-weight: 700;
  font-size: 15px;
  flex-shrink: 0;
}
</style>
