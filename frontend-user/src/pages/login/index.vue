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
              <p class="card-subtitle">{{ authMode === 'login' ? '欢迎回来，请登录账户' : '创建账号，开启您的云上之旅' }}</p>
            </div>
          </div>
        </div>

        <div class="form-area">
          <!-- 登录表单：密码/短信/邮箱三通道 -->
          <t-tabs v-if="authMode === 'login'" v-model:value="activeTab" theme="normal" class="login-tabs">
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

          <!-- 注册表单：与登录共用同一张卡片，按 authMode 原地切换，不再跳独立页 -->
          <t-form
            v-else
            ref="registerFormRef"
            :data="registerForm"
            :rules="registerRules"
            :label-width="0"
            class="register-form"
            @submit="handleRegister"
          >
            <t-form-item name="username">
              <t-input
                v-model="registerForm.username"
                placeholder="请输入用户名"
                size="large"
                class="custom-input"
              >
                <template #prefix-icon>
                  <UserIcon />
                </template>
              </t-input>
            </t-form-item>

            <t-form-item name="email">
              <t-input
                v-model="registerForm.email"
                placeholder="请输入邮箱地址"
                size="large"
                class="custom-input"
              >
                <template #prefix-icon>
                  <MailIcon />
                </template>
              </t-input>
            </t-form-item>

            <!--
              邮箱验证码行：仅当 user_register 场景 otp_required=true 时渲染。
              默认策略虽为 true，但 captcha_enabled=false 时总闸未开 → 不渲染，
              与升级前「注册只填用户名/邮箱/密码」的交互保持一致。
            -->
            <t-form-item v-if="registerEmailCodeNeeded" name="emailCode">
              <div class="captcha-group sms-captcha-row">
                <t-input
                  v-model="registerForm.emailCode"
                  placeholder="请输入邮箱验证码"
                  size="large"
                  class="custom-input sms-code-input"
                  maxlength="6"
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
                  :disabled="emailCodeCountdown > 0"
                  :loading="sendingRegisterCode"
                  @click="handleSendRegisterEmailCode"
                >
                  {{ emailCodeCountdown > 0 ? `${emailCodeCountdown}s` : '获取验证码' }}
                </t-button>
              </div>
            </t-form-item>

            <t-form-item v-if="registerCaptchaNeeded" name="captchaCode">
              <div class="captcha-group">
                <t-input
                  v-model="registerForm.captchaCode"
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
                  ref="registerCaptchaRef"
                  v-model:key="registerCaptchaKey"
                  v-model:code="registerForm.captchaCode"
                  scene="user_register"
                />
              </div>
            </t-form-item>

            <t-form-item name="inviteCode">
              <t-input
                v-model="registerForm.inviteCode"
                placeholder="邀请码（选填，由推广链接自动带入）"
                size="large"
                class="custom-input"
                clearable
              >
                <template #prefix-icon>
                  <ShareIcon />
                </template>
              </t-input>
            </t-form-item>

            <t-form-item name="password">
              <t-input
                v-model="registerForm.password"
                :type="showRegPassword ? 'text' : 'password'"
                placeholder="请输入密码（至少8位）"
                size="large"
                class="custom-input"
              >
                <template #prefix-icon>
                  <LockOnIcon />
                </template>
                <template #suffix-icon>
                  <span class="password-toggle" @click="showRegPassword = !showRegPassword">
                    <BrowseIcon v-if="!showRegPassword" />
                    <BrowseOffIcon v-else />
                  </span>
                </template>
              </t-input>
            </t-form-item>

            <t-form-item name="confirmPassword">
              <t-input
                v-model="registerForm.confirmPassword"
                :type="showRegConfirm ? 'text' : 'password'"
                placeholder="请再次输入密码"
                size="large"
                class="custom-input"
              >
                <template #prefix-icon>
                  <LockOnIcon />
                </template>
                <template #suffix-icon>
                  <span class="password-toggle" @click="showRegConfirm = !showRegConfirm">
                    <BrowseIcon v-if="!showRegConfirm" />
                    <BrowseOffIcon v-else />
                  </span>
                </template>
              </t-input>
            </t-form-item>

            <div class="form-agreement">
              <t-checkbox v-model="agreed">
                我已阅读并同意
                <!--
                  法律文本只在官网门户维护（同内容多端渲染 = 多端维护，必然漂移）。
                  未配置官网地址时渲染为纯文本，不留一个点了没反应的链接。
                -->
                <a v-if="termsUrl" :href="termsUrl" target="_blank" rel="noopener">《用户条款》</a>
                <span v-else class="agreement-plain">《用户条款》</span>
                和
                <a v-if="privacyUrl" :href="privacyUrl" target="_blank" rel="noopener">《隐私政策》</a>
                <span v-else class="agreement-plain">《隐私政策》</span>
              </t-checkbox>
            </div>

            <t-button
              type="submit"
              theme="primary"
              size="large"
              block
              class="auth-btn"
              :loading="registerLoading"
              :disabled="!agreed"
            >
              注 册
            </t-button>
          </t-form>

          <t-button
            v-if="authMode === 'login'"
            theme="primary"
            size="large"
            block
            class="auth-btn"
            :loading="loginLoading"
            @click="handleLogin"
          >
            登 录
          </t-button>

          <!-- 第三方登录（仅登录-密码方式显示）：渠道来自后端「已启用」列表，
               未启用的不渲染。此前这里是一组零 @click 的装饰图标（doc104 §6.8）。 -->
          <div v-if="authMode === 'login' && activeTab === 'password' && oauthProviders.length" class="third-party-login">
            <div class="divider-line">
              <span class="divider-text">其他登录方式</span>
            </div>
            <div class="third-party-icons">
              <div
                v-for="item in oauthProviders"
                :key="item.provider"
                class="third-party-item"
                :class="{ 'third-party-item--loading': oauthLoading === item.provider }"
                :title="`${item.name}登录`"
                role="button"
                tabindex="0"
                @click="handleOAuthLogin(item.provider)"
                @keydown.enter="handleOAuthLogin(item.provider)"
              >
                <component :is="oauthIcon(item)" class="third-party-icon" :class="item.provider" />
              </div>
            </div>
          </div>
        </div>

        <div class="card-footer">
          <p class="register-hint">
            <template v-if="authMode === 'login'">还没有账号？</template>
            <template v-else>已有账号？</template>
            <a class="helper-link toggle-mode-link" @click="toggleAuthMode">
              {{ authMode === 'login' ? '立即注册' : '返回登录' }}
            </a>
          </p>
          <p>{{ brandStore.copyrightText }}</p>
        </div>
      </div>

      <!-- 右侧切换标签：登录/注册在同一张卡片内原地切换 -->
      <div class="side-tabs">
        <div class="side-tab" :class="{ active: authMode === 'login' }" @click="switchAuthMode('login')">
          <UserIcon class="side-tab-icon" />
          <span class="side-tab-label">登录</span>
        </div>
        <div class="side-tab" :class="{ active: authMode === 'register' }" @click="switchAuthMode('register')">
          <UserAddIcon class="side-tab-icon" />
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
  UserAddIcon,
  LockOnIcon,
  BrowseIcon,
  BrowseOffIcon,
  ChatMessageIcon,
  ViewListIcon,
  MailIcon,
  MobileIcon,
  ShareIcon,
  LogoWechatStrokeIcon,
  LogoQqIcon,
  LogoAlipayIcon,
  LogoWecomIcon,
  LogoGithubIcon,
} from 'tdesign-icons-vue-next'

import CaptchaImage from '@/components/verify/CaptchaImage.vue'
import LoginOTPVerifyDialog from '@/pages/login/components/LoginOTPVerifyDialog.vue'
import { sendVerifyCode } from '@/api/public'
import { getOAuthAuthorizeUrl, getOAuthProviders, type PublicOAuthProvider } from '@/api/oauth'
import { useUserStore } from '@/store'
import { useBrandStore } from '@/store/modules/brand'
import { imageRequired, loadAuthConfig, otpChannel, otpRequired } from '@/utils/captcha-resource'
import { sitePath } from '@/utils/site'

defineOptions({ name: 'UserLogin' })

import loginBg from '@/assets/images/login-user.webp'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const brandStore = useBrandStore()

/** 卡片当前形态：登录与注册共用同一张卡片，只切换表单，不再跳独立注册页。 */
const authMode = ref<'login' | 'register'>('login')
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

// —— 注册（原独立注册页并入卡片）——
const registerFormRef = ref()
const registerLoading = ref(false)
const agreed = ref(false)
const showRegPassword = ref(false)
const showRegConfirm = ref(false)
const registerCaptchaKey = ref('')
const registerCaptchaRef = ref<InstanceType<typeof CaptchaImage> | null>(null)
const sendingRegisterCode = ref(false)
const emailCodeCountdown = ref(0)

/** 协议链接指向官网门户的条款/隐私页；未配置官网地址时为空串（渲染成纯文本）。 */
const termsUrl = sitePath('/terms')
const privacyUrl = sitePath('/privacy')

const registerForm = reactive({
  username: '',
  email: '',
  inviteCode: '',
  password: '',
  confirmPassword: '',
  emailCode: '',
  captchaCode: '',
})

/** 图形码/邮箱验证码是否渲染：由 auth-config 决定，总闸关闭时都不显示。 */
const registerCaptchaNeeded = computed(() => imageRequired('user_register'))
const registerEmailCodeNeeded = computed(() => otpRequired('user_register'))

const validateConfirm = (val: string) => {
  if (val !== registerForm.password) {
    return '两次输入的密码不一致'
  }
  return true
}

// 图形码/验证码规则随策略动态增删：不渲染的行若留必填规则会永远提交不了。
const registerRules = computed(() => ({
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, message: '用户名至少3位', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码至少8位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' },
  ],
  emailCode: registerEmailCodeNeeded.value
    ? [{ required: true, message: '请输入邮箱验证码', trigger: 'blur' }]
    : [],
  captchaCode: registerCaptchaNeeded.value
    ? [{ required: true, message: '请输入图形验证码', trigger: 'blur' }]
    : [],
}))

function switchAuthMode(mode: 'login' | 'register') {
  authMode.value = mode
}

function toggleAuthMode() {
  authMode.value = authMode.value === 'login' ? 'register' : 'login'
}

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

/** 注册邮箱验证码下发（scene=user_register，服务端在公开白名单内）。 */
async function handleSendRegisterEmailCode() {
  if (!registerForm.email.trim()) {
    MessagePlugin.warning('请先输入邮箱地址')
    return
  }
  if (registerCaptchaNeeded.value && !registerForm.captchaCode.trim()) {
    MessagePlugin.warning('请先输入图形验证码')
    return
  }
  sendingRegisterCode.value = true
  try {
    const res = await sendVerifyCode({
      scene: 'user_register',
      channel: 'email',
      target: registerForm.email.trim(),
      captcha_key: registerCaptchaKey.value || undefined,
      captcha_code: registerForm.captchaCode.trim() || undefined,
    })
    MessagePlugin.success(`验证码已发送至 ${res.target_masked || registerForm.email.trim()}`)
    emailCodeCountdown.value = 60
    const timer = setInterval(() => {
      emailCodeCountdown.value -= 1
      if (emailCodeCountdown.value <= 0) clearInterval(timer)
    }, 1000)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '验证码发送失败')
  } finally {
    // 图形码一次性：无论成败都要换新图。
    registerForm.captchaCode = ''
    registerCaptchaRef.value?.refresh()
    sendingRegisterCode.value = false
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

// ========== 第三方登录（doc104 §6.8）==========
const oauthProviders = ref<PublicOAuthProvider[]>([])
const oauthLoading = ref('')

// 后端返回的 icon 是描述符里的语义名（wechat/qq/alipay），前端映射到图标组件。
// 未知渠道回落一个通用图标而不是报错：后端新增渠道时前端不该整块渲染失败。
function oauthIcon(item: PublicOAuthProvider) {
  const map: Record<string, unknown> = {
    wechat: LogoWechatStrokeIcon,
    qq: LogoQqIcon,
    alipay: LogoAlipayIcon,
    wecom: LogoWecomIcon,
    github: LogoGithubIcon,
  }
  return map[item.icon || item.provider] || LogoWechatStrokeIcon
}

async function loadOAuthProviders() {
  try {
    const { data } = await getOAuthProviders()
    oauthProviders.value = data || []
  } catch {
    // 渠道列表拉不到时静默隐藏整块：登录主路径（账号密码）必须照常可用。
    oauthProviders.value = []
  }
}

async function handleOAuthLogin(provider: string) {
  if (oauthLoading.value) return
  oauthLoading.value = provider
  try {
    // invite_code 透传给后端签进 state：推广链接进来的用户走第三方注册时
    // 邀请归属不能丢，否则返现关系断在这里。
    const inviteCode = typeof route.query.invite_code === 'string' ? route.query.invite_code : undefined
    const { data } = await getOAuthAuthorizeUrl(provider, inviteCode)
    if (!data?.authorize_url) {
      MessagePlugin.error('该登录方式暂不可用')
      return
    }
    window.location.href = data.authorize_url
  } catch (e) {
    MessagePlugin.error((e as Error)?.message || '该登录方式暂不可用')
  } finally {
    oauthLoading.value = ''
  }
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

async function handleRegister() {
  try {
    const result = await registerFormRef.value?.validate()
    if (result !== true) return

    registerLoading.value = true
    await userStore.register({
      username: registerForm.username,
      email: registerForm.email,
      password: registerForm.password,
      invite_code: registerForm.inviteCode.trim() || undefined,
      email_code: registerEmailCodeNeeded.value ? registerForm.emailCode.trim() : undefined,
      captcha_key: registerCaptchaNeeded.value ? registerCaptchaKey.value || undefined : undefined,
      captcha_code: registerCaptchaNeeded.value ? registerForm.captchaCode.trim() || undefined : undefined,
    })

    MessagePlugin.success('注册成功，请登录')
    // 原地切回登录表单并带入用户名；?redirect 继续由登录流程接管。
    authMode.value = 'login'
    loginForm.username = registerForm.username
    loginForm.password = ''
  } catch (e) {
    console.error('Register failed:', e)
    // 注册失败时图形码与邮箱验证码都已被消费，必须刷新。
    if (registerCaptchaNeeded.value) {
      registerForm.captchaCode = ''
      registerCaptchaRef.value?.refresh()
    }
    if (registerEmailCodeNeeded.value) {
      registerForm.emailCode = ''
    }
  } finally {
    registerLoading.value = false
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

// 预留：策略里 OTP 默认通道（当前仅用于展示提示，可扩展为默认 tab）。
void otpChannel

onMounted(() => {
  // 独立注册页已并入本卡片：/register 重定向到 /login?mode=register，推广邀请码随之带入。
  if (route.query.mode === 'register') authMode.value = 'register'
  if (route.query.invite_code) registerForm.inviteCode = String(route.query.invite_code)
  void loadAuthConfig()
  void loadOAuthProviders()
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

.toggle-mode-link {
  cursor: pointer;
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

/* ============ 注册表单（共用登录卡片） ============ */
/* 行距与登录表单 .input-group 的 22px 对齐，切换时高度节奏一致 */
.register-form :deep(.t-form__item) {
  margin-bottom: 22px;
}

.form-agreement {
  margin: -6px 0 8px;
  font-size: 12px;
  color: #64748b;
}

.form-agreement a {
  color: #0052d9;
  text-decoration: none;
}

.form-agreement a:hover {
  text-decoration: underline;
}

/* 官网地址未配置时协议名退化为纯文本：不留一个点了没反应的链接 */
.form-agreement .agreement-plain {
  color: #64748b;
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

  .register-form :deep(.t-form__item) {
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
