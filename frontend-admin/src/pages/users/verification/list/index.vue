<template>
  <div class="page-body verification-merge-page">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <VerifyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">实名认证审核</h2>
          <p class="page-header__desc">
            待审核、审核通过、审核拒绝三个列表合并为本页，用页签切换；审核动作在「待审核」页签里完成。
          </p>
        </div>
      </div>
    </header>

    <section class="surface-card tab-card">
      <t-tabs v-model="activeTab">
        <t-tab-panel value="pending" label="待审核" />
        <t-tab-panel value="approved" label="审核通过" />
        <t-tab-panel value="rejected" label="审核拒绝" />
      </t-tabs>
    </section>

    <!-- :key 让切换页签时重挂载：筛选条件与分页回到该页签自己的初始状态 -->
    <VerificationListPage
      :key="activeTab"
      no-header
      :icon="currentMeta.icon"
      :title="currentMeta.label"
      :table-title="currentMeta.tableTitle"
      :empty-text="currentMeta.emptyText"
      :fetcher="currentMeta.fetcher"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CheckCircleIcon, ErrorCircleIcon, HistoryIcon, VerifyIcon } from 'tdesign-icons-vue-next'
import type { Component } from 'vue'

import {
  getApprovedVerificationList,
  getPendingVerificationList,
  getRejectedVerificationList,
  type VerificationInfo,
  type VerificationListQuery,
} from '@/api/verification'
import VerificationListPage from '../VerificationListPage.vue'

defineOptions({ name: 'UserVerificationList' })

type TabKey = 'pending' | 'approved' | 'rejected'

// 三个旧页面（pending/approved/rejected）合并为一页页签切换；
// 旧路径由 router redirect 带 ?tab= 落到对应页签，书签不失效。
const TAB_META: Record<TabKey, { label: string; tableTitle: string; emptyText: string; icon: Component; fetcher: (params: VerificationListQuery) => Promise<{ items: VerificationInfo[]; meta: { total: number } }> }> = {
  pending: {
    label: '实名认证待审核',
    tableTitle: '待审核申请',
    emptyText: '暂无待审核实名认证',
    icon: HistoryIcon,
    fetcher: getPendingVerificationList,
  },
  approved: {
    label: '实名认证审核通过',
    tableTitle: '审核通过记录',
    emptyText: '暂无审核通过记录',
    icon: CheckCircleIcon,
    fetcher: getApprovedVerificationList,
  },
  rejected: {
    label: '实名认证审核拒绝',
    tableTitle: '审核拒绝记录',
    emptyText: '暂无审核拒绝记录',
    icon: ErrorCircleIcon,
    fetcher: getRejectedVerificationList,
  },
}

const route = useRoute()
const router = useRouter()

function normalizeTab(value: unknown): TabKey {
  return value === 'approved' || value === 'rejected' ? value : 'pending'
}

const activeTab = ref<TabKey>(normalizeTab(route.query.tab))

// 页签与 ?tab= 双向同步：redirect 带进来的参数生效，手动切换写回地址栏（可刷新、可分享）
watch(
  () => route.query.tab,
  (value) => {
    activeTab.value = normalizeTab(value)
  },
)
watch(activeTab, (value) => {
  if (route.query.tab !== value) {
    router.replace({ query: { ...route.query, tab: value } })
  }
})

const currentMeta = computed(() => TAB_META[activeTab.value])
</script>

<style scoped>
/* 页头与用户模块基准一致（同实名认证配置页） */
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-lg);
  flex-wrap: wrap;
  padding: var(--space-lg) var(--space-xl);
}

.page-header__main {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.page-header__text {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.page-header__chip {
  width: 44px;
  height: 44px;
  border-radius: var(--hs-radius-xl);
  background: linear-gradient(135deg, var(--td-brand-color-6), var(--color-primary));
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.page-header__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  line-height: 1.3;
  color: var(--color-foreground);
}

.page-header__desc {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.tab-card {
  padding: 0 var(--space-xl);
}
</style>
