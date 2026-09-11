<template>
  <div class="page-body lifecycle-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <HistoryIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">到期管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button theme="primary" :loading="scanning" @click="handleScan">
          <template #icon><RocketIcon aria-hidden="true" /></template>
          立即扫描
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="实例标识 / 实例名 / 用户名" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">生命周期阶段</span>
          <t-select v-model="filters.stage" clearable placeholder="全部阶段" :options="stageOptions" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">到期实例</h3>
        <span class="table-card__meta">共 {{ total }} 个实例</span>
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
          <div class="product-cell">
            <span class="cell-strong">{{ row.instance_mark }}</span>
            <span class="product-sub">{{ row.name || '—' }}</span>
          </div>
        </template>
        <template #user="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="product-sub">ID {{ row.user_id }}</span>
          </div>
        </template>
        <template #product_name="{ row }">
          <t-tag variant="light" size="small" shape="round">{{ row.product_name || '—' }}</t-tag>
        </template>
        <template #unit_price="{ row }">
          <span class="cell-strong">¥{{ formatAmount(row.unit_price) }}</span>
          <span class="product-sub">/{{ billingModeLabel(row.billing_mode) }}</span>
        </template>
        <template #expire_at="{ row }">
          <div class="product-cell">
            <span class="time-text">{{ formatTime(row.expire_at) }}</span>
            <span class="product-sub">剩余 {{ row.days_left }} 天</span>
          </div>
        </template>
        <template #stage="{ row }">
          <t-tag :theme="stageTheme(row.stage)" variant="light" size="small" shape="round">
            {{ stageLabel(row.stage) }}
          </t-tag>
        </template>
        <template #auto_renew="{ row }">
          <t-tag v-if="row.auto_renew" theme="success" variant="light" size="small" shape="round">已开启</t-tag>
          <t-tag v-else theme="default" variant="light" size="small" shape="round">未开启</t-tag>
        </template>
        <template #action="{ row }">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '代续费', value: 'renew', theme: 'default' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
        </template>
        <template #empty>
          <t-empty description="暂无到期实例" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="page.current"
        :page-size="page.size"
        :total="total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <!-- 代续费弹窗 -->
    <t-dialog
      v-model:visible="renewVisible"
      header="管理员代续费"
      :confirm-btn="{ content: '确认续费', theme: 'primary', loading: renewing }"
      @confirm="submitRenew"
    >
      <div class="renew-form">
        <div class="renew-form__item">
          <span class="renew-form__label">实例</span>
          <span>{{ currentRow?.instance_mark }}（{{ currentRow?.product_name }}）</span>
        </div>
        <div class="renew-form__item">
          <span class="renew-form__label">当前到期</span>
          <span>{{ formatTime(currentRow?.expire_at || '') }}</span>
        </div>
        <div class="renew-form__item">
          <span class="renew-form__label">续费周期数</span>
          <t-input-number v-model="renewForm.period_count" :min="1" :max="36" theme="column" style="width: 160px" />
        </div>
        <div class="renew-form__item">
          <span class="renew-form__label">自定义金额</span>
          <t-input-number v-model="renewForm.amount" :min="0" :step="0.01" theme="column" style="width: 160px" />
          <span class="renew-form__tip">0 表示按产品单价 × 周期数计算</span>
        </div>
        <div class="renew-form__item">
          <span class="renew-form__label">备注</span>
          <t-textarea v-model="renewForm.remark" placeholder="选填" :autosize="{ minRows: 2, maxRows: 4 }" />
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { HistoryIcon, RefreshIcon, RocketIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getExpiringInstances, renewInstance, triggerLifecycleScan, type ExpiringInstanceItem } from '@/api/lifecycle'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import MobilePagination from '@/components/mobile-pagination/index.vue'

defineOptions({ name: 'LifecycleExpiring' })

const loading = ref(false)
const { isMobile } = useIsMobile()
const scanning = ref(false)
const list = ref<ExpiringInstanceItem[]>([])
const total = ref(0)
const filters = reactive({ keyword: '', stage: '' })
const page = reactive({ current: 1, size: 10 })

const stageOptions = [
  { label: '运行中', value: 'active' },
  { label: '即将到期', value: 'expiring' },
  { label: '宽限期', value: 'grace' },
  { label: '已暂停', value: 'suspended' },
  { label: '待销毁', value: 'destroyed' },
]

const stageMap: Record<string, { label: string; theme: string }> = {
  active: { label: '运行中', theme: 'success' },
  expiring: { label: '即将到期', theme: 'warning' },
  grace: { label: '宽限期', theme: 'warning' },
  suspended: { label: '已暂停', theme: 'danger' },
  destroyed: { label: '待销毁', theme: 'danger' },
}

const billingMap: Record<string, string> = {
  hourly: '小时',
  daily: '天',
  monthly: '月',
  yearly: '年',
}

function stageLabel(stage: string): string {
  return stageMap[stage]?.label || stage
}

function stageTheme(stage: string): string {
  return stageMap[stage]?.theme || 'default'
}

function billingModeLabel(mode: string): string {
  return billingMap[mode] || mode
}

function formatTime(value?: string): string {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

function formatAmount(value?: number): string {
  return (value ?? 0).toFixed(2)
}

const columns: PrimaryTableCol[] = [
  { colKey: 'instance', title: '实例', width: 200 },
  { colKey: 'user', title: '用户', width: 150 },
  { colKey: 'product_name', title: '产品', width: 140 },
  { colKey: 'unit_price', title: '续费单价', width: 120 },
  { colKey: 'expire_at', title: '到期时间', width: 180 },
  { colKey: 'stage', title: '阶段', width: 100 },
  { colKey: 'auto_renew', title: '自动续费', width: 90 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 90 },
]

const pagination = computed(() => ({
  current: page.current,
  pageSize: page.size,
  total: total.value,
  showJumper: true,
}))

async function loadData() {
  loading.value = true
  try {
    const resp = await getExpiringInstances({
      keyword: filters.keyword || undefined,
      stage: filters.stage || undefined,
      page: page.current,
      page_size: page.size,
    })
    list.value = resp.items || []
    total.value = resp.meta?.total ?? 0
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载到期实例失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  page.current = 1
  loadData()
}

function handleReset() {
  filters.keyword = ''
  filters.stage = ''
  page.current = 1
  loadData()
}

function handlePageChange(info: PageInfo) {
  page.current = info.current
  page.size = info.pageSize
  loadData()
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const totalPages = Math.max(1, Math.ceil(total.value / page.size))
  const clamped = Math.min(Math.max(target, 1), totalPages)
  if (clamped === page.current) return
  void applyMobilePage(clamped, page.size)
}

async function applyMobilePage(current: number, pageSize: number) {
  page.current = current
  page.size = pageSize
  await loadData()
}

function handleMobilePageSizeChange(pageSize: number) {
  void applyMobilePage(1, pageSize)
}


async function handleScan() {
  scanning.value = true
  try {
    const msg = await triggerLifecycleScan()
    MessagePlugin.success(msg || '扫描完成')
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '扫描失败')
  } finally {
    scanning.value = false
  }
}

// —— 代续费 ——
const renewVisible = ref(false)
const renewing = ref(false)
const currentRow = ref<ExpiringInstanceItem | null>(null)
const renewForm = reactive({ period_count: 1, amount: 0, remark: '' })

function openRenew(row: ExpiringInstanceItem) {
  currentRow.value = row
  renewForm.period_count = 1
  renewForm.amount = 0
  renewForm.remark = ''
  renewVisible.value = true
}

async function submitRenew() {
  if (!currentRow.value) return
  renewing.value = true
  try {
    await renewInstance(currentRow.value.id, {
      period_count: renewForm.period_count,
      amount: renewForm.amount || undefined,
      remark: renewForm.remark || undefined,
    })
    MessagePlugin.success('代续费成功，已延长实例到期时间')
    renewVisible.value = false
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '代续费失败')
  } finally {
    renewing.value = false
  }
}

onMounted(loadData)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: ExpiringInstanceItem) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'renew':
      openRenew(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.renew-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 4px 0;
}
.renew-form__item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.renew-form__label {
  flex: 0 0 90px;
  text-align: right;
  line-height: 32px;
  color: var(--td-text-color-primary);
}
.renew-form__tip {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}
</style>
