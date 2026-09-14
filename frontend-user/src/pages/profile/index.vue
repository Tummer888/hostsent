<template>
  <div class="account-page">
    <!-- ============ 基本信息 ============ -->
    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">基本信息</h3>
        <t-button variant="text" theme="primary" @click="openProfileDialog">
          <template #icon><Edit1Icon /></template>
          编辑
        </t-button>
      </div>

      <div class="basic-info">
        <t-avatar :size="48" class="basic-avatar">{{ userInitial }}</t-avatar>
        <div class="basic-name">
          <span class="basic-name__text">{{ displayName }}</span>
          <span class="basic-name__id">
            账号ID: {{ accountId }}
            <CopyIcon class="copy-icon" @click="copyText(accountId, '账号ID已复制')" />
          </span>
        </div>
        <div class="basic-field">
          <span class="basic-field__label">联系人</span>
          <span class="basic-field__value">{{ userStore.userInfo?.name || '-' }}</span>
        </div>
        <div class="basic-field">
          <span class="basic-field__label">属性</span>
          <span class="basic-field__value">{{ accountTypeLabel }}</span>
        </div>
      </div>
    </section>

    <!-- ============ 实名认证 ============ -->
    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">实名认证</h3>
        <t-link
          v-if="siteConfigured"
          theme="primary"
          hover="color"
          :href="sitePath('/help')"
          target="_blank"
          rel="noopener"
        >
          认证说明
        </t-link>
      </div>

      <t-alert v-if="!realnameOk" theme="warning" class="verify-alert">
        未完成实名认证，无法购买{{ brandStore.name }}的产品和服务，部分工单分类也无法提交。
      </t-alert>
      <t-alert v-else theme="success" class="verify-alert">
        已完成实名认证，可正常购买产品与提交全部工单分类。
      </t-alert>

      <p class="verify-desc">
        实名认证由平台运营在管理端审核。当前平台尚未开放自助提交入口，需要认证请提交工单（分类选择「授权申请」）
        并附上主体证件信息，由运营人员代为提交与审核。
      </p>

      <div class="danger-actions">
        <t-button theme="primary" @click="goTicket()">
          提交认证工单
        </t-button>
      </div>
    </section>

    <!-- ============ 安全设置 ============ -->
    <!-- 只读概览：真实的二次验证策略在「安全设置」页调整（doc91 场景矩阵），
         这里不重复实现一套开关，避免出现「两处都能改、行为不一致」。 -->
    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">安全设置</h3>
        <span class="panel-sub">验证通道与关键操作场景在独立页面配置</span>
      </div>

      <div class="security-grid">
        <div v-for="item in securityItems" :key="item.key" class="security-item">
          <span class="security-icon">
            <component :is="item.icon" size="20" />
          </span>
          <div class="security-body">
            <div class="security-title">
              <span class="security-label">{{ item.label }}</span>
              <span class="security-status" :class="`is-${item.statusType}`">
                <CheckCircleFilledIcon v-if="item.statusType === 'ok'" size="13" />
                <ErrorCircleFilledIcon v-else size="13" />
                {{ item.status }}
              </span>
            </div>
            <p class="security-desc">{{ item.desc }}</p>
            <div v-if="item.tags?.length" class="security-tags">
              <span v-for="t in item.tags" :key="t" class="security-tag">{{ t }}</span>
            </div>
          </div>
          <t-button size="small" variant="outline" @click="go(item.actionPath)">
            {{ item.actionText }}
          </t-button>
        </div>
      </div>
    </section>

    <!-- ============ 账号注销 ============ -->
    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">账号注销</h3>
      </div>

      <p class="danger-desc">
        平台尚未开放自助注销入口（注销涉及余额清算、发票与在管实例的处置，需人工确认）。
        如需注销当前{{ brandStore.name }}账号，请提交工单说明，运营核对无未结清款项与在管资源后按流程处理。
      </p>
      <div class="danger-actions">
        <t-button theme="default" @click="go('/support/tickets/create')">提交注销申请</t-button>
      </div>
    </section>

    <!-- ============ 编辑资料 ============ -->
    <t-dialog
      v-model:visible="profileVisible"
      header="编辑资料"
      width="480px"
      :confirm-loading="profileLoading"
      @confirm="handleUpdateProfile"
    >
      <t-form ref="profileFormRef" :data="profileForm" :rules="profileRules" label-align="top">
        <t-form-item label="显示名" name="name">
          <t-input v-model="profileForm.name" placeholder="请输入显示名称" />
        </t-form-item>
        <t-form-item label="邮箱" name="email">
          <t-input v-model="profileForm.email" placeholder="请输入邮箱地址" />
        </t-form-item>
        <t-form-item label="手机" name="phone">
          <t-input v-model="profileForm.phone" placeholder="请输入手机号" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- ============ 修改密码 ============ -->
    <t-dialog
      v-model:visible="passwordVisible"
      header="修改登录密码"
      width="480px"
      :confirm-loading="passwordLoading"
      @confirm="handleChangePassword"
    >
      <t-form ref="passwordFormRef" :data="passwordForm" :rules="passwordRules" label-align="top">
        <t-form-item label="当前密码" name="old_password">
          <t-input v-model="passwordForm.old_password" type="password" placeholder="请输入当前密码" />
        </t-form-item>
        <t-form-item label="新密码" name="new_password">
          <t-input v-model="passwordForm.new_password" type="password" placeholder="请输入新密码（至少6位）" />
        </t-form-item>
        <t-form-item label="确认密码" name="confirm_password">
          <t-input v-model="passwordForm.confirm_password" type="password" placeholder="请再次输入新密码" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  CheckCircleFilledIcon,
  CopyIcon,
  Edit1Icon,
  ErrorCircleFilledIcon,
  LockOnIcon,
  MailIcon,
  MobileIcon,
  SecuredIcon,
} from 'tdesign-icons-vue-next'
import type { Component } from 'vue'

import { useUserStore } from '@/store'
import { useBrandStore } from '@/store/modules/brand'
import { updateProfile, changePassword } from '@/api/auth'
import { getTicketCategories } from '@/api/support'
import { sitePath, siteUrlConfigured } from '@/utils/site'

defineOptions({ name: 'UserProfile' })

const router = useRouter()
const userStore = useUserStore()
const brandStore = useBrandStore()

// ========== 基本信息 ==========
const displayName = computed(() => userStore.displayName || '用户')
const userInitial = computed(() => (userStore.displayName || '用').slice(0, 1).toUpperCase())
const accountId = computed(() => String(userStore.userInfo?.id ?? '-'))
const accountTypeLabel = computed(() => (userStore.isSubAccount ? '子账号' : '个人'))

const siteConfigured = siteUrlConfigured

async function copyText(value: string, successMessage: string) {
  if (!value || value === '-') {
    MessagePlugin.warning('暂无可复制内容')
    return
  }
  try {
    await navigator.clipboard.writeText(value)
    MessagePlugin.success(successMessage)
  } catch {
    MessagePlugin.warning('复制失败，请手动选择内容')
  }
}

// ========== 安全设置 ==========
// 这里只做只读概览 + 跳转到真正的安全设置页：
// 二次验证策略（场景 × 通道）由 /profile/security 统一维护，用户端不该有两套开关。
interface SecurityItem {
  key: string
  label: string
  icon: Component
  status: string
  statusType: 'ok' | 'warn'
  desc: string
  tags?: string[]
  actionText: string
  actionPath: string
}

const boundPhone = computed(() => userStore.userInfo?.phone || '')
const boundEmail = computed(() => userStore.userInfo?.email || '')

function maskPhone(phone: string): string {
  if (!phone || phone.length < 7) return phone
  return `${phone.slice(0, 3)}****${phone.slice(-4)}`
}

const securityItems = computed<SecurityItem[]>(() => [
  {
    key: 'phone',
    label: '绑定手机',
    icon: MobileIcon,
    status: boundPhone.value ? '已绑定' : '未绑定',
    statusType: boundPhone.value ? 'ok' : 'warn',
    desc: boundPhone.value
      ? `${maskPhone(boundPhone.value)} 用于身份验证、信息获取、云产品相关的通知接收`
      : '用于身份验证、信息获取、云产品相关的通知接收',
    actionText: boundPhone.value ? '修改' : '绑定',
    actionPath: '/profile/security',
  },
  {
    key: 'email',
    label: '绑定邮箱',
    icon: MailIcon,
    status: boundEmail.value ? '已绑定' : '未绑定',
    statusType: boundEmail.value ? 'ok' : 'warn',
    desc: '用于产品相关的通知接收与关键操作的验证码下发',
    actionText: boundEmail.value ? '修改' : '绑定',
    actionPath: '/profile/security',
  },
  {
    key: 'protect',
    label: '关键操作保护',
    icon: SecuredIcon,
    status: '平台已启用',
    statusType: 'ok',
    desc: '修改密码、绑定手机/邮箱、发起提现、销毁实例等敏感操作会要求二次验证',
    tags: ['手机短信验证', '邮箱验证'],
    actionText: '查看',
    actionPath: '/profile/security',
  },
  {
    key: 'password',
    label: '登录密码',
    icon: LockOnIcon,
    status: '已设置',
    statusType: 'ok',
    desc: '为了您的账号安全，建议您定期更换密码',
    actionText: '修改',
    actionPath: '#password',
  },
])

// ========== 页面动作 ==========
function go(path: string) {
  if (path === '#password') {
    openPasswordDialog()
    return
  }
  router.push(path)
}

// 实名认证与账号注销都走工单（平台无自助入口），跳转前给一句解释
function goTicket(): void {
  router.push('/support/tickets/create')
}

// ========== 实名状态 ==========
const realnameOk = ref(false)

async function loadRealname() {
  try {
    const { data } = await getTicketCategories()
    realnameOk.value = data?.realname_ok ?? false
  } catch {
    realnameOk.value = false
  }
}

// ========== 编辑资料 ==========
const profileFormRef = ref()
const profileVisible = ref(false)
const profileLoading = ref(false)

const profileForm = reactive({ name: '', email: '', phone: '' })

const profileRules = {
  email: [
    { pattern: /^$|^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: '邮箱格式不正确', trigger: 'blur' },
  ],
}

function openProfileDialog() {
  profileForm.name = userStore.userInfo?.name || ''
  profileForm.email = userStore.userInfo?.email || ''
  profileForm.phone = userStore.userInfo?.phone || ''
  profileVisible.value = true
}

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
    userStore.userInfo = data
    MessagePlugin.success('资料已更新')
    profileVisible.value = false
  } catch (e: any) {
    console.error('Update profile failed:', e)
  } finally {
    profileLoading.value = false
  }
}

// ========== 修改密码 ==========
const passwordFormRef = ref()
const passwordVisible = ref(false)
const passwordLoading = ref(false)

const passwordForm = reactive({ old_password: '', new_password: '', confirm_password: '' })

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

function openPasswordDialog() {
  passwordForm.old_password = ''
  passwordForm.new_password = ''
  passwordForm.confirm_password = ''
  passwordVisible.value = true
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
    passwordVisible.value = false
  } catch (e: any) {
    console.error('Change password failed:', e)
  } finally {
    passwordLoading.value = false
  }
}

onMounted(async () => {
  if (!userStore.loaded) {
    await userStore.fetchUserInfo()
  }
  await loadRealname()
})
</script>

<style scoped>
.account-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  min-height: calc(100vh - 112px);
}

/* ========== 通用面板 ========== */
.panel {
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #eef1f5);
  border-radius: 8px;
  padding: 18px 24px 22px;
}

.panel-head {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding-bottom: 12px;
}

.panel-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.panel-sub {
  font-size: 12.5px;
  color: #94a3b8;
}

/* ========== 基本信息 ========== */
.basic-info {
  display: flex;
  align-items: center;
  gap: 40px;
  padding: 8px 0 4px;
  flex-wrap: wrap;
}

.basic-avatar {
  background: #e2e8f0 !important;
  color: #475569 !important;
  font-size: 18px;
  font-weight: 600;
  flex-shrink: 0;
}

.basic-name {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 180px;
}

.basic-name__text {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.basic-name__id {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: #94a3b8;
}

.copy-icon {
  cursor: pointer;
  color: #94a3b8;
  transition: color 0.15s ease;
}

.copy-icon:hover {
  color: var(--color-primary, #2563eb);
}

.basic-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 120px;
}

.basic-field__label {
  font-size: 12.5px;
  color: #94a3b8;
}

.basic-field__value {
  font-size: 14px;
  color: #334155;
}

/* ========== 实名认证 ========== */
.verify-alert {
  border-radius: 6px;
  margin-bottom: 16px;
}

.verify-desc {
  margin: 0 0 16px;
  font-size: 13px;
  line-height: 1.8;
  color: #64748b;
}

/* ========== 安全设置 ========== */
.security-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 40px;
}

/* 宽屏下增加列数，减少大片留白 */
@media (min-width: 1600px) {
  .security-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

.security-item {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 16px 0;
  border-bottom: 1px solid #f1f5f9;
}

.security-item:last-child {
  border-bottom: none;
}

.security-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: #f1f5f9;
  color: #64748b;
  flex-shrink: 0;
}

.security-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.security-title {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.security-label {
  font-size: 14px;
  font-weight: 500;
  color: #1e293b;
}

.security-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12.5px;
}

.security-status.is-ok {
  color: #10b981;
}

.security-status.is-warn {
  color: #f59e0b;
}

.security-desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.6;
  color: #94a3b8;
}

.security-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.security-tag {
  padding: 1px 8px;
  border: 1px solid #dbeafe;
  border-radius: 4px;
  background: #f8fbff;
  color: #2563eb;
  font-size: 12px;
  line-height: 18px;
}

/* ========== 账号注销 ========== */
.danger-desc {
  margin: 0 0 16px;
  font-size: 13px;
  line-height: 1.7;
  color: #8b95a8;
}

.danger-actions {
  display: flex;
  justify-content: flex-end;
}

/* ========== 深色模式 ========== */
.dark .panel {
  background: #141414;
  border-color: #262626;
}

.dark .panel-title,
.dark .basic-name__text,
.dark .basic-field__value,
.dark .security-label {
  color: #e5e7eb;
}

.dark .security-icon {
  background: #1f1f1f;
  color: #94a3b8;
}

.dark .security-item {
  border-bottom-color: #262626;
}

/* ========== 响应式 ========== */
@media (max-width: 1024px) {
  .security-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .basic-info {
    gap: 24px;
  }
}
</style>
