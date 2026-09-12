<template>
  <div class="page-body point-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <UsergroupIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">积分账户</h2>
          <p class="page-header__desc">
            每位用户一个积分账户，与钱包余额完全分离。人工调整需填写备注，方便后续对账追溯。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadAccounts">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">用户账号</span>
          <t-input v-model="filters.user_keyword" placeholder="用户名或邮箱" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">最低可用积分</span>
          <t-input v-model="filters.min_points" placeholder="如 100" clearable @enter="handleSearch" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">账户列表</h3>
        <span class="table-card__meta">共 {{ total }} 个账户</span>
      </div>
      <t-table
        row-key="id"
        :data="accountList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #user="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="price-sub">ID {{ row.user_id }}{{ row.email ? ` · ${row.email}` : '' }}</span>
          </div>
        </template>

        <template #balance="{ row }">
          <span class="cell-strong">{{ formatPoints(row.balance) }}</span>
        </template>

        <template #frozen="{ row }">
          <span class="cell-muted">{{ formatPoints(row.frozen) }}</span>
        </template>

        <template #total_earned="{ row }">
          <span class="amount-income">+{{ formatPoints(row.total_earned) }}</span>
        </template>

        <template #total_spent="{ row }">
          <span class="amount-expense">-{{ formatPoints(row.total_spent) }}</span>
        </template>

        <template #updated_at="{ row }">
          <span class="time-text">{{ formatTime(row.updated_at) }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <template v-if="isMobile">
              <MobileAction
                :options="buildMobileActionOptions([
                  { content: '调整积分', value: 'adjust' },
                  { content: '查看流水', value: 'transactions' },
                ])"
                @select="(value) => handleMobileAction(value, row)"
              />
            </template>
            <template v-else>
              <t-link theme="primary" hover="color" @click="openAdjust(row)">调整积分</t-link>
              <t-link theme="default" hover="color" @click="goTransactions(row)">查看流水</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无积分账户：用户产生首笔积分发放后自动创建" />
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

    <t-dialog
      v-model:visible="adjustVisible"
      header="人工调整积分"
      width="480px"
      :confirm-btn="{ content: '确认调整', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleAdjust"
      @close="adjustVisible = false"
    >
      <t-form label-align="top" :data="adjustForm" @submit.prevent>
        <t-alert
          theme="warning"
          message="积分调整只影响积分账本，不会变动用户余额，也不能用于抵扣订单或账单。"
          style="margin-bottom: 12px"
        />
        <t-form-item label="用户">
          <span>{{ adjustForm.label }}</span>
        </t-form-item>
        <t-form-item label="调整积分（正数发放，负数扣减）">
          <t-input-number v-model="adjustForm.points" :precision="0" theme="column" placeholder="如 100 或 -50" />
        </t-form-item>
        <t-form-item label="备注（必填）">
          <t-textarea v-model="adjustForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="如：活动补发 / 违规扣减" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { RefreshIcon, SearchIcon, UsergroupIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { adjustPoints, getPointAccounts } from '@/api/point'
import { formatPoints, formatTime } from '@/pages/points/constants'
import type { PointAccountInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'PointsAccounts' })

const router = useRouter()
const accountList = ref<PointAccountInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

const filters = reactive<{ user_keyword: string | undefined; user_id: string | undefined; min_points: string | undefined }>({
  user_keyword: undefined,
  user_id: undefined,
  min_points: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const adjustVisible = ref(false)
const adjustForm = reactive<{ user_id: number; label: string; points: number | undefined; remark: string }>({
  user_id: 0,
  label: '',
  points: undefined,
  remark: '',
})

const columns: PrimaryTableCol<PointAccountInfo>[] = [
  { colKey: 'user', title: '用户', minWidth: 200 },
  { colKey: 'balance', title: '可用积分', width: 120 },
  { colKey: 'frozen', title: '冻结', width: 100 },
  { colKey: 'total_earned', title: '累计获得', width: 120 },
  { colKey: 'total_spent', title: '累计消耗', width: 120 },
  { colKey: 'updated_at', title: '更新时间', width: 170 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 180, fixed: 'right' as const, align: 'center' as const },
]

async function loadAccounts() {
  loading.value = true
  try {
    const data = await getPointAccounts({
      user_keyword: filters.user_keyword,
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      min_points: filters.min_points ? Number(filters.min_points) : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    accountList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载积分账户失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadAccounts()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

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
  pagination.current = 1
  loadAccounts()
}

function handleResetFilters() {
  filters.user_keyword = undefined
  filters.user_id = undefined
  filters.min_points = undefined
  pagination.current = 1
  loadAccounts()
}

function openAdjust(row: PointAccountInfo) {
  adjustForm.user_id = row.user_id
  adjustForm.label = `${row.username || '用户'}（ID ${row.user_id}），当前可用 ${formatPoints(row.balance)} 分`
  adjustForm.points = undefined
  adjustForm.remark = ''
  adjustVisible.value = true
}

async function handleAdjust() {
  if (!adjustForm.points) {
    MessagePlugin.warning('请输入非 0 的调整积分')
    return
  }
  if (!adjustForm.remark.trim()) {
    MessagePlugin.warning('请填写调整备注')
    return
  }
  loading.value = true
  try {
    await adjustPoints({ user_id: adjustForm.user_id, points: adjustForm.points, remark: adjustForm.remark })
    MessagePlugin.success('积分已调整')
    adjustVisible.value = false
    loadAccounts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '调整失败')
  } finally {
    loading.value = false
  }
}

function goTransactions(row: PointAccountInfo) {
  void router.push({ path: '/points/transactions', query: { user_id: String(row.user_id) } })
}

function handleMobileAction(value: string | number | Record<string, any>, row: PointAccountInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'adjust':
      openAdjust(row)
      break
    case 'transactions':
      goTransactions(row)
      break
  }
}

onMounted(() => {
  const userId = router.currentRoute.value.query.user_id
  if (typeof userId === 'string' && userId) filters.user_id = userId
  loadAccounts()
})
</script>

<style lang="css">
@import '../shared.css';
</style>
