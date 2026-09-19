<template>
  <div class="page-body system-page oauth-page">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <LinkIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">第三方登录</h2>
          <p class="page-header__desc">
            微信 / QQ / 支付宝的渠道凭证与启用开关。凭证以字段级加密落库，回显为掩码，原样保存表示不修改。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadProviders">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <t-alert theme="info" class="page-alert">
      <template #message>回调地址需在三方开放平台登记</template>
      <template #description>
        每张卡片里的「回调地址」由后端按当前部署域名拼装。请把它逐字复制到对应开放平台的重定向地址白名单，
        否则用户会在三方页面上看到「redirect_uri 不合法」。
      </template>
    </t-alert>

    <div v-if="loading && !providers.length" class="loading-card surface-card">
      <t-loading text="正在加载渠道配置…" />
    </div>

    <div v-else-if="!providers.length" class="surface-card empty-card">
      <t-empty description="后端未注册任何第三方登录渠道" />
    </div>

    <div v-else class="provider-grid">
      <section v-for="item in providers" :key="item.provider" class="provider-card surface-card">
        <header class="provider-card__head">
          <div class="provider-card__title">
            <span class="provider-card__icon">{{ providerInitial(item) }}</span>
            <div class="provider-card__names">
              <span class="cell-strong">{{ item.name || item.provider }}</span>
              <span class="price-sub">{{ item.provider }} · {{ modeLabel(item.mode) }}</span>
            </div>
          </div>
          <div class="provider-card__switch">
            <t-switch
              :value="item.enabled"
              :disabled="!canManage || !item.supported"
              :loading="savingProvider === item.provider"
              @change="(v: unknown) => handleToggleEnabled(item, Boolean(v))"
            />
            <span class="price-sub">{{ item.enabled ? '已启用' : '未启用' }}</span>
          </div>
        </header>

        <t-alert
          v-if="!item.supported"
          theme="warning"
          class="provider-card__alert"
          message="该渠道适配器待接入"
          description="可先保存凭证，但启用按钮不可用：没有适配器的渠道启用了也登录不了。"
        />

        <div class="provider-card__row">
          <span class="provider-card__label">回调地址</span>
          <div class="provider-card__callback">
            <code class="callback-text">{{ item.callback_url || '—' }}</code>
            <t-link v-if="item.callback_url" theme="primary" hover="color" @click="copyText(item.callback_url)">
              复制
            </t-link>
          </div>
        </div>

        <div class="provider-card__row">
          <span class="provider-card__label">健康状态</span>
          <div class="price-cell">
            <t-tag :theme="healthTheme(item.health_status)" variant="light" size="small" shape="round">
              {{ healthLabel(item.health_status) }}
            </t-tag>
            <span v-if="item.last_error" class="price-sub error-text" :title="item.last_error">
              {{ item.last_error }}
            </span>
          </div>
        </div>

        <div class="provider-card__fields">
          <CredentialFields
            v-model="credentialDrafts[item.provider]"
            :fields="schemaOf(item)"
            :title="'凭证配置'"
            :empty-hint="'该渠道未声明凭证字段。'"
          />
        </div>

        <footer class="provider-card__foot">
          <t-button
            v-if="canManage"
            theme="primary"
            size="small"
            :loading="savingProvider === item.provider"
            @click="handleSave(item)"
          >
            保存
          </t-button>
          <t-button
            variant="outline"
            size="small"
            :loading="testingProvider === item.provider"
            :disabled="!item.supported"
            @click="handleTest(item)"
          >
            连通测试
          </t-button>
          <span v-if="item.last_check_at" class="price-sub">上次测试 {{ formatTime(item.last_check_at) }}</span>
        </footer>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
// 第三方登录渠道配置（doc104 §6.7）。
//
// 形态对齐「支付渠道」与「验证码配置」：三家渠道各一张卡，凭证字段由后端描述符
// 驱动动态渲染（@/components/credential-fields），因此后端加字段不需要改本页。
//
// 为什么不做「新建渠道」：渠道类型是固定的三家，先建行再配置会让运营多一步
// 无意义操作，且容易建出重复行。后端按 provider 名 upsert，没有行时按描述符创建。
import { computed, onMounted, reactive, ref } from 'vue'

import { LinkIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  getOAuthProviders,
  getOAuthProviderTypes,
  testOAuthProvider,
  updateOAuthProvider,
  type OAuthProviderInfo,
  type OAuthProviderType,
} from '@/api/oauth'
import CredentialFields from '@/components/credential-fields/index.vue'
import { useUserStore } from '@/store'

defineOptions({ name: 'SystemOAuth' })

const userStore = useUserStore()
const canManage = computed(() => userStore.permissions?.includes('oauth:config') || userStore.isSuperAdmin)

const loading = ref(false)
const providers = ref<OAuthProviderInfo[]>([])
const types = ref<OAuthProviderType[]>([])
// 每张卡一份草稿：直接改 providers 里的 credentials 会让「保存失败后无法回滚」。
const credentialDrafts = reactive<Record<string, Record<string, string>>>({})
const savingProvider = ref('')
const testingProvider = ref('')

function schemaOf(item: OAuthProviderInfo) {
  return types.value.find((t) => t.type === item.provider)?.credential_schema || null
}

function providerInitial(item: OAuthProviderInfo) {
  return (item.name || item.provider || '?').slice(0, 1)
}

function modeLabel(mode: string) {
  const map: Record<string, string> = { api: '授权码模式', qr: '扫码模式' }
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
  loading.value = true
  try {
    const [list, typeList] = await Promise.all([getOAuthProviders(), getOAuthProviderTypes()])
    providers.value = list || []
    types.value = typeList || []
    for (const item of providers.value) {
      credentialDrafts[item.provider] = { ...(item.credentials || {}) }
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载第三方登录配置失败')
  } finally {
    loading.value = false
  }
}

async function handleSave(item: OAuthProviderInfo) {
  savingProvider.value = item.provider
  try {
    // 只提交草稿：脱敏值原样回传即「未修改」，后端会保留原密文。
    const updated = await updateOAuthProvider(item.provider, {
      enabled: item.enabled,
      scopes: item.scopes,
      remark: item.remark,
      credentials: credentialDrafts[item.provider] || {},
    })
    const index = providers.value.findIndex((p) => p.provider === item.provider)
    if (index >= 0) providers.value[index] = updated
    credentialDrafts[item.provider] = { ...(updated.credentials || {}) }
    MessagePlugin.success(`${updated.name || updated.provider} 配置已保存`)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    savingProvider.value = ''
  }
}

// 启用开关单独保存：切换后立即落库，避免运营以为「切了就生效」却忘了点保存。
async function handleToggleEnabled(item: OAuthProviderInfo, enabled: boolean) {
  const previous = item.enabled
  item.enabled = enabled
  try {
    const updated = await updateOAuthProvider(item.provider, {
      enabled,
      scopes: item.scopes,
      remark: item.remark,
      credentials: credentialDrafts[item.provider] || {},
    })
    const index = providers.value.findIndex((p) => p.provider === item.provider)
    if (index >= 0) providers.value[index] = updated
    credentialDrafts[item.provider] = { ...(updated.credentials || {}) }
    MessagePlugin.success(enabled ? '已启用，登录页将展示该渠道' : '已停用，登录页不再展示')
  } catch (error) {
    item.enabled = previous
    MessagePlugin.error((error as Error)?.message || '切换失败')
  }
}

async function handleTest(item: OAuthProviderInfo) {
  testingProvider.value = item.provider
  try {
    const result = await testOAuthProvider(item.provider)
    // 失败也是 200：用 message 表达原因，而不是抛错给通用提示。
    if (result?.ok) {
      MessagePlugin.success(result.message || '连通性正常')
    } else {
      MessagePlugin.warning(result?.message || '连通测试未通过')
    }
    await loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '测试请求失败')
  } finally {
    testingProvider.value = ''
  }
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    MessagePlugin.success('回调地址已复制')
  } catch {
    MessagePlugin.warning('浏览器拒绝了剪贴板访问，请手动复制')
  }
}

onMounted(() => {
  void loadProviders()
})
</script>

<style scoped>
.oauth-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-alert {
  border-radius: var(--hs-radius-lg, 8px);
}

.loading-card,
.empty-card {
  padding: 48px 20px;
  display: flex;
  justify-content: center;
}

.provider-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(420px, 1fr));
  gap: 16px;
}

.provider-card {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.provider-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.provider-card__title {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.provider-card__icon {
  width: 36px;
  height: 36px;
  border-radius: var(--hs-radius-lg, 8px);
  background: var(--td-brand-color-1, #eef2ff);
  color: var(--td-brand-color-7, #2b5cff);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  flex-shrink: 0;
}

.provider-card__names {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.provider-card__switch {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.provider-card__row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.provider-card__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.provider-card__callback {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.callback-text {
  flex: 1;
  min-width: 0;
  padding: 4px 8px;
  border-radius: var(--hs-radius-sm, 4px);
  background: var(--hs-surface-2, #f5f7fa);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-card__fields {
  border-top: 1px solid var(--hs-border-color, #e5e7eb);
  padding-top: 12px;
}

.provider-card__foot {
  display: flex;
  align-items: center;
  gap: 8px;
  border-top: 1px solid var(--hs-border-color, #e5e7eb);
  padding-top: 12px;
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

.error-text {
  color: var(--td-error-color, #d54941);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .provider-grid {
    grid-template-columns: 1fr;
  }
}
</style>
