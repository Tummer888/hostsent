<template>
  <div class="page-body system-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SecuredIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">验证码配置</h2>
          <p class="page-header__desc">
            图形验证码与短信/邮箱验证码的服务商、场景策略与下发统计。总开关 <code>captcha_enabled</code> 默认关闭：关闭时策略行只是「预配置」，不生效。
          </p>
        </div>
      </div>
    </header>

    <section class="surface-card main-card">
      <t-tabs v-model="activeTab" @change="handleTabChange">
        <t-tab-panel value="providers" label="服务商" />
        <t-tab-panel value="policies" label="场景策略" />
        <t-tab-panel value="stats" label="统计" />
      </t-tabs>

      <!-- Tab 1 服务商 -->
      <div v-if="activeTab === 'providers'" class="tab-body">
        <div class="tab-toolbar">
          <span class="tab-toolbar__hint">未接入的服务商显示「配置占位」：凭证可保存加密落库，测试时提示待接入，不会误报失败。</span>
          <t-space size="small">
            <t-button v-if="canManage" theme="primary" @click="openCreate">
              <template #icon><AddIcon aria-hidden="true" /></template>
              新建服务商
            </t-button>
            <t-button variant="outline" :loading="providerLoading" @click="loadProviders">
              <template #icon><RefreshIcon aria-hidden="true" /></template>
              刷新
            </t-button>
          </t-space>
        </div>

        <t-table
          row-key="id"
          :data="providers"
          :columns="providerColumns"
          :loading="providerLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="null"
        >
          <template #name="{ row }">
            <div class="price-cell">
              <span class="cell-strong">{{ row.name }}</span>
              <span class="price-sub">{{ row.provider_type }}</span>
            </div>
          </template>
          <template #mode="{ row }">
            <t-tag variant="light" size="small" shape="round">{{ providerModeLabel(row.mode) }}</t-tag>
          </template>
          <template #is_default="{ row }">
            <t-tag v-if="row.is_default" theme="warning" variant="light" size="small" shape="round">默认</t-tag>
            <span v-else class="price-sub">—</span>
          </template>
          <template #health_status="{ row }">
            <div class="price-cell">
              <t-tag :theme="healthTheme(row.health_status)" variant="light" size="small" shape="round">
                {{ healthLabel(row) }}
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
              :disabled="!canManage || row.builtin"
              :loading="statusLoadingId === row.id"
              @change="(v: unknown) => handleToggleStatus(row, Boolean(v))"
            />
          </template>
          <template #action="{ row }">
            <div class="action-cell">
              <t-link theme="primary" hover="color" @click="handleTest(row)">测试</t-link>
              <t-link v-if="canManage" theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link
                v-if="canManage && !row.builtin"
                theme="danger"
                hover="color"
                @click="handleDelete(row)"
              >
                删除
              </t-link>
            </div>
          </template>
          <template #empty>
            <t-empty description="暂无服务商" />
          </template>
        </t-table>
      </div>

      <!-- Tab 2 场景策略 -->
      <div v-else-if="activeTab === 'policies'" class="tab-body">
        <div class="tab-toolbar">
          <span class="tab-toolbar__hint">
            图形码与 OTP 开关一旦开启即为「平台强制」：用户端开关置灰，用户无法自行关闭。
          </span>
          <t-button variant="outline" :loading="policyLoading" @click="loadPolicies">
            <template #icon><RefreshIcon aria-hidden="true" /></template>
            刷新
          </t-button>
        </div>

        <t-table
          row-key="scene"
          :data="policies"
          :columns="policyColumns"
          :loading="policyLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="null"
        >
          <template #scene="{ row }">
            <div class="price-cell">
              <span class="cell-strong">
                {{ row.name || row.scene }}
                <t-tag
                  v-if="row.platform_forced"
                  theme="warning"
                  variant="light"
                  size="small"
                  shape="round"
                  class="force-badge"
                  title="开启后用户无法关闭"
                >
                  平台强制
                </t-tag>
              </span>
              <span class="price-sub">{{ row.scene }}</span>
            </div>
          </template>
          <template #image="{ row }">
            <div class="switch-cell">
              <t-switch
                :value="row.image_required"
                :disabled="!canManage"
                @change="(v: unknown) => handlePolicyToggle(row, 'image_required', Boolean(v))"
              />
              <t-select
                v-if="row.image_required"
                :value="row.image_level"
                size="small"
                class="level-select"
                :options="imageLevelOptions"
                :disabled="!canManage"
                @change="(v: unknown) => handlePolicyField(row, 'image_level', String(v))"
              />
            </div>
          </template>
          <template #otp="{ row }">
            <div class="switch-cell">
              <t-switch
                :value="row.otp_required"
                :disabled="!canManage"
                @change="(v: unknown) => handlePolicyToggle(row, 'otp_required', Boolean(v))"
              />
              <t-select
                v-if="row.otp_required"
                :value="row.otp_channel"
                size="small"
                class="level-select"
                :options="otpChannelOptions"
                :disabled="!canManage"
                @change="(v: unknown) => handlePolicyField(row, 'otp_channel', String(v))"
              />
            </div>
          </template>
          <template #advanced="{ row }">
            <span class="cell-muted">
              限次 {{ row.max_attempts }} · TTL {{ row.ttl_seconds }}s · 间隔 {{ row.send_interval_seconds }}s · 日限 {{ row.daily_limit_per_target }}
            </span>
          </template>
          <template #user_rights="{ row }">
            <div class="tag-list">
              <t-tag v-if="row.user_can_tighten" theme="success" variant="light" size="small" shape="round">可加严</t-tag>
              <t-tag v-if="row.user_can_choose_channel" theme="success" variant="light" size="small" shape="round">可换通道</t-tag>
              <t-tag v-if="row.min_channel_level > 0" theme="default" variant="light" size="small" shape="round">
                通道≥L{{ row.min_channel_level }}
              </t-tag>
              <span
                v-if="!row.user_can_tighten && !row.user_can_choose_channel && !row.min_channel_level"
                class="price-sub"
              >平台固定</span>
            </div>
          </template>
          <template #status="{ row }">
            <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
              {{ row.status === 'active' ? '启用' : '停用' }}
            </t-tag>
          </template>
          <template #action="{ row }">
            <t-link v-if="canManage" theme="primary" hover="color" @click="openPolicyEdit(row)">编辑</t-link>
            <span v-else class="cell-muted">—</span>
          </template>
          <template #empty>
            <t-empty description="暂无场景策略" />
          </template>
        </t-table>
      </div>

      <!-- Tab 3 统计 -->
      <div v-else class="tab-body">
        <div class="tab-toolbar">
          <t-date-range-picker v-model="statsRange" clearable allow-input @change="loadStats" />
          <t-button variant="outline" :loading="statsLoading" @click="loadStats">
            <template #icon><RefreshIcon aria-hidden="true" /></template>
            刷新
          </t-button>
        </div>

        <div class="stat-grid">
          <div class="stat-card stat-card--mini">
            <span class="stat-label">下发总量</span>
            <span class="stat-value">{{ totals.total }}</span>
          </div>
          <div class="stat-card stat-card--mini">
            <span class="stat-label">通过量</span>
            <span class="stat-value">{{ totals.used }}</span>
          </div>
          <div class="stat-card stat-card--mini">
            <span class="stat-label">通过率</span>
            <span class="stat-value">{{ (totals.passRate * 100).toFixed(1) }}%</span>
          </div>
          <div class="stat-card stat-card--mini">
            <span class="stat-label">短信费用（元）</span>
            <span class="stat-value">{{ fenToYuan(totals.costFen) }}</span>
          </div>
        </div>

        <t-table
          row-key="scene"
          :data="stats"
          :columns="statColumns"
          :loading="statsLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="null"
        >
          <template #scene="{ row }">
            <span class="cell-strong">{{ sceneLabel(row.scene) }}</span>
            <span class="price-sub">{{ row.scene }}</span>
          </template>
          <template #pass_rate="{ row }">
            <span :class="row.pass_rate >= 0.8 ? 'rate-ok' : 'rate-warn'">{{ (row.pass_rate * 100).toFixed(1) }}%</span>
          </template>
          <template #cost="{ row }">
            <span class="cell-muted">{{ row.cost_yuan?.toFixed(2) ?? fenToYuan(row.cost_fen) }} 元</span>
          </template>
          <template #empty>
            <t-empty description="所选区间内无下发记录" />
          </template>
        </t-table>
      </div>
    </section>

    <!-- 服务商 新建/编辑 -->
    <t-dialog
      v-model:visible="formVisible"
      :header="editingId ? '编辑服务商' : '新建服务商'"
      width="720px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: submitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSubmit"
      @close="formVisible = false"
    >
      <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="服务商类型" name="provider_type">
            <t-select
              v-model="form.provider_type"
              :disabled="!!editingId"
              placeholder="请选择服务商类型"
              :options="providerTypeOptions"
              @change="handleTypeChange"
            />
          </t-form-item>
          <t-form-item label="名称" name="name">
            <t-input v-model="form.name" placeholder="用于后台展示，如 易盾-主账号" />
          </t-form-item>
          <t-form-item label="接口地址（Endpoint）" name="endpoint">
            <t-input v-model="form.endpoint" placeholder="选填，服务商 API 网关地址" />
          </t-form-item>
          <t-form-item label="优先级" name="priority">
            <t-input-number v-model="form.priority" :min="0" theme="column" placeholder="数值越大越优先" />
          </t-form-item>
        </div>

        <t-alert
          v-if="selectedType && !selectedType.implemented && !selectedType.builtin"
          theme="info"
          class="form-hint"
          message="该服务商适配器待接入"
          description="凭证可正常保存并加密落库；「测试」会提示待接入，不会误报失败。"
        />

        <CredentialFields
          v-model="form.credentials"
          :fields="selectedType?.credential_schema"
          :empty-hint="selectedType ? '该服务商类型未声明凭证字段。' : '请先选择服务商类型。'"
        />

        <div class="form-grid form-grid--single">
          <t-form-item label="默认服务商" name="is_default">
            <t-switch v-model="form.is_default" :disabled="form.provider_type === 'native'" />
            <p class="field-help">默认服务商用于未指定类型的场景；原生图形码为内建兜底，恒为可用。</p>
          </t-form-item>
          <t-form-item label="备注" name="remark">
            <t-textarea v-model="form.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>

    <!-- 场景策略 编辑 -->
    <t-drawer
      v-model:visible="policyVisible"
      header="编辑场景策略"
      size="560px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: policySaving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handlePolicySave"
      @close="policyVisible = false"
    >
      <div v-if="policyForm" class="policy-form">
        <div class="recon-row">
          <span class="recon-row__label">场景</span>
          <span class="recon-row__value">{{ policyForm.name || policyForm.scene }}（{{ policyForm.scene }}）</span>
        </div>
        <t-form label-align="top" @submit.prevent>
          <div class="form-grid">
            <t-form-item label="图形码">
              <t-switch v-model="policyForm.image_required" />
            </t-form-item>
            <t-form-item label="图形码强度">
              <t-select v-model="policyForm.image_level" :options="imageLevelOptions" :disabled="!policyForm.image_required" />
            </t-form-item>
            <t-form-item label="OTP 验证">
              <t-switch v-model="policyForm.otp_required" />
            </t-form-item>
            <t-form-item label="OTP 通道">
              <t-select v-model="policyForm.otp_channel" :options="otpChannelOptions" :disabled="!policyForm.otp_required" />
            </t-form-item>
            <t-form-item label="用户可加严">
              <t-switch v-model="policyForm.user_can_tighten" />
            </t-form-item>
            <t-form-item label="用户可换通道">
              <t-switch v-model="policyForm.user_can_choose_channel" />
            </t-form-item>
            <t-form-item label="最小通道等级">
              <t-input-number v-model="policyForm.min_channel_level" :min="0" :max="4" theme="column" />
              <p class="field-help">1=图形码 2=邮箱 3=短信 4=TOTP；0=不限制。</p>
            </t-form-item>
            <t-form-item label="单 key 校验次数">
              <t-input-number v-model="policyForm.max_attempts" :min="1" theme="column" />
            </t-form-item>
            <t-form-item label="有效期（秒）">
              <t-input-number v-model="policyForm.ttl_seconds" :min="30" theme="column" />
            </t-form-item>
            <t-form-item label="发送间隔（秒）">
              <t-input-number v-model="policyForm.send_interval_seconds" :min="0" theme="column" />
            </t-form-item>
            <t-form-item label="单目标日限">
              <t-input-number v-model="policyForm.daily_limit_per_target" :min="0" theme="column" />
            </t-form-item>
            <t-form-item label="状态">
              <t-select v-model="policyForm.status" :options="policyStatusOptions" />
            </t-form-item>
          </div>
          <t-form-item label="备注">
            <t-textarea v-model="policyForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
          </t-form-item>
        </t-form>
      </div>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, RefreshIcon, SecuredIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type FormInstanceFunctions, type FormRule, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createCaptchaProvider,
  deleteCaptchaProvider,
  getCaptchaPolicies,
  getCaptchaProviderTypes,
  getCaptchaProviders,
  getCaptchaStats,
  testCaptchaProvider,
  updateCaptchaPolicy,
  updateCaptchaProvider,
  type CaptchaPolicyInfo,
  type CaptchaPolicyUpdateRequest,
  type CaptchaProviderInfo,
  type CaptchaProviderType,
  type CaptchaSceneStat,
} from '@/api/captcha'
import CredentialFields from '@/components/credential-fields/index.vue'
import { formatTime, fenToYuan } from '@/pages/notification/constants'
import { useUserStore } from '@/store'

defineOptions({ name: 'SystemCaptcha' })

const userStore = useUserStore()
const canManage = computed(() => userStore.permissions?.includes('captcha:config:manage') || userStore.isSuperAdmin)

const activeTab = ref<'providers' | 'policies' | 'stats'>('providers')

function handleTabChange(value: unknown) {
  const tab = String(value)
  if (tab === 'providers') loadProviders()
  else if (tab === 'policies') loadPolicies()
  else loadStats()
}

// ===== Tab 1 服务商 =====
const providers = ref<CaptchaProviderInfo[]>([])
const providerTypes = ref<CaptchaProviderType[]>([])
const providerLoading = ref(false)
const statusLoadingId = ref<number | null>(null)

const providerModeMap: Record<string, string> = {
  native: '内建',
  api: '接口',
  manual: '人工',
}

function providerModeLabel(mode: string): string {
  return providerModeMap[mode] || mode || '—'
}

function healthLabel(row: CaptchaProviderInfo): string {
  // 未接入的服务商一律显示「待接入」，覆盖数据库里的历史 health 值。
  if (!row.implemented && !row.builtin) return '待接入'
  switch (row.health_status) {
    case 'healthy':
      return '正常'
    case 'down':
      return '异常'
    case 'pending':
      return '待接入'
    default:
      return '未检测'
  }
}

function healthTheme(status: string): string {
  switch (status) {
    case 'healthy':
      return 'success'
    case 'down':
      return 'danger'
    case 'pending':
      return 'default'
    default:
      return 'warning'
  }
}

const providerColumns: PrimaryTableCol<CaptchaProviderInfo>[] = [
  { colKey: 'name', title: '服务商', minWidth: 180 },
  { colKey: 'mode', title: '模式', width: 90 },
  { colKey: 'is_default', title: '默认', width: 80, align: 'center' },
  { colKey: 'health_status', title: '健康', minWidth: 140 },
  { colKey: 'last_check_at', title: '最后检查', width: 160 },
  { colKey: 'status', title: '启用', width: 80, align: 'center' },
  { colKey: 'action', title: '操作', width: 190, fixed: 'right', align: 'center' },
]

async function loadProviderTypes() {
  try {
    providerTypes.value = await getCaptchaProviderTypes()
  } catch {
    providerTypes.value = []
  }
}

async function loadProviders() {
  providerLoading.value = true
  try {
    providers.value = (await getCaptchaProviders()) || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载服务商失败')
  } finally {
    providerLoading.value = false
  }
}

async function handleToggleStatus(row: CaptchaProviderInfo, enabled: boolean) {
  statusLoadingId.value = row.id
  try {
    await updateCaptchaProvider(row.id, { provider_type: row.provider_type, status: enabled ? 1 : 0 })
    MessagePlugin.success(enabled ? '服务商已启用' : '服务商已停用')
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新状态失败')
  } finally {
    statusLoadingId.value = null
  }
}

async function handleTest(row: CaptchaProviderInfo) {
  try {
    const msg = await testCaptchaProvider(row.id)
    MessagePlugin.success(msg || '测试通过')
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '测试失败')
  }
}

function handleDelete(row: CaptchaProviderInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除服务商',
    body: `确认删除「${row.name}」吗？若它是当前默认服务商，删除后需另设默认。`,
    confirmBtn: { content: '删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteCaptchaProvider(row.id)
        MessagePlugin.success('服务商已删除')
        dialog.destroy()
        loadProviders()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

// ---- 服务商表单 ----
type ProviderForm = {
  provider_type: string
  name: string
  endpoint: string
  credentials: Record<string, string>
  priority: number
  is_default: boolean
  remark: string
}

const formVisible = ref(false)
const submitting = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstanceFunctions | null>(null)

const form = reactive<ProviderForm>({
  provider_type: '',
  name: '',
  endpoint: '',
  credentials: {},
  priority: 0,
  is_default: false,
  remark: '',
})

const rules: Record<string, FormRule[]> = {
  provider_type: [{ required: true, message: '请选择服务商类型', type: 'error', trigger: 'change' }],
}

const providerTypeOptions = computed(() =>
  providerTypes.value.map((t) => ({
    label: t.implemented || t.builtin ? `${t.name}（${t.type}）` : `${t.name} · 配置占位`,
    value: t.type,
  })),
)

const selectedType = computed(() => providerTypes.value.find((t) => t.type === form.provider_type) || null)

function handleTypeChange() {
  form.credentials = {}
  form.is_default = form.provider_type === 'native'
}

function resetForm() {
  Object.assign(form, {
    provider_type: '',
    name: '',
    endpoint: '',
    credentials: {},
    priority: 0,
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

function openEdit(row: CaptchaProviderInfo) {
  editingId.value = row.id
  Object.assign(form, {
    provider_type: row.provider_type,
    name: row.name || '',
    endpoint: row.endpoint || '',
    credentials: { ...(row.credentials || {}) },
    priority: row.priority || 0,
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
      provider_type: form.provider_type,
      name: form.name,
      endpoint: form.endpoint,
      // 密文字段保持掩码原样回传，后端识别为「未修改」。
      credentials: form.credentials,
      priority: form.priority,
      is_default: form.is_default,
      remark: form.remark,
    }
    if (editingId.value) {
      await updateCaptchaProvider(editingId.value, payload)
      MessagePlugin.success('服务商已更新')
    } else {
      await createCaptchaProvider(payload)
      MessagePlugin.success('服务商已创建')
    }
    formVisible.value = false
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    submitting.value = false
  }
}

// ===== Tab 2 场景策略 =====
const policies = ref<CaptchaPolicyInfo[]>([])
const policyLoading = ref(false)

const imageLevelOptions = [
  { label: '简单', value: 'easy' },
  { label: '标准', value: 'normal' },
  { label: '困难', value: 'hard' },
]

const otpChannelOptions = [
  { label: '邮箱', value: 'email' },
  { label: '短信', value: 'sms' },
]

const policyStatusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'disabled' },
]

// 场景中文名（与后端 seed 的 name 对齐；后端 name 优先，这里兜底）。
const sceneLabelMap: Record<string, string> = {
  admin_login: '管理员登录',
  user_login: '用户登录',
  user_login_sms: '短信登录',
  user_login_email: '邮箱登录',
  user_register: '用户注册',
  password_reset: '找回密码',
  password_change: '修改密码',
  phone_bind: '绑定手机',
  email_bind: '绑定邮箱',
  withdraw_apply: '申请提现',
  payout_apply: '代付申请',
  apikey_create: '创建 API 密钥',
  apikey_view: '查看 API 密钥',
  instance_destroy: '销毁实例',
  instance_resize: '变配实例',
  admin_grant_change: '管理员权限变更',
  realname_submit: '提交实名',
}

function sceneLabel(scene: string): string {
  return sceneLabelMap[scene] || scene
}

const policyColumns: PrimaryTableCol<CaptchaPolicyInfo>[] = [
  { colKey: 'scene', title: '场景', minWidth: 200 },
  { colKey: 'image', title: '图形码', width: 170 },
  { colKey: 'otp', title: 'OTP 验证', width: 170 },
  { colKey: 'advanced', title: '频控参数', minWidth: 240 },
  { colKey: 'user_rights', title: '用户权限', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'action', title: '操作', width: 90, fixed: 'right', align: 'center' },
]

async function loadPolicies() {
  policyLoading.value = true
  try {
    policies.value = (await getCaptchaPolicies()) || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载场景策略失败')
  } finally {
    policyLoading.value = false
  }
}

async function patchPolicy(scene: string, body: CaptchaPolicyUpdateRequest, successMsg: string) {
  try {
    await updateCaptchaPolicy(scene, body)
    MessagePlugin.success(successMsg)
    loadPolicies()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新策略失败')
    // 失败时重新拉取，避免本地开关与后端不一致地停在被切换后的位置。
    loadPolicies()
  }
}

function handlePolicyToggle(
  row: CaptchaPolicyInfo,
  field: 'image_required' | 'otp_required',
  value: boolean,
) {
  patchPolicy(row.scene, { [field]: value }, value ? '已开启' : '已关闭')
}

function handlePolicyField(
  row: CaptchaPolicyInfo,
  field: 'image_level' | 'otp_channel',
  value: string,
) {
  patchPolicy(row.scene, { [field]: value }, '已更新')
}

// ---- 策略编辑抽屉 ----
const policyVisible = ref(false)
const policySaving = ref(false)
const policyForm = ref<CaptchaPolicyInfo | null>(null)

function openPolicyEdit(row: CaptchaPolicyInfo) {
  // 深拷贝：抽屉里的改动在点保存前不应影响表格行。
  policyForm.value = { ...row }
  policyVisible.value = true
}

async function handlePolicySave() {
  if (!policyForm.value) return
  const p = policyForm.value
  policySaving.value = true
  try {
    await updateCaptchaPolicy(p.scene, {
      image_required: p.image_required,
      image_level: p.image_level,
      otp_required: p.otp_required,
      otp_channel: p.otp_channel,
      min_channel_level: p.min_channel_level,
      user_can_tighten: p.user_can_tighten,
      user_can_choose_channel: p.user_can_choose_channel,
      max_attempts: p.max_attempts,
      ttl_seconds: p.ttl_seconds,
      send_interval_seconds: p.send_interval_seconds,
      daily_limit_per_target: p.daily_limit_per_target,
      status: p.status,
      remark: p.remark,
    })
    MessagePlugin.success('策略已更新')
    policyVisible.value = false
    loadPolicies()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存策略失败')
  } finally {
    policySaving.value = false
  }
}

// ===== Tab 3 统计 =====
const stats = ref<CaptchaSceneStat[]>([])
const statsLoading = ref(false)
const statsRange = ref<string[] | null>(null)

const totals = computed(() => {
  let total = 0
  let used = 0
  let costFen = 0
  for (const item of stats.value) {
    total += item.total
    used += item.used
    costFen += item.cost_fen
  }
  return { total, used, costFen, passRate: total > 0 ? used / total : 0 }
})

const statColumns: PrimaryTableCol<CaptchaSceneStat>[] = [
  { colKey: 'scene', title: '场景', minWidth: 200 },
  { colKey: 'total', title: '下发量', width: 100, align: 'right' },
  { colKey: 'used', title: '通过量', width: 100, align: 'right' },
  { colKey: 'pass_rate', title: '通过率', width: 110, align: 'right' },
  { colKey: 'failed', title: '校验失败', width: 100, align: 'right' },
  { colKey: 'expired', title: '过期', width: 90, align: 'right' },
  { colKey: 'cost', title: '短信费用', minWidth: 120, align: 'right' },
]

async function loadStats() {
  statsLoading.value = true
  try {
    const resp = await getCaptchaStats({
      from: statsRange.value?.[0] || undefined,
      to: statsRange.value?.[1] || undefined,
    })
    stats.value = resp.items || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载统计失败')
  } finally {
    statsLoading.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadProviderTypes(), loadProviders()])
})
</script>

<style lang="css" scoped>
.main-card {
  padding: var(--space-lg) var(--space-xl);
}

.tab-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  padding-top: var(--space-md);
}

.tab-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.tab-toolbar__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
  max-width: 720px;
}

.price-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.price-sub {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.cell-strong {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.cell-muted {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.time-text {
  font-size: 12px;
  color: #334155;
  font-variant-numeric: tabular-nums;
}

.error-text {
  color: #dc2626;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.action-cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  flex-wrap: wrap;
}

.force-badge {
  margin-left: 6px;
}

.switch-cell {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.level-select {
  width: 100px;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.form-grid--single {
  grid-template-columns: 1fr;
}

.form-hint {
  margin-bottom: var(--space-md);
}

.field-help {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.policy-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.recon-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  font-size: 13px;
}

.recon-row__label {
  color: var(--color-muted-foreground);
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: var(--space-md);
}

.stat-card--mini {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: var(--space-md);
  border: 1px solid var(--hs-border-color, #e5e7eb);
  border-radius: var(--hs-radius-lg, 8px);
  background: var(--hs-surface-1, #fff);
}

.stat-label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.stat-value {
  font-size: 22px;
  font-weight: 600;
  color: #1f2937;
}

.rate-ok {
  color: #16a34a;
}

.rate-warn {
  color: #d97706;
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
