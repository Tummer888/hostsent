<template>
  <div class="page-body notify-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SendIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">消息群发</h2>
          <p class="page-header__desc">
            三步向导：选目标 → 写内容 → 预览确认。站内信落库即送达，邮件与短信进投递队列（可在发送日志追踪与重投）。
          </p>
        </div>
      </div>
    </header>

    <section class="surface-card wizard-card">
      <t-steps :current="step" layout="horizontal" :style="{ marginBottom: 'var(--space-lg)' }">
        <t-step-item title="选择目标" :description="targetSummary" />
        <t-step-item title="编辑内容" :description="contentSummary" />
        <t-step-item title="确认发送" description="预览无误后提交" />
      </t-steps>

      <!-- Step 1 目标 -->
      <div v-if="step === 0" class="wizard-body">
        <t-radio-group v-model="target.mode" variant="default-filled" @change="handleModeChange">
          <t-radio-button value="all">全部用户</t-radio-button>
          <t-radio-button value="group">按用户组</t-radio-button>
          <t-radio-button value="users">指定用户</t-radio-button>
          <t-radio-button value="filter">条件筛选</t-radio-button>
        </t-radio-group>

        <!-- 按用户组 -->
        <div v-if="target.mode === 'group'" class="mode-panel">
          <t-select
            v-model="target.user_group_id"
            placeholder="请选择用户组"
            :options="groupOptions"
            :loading="groupsLoading"
            filterable
          />
          <p v-if="selectedGroup" class="field-help">
            该组当前 {{ selectedGroup.member_count }} 位成员（发送时以实际命中为准）。
          </p>
        </div>

        <!-- 指定用户 -->
        <div v-if="target.mode === 'users'" class="mode-panel">
          <t-select
            v-model="target.user_ids"
            multiple
            filterable
            remote
            :loading="usersLoading"
            :options="userOptions"
            :filter="handleUserSearch"
            placeholder="输入用户名 / 邮箱 / 手机号搜索，最多选 500 人"
            :max="500"
            @remove="handleUserRemove"
          />
          <p class="field-help">已选 {{ target.user_ids.length }} 人（上限 500）。</p>
        </div>

        <!-- 条件筛选 -->
        <div v-if="target.mode === 'filter'" class="mode-panel">
          <div class="form-grid">
            <t-form-item label="注册时间起">
              <t-date-picker v-model="target.filter.registered_after" enable-time-picker allow-input clearable placeholder="选填" />
            </t-form-item>
            <t-form-item label="注册时间止">
              <t-date-picker v-model="target.filter.registered_before" enable-time-picker allow-input clearable placeholder="选填" />
            </t-form-item>
            <t-form-item label="用户等级">
              <t-input v-model="target.filter.tier" placeholder="选填，如 vip / normal" clearable />
            </t-form-item>
            <t-form-item label="账号状态">
              <t-select v-model="target.filter.status" clearable placeholder="全部" :options="userStatusOptions" />
            </t-form-item>
            <t-form-item label="余额下限（元）">
              <t-input v-model="target.filter.min_balance" placeholder="选填" clearable />
            </t-form-item>
            <t-form-item label="余额上限（元）">
              <t-input v-model="target.filter.max_balance" placeholder="选填" clearable />
            </t-form-item>
            <t-form-item label="是否有实例">
              <t-select v-model="hasInstanceValue" clearable placeholder="不限" :options="hasInstanceOptions" />
            </t-form-item>
          </div>
          <t-space size="small">
            <t-button theme="primary" variant="outline" :loading="countLoading" @click="handleCountTargets">
              统计命中人数
            </t-button>
            <span v-if="filterCount !== null" class="field-help">当前条件命中 {{ filterCount }} 人。</span>
          </t-space>
        </div>
      </div>

      <!-- Step 2 内容 -->
      <div v-else-if="step === 1" class="wizard-body">
        <t-form label-align="top" @submit.prevent>
          <t-form-item label="标题">
            <t-input v-model="content.title" placeholder="站内信与邮件主题，如「系统维护通知」" :maxlength="200" />
          </t-form-item>
          <t-form-item label="正文">
            <t-textarea
              ref="bodyRef"
              v-model="content.content"
              placeholder="支持 {var} 变量；逐用户变量开启时按每个用户的资料渲染"
              :autosize="{ minRows: 6, maxRows: 14 }"
              :maxlength="2000"
            />
          </t-form-item>
          <div class="form-grid">
            <t-form-item label="正文格式">
              <t-select v-model="content.format" :options="formatOptions" />
            </t-form-item>
            <t-form-item label="逐用户变量">
              <t-switch v-model="content.per_user_vars" />
              <p class="field-help">开启后 {username} / {email} 等按每个收件人渲染。</p>
            </t-form-item>
          </div>
          <t-form-item label="发送通道">
            <t-checkbox-group v-model="content.channels" :options="channelCheckboxOptions" />
          </t-form-item>
          <t-form-item v-if="smsSelected" label="短信模板">
            <t-select
              v-model="content.sms_template_code"
              placeholder="选择短信模板（scene=notify）"
              :options="smsTemplateOptions"
              :loading="smsLoading"
              filterable
              @change="handleSmsTemplateChange"
            />
            <p class="field-help">选择后自动带出该模板需要的变量（下方变量区）。</p>
          </t-form-item>
        </t-form>

        <!-- 变量区：注册表样例 + 短信模板所需变量 -->
        <div class="var-panel">
          <div class="var-panel__head">
            <h4 class="var-panel__title">变量取值</h4>
            <span class="var-panel__hint">点击变量名插入到正文光标处</span>
          </div>
          <div v-for="v in activeVars" :key="v.var_key" class="var-row">
            <t-tag
              theme="primary"
              variant="light"
              size="small"
              class="var-chip"
              :title="v.description"
              @click="insertVar(v.var_key)"
            >
              {{ varToken(v.var_key) }}
            </t-tag>
            <span class="var-row__label">{{ v.label }}</span>
            <t-input v-model="content.vars[v.var_key]" size="small" :placeholder="`示例：${v.sample}`" clearable />
          </div>
          <p v-if="!activeVars.length" class="field-help">正文未使用变量；插入后会在此填值。</p>
        </div>
      </div>

      <!-- Step 3 确认 -->
      <div v-else class="wizard-body">
        <t-space direction="vertical" size="large" :style="{ width: '100%' }">
          <div class="confirm-grid">
            <div class="confirm-block">
              <h4 class="confirm-block__title">目标</h4>
              <p>{{ targetSummary }}</p>
            </div>
            <div class="confirm-block">
              <h4 class="confirm-block__title">通道</h4>
              <div class="tag-list">
                <t-tag v-for="c in content.channels" :key="c" :theme="deliveryChannelTheme(c)" variant="light" size="small">
                  {{ deliveryChannelLabel(c) }}
                </t-tag>
              </div>
            </div>
          </div>

          <t-alert v-if="preview?.exceeded" theme="error" :message="`命中 ${preview.total} 人，超过单次上限 ${preview.max_targets} 人，请缩小范围`" />
          <t-alert v-else-if="preview && preview.total === 0" theme="warning" :message="preview.message || '未命中任何用户'" />

          <div v-if="preview && preview.total > 0" class="preview-block">
            <h4 class="confirm-block__title">发送预览（渲染后文案）</h4>
            <p class="preview-title">{{ preview.rendered.title }}</p>
            <pre class="preview-content">{{ preview.rendered.content }}</pre>
            <div class="preview-stats">
              <span>命中 {{ preview.total }} 人</span>
              <span v-if="mailSelected">邮件约 {{ preview.estimated_mail_count }} 封</span>
              <span v-if="smsSelected">短信约 {{ preview.total }} 条 / 预计 {{ fenToYuan(preview.estimated_sms_cost_fen) }} 元</span>
            </div>
            <h4 class="confirm-block__title">样例收件人（打码）</h4>
            <t-table
              row-key="id"
              :data="preview.sample"
              :columns="sampleColumns"
              size="small"
              :pagination="null"
              cell-empty-content="—"
            />
          </div>

          <t-form label-align="top" @submit.prevent>
            <t-form-item label="定时发送（预留）">
              <t-date-picker v-model="scheduleAt" enable-time-picker allow-input clearable placeholder="留空 = 立即发送" />
            </t-form-item>
          </t-form>
        </t-space>
      </div>

      <!-- 向导操作栏 -->
      <div class="wizard-actions">
        <t-space size="small">
          <t-button v-if="step > 0" variant="outline" @click="step -= 1">上一步</t-button>
          <t-button v-if="step < 2" theme="primary" :disabled="!canGoNext" @click="handleNext">下一步</t-button>
          <t-button
            v-else
            theme="primary"
            :disabled="!canSend"
            :loading="sending"
            @click="handleSend"
          >
            确认发送
          </t-button>
        </t-space>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { SendIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getBroadcastTargets,
  getSmsTemplates,
  getTemplateVars,
  previewBroadcast,
  sendBroadcast,
  type BroadcastPayload,
  type BroadcastPreviewResponse,
  type BroadcastUserItem,
  type TemplateVarItem,
} from '@/api/notification'
import {
  deliveryChannelLabel,
  deliveryChannelTheme,
  fenToYuan,
  formatOptions,
  varToken,
} from '@/pages/notification/constants'

defineOptions({ name: 'NotifyBroadcast' })

const step = ref(0)

// ===== 目标 =====
type TargetState = {
  mode: 'all' | 'group' | 'users' | 'filter'
  user_group_id?: number
  user_ids: number[]
  filter: {
    registered_after?: string
    registered_before?: string
    tier?: string
    min_balance?: string
    max_balance?: string
    status?: string
  }
}

const target = reactive<TargetState>({
  mode: 'all',
  user_ids: [],
  filter: {},
})

const userStatusOptions = [
  { label: '正常', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const hasInstanceOptions = [
  { label: '有实例', value: true },
  { label: '无实例', value: false },
]

const hasInstanceValue = ref<boolean | undefined>(undefined)

// 用户组 / 用户检索
const groups = ref<{ id: number; name: string; member_count: number }[]>([])
const groupsLoading = ref(false)
const groupOptions = computed(() => groups.value.map((g) => ({ label: `${g.name}（${g.member_count}）`, value: g.id })))
const selectedGroup = computed(() => groups.value.find((g) => g.id === target.user_group_id) || null)

const users = ref<BroadcastUserItem[]>([])
const usersLoading = ref(false)
const userOptions = computed(() =>
  users.value.map((u) => ({
    label: `${u.username}（${u.email_masked || u.phone_masked || '无联系方式'}）`,
    value: u.id,
  })),
)

async function loadGroups() {
  groupsLoading.value = true
  try {
    const resp = await getBroadcastTargets({ mode: 'groups' })
    groups.value = resp.groups || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载用户组失败')
  } finally {
    groupsLoading.value = false
  }
}

async function handleUserSearch(keyword: string) {
  if (!keyword?.trim()) return
  usersLoading.value = true
  try {
    const resp = await getBroadcastTargets({ mode: 'users', keyword: keyword.trim(), page: 1, page_size: 20 })
    users.value = resp.items || []
  } catch {
    users.value = []
  } finally {
    usersLoading.value = false
  }
}

function handleUserRemove() {
  // 仅用于触发已选人数提示的响应式刷新。
}

function handleModeChange() {
  if (target.mode === 'group' && !groups.value.length) loadGroups()
  void refreshCount()
}

// 命中人数（all / group / users 直接取 total；filter 需手动统计）
const filterCount = ref<number | null>(null)
const countLoading = ref(false)
const totalHit = ref<number | null>(null)

function buildTarget(): BroadcastPayload['target'] {
  const base = { mode: target.mode } as BroadcastPayload['target']
  if (target.mode === 'group') base.user_group_id = target.user_group_id
  if (target.mode === 'users') base.user_ids = [...target.user_ids]
  if (target.mode === 'filter') {
    base.filter = {
      ...target.filter,
      has_instance: hasInstanceValue.value,
    }
  }
  return base
}

async function refreshCount() {
  totalHit.value = null
  filterCount.value = null
  if (target.mode === 'filter') return
  try {
    const resp = await previewBroadcast({
      ...baseContent(),
      target: buildTarget(),
    })
    totalHit.value = resp.total
  } catch {
    totalHit.value = null
  }
}

async function handleCountTargets() {
  countLoading.value = true
  try {
    const resp = await previewBroadcast({ ...baseContent(), target: buildTarget() })
    filterCount.value = resp.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '统计失败')
  } finally {
    countLoading.value = false
  }
}

// ===== 内容 =====
const content = reactive({
  title: '',
  content: '',
  format: 'text',
  channels: ['inbox'] as string[],
  sms_template_code: '',
  vars: {} as Record<string, string>,
  per_user_vars: false,
})

const channelCheckboxOptions = [
  { label: '站内信', value: 'inbox' },
  { label: '邮件', value: 'mail' },
  { label: '短信', value: 'sms' },
]

const smsSelected = computed(() => content.channels.includes('sms'))
const mailSelected = computed(() => content.channels.includes('mail'))

const smsTemplates = ref<{ code: string; name: string; var_names: string[] }[]>([])
const smsLoading = ref(false)
const smsTemplateOptions = computed(() => smsTemplates.value.map((t) => ({ label: `${t.name}（${t.code}）`, value: t.code })))

async function loadSmsTemplates() {
  smsLoading.value = true
  try {
    const data = await getSmsTemplates({ scene: 'notify', status: 'active', page: 1, page_size: 100 })
    smsTemplates.value = data.items || []
  } catch {
    smsTemplates.value = []
  } finally {
    smsLoading.value = false
  }
}

// 变量注册表
const varList = ref<TemplateVarItem[]>([])

async function loadVars() {
  try {
    varList.value = await getTemplateVars()
  } catch {
    varList.value = []
  }
}

const varPattern = /\{([a-z_][a-z0-9_]*)\}/g

/** 正文实际使用的变量 + 短信模板声明的变量（并集，去重保序）。 */
const activeVarKeys = computed(() => {
  const keys: string[] = []
  const seen = new Set<string>()
  const push = (k: string) => {
    if (seen.has(k)) return
    seen.add(k)
    keys.push(k)
  }
  for (const match of content.content.matchAll(varPattern)) push(match[1])
  const tpl = smsTemplates.value.find((t) => t.code === content.sms_template_code)
  for (const k of tpl?.var_names || []) push(k)
  return keys
})

const activeVars = computed(() =>
  activeVarKeys.value.map((key) => {
    const found = varList.value.find((v) => v.var_key === key)
    return {
      var_key: key,
      label: found?.label || key,
      sample: found?.sample || '',
      description: found?.description || '未在注册表中登记',
    }
  }),
)

const bodyRef = ref<{ textarea?: HTMLTextAreaElement } | null>(null)

function resolveTextarea(): HTMLTextAreaElement | null {
  const raw = bodyRef.value as unknown
  if (!raw) return null
  const direct = (raw as { textarea?: HTMLTextAreaElement }).textarea
  if (direct) return direct
  const el = (raw as { $el?: HTMLElement }).$el
  if (el?.querySelector) return el.querySelector('textarea')
  return null
}

function insertVar(key: string) {
  const token = `{${key}}`
  const ta = resolveTextarea()
  if (!ta) {
    content.content += token
    return
  }
  const start = ta.selectionStart ?? content.content.length
  const end = ta.selectionEnd ?? content.content.length
  content.content = content.content.slice(0, start) + token + content.content.slice(end)
  requestAnimationFrame(() => {
    ta.focus()
    const pos = start + token.length
    ta.setSelectionRange(pos, pos)
  })
}

function handleSmsTemplateChange() {
  // 模板声明的变量在 activeVarKeys 里自动出现；预填注册表样例值方便确认长度。
  const tpl = smsTemplates.value.find((t) => t.code === content.sms_template_code)
  for (const key of tpl?.var_names || []) {
    if (content.vars[key]) continue
    const found = varList.value.find((v) => v.var_key === key)
    if (found?.sample) content.vars[key] = found.sample
  }
}

// ===== 摘要 / 步骤校验 =====
const targetSummary = computed(() => {
  switch (target.mode) {
    case 'all':
      return totalHit.value === null ? '全部用户' : `全部用户（约 ${totalHit.value} 人）`
    case 'group':
      return selectedGroup.value ? `用户组：${selectedGroup.value.name}` : '按用户组'
    case 'users':
      return `指定用户 ${target.user_ids.length} 人`
    case 'filter':
      return filterCount.value === null ? '条件筛选（未统计）' : `条件筛选（命中 ${filterCount.value} 人）`
    default:
      return ''
  }
})

const contentSummary = computed(() => {
  if (!content.title && !content.content) return '未编辑'
  const channelLabels = content.channels.map((c) => deliveryChannelLabel(c)).join('/')
  return `${channelLabels || '未选通道'}`
})

const canGoNext = computed(() => {
  if (step.value === 0) {
    if (target.mode === 'group') return !!target.user_group_id
    if (target.mode === 'users') return target.user_ids.length > 0
    if (target.mode === 'filter') return filterCount.value !== null && filterCount.value > 0
    return true
  }
  if (step.value === 1) {
    if (!content.title.trim() || !content.content.trim()) return false
    if (!content.channels.length) return false
    if (smsSelected.value && !content.sms_template_code) return false
    return true
  }
  return true
})

function baseContent() {
  return {
    title: content.title.trim(),
    content: content.content,
    format: content.format,
    channels: [...content.channels],
    sms_template_code: content.sms_template_code || undefined,
    vars: { ...content.vars },
    per_user_vars: content.per_user_vars,
  }
}

// ===== 预览 / 发送 =====
const preview = ref<BroadcastPreviewResponse | null>(null)
const previewLoading = ref(false)
const sending = ref(false)
const scheduleAt = ref('')

const sampleColumns: PrimaryTableCol<BroadcastUserItem>[] = [
  { colKey: 'id', title: 'ID', width: 80 },
  { colKey: 'username', title: '用户名', minWidth: 120 },
  { colKey: 'email_masked', title: '邮箱', minWidth: 160 },
  { colKey: 'phone_masked', title: '手机号', width: 130 },
]

async function loadPreview() {
  previewLoading.value = true
  try {
    preview.value = await previewBroadcast({ ...baseContent(), target: buildTarget() })
    content.vars = { ...(preview.value.rendered ? content.vars : content.vars) }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '预览失败')
  } finally {
    previewLoading.value = false
  }
}

async function handleNext() {
  if (step.value === 1) {
    await loadPreview()
  } else {
    // 目标步进入内容步时顺带刷新命中人数（用于摘要展示）。
    if (target.mode !== 'filter') await refreshCount()
  }
  step.value += 1
}

const canSend = computed(() => !!preview.value && preview.value.total > 0 && !preview.value.exceeded)

async function handleSend() {
  sending.value = true
  try {
    const payload: BroadcastPayload = {
      ...baseContent(),
      target: buildTarget(),
      schedule_at: scheduleAt.value || undefined,
    }
    const resp = await sendBroadcast(payload)
    MessagePlugin.success(resp.message || `已提交 ${resp.queued} 条投递`)
    step.value = 0
    preview.value = null
    target.user_ids = []
    filterCount.value = null
    refreshCount()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '发送失败')
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadVars(), loadGroups(), loadSmsTemplates()])
})
</script>

<style lang="css" scoped>
.wizard-card {
  padding: var(--space-lg) var(--space-xl);
  display: flex;
  flex-direction: column;
}

.wizard-body {
  min-height: 220px;
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.mode-panel {
  padding: var(--space-md);
  border: 1px solid var(--hs-border-color, #e5e7eb);
  border-radius: var(--hs-radius-lg, 8px);
}

.wizard-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--space-lg);
  padding-top: var(--space-md);
  border-top: 1px solid var(--hs-border-color, #e5e7eb);
}

.var-panel {
  border: 1px solid var(--hs-border-color, #e5e7eb);
  border-radius: var(--hs-radius-lg, 8px);
  padding: var(--space-md);
}

.var-panel__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: var(--space-sm);
}

.var-panel__title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}

.var-panel__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.var-row {
  display: grid;
  grid-template-columns: auto 120px minmax(0, 1fr);
  align-items: center;
  gap: var(--space-sm);
  padding: 6px 0;
}

.var-row__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.var-chip {
  cursor: pointer;
  user-select: none;
}

.confirm-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md);
}

.confirm-block {
  padding: var(--space-md);
  border: 1px solid var(--hs-border-color, #e5e7eb);
  border-radius: var(--hs-radius-lg, 8px);
}

.confirm-block__title {
  margin: 0 0 6px;
  font-size: 13px;
  font-weight: 600;
}

.preview-block {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.preview-title {
  margin: 0;
  font-weight: 600;
}

.preview-content {
  margin: 0;
  padding: 10px 12px;
  background: var(--hs-surface-2, #f4f6fa);
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 240px;
  overflow-y: auto;
}

.preview-stats {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-md);
  font-size: 13px;
  color: #334155;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

@media (max-width: 768px) {
  .confirm-grid,
  .var-row {
    grid-template-columns: 1fr;
  }
}
</style>

<style lang="css">
@import '../shared.css';
</style>
