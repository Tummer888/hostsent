<template>
  <div class="provider-schedules">
    <t-alert v-if="!syncEnabled" theme="warning" class="schedules-warn">
      <template #message>
        该{{ unit }}的<b>同步总开关已关闭</b>，下面各类型即使显示「启用」也不会被执行。
        开启后调度器才会按这些节奏拉取数据。
      </template>
    </t-alert>

    <div class="schedules-head">
      <div class="schedules-head__meta">
        <span>共 {{ list.length }} 个同步类型</span>
        <span class="schedules-head__hint">按类型分别配置节奏；改动即刻生效，无需重启</span>
      </div>
      <t-button size="small" variant="outline" :loading="loading" @click="load">
        <template #icon><RefreshIcon /></template>
        刷新
      </t-button>
    </div>

    <!-- 总开关：调度行是"配了什么"，这里是"能不能跑"。
         旧页面把它写成「启用实例同步」，看起来只管实例，实际是整个渠道的总闸。 -->
    <t-form-item :label="`${unit}同步总开关`" class="schedules-master">
      <t-switch v-model="masterEnabled" :loading="masterSaving" @change="handleMasterChange" />
      <span class="schedules-master__help">
        关闭后所有同步类型都停止执行（保留下面的配置）；开启后按各自节奏调度。
      </span>
    </t-form-item>

    <t-loading :loading="loading">
      <t-empty v-if="!list.length && !loading" :description="emptyText" />

      <t-table
        v-else
        row-key="id"
        :data="list"
        :columns="columns"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #scope="{ row }">
          <t-tag :theme="scopeTheme(row.scope)" variant="light" size="small" shape="round">
            {{ row.scope_name || row.scope }}
          </t-tag>
        </template>

        <template #interval="{ row }">
          <span class="cell-num">{{ humanInterval(row.interval_seconds) }}</span>
        </template>

        <template #full_interval="{ row }">
          <span class="cell-num cell-muted">{{ humanInterval(row.full_sync_interval_seconds) }}</span>
        </template>

        <template #window="{ row }">
          <span class="cell-muted">{{ windowLabel(row) }}</span>
        </template>

        <template #last_status="{ row }">
          <t-tooltip v-if="row.last_error" :content="row.last_error" placement="top">
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
            <t-link theme="primary" hover="color" @click="openConfig(row)">配置</t-link>
            <t-link theme="primary" hover="color" :disabled="syncing === row.id" @click="trigger(row)">
              立即同步
            </t-link>
          </div>
        </template>
      </t-table>
    </t-loading>

    <ScheduleConfigDialog :row="editing" @saved="load" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { createSyncTask, getSyncScheduleList, setProviderSyncEnabled } from '@/api/admin'
import type { SyncScheduleInfo } from '@/types/interface'
import ScheduleConfigDialog from '@/pages/resource/sync-center/components/ScheduleConfigDialog.vue'

/**
 * 渠道级「同步与调度」面板：按同步类型逐条配置节奏，与同步运维控制台同一套口径。
 *
 * 为什么不再用「同步间隔（秒）+ 启用实例同步」两个字段：那套是单间隔模型 ——
 * 一个数字管所有同步类型，后端调度器早就改成 (渠道 × 类型) 各一行
 * （sync_schedules，含间隔/全量对账/优先级/时段）。渠道页继续显示旧字段的后果是
 * 用户在这里改的值跟实际调度对不上，看起来"接口字符还是旧的"。
 *
 * 现场拉取而不是回显落库快照：调度行会由同步引擎按能力描述符懒创建，
 * 刚建好的渠道第一次打开时需要触发这次创建（后端的 ListSchedules 会顺带补齐）。
 */
const props = defineProps<{
  providerId: number
  /** 渠道当前的总开关状态（channel 编辑弹窗里由父组件持有并回存）。 */
  syncEnabled: boolean
  /** 文案单位：渠道 / 平台。 */
  unit?: string
}>()

const emit = defineEmits<{
  (e: 'update:syncEnabled', value: boolean): void
}>()

const unit = computed(() => props.unit || '渠道')

const list = ref<SyncScheduleInfo[]>([])
const loading = ref(false)
const editing = ref<SyncScheduleInfo | null>(null)
const syncing = ref<number | null>(null)

const masterEnabled = ref(props.syncEnabled)
const masterSaving = ref(false)
// 父组件（编辑弹窗初值 / 详情页回填）刷新总开关时同步本地开关。
// 不这么做的话，先打开详情页、再打开编辑弹窗会沿用第一次的初值。
watch(
  () => props.syncEnabled,
  (v) => {
    masterEnabled.value = v
  },
)

const columns: PrimaryTableCol<SyncScheduleInfo>[] = [
  { colKey: 'scope', title: '同步类型', width: 110 },
  { colKey: 'interval', title: '拉取间隔', width: 110 },
  { colKey: 'full_interval', title: '全量对账', width: 110 },
  { colKey: 'window', title: '执行时段', width: 110 },
  { colKey: 'last_status', title: '上次结果', width: 100 },
  { colKey: 'next_run_at', title: '下次执行', minWidth: 150 },
  { colKey: 'action', title: '操作', width: 140, fixed: 'right', align: 'center' },
]

const emptyText = computed(() =>
  masterEnabled.value
    ? '该渠道尚未生成调度配置。同步引擎会在首次同步或刷新时按渠道能力自动补齐。'
    : '该渠道的同步总开关已关闭，尚未生成调度配置。开启总开关并刷新后会自动补齐。',
)

async function load() {
  if (!props.providerId) return
  loading.value = true
  try {
    const data = await getSyncScheduleList({ provider_id: props.providerId, page: 1, page_size: 100 })
    list.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载调度配置失败')
  } finally {
    loading.value = false
  }
}

function openConfig(row: SyncScheduleInfo) {
  editing.value = row
}

/**
 * 总开关走专用窄接口（只动 sync_enabled 一个字段）。
 * 不用渠道更新接口：那是整行替换语义，只带一个开关会把名称/地址/状态一起清空。
 */
async function handleMasterChange(value: boolean) {
  masterSaving.value = true
  try {
    await setProviderSyncEnabled(props.providerId, value)
    emit('update:syncEnabled', value)
    MessagePlugin.success(value ? '已开启同步' : '已关闭同步')
    await load()
  } catch (error) {
    masterEnabled.value = !value
    MessagePlugin.error((error as Error).message || '保存同步开关失败')
  } finally {
    masterSaving.value = false
  }
}

async function trigger(row: SyncScheduleInfo) {
  syncing.value = row.id
  try {
    await createSyncTask({ provider_id: props.providerId, task_type: row.scope })
    MessagePlugin.success(`已触发「${row.scope_name || row.scope}」同步，稍后刷新查看结果`)
    setTimeout(load, 1500)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '触发同步失败')
  } finally {
    syncing.value = null
  }
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

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function scopeTheme(scope: string): string {
  if (scope === 'catalog') return 'primary'
  if (scope === 'price') return 'warning'
  if (scope === 'pool') return 'success'
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

onMounted(load)

defineExpose({ reload: load })
</script>

<style scoped lang="css">
/* 面板根容器：给内嵌（详情页页签 / 编辑弹窗）时一个稳定的垂直节奏。 */
.provider-schedules {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.schedules-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
}

.schedules-head__meta {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.schedules-head__hint {
  font-size: 12px;
}

.schedules-warn {
  margin-bottom: var(--space-md);
}

.schedules-master {
  margin-bottom: var(--space-sm);
}

.schedules-master__help {
  margin-left: 8px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
