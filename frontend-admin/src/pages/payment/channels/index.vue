<template>
  <div class="page-body payment-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <LinkIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">支付渠道</h2>
          <p class="page-header__desc">每种渠道类型可建多个实例（商户号/费率/结算账户各异），收款与出款按场景和优先级路由。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button theme="primary" @click="openCreateDialog">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新建渠道
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadChannels">
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
          <span class="field__label">渠道类型</span>
          <t-select v-model="filters.type" clearable placeholder="全部类型" :options="typeFilterOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="channelStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="渠道编码 / 名称" clearable @enter="handleSearch" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">渠道实例</h3>
        <span class="table-card__meta">共 {{ total }} 个渠道</span>
      </div>
      <t-table
        row-key="id"
        :data="channelList"
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
            <t-tag theme="primary" variant="light" size="small" shape="round">{{ row.type_name || row.type }}</t-tag>
            <span class="price-sub">{{ modeLabel(row.mode) }}</span>
          </div>
        </template>

        <template #scenes="{ row }">
          <div class="tag-list">
            <t-tag v-for="s in row.scenes" :key="s" theme="default" variant="light" size="small" shape="round">
              {{ sceneLabel(s) }}
            </t-tag>
            <span v-if="!row.scenes?.length" class="price-sub">全部场景</span>
          </div>
        </template>

        <template #fee_rate="{ row }">
          <span class="cell-muted">{{ row.fee_rate > 0 ? `${(row.fee_rate * 100).toFixed(2)}%` : '—' }}</span>
        </template>

        <template #priority="{ row }">
          <span class="cell-muted">{{ row.priority }}</span>
        </template>

        <template #health_status="{ row }">
          <div class="price-cell">
            <t-tag :theme="healthTheme(row.health_status)" variant="light" size="small" shape="round">
              {{ healthLabel(row.health_status) }}
            </t-tag>
            <span v-if="row.last_error" class="price-sub error-text" :title="row.last_error">{{ row.last_error }}</span>
          </div>
        </template>

        <template #is_default="{ row }">
          <t-tag v-if="row.is_default" theme="warning" variant="light" size="small" shape="round">默认</t-tag>
          <span v-else class="price-sub">—</span>
        </template>

        <template #status="{ row }">
          <t-switch
            :value="row.status === 1"
            :loading="statusLoadingId === row.id"
            @change="(v: unknown) => handleToggleStatus(row, Boolean(v))"
          />
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '连通测试', value: 'test' },
                { content: '编辑', value: 'edit', hidden: () => !canManage },
                { content: '能力详情', value: 'detail' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="handleTest(row)">连通测试</t-link>
              <t-link v-if="canManage" theme="primary" hover="color" @click="openEditDialog(row)">编辑</t-link>
              <t-link theme="primary" hover="color" @click="openDetail(row)">能力详情</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无支付渠道，点击右上角「新建渠道」接入" />
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
      :header="editingId ? '编辑支付渠道' : '新建支付渠道'"
      width="760px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: submitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSubmit"
      @close="formVisible = false"
    >
      <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="渠道编码" name="channel_code">
            <t-input v-model="form.channel_code" :disabled="!!editingId" placeholder="唯一编码，如 alipay_main" />
          </t-form-item>
          <t-form-item label="渠道名称" name="name">
            <t-input v-model="form.name" placeholder="用于后台展示，如 支付宝-主商户" />
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
          <t-form-item label="运行环境" name="environment">
            <t-select v-model="form.environment" :options="environmentOptions" />
          </t-form-item>
        </div>

        <PaymentCapability
          v-if="selectedDescriptor"
          :descriptor="selectedDescriptor"
          :title="`${selectedTypeLabel} 能力矩阵`"
          class="form-matrix"
        />

        <PaymentCredentialFields
          v-model="form.credentials"
          :fields="selectedDescriptor?.credential_schema"
          :empty-hint="selectedDescriptor ? '该渠道类型未声明凭证字段。' : '请先选择渠道类型。'"
        />

        <div class="form-grid">
          <t-form-item label="接口地址（Endpoint）" name="endpoint">
            <t-input v-model="form.endpoint" placeholder="选填，渠道 API 网关地址" />
          </t-form-item>
          <t-form-item label="异步通知地址（Notify URL）" name="notify_url">
            <t-input v-model="form.notify_url" placeholder="选填，留空用平台默认" />
          </t-form-item>
          <t-form-item label="同步跳转地址（Return URL）" name="return_url">
            <t-input v-model="form.return_url" placeholder="选填" />
          </t-form-item>
          <t-form-item label="结算模式" name="settle_mode">
            <t-select v-model="form.settle_mode" clearable :options="settleModeOptions" placeholder="选填" />
          </t-form-item>
          <t-form-item label="启用场景" name="scenes">
            <t-select v-model="form.scenes" multiple clearable :options="sceneOptions" placeholder="不选=全部场景" />
          </t-form-item>
          <t-form-item label="路由优先级" name="priority">
            <t-input-number v-model="form.priority" :min="0" theme="column" placeholder="数值越大越优先" />
          </t-form-item>
          <t-form-item label="同优先级权重" name="weight">
            <t-input-number v-model="form.weight" :min="0" theme="column" placeholder="选填" />
          </t-form-item>
          <t-form-item label="费率（%）" name="fee_rate">
            <t-input-number v-model="form.fee_rate_percent" :min="0" :max="100" :precision="2" theme="column" placeholder="如 0.60 表示 0.6%" />
          </t-form-item>
          <t-form-item label="单笔最小金额（元）" name="min_amount">
            <t-input-number v-model="form.min_amount" :min="0" :precision="2" theme="column" placeholder="0=不限" />
          </t-form-item>
          <t-form-item label="单笔最大金额（元）" name="max_amount">
            <t-input-number v-model="form.max_amount" :min="0" :precision="2" theme="column" placeholder="0=不限" />
          </t-form-item>
        </div>

        <div class="form-grid form-grid--single">
          <t-form-item label="平台默认渠道" name="is_default">
            <t-switch v-model="form.is_default" />
            <p class="field-help">开启后作为无用户偏好时的兜底渠道（同场景仅保留一个默认）。</p>
          </t-form-item>
          <t-form-item label="备注" name="remark">
            <t-textarea v-model="form.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>

    <!-- 能力详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="渠道能力详情" size="560px" :footer="false">
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
            <span class="recon-row__label">渠道类型</span>
            <span class="recon-row__value">{{ detail.type_name || detail.type }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">运行环境</span>
            <span class="recon-row__value">{{ detail.environment === 'sandbox' ? '沙箱' : '生产' }}</span>
          </div>
          <div class="recon-row">
            <span class="recon-row__label">最近检测</span>
            <span class="recon-row__value">{{ formatTime(detail.last_check_at) }}</span>
          </div>
        </div>
        <PaymentCapability :descriptor="detail.capabilities" title="能力矩阵" class="detail-matrix" />
        <div class="detail-block">
          <h4 class="detail-block__title">凭证（脱敏回显）</h4>
          <div v-for="(value, key) in detail.credentials" :key="key" class="recon-row">
            <span class="recon-row__label">{{ fieldLabel(key) }}</span>
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

import { AddIcon, LinkIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createChannel,
  getChannelList,
  getChannelTypes,
  testChannel,
  updateChannel,
  updateChannelStatus,
} from '@/api/payment'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import {
  channelStatusOptions,
  formatTime,
  healthLabel,
  healthTheme,
  modeLabel,
  sceneLabel,
  sceneOptions,
} from '@/pages/payment/constants'
import PaymentCapability from '@/pages/payment/components/PaymentCapability.vue'
import PaymentCredentialFields from '@/pages/payment/components/PaymentCredentialFields.vue'
import type { ChannelInfo, ChannelTypeItem } from '@/types/interface'
import { useUserStore } from '@/store'

defineOptions({ name: 'PaymentChannels' })

const userStore = useUserStore()
// 渠道维护（新建/编辑/启停）需要 payment:channel:manage。
const canManage = computed(() => userStore.permissions?.includes('payment:channel:manage') || userStore.isSuperAdmin)

const channelList = ref<ChannelInfo[]>([])
const typeList = ref<ChannelTypeItem[]>([])
const loading = ref(false)
const total = ref(0)
const { isMobile } = useIsMobile()

const filters = reactive<{ type?: string; status?: number; keyword?: string }>({
  type: undefined,
  status: undefined,
  keyword: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const environmentOptions = [
  { label: '生产环境', value: 'prod' },
  { label: '沙箱环境', value: 'sandbox' },
]

const settleModeOptions = [
  { label: 'T+0（当日结算）', value: 'T+0' },
  { label: 'T+1（次日结算）', value: 'T+1' },
]

const typeFilterOptions = computed(() => typeList.value.map((t) => ({ label: t.name, value: t.type })))
const typeSelectOptions = computed(() =>
  typeList.value.map((t) => ({ label: `${t.name}（${t.type}）`, value: t.type })),
)

const columns: PrimaryTableCol<ChannelInfo>[] = [
  { colKey: 'channel_code', title: '渠道', minWidth: 180 },
  { colKey: 'type', title: '类型 / 模式', width: 150 },
  { colKey: 'scenes', title: '启用场景', minWidth: 180 },
  { colKey: 'fee_rate', title: '费率', width: 90 },
  { colKey: 'priority', title: '优先级', width: 80, align: 'center' as const },
  { colKey: 'is_default', title: '默认', width: 80, align: 'center' as const },
  { colKey: 'health_status', title: '健康状态', minWidth: 130 },
  { colKey: 'status', title: '启用', width: 80, align: 'center' as const },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 220, fixed: 'right' as const, align: 'center' as const },
]

async function loadTypes() {
  try {
    const data = await getChannelTypes()
    typeList.value = data.items || []
  } catch {
    typeList.value = []
  }
}

async function loadChannels() {
  loading.value = true
  try {
    const data = await getChannelList({
      type: filters.type,
      status: filters.status,
      keyword: filters.keyword,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    channelList.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载支付渠道失败')
  } finally {
    loading.value = false
  }
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

async function handleToggleStatus(row: ChannelInfo, enabled: boolean) {
  statusLoadingId.value = row.id
  try {
    await updateChannelStatus(row.id, enabled ? 1 : 0)
    MessagePlugin.success(enabled ? '渠道已启用' : '渠道已停用')
    loadChannels()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新渠道状态失败')
  } finally {
    statusLoadingId.value = null
  }
}

// ===== 连通测试 =====
const testingId = ref<number | null>(null)

async function handleTest(row: ChannelInfo) {
  testingId.value = row.id
  try {
    const result = await testChannel(row.id)
    if (result.ok) {
      MessagePlugin.success(`「${row.name}」连通正常`)
    } else {
      MessagePlugin.error(`「${row.name}」连通失败：${result.message}`)
    }
    loadChannels()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '连通测试失败')
  } finally {
    testingId.value = null
  }
}

// ===== 表单 =====
type ChannelForm = {
  channel_code: string
  name: string
  type: string
  credentials: Record<string, string>
  endpoint: string
  notify_url: string
  return_url: string
  scenes: string[]
  priority: number
  weight: number
  fee_rate_percent: number
  settle_mode: string
  min_amount: number
  max_amount: number
  environment: string
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
  notify_url: '',
  return_url: '',
  scenes: [],
  priority: 0,
  weight: 0,
  fee_rate_percent: 0,
  settle_mode: '',
  min_amount: 0,
  max_amount: 0,
  environment: 'prod',
  is_default: false,
  remark: '',
})

const rules: Record<string, FormRule[]> = {
  channel_code: [{ required: true, message: '请输入渠道编码', type: 'error', trigger: 'blur' }],
  name: [{ required: true, message: '请输入渠道名称', type: 'error', trigger: 'blur' }],
  type: [{ required: true, message: '请选择渠道类型', type: 'error', trigger: 'change' }],
}

const selectedType = computed(() => typeList.value.find((t) => t.type === form.type))
const selectedDescriptor = computed(() => selectedType.value?.capabilities || null)
const selectedTypeLabel = computed(() => selectedType.value?.name || form.type)

function handleTypeChange() {
  // 切换类型后凭证字段结构变化，清空已填值避免脏数据。
  form.credentials = {}
  form.scenes = []
  form.settle_mode = ''
  form.fee_rate_percent = 0
}

function resetForm() {
  Object.assign(form, {
    channel_code: '',
    name: '',
    type: '',
    credentials: {},
    endpoint: '',
    notify_url: '',
    return_url: '',
    scenes: [],
    priority: 0,
    weight: 0,
    fee_rate_percent: 0,
    settle_mode: '',
    min_amount: 0,
    max_amount: 0,
    environment: 'prod',
    is_default: false,
    remark: '',
  })
  formRef.value?.clearValidate?.()
}

function openCreateDialog() {
  editingId.value = null
  resetForm()
  formVisible.value = true
}

function openEditDialog(row: ChannelInfo) {
  editingId.value = row.id
  Object.assign(form, {
    channel_code: row.channel_code,
    name: row.name,
    type: row.type,
    credentials: { ...(row.credentials || {}) },
    endpoint: row.endpoint || '',
    notify_url: row.notify_url || '',
    return_url: row.return_url || '',
    scenes: [...(row.scenes || [])],
    priority: row.priority || 0,
    weight: row.weight || 0,
    fee_rate_percent: Number(((row.fee_rate || 0) * 100).toFixed(2)),
    settle_mode: row.settle_mode || '',
    min_amount: (row.min_amount_fen || 0) / 100,
    max_amount: (row.max_amount_fen || 0) / 100,
    environment: row.environment || 'prod',
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
    const payload = {
      name: form.name,
      credentials: form.credentials,
      endpoint: form.endpoint,
      notify_url: form.notify_url,
      return_url: form.return_url,
      scenes: form.scenes,
      priority: form.priority,
      weight: form.weight,
      fee_rate: form.fee_rate_percent / 100,
      settle_mode: form.settle_mode,
      min_amount_fen: Math.round(form.min_amount * 100),
      max_amount_fen: Math.round(form.max_amount * 100),
      environment: form.environment,
      is_default: form.is_default,
      remark: form.remark,
    }
    if (editingId.value) {
      await updateChannel(editingId.value, payload)
      MessagePlugin.success('渠道已更新')
    } else {
      await createChannel({ ...payload, channel_code: form.channel_code, type: form.type })
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

// ===== 能力详情 =====
const detailVisible = ref(false)
const detail = ref<ChannelInfo | null>(null)

function openDetail(row: ChannelInfo) {
  detail.value = row
  detailVisible.value = true
}

function fieldLabel(key: string): string {
  const fields = detail.value?.capabilities?.credential_schema || []
  const found = fields.find((f) => f.key === key)
  return found ? found.label : key
}

function handleMobileAction(value: string | number | Record<string, any>, row: ChannelInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'test':
      handleTest(row)
      break
    case 'edit':
      openEditDialog(row)
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
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.form-grid--single {
  grid-template-columns: 1fr;
}

.form-matrix {
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

.detail-matrix {
  margin-bottom: var(--space-lg);
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

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style lang="css">
@import '../shared.css';
</style>
