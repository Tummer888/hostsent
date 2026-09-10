<template>
  <nav class="referral-nav">
    <button
      v-for="item in items"
      :key="item.path"
      class="referral-nav__item"
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

defineOptions({ name: 'ReferralNav' })

const router = useRouter()
const route = useRoute()

const items = [
  { label: '推广概览', path: '/referral/overview', icon: DashboardIcon },
  { label: '我的邀请', path: '/referral/invitees', icon: UsergroupIcon },
  { label: '返现明细', path: '/referral/cashbacks', icon: MoneyIcon },
  { label: '提现与转出', path: '/referral/withdrawals', icon: WalletIcon },
  { label: '推广素材', path: '/referral/materials', icon: ShareIcon },
]

function isActive(path: string): boolean {
  return route.path === path
}
</script>

<style scoped>
.referral-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px;
  margin-bottom: 16px;
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px;
}

.referral-nav__item {
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

.referral-nav__item:hover {
  color: var(--color-primary, #2563eb);
  background: #eff6ff;
}

.referral-nav__item.is-active {
  color: #b76a00;
  background: #fff7e8;
  font-weight: 600;
}
</style>
