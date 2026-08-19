<template>
  <div class="register-page">
    <div class="register-brand">
      <div class="brand-content">
        <div class="brand-logo">
          <span class="logo-text">H</span>
        </div>
        <h1 class="brand-title">加入宿派云控</h1>
        <p class="brand-desc">创建账号，开启您的云上之旅</p>
        
        <div class="brand-features">
          <div class="feature-item">
            <div class="feature-icon"><RocketIcon /></div>
            <div class="feature-text">
              <h4>快速开通</h4>
              <p>简单几步，快速创建账号</p>
            </div>
          </div>
          <div class="feature-item">
            <div class="feature-icon"><SettingIcon /></div>
            <div class="feature-text">
              <h4>灵活便捷</h4>
              <p>随时随地管理您的云资源</p>
            </div>
          </div>
          <div class="feature-item">
            <div class="feature-icon"><ServiceIcon /></div>
            <div class="feature-text">
              <h4>专属服务</h4>
              <p>专业团队为您提供支持</p>
            </div>
          </div>
        </div>
      </div>
    </div>
    
    <div class="register-form-wrapper">
      <div class="register-form-container">
        <div class="form-header">
          <h2>创建您的账号</h2>
          <p>已有账号？ <router-link to="/login">立即登录</router-link></p>
        </div>
        
        <t-form
          ref="formRef"
          :data="formData"
          :rules="rules"
          class="register-form"
          @submit="handleRegister"
        >
          <t-form-item label="用户名" name="username">
            <t-input
              v-model="formData.username"
              placeholder="请输入用户名"
              size="large"
            >
              <template #prefix-icon>
                <UserIcon />
              </template>
            </t-input>
          </t-form-item>
          
          <t-form-item label="邮箱" name="email">
            <t-input
              v-model="formData.email"
              placeholder="请输入邮箱地址"
              size="large"
            >
              <template #prefix-icon>
                <MailIcon />
              </template>
            </t-input>
          </t-form-item>
          
          <t-form-item label="密码" name="password">
            <t-input
              v-model="formData.password"
              type="password"
              placeholder="请输入密码（至少8位）"
              size="large"
            >
              <template #prefix-icon>
                <LockOnIcon />
              </template>
            </t-input>
          </t-form-item>
          
          <t-form-item label="确认密码" name="confirmPassword">
            <t-input
              v-model="formData.confirmPassword"
              type="password"
              placeholder="请再次输入密码"
              size="large"
            >
              <template #prefix-icon>
                <LockOnIcon />
              </template>
            </t-input>
          </t-form-item>
          
          <div class="form-agreement">
            <t-checkbox v-model="agreed">
              我已阅读并同意 <a href="#">《服务协议》</a> 和 <a href="#">《隐私政策》</a>
            </t-checkbox>
          </div>
          
          <t-button
            type="submit"
            theme="primary"
            size="large"
            block
            :loading="loading"
            :disabled="!agreed"
            class="register-button"
          >
            注册
          </t-button>
        </t-form>
        
        <div class="divider">
          <span>其他注册方式</span>
        </div>
        
        <div class="social-login">
          <div class="social-btn social-btn--wechat" title="微信" @click="handleSocialLogin('wechat')">
            <LogoWechatStrokeIcon class="social-icon" />
          </div>
          <div class="social-btn social-btn--qq" title="QQ" @click="handleSocialLogin('qq')">
            <LogoQqIcon class="social-icon" />
          </div>
          <div class="social-btn social-btn--apple" title="Apple" @click="handleSocialLogin('apple')">
            <LogoAppleIcon class="social-icon" />
          </div>
          <div class="social-btn social-btn--google" title="Google" @click="handleSocialLogin('google')">
            <LogoChromeIcon class="social-icon" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  UserIcon,
  MailIcon,
  LockOnIcon,
  RocketIcon,
  SettingIcon,
  ServiceIcon,
  LogoWechatStrokeIcon,
  LogoQqIcon,
  LogoAppleIcon,
  LogoChromeIcon,
} from 'tdesign-icons-vue-next'

import { useUserStore } from '@/store'

defineOptions({ name: 'UserRegister' })

const router = useRouter()
const userStore = useUserStore()

const formRef = ref()
const loading = ref(false)
const agreed = ref(false)

const formData = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
})

const validateConfirm = (val: string) => {
  if (val !== formData.password) {
    return '两次输入的密码不一致'
  }
  return true
}

const rules = {
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
}

async function handleRegister() {
  try {
    const result = await formRef.value.validate()
    if (result !== true) return
    
    loading.value = true
    await userStore.register({
      username: formData.username,
      email: formData.email,
      password: formData.password,
    })
    
    MessagePlugin.success('注册成功，请登录')
    router.replace('/login')
  } catch (e: any) {
    console.error('Register failed:', e)
  } finally {
    loading.value = false
  }
}

function handleSocialLogin(provider: string) {
  const names: Record<string, string> = {
    wechat: '微信',
    qq: 'QQ',
    apple: 'Apple',
    google: 'Google',
  }
  MessagePlugin.info(`${names[provider] || provider}登录功能即将开放`)
}
</script>

<style scoped>
.register-page {
  display: flex;
  min-height: 100vh;
  background: linear-gradient(135deg, #f5f7fa 0%, #e4e8f0 100%);
}

.register-brand {
  flex: 1;
  background: linear-gradient(135deg, #1E40AF 0%, #2563EB 50%, #3B82F6 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  position: relative;
  overflow: hidden;
}

.brand-content {
  color: #fff;
  max-width: 400px;
}

.brand-logo {
  width: 56px;
  height: 56px;
  background: rgba(255, 255, 255, 0.2);
  backdrop-filter: blur(10px);
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 24px;
  border: 1px solid rgba(255, 255, 255, 0.3);
}

.logo-text {
  font-size: 28px;
  font-weight: 700;
  color: #fff;
}

.brand-title {
  font-size: 32px;
  font-weight: 700;
  margin-bottom: 8px;
}

.brand-desc {
  font-size: 14px;
  opacity: 0.8;
  margin-bottom: 40px;
}

.brand-features {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.feature-item {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.feature-icon {
  width: 40px;
  height: 40px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.feature-icon :deep(svg) {
  font-size: 20px;
  color: #fff;
}

.feature-text h4 {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 4px;
}

.feature-text p {
  font-size: 13px;
  opacity: 0.75;
}

.register-form-wrapper {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.register-form-container {
  width: 100%;
  max-width: 420px;
}

.form-header {
  text-align: center;
  margin-bottom: 32px;
}

.form-header h2 {
  font-size: 28px;
  font-weight: 700;
  color: #1E293B;
  margin-bottom: 8px;
}

.form-header p {
  font-size: 14px;
  color: #64748B;
}

.form-header a {
  color: #2563EB;
  font-weight: 500;
}

.register-form :deep(.t-form__item) {
  margin-bottom: 16px;
}

.register-form :deep(.t-form__item-label) {
  display: none;
}

.register-form :deep(.t-input) {
  padding: 0;
}

.register-form :deep(.t-input) :deep(.t-input__inner) {
  height: 44px;
}

.form-agreement {
  margin-bottom: 20px;
  font-size: 13px;
  color: #64748B;
}

.form-agreement a {
  color: #2563EB;
}

.register-button {
  height: 48px;
  font-size: 16px;
  font-weight: 600;
  background: linear-gradient(135deg, #2563EB 0%, #3B82F6 100%);
  border: none;
  border-radius: 10px;
}

.register-button:hover:not(:disabled) {
  background: linear-gradient(135deg, #1D4ED8 0%, #2563EB 100%);
  transform: translateY(-1px);
  box-shadow: 0 8px 25px rgba(37, 99, 235, 0.3);
}

@media (max-width: 768px) {
  .register-page {
    flex-direction: column;
  }
  
  .register-brand {
    padding: 32px 24px;
  }

  .brand-content {
    max-width: 100%;
  }

  .brand-title {
    font-size: 24px;
  }

  .brand-desc {
    font-size: 13px;
    margin-bottom: 24px;
  }

  .brand-features {
    gap: 14px;
  }

  .feature-item {
    gap: 12px;
  }

  .feature-icon {
    width: 36px;
    height: 36px;
  }

  .feature-text h4 {
    font-size: 14px;
  }

  .feature-text p {
    font-size: 12px;
  }
  
  .register-form-wrapper {
    padding: 24px 20px;
  }

  .form-header {
    margin-bottom: 24px;
  }

  .form-header h2 {
    font-size: 24px;
  }
}

@media (max-width: 480px) {
  .register-brand {
    padding: 24px 20px;
  }

  .brand-features {
    display: none;
  }

  .register-form-container {
    max-width: 100%;
  }

  .form-agreement {
    font-size: 12px;
  }
}

.divider {
  display: flex;
  align-items: center;
  margin: 20px 0 16px;
  font-size: 12px;
  color: #94A3B8;
}

.divider::before,
.divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: #E2E8F0;
}

.divider span {
  padding: 0 16px;
  white-space: nowrap;
}

.social-login {
  display: flex;
  justify-content: center;
  gap: 16px;
  margin-bottom: 8px;
}

.social-btn {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid #E2E8F0;
  background: #fff;
}

.social-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.social-icon {
  font-size: 22px;
  display: flex;
}

.social-icon svg {
  width: 22px;
  height: 22px;
}

.social-btn--wechat {
  color: #07C160;
}

.social-btn--qq {
  color: #12B7F5;
}

.social-btn--apple {
  color: #000;
}

.social-btn--google {
  color: #EA4335;
}

@media (max-width: 480px) {
  .social-login {
    gap: 12px;
  }

  .social-btn {
    width: 40px;
    height: 40px;
  }

  .social-icon {
    font-size: 20px;
  }

  .social-icon svg {
    width: 20px;
    height: 20px;
  }
}
</style>
