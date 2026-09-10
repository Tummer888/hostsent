<template>
  <section id="contact" class="cta">
    <div class="site-container cta__inner">
      <div class="cta__text">
        <h2 class="cta__title">{{ home.ctaTitle }}</h2>
        <p class="cta__desc">{{ home.ctaDesc }}</p>
      </div>

      <div class="cta__side">
        <NuxtLink class="cta__btn" :to="home.heroPrimaryLink || '/products'">
          {{ home.heroPrimaryCta }}
          <SiteIcon name="arrow-right" :stroke-width="1.9" />
        </NuxtLink>
        <div v-if="site.contactPhone || site.contactEmail" class="cta__contact">
          <span v-if="site.contactPhone">售前咨询：{{ site.contactPhone }}</span>
          <span v-if="site.contactEmail">{{ site.contactEmail }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
const { content } = useSiteContent()

const site = computed(() => content.value.site)
const home = computed(() => content.value.home)
</script>

<style scoped>
/* 底部通栏：标题居左、行动按钮居右，做成整幅浅色带（参考图最后一条横幅） */
.cta {
  padding: 64px 0;
  background: linear-gradient(180deg, var(--site-bg-muted) 0%, var(--site-primary-soft) 100%);
  border-top: 1px solid var(--site-border-soft);
}

.cta__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 40px;
}

.cta__title {
  font-size: 27px;
  font-weight: 700;
}

.cta__desc {
  margin-top: 12px;
  max-width: 560px;
  font-size: 14.5px;
  line-height: 1.8;
  color: var(--site-text-muted);
}

.cta__side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 14px;
  flex-shrink: 0;
}

.cta__btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 48px;
  padding: 0 30px;
  border-radius: 999px;
  background: var(--site-primary);
  color: #fff;
  font-size: 15px;
  font-weight: 600;
  transition: background-color 0.18s ease, transform 0.18s ease;
}

.cta__btn:hover {
  background: var(--site-primary-strong);
  transform: translateY(-1px);
}

.cta__btn :deep(.site-icon) {
  width: 17px;
  height: 17px;
}

.cta__contact {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  font-size: 13px;
  color: var(--site-text-muted);
}

@media (max-width: 768px) {
  .cta {
    padding: 44px 0;
  }

  .cta__inner {
    flex-direction: column;
    align-items: flex-start;
    gap: 26px;
  }

  .cta__title {
    font-size: 21px;
  }

  .cta__side {
    align-items: flex-start;
  }

  .cta__contact {
    align-items: flex-start;
  }
}
</style>
