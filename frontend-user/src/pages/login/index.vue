<template>
  <div class="login-page">
    <div class="page-bg" :style="{ backgroundImage: `url(${loginBg})` }"></div>

    <!-- 主内容区：卡片 + 侧边栏 -->
    <div class="main-content">
      <div class="login-card">
        <div class="card-header">
          <div class="header-left">
            <div class="card-logo">
              <img v-if="brandStore.logo" class="card-logo-img" :src="brandStore.logo" :alt="brandStore.name" />
              <CloudIcon v-else class="card-logo-icon" />
            </div>
            <div class="header-text">
              <h2 class="card-title">{{ brandStore.name }}</h2>
              <p class="card-subtitle">欢迎回来，请登录账户</p>
            </div>
          </div>
        </div>

        <div class="form-area">
          <t-tabs v-model:value="activeTab" theme="normal" class="login-tabs">
            <!-- ① 密码登录 -->
            <t-tab-panel value="password" label="密码登录">
              <div class="tab-content-inner">
                <div class="input-group">
                  <t-input
                    v-model="loginForm.username"
                    placeholder="请输入登录名/手机号"
                    size="large"
                    class="custom-input"
                  >
                    <template #prefix-icon>
                      <UserIcon />
                    </template>
                  </t-input>
                </div>
                <div class="input-group">
                  <t-input
                    v-model="loginForm.password"
                    :type="showPassword ? 'text' : 'password'"
                    placeholder="请输入登录密码"
                    size="large"
                    class="custom-input"
                    @enter="handleLogin"
                  >
                    <template #prefix-icon>
                      <LockOnIcon />
                    </template>
                    <template #suffix-icon>
                      <span class="password-toggle" @click="showPassword = !showPassword">
                        <BrowseIcon v-if="!showPassword" />
                        <BrowseOffIcon v-else />
                      </span>
                    </template>
                  </t-input>
                </div>
                <div v-if="needCaptcha('user_login')" class="input-group captcha-group">
                  <t-input
                    v-model="loginForm.captchaCode"
                    placeholder="请输入图形验证码"
                    size="large"
                    class="custom-input"
                    maxlength="5"
                    @enter="handleLogin"
                  >
                    <template #prefix-icon>
                      <ViewListIcon />
                    </template>
                  </t-input>
                  <CaptchaImage
                    ref="passwordCaptchaRef"
                    v-model:key="captchaKeys.password"
                    v-model:code="loginForm.captchaCode"
                    scene="user_login"
                  />
                </div>
                <div class="helper-row">
                  <router-link to="/forgot-password" class="helper-link">忘记密码？</router-link>
                </div>
              </div>
            </t-tab-panel>

            <!-- ② 短信登录 -->
            <t-tab-panel value="sms" label="短信登录">
              <div class="tab-content-inner">
                <div class="input-group">
                  <t-input
                    v-model="loginForm.phone"
                    placeholder="请输入手机号"
                    size="large"
                    class="custom-input"
                  >
                    <template #prefix-icon>
                      <MobileIcon />
                    </template>
                  </t-input>
                </div>
                <div v-if="needCaptcha('user_login_sms')" class="input-group captcha-group">
                  <t-input
                    v-model="smsCaptchaCode"
                    placeholder="请输入图形验证码"
                    size="large"
                    class="custom-input"
                    maxlength="5"
                  >
                    <template #prefix-icon>
                      <ViewListIcon />
                    </template>
                  </t-input>
                  <CaptchaImage
                    ref="smsCaptchaRef"
                    v-model:key="captchaKeys.sms"
                    v-model:code="smsCaptchaCode"
                    scene="user_login_sms"
                  />
                </div>
                <div class="input-group captcha-group sms-captcha-row">
                  <t-input
                    v-model="loginForm.smsCode"
                    placeholder="请输入短信验证码"
                    size="large"
                    class="custom-input sms-code-input"
                    maxlength="6"
                    @enter="handleLogin"
                  >
                    <template #prefix-icon>
                      <ChatMessageIcon />
                    </template>
                  </t-input>
                  <t-button
                    size="small"
                    variant="outline"
                    theme="primary"
                    class="send-code-btn"
                    :disabled="smsCountdown > 0"
                    :loading="sending.sms"
                    @click="handleSendCode('sms')"
                  >
                    {{ smsCountdown > 0 ? `${smsCountdown}s` : '获取验证码' }}
                  </t-button>
                </div>
              </div>
            </t-tab-panel>

            <!-- ③ 邮箱登录 -->
            <t-tab-panel value="email" label="邮箱登录">
              <div class="tab-content-inner">
                <div class="input-group">
                  <t-input
                    v-model="loginForm.email"
                    placeholder="请输入邮箱地址"
                    size="large"
                    class="custom-input"
                  >
                    <template #prefix-icon>
                      <MailIcon />
                    </template>
                  </t-input>
                </div>
                <div v-if="needCaptcha('user_login_email')" class="input-group captcha-group">
                  <t-input
                    v-model="emailCaptchaCode"
                    placeholder="请输入图形验证码"
                    size="large"
                    class="custom-input"
                    maxlength="5"
                  >
                    <template #prefix-icon>
                      <ViewListIcon />
                    </template>
                  </t-input>
                  <CaptchaImage
                    ref="emailCaptchaRef"
                    v-model:key="captchaKeys.email"
                    v-model:code="emailCaptchaCode"
                    scene="user_login_email"
                  />
                </div>
                <div class="input-group captcha-group sms-captcha-row">
                  <t-input
                    v-model="loginForm.emailCode"
                    placeholder="请输入邮箱验证码"
                    size="large"
                    class="custom-input sms-code-input"
                    maxlength="6"
                    @enter="handleLogin"
                  >
                    <template #prefix-icon>
                      <ChatMessageIcon />
                    </template>
                  </t-input>
                  <t-button
                    size="small"
                    variant="outline"
                    theme="primary"
                    class="send-code-btn"
                    :disabled="emailCountdown > 0"
                    :loading="sending.email"
                    @click="handleSendCode('email')"
                  >
                    {{ emailCountdown > 0 ? `${emailCountdown}s` : '获取验证码' }}
                  </t-button>
                </div>
              </div>
            </t-tab-panel>
          </t-tabs>

          <t-button theme="primary" size="large" block class="auth-btn" :loading="loginLoading" @click="handleLogin">
            登 录
          </t-button>

          <!-- 第三方登录（仅密码登录显示） -->
          <div v-if="activeTab === 'password'" class="third-party-login">
            <div class="divider-line">
              <span class="divider-text">其他登录方式</span>
            </div>
            <div class="third-party-icons">
              <div class="third-party-item" title="微信登录">
                <LogoWechatStrokeIcon class="third-party-icon wechat" />
              </div>
              <div class="third-party-item" title="QQ登录">
                <LogoQqIcon class="third-party-icon qq" />
              </div>
              <div class="third-party-item" title="支付宝登录">
                <LogoAlipayIcon class="third-party-icon alipay" />
              </div>
              <div class="third-party-item" title="企业微信登录">
                <LogoWecomIcon class="third-party-icon wecom" />
              </div>
              <div class="third-party-item" title="GitHub登录">
                <LogoGithubIcon class="third-party-icon github" />
              </div>
            </div>
          </div>
        </div>

        <div class="card-footer">
          <p class="register-hint">
            还没有账号？<router-link to="/register" class="helper-link">立即注册</router-link>
          </p>
          <p>{{ brandStore.copyrightText }}</p>
        </div>
      </div>

      <!-- 右侧切换标签：注册跳独立页（与登录页表单不重复维护两套校验） -->
      <div class="side-tabs">
        <div class="side-tab active">
          <UserIcon class="side-tab-icon" />
          <span class="side-tab-label">登录</span>
        </div>
        <div class="side-tab" @click="goRegister">
          <UserIcon class="side-tab-icon" />
          <span class="side-tab-label">注册</span>
        </div>
      </div>
    </div>

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
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  CloudIcon,
  UserIcon,
  LockOnIcon,
  BrowseIcon,
  BrowseOffIcon,
  ChatMessageIcon,
  ViewListIcon,
  MailIcon,
  MobileIcon,
  LogoWechatStrokeIcon,
  LogoQqIcon,
  LogoAlipayIcon,
  LogoWecomIcon,
  LogoGithubIcon,
} from 'tdesign-icons-vue-next'

import CaptchaImage from '@/components/verify/CaptchaImage.vue'
import LoginOTPVerifyDialog from '@/pages/login/components/LoginOTPVerifyDialog.vue'
import { sendVerifyCode } from '@/api/public'
import { useUserStore } from '@/store'
import { useBrandStore } from '@/store/modules/brand'
import { imageRequired, loadAuthConfig, otpChannel } from '@/utils/captcha-resource'

defineOptions({ name: 'UserLogin' })

import loginBg from '@/assets/images/login-user.webp'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const brandStore = useBrandStore()

const activeTab = ref<'password' | 'sms' | 'email'>('password')
const showPassword = ref(false)
const loginLoading = ref(false)
const sending = reactive({ sms: false, email: false })
const smsCountdown = ref(0)
const emailCountdown = ref(0)

/** 各 tab 的图形码 key（图形码一次性：换 tab 也要保留各自的 key，不能互相覆盖）。 */
const captchaKeys = reactive({ password: '', sms: '', email: '' })

const passwordCaptchaRef = ref<InstanceType<typeof CaptchaImage> | null>(null)
const smsCaptchaRef = ref<InstanceType<typeof CaptchaImage> | null>(null)
const emailCaptchaRef = ref<InstanceType<typeof CaptchaImage> | null>(null)

/** 短信/邮箱 tab 的图形码输入值单独存放：密码 tab 那个同名输入框是给密码登录用的。 */
const smsCaptchaCode = ref('')
const emailCaptchaCode = ref('')

const loginForm = reactive({
  username: '',
  password: '',
  captchaCode: '',
  phone: '',
  smsCode: '',
  email: '',
  emailCode: '',
})

// —— 登录二次验证（doc91 §4.6）——
const otpVisible = ref(false)
const otpSubmitting = ref(false)
const otpToken = ref('')
const otpContext = reactive({ channel: '', targetMasked: '', expireIn: 0 })
/** 重发验证码 = 重放密码登录（后端会重新下发 OTP 并换发新 otp_token）。 */
const lastCredentials = ref<{ username: string; password: string } | null>(null)

/** 场景是否要求图形码（auth-config 未加载或总闸关闭时恒 false）。 */
function needCaptcha(scene: string): boolean {
  return imageRequired(scene)
}

/** 各 tab 对应的登录场景（决定图形码 key 与发送场景）。 */
const sceneOf = computed(() => ({ sms: 'user_login_sms', email: 'user_login_email' }) as const)

function startCountdown(target: 'sms' | 'email') {
  const counter = target === 'sms' ? smsCountdown : emailCountdown
  counter.value = 60
  const timer = setInterval(() => {
    counter.value -= 1
    if (counter.value <= 0) clearInterval(timer)
  }, 1000)
}

/** 真实下发验证码（doc91 §4.1）：图形码先验、失败即换新图。 */
async function handleSendCode(target: 'sms' | 'email') {
  const scene = sceneOf.value[target]
  const channel = target === 'sms' ? 'sms' : 'email'
  const rawTarget = target === 'sms' ? loginForm.phone.trim() : loginForm.email.trim()
  if (!rawTarget) {
    MessagePlugin.warning(target === 'sms' ? '请先输入手机号' : '请先输入邮箱地址')
    return
  }
  const captchaCode = target === 'sms' ? smsCaptchaCode.value : emailCaptchaCode.value
  const captchaKey = target === 'sms' ? captchaKeys.sms : captchaKeys.email
  if (needCaptcha(scene) && !captchaCode.trim()) {
    MessagePlugin.warning('请先输入图形验证码')
    return
  }
  sending[target] = true
  try {
    const res = await sendVerifyCode({
      scene,
      channel,
      target: rawTarget,
      captcha_key: captchaKey || undefined,
      captcha_code: captchaCode.trim() || undefined,
    })
    MessagePlugin.success(`验证码已发送至 ${res.target_masked || rawTarget}`)
    startCountdown(target)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '验证码发送失败')
  } finally {
    // 图形码是一次性的：无论成功失败都要换新图，否则用户拿着旧图永远过不了。
    if (target === 'sms') {
      smsCaptchaCode.value = ''
      smsCaptchaRef.value?.refresh()
    } else {
      emailCaptchaCode.value = ''
      emailCaptchaRef.value?.refresh()
    }
    sending[target] = false
  }
}

/** 登录成功后的统一收尾。 */
async function finishLogin() {
  MessagePlugin.success('登录成功')
  const redirect = (route.query.redirect as string) || '/'
  router.replace(redirect)
}

/** 命中二次验证时弹 OTP 框，否则直接完成登录。 */
async function afterLogin(outcome: { needOTP: boolean; otpToken?: string; otpChannel?: string; otpTargetMasked?: string; otpExpireIn?: number }) {
  if (outcome.needOTP && outcome.otpToken) {
    otpToken.value = outcome.otpToken
    otpContext.channel = outcome.otpChannel || ''
    otpContext.targetMasked = outcome.otpTargetMasked || ''
    otpContext.expireIn = outcome.otpExpireIn || 0
    otpVisible.value = true
    MessagePlugin.info('请完成二次验证')
    return
  }
  await finishLogin()
}

async function handleLogin() {
  loginLoading.value = true
  try {
    if (activeTab.value === 'password') {
      if (!loginForm.username || !loginForm.password) {
        MessagePlugin.warning('请输入用户名和密码')
        return
      }
      if (needCaptcha('user_login') && !loginForm.captchaCode.trim()) {
        MessagePlugin.warning('请输入图形验证码')
        return
      }
      lastCredentials.value = { username: loginForm.username, password: loginForm.password }
      const outcome = await userStore.login({
        login_type: 'password',
        username: loginForm.username,
        password: loginForm.password,
        captcha_key: captchaKeys.password || undefined,
        captcha_code: loginForm.captchaCode.trim() || undefined,
      })
      await afterLogin(outcome)
      return
    }

    if (activeTab.value === 'sms') {
      if (!loginForm.phone.trim() || !loginForm.smsCode.trim()) {
        MessagePlugin.warning('请输入手机号和短信验证码')
        return
      }
      const outcome = await userStore.login({
        login_type: 'sms',
        phone: loginForm.phone.trim(),
        code: loginForm.smsCode.trim(),
        captcha_key: captchaKeys.sms || undefined,
        captcha_code: smsCaptchaCode.value.trim() || undefined,
      })
      await afterLogin(outcome)
      return
    }

    if (!loginForm.email.trim() || !loginForm.emailCode.trim()) {
      MessagePlugin.warning('请输入邮箱地址和邮箱验证码')
      return
    }
    const outcome = await userStore.login({
      login_type: 'email',
      email: loginForm.email.trim(),
      code: loginForm.emailCode.trim(),
      captcha_key: captchaKeys.email || undefined,
      captcha_code: emailCaptchaCode.value.trim() || undefined,
    })
    await afterLogin(outcome)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '登录失败，请稍后重试')
    // 登录失败同样消费掉了图形码（后端校验即销毁），必须换新图。
    if (activeTab.value === 'password') {
      loginForm.captchaCode = ''
      passwordCaptchaRef.value?.refresh()
    }
  } finally {
    loginLoading.value = false
  }
}

/** OTP 校验通过 → 换正式令牌。 */
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

/** 重发：重放密码登录以拿新的 otp_token。 */
async function onResendOTP() {
  const creds = lastCredentials.value
  if (!creds) {
    MessagePlugin.warning('请返回重新输入账号密码')
    otpVisible.value = false
    return
  }
  try {
    const outcome = await userStore.login({
      login_type: 'password',
      username: creds.username,
      password: creds.password,
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

function goRegister() {
  router.push('/register')
}

// 预留：策略里 OTP 默认通道（当前仅用于展示提示，可扩展为默认 tab）。
void otpChannel

onMounted(() => {
  void loadAuthConfig()
})
</script>

<style scoped>
.login-page {
  position: relative;
  width: 100%;
  height: 100vh;
  overflow: hidden;
}

.page-bg {
  position: absolute;
  inset: 0;
  background-size: 100% 100%;
  background-position: center;
  background-repeat: no-repeat;
  z-index: 0;
}

.main-content {
  position: relative;
  z-index: 1;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding-right: 8%;
}

/* ============ 登录卡片 ============ */
.login-card {
  width: 100%;
  max-width: 460px;
  background:
    radial-gradient(ellipse at 0% 0%, rgba(255, 182, 193, 0.6) 0%, transparent 55%),
    radial-gradient(ellipse at 100% 0%, rgba(135, 206, 250, 0.6) 0%, transparent 55%),
    radial-gradient(ellipse at 100% 100%, rgba(255, 255, 255, 0.55) 0%, transparent 55%),
    radial-gradient(ellipse at 0% 100%, rgba(152, 251, 152, 0.55) 0%, transparent 55%),
    rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(50px) saturate(1.8);
  -webkit-backdrop-filter: blur(50px) saturate(1.8);
  border-radius: 28px;
  border: 1px solid rgba(255, 255, 255, 0.5);
  box-shadow:
    0 32px 72px rgba(30, 64, 175, 0.08),
    0 12px 32px rgba(0, 0, 0, 0.04),
    inset 0 1px 0 rgba(255, 255, 255, 0.7);
  padding: 48px 44px 32px;
  margin: 20px 40px 20px 20px;
  max-height: calc(100vh - 40px);
  overflow-y: auto;
  overflow-x: hidden;
}

/* 强制 TDesign 子组件透明（保留主按钮和输入框的自定义样式） */
.login-card :deep(.t-tabs),
.login-card :deep(.t-tabs__nav),
.login-card :deep(.t-tab-panel),
.login-card :deep(.t-button--outline) {
  background: transparent !important;
}

.login-card::-webkit-scrollbar {
  width: 4px;
}

.login-card::-webkit-scrollbar-thumb {
  background: rgba(0, 82, 217, 0.2);
  border-radius: 2px;
}

/* 卡片头部：横排 */
.card-header {
  margin-bottom: 36px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.card-logo {
  width: 48px;
  height: 48px;
  background: #0052d9;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 8px 20px rgba(0, 82, 217, 0.3);
}

.card-logo-icon {
  font-size: 24px;
  color: #fff;
}

/* 后台配置了 Logo 时替换默认云图标；去掉底色，让品牌图自带背景 */
.card-logo:has(.card-logo-img) {
  background: transparent;
  box-shadow: none;
}

.card-logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 12px;
}

.header-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.card-title {
  font-size: 22px;
  font-weight: 700;
  color: #1a365d;
  margin: 0;
  letter-spacing: 0.5px;
  line-height: 1.2;
}

.card-subtitle {
  font-size: 12px;
  color: #64748b;
  margin: 0;
  line-height: 1.4;
}

/* 表单区域 */
.form-area {
  position: relative;
  z-index: 1;
}

/* 子Tab样式 */
.login-tabs {
  margin-bottom: 28px;
}

.login-tabs :deep(.t-tabs__nav) {
  border-bottom: 1px solid rgba(0, 82, 217, 0.15);
}

.login-tabs :deep(.t-tabs__nav-item) {
  font-size: 14px;
  font-weight: 500;
  color: #475569;
  padding: 10px 0;
}

.login-tabs :deep(.t-tabs__nav-item--active) {
  color: #0052d9;
  font-weight: 600;
}

.login-tabs :deep(.t-tabs__bar) {
  background: #0052d9;
  height: 2px;
  border-radius: 2px;
}

.tab-content-inner {
  padding-top: 12px;
}

/* 输入框样式 */
.input-group {
  margin-bottom: 22px;
}

/* TDesign Input - 直接在 .t-input 上设置背景和边框 */
.custom-input :deep(.t-input),
.custom-input :deep(.t-input__affix-wrapper) {
  background: rgba(255, 255, 255, 0.98) !important;
  background-color: rgba(255, 255, 255, 0.98) !important;
  border: 1px solid rgba(0, 82, 217, 0.15) !important;
  border-radius: 12px !important;
  transition: all 0.25s ease !important;
  height: 46px !important;
  box-shadow: none !important;
  -webkit-box-shadow: none !important;
}

.custom-input :deep(.t-input:hover) {
  border-color: #0052d9 !important;
  background: #ffffff !important;
  background-color: #ffffff !important;
}

.custom-input :deep(.t-input:focus),
.custom-input :deep(.t-input--focused) {
  border-color: #0052d9 !important;
  box-shadow: 0 0 0 3px rgba(0, 82, 217, 0.1) !important;
  background: #ffffff !important;
  background-color: #ffffff !important;
}

.custom-input :deep(.t-input__wrap) {
  background: transparent !important;
  background-color: transparent !important;
}

.custom-input :deep(.t-input__inner) {
  height: 44px !important;
  font-size: 14px !important;
  color: #1e293b !important;
  background: transparent !important;
  background-color: transparent !important;
}

.custom-input :deep(.t-input__inner::placeholder),
.custom-input :deep(.t-input__inner::-webkit-input-placeholder) {
  color: #64748b !important;
}

.custom-input :deep(.t-input__prefix-icon) {
  color: #64748b !important;
  font-size: 18px !important;
}

.custom-input :deep(.t-input__suffix-icon) {
  color: #64748b !important;
}

.custom-input :deep(.t-input__clear),
.custom-input :deep(.t-input__password-icon) {
  color: #64748b !important;
}

/* 验证码输入组 */
.captcha-group {
  display: flex;
  gap: 12px;
  align-items: center;
}

.captcha-group > :first-child {
  flex: 1;
  min-width: 0;
}

/* 短信验证码行：输入框 + 按钮 */
.sms-captcha-row .sms-code-input {
  flex: 1;
  min-width: 0;
}

.sms-captcha-row .send-code-btn {
  margin-left: 8px;
  min-width: 110px;
  height: 46px !important;
  flex-shrink: 0;
}

.helper-row {
  display: flex;
  justify-content: flex-end;
  margin: -12px 0 4px;
}

.helper-link {
  font-size: 12px;
  color: #0052d9;
  text-decoration: none;
}

.helper-link:hover {
  text-decoration: underline;
}

/* 发送验证码按钮 - 通用 */
.send-code-btn {
  height: 46px !important;
  border-radius: 12px !important;
  font-size: 13px !important;
  padding: 0 14px !important;
  white-space: nowrap;
}

/* 密码显示切换 */
.password-toggle {
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: color 0.2s;
}

.password-toggle:hover {
  color: #0052d9;
}

/* 主操作按钮 */
.auth-btn {
  height: 50px !important;
  font-size: 16px !important;
  font-weight: 600 !important;
  letter-spacing: 2px !important;
  border-radius: 14px !important;
  background: #0052d9 !important;
  border: none !important;
  box-shadow: 0 8px 24px rgba(0, 82, 217, 0.3);
  transition: all 0.25s !important;
  margin-top: 12px;
  color: #fff !important;
}

.auth-btn:hover {
  background: #003faa !important;
  transform: translateY(-1px);
  box-shadow: 0 12px 32px rgba(0, 82, 217, 0.4);
}

/* 第三方登录 */
.third-party-login {
  margin-top: 28px;
}

.divider-line {
  position: relative;
  text-align: center;
  margin-bottom: 20px;
}

.divider-line::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  height: 1px;
  background: rgba(0, 82, 217, 0.12);
}

.divider-text {
  position: relative;
  display: inline-block;
  padding: 0 14px;
  font-size: 12px;
  color: #94a3b8;
  background: transparent;
}

.third-party-icons {
  display: flex;
  justify-content: center;
  gap: 20px;
}

.third-party-item {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.7);
  border: 1px solid rgba(0, 82, 217, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.25s ease;
}

.third-party-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(0, 82, 217, 0.15);
}

.third-party-icon {
  font-size: 20px;
}

.third-party-icon.wechat {
  color: #07c160;
}

.third-party-icon.qq {
  color: #12b7f5;
}

.third-party-icon.alipay {
  color: #1677ff;
}

.third-party-icon.wecom {
  color: #0052d9;
}

.third-party-icon.github {
  color: #24292e;
}

/* 卡片底部版权 */
.card-footer {
  text-align: center;
  margin-top: 32px;
  padding-top: 24px;
  border-top: 1px solid rgba(0, 82, 217, 0.08);
}

.card-footer p {
  font-size: 12px;
  color: #94a3b8;
  margin: 0;
  line-height: 1.6;
}

.register-hint {
  margin-bottom: 8px !important;
  font-size: 13px !important;
}

/* ============ 右侧切换标签 ============ */
.side-tabs {
  position: fixed;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  z-index: 100;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-right: 8px;
}

.side-tab {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 18px;
  background: #fff;
  border: 1px solid rgba(0, 82, 217, 0.12);
  border-right: none;
  border-radius: 12px 0 0 12px;
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: -4px 4px 16px rgba(0, 0, 0, 0.06);
  position: relative;
}

.side-tab::after {
  content: '';
  position: absolute;
  right: -8px;
  top: 50%;
  transform: translateY(-50%);
  width: 0;
  height: 0;
  border-top: 8px solid transparent;
  border-bottom: 8px solid transparent;
  border-right: 8px solid #fff;
}

.side-tab:hover {
  background: #f0f5ff;
  transform: translateX(-4px);
}

.side-tab.active {
  background: #0052d9;
  border-color: #0052d9;
}

.side-tab.active::after {
  border-right-color: #0052d9;
}

.side-tab.active:hover {
  background: #003faa;
  border-color: #003faa;
}

.side-tab-icon {
  font-size: 16px;
  color: #0052d9;
}

.side-tab.active .side-tab-icon {
  color: #fff;
}

.side-tab-label {
  font-size: 14px;
  font-weight: 600;
  color: #334155;
}

.side-tab.active .side-tab-label {
  color: #fff;
}

/* ============ 响应式 ============ */
@media (max-width: 1024px) {
  .main-content {
    padding-right: 4%;
  }

  .login-card {
    margin: 20px 30px 20px 10px;
  }
}

@media (max-width: 768px) {
  .main-content {
    justify-content: center;
    padding: 20px;
  }

  .login-card {
    max-width: 100%;
    padding: 32px 28px 24px;
    border-radius: 24px;
    margin: 0;
    max-height: calc(100vh - 40px);
  }

  .card-title {
    font-size: 20px;
  }

  .card-logo {
    width: 42px;
    height: 42px;
  }

  .card-logo-icon {
    font-size: 22px;
  }

  .side-tabs {
    display: none;
  }

  .input-group {
    margin-left: 4px;
    margin-right: 4px;
  }
}

@media (max-width: 480px) {
  .main-content {
    padding: 12px;
  }

  .login-card {
    padding: 28px 22px 20px;
    border-radius: 20px;
  }

  .card-title {
    font-size: 18px;
  }

  .card-header {
    margin-bottom: 28px;
  }

  .input-group {
    margin-bottom: 20px;
    margin-left: 6px;
    margin-right: 6px;
  }

  .captcha-group {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .send-code-btn {
    width: 100%;
  }

  .sms-captcha-row .send-code-btn {
    margin-left: 0;
    min-width: unset;
    width: 100%;
  }
}
</style>
