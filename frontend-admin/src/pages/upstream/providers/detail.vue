<template>
  <div class="page-body">
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
        <t-button variant="outline" @click="router.push('/upstream/providers')">返回列表</t-button>
        <t-button variant="outline" :loading="testing" @click="handleTestConnection">测试连接</t-button>
        <t-button theme="primary" :loading="saving" @click="toggleEdit">{{ editing ? '保存' : '编辑资料' }}</t-button>
      </t-space>
    </header>

    <section class="tabs-card surface-card">
      <t-tabs v-model="activeTab" theme="card" size="medium">
        <t-tab-panel value="basic" label="基本信息" />
        <t-tab-panel value="pools" label="资源池" />
        <t-tab-panel value="logs" label="同步日志" />
      </t-tabs>
    </section>

    <section class="content-card surface-card">
      <!-- 基本信息 -->
      <template v-if="activeTab === 'basic'">
        <t-loading :loading="loading" class="panel-loading">
          <template v-if="detail">
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
                <t-form-item label="API 密钥">
                  <t-input v-if="editing" v-model="formData.api_key" type="password" placeholder="留空表示不修改" />
                  <span v-else class="readonly-text mono-text">{{ detail.api_key || '—' }}</span>
                </t-form-item>
                <t-form-item label="API 密码">
                  <t-input v-if="editing" v-model="formData.api_secret" type="password" placeholder="留空表示不修改" />
                  <span v-else class="readonly-text mono-text">{{ detail.api_secret || '—' }}</span>
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
                <t-form-item label="创建时间">
                  <span class="readonly-text">{{ formatTime(detail.created_at) }}</span>
                </t-form-item>
                <t-form-item label="更新时间">
                  <span class="readonly-text">{{ formatTime(detail.updated_at) }}</span>
                </t-form-item>
              </div>
            </t-form>

            <t-descriptions v-if="!editing" :column="3" bordered size="small" class="resource-summary">
              <t-descriptions-item label="CPU 配额">{{ detail.used_cpu }} / {{ detail.total_cpu }}</t-descriptions-item>
              <t-descriptions-item label="内存配额">{{ detail.used_memory }} / {{ detail.total_memory }}</t-descriptions-item>
              <t-descriptions-item label="磁盘配额">{{ detail.used_disk }} / {{ detail.total_disk }}</t-descriptions-item>
            </t-descriptions>
          </template>
          <t-empty v-else-if="!loading" description="未获取到提供商信息" />
        </t-loading>

        <t-alert v-if="editing" theme="info" message="编辑时 API 密钥/密码留空表示保持原值不变。" class="edit-alert" />
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

import { getProviderDetail, getProviderTypes, testConnection, updateProvider } from '@/api/admin'
import type { ProviderInfo, ProviderTypeItem } from '@/types/interface'
import PoolPanel from '@/pages/upstream/pools/index.vue'

defineOptions({ name: 'UpstreamProvidersDetail' })

const route = useRoute()
const router = useRouter()

const providerId = computed(() => Number(route.params.id))
const detail = ref<ProviderInfo | null>(null)
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const editing = ref(false)
const activeTab = ref('basic')
const typeNameMap = ref<Record<string, string>>({})
const formRef = ref<FormInstanceFunctions | null>(null)

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
  sync_interval: 3600,
  sync_enabled: false,
  status: 1,
})

const formRules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入提供商名称', type: 'error', trigger: 'blur' }],
  api_endpoint: [
    { required: true, message: '请输入 API 地址', type: 'error', trigger: 'blur' },
    { pattern: /^https?:\/\//, message: 'API 地址需以 http(s):// 开头', type: 'error', trigger: 'blur' },
  ],
}

function typeLabel(type: string): string {
  return typeNameMap.value[type] || type
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
      sync_interval: detail.value.sync_interval,
      sync_enabled: detail.value.sync_enabled,
      status: detail.value.status,
    })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商详情失败')
    router.replace('/upstream/providers')
  } finally {
    loading.value = false
  }
}

async function loadTypes() {
  try {
    const types = await getProviderTypes()
    typeNameMap.value = Object.fromEntries(types.map((item: ProviderTypeItem) => [item.type, item.name]))
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

async function handleSave() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  saving.value = true
  try {
    const updated = await updateProvider(providerId.value, {
      name: formData.name,
      api_endpoint: formData.api_endpoint,
      api_key: formData.api_key || undefined,
      api_secret: formData.api_secret || undefined,
      region: formData.region,
      sync_enabled: formData.sync_enabled,
      sync_interval: formData.sync_interval,
      status: formData.status,
    })
    detail.value = updated
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

onMounted(() => {
  loadTypes()
  loadDetail()
})
</script>

<style scoped lang="css">
.page-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

.surface-card {
  position: relative;
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  border: 1px solid var(--color-border);
  box-shadow: var(--hs-shadow-xs);
  transition:
    border-color var(--hs-duration-fast),
    box-shadow var(--hs-duration-fast);
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-lg);
  padding: var(--space-lg) var(--space-xl);
  flex-wrap: wrap;
}

.page-header__main {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  min-width: 0;
}

.page-header__chip {
  width: 44px;
  height: 44px;
  border-radius: var(--hs-radius-xl);
  background: linear-gradient(135deg, #16a34a, #15803d);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 10px rgba(22, 163, 74, 0.25);
}

.page-header__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--color-foreground);
}

.page-header__desc {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
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
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
