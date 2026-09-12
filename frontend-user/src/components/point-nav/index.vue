<template>
  <nav class="point-nav">
    <button
      v-for="item in items"
      :key="item.path"
      class="point-nav__item"
      :class="{ 'is-active': isActive(item.path) }"
      @click="router.push(item.path)"
    >
      <component :is="item.icon" size="16" />
      <span>{{ item.label }}</span>
    </button>
  </nav>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { GiftIcon, MoneyIcon } from 'tdesign-icons-vue-next'

defineOptions({ name: 'PointNav' })

const router = useRouter()
const route = useRoute()

const items = [
  { label: '我的积分', path: '/points', icon: GiftIcon },
  { label: '积分明细', path: '/points/transactions', icon: MoneyIcon },
]

function isActive(path: string): boolean {
  return route.path === path
}
</script>

<style scoped>
.point-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px;
  margin-bottom: 16px;
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px;
}

.point-nav__item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #475569;
  font-size: 13.5px;
  cursor: pointer;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.point-nav__item:hover {
  color: var(--color-primary, #2563eb);
  background: #eff6ff;
}

.point-nav__item.is-active {
  color: #b76a00;
  background: #fff7e8;
  font-weight: 600;
}

.dark .point-nav {
  background: #141414;
  border-color: #262626;
}
</style>
