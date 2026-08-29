<template>
  <div class="profile-page">
    <!-- 用户信息卡片 -->
    <t-card class="profile-card" :bordered="false">
      <div class="user-header">
        <t-avatar :size="72" class="user-avatar">
          {{ userInitial }}
        </t-avatar>
        <div class="user-meta">
          <h2 class="display-name">{{ displayName }}</h2>
          <p class="user-username">@{{ userStore.userInfo?.username }}</p>
          <div class="user-tags">
            <t-tag theme="primary" variant="light" size="small">{{ tierLabel }}</t-tag>
            <t-tag theme="success" variant="light" size="small">已认证</t-tag>
          </div>
        </div>
      </div>
      <div class="user-grid">
        <div class="grid-item">
          <span class="item-label">邮箱</span>
          <span class="item-value">{{ userStore.userInfo?.email || '-' }}</span>
        </div>
        <div class="grid-item">
          <span class="item-label">手机</span>
          <span class="item-value">{{ userStore.userInfo?.phone || '-' }}</span>
        </div>
        <div class="grid-item">
          <span class="item-label">用户等级</span>
          <span class="item-value">{{ tierLabel }}</span>
        </div>
        <div class="grid-item">
          <span class="item-label">账户状态</span>
          <span class="item-value status-active">正常</span>
        </div>
      </div>
    </t-card>

    <!-- 编辑资料 -->
    <t-card class="section-card" :bordered="false">
      <template #title><span class="section-title">编辑资料</span></template>
      <t-form
        ref="profileFormRef"
        :data="profileForm"
        :rules="profileRules"
        class="profile-form"
        @submit="handleUpdateProfile"
      >
        <t-form-item label="显示名" name="name">
          <t-input v-model="profileForm.name" placeholder="请输入显示名称" />
        </t-form-item>
        <t-form-item label="邮箱" name="email">
          <t-input v-model="profileForm.email" placeholder="请输入邮箱地址" />
        </t-form-item>
        <t-form-item label="手机" name="phone">
          <t-input v-model="profileForm.phone" placeholder="请输入手机号" />
        </t-form-item>
        <t-form-item>
          <t-button theme="primary" type="submit" :loading="profileLoading">保存修改</t-button>
        </t-form-item>
      </t-form>
    </t-card>

    <!-- 修改密码 -->
    <t-card class="section-card" :bordered="false">
      <template #title><span class="section-title">修改密码</span></template>
      <t-form
        ref="passwordFormRef"
        :data="passwordForm"
        :rules="passwordRules"
        class="profile-form"
        @submit="handleChangePassword"
      >
        <t-form-item label="当前密码" name="old_password">
          <t-input
            v-model="passwordForm.old_password"
            type="password"
            placeholder="请输入当前密码"
          />
        </t-form-item>
        <t-form-item label="新密码" name="new_password">
          <t-input
            v-model="passwordForm.new_password"
            type="password"
            placeholder="请输入新密码（至少6位）"
          />
        </t-form-item>
        <t-form-item label="确认密码" name="confirm_password">
          <t-input
            v-model="passwordForm.confirm_password"
            type="password"
            placeholder="请再次输入新密码"
          />
        </t-form-item>
        <t-form-item>
          <t-button theme="primary" type="submit" :loading="passwordLoading">修改密码</t-button>
        </t-form-item>
      </t-form>
    </t-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useUserStore } from '@/store'
import { updateProfile, changePassword } from '@/api/auth'

defineOptions({ name: 'UserProfile' })

const userStore = useUserStore()

const profileFormRef = ref()
const passwordFormRef = ref()
const profileLoading = ref(false)
const passwordLoading = ref(false)

// ========== 用户信息 ==========
const displayName = computed(() => userStore.displayName || '用户')
const userInitial = computed(() => {
  const base = displayName.value
  return base.slice(0, 1).toUpperCase()
})
const tierLabel = computed(() => {
  const tier = userStore.userInfo?.tier
  const map: Record<string, string> = { free: '免费版', pro: '专业版', enterprise: '企业版' }
  return map[tier || ''] || tier || '免费版'
})

// ========== 编辑资料表单 ==========
const profileForm = reactive({
  name: '',
  email: '',
  phone: '',
})

const profileRules = {
  email: [
    { pattern: /^$|^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: '邮箱格式不正确', trigger: 'blur' },
  ],
}

// 初始化时填充当前用户信息
onMounted(() => {
  if (userStore.userInfo) {
    profileForm.name = userStore.userInfo.name || ''
    profileForm.email = userStore.userInfo.email || ''
    profileForm.phone = userStore.userInfo.phone || ''
  }
})

async function handleUpdateProfile() {
  const result = await profileFormRef.value.validate()
  if (result !== true) return

  profileLoading.value = true
  try {
    const { data } = await updateProfile({
      name: profileForm.name,
      email: profileForm.email,
      phone: profileForm.phone,
    })
    // 更新 store 中的用户信息
    userStore.userInfo = data
    MessagePlugin.success('资料已更新')
  } catch (e: any) {
    console.error('Update profile failed:', e)
  } finally {
    profileLoading.value = false
  }
}

// ========== 修改密码表单 ==========
const passwordForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

const passwordRules = {
  old_password: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '新密码至少6位', trigger: 'blur' },
  ],
  confirm_password: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (val: string) => val === passwordForm.new_password || '两次输入的密码不一致',
      trigger: 'blur',
    },
  ],
}

async function handleChangePassword() {
  const result = await passwordFormRef.value.validate()
  if (result !== true) return

  passwordLoading.value = true
  try {
    await changePassword({
      old_password: passwordForm.old_password,
      new_password: passwordForm.new_password,
    })
    MessagePlugin.success('密码修改成功，下次登录请使用新密码')
    passwordForm.old_password = ''
    passwordForm.new_password = ''
    passwordForm.confirm_password = ''
  } catch (e: any) {
    console.error('Change password failed:', e)
  } finally {
    passwordLoading.value = false
  }
}
</script>

<style scoped>
.profile-page {
  max-width: 720px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* ========== 用户信息卡片 ========== */
.profile-card {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 16px;
  color: #fff;
  padding: 32px;
}

.user-header {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 24px;
}

.user-avatar {
  background: rgba(255, 255, 255, 0.2) !important;
  color: #fff !important;
  font-size: 28px;
  font-weight: 700;
  border: 3px solid rgba(255, 255, 255, 0.3);
  flex-shrink: 0;
}

.display-name {
  font-size: 24px;
  font-weight: 700;
  margin: 0 0 4px;
}

.user-username {
  font-size: 14px;
  opacity: 0.8;
  margin: 0 0 8px;
}

.user-tags {
  display: flex;
  gap: 8px;
}

.user-tags :deep(.t-tag) {
  background: rgba(255, 255, 255, 0.15) !important;
  color: #fff !important;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.user-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  padding-top: 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.15);
}

.grid-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.item-label {
  font-size: 12px;
  opacity: 0.7;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.item-value {
  font-size: 15px;
  font-weight: 500;
}

.status-active {
  color: #a0f0b0;
}

/* ========== 通用卡片 ========== */
.section-card {
  border-radius: 16px;
  border: 1px solid #e2e8f0;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  color: #1e293b;
}

.profile-form {
  max-width: 480px;
  margin-top: 8px;
}

.profile-form :deep(.t-form__label) {
  font-weight: 500;
  color: #475569;
}

@media (max-width: 768px) {
  .profile-card {
    padding: 24px 20px;
  }

  .user-header {
    flex-direction: column;
    text-align: center;
  }

  .user-tags {
    justify-content: center;
  }

  .user-grid {
    grid-template-columns: 1fr;
  }

  .profile-form {
    max-width: 100%;
  }
}
</style>