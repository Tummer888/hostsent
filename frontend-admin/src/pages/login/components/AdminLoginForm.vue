<template>
  <div class="admin-login-card" role="region" aria-labelledby="login-heading">
    <div class="admin-login-card__header">
      <h2 class="admin-login-card__title" id="login-heading">管理员登录</h2>
      <p class="admin-login-card__subtitle">
        欢迎回来，使用管理员账号登录 <span class="accent-word">{{ brandStore.name }}</span> 管理平台。
      </p>
    </div>

    <div
      v-if="hasErrorSummary"
      class="error-summary"
      role="alert"
      tabindex="-1"
      ref="errorSummaryRef"
      aria-labelledby="error-summary-title"
    >
      <div class="error-summary__head">
        <ErrorCircleFilledIcon />
        <strong id="error-summary-title">无法登录，请检查以下信息：</strong>
      </div>
      <ul class="error-summary__list">
        <li v-for="(err, i) in errorSummary" :key="i">
          <a :href="err.href" @click.prevent="onErrorClick(err.field)">
            {{ err.message }}
          </a>
        </li>
      </ul>
    </div>

    <t-form
      ref="formRef"
      :data="formData"
      :rules="rules"
      label-align="top"
      colonless
      class="admin-login-card__form"
      :disabled="loading"
      novalidate
      @submit="onSubmit"
    >
      <t-form-item
        id="field-username"
        label="用户名（账号）"
        name="username"
        for="input-username"
      >
        <t-input
          id="input-username"
          v-model="formData.username"
          size="large"
          placeholder="请输入管理员账号"
          autocomplete="username"
          :status="usernameStatus"
          aria-required="true"
        >
          <template #prefix-icon>
            <UserIcon aria-hidden="true" />
          </template>
        </t-input>
      </t-form-item>

      <t-form-item
        id="field-password"
        label="登录密码"
        name="password"
        for="input-password"
      >
        <t-input
          id="input-password"
          v-model="formData.password"
          :type="passwordInputType"
          size="large"
          placeholder="请输入密码"
          autocomplete="current-password"
          :status="passwordStatus"
          aria-required="true"
          @enter="onSubmitClick"
        >
          <template #prefix-icon>
            <LockOnIcon aria-hidden="true" />
          </template>
          <template #suffix-icon>
            <button
              type="button"
              class="icon-btn"
              :aria-label="passwordAriaLabel"
              :aria-pressed="showPassword"
              @click="onTogglePassword"
            >
              <BrowseOffIcon v-if="showPassword" aria-hidden="true" />
              <BrowseIcon v-else aria-hidden="true" />
            </button>
          </template>
        </t-input>
      </t-form-item>

      <!--
        图形码只在场景策略要求时渲染（GET /public/auth-config）。
        默认 admin_login.image_required=false → 升级后登录页视觉与升级前完全一致。
      -->
      <div v-if="captchaNeeded" class="captcha-row">
        <t-form-item
          id="field-captcha"
          label="图形验证码"
          name="captchaCode"
          class="captcha-row__input"
          for="input-captcha"
        >
          <t-input
            id="input-captcha"
            v-model="formData.captchaCode"
            size="large"
            placeholder="请输入验证码"
            autocomplete="off"
            maxlength="5"
            :status="captchaStatus"
            aria-required="true"
            @enter="onSubmitClick"
          >
            <template #prefix-icon>
              <ShieldErrorIcon aria-hidden="true" />
            </template>
          </t-input>
        </t-form-item>
        <CaptchaImage
          ref="captchaRef"
          v-model:key="captchaKey"
          v-model:code="formData.captchaCode"
          scene="admin_login"
          class="captcha-row__image"
        />
      </div>

      <div class="action-row">
        <t-checkbox v-model="formData.remember" size="medium" aria-label="记住账号和密码">
          记住我（当前设备）
        </t-checkbox>
      </div>

      <t-button
        type="submit"
        theme="primary"
        size="large"
        block
        :loading="loading"
        class="submit-btn"
        :aria-busy="loading"
      >
        <template #icon v-if="notLoading">
          <LoginIcon aria-hidden="true" />
        </template>
        {{ buttonText }}
      </t-button>
    </t-form>

    <!--
      安全选项说明（替代原先的两个假入口）：
      「密钥登录」全仓没有后端（grep passkey|webauthn 零命中），「安全策略」只是一句
      提示文案 —— 两个按钮点了都只会弹「即将上线」。这里改成如实说明当前登录会走哪些
      保护，保护本身由「系统管理 → 验证码配置 / 系统配置 → 安全配置」控制（真实能力）。
    -->
    <p class="login-security-note">
      <LockCheckedIcon size="15" aria-hidden="true" />
      <span>登录保护：图形验证码、短信/邮箱二次验证与失败锁定，按后台安全策略自动生效。</span>
    </p>

    <LoginOTPVerifyDialog
      v-model:visible="otpVisible"
      :otp-token="otpToken"
      :otp-channel="otpContext.channel"
      :otp-target-masked="otpContext.targetMasked"
      :otp-expire-in="otpContext.expireIn"
      :submitting="otpSubmitting"
      @verify="onVerifyOTP"
      @resend="onResendOTP"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  BrowseIcon,
  BrowseOffIcon,
  ErrorCircleFilledIcon,
  LockCheckedIcon,
  LockOnIcon,
  LoginIcon,
  ShieldErrorIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, SubmitContext } from 'tdesign-vue-next'

import CaptchaImage from '@/components/verify/CaptchaImage.vue'
import LoginOTPVerifyDialog from '@/pages/login/components/LoginOTPVerifyDialog.vue'
import { useBrandStore, useUserStore } from '@/store'
import { imageRequired, loadAuthConfig } from '@/utils/captcha-resource'

defineOptions({ name: 'AdminLoginForm' })

type FormField = 'username' | 'password' | 'captchaCode'
type VRItem = { result?: boolean; message?: string }
type VR = boolean | VRItem[] | VRItem

const formRef = ref<FormInstanceFunctions | null>(null)
const errorSummaryRef = ref<HTMLElement | null>(null)
const captchaRef = ref<InstanceType<typeof CaptchaImage> | null>(null)
const showPassword = ref(false)
const loading = ref(false)
const captchaKey = ref('')
const touchedFields = ref<Record<string, boolean>>({})
const submitError = ref('')

// —— 登录二次验证（doc91 §4.6）——
const otpVisible = ref(false)
const otpSubmitting = ref(false)
const otpToken = ref('')
const otpContext = reactive({ channel: '', targetMasked: '', expireIn: 0 })
/** 记住上次密码登录的入参，重发验证码时重放（后端重发 = 再走一次密码登录）。 */
const lastCredentials = ref<{ username: string; password: string } | null>(null)

const userStore = useUserStore()
// 品牌名由登录页 onMounted 拉取（公开接口），这里只读。
const brandStore = useBrandStore()
const router = useRouter()
const route = useRoute()

const formData = reactive({
  username: userStore.savedUsername || '',
  password: userStore.savedPassword || '',
  captchaCode: '',
  remember: userStore.remember,
})

/** 策略是否要求图形码；auth-config 未加载完成或总闸关闭时为 false（不渲染）。 */
const captchaNeeded = computed(() => imageRequired('admin_login'))

const notLoading = computed(() => !loading.value)
const passwordInputType = computed(() => (showPassword.value ? 'text' : 'password'))
const passwordAriaLabel = computed(() => (showPassword.value ? '隐藏密码' : '显示密码'))
const buttonText = computed(() => (notLoading.value ? '登录管理平台' : '正在登录，请稍候…'))

// 图形码规则随策略动态增删：策略关着时不渲染输入框，若仍留必填规则会永远提交不了。
const rules = computed<Record<FormField, FormRule[]>>(() => ({
  username: [
    { required: true, message: '请输入管理员账号', type: 'error', trigger: 'blur' },
    { min: 2, max: 50, message: '账号长度为 2-50 个字符', type: 'error', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', type: 'error', trigger: 'blur' },
    { min: 4, max: 64, message: '密码长度为 4-64 个字符', type: 'error', trigger: 'blur' },
  ],
  captchaCode: captchaNeeded.value
    ? [{ required: true, message: '请输入图形验证码', type: 'error', trigger: 'blur' }]
    : [],
}))

type ErrorItem = { field: FormField; href: string; message: string }

function getResult(key: FormField): VR | undefined {
  return (formRef.value as unknown as { validateResult?: Record<string, VR> })?.validateResult?.[key]
}

const errorSummary = computed<ErrorItem[]>(() => {
  const list: ErrorItem[] = []
  if (submitError.value) {
    list.push({ field: 'username', href: '#field-username', message: submitError.value })
  }
  const mapping: Array<{ key: FormField; label: string }> = [
    { key: 'username', label: '账号' },
    { key: 'password', label: '密码' },
    { key: 'captchaCode', label: '验证码' },
  ]
  for (const { key, label } of mapping) {
    const result = getResult(key)
    if (result === true || !result) continue
    const arr: VRItem[] = Array.isArray(result) ? result : [result]
    const msg = arr[0]?.message
    if (msg) list.push({ field: key, href: `#field-${key}`, message: `${label}：${msg}` })
  }
  return list
})

const hasErrorSummary = computed(() => errorSummary.value.length > 0)

function computeStatus(field: FormField): 'error' | 'warning' | 'success' | undefined {
  if (!touchedFields.value[field] && !submitError.value) return undefined
  const current = getResult(field)
  if (!current || current === true) return touchedFields.value[field] ? 'success' : undefined
  const arr: VRItem[] = Array.isArray(current) ? current : [current]
  const bad = arr.some((r) => r.result === false)
  return bad ? 'error' : touchedFields.value[field] ? 'success' : undefined
}

const usernameStatus = computed(() => computeStatus('username'))
const passwordStatus = computed(() => computeStatus('password'))
const captchaStatus = computed(() => computeStatus('captchaCode'))

/** 刷新图形码（清空已输入答案）。策略关闭时组件未渲染，直接跳过。 */
function refreshCaptchaInternal() {
  submitError.value = ''
  formData.captchaCode = ''
  touchedFields.value.captchaCode = false
  captchaRef.value?.refresh()
}

function onTogglePassword() {
  showPassword.value = !showPassword.value
}

async function focusFieldByName(field: FormField) {
  const selector =
    field === 'username' ? '#input-username' : field === 'password' ? '#input-password' : '#input-captcha'
  const el = document.querySelector<HTMLElement>(selector)
  el?.focus()
}

function onErrorClick(field: FormField) {
  focusFieldByName(field)
}

/** 密码步骤：命中二次验证时不写令牌，转而弹 OTP 框。 */
async function doPasswordLogin(username: string, password: string) {
  const outcome = await userStore.login({
    username,
    password,
    captchaKey: captchaKey.value,
    captchaCode: formData.captchaCode.trim().toUpperCase(),
    remember: formData.remember,
  })
  if (outcome.needOTP && outcome.otpToken) {
    otpToken.value = outcome.otpToken
    otpContext.channel = outcome.otpChannel || ''
    otpContext.targetMasked = outcome.otpTargetMasked || ''
    otpContext.expireIn = outcome.otpExpireIn || 0
    otpVisible.value = true
    touchedFields.value = {}
    MessagePlugin.info('请完成二次验证')
    return
  }
  await finishLogin()
}

async function onSubmit(ctx: SubmitContext) {
  submitError.value = ''
  if (ctx.validateResult !== true) {
    await nextTick()
    const first =
      errorSummary.value.find((e) => e.field === 'username')?.field ||
      errorSummary.value.find((e) => e.field === 'password')?.field ||
      errorSummary.value.find((e) => e.field === 'captchaCode')?.field
    if (first) {
      await focusFieldByName(first)
      errorSummaryRef.value?.scrollIntoView({ block: 'start', behavior: 'smooth' })
      errorSummaryRef.value?.focus({ preventScroll: true })
    }
    return
  }
  const username = formData.username.trim()
  const password = formData.password
  try {
    loading.value = true
    lastCredentials.value = { username, password }
    await doPasswordLogin(username, password)
  } catch (error) {
    submitError.value = (error as Error)?.message || '登录失败，请稍后重试'
    MessagePlugin.error(submitError.value)
    // 图形码一次性：失败后必须换新图，否则用户拿着旧图再怎么输都错。
    refreshCaptchaInternal()
    await nextTick()
    errorSummaryRef.value?.focus({ preventScroll: true })
  } finally {
    loading.value = false
  }
}

/** OTP 校验通过 → 用 otp_token 换正式令牌。 */
async function onVerifyOTP(code: string) {
  if (!otpToken.value) return
  try {
    otpSubmitting.value = true
    await userStore.loginVerifyOTP(otpToken.value, code)
    otpVisible.value = false
    await finishLogin()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '验证失败，请重试')
  } finally {
    otpSubmitting.value = false
  }
}

/** 重发验证码：重放密码登录，后端会重新下发 OTP 并换发新的 otp_token。 */
async function onResendOTP() {
  const creds = lastCredentials.value
  if (!creds) {
    MessagePlugin.warning('请返回重新输入账号密码')
    otpVisible.value = false
    return
  }
  try {
    const outcome = await userStore.login({
      username: creds.username,
      password: creds.password,
      remember: false,
    })
    if (outcome.needOTP && outcome.otpToken) {
      otpToken.value = outcome.otpToken
      otpContext.channel = outcome.otpChannel || otpContext.channel
      otpContext.targetMasked = outcome.otpTargetMasked || otpContext.targetMasked
      otpContext.expireIn = outcome.otpExpireIn || otpContext.expireIn
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '验证码发送失败，请稍后重试')
  }
}

/** 登录成功后的统一收尾：提示 + 跳转（含 redirect 参数）。 */
async function finishLogin() {
  MessagePlugin.success('登录成功')
  const redirect = typeof route.query.redirect === 'string' ? decodeURIComponent(route.query.redirect) : ''
  await router.replace(redirect || '/dashboard/base')
}

async function onSubmitClick() {
  await formRef.value?.submit?.()
}

function markTouched(field: FormField) {
  touchedFields.value[field] = true
}

// 策略加载完成后再决定是否绑定图形码输入框的 blur 监听（元素此前不存在）。
watch(captchaNeeded, async (needed) => {
  if (!needed) return
  await nextTick()
  document.getElementById('input-captcha')?.addEventListener('blur', () => markTouched('captchaCode'))
})

onMounted(async () => {
  // 并行拉策略，不阻塞首屏：接口失败时按「不需要图形码」处理，登录照常可用。
  void loadAuthConfig()
  for (const field of ['username', 'password'] as const) {
    document.getElementById(`input-${field}`)?.addEventListener('blur', () => markTouched(field))
  }
  await nextTick()
  if (!formData.username) {
    document.getElementById('input-username')?.focus()
  } else {
    document.getElementById('input-password')?.focus()
  }
})
</script>

<style scoped lang="css">
.admin-login-card {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 400px;
  padding: 24px 22px 20px;
  border-radius: var(--hs-radius-lg);
  color: var(--color-card-foreground);
  background: var(--color-card);
  border: 1px solid var(--color-border);
  box-shadow: var(--hs-shadow-md);
}

.admin-login-card__header {
  margin-bottom: 16px;
  text-align: left;
}

.admin-login-card__title {
  margin: 0 0 6px;
  font-family: var(--hs-font-heading);
  font-size: 22px;
  font-weight: 700;
  color: var(--color-foreground);
}

.admin-login-card__subtitle {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.accent-word {
  color: var(--color-primary);
  font-weight: 600;
}

.error-summary {
  padding: 10px 12px;
  border-radius: var(--hs-radius-md);
  margin-bottom: 14px;
  background: #fef2f2;
  border: 1px solid #fecaca;
}

.error-summary__head {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #b91c1c;
  font-size: 12.5px;
  margin-bottom: 4px;
}

.error-summary__list {
  margin: 0;
  padding-left: 20px;
  color: #991b1b;
  font-size: 12px;
  line-height: 1.7;
}

.error-summary__list a {
  color: #b91c1c;
  text-decoration: underline dotted rgba(185, 28, 28, 0.5);
  text-underline-offset: 3px;
}

.error-summary__list a:hover {
  color: #7f1d1d;
}

.admin-login-card__form {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.icon-btn {
  background: transparent;
  border: 0;
  padding: 4px;
  border-radius: var(--hs-radius-sm);
  cursor: pointer;
  color: var(--color-muted-foreground);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: color var(--hs-duration-fast), background-color var(--hs-duration-fast);
}

.icon-btn:hover,
.icon-btn:focus-visible {
  color: var(--color-foreground);
  background: var(--hs-surface-3);
}

:deep(.t-form__label) {
  color: #334155;
  font-size: 12.5px;
  font-weight: 500;
  padding-bottom: 4px;
}

:deep(.t-input) {
  background: var(--hs-surface-1);
}

:deep(.t-input--focused) {
  box-shadow: 0 0 0 2px rgba(22, 163, 74, 0.16);
}

.captcha-row {
  display: grid;
  /* minmax(0,1fr)：允许输入列收缩，防止原生 input 的 size 固有宽度在移动端把行撑出视口 */
  grid-template-columns: minmax(0, 1fr) 110px;
  gap: 10px;
  align-items: start;
  margin-bottom: 2px;
}

.captcha-row__input {
  margin: 0;
  min-width: 0;
}

.captcha-row__input :deep(.t-form__controls) {
  min-width: 0;
  width: 100%;
}

.captcha-row__image {
  margin-top: 26px;
}

.action-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 22px 0 12px;
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.submit-btn {
  height: 42px;
  font-family: var(--hs-font-heading);
  font-weight: 600;
  letter-spacing: 0.02em;
  background: var(--color-primary);
  border: 0;
  color: #ffffff;
  border-radius: var(--hs-radius-md);
  transition: background-color var(--hs-duration-fast), transform var(--hs-duration-fast), box-shadow var(--hs-duration-fast);
  box-shadow: 0 4px 12px rgba(22, 163, 74, 0.22);
}

.submit-btn:hover {
  background: var(--td-brand-color-8);
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(22, 163, 74, 0.32);
}

.submit-btn:active {
  transform: translateY(0);
}

.alt-divider {
  margin: 18px 0 12px;
  color: var(--color-muted-foreground);
}

/* 登录保护说明：替代原先两个点了只弹「即将上线」的假入口 */
.login-security-note {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  margin: 16px 0 0;
  padding: 10px 12px;
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  border: 1px solid var(--color-border);
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.login-security-note > svg {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--color-primary);
}
</style>
