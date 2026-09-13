<template>
  <div class="forgot-page">
    <div class="page-bg" :style="{ backgroundImage: `url(${loginBg})` }"></div>

    <div class="forgot-card">
      <div class="card-header">
        <div class="card-logo">
          <img v-if="brandStore.logo" class="card-logo-img" :src="brandStore.logo" :alt="brandStore.name" />
          <CloudIcon v-else class="card-logo-icon" />
        </div>
        <div class="header-text">
          <h2 class="card-title">找回密码</h2>
          <p class="card-subtitle">
            {{ step === 1 ? '验证账号后，我们会向绑定的邮箱或手机发送验证码' : '请输入收到的验证码并设置新密码' }}
          </p>
        </div>
      </div>

      <!-- 步骤条：两步，让用户明确知道当前进度 -->
      <t-steps :current="step - 1" class="forgot-steps">
        <t-step-item title="验证账号" />
        <t-step-item title="重置密码" />
      </t-steps>

      <!-- 第一步：申请下发验证码 -->
      <div v-if="step === 1" class="form-area">
        <div class="input-group">
          <t-input
            v-model="form.account"
            placeholder="请输入用户名 / 邮箱 / 手机号"
            size="large"
            class="custom-input"
          >
            <template #prefix-icon>
              <UserIcon />
            </template>
          </t-input>
        </div>

        <div v-if="captchaNeeded" class="input-group captcha-group">
          <t-input
            v-model="form.captchaCode"
            placeholder="请输入图形验证码"
            size="large"
            class="custom-input"
            maxlength="5"
            @enter="handleApply"
          >
            <template #prefix-icon>
              <ViewListIcon />
            </template>
          </t-input>
          <CaptchaImage
            ref="captchaRef"
            v-model:key="captchaKey"
            v-model:code="form.captchaCode"
            scene="password_reset"
          />
        </div>

        <t-button
          theme="primary"
          size="large"
          block
          class="submit-btn"
          :loading="applying"
          @click="handleApply"
        >
          发送验证码
        </t-button>
      </div>

      <!-- 第二步：校验验证码 + 设置新密码 -->
      <div v-else class="form-area">
        <div class="account-tip">
          验证码已发送至账号绑定的邮箱或手机
          <router-link to="/login" class="helper-link">返回登录</router-link>
        </div>

        <div class="input-group">
          <t-input
            v-model="form.code"
            placeholder="请输入收到的验证码"
            size="large"
            class="custom-input"
            maxlength="6"
          >
            <template #prefix-icon>
              <ChatMessageIcon />
            </template>
            <template #suffix-icon>
              <t-button
                variant="text"
                theme="primary"
                size="small"
                :disabled="countdown > 0"
                :loading="applying"
                @click="handleApply"
              >
                {{ countdown > 0 ? `${countdown}s` : '重新发送' }}
              </t-button>
            </template>
          </t-input>
        </div>

        <div class="input-group">
          <t-input
            v-model="form.newPassword"
            type="password"
            placeholder="请输入新密码"
            size="large"
            class="custom-input"
          >
            <template #prefix-icon>
              <LockOnIcon />
            </template>
          </t-input>
        </div>

        <div class="input-group">
          <t-input
            v-model="form.confirmPassword"
            type="password"
            placeholder="请再次输入新密码"
            size="large"
            class="custom-input"
            @enter="handleReset"
          >
            <template #prefix-icon>
              <LockOnIcon />
            </template>
          </t-input>
        </div>

        <t-button
          theme="primary"
          size="large"
          block
          class="submit-btn"
          :loading="resetting"
          @click="handleReset"
        >
          重置密码
        </t-button>
      </div>

      <div class="card-footer">
        <p>{{ brandStore.copyrightText }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  CloudIcon,
  UserIcon,
  LockOnIcon,
  ChatMessageIcon,
  ViewListIcon,
} from 'tdesign-icons-vue-next'

import CaptchaImage from '@/components/verify/CaptchaImage.vue'
import { forgotPassword, resetPassword } from '@/api/auth'
import { useBrandStore } from '@/store/modules/brand'
import { imageRequired, loadAuthConfig } from '@/utils/captcha-resource'

defineOptions({ name: 'ForgotPassword' })

import loginBg from '@/assets/images/login-user.webp'

const router = useRouter()
const brandStore = useBrandStore()

const step = ref<1 | 2>(1)
const applying = ref(false)
const resetting = ref(false)
const countdown = ref(0)
const captchaKey = ref('')
const captchaRef = ref<InstanceType<typeof CaptchaImage> | null>(null)

const form = reactive({
  account: '',
  captchaCode: '',
  code: '',
  newPassword: '',
  confirmPassword: '',
})

const captchaNeeded = computed(() => imageRequired('password_reset'))

function startCountdown() {
  countdown.value = 60
  const timer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) clearInterval(timer)
  }, 1000)
}

/** 第一步：下发验证码。后端恒定返回 sent=true（防账号枚举），不回显目标。 */
async function handleApply() {
  if (!form.account.trim()) {
    MessagePlugin.warning('请输入账号')
    return
  }
  if (captchaNeeded.value && !form.captchaCode.trim()) {
    MessagePlugin.warning('请输入图形验证码')
    return
  }
  applying.value = true
  try {
    await forgotPassword({
      account: form.account.trim(),
      captcha_key: captchaKey.value || undefined,
      captcha_code: form.captchaCode.trim() || undefined,
    })
    MessagePlugin.success('若账号存在，验证码已发送，请查收')
    step.value = 2
    startCountdown()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '发送失败，请稍后重试')
  } finally {
    // 图形码一次性：消费后必须换新图。
    form.captchaCode = ''
    captchaRef.value?.refresh()
    applying.value = false
  }
}

/** 第二步：校验验证码并写入新密码（后端会撤销历史会话）。 */
async function handleReset() {
  if (!form.code.trim()) {
    MessagePlugin.warning('请输入验证码')
    return
  }
  if (!form.newPassword) {
    MessagePlugin.warning('请输入新密码')
    return
  }
  if (form.newPassword !== form.confirmPassword) {
    MessagePlugin.warning('两次输入的密码不一致')
    return
  }
  resetting.value = true
  try {
    await resetPassword({
      account: form.account.trim(),
      code: form.code.trim(),
      new_password: form.newPassword,
    })
    MessagePlugin.success('密码已重置，请使用新密码登录')
    router.replace('/login')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '重置失败，请稍后重试')
  } finally {
    resetting.value = false
  }
}

onMounted(() => {
  void loadAuthConfig()
})
</script>

<style scoped>
.forgot-page {
  position: relative;
  width: 100%;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
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

.forgot-card {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 460px;
  background:
    radial-gradient(ellipse at 0% 0%, rgba(255, 182, 193, 0.5) 0%, transparent 55%),
    radial-gradient(ellipse at 100% 0%, rgba(135, 206, 250, 0.5) 0%, transparent 55%),
    rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(50px) saturate(1.6);
  -webkit-backdrop-filter: blur(50px) saturate(1.6);
  border-radius: 24px;
  border: 1px solid rgba(255, 255, 255, 0.5);
  box-shadow: 0 24px 56px rgba(30, 64, 175, 0.12);
  padding: 40px 36px 28px;
  margin: 24px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 24px;
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
}

.card-logo:has(.card-logo-img) {
  background: transparent;
}

.card-logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 12px;
}

.card-logo-icon {
  font-size: 24px;
  color: #fff;
}

.header-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.card-title {
  font-size: 20px;
  font-weight: 700;
  color: #1a365d;
  margin: 0;
}

.card-subtitle {
  font-size: 12px;
  color: #64748b;
  margin: 0;
  line-height: 1.5;
}

.forgot-steps {
  margin-bottom: 26px;
}

.form-area {
  position: relative;
  z-index: 1;
}

.input-group {
  margin-bottom: 22px;
}

.custom-input :deep(.t-input) {
  background: rgba(255, 255, 255, 0.98) !important;
  border: 1px solid rgba(0, 82, 217, 0.15) !important;
  border-radius: 12px !important;
  height: 46px !important;
}

.custom-input :deep(.t-input--focused) {
  border-color: #0052d9 !important;
  box-shadow: 0 0 0 3px rgba(0, 82, 217, 0.1) !important;
}

.custom-input :deep(.t-input__inner) {
  height: 44px !important;
  font-size: 14px !important;
}

.captcha-group {
  display: flex;
  gap: 12px;
  align-items: center;
}

.captcha-group > :first-child {
  flex: 1;
  min-width: 0;
}

.account-tip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 18px;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(0, 82, 217, 0.06);
  font-size: 12px;
  color: #475569;
}

.helper-link {
  color: #0052d9;
  text-decoration: none;
  white-space: nowrap;
}

.helper-link:hover {
  text-decoration: underline;
}

.submit-btn {
  height: 50px !important;
  font-size: 16px !important;
  font-weight: 600 !important;
  border-radius: 14px !important;
  background: #0052d9 !important;
  border: none !important;
  color: #fff !important;
  box-shadow: 0 8px 24px rgba(0, 82, 217, 0.3);
}

.submit-btn:hover {
  background: #003faa !important;
}

.card-footer {
  text-align: center;
  margin-top: 28px;
  padding-top: 20px;
  border-top: 1px solid rgba(0, 82, 217, 0.08);
}

.card-footer p {
  font-size: 12px;
  color: #94a3b8;
  margin: 0;
}
</style>
