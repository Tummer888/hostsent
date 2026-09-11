<template>
  <div class="page-body notify-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">通知模板</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">模板列表</h3>
        <span class="table-card__meta">共 {{ list.length }} 个模板</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        :pagination="null"
      >
        <template #event="{ row }">
          <span class="cell-strong">{{ row.event }}</span>
        </template>
        <template #title_tpl="{ row }">
          <span class="cell-muted">{{ row.title_tpl || '—' }}</span>
        </template>
        <template #inbox_on="{ row }">
          <t-switch
            :value="row.inbox_on"
            @change="(val: boolean) => handleToggle(row, 'inbox_on', val)"
          />
        </template>
        <template #mail_on="{ row }">
          <t-switch
            :value="row.mail_on"
            @change="(val: boolean) => handleToggle(row, 'mail_on', val)"
          />
        </template>
        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
            {{ statusLabel(row.status) }}
          </t-tag>
        </template>
        <template #updated_at="{ row }">
          <span class="time-text">{{ formatTime(row.updated_at) }}</span>
        </template>
        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无通知模板" />
        </template>
      </t-table>
    </section>

    <!-- 编辑模板抽屉 -->
    <t-drawer
      v-model:visible="formVisible"
      header="编辑通知模板"
      size="520px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <div v-if="currentRow" class="tpl-form">
        <div class="tpl-form__row">
          <span class="tpl-form__label">事件</span>
          <t-tag variant="light" size="small">{{ currentRow.event }}</t-tag>
        </div>
        <t-form label-align="top" :data="form" @submit.prevent>
          <t-form-item label="标题模板" name="title_tpl">
            <t-input
              v-model="form.title_tpl"
              placeholder="支持变量占位，如 {order_no}"
              :maxlength="200"
            />
          </t-form-item>
          <t-form-item label="正文模板" name="content_tpl">
            <t-textarea
              v-model="form.content_tpl"
              placeholder="支持变量占位，如 {amount}、{expire_at}"
              :autosize="{ minRows: 6, maxRows: 14 }"
            />
          </t-form-item>
          <t-form-item label="站内信投递" name="inbox_on">
            <t-switch v-model="form.inbox_on" />
          </t-form-item>
          <t-form-item label="邮件投递" name="mail_on">
            <t-switch v-model="form.mail_on" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" :options="statusOptions" />
          </t-form-item>
        </t-form>
      </div>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { RefreshIcon, SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getTemplates,
  updateTemplate,
  type NotificationTemplateItem,
} from '@/api/notification'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'NotifyTemplates' })

const loading = ref(false)
const { isMobile } = useIsMobile()
const saving = ref(false)
const list = ref<NotificationTemplateItem[]>([])

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const statusMap: Record<string, { label: string; theme: string }> = {
  active: { label: '启用', theme: 'success' },
  disabled: { label: '禁用', theme: 'default' },
}

function statusLabel(s: string): string {
  return statusMap[s]?.label || s
}

function statusTheme(s: string): string {
  return statusMap[s]?.theme || 'default'
}

function formatTime(value?: string): string {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

const columns: PrimaryTableCol[] = [
  { colKey: 'event', title: '事件', width: 180 },
  { colKey: 'title_tpl', title: '标题模板', minWidth: 220 },
  { colKey: 'inbox_on', title: '站内信', width: 90, align: 'center', cell: 'inbox_on' },
  { colKey: 'mail_on', title: '邮件', width: 90, align: 'center', cell: 'mail_on' },
  { colKey: 'status', title: '状态', width: 90, cell: 'status' },
  { colKey: 'updated_at', title: '更新时间', width: 160 },
  { colKey: 'action', title: '操作', width: 100, fixed: 'right', align: 'center', cell: 'action' },
]

async function loadData() {
  loading.value = true
  try {
    list.value = (await getTemplates()) || []
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载通知模板失败')
  } finally {
    loading.value = false
  }
}

// —— 编辑抽屉 ——
const formVisible = ref(false)
const currentRow = ref<NotificationTemplateItem | null>(null)
const form = reactive<{
  title_tpl: string
  content_tpl: string
  inbox_on: boolean
  mail_on: boolean
  status: 'active' | 'disabled'
}>({
  title_tpl: '',
  content_tpl: '',
  inbox_on: false,
  mail_on: false,
  status: 'active',
})

function openEdit(row: NotificationTemplateItem) {
  currentRow.value = row
  form.title_tpl = row.title_tpl || ''
  form.content_tpl = row.content_tpl || ''
  form.inbox_on = !!row.inbox_on
  form.mail_on = !!row.mail_on
  form.status = row.status
  formVisible.value = true
}

async function handleSave() {
  if (!currentRow.value) return
  if (!form.title_tpl.trim()) {
    MessagePlugin.warning('请输入标题模板')
    return
  }
  if (!form.content_tpl.trim()) {
    MessagePlugin.warning('请输入正文模板')
    return
  }
  saving.value = true
  try {
    await updateTemplate(currentRow.value.id, {
      title_tpl: form.title_tpl.trim(),
      content_tpl: form.content_tpl.trim(),
      inbox_on: form.inbox_on,
      mail_on: form.mail_on,
      status: form.status,
    })
    MessagePlugin.success('模板已更新')
    formVisible.value = false
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '保存模板失败')
  } finally {
    saving.value = false
  }
}

// —— 行内开关切换 ——
async function handleToggle(
  row: NotificationTemplateItem,
  field: 'inbox_on' | 'mail_on',
  value: boolean,
) {
  try {
    await updateTemplate(row.id, { [field]: value } as { inbox_on?: boolean; mail_on?: boolean })
    row[field] = value
    MessagePlugin.success(value ? '已开启' : '已关闭')
  } catch (e) {
    MessagePlugin.error((e as Error).message || '更新失败')
  }
}

onMounted(loadData)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: NotificationTemplateItem) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.tpl-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.tpl-form__row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.tpl-form__label {
  flex: 0 0 88px;
  font-size: 13px;
  color: var(--color-muted-foreground, #94a3b8);
}
</style>
