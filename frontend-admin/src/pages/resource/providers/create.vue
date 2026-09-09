<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AddIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">添加提供商</h2>
        </div>
      </div>
      <t-button variant="outline" @click="router.push('/resource/providers')">返回列表</t-button>
    </header>

    <section class="steps-card surface-card">
      <t-steps :current="current" layout="horizontal">
        <t-step title="选择类型" content="选择上游云厂商" />
        <t-step title="填写 API 信息" content="配置端点与密钥" />
        <t-step title="完成" content="创建并测试连接" />
      </t-steps>
    </section>

    <!-- Step 1: 选择类型 -->
    <section v-if="current === 0" class="panel-card surface-card">
      <header class="panel-card__head">
        <div>
          <h3 class="card-title">选择提供商类型</h3>
          <p class="card-subtitle">请选择待接入的上游云厂商类型，类型决定了适配器与字段。</p>
        </div>
      </header>
      <div v-loading="loadingTypes" class="type-grid">
        <div
          v-for="item in typeOptions"
          :key="item.value"
          class="type-card"
          :class="{ 'type-card--active': formData.provider_type === item.value }"
          @click="formData.provider_type = item.value"
        >
          <div class="type-card__icon">
            <CloudIcon size="24" aria-hidden="true" />
          </div>
          <div class="type-card__name">{{ item.label }}</div>
          <div class="type-card__code">{{ item.value }}</div>
          <div class="type-card__radio">
            <t-icon v-if="formData.provider_type === item.value" name="check-circle-filled" />
          </div>
        </div>
      </div>
      <t-alert v-if="!typeOptions.length && !loadingTypes" theme="warning" :message="`暂无可用的提供商类型，请联系后端确认适配器已注册`" />
      <div class="steps-footer">
        <t-button theme="primary" :disabled="!formData.provider_type" @click="handleNext">下一步</t-button>
      </div>
    </section>

    <!-- Step 2: 填写 API 信息 -->
    <section v-else-if="current === 1" class="panel-card surface-card">
      <header class="panel-card__head">
        <div>
          <h3 class="card-title">填写 API 信息</h3>
          <p class="card-subtitle">端点为可访问的上游 API 地址，密钥加密落库，提交后以脱敏形式展示。</p>
        </div>
      </header>
      <t-alert v-if="providerHint" theme="info" :message="providerHint" class="provider-hint" />
      <t-form ref="formRef" :data="formData" :rules="formRules" label-align="top" class="provider-form">
        <!-- 魔方财务：createApi 契约表单 -->
        <template v-if="isMofangFinance">
          <div class="form-section__title">基本信息</div>
          <div class="form-grid">
            <t-form-item label="名称" name="name">
              <t-input v-model="formData.name" placeholder="请输入名称" maxlength="100" />
            </t-form-item>
            <t-form-item label="联系方式" name="contact_way">
              <t-input v-model="formData.contact_way" placeholder="请输入联系方式" maxlength="100" />
            </t-form-item>
            <t-form-item label="备注" name="des" class="form-item--full">
              <t-textarea v-model="formData.des" placeholder="请输入备注" :autosize="{ minRows: 2, maxRows: 4 }" maxlength="255" />
            </t-form-item>
          </div>
          <div class="form-section__title">自动化设置</div>
          <div class="form-grid">
            <t-form-item label="接口类型" name="upstream_type">
              <t-select v-model="formData.upstream_type" :options="financeTypeOptions" />
            </t-form-item>
            <t-form-item label="接口地址" name="api_endpoint">
              <t-input v-model="formData.api_endpoint" placeholder="请输入接口地址" />
            </t-form-item>
            <t-form-item label="用户名" name="api_key">
              <t-input v-model="formData.api_key" placeholder="请输入用户名" maxlength="100" />
            </t-form-item>
            <t-form-item label="API密钥" name="api_secret">
              <t-input v-model="formData.api_secret" type="password" placeholder="请输入密码" maxlength="255" />
            </t-form-item>
          </div>
        </template>

        <!-- 魔方云：新增接口(添加服务器 /admin/dcimcloud/server) 契约表单 -->
        <template v-else-if="isMofangYun">
          <div class="form-grid">
            <t-form-item label="名称" name="name">
              <t-input v-model="formData.name" placeholder="请输入名称" maxlength="100" />
            </t-form-item>
            <t-form-item label="地址" name="api_endpoint">
              <t-input v-model="formData.api_endpoint" placeholder="请输入Ip地址或域名" />
            </t-form-item>
          </div>
          <p class="form-tip">填写魔方云的域名或ip+端口+后台管理员路径 如：192.168.10.1:443/zjmf123</p>
          <div class="form-grid">
            <t-form-item label="账号" name="api_key">
              <t-input v-model="formData.api_key" placeholder="请输入用户名" maxlength="100" />
            </t-form-item>
            <t-form-item label="密码" name="api_secret">
              <t-input v-model="formData.api_secret" type="password" placeholder="请输入密码" maxlength="255" />
            </t-form-item>
            <t-form-item class="form-item--half" label="是否https" name="secure">
              <t-switch v-model="formData.secure" />
            </t-form-item>
            <t-form-item class="form-item--half" label="是否启用" name="mofangyun_enabled">
              <t-switch v-model="mofangYunEnabled" />
            </t-form-item>
          </div>
          <div class="advanced-box">
            <t-divider align="left">高级设置</t-divider>
            <div class="form-grid">
              <t-form-item label="端口" name="port">
                <t-input v-model="formData.port" placeholder="例如 443" maxlength="20" />
              </t-form-item>
              <t-form-item label="财务标识" name="user_prefix">
                <t-input v-model="formData.user_prefix" placeholder="请输入财务标识" maxlength="50" />
              </t-form-item>
              <t-form-item label="账号类型" name="account_type">
                <t-select v-model="formData.account_type" :options="accountTypeOptions" placeholder="请选择账号类型" />
              </t-form-item>
            </div>
          </div>
        </template>

        <!-- 其它类型：通用表单 -->
        <template v-else>
          <div class="form-grid">
            <t-form-item label="提供商名称" name="name">
              <t-input v-model="formData.name" placeholder="例如：华东魔方云" maxlength="50" />
            </t-form-item>
            <t-form-item label="提供商类型" name="provider_type">
              <t-input :model-value="selectedTypeLabel" disabled />
            </t-form-item>
            <t-form-item label="接口地址" name="api_endpoint">
              <t-input v-model="formData.api_endpoint" :placeholder="endpointPlaceholder" />
            </t-form-item>
            <t-form-item label="区域" name="region">
              <t-input v-model="formData.region" :placeholder="regionPlaceholder" />
            </t-form-item>
            <t-form-item :label="credentialLabels.key" name="api_key">
              <t-input v-model="formData.api_key" type="password" :placeholder="credentialLabels.keyPlaceholder" />
            </t-form-item>
            <t-form-item :label="credentialLabels.secret" name="api_secret">
              <t-input v-model="formData.api_secret" type="password" :placeholder="credentialLabels.secretPlaceholder" />
            </t-form-item>
            <t-form-item label="同步间隔（秒）" name="sync_interval">
              <t-input-number v-model="formData.sync_interval" :min="0" :step="60" placeholder="默认 3600" />
            </t-form-item>
            <t-form-item label="启用实例同步" name="sync_enabled">
              <t-switch v-model="formData.sync_enabled" />
            </t-form-item>
          </div>
        </template>
      </t-form>
      <div class="steps-footer">
        <t-button variant="outline" @click="current = 0">上一步</t-button>
        <t-space size="small">
          <t-button variant="outline" :loading="submitting" @click="handleSubmit">提交</t-button>
          <t-button theme="primary" :loading="submitting" @click="handleSubmitAndTest">提交并测试连接</t-button>
        </t-space>
      </div>
    </section>

    <!-- Step 3: 完成 -->
    <section v-else class="panel-card surface-card">
      <div class="done-box">
        <div class="done-box__icon">
          <CheckCircleIcon size="40" aria-hidden="true" />
        </div>
        <h3 class="done-box__title">提供商已添加成功</h3>
        <t-loading v-if="submitting" text="正在创建并测试连接，请稍候…" />
        <t-descriptions v-else :column="2" bordered size="small" class="done-desc">
          <t-descriptions-item label="名称">{{ created?.name }}</t-descriptions-item>
          <t-descriptions-item label="类型">{{ selectedTypeLabel }}</t-descriptions-item>
          <t-descriptions-item label="API 地址">{{ created?.api_endpoint }}</t-descriptions-item>
          <t-descriptions-item v-if="isMofangFinance" label="上游 API ID">{{ created?.zjmf_finance_api_id }}</t-descriptions-item>
          <t-descriptions-item label="实例同步">{{ created?.sync_enabled ? '启用' : '禁用' }}</t-descriptions-item>
          <t-descriptions-item label="连接">{{ testResultText }}</t-descriptions-item>
        </t-descriptions>
        <div class="steps-footer">
          <t-button theme="primary" @click="router.push(created ? `/resource/providers/${created.id}` : '/resource/providers')">查看详情</t-button>
          <t-button variant="outline" @click="router.push('/resource/providers')">返回列表</t-button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AddIcon, CheckCircleIcon, CloudIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule } from 'tdesign-vue-next'

import { createProvider, getProviderTypes, testConnection } from '@/api/admin'
import type { ProviderCreateRequest, ProviderInfo, ProviderTypeItem } from '@/types/interface'

defineOptions({ name: 'ResourceProvidersCreate' })

const router = useRouter()

const current = ref(0)
const loadingTypes = ref(false)
const submitting = ref(false)
const created = ref<ProviderInfo | null>(null)
const testResultText = ref('—')
const typeOptions = ref<{ label: string; value: string }[]>([])
const formRef = ref<FormInstanceFunctions | null>(null)

const formData = reactive<ProviderCreateRequest>({
  name: '',
  provider_type: '',
  api_endpoint: '',
  api_key: '',
  api_secret: '',
  region: '',
  status: 1,
  sync_enabled: false,
  sync_interval: 3600,
  contact_way: '',
  des: '',
  upstream_type: 'zjmf_api',
  port: '',
  secure: false,
  disabled: false,
  user_prefix: '',
  account_type: '',
})

const financeTypeOptions = [
  { label: '智简魔方', value: 'zjmf_api' },
  { label: '手动', value: 'manual' },
  { label: 'V10', value: 'v10' },
]

const accountTypeOptions = [
  { label: '管理员', value: 'admin' },
  { label: '代理商', value: 'agent' },
]

const selectedTypeLabel = computed(() => {
  const item = typeOptions.value.find((type) => type.value === formData.provider_type)
  return item?.label || formData.provider_type
})

const isMofangFinance = computed(() => formData.provider_type === 'mofangfinance')
const isMofangYun = computed(() => formData.provider_type === 'mofangyun')
// 魔方云契约 disabled=是否禁用(0启用 1禁用)，开关以「是否启用」呈现，提交时取反。
const mofangYunEnabled = ref(true)

// 各提供商类型的动态字段配置：根据上游语义定制标签、占位符与提示
interface FieldLabels {
  key: string
  keyPlaceholder: string
  secret: string
  secretPlaceholder: string
  endpointPlaceholder: string
  regionPlaceholder: string
  hint: string
}

const FIELD_CONFIG: Record<string, Partial<FieldLabels>> = {
  mofangyun: {
    key: 'API 账号',
    keyPlaceholder: 'API 账户的账号（用户名）',
    secret: 'API 密码',
    secretPlaceholder: 'API 账户的密码',
    endpointPlaceholder: '例如：https://y.host.youcloude.com',
    regionPlaceholder: '区域 ID，可留空（如 cn-east）',
    hint: '魔方云：填写面板地址 + API 账户「账号 / 密码」，用于调用 index.php?m=api 接口',
  },
  mofangfinance: {
    key: 'API 密钥',
    keyPlaceholder: 'API 应用密钥（api_key）',
    secret: 'API 密钥',
    secretPlaceholder: 'API 应用密钥（api_secret）',
    endpointPlaceholder: '例如：https://finance.example.com',
    regionPlaceholder: '区域 ID，可留空',
    hint: '魔方财务：填写 API 地址 + 应用「密钥 / 密钥（api_key / api_secret）」，用于调用 createApi 等接口',
  },
}

const typedConfig = computed<Partial<FieldLabels>>(() => FIELD_CONFIG[formData.provider_type] || {})

const providerHint = computed(() => typedConfig.value.hint || '')
const credentialLabels = computed<FieldLabels>(() => ({
  key: typedConfig.value.key || 'API 密钥',
  keyPlaceholder: typedConfig.value.keyPlaceholder || '请输入 API 密钥',
  secret: typedConfig.value.secret || 'API 密码',
  secretPlaceholder: typedConfig.value.secretPlaceholder || '请输入 API 密码',
  endpointPlaceholder: typedConfig.value.endpointPlaceholder || '请输入 API 地址',
  regionPlaceholder: typedConfig.value.regionPlaceholder || '请输入区域 ID',
  hint: typedConfig.value.hint || '',
}))
const endpointPlaceholder = computed(() => credentialLabels.value.endpointPlaceholder)
const regionPlaceholder = computed(() => credentialLabels.value.regionPlaceholder)

const formRules = computed<Record<string, FormRule[]>>(() => {
  const base: Record<string, FormRule[]> = {
    name: [{ required: true, message: '请输入提供商名称', type: 'error', trigger: 'blur' }],
    provider_type: [{ required: true, message: '请选择提供商类型', type: 'error', trigger: 'change' }],
    api_endpoint: [{ required: true, message: '请输入 API 地址', type: 'error', trigger: 'blur' }],
    api_key: [{ required: true, message: '请输入用户名/密钥', type: 'error', trigger: 'blur' }],
    api_secret: [{ required: true, message: '请输入密码/密钥', type: 'error', trigger: 'blur' }],
  }
  // 魔方云地址为「Ip地址或域名」而非 http 链接，不校验协议前缀。
  if (!isMofangYun.value) {
    base.api_endpoint.push({ pattern: /^https?:\/\//, message: 'API 地址需以 http(s):// 开头', type: 'error', trigger: 'blur' })
  }
  if (isMofangFinance.value) {
    base.upstream_type = [{ required: true, message: '请选择接口类型', type: 'error', trigger: 'change' }]
  }
  return base
})

async function loadTypes() {
  loadingTypes.value = true
  try {
    const types = await getProviderTypes()
    typeOptions.value = types.map((item: ProviderTypeItem) => ({ label: item.name, value: item.type }))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商类型失败')
  } finally {
    loadingTypes.value = false
  }
}

function handleNext() {
  if (!formData.provider_type) {
    MessagePlugin.warning('请先选择提供商类型')
    return
  }
  current.value = 1
}

async function handleSubmit() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  submitting.value = true
  try {
    created.value = await createProvider({
      name: formData.name,
      provider_type: formData.provider_type,
      api_endpoint: formData.api_endpoint,
      api_key: formData.api_key || undefined,
      api_secret: formData.api_secret || undefined,
      region: formData.region,
      status: 1,
      sync_enabled: formData.sync_enabled,
      sync_interval: formData.sync_interval,
      contact_way: formData.contact_way || undefined,
      des: formData.des || undefined,
      upstream_type: formData.upstream_type || undefined,
      port: formData.port || undefined,
      secure: formData.secure,
      disabled: !mofangYunEnabled.value,
      user_prefix: formData.user_prefix || undefined,
      account_type: formData.account_type || undefined,
    })
    testResultText.value = '未测试'
    current.value = 2
    MessagePlugin.success('提供商已创建')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建提供商失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmitAndTest() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  submitting.value = true
  try {
    created.value = await createProvider({
      name: formData.name,
      provider_type: formData.provider_type,
      api_endpoint: formData.api_endpoint,
      api_key: formData.api_key || undefined,
      api_secret: formData.api_secret || undefined,
      region: formData.region,
      status: 1,
      sync_enabled: formData.sync_enabled,
      sync_interval: formData.sync_interval,
      contact_way: formData.contact_way || undefined,
      des: formData.des || undefined,
      upstream_type: formData.upstream_type || undefined,
      port: formData.port || undefined,
      secure: formData.secure,
      disabled: !mofangYunEnabled.value,
      user_prefix: formData.user_prefix || undefined,
      account_type: formData.account_type || undefined,
    })
    const result = await testConnection(created.value.id)
    testResultText.value = result.success ? `测试通过（${result.message}）` : `测试失败：${result.message}`
    current.value = 2
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建或测试连接失败')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadTypes()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, #16a34a, #15803d);
  --chip-shadow: 0 4px 10px rgba(22, 163, 74, 0.25);
}

.steps-card,
.panel-card {
  padding: var(--space-lg) 24px;
}

.card-subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.panel-card__head {
  margin-bottom: var(--space-lg);
}

.type-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 14px;
}

.type-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 20px 16px;
  border-radius: var(--hs-radius-lg);
  border: 1px solid var(--color-border);
  background: var(--hs-surface-2);
  cursor: pointer;
  transition:
    border-color var(--hs-duration-fast),
    box-shadow var(--hs-duration-fast),
    transform var(--hs-duration-fast);
}

.type-card:hover {
  box-shadow: var(--hs-shadow-sm);
  transform: translateY(-2px);
}

.type-card--active {
  border-color: #16a34a;
  box-shadow: 0 0 0 2px rgba(22, 163, 74, 0.18);
}

.type-card__icon {
  width: 44px;
  height: 44px;
  border-radius: var(--hs-radius-xl);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--hs-surface-3);
  color: #16a34a;
}

.type-card__name {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.type-card__code {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.type-card__radio {
  position: absolute;
  top: 10px;
  right: 10px;
  color: #16a34a;
  display: inline-flex;
}

.steps-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-md);
  margin-top: var(--space-lg);
}

.provider-form {
  max-width: 720px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.form-tip {
  margin: 4px 0 var(--space-md);
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.form-item--full {
  grid-column: 1 / -1;
}

.form-item--half {
  min-width: 0;
}

.advanced-box {
  margin-top: var(--space-md);
  border: 1px dashed var(--color-border);
  border-radius: var(--hs-radius-lg);
  padding: var(--space-md) var(--space-lg) 0;
}

.done-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-xl) 0;
  text-align: center;
}

.done-box__icon {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(22, 163, 74, 0.12);
  color: #16a34a;
}

.done-box__title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--color-foreground);
}

.done-desc {
  max-width: 560px;
  width: 100%;
  text-align: left;
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
