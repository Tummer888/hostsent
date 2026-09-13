<template>
  <div class="page-body notify-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MailIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">渠道配置</h2>
          <p class="page-header__desc">
            邮件与短信按「渠道实例」管理：同一类型可建多个实例（签名/发件人/额度各异），发送时按场景与优先级路由。凭证加密落库，回显一律脱敏。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button v-if="canManage" theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新建渠道
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadChannels">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <t-tabs v-model="activeCategory" :style="{ width: '320px' }" @change="handleCategoryChange">
          <t-tab-panel value="mail" label="邮件渠道" />
          <t-tab-panel value="sms" label="短信渠道" />
        </t-tabs>
        <span class="table-card__meta">共 {{ total }} 个渠道</span>
      </div>

      <div class="filter-card__grid filter-card__grid--inline">
        <div class="field">
          <span class="field__label">渠道类型</span>
          <t-select v-model="filters.type" clearable placeholder="全部类型" :options="typeFilterOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusFilterOptions" />
        </div>
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="渠道编码 / 名称" clearable @enter="handleSearch" />
        </div>
        <div class="field field--actions">
          <t-space size="small">
            <t-button theme="primary" @click="handleSearch">
              <template #icon><SearchIcon aria-hidden="true" /></template>
              查询
            </t-button>
            <t-button variant="outline" @click="handleResetFilters">重置</t-button>
          </t-space>
        </div>
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
        <template #channel_code="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.channel_code }}</span>
            <span class="price-sub">{{ row.name }}</span>
          </div>
        </template>

        <template #type="{ row }">
          <div class="price-cell">
            <t-tag :theme="categoryTheme(row.category)" variant="light" size="small" shape="round">
              {{ row.type_name || row.type }}
            </t-tag>
            <span v-if="!row.implemented" class="price-sub">配置占位</span>
            <span v-else class="price-sub">{{ channelModeLabel(row.capabilities?.mode || 'api') }}</span>
          </div>
        </template>

        <template #scenes="{ row }">
          <div class="tag-list">
            <t-tag v-for="s in row.scenes" :key="s" theme="default" variant="light" size="small" shape="round">
              {{ channelSceneLabel(s) }}
            </t-tag>
            <span v-if="!row.scenes?.length" class="price-sub">全部场景</span>
          </div>
        </template>

        <template #is_default="{ row }">
          <t-tag v-if="row.is_default" theme="warning" variant="light" size="small" shape="round">默认</t-tag>
          <span v-else class="price-sub">—</span>
        </template>

        <template #health_status="{ row }">
          <div class="price-cell">
            <t-tag :theme="channelHealthTheme(row.health_status)" variant="light" size="small" shape="round">
              {{ channelHealthLabel(row.health_status) }}
            </t-tag>
            <span v-if="row.last_error" class="price-sub error-text" :title="row.last_error">{{ row.last_error }}</span>
          </div>
        </template>

        <template #last_check_at="{ row }">
          <span class="time-text">{{ formatTime(row.last_check_at) }}</span>
        </template>

        <template #status="{ row }">
          <t-switch
            :value="row.status === 1"
            :disabled="!canManage"
            :loading="statusLoadingId === row.id"
            @change="(v: unknown) => handleToggleStatus(row, Boolean(v))"
          />
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '测试发送', value: 'test' },
                { content: '编辑', value: 'edit', hidden: () => !canManage },
                { content: '详情', value: 'detail' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openTest(row)">测试发送</t-link>
              <t-link v-if="canManage" theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty :description="`暂无${categoryLabel(activeCategory)}，点击右上角「新建渠道」接入`" />
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

    <!-- 新建 / 编辑渠道 -->
    <t-dialog
      v-model:visible="formVisible"
      :header="editingId ? '编辑渠道' : '新建渠道'"
      width="760px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: submitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSubmit"
      @close="formVisible = false"
    >
      <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="渠道编码" name="channel_code">
            <t-input v-model="form.channel_code" :disabled="!!editingId" placeholder="唯一编码，如 mail_main / sms_main" />
          </t-form-item>
          <t-form-item label="渠道名称" name="name">
            <t-input v-model="form.name" placeholder="用于后台展示，如 主站 SMTP" />
          </t-form-item>
          <t-form-item label="渠道类型" name="type">
            <t-select
              v-model="form.type"
              :disabled="!!editingId"
              placeholder="请选择渠道类型"
              :options="typeSelectOptions"
              @change="handleTypeChange"
            />
          </t-form-item>
          <t-form-item label="优先级" name="priority">
            <t-input-number v-model="form.priority" :min="0" theme="column" placeholder="数值越大越优先" />
          </t-form-item>
        </div>

        <t-alert
          v-if="selectedType && !selectedType.implemented"
          theme="info"
          class="form-hint"
          message="该服务商适配器待接入"
          description="配置可正常保存并加密落库；「测试发送」会提示待接入，不会误报失败。"
        />

        <CredentialFields
          v-model="form.credentials"
          title="凭证配置"
          :fields="selectedType?.capabilities?.credential_schema"
          :empty-hint="selectedType ? '该渠道类型未声明凭证字段。' : '请先选择渠道类型。'"
        />

        <div class="form-grid">
          <t-form-item v-if="activeCategory === 'mail'" label="发件人" name="sender">
            <t-input v-model="form.sender" placeholder="如 HostSent <no-reply@example.com>" />
          </t-form-item>
          <t-form-item v-else label="短信签名" name="sign_name">
            <t-input v-model="form.sign_name" placeholder="如 【HostSent】" />
          </t-form-item>
          <t-form-item label="服务商模板号" name="template_code">
            <t-input v-model="form.template_code" placeholder="选填，短信服务商侧模板 ID" />
          </t-form-item>
          <t-form-item label="接口地址（Endpoint）" name="endpoint">
            <t-input v-model="form.endpoint" placeholder="选填，渠道 API 网关地址" />
          </t-form-item>
          <t-form-item label="日均上限（条）" name="daily_limit">
            <t-input-number v-model="form.daily_limit" :min="0" theme="column" placeholder="0=不限" />
          </t-form-item>
          <t-form-item label="同优先级权重" name="weight">
            <t-input-number v-model="form.weight" :min="0" theme="column" placeholder="选填" />
          </t-form-item>
          <t-form-item label="启用场景" name="scenes">
            <t-select v-model="form.scenes" multiple clearable :options="channelSceneOptions" placeholder="不选=全部场景" />
          </t-form-item>
        </div>

        <div class="form-grid form-grid--single">
          <t-form-item label="默认渠道" name="is_default">
            <t-switch v-model="form.is_default" />
            <p class="field-help">开启后作为该类别无场景匹配时的兜底渠道（同类别仅保留一个默认）。</p>
          </t-form-item>
          <t-form-item label="备注" name="remark">
            <t-textarea v-model="form.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>

    <!-- 测试发送 -->
    <t-dialog
      v-model:visible="testVisible"
      header="测试发送"
      width="520px"
      :confirm-btn="{ content: '发送', theme: 'primary', loading: testing }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleTestSend"
      @close="testVisible = false"
    >
      <div v-if="testRow" class="test-body">
        <p class="field-help">渠道：<b>{{ testRow.name }}</b>（{{ testRow.type_name || testRow.type }}）</p>
        <t-form label-align="top" @submit.prevent>
          <t-form-item :label="testRow.category === 'mail' ? '接收邮箱' : '接收手机号'">
            <t-input v-model="testTarget" :placeholder="testRow.category === 'mail' ? 'test@example.com' : '13800138000'" />
          </t-form-item>
          <t-form-item label="发送内容">
            <t-textarea v-model="testContent" :autosize="{ minRows: 3, maxRows: 5 }" placeholder="留空则发送默认测试文案" />
          </t-form-item>
        </t-form>
      </div>
    </t-dialog>

    <!-- 详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="渠道详情" size="560px" :footer="false">
      <template v-if="detail">
        <div class="detail-block">
          <div class="recon-row">
            <span class="recon-row__label">渠道编码</span>
            <span class="recon-row__value">{{ detail.channel_code }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">渠道名称</span>
            <span class="recon-row__value">{{ detail.name }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">类型</span>
            <span class="recon-row__value">{{ detail.type_name || detail.type }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">适配器</span>
            <span class="recon-row__value">{{ detail.implemented ? '已接入' : '配置占位（待接入）' }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">最近检测</span>
            <span class="recon-row__value">{{ formatTime(detail.last_check_at) }}</span>
          </div>
        </div>
        <div class="detail-block">
          <h4 class="detail-block__title">凭证（脱敏回显）</h4>
          <div v-for="(value, key) in detail.credentials" :key="key" class="recon-row">
            <span class="recon-row__label">{{ credentialLabel(key) }}</span>
            <span class="recon-row__value">{{ value || '—' }}</span>
          </div>
          <p v-if="!Object.keys(detail.credentials || {}).length" class="field-help">该渠道未配置凭证。</p>
        </div>
      </template>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, MailIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createNotifyChannel,
  getNotifyChannelTypes,
  getNotifyChannels,
  notifyTestSend,
  updateNotifyChannel,
  updateNotifyChannelStatus,
  type NotifyChannelInfo,
  type NotifyChannelTypeItem,
} from '@/api/notification'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import {
  categoryLabel,
  categoryTheme,
  channelHealthLabel,
  channelHealthTheme,
  channelModeLabel,
  channelSceneLabel,
  channelSceneOptions,
  formatTime,
} from '@/pages/notification/constants'
import CredentialFields from '@/components/credential-fields/index.vue'
import { useUserStore } from '@/store'

defineOptions({ name: 'NotifyChannels' })

const userStore = useUserStore()
const canManage = computed(() => userStore.permissions?.includes('notify:channel:manage') || userStore.isSuperAdmin)

const { isMobile } = useIsMobile()

const activeCategory = ref<'mail' | 'sms'>('mail')
const list = ref<NotifyChannelInfo[]>([])
const typeList = ref<NotifyChannelTypeItem[]>([])
const loading = ref(false)
const total = ref(0)

const filters = reactive<{ type?: string; status?: number; keyword?: string }>({
  type: undefined,
  status: undefined,
  keyword: undefined,
})

const statusFilterOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<NotifyChannelInfo>[] = [
  { colKey: 'channel_code', title: '渠道', minWidth: 180 },
  { colKey: 'type', title: '类型', width: 150 },
  { colKey: 'scenes', title: '启用场景', minWidth: 170 },
  { colKey: 'is_default', title: '默认', width: 80, align: 'center' as const },
  { colKey: 'health_status', title: '健康状态', minWidth: 130 },
  { colKey: 'last_check_at', title: '最后检查', width: 160 },
  { colKey: 'status', title: '启用', width: 80, align: 'center' as const },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 200, fixed: 'right' as const, align: 'center' as const },
]

// 类型下拉按当前 Tab 的 category 过滤：邮件 Tab 不出现短信服务商。
const categoryTypes = computed(() => typeList.value.filter((t) => t.category === activeCategory.value))
const typeFilterOptions = computed(() => categoryTypes.value.map((t) => ({ label: t.name, value: t.type })))
const typeSelectOptions = computed(() =>
  categoryTypes.value.map((t) => ({
    label: t.implemented ? `${t.name}（${t.type}）` : `${t.name} · 配置占位`,
    value: t.type,
  })),
)

async function loadTypes() {
  try {
    typeList.value = await getNotifyChannelTypes()
  } catch {
    typeList.value = []
  }
}

async function loadChannels() {
  loading.value = true
  try {
    const data = await getNotifyChannels({
      category: activeCategory.value,
      type: filters.type,
      status: filters.status,
      keyword: filters.keyword,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items || []
    total.value = data.meta?.total ?? 0
    pagination.total = total.value
    mobilePage.total = total.value
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载渠道失败')
  } finally {
    loading.value = false
  }
}

function handleCategoryChange() {
  filters.type = undefined
  pagination.current = 1
  mobilePage.current = 1
  loadChannels()
}

function handleSearch() {
  pagination.current = 1
  loadChannels()
}

function handleResetFilters() {
  filters.type = undefined
  filters.status = undefined
  filters.keyword = undefined
  pagination.current = 1
  loadChannels()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadChannels()
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

// ===== 启停 =====
const statusLoadingId = ref<number | null>(null)

async function handleToggleStatus(row: NotifyChannelInfo, enabled: boolean) {
  statusLoadingId.value = row.id
  try {
    await updateNotifyChannelStatus(row.id, enabled ? 1 : 0)
    MessagePlugin.success(enabled ? '渠道已启用' : '渠道已停用')
    loadChannels()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新渠道状态失败')
  } finally {
    statusLoadingId.value = null
  }
}

// ===== 表单 =====
type ChannelForm = {
  channel_code: string
  name: string
  type: string
  credentials: Record<string, string>
  endpoint: string
  sign_name: string
  sender: string
  template_code: string
  scenes: string[]
  priority: number
  weight: number
  daily_limit: number
  is_default: boolean
  remark: string
}

const formVisible = ref(false)
const submitting = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstanceFunctions | null>(null)

const form = reactive<ChannelForm>({
  channel_code: '',
  name: '',
  type: '',
  credentials: {},
  endpoint: '',
  sign_name: '',
  sender: '',
  template_code: '',
  scenes: [],
  priority: 0,
  weight: 0,
  daily_limit: 0,
  is_default: false,
  remark: '',
})

const rules: Record<string, FormRule[]> = {
  channel_code: [{ required: true, message: '请输入渠道编码', type: 'error', trigger: 'blur' }],
  name: [{ required: true, message: '请输入渠道名称', type: 'error', trigger: 'blur' }],
  type: [{ required: true, message: '请选择渠道类型', type: 'error', trigger: 'change' }],
}

const selectedType = computed(() => typeList.value.find((t) => t.type === form.type) || null)

function handleTypeChange() {
  // 切换类型后凭证结构变化，清空已填值避免脏数据。
  form.credentials = {}
  form.scenes = []
}

function resetForm() {
  Object.assign(form, {
    channel_code: '',
    name: '',
    type: '',
    credentials: {},
    endpoint: '',
    sign_name: '',
    sender: '',
    template_code: '',
    scenes: [],
    priority: 0,
    weight: 0,
    daily_limit: 0,
    is_default: false,
    remark: '',
  })
  formRef.value?.clearValidate?.()
}

function openCreate() {
  editingId.value = null
  resetForm()
  formVisible.value = true
}

function openEdit(row: NotifyChannelInfo) {
  editingId.value = row.id
  activeCategory.value = row.category
  Object.assign(form, {
    channel_code: row.channel_code,
    name: row.name,
    type: row.type,
    credentials: { ...(row.credentials || {}) },
    endpoint: row.endpoint || '',
    sign_name: row.sign_name || '',
    sender: row.sender || '',
    template_code: row.template_code || '',
    scenes: [...(row.scenes || [])],
    priority: row.priority || 0,
    weight: row.weight || 0,
    daily_limit: row.daily_limit || 0,
    is_default: row.is_default,
    remark: row.remark || '',
  })
  formRef.value?.clearValidate?.()
  formVisible.value = true
}

async function handleSubmit() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  submitting.value = true
  try {
    // 凭证里的密文字段保持掩码原样回传，后端 IsMaskedEcho 识别为「未修改」。
    const payload = {
      name: form.name,
      credentials: form.credentials,
      endpoint: form.endpoint,
      sign_name: form.sign_name,
      sender: form.sender,
      template_code: form.template_code,
      scenes: form.scenes,
      priority: form.priority,
      weight: form.weight,
      daily_limit: form.daily_limit,
      is_default: form.is_default,
      remark: form.remark,
    }
    if (editingId.value) {
      await updateNotifyChannel(editingId.value, payload)
      MessagePlugin.success('渠道已更新')
    } else {
      await createNotifyChannel({ ...payload, channel_code: form.channel_code, type: form.type })
      MessagePlugin.success('渠道已创建')
    }
    formVisible.value = false
    loadChannels()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存渠道失败')
  } finally {
    submitting.value = false
  }
}

// ===== 测试发送 =====
const testVisible = ref(false)
const testRow = ref<NotifyChannelInfo | null>(null)
const testTarget = ref('')
const testContent = ref('')
const testing = ref(false)

function openTest(row: NotifyChannelInfo) {
  testRow.value = row
  testTarget.value = ''
  testContent.value = ''
  testVisible.value = true
}

async function handleTestSend() {
  if (!testRow.value) return
  if (!testTarget.value.trim()) {
    MessagePlugin.warning(testRow.value.category === 'mail' ? '请输入接收邮箱' : '请输入接收手机号')
    return
  }
  testing.value = true
  try {
    const resp = await notifyTestSend({
      category: testRow.value.category,
      channel_id: testRow.value.id,
      target: testTarget.value.trim(),
      content: testContent.value.trim() || undefined,
    })
    if (resp.pending) {
      MessagePlugin.info(resp.message || '该服务商适配器待接入，配置已保存')
    } else if (resp.ok) {
      MessagePlugin.success(resp.message || '测试发送成功')
    } else {
      MessagePlugin.error(resp.message || '测试发送失败')
    }
    testVisible.value = false
    loadChannels()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '测试发送失败')
  } finally {
    testing.value = false
  }
}

// ===== 详情 =====
const detailVisible = ref(false)
const detail = ref<NotifyChannelInfo | null>(null)

function openDetail(row: NotifyChannelInfo) {
  detail.value = row
  detailVisible.value = true
}

function credentialLabel(key: string): string {
  const fields = detail.value?.capabilities?.credential_schema || []
  const found = fields.find((f) => f.key === key)
  return found ? found.label : key
}

function handleMobileAction(value: string | number | Record<string, any>, row: NotifyChannelInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'test':
      openTest(row)
      break
    case 'edit':
      openEdit(row)
      break
    case 'detail':
      openDetail(row)
      break
  }
}

onMounted(async () => {
  await loadTypes()
  await loadChannels()
})
</script>

<style lang="css" scoped>
.filter-card__grid--inline {
  margin-bottom: var(--space-md);
}

.field--actions {
  justify-content: flex-end;
}

.form-hint {
  margin-bottom: var(--space-md);
}

.detail-block {
  margin-bottom: var(--space-lg);
}

.detail-block__title {
  margin: 0 0 var(--space-sm);
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.error-text {
  color: #dc2626;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.test-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

@media (max-width: 768px) {
  .filter-card__grid--inline {
    grid-template-columns: 1fr;
  }
}
</style>

<style lang="css">
@import '../shared.css';
</style>
