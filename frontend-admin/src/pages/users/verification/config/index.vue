<template>
  <div class="page-body system-page verification-config-page">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">认证配置</h2>
          <p class="page-header__desc">
            实名认证的开关、允许的主体类型、生效的核验服务商与审核策略。此处改的是策略，核验服务商的密钥在「核验服务商」页签里配置。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button v-if="canConfig" theme="primary" @click="openCreateConfig">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新增配置项
        </t-button>
        <t-button variant="outline" :loading="configLoading" @click="loadConfigs">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="surface-card main-card">
      <t-tabs v-model="activeTab" @change="handleTabChange">
        <t-tab-panel value="policies" label="认证策略" />
        <t-tab-panel value="providers" label="核验服务商" />
      </t-tabs>

      <!-- Tab 1 认证策略 -->
      <div v-if="activeTab === 'policies'" class="tab-body">
        <div class="tab-toolbar">
          <span class="tab-toolbar__hint">
            布尔型配置直接改开关即保存；其余字段点「编辑」在弹窗里改。配置项缺失时后端按代码默认值回落，不会让实名流程中断。
          </span>
        </div>

        <t-table
          row-key="id"
          :data="configs"
          :columns="configColumns"
          :loading="configLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="null"
        >
          <template #config_key="{ row }">
            <div class="price-cell">
              <span class="cell-strong">{{ configLabel(row.config_key) }}</span>
              <span class="price-sub">{{ row.config_key }}</span>
            </div>
          </template>
          <template #config_value="{ row }">
            <t-switch
              v-if="row.value_type === 'bool'"
              :value="row.config_value === 'true'"
              :disabled="!canConfig"
              :loading="savingKey === row.config_key"
              @change="(v: unknown) => toggleBoolConfig(row, Boolean(v))"
            />
            <span v-else class="cell-muted">{{ row.config_value || '—' }}</span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
              {{ row.status === 'active' ? '启用' : '停用' }}
            </t-tag>
          </template>
          <template #updated_at="{ row }">{{ formatTime(row.updated_at) }}</template>
          <template #action="{ row }">
            <t-space size="small">
              <t-link v-if="canConfig" theme="primary" hover="color" @click="openEditConfig(row)">编辑</t-link>
              <t-link v-if="canConfig" theme="danger" hover="color" @click="handleDeleteConfig(row)">删除</t-link>
            </t-space>
          </template>
        </t-table>
      </div>

      <!-- Tab 2 核验服务商 -->
      <div v-else class="tab-body">
        <div class="tab-toolbar">
          <span class="tab-toolbar__hint">
            人工审核是内置兜底（不可删除、不可停用）：未接入三方核验时实名走人工审核。三方核验凭证以字段级加密落库，回显为掩码。
          </span>
          <t-space size="small">
            <t-button v-if="canConfig" theme="primary" @click="openCreateProvider">
              <template #icon><AddIcon aria-hidden="true" /></template>
              新增服务商
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
                {{ healthLabel(row.health_status) }}
              </t-tag>
              <span v-if="row.last_error" class="price-sub error-text" :title="row.last_error">{{ row.last_error }}</span>
            </div>
          </template>
          <template #status="{ row }">
            <t-switch
              :value="row.status === 1"
              :disabled="!canConfig || row.builtin"
              :loading="statusLoadingId === row.id"
              @change="(v: unknown) => handleToggleProviderStatus(row, Boolean(v))"
            />
          </template>
          <template #action="{ row }">
            <t-space size="small">
              <t-link v-if="canConfig" theme="primary" hover="color" @click="openEditProvider(row)">编辑</t-link>
              <t-link theme="primary" hover="color" :disabled="testingId === row.id" @click="handleTestProvider(row)">测试</t-link>
              <t-link v-if="canConfig && !row.builtin" theme="danger" hover="color" @click="handleDeleteProvider(row)">删除</t-link>
            </t-space>
          </template>
        </t-table>
      </div>
    </section>

    <!-- 配置项编辑 -->
    <t-dialog
      v-model:visible="configDialogVisible"
      :header="editingConfigKey ? '编辑配置项' : '新增配置项'"
      width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: configSaving }"
      @confirm="handleSaveConfig"
    >
      <t-form ref="configFormRef" label-align="top" :data="configForm" :rules="configRules" @submit.prevent>
        <t-form-item label="配置键" name="config_key">
          <t-input v-model="configForm.config_key" :disabled="!!editingConfigKey" placeholder="如 verification.resubmit_cooldown_hours" />
        </t-form-item>
        <t-form-item label="值类型" name="value_type">
          <t-select v-model="configForm.value_type" :options="valueTypeOptions" />
        </t-form-item>
        <t-form-item label="配置值" name="config_value">
          <t-input v-model="configForm.config_value" placeholder="布尔填 true/false；JSON 填合法 JSON 文本" />
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-select v-model="configForm.status" :options="configStatusOptions" />
        </t-form-item>
        <t-form-item label="说明" name="description">
          <t-textarea v-model="configForm.description" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，展示在列表与配置页" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 服务商编辑 -->
    <t-dialog
      v-model:visible="providerDialogVisible"
      :header="providerForm.id ? '编辑核验服务商' : '新增核验服务商'"
      width="640px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: providerSaving }"
      @confirm="handleSaveProvider"
    >
      <t-form ref="providerFormRef" label-align="top" :data="providerForm" :rules="providerRules" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="服务商类型" name="provider_type">
            <t-select
              v-model="providerForm.provider_type"
              :disabled="!!providerForm.id"
              :options="providerTypeOptions"
              placeholder="请选择服务商类型"
              @change="handleProviderTypeChange"
            />
          </t-form-item>
          <t-form-item label="名称" name="name">
            <t-input v-model="providerForm.name" placeholder="用于后台展示，如 支付宝-主账号" />
          </t-form-item>
          <t-form-item label="接口地址（Endpoint）" name="endpoint">
            <t-input v-model="providerForm.endpoint" placeholder="选填，服务商 API 网关地址" />
          </t-form-item>
          <t-form-item label="优先级" name="priority">
            <t-input-number v-model="providerForm.priority" :min="0" theme="column" placeholder="数值越大越优先" />
          </t-form-item>
        </div>

        <t-alert
          v-if="selectedProviderType && !selectedProviderType.implemented"
          theme="info"
          class="form-hint"
          message="该服务商适配器待接入"
          description="凭证可正常保存并加密落库；「测试」会提示待接入，不会误报失败。"
        />

        <CredentialFields
          v-model="providerForm.credentials"
          :fields="selectedProviderType?.credential_schema"
          :empty-hint="selectedProviderType ? '该服务商类型未声明凭证字段。' : '请先选择服务商类型。'"
        />

        <div class="form-grid form-grid--single">
          <t-form-item label="默认服务商" name="is_default">
            <t-switch v-model="providerForm.is_default" :disabled="providerForm.provider_type === 'manual'" />
            <p class="field-help">
              默认服务商即「生效的核验服务商」，需与「认证策略 → verification.provider」的取值一致才会被使用。
              人工审核为内置兜底，恒为默认可用。
            </p>
          </t-form-item>
          <t-form-item label="备注" name="remark">
            <t-textarea v-model="providerForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
// 实名认证配置（doc104 §5.4/§5.5）。
//
// 本页此前是「功能待接入」的空壳：后端已有 verification_configs 与 realname_providers
// 两张表与完整读写接口，前端却没有入口，运营改不了策略、配不了三方核验凭证。
// 现在拆成两个页签：策略（verification_configs）+ 服务商（realname_providers）。
import { computed, onMounted, reactive, ref } from 'vue'
import { AddIcon, RefreshIcon, SettingIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type FormInstanceFunctions, type FormRule, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  deleteVerificationConfig,
  deleteVerificationProvider,
  getVerificationConfigs,
  getVerificationProviders,
  getVerificationProviderTypes,
  testVerificationProvider,
  upsertVerificationConfig,
  upsertVerificationProvider,
  type VerificationConfigInfo,
  type VerificationConfigUpsertRequest,
  type VerificationProviderInfo,
  type VerificationProviderType,
  type VerificationProviderUpsertRequest,
} from '@/api/verification'
import CredentialFields from '@/components/credential-fields/index.vue'
import { useUserStore } from '@/store'

defineOptions({ name: 'UserVerificationConfig' })

const userStore = useUserStore()
const canConfig = computed(() => userStore.permissions?.includes('verification:config') || userStore.isSuperAdmin)

const activeTab = ref<'policies' | 'providers'>('policies')

// ---------- 配置项 ----------

const configLoading = ref(false)
const configs = ref<VerificationConfigInfo[]>([])
const configDialogVisible = ref(false)
const configSaving = ref(false)
const editingConfigKey = ref('')
const savingKey = ref('')
const configFormRef = ref<FormInstanceFunctions>()
const configForm = reactive<VerificationConfigUpsertRequest>({
  config_key: '',
  config_value: '',
  value_type: 'string',
  status: 'active',
  description: '',
})

const configRules: Record<string, FormRule[]> = {
  config_key: [{ required: true, message: '请填写配置键' }],
}

const valueTypeOptions = [
  { label: '字符串 string', value: 'string' },
  { label: '布尔 bool', value: 'bool' },
  { label: '整数 int', value: 'int' },
  { label: 'JSON', value: 'json' },
]

const configStatusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'disabled' },
]

// 配置键的中文标签：运营不该靠猜键名判断一项配置在管什么。
const configLabels: Record<string, string> = {
  'verification.enabled': '开放自助实名提交',
  'verification.allowed_types': '允许的认证主体类型',
  'verification.provider': '生效的核验服务商',
  'verification.auto_approve_on_pass': '三方核验通过即自动通过',
  'verification.auto_reject_on_fail': '三方核验失败即自动驳回',
  'verification.require_before_order': '下单前强制实名',
  'verification.resubmit_cooldown_hours': '驳回后重新提交冷却（小时）',
  'verification.real_name_locked': '通过后禁止再次提交',
}

function configLabel(key: string) {
  return configLabels[key] || key
}

const configColumns: PrimaryTableCol<VerificationConfigInfo>[] = [
  { colKey: 'config_key', title: '配置项', minWidth: 240 },
  { colKey: 'config_value', title: '当前值', width: 160 },
  { colKey: 'value_type', title: '类型', width: 100 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'description', title: '说明', minWidth: 240, ellipsis: true },
  { colKey: 'updated_at', title: '更新时间', width: 180 },
  { colKey: 'action', title: '操作', width: 130, fixed: 'right' },
]

async function loadConfigs() {
  configLoading.value = true
  try {
    configs.value = await getVerificationConfigs()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载实名配置失败')
  } finally {
    configLoading.value = false
  }
}

function openCreateConfig() {
  editingConfigKey.value = ''
  configForm.config_key = ''
  configForm.config_value = ''
  configForm.value_type = 'string'
  configForm.status = 'active'
  configForm.description = ''
  configDialogVisible.value = true
}

function openEditConfig(row: VerificationConfigInfo) {
  editingConfigKey.value = row.config_key
  configForm.config_key = row.config_key
  configForm.config_value = row.config_value
  configForm.value_type = row.value_type || 'string'
  configForm.status = row.status || 'active'
  configForm.description = row.description || ''
  configDialogVisible.value = true
}

async function handleSaveConfig() {
  const valid = await configFormRef.value?.validate()
  if (valid !== true) return
  configSaving.value = true
  try {
    await upsertVerificationConfig({ ...configForm })
    MessagePlugin.success('配置已保存')
    configDialogVisible.value = false
    await loadConfigs()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    configSaving.value = false
  }
}

// 布尔型配置直接切开关：改成文本再点保存太绕，运营只是想把开关翻过来。
async function toggleBoolConfig(row: VerificationConfigInfo, value: boolean) {
  const previous = row.config_value
  row.config_value = value ? 'true' : 'false'
  savingKey.value = row.config_key
  try {
    await upsertVerificationConfig({
      config_key: row.config_key,
      config_value: row.config_value,
      value_type: row.value_type,
      status: row.status,
      description: row.description,
    })
    MessagePlugin.success(`${configLabel(row.config_key)} 已${value ? '开启' : '关闭'}`)
  } catch (error) {
    row.config_value = previous
    MessagePlugin.error((error as Error)?.message || '切换失败')
  } finally {
    savingKey.value = ''
  }
}

function handleDeleteConfig(row: VerificationConfigInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除配置项',
    body: `确认删除「${configLabel(row.config_key)}」？删除后该项回落为代码默认值，而不是变成「未配置」。`,
    theme: 'warning',
    onConfirm: async () => {
      try {
        await deleteVerificationConfig(row.id)
        MessagePlugin.success('已删除')
        await loadConfigs()
      } catch (error) {
        MessagePlugin.error((error as Error)?.message || '删除失败')
      } finally {
        dialog.destroy()
      }
    },
  })
}

// ---------- 服务商 ----------

const providerLoading = ref(false)
const providers = ref<VerificationProviderInfo[]>([])
const providerTypes = ref<VerificationProviderType[]>([])
const providerDialogVisible = ref(false)
const providerSaving = ref(false)
const testingId = ref(0)
const statusLoadingId = ref(0)
const providerFormRef = ref<FormInstanceFunctions>()
const providerForm = reactive<VerificationProviderUpsertRequest>({
  id: 0,
  provider_type: '',
  name: '',
  endpoint: '',
  priority: 0,
  status: 1,
  is_default: false,
  remark: '',
  credentials: {},
})

const providerRules: Record<string, FormRule[]> = {
  provider_type: [{ required: true, message: '请选择服务商类型' }],
}

const providerTypeOptions = computed(() =>
  providerTypes.value.map((t) => ({
    label: t.implemented ? `${t.name}（${t.type}）` : `${t.name}（待接入）`,
    value: t.type,
  })),
)

const selectedProviderType = computed(() => providerTypes.value.find((t) => t.type === providerForm.provider_type))

const providerColumns: PrimaryTableCol<VerificationProviderInfo>[] = [
  { colKey: 'name', title: '服务商', minWidth: 180 },
  { colKey: 'mode', title: '核验形态', width: 110 },
  { colKey: 'is_default', title: '默认', width: 90 },
  { colKey: 'priority', title: '优先级', width: 90 },
  { colKey: 'health_status', title: '健康状态', minWidth: 180 },
  { colKey: 'status', title: '启用', width: 90 },
  { colKey: 'remark', title: '备注', minWidth: 160, ellipsis: true },
  { colKey: 'action', title: '操作', width: 170, fixed: 'right' },
]

function providerModeLabel(mode: string) {
  const map: Record<string, string> = { manual: '人工审核', api: '无跳转核验', redirect: '跳转式核验' }
  return map[mode] || mode || '—'
}

function healthLabel(status: string) {
  const map: Record<string, string> = { healthy: '正常', down: '异常', pending: '待接入' }
  return map[status] || '未测试'
}

function healthTheme(status: string) {
  const map: Record<string, string> = { healthy: 'success', down: 'danger', pending: 'warning' }
  return (map[status] || 'default') as 'success' | 'danger' | 'warning' | 'default'
}

function formatTime(value?: string) {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

async function loadProviders() {
  providerLoading.value = true
  try {
    const [list, types] = await Promise.all([getVerificationProviders(), getVerificationProviderTypes()])
    providers.value = list || []
    providerTypes.value = types || []
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载核验服务商失败')
  } finally {
    providerLoading.value = false
  }
}

function openCreateProvider() {
  providerForm.id = 0
  providerForm.provider_type = ''
  providerForm.name = ''
  providerForm.endpoint = ''
  providerForm.priority = 0
  providerForm.status = 1
  providerForm.is_default = false
  providerForm.remark = ''
  providerForm.credentials = {}
  providerDialogVisible.value = true
}

function openEditProvider(row: VerificationProviderInfo) {
  providerForm.id = row.id
  providerForm.provider_type = row.provider_type
  providerForm.name = row.name
  providerForm.endpoint = row.endpoint
  providerForm.priority = row.priority
  providerForm.status = row.status
  providerForm.is_default = row.is_default
  providerForm.remark = row.remark
  // 脱敏回显：原样保存即「未修改」，后端保留原密文。
  providerForm.credentials = { ...(row.credentials || {}) }
  providerDialogVisible.value = true
}

function handleProviderTypeChange(value: unknown) {
  const type = String(value || '')
  const matched = providerTypes.value.find((t) => t.type === type)
  if (matched && !providerForm.name) {
    providerForm.name = matched.name
  }
}

async function handleSaveProvider() {
  const valid = await providerFormRef.value?.validate()
  if (valid !== true) return
  providerSaving.value = true
  try {
    await upsertVerificationProvider({ ...providerForm })
    MessagePlugin.success('服务商配置已保存')
    providerDialogVisible.value = false
    await loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    providerSaving.value = false
  }
}

async function handleToggleProviderStatus(row: VerificationProviderInfo, enabled: boolean) {
  statusLoadingId.value = row.id
  try {
    await upsertVerificationProvider({
      id: row.id,
      provider_type: row.provider_type,
      name: row.name,
      endpoint: row.endpoint,
      priority: row.priority,
      status: enabled ? 1 : 0,
      is_default: row.is_default,
      remark: row.remark,
      credentials: {},
    })
    MessagePlugin.success(enabled ? '已启用' : '已停用')
    await loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '切换失败')
  } finally {
    statusLoadingId.value = 0
  }
}

async function handleTestProvider(row: VerificationProviderInfo) {
  testingId.value = row.id
  try {
    const result = await testVerificationProvider(row.id)
    if (result?.ok) {
      MessagePlugin.success(result.message || '连通性正常')
    } else {
      MessagePlugin.warning(result?.message || '连通测试未通过')
    }
    await loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '测试请求失败')
  } finally {
    testingId.value = 0
  }
}

function handleDeleteProvider(row: VerificationProviderInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除核验服务商',
    body: `确认删除「${row.name}」？删除后实名申请若指向该服务商会自动回落到人工审核。`,
    theme: 'warning',
    onConfirm: async () => {
      try {
        await deleteVerificationProvider(row.id)
        MessagePlugin.success('已删除')
        await loadProviders()
      } catch (error) {
        MessagePlugin.error((error as Error)?.message || '删除失败')
      } finally {
        dialog.destroy()
      }
    },
  })
}

function handleTabChange(value: unknown) {
  if (String(value) === 'providers' && !providers.value.length) {
    void loadProviders()
  }
}

onMounted(() => {
  void loadConfigs()
})
</script>

<style scoped>
.verification-config-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header,
.main-card {
  padding: 18px 20px;
}

.tab-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-top: 14px;
}

.tab-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.tab-toolbar__hint {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
  max-width: 720px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.form-grid--single {
  grid-template-columns: 1fr;
}

.form-hint {
  margin-bottom: 12px;
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
  font-weight: 600;
}

.cell-muted {
  color: var(--color-muted-foreground);
}

.error-text {
  color: var(--td-error-color, #d54941);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.field-help {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
