<template>
  <nav class="agent-nav">
    <button
      v-for="item in items"
      :key="item.path"
      class="agent-nav__item"
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
import { DashboardIcon, MoneyIcon, ShareIcon, UsergroupIcon, WalletIcon } from 'tdesign-icons-vue-next'

defineOptions({ name: 'AgentNav' })

const router = useRouter()
const route = useRoute()

const items = [
  { label: '概览看板', path: '/agent/overview', icon: DashboardIcon },
  { label: '我的团队', path: '/agent/team', icon: UsergroupIcon },
  { label: '佣金明细', path: '/agent/commissions', icon: MoneyIcon },
  { label: '结算记录', path: '/agent/settlements', icon: WalletIcon },
  { label: '推广素材', path: '/agent/materials', icon: ShareIcon },
]

function isActive(path: string): boolean {
  return route.path === path
}
</script>

<style scoped>
.agent-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px;
  margin-bottom: 16px;
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px;
}

.agent-nav__item {
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

.agent-nav__item:hover {
  color: var(--color-primary, #2563eb);
  background: #eff6ff;
}

.agent-nav__item.is-active {
  color: #b76a00;
  background: #fff7e8;
  font-weight: 600;
}
</style>
