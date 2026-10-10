<template>
  <div class="page-body lifecycle-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <VerifyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">到期处置</h2>
          <p class="page-header__desc">预演到期实例将被如何处置，并支持对单个实例手动执行</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <t-alert
      :theme="preview?.enabled && !preview?.dry_run ? 'error' : 'info'"
      :message="gateMessage"
    />

    <section class="stat-grid">
      <div class="stat-card stat-card--info">
        <span class="stat-card__icon"><TimeIcon size="22" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ preview?.total ?? 0 }}</span>
          <span class="stat-card__label">待处置实例</span>
        </div>
      </div>
      <div class="stat-card stat-card--warning">
        <span class="stat-card__icon"><PoweroffIcon size="22" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ preview?.stage_counts?.suspended ?? 0 }}</span>
          <span class="stat-card__label">将暂停</span>
        </div>
      </div>
      <div class="stat-card stat-card--danger">
        <span class="stat-card__icon"><DeleteIcon size="22" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ preview?.stage_counts?.destroyed ?? 0 }}</span>
          <span class="stat-card__label">将销毁</span>
        </div>
      </div>
      <div class="stat-card stat-card--success">
        <span class="stat-card__icon"><CheckCircleIcon size="22" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ preview?.stage_counts?.active ?? 0 }}</span>
          <span class="stat-card__label">将恢复服务</span>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">预演结果</h3>
        <span class="table-card__meta">生成于 {{ formatTime(preview?.generated_at || '') }}</span>
      </div>
      <t-table
        row-key="instance_id"
        :data="preview?.items ?? []"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="undefined"
      >
        <template #instance="{ row }">
          <div>
            <span class="cell-strong">{{ row.instance_mark || '—' }}</span>
            <div class="cell-sub">{{ row.name || '—' }}</div>
          </div>
        </template>
        <template #user="{ row }">
          <div>
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <div class="cell-sub">ID {{ row.user_id }}</div>
          </div>
        </template>
        <template #stage="{ row }">
          <t-tag :theme="lifecycleStageTheme(row.stage)" variant="light" size="small" shape="round">
            {{ lifecycleStageLabel(row.stage) }}
          </t-tag>
          <span class="status-arrow">→</span>
          <t-tag :theme="lifecycleStageTheme(row.target_stage)" variant="light" size="small" shape="round">
            {{ lifecycleStageLabel(row.target_stage) }}
          </t-tag>
        </template>
        <template #action="{ row }">
          <span>{{ enforcementActionLabel(row.action) }}</span>
        </template>
        <template #expire_at="{ row }">
          <div>
            <span class="time-text">{{ formatTime(row.expire_at) }}</span>
            <div class="cell-sub">剩余 {{ row.days_left }} 天</div>
          </div>
        </template>
        <template #reason="{ row }">
          <span class="cell-muted">{{ row.reason }}</span>
          <t-tag v-if="row.capability_missing" theme="warning" variant="light" size="small" shape="round">缺能力</t-tag>
        </template>
        <template #ops="{ row }">
          <t-link
            v-permission="'lifecycle:enforce'"
            theme="danger"
            hover="color"
            :disabled="row.action === 'mark'"
            @click="openEnforce(row)"
          >
            手动执行
          </t-link>
        </template>
        <template #empty>
          <t-empty description="当前没有需要处置的实例" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="enforceVisible"
      header="手动执行到期处置"
      width="520px"
      :confirm-btn="{ content: '确认执行', theme: 'danger', loading: enforcing }"
      :cancel-btn="{ content: '取消' }"
      @confirm="submitEnforce"
      @close="enforceVisible = false"
    >
      <t-form label-align="top" :data="enforceForm" @submit.prevent>
        <t-form-item>
          <t-alert
            theme="error"
            :message="`将对实例「${currentRow?.name || currentRow?.instance_mark || ''}」执行${enforcementActionLabel(currentRow?.action || '')}，该动作真实下发上游且不可撤销。`"
          />
        </t-form-item>
        <t-form-item label="执行原因" name="reason">
          <t-textarea
            v-model="enforceForm.reason"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="选填，例如：客户已确认不再续费"
          />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import {
  CheckCircleIcon,
  DeleteIcon,
  PoweroffIcon,
  RefreshIcon,
  TimeIcon,
  VerifyIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { enforceInstance, previewEnforcement, type EnforcementPreviewItem, type EnforcementPreviewResponse } from '@/api/lifecycle'
import {
  enforcementActionLabel,
  formatTime,
  lifecycleStageLabel,
  lifecycleStageTheme,
} from '@/pages/instances/constants'

defineOptions({ name: 'LifecycleEnforcement' })

const loading = ref(false)
const enforcing = ref(false)
const preview = ref<EnforcementPreviewResponse | null>(null)

const gateMessage = computed(() => {
  const p = preview.value
  if (!p) return '正在读取策略…'
  if (!p.enabled) {
    return '当前「自动执行阶段动作」已关闭：下表仅为预演，系统不会自动暂停或销毁任何实例。如需开启请到「生命周期策略」页。'
  }
  if (p.dry_run) {
    return '自动执行已开启但处于「预演模式」：下表为将要执行的动作，系统仍不会真正下发上游。核对无误后可在「生命周期策略」页关闭预演。'
  }
  return '自动执行已开启且预演模式已关闭：下列动作将由系统真实下发上游，请谨慎确认。'
})

const columns: PrimaryTableCol<EnforcementPreviewItem>[] = [
  { colKey: 'instance', title: '实例', minWidth: 190 },
  { colKey: 'user', title: '归属用户', width: 140 },
  { colKey: 'stage', title: '阶段变化', minWidth: 200 },
  { colKey: 'action', title: '将执行', width: 110 },
  { colKey: 'expire_at', title: '到期时间', width: 160 },
  { colKey: 'reason', title: '判定依据', minWidth: 240 },
  { colKey: 'ops', title: '操作', width: 100, fixed: 'right' as const, align: 'center' as const },
]

async function loadData() {
  loading.value = true
  try {
    preview.value = await previewEnforcement()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载预演报告失败')
  } finally {
    loading.value = false
  }
}

const enforceVisible = ref(false)
const currentRow = ref<EnforcementPreviewItem | null>(null)
const enforceForm = reactive({ reason: '' })

function openEnforce(row: EnforcementPreviewItem) {
  currentRow.value = row
  enforceForm.reason = ''
  enforceVisible.value = true
}

async function submitEnforce() {
  if (!currentRow.value) return
  enforcing.value = true
  try {
    await enforceInstance(currentRow.value.instance_id, enforceForm.reason || undefined)
    MessagePlugin.success('处置已执行')
    enforceVisible.value = false
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '执行失败')
  } finally {
    enforcing.value = false
  }
}

onMounted(loadData)
</script>

<style lang="css">
@import '../shared.css';
</style>
