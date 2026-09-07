<template>
  <div class="preferences-page">
    <!-- 页头 -->
    <section class="preferences-hero">
      <div class="hero-left">
        <span class="hero-chip"><SettingIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">通知偏好</span>
          <span class="hero-desc">按事件类型选择站内信与邮件通知的接收方式。</span>
        </div>
      </div>
      <t-button
        theme="primary"
        size="large"
        :loading="saving"
        :disabled="!dirty"
        @click="handleSave"
      >
        <template #icon><SaveIcon /></template>
        保存设置
      </t-button>
    </section>

    <!-- 偏好列表 -->
    <section class="panel">
      <div class="panel-head">
        <span class="panel-title">事件偏好设置</span>
        <t-button variant="outline" size="small" :loading="loading" @click="loadPreferences">
          <template #icon><RefreshIcon /></template>
          刷新
        </t-button>
      </div>

      <t-table
        :data="preferences"
        :columns="columns"
        row-key="event"
        size="small"
        :bordered="false"
        hover
        cell-empty-content="—"
        :loading="loading"
      >
        <template #event="{ row }">
          <span class="cell-strong">{{ eventLabel(row.event) }}</span>
        </template>
        <template #inbox_on="{ row }">
          <t-switch :value="row.inbox_on" size="small" @change="(val: boolean) => { row.inbox_on = val; markDirty() }" />
        </template>
        <template #mail_on="{ row }">
          <t-switch :value="row.mail_on" size="small" @change="(val: boolean) => { row.mail_on = val; markDirty() }" />
        </template>
        <template #empty>
          <t-empty description="暂无通知偏好配置" />
        </template>
      </t-table>

      <div class="panel-foot">
        <t-button theme="primary" :loading="saving" :disabled="!dirty" @click="handleSave">
          <template #icon><SaveIcon /></template>
          保存设置
        </t-button>
        <t-button v-if="dirty" variant="outline" :disabled="loading" @click="handleReset">
          撤销修改
        </t-button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { RefreshIcon, SaveIcon, SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getMyPreferences,
  updateMyPreferences,
  type NotificationPreference,
} from '@/api/notification'

defineOptions({ name: 'UserNotifyPrefs' })

const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)

const preferences = ref<NotificationPreference[]>([])
// 保留原始副本，便于撤销
const snapshot = ref<NotificationPreference[]>([])

const columns: PrimaryTableCol<NotificationPreference>[] = [
  { colKey: 'event', title: '事件类型', width: 220 },
  { colKey: 'inbox_on', title: '站内信', width: 140 },
  { colKey: 'mail_on', title: '邮件通知', width: 140 },
]

async function loadPreferences() {
  loading.value = true
  try {
    const { data } = await getMyPreferences()
    const list = data?.list ?? []
    // 深拷贝快照，避免后续编辑影响
    preferences.value = list.map((p) => ({ ...p }))
    snapshot.value = list.map((p) => ({ ...p }))
    dirty.value = false
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载通知偏好失败')
  } finally {
    loading.value = false
  }
}

function markDirty() {
  dirty.value = true
}

function handleReset() {
  preferences.value = snapshot.value.map((p) => ({ ...p }))
  dirty.value = false
}

async function handleSave() {
  if (!preferences.value.length) return
  saving.value = true
  try {
    await updateMyPreferences({ list: preferences.value })
    snapshot.value = preferences.value.map((p) => ({ ...p }))
    dirty.value = false
    MessagePlugin.success('通知偏好已更新')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 事件类型 -> 中文标签（对齐后端 notification model 事件常量）
const EVENT_LABELS: Record<string, string> = {
  order_paid: '订单支付成功',
  renewal_success: '续费成功',
  renewal_failed: '续费失败',
  instance_expiring: '实例到期提醒',
  ticket_replied: '工单回复',
  balance_low: '余额不足预警',
  sync_failed: '上游同步失败',
  system: '系统通知',
}

function eventLabel(event?: string): string {
  if (!event) return '—'
  return EVENT_LABELS[event] || event
}

onMounted(loadPreferences)
</script>

<style scoped>
.preferences-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* 页头横幅 */
.preferences-hero {
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  border-radius: 16px;
  padding: 24px 32px;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}

.hero-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.hero-chip {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.18);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-label {
  font-size: 18px;
  font-weight: 700;
}

.hero-desc {
  font-size: 13px;
  opacity: 0.85;
}

.preferences-hero .t-button {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.3);
  color: #fff;
}

/* 面板 */
.panel {
  background: #fff;
  border-radius: 12px;
  padding: 20px 24px 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 8px;
}

.panel-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.panel-foot {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid #f1f5f9;
}

.cell-strong {
  font-weight: 600;
  color: #334155;
}

@media (max-width: 768px) {
  .preferences-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
