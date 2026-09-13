<template>
  <div class="security-page">
    <!-- 页头：平台提升提示（notice 字段驱动，后端只在「用户请求与生效值不一致」时给） -->
    <section class="security-hero">
      <div class="hero-left">
        <span class="hero-chip"><SecuredIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">安全设置</span>
          <span class="hero-desc">账号绑定、二次验证与关键操作的验证强度。平台强制项无法降低。</span>
        </div>
      </div>
      <t-button theme="primary" size="large" :loading="saving" :disabled="!dirty" @click="handleSave">
        <template #icon><SaveIcon /></template>
        保存设置
      </t-button>
    </section>

    <t-alert v-if="platformNotice" theme="info" class="notice-alert" close>
      {{ platformNotice }}
    </t-alert>

    <div v-if="loading" class="loading-wrap"><t-loading size="large" text="加载中..." /></div>

    <template v-else-if="settings">
      <!-- 账号绑定 -->
      <section class="panel">
        <div class="panel-head">
          <span class="panel-title">账号绑定</span>
          <span class="panel-sub">验证码会发送到已绑定的邮箱或手机</span>
        </div>
        <div class="bind-grid">
          <div class="bind-item">
            <div class="bind-icon"><MobileIcon size="20" /></div>
            <div class="bind-body">
              <div class="bind-title">
                手机号
                <t-tag v-if="settings.phone_bound" theme="success" variant="light" size="small">已绑定</t-tag>
                <t-tag v-else theme="warning" variant="light" size="small">未绑定</t-tag>
              </div>
              <div class="bind-value">{{ settings.phone_masked || '—' }}</div>
            </div>
            <t-button
              variant="outline"
              size="small"
              :disabled="settings.phone_bound"
              @click="onBind('sms')"
            >
              {{ settings.phone_bound ? '已绑定' : '去绑定' }}
            </t-button>
          </div>

          <div class="bind-item">
            <div class="bind-icon"><MailIcon size="20" /></div>
            <div class="bind-body">
              <div class="bind-title">
                邮箱
                <t-tag v-if="settings.email_bound" theme="success" variant="light" size="small">已绑定</t-tag>
                <t-tag v-else theme="warning" variant="light" size="small">未绑定</t-tag>
              </div>
              <div class="bind-value">{{ settings.email_masked || '—' }}</div>
            </div>
            <t-button
              variant="outline"
              size="small"
              :disabled="settings.email_bound"
              @click="onBind('email')"
            >
              {{ settings.email_bound ? '已绑定' : '去绑定' }}
            </t-button>
          </div>
        </div>
      </section>

      <!-- 二次验证 -->
      <section class="panel">
        <div class="panel-head">
          <span class="panel-title">二次验证</span>
          <span class="panel-sub">开启后，登录与关键操作需要额外验证一次</span>
        </div>

        <div class="form-grid">
          <div class="form-row">
            <span class="form-label">总开关</span>
            <div class="form-control">
              <t-switch v-model="form.mfaEnabled" :disabled="mfaForced" @change="markDirty" />
              <span v-if="mfaForced" class="form-help form-help--lock">
                <LockOnIcon size="14" /> 平台已强制开启，无法关闭
              </span>
            </div>
          </div>

          <div class="form-row">
            <span class="form-label">验证通道</span>
            <div class="form-control">
              <t-radio-group v-model="form.mfaChannel" :disabled="!form.mfaEnabled" @change="markDirty">
                <t-radio value="email">邮箱</t-radio>
                <t-radio value="sms">短信</t-radio>
              </t-radio-group>
              <span v-if="minChannelLevel > 1" class="form-help">
                平台要求不低于{{ minLevelLabel }}，低等级通道不会生效
              </span>
            </div>
          </div>

          <div class="form-row">
            <span class="form-label">信任窗口</span>
            <div class="form-control">
              <t-input-number
                v-model="form.trustWindowMinutes"
                :min="0"
                :max="1440"
                :disabled="!form.mfaEnabled"
                theme="column"
                @change="markDirty"
              />
              <span class="form-help">分钟内同一设备不再重复验证；0 表示每次都要验证</span>
            </div>
          </div>
        </div>
      </section>

      <!-- 场景列表 -->
      <section class="panel">
        <div class="panel-head">
          <span class="panel-title">关键操作验证</span>
          <span class="panel-sub">平台强制的操作无法关闭；可在此对其它操作自行加严</span>
        </div>

        <t-table
          :data="settings.scenes"
          :columns="sceneColumns"
          row-key="scene"
          size="small"
          :bordered="false"
          hover
          cell-empty-content="—"
        >
          <template #name="{ row }">
            <div class="scene-name">
              <span class="cell-strong">{{ row.name || row.scene }}</span>
              <t-tooltip v-if="row.platform_forced" :content="row.notice || '平台强制开启，无法关闭'">
                <t-tag theme="warning" variant="light" size="small">
                  <template #icon><LockOnIcon /></template>
                  平台强制
                </t-tag>
              </t-tooltip>
            </div>
            <span v-if="row.notice" class="scene-notice">{{ row.notice }}</span>
          </template>
          <template #otp_required="{ row }">
            <t-switch
              :value="row.otp_required"
              size="small"
              :disabled="row.platform_forced || !row.user_can_tighten"
              @change="(val: boolean) => onSceneToggle(row, val)"
            />
          </template>
          <template #otp_channel="{ row }">
            <span>{{ channelLabel(row.otp_channel) }}</span>
          </template>
          <template #source="{ row }">
            <t-tag v-if="row.source === 'user_tightened'" theme="primary" variant="light" size="small">自行加严</t-tag>
            <span v-else class="cell-muted">平台基线</span>
          </template>
          <template #empty>
            <t-empty description="暂无可配置的关键操作" />
          </template>
        </t-table>

        <div class="panel-foot">
          <t-button theme="primary" :loading="saving" :disabled="!dirty" @click="handleSave">
            <template #icon><SaveIcon /></template>
            保存设置
          </t-button>
          <t-button v-if="dirty" variant="outline" @click="handleReset">撤销修改</t-button>
        </div>
      </section>
    </template>

    <!-- 绑定验证码弹窗：未绑定目标时走 OTP 流程 -->
    <t-dialog
      v-model:visible="bind.visible"
      :header="`绑定${bind.channel === 'sms' ? '手机号' : '邮箱'}`"
      :confirm-btn="{ content: '确认绑定', loading: bind.submitting }"
      width="420px"
      @confirm="onSubmitBind"
    >
      <div class="bind-dialog">
        <p class="bind-dialog__tip">
          请输入要绑定的{{ bind.channel === 'sms' ? '手机号' : '邮箱地址' }}，我们将发送验证码进行验证。
        </p>
        <t-input
          v-model="bind.target"
          :placeholder="bind.channel === 'sms' ? '请输入手机号' : '请输入邮箱地址'"
          size="large"
        />
        <div class="bind-dialog__code">
          <t-input v-model="bind.code" placeholder="请输入验证码" size="large" maxlength="6" />
          <t-button
            variant="outline"
            theme="primary"
            :disabled="bind.countdown > 0"
            :loading="bind.sending"
            @click="onSendBindCode"
          >
            {{ bind.countdown > 0 ? `${bind.countdown}s` : '获取验证码' }}
          </t-button>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { LockOnIcon, MailIcon, MobileIcon, SaveIcon, SecuredIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getSecuritySettings,
  updateSecuritySettings,
  sendSecurityVerification,
  verifySecurityCode,
  type EffectiveScene,
  type SecuritySettings,
} from '@/api/security'

defineOptions({ name: 'UserSecuritySettings' })

const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const settings = ref<SecuritySettings | null>(null)
const platformNotice = ref('')

const form = reactive({
  mfaEnabled: false,
  mfaChannel: 'email',
  trustWindowMinutes: 0,
})

/** 场景级加严（只记录用户改过的场景，提交时作为 scene_overrides 发出）。 */
const sceneOverrides = ref<Record<string, { otp_required?: boolean; otp_channel?: string }>>({})
/** 原始快照，供撤销。 */
let snapshot = { form: { ...form }, overrides: {} as typeof sceneOverrides.value }

const CHANNEL_LABELS: Record<string, string> = { email: '邮箱', sms: '短信' }
function channelLabel(channel?: string): string {
  if (!channel) return '—'
  return CHANNEL_LABELS[channel] || channel
}

function markDirty() {
  dirty.value = true
}

/** 平台是否强制 MFA：任一场景 platform_forced 且要求 OTP。 */
const mfaForced = computed(() => {
  const scenes = settings.value?.scenes || []
  return scenes.some((s) => s.platform_forced && s.otp_required)
})

/** 场景里要求的最强通道等级：用于提示「平台要求不低于 X」。 */
const minChannelLevel = computed(() => {
  const scenes = settings.value?.scenes || []
  return scenes.reduce((max, s) => Math.max(max, s.min_channel_level || 0), 0)
})

const minLevelLabel = computed(() => (minChannelLevel.value >= 2 ? '短信' : '邮箱'))

const sceneColumns: PrimaryTableCol<EffectiveScene>[] = [
  { colKey: 'name', title: '操作场景', width: 260 },
  { colKey: 'otp_required', title: '需要验证', width: 110 },
  { colKey: 'otp_channel', title: '验证通道', width: 110 },
  { colKey: 'source', title: '来源', width: 110 },
]

async function load() {
  loading.value = true
  try {
    const { data } = await getSecuritySettings()
    settings.value = data
    form.mfaEnabled = data.mfa_enabled
    form.mfaChannel = data.mfa_channel || 'email'
    form.trustWindowMinutes = data.trust_window_minutes || 0
    sceneOverrides.value = {}
    // 顶部提示：后端只在「用户请求被平台提回」时给 notice，一次展示即可。
    if (data.notice) platformNotice.value = data.notice
    snapshot = { form: { ...form }, overrides: {} }
    dirty.value = false
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载安全设置失败')
  } finally {
    loading.value = false
  }
}

function onSceneToggle(row: EffectiveScene, val: boolean) {
  // 只记录「加严」：关闭平台基线不产生 override（后端也会忽略）。
  if (val) {
    sceneOverrides.value[row.scene] = { ...sceneOverrides.value[row.scene], otp_required: true }
  } else {
    delete sceneOverrides.value[row.scene]
  }
  // 表格数据也要同步，否则开关视觉不跟随。
  row.otp_required = val
  markDirty()
}

function handleReset() {
  form.mfaEnabled = snapshot.form.mfaEnabled
  form.mfaChannel = snapshot.form.mfaChannel
  form.trustWindowMinutes = snapshot.form.trustWindowMinutes
  sceneOverrides.value = { ...snapshot.overrides }
  void load()
}

async function handleSave() {
  saving.value = true
  try {
    const { data } = await updateSecuritySettings({
      mfa_enabled: form.mfaEnabled,
      mfa_channel: form.mfaChannel,
      trust_window_minutes: form.trustWindowMinutes,
      scene_overrides: sceneOverrides.value,
    })
    settings.value = data
    form.mfaEnabled = data.mfa_enabled
    form.mfaChannel = data.mfa_channel || form.mfaChannel
    form.trustWindowMinutes = data.trust_window_minutes || 0
    sceneOverrides.value = {}
    snapshot = { form: { ...form }, overrides: {} }
    dirty.value = false
    // 后端在被平台提回时会带 notice，这里展示一次。
    platformNotice.value = data.notice || ''
    MessagePlugin.success('安全设置已更新')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// —— 账号绑定 ——
const bind = reactive({
  visible: false,
  channel: 'sms' as 'sms' | 'email',
  target: '',
  code: '',
  sending: false,
  submitting: false,
  countdown: 0,
})

function onBind(channel: 'sms' | 'email') {
  bind.channel = channel
  bind.target = ''
  bind.code = ''
  bind.countdown = 0
  bind.visible = true
}

async function onSendBindCode() {
  if (!bind.target.trim()) {
    MessagePlugin.warning(bind.channel === 'sms' ? '请输入手机号' : '请输入邮箱地址')
    return
  }
  bind.sending = true
  try {
    await sendSecurityVerification({ scene: bind.channel === 'sms' ? 'phone_bind' : 'email_bind', channel: bind.channel })
    MessagePlugin.success('验证码已发送，请查收')
    bind.countdown = 60
    const timer = setInterval(() => {
      bind.countdown -= 1
      if (bind.countdown <= 0) clearInterval(timer)
    }, 1000)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '验证码发送失败')
  } finally {
    bind.sending = false
  }
}

async function onSubmitBind() {
  if (!bind.target.trim() || !bind.code.trim()) {
    MessagePlugin.warning('请填写绑定目标与验证码')
    return
  }
  bind.submitting = true
  try {
    // 先换票据再改绑定：PUT /uc/auth/profile 的改手机/改邮箱分支会校验 X-Verify-Ticket。
    const { data } = await verifySecurityCode({
      scene: bind.channel === 'sms' ? 'phone_bind' : 'email_bind',
      code: bind.code.trim(),
    })
    const { updateProfile } = await import('@/api/auth')
    await updateProfile(
      bind.channel === 'sms'
        ? { phone: bind.target.trim() }
        : { email: bind.target.trim() },
    )
    bind.visible = false
    MessagePlugin.success('绑定成功')
    void data
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '绑定失败')
  } finally {
    bind.submitting = false
  }
}

onMounted(load)
</script>

<style scoped>
.security-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.security-hero {
  background: linear-gradient(135deg, #1e40af 0%, #2563eb 100%);
  border-radius: 16px;
  padding: 24px 32px;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}

.hero-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.hero-chip {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.18);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-label {
  font-size: 18px;
  font-weight: 700;
}

.hero-desc {
  font-size: 13px;
  opacity: 0.85;
}

.security-hero .t-button {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.3);
  color: #fff;
}

.notice-alert {
  border-radius: 10px;
}

.loading-wrap {
  display: flex;
  justify-content: center;
  padding: 60px;
}

.panel {
  background: #fff;
  border-radius: 12px;
  padding: 20px 24px 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.panel-head {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.panel-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.panel-sub {
  font-size: 12px;
  color: #94a3b8;
}

.panel-foot {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid #f1f5f9;
}

/* 账号绑定 */
.bind-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.bind-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  border: 1px solid #eef2f8;
  border-radius: 12px;
  background: #fbfdff;
}

.bind-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(37, 99, 235, 0.1);
  color: #2563eb;
  flex-shrink: 0;
}

.bind-body {
  flex: 1;
  min-width: 0;
}

.bind-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.bind-value {
  font-size: 13px;
  color: #64748b;
  margin-top: 2px;
  word-break: break-all;
}

/* 表单 */
.form-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 18px;
}

.form-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.form-label {
  width: 84px;
  flex-shrink: 0;
  font-size: 13px;
  color: #475569;
  padding-top: 6px;
}

.form-control {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.form-help {
  font-size: 12px;
  color: #94a3b8;
}

.form-help--lock {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #d97706;
}

/* 场景列表 */
.scene-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.scene-notice {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  color: #d97706;
}

.cell-strong {
  font-weight: 600;
  color: #334155;
}

.cell-muted {
  color: #94a3b8;
}

.bind-dialog {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.bind-dialog__tip {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: #475569;
}

.bind-dialog__code {
  display: flex;
  gap: 10px;
}

.bind-dialog__code > :first-child {
  flex: 1;
  min-width: 0;
}

@media (max-width: 768px) {
  .security-hero {
    flex-direction: column;
    align-items: flex-start;
  }

  .bind-grid {
    grid-template-columns: 1fr;
  }

  .form-row {
    flex-direction: column;
    gap: 8px;
  }

  .form-label {
    width: auto;
    padding-top: 0;
  }
}
</style>
