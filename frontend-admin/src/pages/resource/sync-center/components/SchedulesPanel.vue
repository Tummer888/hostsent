<template>
  <div class="schedules-panel">
    <FilterCard embedded class="tab-filters" :primary-count="1">
      <div class="field">
        <span class="field__label">渠道</span>
        <t-select v-model="filterProvider" clearable placeholder="全部渠道" :options="providerOptions" @change="load" />
      </div>
      <template #head-extra>
        <span class="filter-card__meta">共 {{ list.length }} 条调度</span>
      </template>
    </FilterCard>

    <p class="table-hint">
      每个「渠道 × 同步类型」一行：决定多久拉一次、什么时段允许跑、上次跑成没成。
      「全量对账」是更慢的兜底周期，用于发现上游已删除但我们这边还在的商品/资源池。
      <template v-if="isMobile">移动端仅查看，修改请到桌面端。</template>
    </p>

    <t-table
      row-key="id"
      :data="list"
      :columns="columns"
      :loading="loading"
      size="small"
      hover
      table-layout="fixed"
      cell-empty-content="—"
    >
      <template #provider="{ row }">
        <ProviderCell :provider-id="row.provider_id" :provider-name="row.provider_name" :provider-kind="row.provider_kind" />
      </template>
      <template #scope="{ row }">
        <t-tag :theme="scopeTheme(row.scope)" variant="light" size="small" shape="round">
          {{ row.scope_name || row.scope }}
        </t-tag>
      </template>
      <template #interval="{ row }">
        <span class="cell-num">{{ humanInterval(row.interval_seconds) }}</span>
      </template>
      <template #full_interval="{ row }">
        <span class="cell-muted">{{ humanInterval(row.full_sync_interval_seconds) }}</span>
      </template>
      <template #window="{ row }">
        <span class="cell-muted">{{ windowLabel(row) }}</span>
      </template>
      <template #enabled="{ row }">
        <t-tag :theme="row.enabled ? 'success' : 'default'" variant="light" size="small" shape="round">
          {{ row.enabled ? '启用' : '停用' }}
        </t-tag>
      </template>
      <template #last_status="{ row }">
        <t-tooltip v-if="row.last_error" :content="row.last_error" placement="top-left">
          <t-tag :theme="statusTheme(row.last_status)" variant="light" size="small" shape="round">
            {{ statusLabel(row.last_status) }}
          </t-tag>
        </t-tooltip>
        <t-tag v-else :theme="statusTheme(row.last_status)" variant="light" size="small" shape="round">
          {{ statusLabel(row.last_status) }}
        </t-tag>
      </template>
      <template #next_run_at="{ row }">
        <span class="cell-muted">{{ row.next_run_at ? formatTime(row.next_run_at) : '待触发' }}</span>
      </template>
      <template #action="{ row }">
        <div class="action-cell">
          <MobileAction
            v-if="isMobile"
            :options="buildMobileActionOptions([{ content: '立即同步', value: 'sync' }])"
            @select="() => emit('trigger', row)"
          />
          <template v-else>
            <t-link theme="primary" hover="color" @click="emit('configure', row)">配置</t-link>
            <t-link theme="primary" hover="color" @click="emit('trigger', row)">立即同步</t-link>
          </template>
        </div>
      </template>
      <template #empty>
        <t-empty description="暂无调度配置，请先在渠道页启用同步" />
      </template>
    </t-table>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getSyncScheduleList } from '@/api/admin'
import type { ProviderInfo, SyncScheduleInfo } from '@/types/interface'
import FilterCard from '@/components/filter-card/index.vue'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import ProviderCell from './ProviderCell.vue'

/**
 * 调度配置页签：渠道 × 同步类型的节奏与上次结果。
 *
 * 拆成独立组件的原因：它是控制台里唯一"写操作密集"的页签（改间隔/时段/优先级），
 * 和只读的入站/对账视图生命周期不同；放在主文件里会让那个文件继续膨胀。
 */
const props = defineProps<{
  chainKind: string
  providers: ProviderInfo[]
}>()

const emit = defineEmits<{
  (e: 'configure', row: SyncScheduleInfo): void
  (e: 'trigger', row: SyncScheduleInfo): void
}>()

const { isMobile } = useIsMobile()

const list = ref<SyncScheduleInfo[]>([])
const loading = ref(false)
const filterProvider = ref<number | undefined>(undefined)

const providerOptions = computed(() => props.providers.map((item) => ({ label: item.name, value: item.id })))

function formatTime(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function humanInterval(seconds: number): string {
  if (!seconds || seconds <= 0) return '—'
  if (seconds % 86400 === 0) return `${seconds / 86400} 天`
  if (seconds % 3600 === 0) return `${seconds / 3600} 小时`
  if (seconds % 60 === 0) return `${seconds / 60} 分钟`
  return `${seconds} 秒`
}

function windowLabel(row: SyncScheduleInfo): string {
  if (row.window_start === null || row.window_end === null) return '不限'
  return `${row.window_start} 点 - ${row.window_end} 点`
}

function scopeTheme(scope: string): string {
  if (scope === 'catalog') return 'primary'
  if (scope === 'price') return 'warning'
  if (scope === 'pool') return 'success'
  if (scope === 'region') return 'default'
  if (scope === 'instance') return 'danger'
  return 'default'
}

const statusLabels: Record<string, string> = {
  pending: '待执行',
  running: '执行中',
  success: '成功',
  failed: '失败',
  skipped: '已跳过',
}

function statusLabel(status: string): string {
  return statusLabels[status] || status || '未执行'
}

function statusTheme(status: string): string {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'default'
}

const columns: PrimaryTableCol<SyncScheduleInfo>[] = [
  { colKey: 'provider', title: '渠道', minWidth: 180 },
  { colKey: 'scope', title: '同步类型', width: 110 },
  { colKey: 'interval', title: '同步间隔', width: 100 },
  { colKey: 'full_interval', title: '全量对账', width: 100 },
  { colKey: 'window', title: '执行时段', width: 110 },
  { colKey: 'enabled', title: '状态', width: 80 },
  { colKey: 'last_status', title: '上次结果', width: 100 },
  { colKey: 'next_run_at', title: '下次执行', minWidth: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 140, fixed: 'right' as const, align: 'center' as const },
]

async function load() {
  loading.value = true
  try {
    const data = await getSyncScheduleList({
      provider_id: filterProvider.value,
      provider_kind: props.chainKind || undefined,
      page: 1,
      page_size: 200,
    })
    list.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载调度配置失败')
  } finally {
    loading.value = false
  }
}

// 链路切换后按新链路重查（服务端过滤，口径与分页一致）。
watch(() => props.chainKind, () => {
  filterProvider.value = undefined
  void load()
})

onMounted(load)

defineExpose({ reload: load })
</script>

<style scoped lang="css">
.tab-filters {
  margin: 0 0 var(--space-md);
}

.table-hint {
  margin: 0 0 var(--space-md);
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.cell-num {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.cell-muted {
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
