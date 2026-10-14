<template>
  <div class="schedules-panel">
    <FilterCard embedded class="tab-filters" :primary-count="1">
      <div class="field">
        <span class="field__label">渠道</span>
        <t-select v-model="filterProvider" clearable placeholder="全部渠道" :options="providerOptions" @change="load" />
      </div>
      <div class="field">
        <span class="field__label">同步类型</span>
        <t-select v-model="filterScope" clearable placeholder="全部类型" :options="scopeFilterOptions" />
      </div>
      <template #head-extra>
        <span class="filter-card__meta">
          共 {{ list.length }} 条调度<template v-if="abnormalCount"> · {{ abnormalCount }} 条异常</template>
        </span>
      </template>
    </FilterCard>

    <p class="table-hint">
      按渠道分组：每个渠道下的「同步类型」决定多久拉一次、什么时段允许跑、上次跑成没成。
      <b>拉取间隔</b>负责发现新增与变更，<b>全量对账</b>是更慢的兜底周期，用来发现上游已删除但本地还在的数据。
      <template v-if="isMobile">移动端仅查看，修改请到桌面端。</template>
    </p>

    <!-- 按渠道分组的卡片列表：一组 = 一个渠道，组内是该渠道的全部同步类型。
         为什么不用平铺大表：渠道多起来后「哪几行属于同一个渠道」只能靠渠道列横向对齐来辨，
         而运营实际关心的是"这个渠道同步正常吗"——按渠道成组后一眼看一个渠道的全貌。 -->
    <div v-if="!groups.length && !loading" class="empty-block">
      <t-empty description="暂无调度配置，请先在渠道页启用同步" />
    </div>

    <section v-for="group in groups" :key="group.providerId" class="provider-group surface-card">
      <header class="provider-group__head">
        <div class="provider-group__title">
          <ProviderCell
            :provider-id="group.providerId"
            :provider-name="group.providerName"
            :provider-kind="group.providerKind"
          />
          <t-tag v-if="group.abnormal" theme="danger" variant="light" size="small" shape="round">
            {{ group.abnormal }} 条异常
          </t-tag>
          <t-tag v-else theme="success" variant="light" size="small" shape="round">全部正常</t-tag>
        </div>
        <span class="provider-group__meta">
          {{ group.rows.length }} 个同步类型 · 最近执行 {{ group.lastRunText }}
        </span>
      </header>

      <div class="scope-grid">
        <article
          v-for="row in group.rows"
          :key="row.id"
          class="scope-card"
          :class="{ 'scope-card--abnormal': isAbnormal(row) }"
        >
          <div class="scope-card__head">
            <t-tag :theme="scopeTheme(row.scope)" variant="light" size="small" shape="round">
              {{ row.scope_name || row.scope }}
            </t-tag>
            <t-tag :theme="row.enabled ? 'success' : 'default'" variant="light" size="small" shape="round">
              {{ row.enabled ? '启用' : '停用' }}
            </t-tag>
            <t-tooltip v-if="row.last_error" :content="row.last_error" placement="top-left">
              <t-tag :theme="statusTheme(row.last_status)" variant="light" size="small" shape="round">
                {{ statusLabel(row.last_status) }}
              </t-tag>
            </t-tooltip>
            <t-tag v-else :theme="statusTheme(row.last_status)" variant="light" size="small" shape="round">
              {{ statusLabel(row.last_status) }}
            </t-tag>
          </div>

          <dl class="scope-card__facts">
            <div class="fact">
              <dt>拉取间隔</dt>
              <dd>{{ humanInterval(row.interval_seconds) }}</dd>
            </div>
            <div class="fact">
              <dt>全量对账</dt>
              <dd>{{ humanInterval(row.full_sync_interval_seconds) }}</dd>
            </div>
            <div class="fact">
              <dt>执行时段</dt>
              <dd>{{ windowLabel(row) }}</dd>
            </div>
            <div class="fact">
              <dt>下次执行</dt>
              <dd>{{ row.next_run_at ? formatTime(row.next_run_at) : '待触发' }}</dd>
            </div>
          </dl>

          <div class="scope-card__actions">
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
        </article>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'

import { getSyncScheduleList } from '@/api/admin'
import type { ProviderInfo, SyncScheduleInfo } from '@/types/interface'
import FilterCard from '@/components/filter-card/index.vue'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import ProviderCell from './ProviderCell.vue'

/**
 * 调度配置页签：按「渠道」分组展示各同步类型的节奏与上次结果。
 *
 * 为什么从平铺表格改成分组卡片（用户反馈"三个类型混在一起比较混乱、不好快速定位"）：
 * 资源池 / 区域 / 实例这几种同步在多渠道下混在一张长表里，同一渠道的几行只能靠渠道列
 * 横向对齐来辨认。改成「一个渠道一张卡片、卡内按同步类型分块」后，渠道与类型各自成层：
 * 先按渠道收敛范围，再在组内看类型，且异常渠道会自动排到前面。
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
const filterScope = ref<string | undefined>(undefined)

const providerOptions = computed(() => props.providers.map((item) => ({ label: item.name, value: item.id })))

/** 类型筛选项从当前数据现取，避免写死一份可能与后端失配的清单。 */
const scopeFilterOptions = computed(() => {
  const seen = new Map<string, string>()
  for (const row of list.value) {
    if (!seen.has(row.scope)) seen.set(row.scope, row.scope_name || row.scope)
  }
  return [...seen.entries()].map(([value, label]) => ({ label, value }))
})

interface ProviderGroup {
  providerId: number
  providerName: string
  providerKind: string
  rows: SyncScheduleInfo[]
  abnormal: number
  lastRunText: string
}

/** 异常判定：上次执行失败。这是唯一需要人工介入的调度状态。 */
function isAbnormal(row: SyncScheduleInfo): boolean {
  return row.last_status === 'failed'
}

const abnormalCount = computed(() => list.value.filter((row) => isAbnormal(row)).length)

const groups = computed<ProviderGroup[]>(() => {
  const filtered = filterScope.value ? list.value.filter((row) => row.scope === filterScope.value) : list.value
  const byProvider = new Map<number, ProviderGroup>()
  for (const row of filtered) {
    let group = byProvider.get(row.provider_id)
    if (!group) {
      group = {
        providerId: row.provider_id,
        providerName: row.provider_name,
        providerKind: row.provider_kind,
        rows: [],
        abnormal: 0,
        lastRunText: '—',
      }
      byProvider.set(row.provider_id, group)
    }
    group.rows.push(row)
    if (isAbnormal(row)) group.abnormal += 1
  }
  const out = [...byProvider.values()]
  for (const group of out) {
    // 组内按固定类型顺序排（资源池 → 区域 → 实例 → 其余）：跨渠道看到的顺序一致，
    // 便于横向对比"这个渠道的实例同步是不是也失败了"。
    group.rows.sort((a, b) => scopeOrder(a.scope) - scopeOrder(b.scope))
    const times = group.rows
      .map((row) => (row.last_run_at ? new Date(row.last_run_at).getTime() : 0))
      .filter((t) => t > 0)
    group.lastRunText = times.length ? formatTime(new Date(Math.max(...times)).toISOString()) : '—'
  }
  // 异常渠道排前面：需要关注的对象不该沉到列表底部。
  out.sort((a, b) => b.abnormal - a.abnormal || a.providerId - b.providerId)
  return out
})

/** 组内类型顺序：与"资源池、区域、实例"的阅读顺序一致。 */
function scopeOrder(scope: string): number {
  const order = ['pool', 'region', 'instance', 'catalog', 'price', 'image', 'stock']
  const idx = order.indexOf(scope)
  return idx === -1 ? order.length : idx
}

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
watch(
  () => props.chainKind,
  () => {
    filterProvider.value = undefined
    filterScope.value = undefined
    void load()
  },
)

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

.empty-block {
  padding: var(--space-xl) 0;
}

/* 渠道分组卡片 */
.provider-group {
  padding: var(--space-md) var(--space-lg);
  margin-bottom: var(--space-md);
}

.provider-group__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
  padding-bottom: var(--space-sm);
  margin-bottom: var(--space-md);
  border-bottom: 1px solid var(--color-border);
}

.provider-group__title {
  display: inline-flex;
  align-items: center;
  gap: var(--space-sm);
}

.provider-group__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* 组内同步类型卡片：自适应多列，窄屏一列 */
.scope-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: var(--space-md);
}

.scope-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-sm) var(--space-md);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md, 6px);
}

/* 失败的类型加左侧红条：一眼扫到哪个渠道的哪个类型坏了 */
.scope-card--abnormal {
  border-left: 3px solid #dc2626;
}

.scope-card__head {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.scope-card__facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px var(--space-sm);
  margin: 0;
}

.fact {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.fact dt {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.fact dd {
  margin: 0;
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scope-card__actions {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding-top: 4px;
}
</style>
