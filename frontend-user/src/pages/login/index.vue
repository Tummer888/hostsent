<template>
  <div class="login-page">
    <div class="page-bg" :style="{ backgroundImage: `url(${loginBg})` }"></div>

    <!-- 主内容区：卡片 + 侧边栏 -->
    <div class="main-content">
      <div class="login-card">
        <div class="card-header">
          <div class="header-left">
            <div class="card-logo">
              <CloudIcon class="card-logo-icon" />
            </div>
            <div class="header-text">
              <h2 class="card-title">宿派云控</h2>
              <p class="card-subtitle">{{ authTab === 'login' ? '欢迎回来，请登录账户' : '创建新账户，开启云端之旅' }}</p>
            </div>
          </div>
        </div>

        <!-- 登录表单 -->
        <div v-if="authTab === 'login'" class="form-area">
          <t-tabs v-model:value="activeTab" theme="normal" class="login-tabs">
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
                <div class="input-group captcha-group">
                  <t-input
                    v-model="loginForm.captcha"
                    placeholder="请输入数字验证码"
                    size="large"
                    class="custom-input"
                  >
                    <template #prefix-icon>
                      <ViewListIcon />
                    </template>
                  </t-input>
                  <div class="captcha-image" title="点击刷新验证码" @click="refreshCaptcha">
                    <span class="captcha-text">{{ captchaCode }}</span>
                  </div>
                </div>
              </div>
            </t-tab-panel>

            <t-tab-panel value="sms" label="短信登录">
              <div class="tab-content-inner">
                <div class="input-group">
                  <t-input
                    v-model="loginForm.phone"
                    placeholder="请输入登录名/手机号"
                    size="large"
                    class="custom-input"
                  >
                    <template #prefix-icon>
                      <UserIcon />
                    </template>
                  </t-input>
                </div>
                <div class="input-group captcha-group sms-captcha-row">
                  <t-input
                    v-model="loginForm.smsCode"
                    placeholder="请输入短信验证码"
                    size="large"
                    class="custom-input sms-code-input"
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
                    :disabled="countdown > 0"
                    @click="handleSendCode"
                  >
                    {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
                  </t-button>
                </div>
              </div>
            </t-tab-panel>
          </t-tabs>

          <t-button theme="primary" size="large" block class="auth-btn">
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

        <!-- 注册表单 -->
        <div v-else class="form-area">
          <div class="input-group">
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
          </div>
          <div class="input-group">
            <t-input
              v-model="registerForm.email"
              placeholder="请输入邮箱"
              size="large"
              class="custom-input"
            >
              <template #prefix-icon>
                <MailIcon />
              </template>
            </t-input>
          </div>
          <div class="input-group">
            <t-input
              v-model="registerForm.phone"
              placeholder="请输入手机号"
              size="large"
              class="custom-input"
            >
              <template #prefix-icon>
                <MobileIcon />
              </template>
            </t-input>
          </div>
          <div class="input-group">
            <t-input
              v-model="registerForm.password"
              :type="showRegisterPassword ? 'text' : 'password'"
              placeholder="请设置登录密码"
              size="large"
              class="custom-input"
            >
              <template #prefix-icon>
                <LockOnIcon />
              </template>
              <template #suffix-icon>
                <span class="password-toggle" @click="showRegisterPassword = !showRegisterPassword">
                  <BrowseIcon v-if="!showRegisterPassword" />
                  <BrowseOffIcon v-else />
                </span>
              </template>
            </t-input>
          </div>
          <div class="input-group">
            <t-input
              v-model="registerForm.confirmPassword"
              :type="showConfirmPassword ? 'text' : 'password'"
              placeholder="请确认密码"
              size="large"
              class="custom-input"
            >
              <template #prefix-icon>
                <LockOnIcon />
              </template>
              <template #suffix-icon>
                <span class="password-toggle" @click="showConfirmPassword = !showConfirmPassword">
                  <BrowseIcon v-if="!showConfirmPassword" />
                  <BrowseOffIcon v-else />
                </span>
              </template>
            </t-input>
          </div>
          <div class="input-group captcha-group">
            <t-input
              v-model="registerForm.captcha"
              placeholder="请输入验证码"
              size="large"
              class="custom-input"
            >
              <template #prefix-icon>
                <ViewListIcon />
              </template>
            </t-input>
            <div class="captcha-image" title="点击刷新验证码" @click="refreshCaptcha">
              <span class="captcha-text">{{ captchaCode }}</span>
            </div>
          </div>

          <t-button theme="primary" size="large" block class="auth-btn">
            注 册
          </t-button>
        </div>

        <div class="card-footer">
          <p>Copyright © 2024 宿派云控 HostSent. All Rights Reserved.</p>
        </div>

        <!-- 移动端底部切换 -->
        <div class="mobile-switch">
          <span
            class="mobile-switch-item"
            :class="{ active: authTab === 'login' }"
            @click="authTab = 'login'"
          >登录</span>
          <span class="mobile-switch-divider">|</span>
          <span
            class="mobile-switch-item"
            :class="{ active: authTab === 'register' }"
            @click="authTab = 'register'"
          >注册</span>
        </div>
      </div>

      <!-- 右侧切换标签 -->
      <div class="side-tabs">
        <div
          class="side-tab"
          :class="{ active: authTab === 'login' }"
          @click="authTab = 'login'"
        >
          <UserIcon class="side-tab-icon" />
          <span class="side-tab-label">登录</span>
        </div>
        <div
          class="side-tab"
          :class="{ active: authTab === 'register' }"
          @click="authTab = 'register'"
        >
          <UserIcon class="side-tab-icon" />
          <span class="side-tab-label">注册</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
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

defineOptions({ name: 'UserLogin' })

import loginBg from '@/assets/images/login-user.webp'

const authTab = ref<'login' | 'register'>('login')
const activeTab = ref('password')
const showPassword = ref(false)
const showRegisterPassword = ref(false)
const showConfirmPassword = ref(false)
const countdown = ref(0)
const captchaCode = ref('7044')

const loginForm = reactive({
  username: '',
  password: '',
  captcha: '',
  phone: '',
  smsCode: '',
})

const registerForm = reactive({
  username: '',
  email: '',
  phone: '',
  password: '',
  confirmPassword: '',
  captcha: '',
})

function handleSendCode() {
  countdown.value = 60
  const timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      clearInterval(timer)
    }
  }, 1000)
}

function refreshCaptcha() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'
  let code = ''
  for (let i = 0; i < 4; i++) {
    code += chars[Math.floor(Math.random() * chars.length)]
  }
  captchaCode.value = code
}
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
  margin-bottom: 32px;
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
  margin-bottom: 24px;
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

.captcha-image {
  width: 100px;
  height: 46px;
  border-radius: 12px;
  background: linear-gradient(135deg, #f0f5ff 0%, #e0ecff 50%, #e8f0ff 100%);
  border: 1px solid rgba(0, 82, 217, 0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  user-select: none;
  transition: all 0.25s ease;
  flex-shrink: 0;
  position: relative;
  overflow: hidden;
}

.captcha-image::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    repeating-linear-gradient(
      45deg,
      transparent,
      transparent 4px,
      rgba(0, 82, 217, 0.03) 4px,
      rgba(0, 82, 217, 0.03) 8px
    );
  pointer-events: none;
}

.captcha-image:hover {
  border-color: #0052d9;
  transform: scale(1.02);
}

.captcha-text {
  font-family: 'Courier New', 'Consolas', monospace;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 3px;
  color: #0052d9;
  text-decoration: line-through;
  text-decoration-color: rgba(0, 82, 217, 0.25);
  position: relative;
  z-index: 1;
  transform: skewX(-5deg);
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

/* 移动端底部切换 */
.mobile-switch {
  display: none;
  justify-content: center;
  align-items: center;
  gap: 12px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid rgba(0, 82, 217, 0.08);
}

.mobile-switch-item {
  font-size: 14px;
  font-weight: 500;
  color: #64748b;
  cursor: pointer;
  padding: 4px 8px;
  transition: all 0.2s;
}

.mobile-switch-item.active {
  color: #0052d9;
  font-weight: 600;
}

.mobile-switch-divider {
  color: #cbd5e1;
  font-size: 14px;
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

  .mobile-switch {
    display: flex;
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
    gap: 12px;
  }

  .captcha-image {
    width: 100%;
    height: 44px;
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
