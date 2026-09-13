<template>
  <div class="page-body sales-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ChartBarIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">业绩与排行</h2>
          <p class="page-header__desc">按周期统计销售额与订单量，维护销售/部门目标并跟踪达成率</p>
        </div>
      </div>
      <t-space size="small">
        <t-button v-if="canManageTarget" variant="outline" @click="openTargetDialog">
          <template #icon>
            <EditIcon aria-hidden="true" />
          </template>
          目标维护
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section v-if="myPerformance" class="stat-grid">
      <article class="stat-card stat-card--green">
        <span class="stat-card__icon">
          <MoneyIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(myPerformance.amount) }}</span>
          <span class="stat-card__label">本月销售额（{{ myPerformance.orders }} 单）</span>
        </div>
      </article>
      <article class="stat-card stat-card--blue">
        <span class="stat-card__icon">
          <FlagIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">
            {{ myPerformance.target_amount > 0 ? `¥${formatPrice(myPerformance.target_amount)}` : '未设定' }}
          </span>
          <span class="stat-card__label">本月目标</span>
        </div>
      </article>
      <article class="stat-card stat-card--orange">
        <span class="stat-card__icon">
          <ChartBarIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ formatPercent(myPerformance.achievement) }}</span>
          <span class="stat-card__label">目标达成率</span>
        </div>
      </article>
      <article class="stat-card stat-card--indigo">
        <span class="stat-card__icon">
          <StarIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">
            {{ myPerformance.rank > 0 ? `第 ${myPerformance.rank} 名` : '未上榜' }}
          </span>
          <span class="stat-card__label">
            部门排名{{ myPerformance.dept_members > 0 ? `（共 ${myPerformance.dept_members} 人）` : '' }}
          </span>
        </div>
      </article>
    </section>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">统计周期</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">周期（月份）</span>
          <t-date-picker v-model="period" mode="month" clearable allow-input placeholder="默认当月" @change="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">部门</span>
          <t-select
            v-model="departmentID"
            clearable
            filterable
            placeholder="全部部门"
            :options="departmentOptions"
            @change="handleSearch"
          />
        </div>
        <div class="field">
          <span class="field__label">排序依据</span>
          <t-radio-group v-model="sortBy" variant="default-filled" @change="handleSearch">
            <t-radio-button value="amount">销售额</t-radio-button>
            <t-radio-button value="orders">订单量</t-radio-button>
          </t-radio-group>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">业绩排行 · {{ rankingPeriod || period || '当月' }}</h3>
        <span class="table-card__meta">共 {{ ranking.length }} 位销售</span>
      </div>
      <t-table
        row-key="admin_id"
        :data="ranking"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #rank="{ row }">
          <span :class="['rank-badge', row.rank <= 3 ? `rank-badge--top${row.rank}` : '']">{{ row.rank }}</span>
        </template>

        <template #admin="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.admin_real_name || row.admin_name || '—' }}</span>
            <span class="price-sub">{{ row.department_name || '未分部门' }}</span>
          </div>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #orders="{ row }">
          <span class="cell-muted">{{ row.orders }}</span>
        </template>

        <template #target="{ row }">
          <span class="cell-muted">
            {{ row.target_amount > 0 ? `¥${formatPrice(row.target_amount)}` : '未设定' }}
            <template v-if="row.target_orders > 0"> / {{ row.target_orders }} 单</template>
          </span>
        </template>

        <template #achievement="{ row }">
          <div v-if="row.target_amount > 0" class="progress-cell">
            <t-progress
              :percentage="Math.min(Math.round(row.achievement * 100), 100)"
              :color="achievementColor(row.achievement)"
              :label="formatPercent(row.achievement)"
              :stroke-width="10"
            />
          </div>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #empty>
          <t-empty description="当前周期暂无业绩数据" />
        </template>
      </t-table>
    </section>

    <!-- 目标维护 -->
    <t-dialog
      v-model:visible="targetVisible"
      header="业绩目标维护"
      width="720px"
      :confirm-btn="{ content: '保存目标', theme: 'primary', loading: targetSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleTargetSave"
      @close="targetVisible = false"
    >
      <t-alert
        theme="info"
        :message="`当前周期：${rankingPeriod || period || '当月'}；留空表示清除该行目标（按行 upsert）`"
        style="margin-bottom: 12px"
      />
      <t-table
        row-key="admin_id"
        :data="targetRows"
        :columns="targetColumns"
        size="small"
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #target_admin="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.admin_real_name || row.admin_name || '—' }}</span>
            <span class="price-sub">{{ row.department_name || '未分部门' }}</span>
          </div>
        </template>
        <template #target_amount="{ row }">
          <t-input-number v-model="row.target_amount" :min="0" :precision="2" theme="column" placeholder="目标销售额" />
        </template>
        <template #target_orders="{ row }">
          <t-input-number v-model="row.target_orders" :min="0" theme="column" placeholder="目标单量" />
        </template>
        <template #empty>
          <t-empty description="暂无可维护目标的销售" />
        </template>
      </t-table>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'
import {
  ChartBarIcon,
  EditIcon,
  FlagIcon,
  MoneyIcon,
  RefreshIcon,
  StarIcon,
} from 'tdesign-icons-vue-next'

import { getMyPerformance, getPerformanceRanking, getSalesTargets, upsertSalesTargets } from '@/api/sales'
import { getDepartmentList } from '@/api/admin'
import { useUserStore } from '@/store'
import type { SalesMyPerformance, SalesRankingRow } from '@/types/interface'

defineOptions({ name: 'SalesPerformance' })

const userStore = useUserStore()
const canManageTarget = computed(() => userStore.hasPermission('sales:performance:view'))

const loading = ref(false)
const period = ref<string | undefined>(undefined)
const rankingPeriod = ref('')
const departmentID = ref<number | undefined>(undefined)
const sortBy = ref<'amount' | 'orders'>('amount')

const ranking = ref<SalesRankingRow[]>([])
const myPerformance = ref<SalesMyPerformance | null>(null)
const departmentOptions = ref<Array<{ label: string; value: number }>>([])

const columns: PrimaryTableCol<SalesRankingRow>[] = [
  { colKey: 'rank', title: '名次', width: 80, align: 'center' },
  { colKey: 'admin', title: '销售', minWidth: 170 },
  { colKey: 'amount', title: '销售额', width: 140, align: 'right' },
  { colKey: 'orders', title: '订单数', width: 100, align: 'right' },
  { colKey: 'target', title: '目标', minWidth: 190 },
  { colKey: 'achievement', title: '达成率', minWidth: 200 },
]

const targetColumns: PrimaryTableCol<TargetRow>[] = [
  { colKey: 'target_admin', title: '销售', minWidth: 180 },
  { colKey: 'target_amount', title: '目标销售额（元）', width: 220 },
  { colKey: 'target_orders', title: '目标单量', width: 160 },
]

interface TargetRow {
  admin_id: number
  admin_name: string
  admin_real_name: string
  department_name: string
  target_amount: number | undefined
  target_orders: number | undefined
}

function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

function formatPercent(value: number): string {
  return `${(Number(value || 0) * 100).toFixed(1)}%`
}

function achievementColor(value: number): string {
  if (value >= 1) return '#059669'
  if (value >= 0.6) return '#2563eb'
  return '#ea580c'
}

async function loadRanking() {
  loading.value = true
  try {
    const data = await getPerformanceRanking({
      period: period.value || undefined,
      department_id: departmentID.value,
      sort_by: sortBy.value,
    })
    ranking.value = data.items || []
    rankingPeriod.value = data.period
  } catch (error) {
    ranking.value = []
    MessagePlugin.error((error as Error).message || '加载业绩排行失败')
  } finally {
    loading.value = false
  }
}

async function loadMyPerformance() {
  try {
    myPerformance.value = await getMyPerformance({ period: period.value || undefined })
  } catch {
    myPerformance.value = null
  }
}

async function loadDepartments() {
  try {
    const data = await getDepartmentList({ status: 'active', flat: 1 })
    departmentOptions.value = (data.items || []).map((item) => ({ label: item.name, value: item.id }))
  } catch {
    departmentOptions.value = []
  }
}

async function loadAll() {
  await Promise.all([loadDepartments(), loadRanking(), loadMyPerformance()])
}

function handleSearch() {
  void loadRanking()
  void loadMyPerformance()
}

// —— 目标维护 ——
const targetVisible = ref(false)
const targetSubmitting = ref(false)
const targetRows = ref<TargetRow[]>([])

async function openTargetDialog() {
  targetVisible.value = true
  targetRows.value = []
  try {
    const [rankData, targetData] = await Promise.all([
      getPerformanceRanking({
        period: period.value || undefined,
        department_id: departmentID.value,
        sort_by: sortBy.value,
      }),
      getSalesTargets({ period: period.value || undefined, department_id: departmentID.value }),
    ])
    const existing = new Map<number, { amount: number; orders: number }>()
    for (const item of targetData.items || []) {
      if (item.scope === 'admin' && item.admin_id > 0) {
        existing.set(item.admin_id, { amount: item.target_amount, orders: item.target_orders })
      }
    }
    targetRows.value = (rankData.items || []).map((row) => {
      const saved = existing.get(row.admin_id)
      return {
        admin_id: row.admin_id,
        admin_name: row.admin_name,
        admin_real_name: row.admin_real_name,
        department_name: row.department_name,
        target_amount: saved?.amount || undefined,
        target_orders: saved?.orders || undefined,
      }
    })
    rankingPeriod.value = rankData.period
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载目标数据失败')
  }
}

async function handleTargetSave() {
  const currentPeriod = rankingPeriod.value || period.value
  if (!currentPeriod) {
    MessagePlugin.warning('请先选择统计周期')
    return
  }
  const items = targetRows.value
    .filter((row) => (row.target_amount ?? 0) > 0 || (row.target_orders ?? 0) > 0)
    .map((row) => ({
      period: currentPeriod,
      scope: 'admin',
      admin_id: row.admin_id,
      target_amount: row.target_amount ?? 0,
      target_orders: row.target_orders ?? 0,
    }))
  if (items.length === 0) {
    MessagePlugin.warning('请至少为一位销售填写目标')
    return
  }
  targetSubmitting.value = true
  try {
    await upsertSalesTargets(items)
    MessagePlugin.success(`已保存 ${items.length} 条目标`)
    targetVisible.value = false
    await loadRanking()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存目标失败')
  } finally {
    targetSubmitting.value = false
  }
}

onMounted(loadAll)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.sales-module .stat-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.rank-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 700;
  color: #64748b;
  background: #f1f5f9;
}

.rank-badge--top1 {
  color: #b45309;
  background: linear-gradient(135deg, #fde68a, #fbbf24);
}

.rank-badge--top2 {
  color: #475569;
  background: linear-gradient(135deg, #e2e8f0, #cbd5e1);
}

.rank-badge--top3 {
  color: #9a3412;
  background: linear-gradient(135deg, #fed7aa, #fdba74);
}

.progress-cell {
  min-width: 140px;
}

@media (max-width: 1200px) {
  .sales-module .stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>
