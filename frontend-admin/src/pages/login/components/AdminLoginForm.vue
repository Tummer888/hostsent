<template>
  <div class="admin-login-card" role="region" aria-labelledby="login-heading">
    <div class="admin-login-card__header">
      <p class="admin-login-card__eyebrow" id="login-heading-eyebrow">Secure Sign In</p>
      <h2 class="admin-login-card__title" id="login-heading">管理员登录</h2>
      <p class="admin-login-card__subtitle">
        欢迎回来，使用管理员账号登录 <span class="accent-word">宿派云控</span> 管理平台。
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
      aria-describedby="login-heading-eyebrow"
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

      <div class="captcha-row">
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
            maxlength="4"
            :status="captchaStatus"
            aria-required="true"
            @enter="onSubmitClick"
          >
            <template #prefix-icon>
              <ShieldErrorIcon aria-hidden="true" />
            </template>
          </t-input>
        </t-form-item>
        <button
          type="button"
          class="captcha-image"
          @click="onRefreshCaptcha"
          title="点击刷新验证码"
          aria-label="点击刷新验证码"
        >
          <img
            v-if="captchaImage"
            :src="captchaImage"
            alt="图形验证码"
            width="110"
            height="40"
            loading="eager"
          />
          <span v-else class="captcha-fallback">验证码</span>
        </button>
      </div>

      <div class="action-row">
        <t-checkbox v-model="formData.remember" size="medium" aria-label="记住账号和密码">
          记住我（当前设备）
        </t-checkbox>
        <t-link theme="primary" size="small" hover="color" tabindex="0" @click="onForget">
          忘记密码？
        </t-link>
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

    <t-divider align="middle" class="alt-divider">安全选项</t-divider>

    <div class="alt-login">
      <t-tooltip content="密钥登录功能即将上线" placement="top">
        <button type="button" class="alt-login__item" @click="onKeyLogin">
          <span class="alt-login__icon" aria-hidden="true">
            <LockCheckedIcon size="20" />
          </span>
          <span class="alt-login__text">密钥登录</span>
        </button>
      </t-tooltip>
      <t-tooltip content="IP 白名单 + 双因子认证" placement="top">
        <button type="button" class="alt-login__item" @click="onSecurity">
          <span class="alt-login__icon" aria-hidden="true">
            <ServiceIcon size="20" />
          </span>
          <span class="alt-login__text">安全策略</span>
        </button>
      </t-tooltip>
    </div>

    <p class="security-hint" aria-live="polite">
      <LockOnIcon size="13" aria-hidden="true" />
      登录过程受 TLS 加密保护；系统会对异常登录进行二次验证。
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  BrowseIcon,
  BrowseOffIcon,
  ErrorCircleFilledIcon,
  LockCheckedIcon,
  LockOnIcon,
  LoginIcon,
  ServiceIcon,
  ShieldErrorIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, SubmitContext } from 'tdesign-vue-next'

import { useUserStore } from '@/store'

defineOptions({ name: 'AdminLoginForm' })

type FormField = 'username' | 'password' | 'captchaCode'
type VRItem = { result?: boolean; message?: string }
type VR = boolean | VRItem[] | VRItem

const formRef = ref<FormInstanceFunctions | null>(null)
const errorSummaryRef = ref<HTMLElement | null>(null)
const showPassword = ref(false)
const loading = ref(false)
const captchaKey = ref('')
const captchaImage = ref('')
const touchedFields = ref<Record<string, boolean>>({})
const submitError = ref('')

const userStore = useUserStore()
const router = useRouter()
const route = useRoute()

const formData = reactive({
  username: userStore.savedUsername || '',
  password: userStore.savedPassword || '',
  captchaCode: '',
  remember: userStore.remember,
})

const notLoading = computed(() => !loading.value)
const passwordInputType = computed(() => (showPassword.value ? 'text' : 'password'))
const passwordAriaLabel = computed(() => (showPassword.value ? '隐藏密码' : '显示密码'))
const buttonText = computed(() => (notLoading.value ? '登录管理平台' : '正在登录，请稍候…'))

const rules: Record<FormField, FormRule[]> = {
  username: [
    { required: true, message: '请输入管理员账号', type: 'error', trigger: 'blur' },
    { min: 2, max: 50, message: '账号长度为 2-50 个字符', type: 'error', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', type: 'error', trigger: 'blur' },
    { min: 4, max: 64, message: '密码长度为 4-64 个字符', type: 'error', trigger: 'blur' },
  ],
  captchaCode: [
    { required: true, message: '请输入图形验证码', type: 'error', trigger: 'blur' },
    { len: 4, message: '验证码为 4 位字符', type: 'error', trigger: 'blur' },
  ],
}

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

function generateCaptcha() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'
  let result = ''
  for (let i = 0; i < 4; i += 1) {
    result += chars[Math.floor(Math.random() * chars.length)]
  }
  captchaKey.value = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  captchaImage.value = renderCaptcha(result)
}

function refreshCaptchaInternal() {
  submitError.value = ''
  generateCaptcha()
  formData.captchaCode = ''
  touchedFields.value.captchaCode = false
}

function onRefreshCaptcha() {
  refreshCaptchaInternal()
  MessagePlugin.success('验证码已刷新')
}

function renderCaptcha(text: string): string {
  const canvas = document.createElement('canvas')
  canvas.width = 110
  canvas.height = 40
  const ctx = canvas.getContext('2d')
  if (!ctx) return ''
  ctx.fillStyle = '#f8fafc'
  ctx.fillRect(0, 0, canvas.width, canvas.height)
  for (let i = 0; i < 6; i += 1) {
    ctx.strokeStyle = `rgba(22, 163, 74, ${0.12 + Math.random() * 0.2})`
    ctx.beginPath()
    ctx.moveTo(Math.random() * canvas.width, Math.random() * canvas.height)
    ctx.lineTo(Math.random() * canvas.width, Math.random() * canvas.height)
    ctx.stroke()
  }
  for (let i = 0; i < 36; i += 1) {
    ctx.fillStyle = `rgba(37, 99, 235, ${0.16 + Math.random() * 0.38})`
    ctx.beginPath()
    ctx.arc(Math.random() * canvas.width, Math.random() * canvas.height, Math.random() * 1.2, 0, Math.PI * 2)
    ctx.fill()
  }
  const palette = ['#16a34a', '#2563eb', '#6366f1', '#d97706', '#0f172a']
  for (let i = 0; i < text.length; i += 1) {
    ctx.save()
    ctx.font = 'bold 22px "Fira Code", "PingFang SC", "Microsoft YaHei", monospace'
    ctx.fillStyle = palette[i % palette.length]
    const offsetX = 16 + i * 20 + (Math.random() - 0.5) * 3
    const offsetY = 27 + (Math.random() - 0.5) * 5
    const rotate = ((Math.random() - 0.5) * Math.PI) / 7
    ctx.translate(offsetX, offsetY)
    ctx.rotate(rotate)
    ctx.fillText(text[i], 0, 0)
    ctx.restore()
  }
  ctx.strokeStyle = 'rgba(239, 68, 68, 0.5)'
  ctx.beginPath()
  ctx.moveTo(6, 18 + Math.random() * 6)
  ctx.bezierCurveTo(28, 4, 56, 38, 104, 22 + Math.random() * 8)
  ctx.stroke()
  return canvas.toDataURL('image/png')
}

function onTogglePassword() {
  showPassword.value = !showPassword.value
}

async function focusFieldByName(field: FormField) {
  const selector = field === 'username' ? '#input-username' : field === 'password' ? '#input-password' : '#input-captcha'
  const el = document.querySelector<HTMLElement>(selector)
  el?.focus()
}

function onErrorClick(field: FormField) {
  focusFieldByName(field)
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
  try {
    loading.value = true
    await userStore.login({
      username: formData.username.trim(),
      password: formData.password,
      captchaKey: captchaKey.value,
      captchaCode: formData.captchaCode.trim().toUpperCase(),
      remember: formData.remember,
    })
    MessagePlugin.success('登录成功，正在进入管理平台…')
    const redirect = typeof route.query.redirect === 'string' ? decodeURIComponent(route.query.redirect) : ''
    await router.replace(redirect || '/dashboard/base')
  } catch (error) {
    submitError.value = (error as Error)?.message || '登录失败，请稍后重试'
    MessagePlugin.error(submitError.value)
    refreshCaptchaInternal()
    await nextTick()
    errorSummaryRef.value?.focus({ preventScroll: true })
  } finally {
    loading.value = false
  }
}

async function onSubmitClick() {
  await formRef.value?.submit?.()
}

function onForget() {
  MessagePlugin.info('请联系系统管理员或使用找回密码邮箱流程重置密码')
}

function onKeyLogin() {
  MessagePlugin.info('密钥登录功能正在内测，敬请期待')
}

function onSecurity() {
  MessagePlugin.info('默认启用：强密码 + IP 异常检测 + 登录审计 + 失败锁定')
}

function markTouched(field: FormField) {
  touchedFields.value[field] = true
}

onMounted(async () => {
  generateCaptcha()
  await nextTick()
  const fields: readonly FormField[] = ['username', 'password', 'captchaCode'] as const
  for (const field of fields) {
    const id = field === 'captchaCode' ? 'input-captcha' : `input-${field}`
    const el = document.getElementById(id)
    if (el) {
      el.addEventListener('blur', () => markTouched(field), { once: false })
    }
  }
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

.admin-login-card__eyebrow {
  margin: 0 0 6px;
  font-family: var(--hs-font-mono);
  font-size: 11px;
  letter-spacing: 0.24em;
  text-transform: uppercase;
  color: #16a34a;
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
  color: #16a34a;
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
  grid-template-columns: 1fr 110px;
  gap: 10px;
  align-items: end;
  margin-bottom: 2px;
}

.captcha-row__input {
  margin: 0;
}

.captcha-image {
  width: 110px;
  height: 40px;
  border-radius: var(--hs-radius-sm);
  overflow: hidden;
  border: 1px solid var(--color-border);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f8fafc;
  user-select: none;
  padding: 0;
  transition: border-color var(--hs-duration-fast), transform var(--hs-duration-fast);
}

.captcha-image:hover {
  border-color: #86efac;
  transform: translateY(-1px);
}

.captcha-image img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.captcha-fallback {
  font-size: 11px;
  color: var(--color-muted-foreground);
  letter-spacing: 0.08em;
}

.action-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 6px 0 12px;
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.submit-btn {
  height: 42px;
  font-family: var(--hs-font-heading);
  font-weight: 600;
  letter-spacing: 0.02em;
  background: #16a34a;
  border: 0;
  color: #ffffff;
  border-radius: var(--hs-radius-md);
  transition: background-color var(--hs-duration-fast), transform var(--hs-duration-fast), box-shadow var(--hs-duration-fast);
  box-shadow: 0 4px 12px rgba(22, 163, 74, 0.22);
}

.submit-btn:hover {
  background: #15803d;
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

.alt-login {
  display: flex;
  justify-content: center;
  gap: 18px;
}

.alt-login__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  color: var(--color-muted-foreground);
  background: transparent;
  border: 0;
  padding: 5px 8px;
  border-radius: var(--hs-radius-sm);
  transition: color var(--hs-duration-fast), background-color var(--hs-duration-fast), transform var(--hs-duration-fast);
}

.alt-login__item:hover {
  color: var(--color-foreground);
  background: var(--hs-surface-3);
  transform: translateY(-1px);
}

.alt-login__icon {
  width: 40px;
  height: 40px;
  border-radius: var(--hs-radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--hs-surface-2);
  color: #16a34a;
  border: 1px solid var(--color-border);
}

.alt-login__text {
  font-size: 11.5px;
}

.security-hint {
  margin: 14px 0 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11.5px;
  color: var(--color-muted-foreground);
  line-height: 1.55;
}
</style>
