<template>
  <div class="page-body sales-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <UsergroupIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">客户归属</h2>
          <p class="page-header__desc">维护用户与销售的服务关系；归属决定订单提成，变更受保护期约束</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="stat-grid">
      <article class="stat-card stat-card--green">
        <span class="stat-card__icon">
          <UsergroupIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ myStats.active_customers }}</span>
          <span class="stat-card__label">我的客户数</span>
        </div>
      </article>
      <article class="stat-card stat-card--blue">
        <span class="stat-card__icon">
          <AddIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ myStats.month_new_customers }}</span>
          <span class="stat-card__label">本月新增</span>
        </div>
      </article>
      <article class="stat-card stat-card--orange">
        <span class="stat-card__icon">
          <MoneyIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(myStats.amount) }}</span>
          <span class="stat-card__label">本月成交额</span>
        </div>
      </article>
    </section>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="用户名 / 邮箱 / 手机" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">部门</span>
          <t-select
            v-model="filters.department_id"
            clearable
            filterable
            placeholder="全部部门"
            :options="departmentOptions"
            @change="handleDepartmentChange"
          />
        </div>
        <div class="field">
          <span class="field__label">销售</span>
          <t-select
            v-model="filters.admin_id"
            clearable
            filterable
            placeholder="全部销售"
            :options="salesOptions"
          />
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
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">归属客户</h3>
        <span class="table-card__meta">共 {{ total }} 位客户</span>
      </div>

      <t-tabs v-model="activeTab" class="sales-tabs" @change="handleTabChange">
        <t-tab-panel value="assigned" label="已归属客户">
          <t-table
            row-key="user_id"
            :data="customers"
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
                <span class="cell-strong">{{ row.username }}</span>
                <span class="price-sub">{{ row.email || row.phone || '—' }}</span>
              </div>
            </template>

            <template #admin="{ row }">
              <div class="price-cell">
                <span class="cell-strong">{{ row.admin_real_name || row.admin_name || '—' }}</span>
                <span class="price-sub">{{ row.department_name || '未分部门' }}</span>
              </div>
            </template>

            <template #protect_until="{ row }">
              <t-tag
                v-if="row.protected"
                theme="warning"
                variant="light"
                size="small"
                shape="round"
              >
                保护至 {{ formatDate(row.protect_until) }}
              </t-tag>
              <span v-else class="cell-muted">{{ row.protect_until ? formatDate(row.protect_until) : '—' }}</span>
            </template>

            <template #total_consume="{ row }">
              <span class="price-main">¥{{ formatPrice(row.total_consume) }}</span>
            </template>

            <template #registered_at="{ row }">
              <span class="time-text">{{ formatTime(row.registered_at) }}</span>
            </template>

            <template #action="{ row }">
              <div class="action-cell">
                <MobileAction
                  v-if="isMobile"
                  :options="buildMobileActionOptions([
                    { content: '分配归属', value: 'assign', disabled: () => row.protected },
                    { content: '释放归属', value: 'release' },
                    { content: '变更历史', value: 'history' },
                  ])"
                  @select="(value) => handleMobileAction(value, row)"
                />
                <template v-else>
                  <t-tooltip :content="row.protected ? `保护期内（至 ${formatDate(row.protect_until)}）不可改派` : ''" :disabled="!row.protected">
                    <t-link
                      :theme="row.protected ? 'default' : 'primary'"
                      :disabled="row.protected"
                      hover="color"
                      @click="openAssignDialog(row, 'assign')"
                    >
                      分配归属
                    </t-link>
                  </t-tooltip>
                  <t-link theme="warning" hover="color" @click="openAssignDialog(row, 'release')">释放归属</t-link>
                  <t-link theme="primary" hover="color" @click="openHistory(row)">变更历史</t-link>
                </template>
              </div>
            </template>

            <template #empty>
              <t-empty description="暂无归属客户" />
            </template>
          </t-table>
        </t-tab-panel>

        <t-tab-panel value="unassigned" label="未归属池">
          <div class="pool-toolbar">
            <t-space size="small">
              <t-input
                v-model="poolKeyword"
                placeholder="用户名 / 邮箱 / 手机"
                clearable
                style="width: 220px"
                @enter="handleSearchUnassigned"
              />
              <t-button theme="primary" variant="outline" @click="handleSearchUnassigned">查询</t-button>
            </t-space>
            <t-space size="small">
              <span class="pool-toolbar__hint">已选 {{ selectedUnassigned.length }} 位</span>
              <t-button theme="primary" :disabled="selectedUnassigned.length === 0" @click="openBatchAssign">
                批量分配
              </t-button>
            </t-space>
          </div>

          <t-table
            row-key="user_id"
            :data="unassignedList"
            :columns="unassignedColumns"
            :loading="poolLoading"
            size="small"
            hover
            table-layout="fixed"
            cell-empty-content="—"
            :selected-row-keys="selectedUnassigned"
            :pagination="isMobile ? undefined : poolPagination"
            @select-change="handlePoolSelectChange"
            @page-change="handlePoolPageChange"
          >
            <template #username="{ row }">
              <div class="price-cell">
                <span class="cell-strong">{{ row.username }}</span>
                <span class="price-sub">{{ row.email || row.phone || '—' }}</span>
              </div>
            </template>
            <template #total_consume="{ row }">
              <span class="price-main">¥{{ formatPrice(row.total_consume) }}</span>
            </template>
            <template #registered_at="{ row }">
              <span class="time-text">{{ formatTime(row.registered_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="未归属池已清空" />
            </template>
          </t-table>

          <MobilePagination
            v-if="isMobile"
            :current="poolMobilePage.current"
            :page-size="poolMobilePage.pageSize"
            :total="poolMobilePage.total"
            @go="goPoolMobilePage"
            @page-size="handlePoolPageSizeChange"
          />
        </t-tab-panel>
      </t-tabs>

      <MobilePagination
        v-if="isMobile && activeTab === 'assigned'"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <!-- 分配 / 释放归属 -->
    <t-dialog
      v-model:visible="assignVisible"
      :header="assignForm.mode === 'assign' ? '分配客户归属' : '释放客户归属'"
      width="520px"
      :confirm-btn="{ content: assignForm.mode === 'assign' ? '确认分配' : '确认释放', theme: assignForm.mode === 'assign' ? 'primary' : 'warning' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleAssignConfirm"
      @close="assignVisible = false"
    >
      <t-form label-align="top" :data="assignForm" @submit.prevent>
        <t-alert
          v-if="assignForm.mode === 'release'"
          theme="warning"
          message="释放后该客户不再计入任何销售的业绩，订单提成按释放后状态处理。"
          style="margin-bottom: 12px"
        />
        <t-form-item label="客户">
          <t-input :model-value="assignForm.label" disabled />
        </t-form-item>
        <t-form-item v-if="assignForm.mode === 'assign'" label="目标销售" name="admin_id" :rules="[{ required: true, message: '请选择目标销售' }]">
          <t-select
            v-model="assignForm.admin_id"
            filterable
            placeholder="按部门筛选后选择销售"
            :options="candidateOptions"
            :loading="candidateLoading"
          />
        </t-form-item>
        <t-form-item label="变更原因" name="reason" :rules="[{ required: true, message: '请填写变更原因（≥5 字）' }, { min: 5, message: '变更原因至少 5 个字' }]">
          <t-textarea
            v-model="assignForm.reason"
            :autosize="{ minRows: 3, maxRows: 5 }"
            :placeholder="assignForm.mode === 'assign' ? '必填，说明分配理由（≥5 字）' : '必填，说明释放理由（≥5 字）'"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 变更历史 -->
    <t-drawer
      v-model:visible="historyVisible"
      :header="`归属变更历史 · ${historyUser}`"
      size="520px"
      :footer="false"
    >
      <t-loading :loading="historyLoading">
        <t-timeline v-if="historyList.length > 0">
          <t-timeline-item
            v-for="item in historyList"
            :key="item.id"
            :label="formatTime(item.created_at)"
            :theme="item.status === 'active' ? 'primary' : 'default'"
          >
            <div class="history-item">
              <div class="history-item__head">
                <span class="cell-strong">{{ item.admin_real_name || item.admin_name || '—' }}</span>
                <t-tag :theme="item.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
                  {{ item.status === 'active' ? '生效中' : '已释放' }}
                </t-tag>
              </div>
              <p class="history-item__meta">{{ item.department_name || '未分部门' }}</p>
              <p class="history-item__reason">原因：{{ item.reason || '—' }}</p>
              <p class="history-item__meta">
                生效 {{ formatDate(item.effective_at) }}
                <template v-if="item.protect_until"> · 保护至 {{ formatDate(item.protect_until) }}</template>
                <template v-if="item.released_at"> · 释放于 {{ formatDate(item.released_at) }}</template>
              </p>
            </div>
          </t-timeline-item>
        </t-timeline>
        <t-empty v-else description="暂无变更记录" />
      </t-loading>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { AddIcon, MoneyIcon, RefreshIcon, SearchIcon, UsergroupIcon } from 'tdesign-icons-vue-next'

import {
  assignCustomer,
  getCustomerRelations,
  getMyPerformance,
  getSalesCandidates,
  getSalesCustomers,
  getUnassignedCustomers,
  releaseCustomer,
} from '@/api/sales'
import { getDepartmentList } from '@/api/admin'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import type {
  SalesCandidateInfo,
  SalesCustomerInfo,
  SalesMyPerformance,
  SalesRelationInfo,
  SalesUnassignedInfo,
} from '@/types/interface'

defineOptions({ name: 'SalesCustomers' })

const route = useRoute()
const router = useRouter()
const { isMobile } = useIsMobile()

const loading = ref(false)
const total = ref(0)
const customers = ref<SalesCustomerInfo[]>([])
const activeTab = ref<'assigned' | 'unassigned'>('assigned')

const myStats = ref<SalesMyPerformance>({
  period: '',
  admin_id: 0,
  admin_name: '',
  department_name: '',
  amount: 0,
  orders: 0,
  target_amount: 0,
  target_orders: 0,
  achievement: 0,
  rank: 0,
  dept_members: 0,
  active_customers: 0,
  month_new_customers: 0,
})

const filters = reactive<{
  keyword: string
  department_id: number | undefined
  admin_id: number | undefined
}>({
  keyword: '',
  department_id: undefined,
  admin_id: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<SalesCustomerInfo>[] = [
  { colKey: 'user_id', title: '用户 ID', width: 100 },
  { colKey: 'username', title: '客户', minWidth: 220 },
  { colKey: 'admin', title: '归属销售', minWidth: 170 },
  { colKey: 'effective_at', title: '归属生效', width: 160 },
  { colKey: 'protect_until', title: '保护期', width: 170 },
  { colKey: 'total_consume', title: '累计消费', width: 130, align: 'right' },
  { colKey: 'registered_at', title: '注册时间', width: 170 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 220, fixed: 'right' as const, align: 'center' as const },
]

// —— 未归属池 ——
const poolLoading = ref(false)
const unassignedList = ref<SalesUnassignedInfo[]>([])
const poolKeyword = ref('')
const selectedUnassigned = ref<Array<string | number>>([])
const poolPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const poolMobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const unassignedColumns: PrimaryTableCol<SalesUnassignedInfo>[] = [
  { colKey: 'row-select', type: 'multiple', width: 50 },
  { colKey: 'user_id', title: '用户 ID', width: 100 },
  { colKey: 'username', title: '客户', minWidth: 220 },
  { colKey: 'total_consume', title: '累计消费', width: 130, align: 'right' },
  { colKey: 'registered_at', title: '注册时间', width: 170 },
]

// —— 销售 / 部门下拉 ——
const departmentOptions = ref<Array<{ label: string; value: number }>>([])
const salesOptions = ref<Array<{ label: string; value: number }>>([])
const candidateOptions = ref<Array<{ label: string; value: number }>>([])
const candidateLoading = ref(false)

// —— 分配 / 释放 ——
const assignVisible = ref(false)
const assignForm = reactive<{
  mode: 'assign' | 'release'
  userID: number
  label: string
  admin_id: number | undefined
  reason: string
}>({
  mode: 'assign',
  userID: 0,
  label: '',
  admin_id: undefined,
  reason: '',
})

// —— 变更历史 ——
const historyVisible = ref(false)
const historyLoading = ref(false)
const historyUser = ref('')
const historyList = ref<SalesRelationInfo[]>([])

function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

function formatDate(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

async function loadCustomers() {
  loading.value = true
  try {
    const data = await getSalesCustomers({
      keyword: filters.keyword || undefined,
      admin_id: filters.admin_id,
      department_id: filters.department_id,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    customers.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    customers.value = []
    MessagePlugin.error((error as Error).message || '加载归属客户失败')
  } finally {
    loading.value = false
  }
}

async function loadUnassigned() {
  poolLoading.value = true
  try {
    const data = await getUnassignedCustomers({
      keyword: poolKeyword.value || undefined,
      page: poolPagination.current,
      page_size: poolPagination.pageSize,
    })
    unassignedList.value = data.items || []
    poolPagination.total = data.meta.total
    poolMobilePage.total = data.meta.total
  } catch (error) {
    unassignedList.value = []
    MessagePlugin.error((error as Error).message || '加载未归属客户失败')
  } finally {
    poolLoading.value = false
  }
}

async function loadMyStats() {
  try {
    myStats.value = await getMyPerformance({})
  } catch {
    // 统计失败不阻断页面主体
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

async function loadCandidates(departmentID?: number) {
  candidateLoading.value = true
  try {
    const data = await getSalesCandidates({ department_id: departmentID })
    const items: SalesCandidateInfo[] = data.items || []
    candidateOptions.value = items.map((item) => ({
      label: `${item.real_name || item.username}${item.department_name ? `（${item.department_name}·${item.active_customers}户）` : ''}`,
      value: item.admin_id,
    }))
    // 列表筛选用同一份候选，避免额外请求
    if (!departmentID) {
      salesOptions.value = candidateOptions.value
    }
  } catch {
    candidateOptions.value = []
  } finally {
    candidateLoading.value = false
  }
}

async function loadAll() {
  await Promise.all([loadMyStats(), loadDepartments(), loadCandidates(), loadCustomers(), loadUnassigned()])
}

function handleSearch() {
  pagination.current = 1
  loadCustomers()
}

function handleResetFilters() {
  filters.keyword = ''
  filters.department_id = undefined
  filters.admin_id = undefined
  pagination.current = 1
  loadCustomers()
}

function handleDepartmentChange(value: unknown) {
  const deptID = typeof value === 'number' ? value : undefined
  filters.admin_id = undefined
  loadCandidates(deptID)
}

function handleSearchUnassigned() {
  poolPagination.current = 1
  selectedUnassigned.value = []
  loadUnassigned()
}

function handleTabChange(value: unknown) {
  if (value === 'unassigned') {
    loadUnassigned()
  }
}

function handlePoolSelectChange(keys: Array<string | number>) {
  selectedUnassigned.value = keys
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadCustomers()
}

function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  pagination.current = clamped
  mobilePage.current = clamped
  loadCustomers()
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  pagination.pageSize = pageSize
  pagination.current = 1
  mobilePage.current = 1
  loadCustomers()
}

function handlePoolPageChange(pageInfo: PageInfo) {
  poolPagination.current = pageInfo.current
  poolPagination.pageSize = pageInfo.pageSize
  poolMobilePage.current = pageInfo.current
  poolMobilePage.pageSize = pageInfo.pageSize
  loadUnassigned()
}

function goPoolMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(poolMobilePage.total / poolMobilePage.pageSize)))
  if (clamped === poolMobilePage.current) return
  poolPagination.current = clamped
  poolMobilePage.current = clamped
  loadUnassigned()
}

function handlePoolPageSizeChange(pageSize: number) {
  poolMobilePage.pageSize = pageSize
  poolPagination.pageSize = pageSize
  poolPagination.current = 1
  poolMobilePage.current = 1
  loadUnassigned()
}

function openAssignDialog(row: SalesCustomerInfo, mode: 'assign' | 'release') {
  assignForm.mode = mode
  assignForm.userID = row.user_id
  assignForm.label = `${row.username}（ID ${row.user_id}）`
  assignForm.admin_id = undefined
  assignForm.reason = ''
  assignVisible.value = true
  if (mode === 'assign') {
    loadCandidates(filters.department_id)
  }
}

function openBatchAssign() {
  assignForm.mode = 'assign'
  assignForm.userID = 0
  assignForm.label = `批量分配 ${selectedUnassigned.value.length} 位未归属客户`
  assignForm.admin_id = undefined
  assignForm.reason = ''
  assignVisible.value = true
  loadCandidates(filters.department_id)
}

async function handleAssignConfirm() {
  const reason = assignForm.reason.trim()
  if (reason.length < 5) {
    MessagePlugin.warning('变更原因至少 5 个字')
    return
  }
  if (assignForm.mode === 'assign' && !assignForm.admin_id) {
    MessagePlugin.warning('请选择目标销售')
    return
  }
  try {
    if (assignForm.mode === 'assign') {
      const targets = assignForm.userID > 0 ? [assignForm.userID] : selectedUnassigned.value.map((key) => Number(key))
      for (const userID of targets) {
        await assignCustomer({ user_id: userID, admin_id: assignForm.admin_id as number, reason })
      }
      MessagePlugin.success(`已分配 ${targets.length} 位客户`)
      selectedUnassigned.value = []
    } else {
      await releaseCustomer({ user_id: assignForm.userID, reason })
      MessagePlugin.success('已释放客户归属')
    }
    assignVisible.value = false
    await Promise.all([loadCustomers(), loadUnassigned(), loadMyStats()])
  } catch (error) {
    MessagePlugin.error((error as Error).message || '操作失败')
  }
}

async function openHistory(row: SalesCustomerInfo) {
  historyUser.value = row.username
  historyVisible.value = true
  historyLoading.value = true
  try {
    const data = await getCustomerRelations(row.user_id)
    historyList.value = data.items || []
  } catch (error) {
    historyList.value = []
    MessagePlugin.error((error as Error).message || '加载变更历史失败')
  } finally {
    historyLoading.value = false
  }
}

function handleMobileAction(value: string | number | Record<string, any>, row: SalesCustomerInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'assign':
      openAssignDialog(row, 'assign')
      break
    case 'release':
      openAssignDialog(row, 'release')
      break
    case 'history':
      openHistory(row)
      break
  }
}

// 从用户列表带 keyword 跳转过来时自动带入筛选（doc86 §4.1.10）
function syncKeywordFromRoute() {
  const keyword = (route.query.keyword as string) || ''
  if (keyword) {
    filters.keyword = keyword
    void router.replace({ query: {} })
  }
}

onMounted(() => {
  syncKeywordFromRoute()
  loadAll()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.sales-tabs {
  margin-top: 4px;
}

.sales-module .pool-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 4px 0 12px;
}

.sales-module .pool-toolbar__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.sales-module .history-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-bottom: 4px;
}

.sales-module .history-item__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sales-module .history-item__meta {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.sales-module .history-item__reason {
  margin: 0;
  font-size: 13px;
  color: #334155;
}
</style>
