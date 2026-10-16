<template>
  <t-dialog
    v-model:visible="visible"
    :header="`调度配置 · ${row?.scope_name || row?.scope || ''}`"
    width="560px"
    :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
    @confirm="handleSave"
    @close="close"
  >
    <t-form v-if="row" label-align="top" :data="form" @submit.prevent>
      <t-form-item label="同步间隔" :help="intervalHelp">
        <IntervalField v-model:seconds="form.interval_seconds" :min-seconds="30" />
      </t-form-item>
      <t-form-item label="全量对账周期" :help="fullIntervalHelp">
        <IntervalField v-model:seconds="form.full_sync_interval_seconds" :min-seconds="300" />
      </t-form-item>
      <t-form-item label="优先级" help="数值越大越先触发；多条调度同时到期时按此排序">
        <t-input-number v-model="form.priority" :min="0" theme="column" />
      </t-form-item>
      <t-form-item label="启用">
        <t-switch v-model="form.enabled" />
      </t-form-item>
      <t-form-item label="允许执行时段" help="限定只在某几个小时之间执行，避开上游业务高峰；关闭表示全天可执行">
        <t-switch v-model="form.window_enabled" />
      </t-form-item>
      <t-form-item v-if="form.window_enabled" label="时段（小时 0-23）">
        <t-space align="center">
          <t-input-number v-model="form.window_start" :min="0" :max="23" theme="column" />
          <span>点 至</span>
          <t-input-number v-model="form.window_end" :min="0" :max="23" theme="column" />
          <span>点</span>
        </t-space>
      </t-form-item>
      <t-form-item
        label="立即生效"
        help="开启后保存即把下次执行时间置空，调度器下一轮扫描（每分钟一次）就会触发，不用等原定周期"
      >
        <t-switch v-model="form.reset_next_run" />
      </t-form-item>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'

import { updateSyncSchedule } from '@/api/admin'
import type { SyncScheduleInfo } from '@/types/interface'
import IntervalField from '@/pages/resource/sync-center/components/IntervalField.vue'

/**
 * 调度配置弹窗（同步中心与渠道详情/编辑共用一份）。
 *
 * 为什么抽成组件：这段表单原先只长在同步中心控制台里，渠道页面又各自维护了一套
 * 「同步间隔（秒）+ 启用实例同步」的旧字段。两处口径不一致的后果是运营在渠道页改了
 * 间隔、调度器却按另一套配置跑（用户反馈"这里显示的接口字符还是旧的一个接口"）。
 * 现在字段集、校验与请求只有一个来源，改一处两边同时生效。
 */
const props = defineProps<{
  row?: SyncScheduleInfo | null
}>()

const emit = defineEmits<{
  (e: 'saved'): void
}>()

const visible = ref(false)
const saving = ref(false)

const form = reactive({
  interval_seconds: 3600,
  full_sync_interval_seconds: 86400,
  priority: 0,
  enabled: true,
  window_enabled: false,
  window_start: 0,
  window_end: 6,
  reset_next_run: false,
})

/**
 * 两个周期的解释按同步类型分别写。
 *
 * 只写一句「目录/价格建议小时级，实例建议分钟级」用户看不出这两个字段到底在同步什么、
 * 为什么默认值差这么多。按类型给具体建议更省事：实例状态变化快（开关机、到期），
 * 目录与价格变化慢且上游接口更重。
 */
const intervalHelp = computed(() => {
  const scope = props.row?.scope
  const base: Record<string, string> = {
    catalog: '多久拉一次上游商品目录（新增/改配/下架）。目录变动慢，建议 6 小时~1 天。',
    price: '多久核对一次上游价格。上游调价会进「待确认调价」，建议 6 小时~1 天。',
    pool: '多久刷新一次资源池容量（CPU/内存/磁盘用量）。建议 1~6 小时。',
    region: '多久刷新一次区域与节点清单。区域变动很少，建议 1 天。',
    instance: '多久拉一次实例状态、IP 与到期时间。状态变化快，建议 5~15 分钟。',
  }
  const hint = (scope && base[scope]) || '每隔多久向上游拉取一次本类型的数据。'
  return `${hint} 间隔越短越及时，但上游接口调用越频繁；小于 30 秒会被拒绝。`
})

const fullIntervalHelp = computed(() => {
  const scope = props.row?.scope
  const base: Record<string, string> = {
    catalog: '多久做一次全量对账：找出上游已删除、但我们这边还在的商品并标记下架。建议 1 天。',
    price: '多久做一次全量价格核对（不只看有变化的）。建议 1 天。',
    pool: '多久做一次全量池对账：上游已不存在的池会被标记离线。建议 1 天。',
    region: '多久做一次全量区域对账。建议 1 天。',
    instance: '多久做一次全量实例对账：上游已不存在的实例会被标记离线。建议 1 天。',
  }
  const hint = (scope && base[scope]) || '全量对账的周期。'
  return `${hint} 它比「同步间隔」慢得多，用来兜住增量同步发现不了的删除类变化；不得小于 5 分钟。`
})

// 打开时用传入的调度行填充表单：同一个弹窗被列表里任意一行复用。
watch(
  () => props.row,
  (row) => {
    if (!row) return
    form.interval_seconds = row.interval_seconds
    form.full_sync_interval_seconds = row.full_sync_interval_seconds
    form.priority = row.priority
    form.enabled = row.enabled
    form.window_enabled = row.window_start !== null && row.window_end !== null
    form.window_start = row.window_start ?? 0
    form.window_end = row.window_end ?? 6
    form.reset_next_run = false
    visible.value = true
  },
)

function close() {
  visible.value = false
}

async function handleSave() {
  if (!props.row) return
  saving.value = true
  try {
    await updateSyncSchedule(props.row.id, {
      interval_seconds: form.interval_seconds,
      full_sync_interval_seconds: form.full_sync_interval_seconds,
      priority: form.priority,
      enabled: form.enabled,
      window_clear: !form.window_enabled,
      window_start: form.window_enabled ? form.window_start : null,
      window_end: form.window_enabled ? form.window_end : null,
      reset_next_run: form.reset_next_run,
    })
    MessagePlugin.success('调度配置已保存')
    visible.value = false
    emit('saved')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存调度配置失败')
  } finally {
    saving.value = false
  }
}
</script>
