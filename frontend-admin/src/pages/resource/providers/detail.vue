<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CloudIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ detail?.name || '提供商详情' }}</h2>
          <p class="page-header__desc">
            <template v-if="detail">类型：{{ typeLabel(detail.provider_type) }} · {{ detail.provider_type }}</template>
            <template v-else>加载提供商信息…</template>
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" @click="router.push(listPath)">返回列表</t-button>
        <t-button v-if="detail?.ops_console_url" variant="outline" @click="openOpsConsole">运维平台</t-button>
        <t-button variant="outline" :loading="testing" @click="handleTestConnection">测试连接</t-button>
        <t-button v-if="detail?.sync_paused" theme="warning" :loading="resuming" @click="handleResumeSync">恢复同步</t-button>
        <t-button theme="primary" :loading="saving" @click="toggleEdit">{{ editing ? '保存' : '编辑资料' }}</t-button>
      </t-space>
    </header>

    <section class="tabs-card surface-card">
      <t-tabs v-model="activeTab" theme="card" size="medium">
        <t-tab-panel value="basic" label="基本信息" />
        <t-tab-panel value="capability" label="能力矩阵" />
        <t-tab-panel value="pools" label="资源池" />
        <t-tab-panel value="logs" label="同步日志" />
      </t-tabs>
    </section>

    <section class="content-card surface-card">
      <!-- 基本信息 -->
      <template v-if="activeTab === 'basic'">
        <t-loading :loading="loading" class="panel-loading">
          <template v-if="detail">
            <t-alert
              v-if="detail.credential_error"
              theme="error"
              :message="`凭证不可用：${detail.credential_error}`"
              class="credential-alert"
            />
            <t-form ref="formRef" :data="formData" :rules="formRules" label-align="top">
              <div class="form-grid">
                <t-form-item label="提供商名称" name="name">
                  <t-input v-if="editing" v-model="formData.name" maxlength="50" />
                  <span v-else class="readonly-text">{{ detail.name }}</span>
                </t-form-item>
                <t-form-item label="提供商类型">
                  <span class="readonly-text">{{ typeLabel(detail.provider_type) }}</span>
                </t-form-item>
                <t-form-item label="API 地址" name="api_endpoint">
                  <t-input v-if="editing" v-model="formData.api_endpoint" />
                  <span v-else class="readonly-text mono-text">{{ detail.api_endpoint }}</span>
                </t-form-item>
                <t-form-item label="区域">
                  <t-input v-if="editing" v-model="formData.region" placeholder="例如：cn-east-1" />
                  <span v-else class="readonly-text">{{ detail.region || '—' }}</span>
                </t-form-item>
                <t-form-item label="同步间隔（秒）">
                  <t-input-number v-if="editing" v-model="formData.sync_interval" :min="0" :step="60" />
                  <span v-else class="readonly-text">{{ detail.sync_interval }} 秒</span>
                </t-form-item>
                <t-form-item label="启用实例同步">
                  <t-switch v-if="editing" v-model="formData.sync_enabled" />
                  <span v-else class="readonly-text">{{ detail.sync_enabled ? '启用' : '禁用' }}</span>
                </t-form-item>
                <t-form-item label="状态">
                  <span class="readonly-text">
                    <t-tag :theme="detail.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
                      {{ statusLabelMap[detail.status] || '未知' }}
                    </t-tag>
                  </span>
                </t-form-item>
                <t-form-item label="运维平台">
                  <t-input v-if="editing" v-model="formData.ops_console_url" placeholder="上游/平台运维控制台地址，留空则不显示跳转入口" />
                  <span v-else class="readonly-text">
                    <t-link v-if="detail.ops_console_url" theme="primary" hover="color" @click="openOpsConsole">
                      {{ detail.ops_console_url }}
                    </t-link>
                    <span v-else>—</span>
                  </span>
                </t-form-item>
                <t-form-item label="同步健康">
                  <span class="readonly-text">
                    <t-tag v-if="detail.sync_paused" theme="warning" variant="light" size="small" shape="round">
                      {{ detail.last_sync_error === '适配器未实现' ? '未接入' : '已暂停' }}
                    </t-tag>
                    <t-tag v-else-if="detail.consecutive_failures > 0" theme="danger" variant="light" size="small" shape="round">
                      失败 {{ detail.consecutive_failures }} 次
                    </t-tag>
                    <t-tag v-else theme="success" variant="light" size="small" shape="round">正常</t-tag>
                    <span v-if="detail.last_sync_error" class="readonly-text sync-error">{{ detail.last_sync_error }}</span>
                  </span>
                </t-form-item>
                <t-form-item label="创建时间">
                  <span class="readonly-text">{{ formatTime(detail.created_at) }}</span>
                </t-form-item>
                <t-form-item label="更新时间">
                  <span class="readonly-text">{{ formatTime(detail.updated_at) }}</span>
                </t-form-item>
              </div>

              <!-- 传输参数（契约③）：超时/重试/限流，0 表示用默认 -->
              <div v-if="editing" class="advanced-box">
                <t-divider align="left">传输设置</t-divider>
                <div class="form-grid">
                  <t-form-item label="超时（秒）" name="timeout_seconds">
                    <t-input-number v-model="formData.timeout_seconds" :min="0" placeholder="0 表示默认" />
                  </t-form-item>
                  <t-form-item label="最大重试次数" name="retry_max">
                    <t-input-number v-model="formData.retry_max" :min="0" placeholder="0 表示默认" />
                  </t-form-item>
                  <t-form-item label="限流 QPS" name="rate_limit_qps">
                    <t-input-number v-model="formData.rate_limit_qps" :min="0" placeholder="0 表示不限" />
                  </t-form-item>
                </div>
              </div>
            </t-form>

            <!-- 凭证：只读态展示脱敏值；编辑态按描述符渲染动态字段 -->
            <div v-if="!editing" class="credential-view">
              <div class="form-section__title">凭证（脱敏）</div>
              <t-descriptions :column="2" bordered size="small">
                <t-descriptions-item v-for="field in credentialFields" :key="field.key" :label="field.label">
                  <span class="mono-text">{{ detail.credentials?.[field.key] || '—' }}</span>
                </t-descriptions-item>
                <t-descriptions-item v-if="!credentialFields.length" label="凭证">
                  <span class="readonly-text">该渠道类型未声明凭证字段</span>
                </t-descriptions-item>
              </t-descriptions>
            </div>
            <CredentialFormFields
              v-else-if="credentialFields.length"
              v-model="formData.credentials"
              :fields="credentialFields"
            />
            <div v-else-if="editing" class="form-grid credential-legacy">
              <t-form-item label="API 密钥">
                <t-input v-model="formData.api_key" type="password" placeholder="留空表示不修改" />
              </t-form-item>
              <t-form-item label="API 密码">
                <t-input v-model="formData.api_secret" type="password" placeholder="留空表示不修改" />
              </t-form-item>
            </div>

            <t-descriptions v-if="!editing" :column="3" bordered size="small" class="resource-summary">
              <t-descriptions-item label="CPU 配额">{{ detail.used_cpu }} / {{ detail.total_cpu }}</t-descriptions-item>
              <t-descriptions-item label="内存配额">{{ detail.used_memory }} / {{ detail.total_memory }}</t-descriptions-item>
              <t-descriptions-item label="磁盘配额">{{ detail.used_disk }} / {{ detail.total_disk }}</t-descriptions-item>
            </t-descriptions>
          </template>
          <t-empty v-else-if="!loading" description="未获取到提供商信息" />
        </t-loading>

        <t-alert
          v-if="editing"
          theme="info"
          :message="credentialFields.length
            ? '编辑时凭证字段留空或保持脱敏回显表示不修改；修改后将重新加密落库。'
            : '编辑时 API 密钥/密码留空表示保持原值不变。'"
          class="edit-alert"
        />
      </template>

      <!-- 能力矩阵 -->
      <template v-else-if="activeTab === 'capability'">
        <CapabilityMatrix v-if="detail" :descriptor="detail.capabilities" :title="`${detail.name} 能力矩阵`" />
        <t-empty v-else description="加载能力矩阵…" />
      </template>

      <!-- 资源池 -->
      <template v-else-if="activeTab === 'pools'">
        <PoolPanel :provider-id="providerId" />
      </template>

      <!-- 同步日志 -->
      <template v-else-if="activeTab === 'logs'">
        <t-empty description="同步日志将在阶段四实现" />
        <p class="logs-hint">当前阶段为同步日志占位，功能将在资源同步模块中提供。</p>
      </template>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { CloudIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule } from 'tdesign-vue-next'

import { getProviderDetail, getProviderTypes, resumeProviderSync, testConnection, updateProvider } from '@/api/admin'
import type { ProviderField, ProviderInfo, ProviderTypeItem } from '@/types/interface'
import PoolPanel from '@/pages/resource/pools/index.vue'
import CapabilityMatrix from './components/CapabilityMatrix.vue'
import CredentialFormFields from './components/CredentialFormFields.vue'

defineOptions({ name: 'ResourceProvidersDetail' })

const route = useRoute()
const router = useRouter()

const providerId = computed(() => Number(route.params.id))
// 详情页归属哪个渠道列表：按渠道链路回跳（自营平台 / 上游转售），避免跨页跳错。
const listPath = computed(() => (detail.value?.kind === 'compute' ? '/resource/platforms' : '/resource/providers'))
const detail = ref<ProviderInfo | null>(null)
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const resuming = ref(false)
const editing = ref(false)
const activeTab = ref('basic')
const typeList = ref<ProviderTypeItem[]>([])
const formRef = ref<FormInstanceFunctions | null>(null)

// 凭证字段由渠道能力描述符驱动（落库的脱敏值据此回显）。
const credentialFields = computed<ProviderField[]>(() => detail.value?.capabilities?.credential_schema || [])

const statusLabelMap: Record<number, string> = {
  1: '启用',
  0: '禁用',
}

const formData = reactive({
  name: '',
  api_endpoint: '',
  region: '',
  api_key: '',
  api_secret: '',
  credentials: {} as Record<string, string>,
  sync_interval: 3600,
  sync_enabled: false,
  status: 1,
  timeout_seconds: 0,
  retry_max: 0,
  rate_limit_qps: 0,
  ops_console_url: '',
})

const formRules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入提供商名称', type: 'error', trigger: 'blur' }],
  api_endpoint: [
    { required: true, message: '请输入 API 地址', type: 'error', trigger: 'blur' },
    { pattern: /^https?:\/\//, message: 'API 地址需以 http(s):// 开头', type: 'error', trigger: 'blur' },
  ],
}

function typeLabel(type: string): string {
  return typeList.value.find((item) => item.type === type)?.name || type
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

async function loadDetail() {
  loading.value = true
  try {
    detail.value = await getProviderDetail(providerId.value)
    Object.assign(formData, {
      name: detail.value.name,
      api_endpoint: detail.value.api_endpoint,
      region: detail.value.region,
      api_key: '',
      api_secret: '',
      credentials: { ...(detail.value.credentials || {}) },
      sync_interval: detail.value.sync_interval,
      sync_enabled: detail.value.sync_enabled,
      status: detail.value.status,
      timeout_seconds: detail.value.timeout_seconds,
      retry_max: detail.value.retry_max,
      rate_limit_qps: detail.value.rate_limit_qps,
      ops_console_url: detail.value.ops_console_url || '',
    })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商详情失败')
    router.replace('/resource/providers')
  } finally {
    loading.value = false
  }
}

async function loadTypes() {
  try {
    typeList.value = await getProviderTypes()
  } catch {
    /* 类型名映射失败不阻塞详情展示 */
  }
}

function toggleEdit() {
  if (!editing.value) {
    editing.value = true
    return
  }
  handleSave()
}

// 运维平台为外部系统，新窗口打开（不接管站内路由）。
function openOpsConsole() {
  const url = detail.value?.ops_console_url
  if (url) window.open(url, '_blank', 'noopener')
}

async function handleSave() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  saving.value = true
  try {
    const payload = {
      name: formData.name,
      api_endpoint: formData.api_endpoint,
      region: formData.region,
      sync_enabled: formData.sync_enabled,
      sync_interval: formData.sync_interval,
      status: formData.status,
      timeout_seconds: formData.timeout_seconds,
      retry_max: formData.retry_max,
      rate_limit_qps: formData.rate_limit_qps,
      ops_console_url: formData.ops_console_url || '',
    } as Parameters<typeof updateProvider>[1]
    if (credentialFields.value.length) {
      payload.credentials = formData.credentials
    } else {
      payload.api_key = formData.api_key || undefined
      payload.api_secret = formData.api_secret || undefined
    }
    const updated = await updateProvider(providerId.value, payload)
    detail.value = updated
    formData.credentials = { ...(updated.credentials || {}) }
    formData.api_key = ''
    formData.api_secret = ''
    editing.value = false
    MessagePlugin.success('提供商资料已保存')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存提供商资料失败')
  } finally {
    saving.value = false
  }
}

async function handleTestConnection() {
  testing.value = true
  try {
    const result = await testConnection(providerId.value)
    if (result.success) {
      MessagePlugin.success(`连接测试通过：${result.message}`)
    } else {
      MessagePlugin.error(`连接测试失败：${result.message}`)
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '连接测试失败')
  } finally {
    testing.value = false
  }
}

async function handleResumeSync() {
  resuming.value = true
  try {
    await resumeProviderSync(providerId.value)
    MessagePlugin.success('已恢复同步，下次调度将重新尝试')
    await loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复同步失败')
  } finally {
    resuming.value = false
  }
}

onMounted(() => {
  loadTypes()
  loadDetail()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, var(--color-primary), var(--td-brand-color-8));
}

.tabs-card,
.content-card {
  padding: var(--space-lg) 24px;
}

.panel-loading {
  min-height: 160px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.sync-error {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  word-break: break-all;
  color: var(--color-muted-foreground);
}

.readonly-text {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  color: var(--color-foreground);
  line-height: 1.6;
}

.mono-text {
  font-family: var(--hs-font-mono);
  font-size: 13px;
  word-break: break-all;
}

.resource-summary {
  margin-top: var(--space-lg);
}

.edit-alert {
  margin-top: var(--space-lg);
}

.credential-alert {
  margin-bottom: var(--space-lg);
}

.credential-view {
  margin-top: var(--space-lg);
}

.credential-view .form-section__title,
.form-section__title {
  margin: 0 0 var(--space-sm);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.credential-legacy {
  margin-top: var(--space-md);
}

.advanced-box {
  margin-top: var(--space-md);
  border: 1px dashed var(--color-border);
  border-radius: var(--hs-radius-lg);
  padding: var(--space-md) var(--space-lg) 0;
}

.logs-hint {
  margin: var(--space-md) 0 0;
  text-align: center;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

@media (max-width: 1200px) {
  .form-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
