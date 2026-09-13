<template>
  <div class="page-body notify-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ChatIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">短信模板</h2>
          <p class="page-header__desc">
            正文变量只能取「变量注册表」里已登记的键；保存时逐个校验，未注册的变量直接拒绝，避免上线后发出带 <code>&#123;placeholder&#125;</code> 的短信。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button v-if="canManage" theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新建模板
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">场景</span>
          <t-select v-model="filters.scene" clearable placeholder="全部场景" :options="smsSceneOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="模板编码 / 名称" clearable @enter="handleSearch" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">模板列表</h3>
        <span class="table-card__meta">共 {{ total }} 个模板</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #code="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.code }}</span>
            <span class="price-sub">{{ row.name }}</span>
          </div>
        </template>
        <template #scene="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ smsSceneLabel(row.scene) }}</t-tag>
        </template>
        <template #content="{ row }">
          <span class="cell-muted content-ellipsis" :title="row.content">{{ row.content }}</span>
        </template>
        <template #var_names="{ row }">
          <div class="tag-list">
            <t-tag v-for="v in row.var_names" :key="v" theme="default" variant="light" size="small" shape="round">
              {{ varToken(v) }}
            </t-tag>
            <span v-if="!row.var_names?.length" class="price-sub">无变量</span>
          </div>
        </template>
        <template #segments="{ row }">
          <span class="cell-muted">{{ row.char_count }} 字 / {{ row.segments }} 条</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 'active' ? '启用' : '停用' }}
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
                { content: '编辑', value: 'edit', hidden: () => !canManage },
                { content: '删除', value: 'delete', theme: 'error', hidden: () => !canManage },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link v-if="canManage" theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link v-if="canManage" theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无短信模板，点击右上角「新建模板」创建" />
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

    <!-- 编辑抽屉 -->
    <t-drawer
      v-model:visible="formVisible"
      :header="editingId ? '编辑短信模板' : '新建短信模板'"
      size="640px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <div class="tpl-form">
        <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
          <div class="form-grid">
            <t-form-item label="模板编码" name="code">
              <t-input v-model="form.code" :disabled="!!editingId" placeholder="唯一编码，如 notify_order_paid" />
            </t-form-item>
            <t-form-item label="模板名称" name="name">
              <t-input v-model="form.name" placeholder="用于后台展示" />
            </t-form-item>
            <t-form-item label="适用场景" name="scene">
              <t-select v-model="form.scene" :options="smsSceneOptions" />
            </t-form-item>
            <t-form-item label="服务商模板号" name="upstream_code">
              <t-input v-model="form.upstream_code" placeholder="选填，服务商侧模板 ID" />
            </t-form-item>
          </div>

          <t-form-item label="模板正文" name="content">
            <t-textarea
              ref="contentRef"
              v-model="form.content"
              placeholder="如：【HostSent】您的 {code}，{minutes} 分钟内有效。"
              :autosize="{ minRows: 4, maxRows: 8 }"
              :maxlength="1000"
            />
            <div class="content-meta">
              <span>{{ charCount }} 字</span>
              <span>预计 {{ segments }} 条短信</span>
            </div>
            <p v-if="invalidVars.length" class="invalid-hint">
              未注册变量：{{ invalidVarTokens }} —— 请先在下方变量注册表登记，或改用已有变量。
            </p>
          </t-form-item>
        </t-form>

        <!-- 变量面板：点击插入到光标处 -->
        <div class="var-panel">
          <div class="var-panel__head">
            <h4 class="var-panel__title">变量注册表</h4>
            <span class="var-panel__hint">点击插入到光标位置</span>
          </div>
          <div v-for="group in varGroups" :key="group.category" class="var-group">
            <span class="var-group__label">{{ group.label }}</span>
            <div class="tag-list">
              <t-tag
                v-for="v in group.items"
                :key="v.var_key"
                theme="primary"
                variant="light"
                size="small"
                class="var-chip"
                :title="`${v.label}（示例：${v.sample}）`"
                @click="insertVar(v.var_key)"
              >
                {{ varToken(v.var_key) }}
              </t-tag>
            </div>
          </div>
          <p v-if="!varList.length" class="field-help">变量注册表为空，请先在种子数据或后台登记变量。</p>
        </div>

        <!-- 预览：用注册表示例值渲染 -->
        <div class="preview-panel">
          <div class="preview-panel__head">
            <h4 class="preview-panel__title">预览（示例值）</h4>
            <span class="preview-panel__hint">{{ renderedChars }} 字 / {{ renderedSegments }} 条</span>
          </div>
          <div class="sms-bubble">{{ rendered || '（正文为空）' }}</div>
        </div>

        <t-form label-align="top" :data="form" @submit.prevent>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" :options="statusOptions" />
          </t-form-item>
          <t-form-item label="备注" name="remark">
            <t-textarea v-model="form.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
          </t-form-item>
        </t-form>
      </div>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, ChatIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type FormInstanceFunctions, type FormRule, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createSmsTemplate,
  deleteSmsTemplate,
  getSmsTemplates,
  getTemplateVars,
  updateSmsTemplate,
  type SmsTemplateItem,
  type TemplateVarItem,
} from '@/api/notification'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { formatTime, smsSceneLabel, smsSceneOptions, varToken } from '@/pages/notification/constants'
import { useUserStore } from '@/store'

defineOptions({ name: 'NotifySmsTemplates' })

const userStore = useUserStore()
const canManage = computed(() => userStore.permissions?.includes('notify:sms-template:manage') || userStore.isSuperAdmin)
const { isMobile } = useIsMobile()

const loading = ref(false)
const list = ref<SmsTemplateItem[]>([])
const total = ref(0)
const filters = reactive({ scene: undefined as string | undefined, status: undefined as string | undefined, keyword: '' })

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'disabled' },
]

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<SmsTemplateItem>[] = [
  { colKey: 'code', title: '模板', minWidth: 190 },
  { colKey: 'scene', title: '场景', width: 90 },
  { colKey: 'content', title: '正文', minWidth: 240 },
  { colKey: 'var_names', title: '变量', minWidth: 150 },
  { colKey: 'segments', title: '长度', width: 130 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'updated_at', title: '更新时间', width: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 120, fixed: 'right' as const, align: 'center' as const },
]

async function loadData() {
  loading.value = true
  try {
    const data = await getSmsTemplates({
      scene: filters.scene,
      status: filters.status,
      keyword: filters.keyword || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items || []
    total.value = data.meta?.total ?? 0
    pagination.total = total.value
    mobilePage.total = total.value
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载短信模板失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  loadData()
}

function handleReset() {
  filters.scene = undefined
  filters.status = undefined
  filters.keyword = ''
  pagination.current = 1
  loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadData()
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

// ===== 变量注册表 =====
const varList = ref<TemplateVarItem[]>([])

const varGroups = computed(() => {
  const groups = new Map<string, TemplateVarItem[]>()
  for (const v of varList.value) {
    const key = v.category || '通用'
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key)!.push(v)
  }
  return Array.from(groups.entries()).map(([category, items]) => ({
    category,
    label: categoryLabelOf(category),
    items: items.sort((a, b) => a.sort_order - b.sort_order),
  }))
})

const categoryLabelMap: Record<string, string> = {
  otp: '验证码',
  order: '订单',
  finance: '财务',
  instance: '实例',
  common: '通用',
  account: '账号',
}

function categoryLabelOf(category: string): string {
  return categoryLabelMap[category] || category
}

async function loadVars() {
  try {
    varList.value = await getTemplateVars()
  } catch {
    varList.value = []
  }
}

// ===== 表单 =====
type TemplateForm = {
  code: string
  name: string
  scene: string
  content: string
  upstream_code: string
  status: string
  remark: string
}

const formVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstanceFunctions | null>(null)
const contentRef = ref<{ textarea?: HTMLTextAreaElement } | null>(null)

const form = reactive<TemplateForm>({
  code: '',
  name: '',
  scene: 'notify',
  content: '',
  upstream_code: '',
  status: 'active',
  remark: '',
})

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入模板名称', type: 'error', trigger: 'blur' }],
  content: [{ required: true, message: '请输入模板正文', type: 'error', trigger: 'blur' }],
}

// 与后端 varPattern 一致：小写字母/下划线开头，后接字母数字下划线。
const varPattern = /\{([a-z_][a-z0-9_]*)\}/g

const registeredKeys = computed(() => new Set(varList.value.map((v) => v.var_key)))

const invalidVars = computed(() => {
  const found = new Set<string>()
  for (const match of form.content.matchAll(varPattern)) {
    if (!registeredKeys.value.has(match[1])) found.add(match[1])
  }
  return Array.from(found)
})

// 提示文案在 script 里拼好：模板花括号与 mustache 插值会互相干扰。
const invalidVarTokens = computed(() => invalidVars.value.map((v) => varToken(v)).join('、'))

// 字符数与条数口径与后端 SmsSegments 保持一致（按 rune，70/67 分段）。
function segmentsOf(content: string): number {
  const chars = [...content].length
  if (chars === 0) return 0
  if (chars <= 70) return 1
  return Math.ceil(chars / 67)
}

const charCount = computed(() => [...form.content].length)
const segments = computed(() => segmentsOf(form.content))

const rendered = computed(() => {
  let out = form.content
  for (const v of varList.value) {
    out = out.split(`{${v.var_key}}`).join(v.sample || `{${v.var_key}}`)
  }
  return out
})
const renderedChars = computed(() => [...rendered.value].length)
const renderedSegments = computed(() => segmentsOf(rendered.value))

/** 取到 textarea DOM：TDesign 的 ref 层级不稳定，两种形状都试。 */
function resolveTextarea(): HTMLTextAreaElement | null {
  const raw = contentRef.value as unknown
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
    form.content += token
    return
  }
  const start = ta.selectionStart ?? form.content.length
  const end = ta.selectionEnd ?? form.content.length
  form.content = form.content.slice(0, start) + token + form.content.slice(end)
  // 光标落到插入内容之后，方便连续插入多个变量。
  requestAnimationFrame(() => {
    ta.focus()
    const pos = start + token.length
    ta.setSelectionRange(pos, pos)
  })
}

function resetForm() {
  Object.assign(form, {
    code: '',
    name: '',
    scene: 'notify',
    content: '',
    upstream_code: '',
    status: 'active',
    remark: '',
  })
  formRef.value?.clearValidate?.()
}

function openCreate() {
  editingId.value = null
  resetForm()
  formVisible.value = true
}

function openEdit(row: SmsTemplateItem) {
  editingId.value = row.id
  Object.assign(form, {
    code: row.code,
    name: row.name,
    scene: row.scene || 'notify',
    content: row.content,
    upstream_code: row.upstream_code || '',
    status: row.status || 'active',
    remark: row.remark || '',
  })
  formRef.value?.clearValidate?.()
  formVisible.value = true
}

async function handleSave() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  // 前端先拦一次未注册变量，给即时反馈；后端仍会独立校验（不信任前端）。
  if (invalidVars.value.length) {
    MessagePlugin.warning(`存在未注册变量：${invalidVars.value.map((v) => `{${v}}`).join('、')}`)
    return
  }
  saving.value = true
  try {
    const payload = {
      name: form.name.trim(),
      scene: form.scene,
      content: form.content,
      upstream_code: form.upstream_code.trim(),
      status: form.status,
      remark: form.remark,
    }
    if (editingId.value) {
      await updateSmsTemplate(editingId.value, payload)
      MessagePlugin.success('模板已更新')
    } else {
      await createSmsTemplate({ ...payload, code: form.code.trim() })
      MessagePlugin.success('模板已创建')
    }
    formVisible.value = false
    loadData()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存模板失败')
  } finally {
    saving.value = false
  }
}

function handleDelete(row: SmsTemplateItem) {
  const dialog = DialogPlugin.confirm({
    header: '删除短信模板',
    body: `确认删除模板「${row.name}」吗？被通知模板引用的模板无法删除。`,
    confirmBtn: { content: '删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteSmsTemplate(row.id)
        MessagePlugin.success('模板已删除')
        dialog.destroy()
        loadData()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

function handleMobileAction(value: string | number | Record<string, any>, row: SmsTemplateItem) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'delete':
      handleDelete(row)
      break
  }
}

onMounted(async () => {
  await Promise.all([loadVars(), loadData()])
})
</script>

<style lang="css" scoped>
.content-ellipsis {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tpl-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.content-meta {
  display: flex;
  gap: var(--space-md);
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.invalid-hint {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--td-error-color, #d54941);
}

.var-panel,
.preview-panel {
  border: 1px solid var(--hs-border-color, #e5e7eb);
  border-radius: var(--hs-radius-lg, 8px);
  padding: var(--space-md);
}

.var-panel__head,
.preview-panel__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: var(--space-sm);
}

.var-panel__title,
.preview-panel__title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.var-panel__hint,
.preview-panel__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.var-group {
  display: flex;
  align-items: flex-start;
  gap: var(--space-sm);
  padding: 6px 0;
}

.var-group__label {
  flex: 0 0 64px;
  font-size: 12px;
  color: var(--color-muted-foreground);
  padding-top: 3px;
}

.var-chip {
  cursor: pointer;
  user-select: none;
}

.sms-bubble {
  padding: 12px 14px;
  border-radius: 14px 14px 14px 4px;
  background: var(--hs-surface-2, #f4f6fa);
  font-size: 13px;
  line-height: 1.7;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>

<style lang="css">
@import '../shared.css';
</style>
