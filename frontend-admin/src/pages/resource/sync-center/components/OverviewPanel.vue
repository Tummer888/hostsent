<template>
  <div class="overview-panel">
    <!-- 待处理事项：运维打开控制台第一眼要看到的"有没有活要干"。 -->
    <section class="stat-grid">
      <article
        v-for="card in cards"
        :key="card.key"
        class="stat-card"
        :class="[`stat-card--${card.theme}`, { 'stat-card--clickable': card.tab }]"
        :role="card.tab ? 'button' : undefined"
        :tabindex="card.tab ? 0 : undefined"
        @click="card.tab && emit('go-tab', card.tab)"
        @keydown.enter="card.tab && emit('go-tab', card.tab)"
      >
        <div class="stat-card__icon">
          <component :is="card.icon" size="20" aria-hidden="true" />
        </div>
        <div class="stat-card__text">
          <span class="stat-card__value">{{ card.value }}</span>
          <span class="stat-card__label">{{ card.label }}</span>
          <span v-if="card.hint" class="stat-card__hint">{{ card.hint }}</span>
        </div>
      </article>
    </section>

    <!-- 渠道健康总览：按链路分组，一眼看清哪些渠道在正常跑、哪些已熔断。 -->
    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">渠道同步健康</h3>
        <span class="table-card__meta">{{ healthRows.length }} 个渠道</span>
      </div>
      <p class="table-hint">
        按链路分组：自营平台是我方执行器（本地定价、平台开通），上游转售渠道的目录与生命周期以对上为准。
        熔断的渠道不会再被调度，处理完问题后到渠道页点「恢复同步」。
      </p>
      <t-table
        row-key="id"
        :data="healthRows"
        :columns="healthColumns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #name="{ row }">
          <ProviderCell :provider-id="row.id" :provider-name="row.name" :provider-kind="row.kind" />
        </template>
        <template #sync_state="{ row }">
          <t-tooltip v-if="row.sync_paused" :content="row.last_sync_error || '同步已暂停'" placement="top">
            <t-tag theme="warning" variant="light" size="small" shape="round">已熔断</t-tag>
          </t-tooltip>
          <t-tag v-else-if="row.consecutive_failures > 0" theme="danger" variant="light" size="small" shape="round">
            连续失败 {{ row.consecutive_failures }} 次
          </t-tag>
          <t-tag v-else theme="success" variant="light" size="small" shape="round">正常</t-tag>
        </template>
        <template #last_success_at="{ row }">
          <span class="cell-muted">{{ row.last_success_at ? formatTime(row.last_success_at) : '从未成功' }}</span>
        </template>
        <template #last_sync_error="{ row }">
          <t-tooltip v-if="row.last_sync_error" :content="row.last_sync_error" placement="top-left">
            <span class="cell-error">{{ row.last_sync_error }}</span>
          </t-tooltip>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #action="{ row }">
          <t-link theme="primary" hover="color" @click="goProvider(row)">渠道详情</t-link>
        </template>
        <template #empty>
          <t-empty description="暂无渠道" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import { ErrorCircleIcon, MoneyIcon, RefreshIcon, ServerIcon } from 'tdesign-icons-vue-next'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import type { ProviderInfo } from '@/types/interface'
import ProviderCell from './ProviderCell.vue'

/**
 * 概览面板：待处理事项计数 + 渠道同步健康（按链路分组）。
 *
 * 为什么需要它：重构前控制台直接落在「调度配置」表格上，运维看不出"现在有没有事"，
 * 得逐个页签翻。这里把三个最该被看见的数字（同步失败 / 待确认调价 / 未闭环差异）
 * 提到最前面，并让卡片可点击直达对应页签。
 */
const props = defineProps<{
  providers: ProviderInfo[]
  loading: boolean
  /** 链路筛选：决定统计口径（'' = 全部）。 */
  chainKind: string
  failedSyncCount: number
  pendingPriceCount: number
  openFindingCount: number
}>()

const emit = defineEmits<{
  (e: 'go-tab', tab: string): void
}>()

const router = useRouter()

function formatTime(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const filteredProviders = computed(() =>
  props.chainKind ? props.providers.filter((p) => p.kind === props.chainKind) : props.providers,
)

/** 健康行：熔断/连续失败的排前面，正常渠道沉底——列表要按"需要关注的程度"排序。 */
const healthRows = computed(() => {
  const rows = [...filteredProviders.value]
  rows.sort((a, b) => {
    const rank = (p: ProviderInfo) => (p.sync_paused ? 0 : p.consecutive_failures > 0 ? 1 : 2)
    return rank(a) - rank(b)
  })
  return rows
})

const pausedCount = computed(() => filteredProviders.value.filter((p) => p.sync_paused).length)

const cards = computed(() => [
  {
    key: 'failed',
    label: '同步失败',
    value: String(props.failedSyncCount),
    hint: props.failedSyncCount > 0 ? '点击查看失败记录' : '近期没有失败的同步运行',
    icon: ErrorCircleIcon,
    theme: props.failedSyncCount > 0 ? ('danger' as const) : ('slate' as const),
    tab: 'inbound',
  },
  {
    key: 'prices',
    label: '待确认调价',
    value: String(props.pendingPriceCount),
    hint: props.pendingPriceCount > 0 ? '确认前不会改动售价' : '没有等待处理的改价',
    icon: MoneyIcon,
    theme: props.pendingPriceCount > 0 ? ('warning' as const) : ('info' as const),
    tab: 'inbound',
  },
  {
    key: 'findings',
    label: '未闭环差异',
    value: String(props.openFindingCount),
    hint: props.openFindingCount > 0 ? '售价/到期/状态与上游不一致' : '本地与上游一致',
    icon: ServerIcon,
    theme: props.openFindingCount > 0 ? ('indigo' as const) : ('cyan' as const),
    tab: 'reconcile',
  },
  {
    key: 'paused',
    label: '熔断渠道',
    value: String(pausedCount.value),
    hint: pausedCount.value > 0 ? '已停止调度，需人工恢复' : '没有被熔断的渠道',
    icon: RefreshIcon,
    theme: pausedCount.value > 0 ? ('violet' as const) : ('teal' as const),
    tab: '',
  },
])

const healthColumns: PrimaryTableCol<ProviderInfo>[] = [
  { colKey: 'name', title: '渠道', minWidth: 200 },
  { colKey: 'provider_type', title: '类型', width: 110 },
  { colKey: 'sync_state', title: '同步状态', width: 140 },
  { colKey: 'last_success_at', title: '最近成功', width: 160 },
  { colKey: 'last_sync_error', title: '最近错误', minWidth: 200 },
  { colKey: 'action', title: '操作', width: 100, align: 'center' as const },
]

function goProvider(row: ProviderInfo) {
  const path = row.kind === 'compute' ? '/resource/platforms' : '/resource/providers'
  router.push(`${path}/${row.id}`)
}
</script>

<style scoped lang="css">
.overview-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

/* 卡片可点击时的提示：光标与悬停阴影，避免用户以为只是装饰。 */
.stat-card--clickable {
  cursor: pointer;
}

.stat-card--clickable:hover {
  box-shadow: var(--td-shadow-2, 0 2px 8px rgba(0, 0, 0, 0.08));
}

.table-hint {
  margin: 0 0 var(--space-md);
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}
</style>
