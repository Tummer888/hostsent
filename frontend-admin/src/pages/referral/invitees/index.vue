<template>
  <div class="page-body referral-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <UsergroupIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">邀请关系</h2>
          <p class="page-header__desc">按邀请人查询其直接邀请的用户与贡献（单级邀请）</p>
        </div>
      </div>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">选择邀请人</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">邀请人用户 ID</span>
          <t-input v-model="inviterUserId" placeholder="请输入邀请人用户 ID" clearable @enter="handleSearch" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">被邀请人列表</h3>
        <span class="table-card__meta">共 {{ total }} 人</span>
      </div>
      <t-table
        row-key="user_id"
        :data="invitees"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #username="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="price-sub">{{ row.email || '—' }}</span>
          </div>
        </template>

        <template #cashback_total="{ row }">
          <span class="price-main">¥{{ formatPrice(row.cashback_total) }}</span>
        </template>

        <template #order_count="{ row }">
          <span class="cell-strong">{{ row.order_count || 0 }}</span>
        </template>

        <template #invited_at="{ row }">
          <span class="time-text">{{ formatTime(row.invited_at) }}</span>
        </template>

        <template #empty>
          <t-empty :description="emptyText" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { SearchIcon, UsergroupIcon } from 'tdesign-icons-vue-next'

import { getReferralInvitees } from '@/api/referral'
import { formatPrice, formatTime } from '@/pages/referral/constants'
import type { ReferralInviteeInfo } from '@/types/interface'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ReferralInvitees' })

const invitees = ref<ReferralInviteeInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const inviterUserId = ref('')

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const emptyText = computed(() =>
  inviterUserId.value.trim() ? '该邀请人暂无邀请记录' : '请先输入邀请人用户 ID 后查询',
)

const columns: PrimaryTableCol<ReferralInviteeInfo>[] = [
  { colKey: 'user_id', title: '用户ID', width: 100, align: 'center' as const },
  { colKey: 'username', title: '被邀请人', minWidth: 200 },
  { colKey: 'cashback_total', title: '贡献返现', width: 140 },
  { colKey: 'order_count', title: '有效订单数', width: 120, align: 'center' as const },
  { colKey: 'invited_at', title: '绑定时间', width: 170 },
]

async function loadInvitees() {
  const inviterId = Number(inviterUserId.value)
  if (!inviterId) {
    invitees.value = []
    total.value = 0
    pagination.total = 0
    mobilePage.total = 0
    return
  }
  loading.value = true
  try {
    const data = await getReferralInvitees({
      inviter_user_id: inviterId,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    invitees.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载邀请关系失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadInvitees()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}


function handleSearch() {
  if (!Number(inviterUserId.value)) {
    MessagePlugin.warning('请输入邀请人用户 ID')
    return
  }
  pagination.current = 1
  loadInvitees()
}
</script>

<style lang="css">
@import '../shared.css';
</style>
