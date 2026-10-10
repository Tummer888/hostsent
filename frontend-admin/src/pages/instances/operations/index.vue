<template>
  <div class="page-body instances-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <HistoryIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">操作流水</h2>
          <p class="page-header__desc">跨实例查看谁在什么时候对哪台机器做了什么，含系统自动动作与失败原因</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" placeholder="实例标识 / 操作人 / 用户名" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">动作</span>
        <t-select v-model="filters.action" clearable placeholder="全部动作" :options="operationActionOptions" />
      </div>
      <div class="field">
        <span class="field__label">操作人类型</span>
        <t-select v-model="filters.operator_type" clearable placeholder="全部" :options="operatorTypeOptions" />
      </div>
      <div class="field">
        <span class="field__label">结果</span>
        <t-select v-model="filters.result" clearable placeholder="全部结果" :options="operationResultOptions" />
      </div>
      <div class="field">
        <span class="field__label">用户 ID</span>
        <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">操作记录</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #instance="{ row }">
          <div>
            <span class="cell-strong">{{ row.instance_mark || '—' }}</span>
            <div class="cell-sub">{{ row.instance_name || `记录 #${row.instance_id}` }}</div>
          </div>
        </template>
        <template #user="{ row }">
          <div>
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <div class="cell-sub">ID {{ row.user_id }}</div>
          </div>
        </template>
        <template #action="{ row }">
          <span>{{ operationActionLabel(row.action) }}</span>
        </template>
        <template #operator="{ row }">
          <div>
            <span class="cell-strong">{{ operatorTypeLabel(row.operator_type) }}</span>
            <div class="cell-sub">{{ row.operator_name || '—' }}</div>
          </div>
        </template>
        <template #result="{ row }">
          <t-tag :theme="resultTheme(row.result)" variant="light" size="small" shape="round">
            {{ operationResultLabel(row.result) }}
          </t-tag>
        </template>
        <template #status_change="{ row }">
          <span class="cell-muted">{{ row.before_status || '—' }}</span>
          <span class="status-arrow">→</span>
          <span class="cell-muted">{{ row.after_status || '—' }}</span>
        </template>
        <template #detail="{ row }">
          <t-tooltip v-if="row.error_message" :content="row.error_message">
            <span class="op-error">{{ row.error_message }}</span>
          </t-tooltip>
          <span v-else class="cell-muted">{{ paramsMessage(row) }}</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无操作流水" />
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
import FilterCard from '@/components/filter-card/index.vue'
import { onMounted, reactive, ref } from 'vue'

import { HistoryIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getInstanceOperationLogs } from '@/api/instance'
import {
  formatTime,
  operationActionLabel,
  operationActionOptions,
  operationResultLabel,
  operationResultOptions,
  operatorTypeLabel,
  operatorTypeOptions,
} from '@/pages/instances/constants'
import type { OperationLogItem } from '@/types/interface'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'InstanceOperations' })

const { isMobile } = useIsMobile()
const list = ref<OperationLogItem[]>([])
const loading = ref(false)
const total = ref(0)

const filters = reactive<{
  keyword: string | undefined
  action: string | undefined
  operator_type: string | undefined
  result: string | undefined
  user_id: string | undefined
}>({
  keyword: undefined,
  action: undefined,
  operator_type: undefined,
  result: undefined,
  user_id: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<OperationLogItem>[] = [
  { colKey: 'created_at', title: '时间', width: 160 },
  { colKey: 'instance', title: '实例', minWidth: 180 },
  { colKey: 'user', title: '归属用户', width: 140 },
  { colKey: 'action', title: '动作', width: 100 },
  { colKey: 'result', title: '结果', width: 90 },
  { colKey: 'operator', title: '操作人', width: 130 },
  { colKey: 'status_change', title: '状态变化', minWidth: 160 },
  { colKey: 'detail', title: '详情', minWidth: 200 },
]

function resultTheme(result: string): string {
  if (result === 'success') return 'success'
  if (result === 'failed') return 'danger'
  return 'default'
}

function paramsMessage(row: OperationLogItem): string {
  const raw = row.params
  if (!raw) return '—'
  try {
    const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw
    if (parsed && typeof parsed === 'object' && 'message' in parsed) {
      return String((parsed as { message?: unknown }).message || '—')
    }
    if (parsed && typeof parsed === 'object' && 'remark' in parsed) {
      return `备注：${String((parsed as { remark?: unknown }).remark || '')}`
    }
    return JSON.stringify(parsed)
  } catch {
    return String(raw)
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await getInstanceOperationLogs({
      keyword: filters.keyword,
      action: filters.action,
      operator_type: filters.operator_type,
      result: filters.result,
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载操作流水失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  loadData()
}

function handleReset() {
  filters.keyword = undefined
  filters.action = undefined
  filters.operator_type = undefined
  filters.result = undefined
  filters.user_id = undefined
  pagination.current = 1
  loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadData()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

function goMobilePage(target: number) {
  const max = Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize))
  const clamped = Math.min(Math.max(target, 1), max)
  if (clamped === mobilePage.current) return
  applyMobilePage(clamped, mobilePage.pageSize)
}

function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  loadData()
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  applyMobilePage(1, pageSize)
}

onMounted(loadData)
</script>

<style lang="css">
@import '../shared.css';
</style>
