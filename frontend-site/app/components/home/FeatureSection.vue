<template>
  <section id="features" class="site-section features">
    <div class="site-container">
      <SectionHeading :title="home.featuresTitle" :subtitle="subtitle" />

      <div class="features__grid">
        <article v-for="feature in home.features" :key="feature.title" class="feature-card">
          <span class="feature-card__icon">
            <SiteIcon :name="feature.icon" />
          </span>
          <h3 class="feature-card__title">{{ feature.title }}</h3>
          <p class="feature-card__desc">{{ feature.desc }}</p>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
const { content } = useSiteContent()

const home = computed(() => content.value.home)

const subtitle = computed(
  () => `围绕交付、稳定与可控性设计的 ${content.value.site.name}，让日常运维少花力气`,
)
</script>

<style scoped>
.features {
  background: #fff;
}

.features__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
  margin-top: 34px;
}

.feature-card {
  padding: 26px 22px;
  border-radius: var(--site-radius-lg);
  border: 1px solid var(--site-border);
  background: #fff;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.feature-card:hover {
  transform: translateY(-3px);
  border-color: var(--site-primary-border);
  box-shadow: 0 16px 34px rgba(15, 23, 42, 0.08);
}

.feature-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 46px;
  height: 46px;
  border-radius: var(--site-radius);
  background: var(--site-primary-soft);
  color: var(--site-primary-strong);
}

.feature-card__title {
  margin-top: 18px;
  font-size: 17px;
  font-weight: 600;
}

.feature-card__desc {
  margin-top: 10px;
  font-size: 13.5px;
  line-height: 1.8;
  color: var(--site-text-muted);
}

@media (max-width: 1024px) {
  .features__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .features__grid {
    grid-template-columns: minmax(0, 1fr);
    margin-top: 30px;
  }
}
</style>
