<template>
  <div class="page-body payment-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <VerifyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">渠道对账</h2>
          <p class="page-header__desc">按渠道 + 账期核对本地已支付单与渠道账单：本地取 payment_orders 已支付合计，差异非 0 记为疑似异常。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button theme="primary" :loading="running" @click="openRunDialog">
          <template #icon><AddIcon aria-hidden="true" /></template>
          发起对账
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadRecords">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section v-if="latest" class="recon-grid">
      <div class="recon-result surface-card">
        <div class="recon-row">
          <span class="recon-row__label">对账记录号</span>
          <span class="recon-row__value">{{ latest.recon_no }}</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">渠道 / 账期</span>
          <span class="recon-row__value">{{ latest.channel_code || '全部渠道' }} · {{ latest.period || '全量' }}</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">状态</span>
          <t-tag :theme="reconStatusTheme(latest.status)" variant="light" size="small" shape="round">
            {{ reconStatusLabel(latest.status) }}
          </t-tag>
        </div>
      </div>

      <div class="recon-result surface-card">
        <div class="recon-row">
          <span class="recon-row__label">本地已支付</span>
          <span class="recon-row__value">¥{{ formatPrice(latest.local_amount) }} / {{ latest.local_count }} 笔</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">渠道账单</span>
          <span class="recon-row__value">¥{{ formatPrice(latest.channel_amount) }} / {{ latest.channel_count }} 笔</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">差异</span>
          <span class="recon-row__value" :class="latest.diff === 0 ? 'recon-diff-ok' : 'recon-diff-bad'">
            ¥{{ formatPrice(latest.diff) }}
          </span>
        </div>
      </div>

      <div class="recon-result surface-card">
        <div class="recon-row">
          <span class="recon-row__label">对账时间</span>
          <span class="recon-row__value">{{ formatTime(latest.created_at) }}</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">说明</span>
          <span class="recon-row__value recon-note">
            渠道侧账单据未接入下载接口前以本地值占位，差异恒为 0；接入后此处将反映真实渠道账单。
          </span>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">对账记录</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="records"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #recon_no="{ row }">
          <span class="cell-strong">{{ row.recon_no }}</span>
        </template>

        <template #channel_code="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.channel_code || '全部渠道' }}</span>
            <span class="price-sub">{{ row.period || '全量' }}</span>
          </div>
        </template>

        <template #local="{ row }">
          <span class="cell-muted">¥{{ formatPrice(row.local_amount) }} / {{ row.local_count }}</span>
        </template>

        <template #channel="{ row }">
          <span class="cell-muted">¥{{ formatPrice(row.channel_amount) }} / {{ row.channel_count }}</span>
        </template>

        <template #diff="{ row }">
          <span :class="row.diff === 0 ? 'amount-income' : 'amount-expense'">¥{{ formatPrice(row.diff) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="reconStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ reconStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #empty>
          <t-empty description="暂无对账记录，点击右上角「发起对账」" />
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
      v-model:visible="runVisible"
      header="发起渠道对账"
      width="460px"
      :confirm-btn="{ content: '开始对账', theme: 'primary', loading: running }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleRun"
      @close="runVisible = false"
    >
      <t-alert theme="info" message="对账口径：本地已支付单合计（按支付渠道与账期过滤）。渠道编码留空表示全部渠道。" style="margin-bottom: 12px" />
      <t-form label-align="top" :data="runForm" @submit.prevent>
        <t-form-item label="支付渠道" name="channel_code">
          <t-select v-model="runForm.channel_code" clearable placeholder="全部渠道" :options="channelOptions" />
        </t-form-item>
        <t-form-item label="账期" name="period">
          <t-input v-model="runForm.period" placeholder="如 2026-09，留空为全量" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, RefreshIcon, VerifyIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getChannelList, getReconRecords, runReconcile } from '@/api/payment'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import { formatPrice, formatTime, reconStatusLabel, reconStatusTheme } from '@/pages/payment/constants'
import type { ReconRecordInfo } from '@/types/interface'

defineOptions({ name: 'PaymentRecon' })

const records = ref<ReconRecordInfo[]>([])
const loading = ref(false)
const running = ref(false)
const total = ref(0)
const { isMobile } = useIsMobile()

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const latest = computed<ReconRecordInfo | null>(() => records.value[0] ?? null)

const channelOptions = ref<Array<{ label: string; value: string }>>([])

const columns: PrimaryTableCol<ReconRecordInfo>[] = [
  { colKey: 'recon_no', title: '对账记录号', minWidth: 180 },
  { colKey: 'channel_code', title: '渠道 / 账期', minWidth: 160 },
  { colKey: 'local', title: '本地（金额/笔数）', minWidth: 150 },
  { colKey: 'channel', title: '渠道（金额/笔数）', minWidth: 150 },
  { colKey: 'diff', title: '差异', width: 110 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '对账时间', width: 170 },
]

async function loadRecords() {
  loading.value = true
  try {
    const data = await getReconRecords(pagination.current, pagination.pageSize)
    records.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载对账记录失败')
  } finally {
    loading.value = false
  }
}

async function loadChannelOptions() {
  try {
    const data = await getChannelList({ page: 1, page_size: 100 })
    channelOptions.value = (data.items || []).map((c) => ({ label: `${c.name}（${c.channel_code}）`, value: c.channel_code }))
  } catch {
    channelOptions.value = []
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadRecords()
}

function goMobilePage(target: number) {
  const maxPage = Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize))
  const clamped = Math.min(Math.max(target, 1), maxPage)
  if (clamped === mobilePage.current) return
  handlePageChange({ current: clamped, pageSize: mobilePage.pageSize } as PageInfo)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  handlePageChange({ current: 1, pageSize } as PageInfo)
}

const runVisible = ref(false)
const runForm = reactive<{ channel_code?: string; period?: string }>({ channel_code: undefined, period: undefined })

function openRunDialog() {
  runForm.channel_code = undefined
  runForm.period = undefined
  runVisible.value = true
}

async function handleRun() {
  running.value = true
  try {
    const result = await runReconcile({ channel_code: runForm.channel_code, period: runForm.period })
    if (result.diff === 0) {
      MessagePlugin.success(`对账完成（${result.recon_no}）：账实一致`)
    } else {
      MessagePlugin.warning(`对账完成（${result.recon_no}）：存在差异 ¥${formatPrice(result.diff)}`)
    }
    runVisible.value = false
    pagination.current = 1
    loadRecords()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '发起对账失败')
  } finally {
    running.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadRecords(), loadChannelOptions()])
})
</script>

<style lang="css" scoped>
.recon-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.recon-note {
  font-size: 12px;
  font-weight: 400;
  color: var(--color-muted-foreground);
  text-align: right;
  max-width: 260px;
  line-height: 1.6;
}

@media (max-width: 1200px) {
  .recon-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style lang="css">
@import '../shared.css';
</style>
