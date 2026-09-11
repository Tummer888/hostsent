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
        <span class="panel-sub">使用企业的账号请注意使用个人实名，以免后续人员变动导致账号管理争议</span>
      </div>

      <t-alert theme="warning" class="verify-alert">
        未完成实名认证，无法正常购买宿派云控的产品和服务
      </t-alert>

      <div class="verify-grid">
        <button class="verify-card" @click="onDevelop('企业认证')">
          <span class="verify-icon verify-icon--blue"><BuildingIcon size="30" /></span>
          <div class="verify-body">
            <strong>企业认证</strong>
            <span>适用于企业、事业单位等各类组织</span>
            <span>可享受企业专属优惠权益</span>
          </div>
        </button>

        <button class="verify-card" @click="onDevelop('个人认证')">
          <span class="verify-badge">推荐</span>
          <span class="verify-icon verify-icon--green"><UserIcon size="30" /></span>
          <div class="verify-body">
            <strong>个人认证</strong>
            <span>适用于个人开发者</span>
            <span>完成认证可购买全部产品，享更多专项福利</span>
          </div>
        </button>
      </div>
    </section>

    <!-- ============ 安全设置 ============ -->
    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">安全设置</h3>
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
          <t-button size="small" variant="outline" @click="item.action()">
            {{ item.actionText }}
          </t-button>
        </div>
      </div>
    </section>

    <!-- ============ 第三方登录 ============ -->
    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">第三方登录</h3>
      </div>

      <div class="third-grid">
        <div v-for="item in thirdPartyItems" :key="item.key" class="security-item">
          <span class="security-icon security-icon--third">
            <component :is="item.icon" size="20" />
          </span>
          <div class="security-body">
            <div class="security-title">
              <span class="security-label">{{ item.label }}</span>
              <span class="security-status" :class="`is-${item.bound ? 'ok' : 'warn'}`">
                <CheckCircleFilledIcon v-if="item.bound" size="13" />
                <ErrorCircleFilledIcon v-else size="13" />
                {{ item.bound ? '已绑定' : '未绑定' }}
              </span>
            </div>
            <p class="security-desc">{{ item.desc }}</p>
          </div>
          <t-button size="small" variant="outline" @click="onDevelop(item.label)">
            {{ item.bound ? '修改' : '绑定' }}
          </t-button>
        </div>
      </div>
    </section>

    <!-- ============ 公众号通知 ============ -->
    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">公众号通知</h3>
      </div>

      <div class="security-item">
        <span class="security-icon"><NotificationIcon size="20" /></span>
        <div class="security-body">
          <div class="security-title">
            <span class="security-label">开启公众号通知</span>
            <span class="security-status is-warn">
              <ErrorCircleFilledIcon size="13" />
              未绑定
            </span>
          </div>
          <p class="security-desc">
            当前用户号与公众号绑定后，可前往消息中心，将当前用户设置为消息接收人，即可使用公众号接收消息通知。
          </p>
        </div>
        <t-button size="small" variant="outline" @click="onDevelop('公众号通知')">绑定</t-button>
      </div>
    </section>

    <!-- ============ 账号注销 ============ -->
    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">账号注销</h3>
        <t-link theme="primary" hover="color" @click="onDevelop('注销帮助文档')">帮助文档</t-link>
      </div>

      <p class="danger-desc">
        您可以在此注销当前宿派云控账号。账号注销成功后，当前账号内的所有服务将不可用。除法律法规另有规定外，当前账号内的信息、数据将被删除，且无法恢复。
      </p>
      <div class="danger-actions">
        <t-button theme="danger" @click="handleCloseAccount">注销</t-button>
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
import { MessagePlugin } from 'tdesign-vue-next'
import {
  BuildingIcon,
  CheckCircleFilledIcon,
  CopyIcon,
  Edit1Icon,
  ErrorCircleFilledIcon,
  FingerprintIcon,
  LockOnIcon,
  LogoQqIcon,
  LogoTwitterIcon,
  LogoWechatStrokeIcon,
  MailIcon,
  MobileIcon,
  NotificationIcon,
  SecuredIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import type { Component } from 'vue'

import { useUserStore } from '@/store'
import { updateProfile, changePassword } from '@/api/auth'

defineOptions({ name: 'UserProfile' })

const userStore = useUserStore()

// ========== 基本信息 ==========
const displayName = computed(() => userStore.displayName || '用户')
const userInitial = computed(() => (userStore.displayName || '用').slice(0, 1).toUpperCase())
const accountId = computed(() => String(userStore.userInfo?.id ?? '-'))
const accountTypeLabel = computed(() => (userStore.isSubAccount ? '子账号' : '个人'))

function onDevelop(name: string) {
  MessagePlugin.info(`${name}功能开发中`)
}

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
interface SecurityItem {
  key: string
  label: string
  icon: Component
  status: string
  statusType: 'ok' | 'warn'
  desc: string
  tags?: string[]
  actionText: string
  action: () => void
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
    action: () => openProfileDialog(),
  },
  {
    key: 'mfa',
    label: '虚拟MFA设备',
    icon: FingerprintIcon,
    status: '未绑定',
    statusType: 'warn',
    desc: '绑定基于 TOTP 的虚拟 MFA 设备后，可用于身份的二次验证',
    actionText: '绑定',
    action: () => onDevelop('虚拟MFA设备'),
  },
  {
    key: 'email',
    label: '绑定邮箱',
    icon: MailIcon,
    status: boundEmail.value ? '已绑定' : '未绑定',
    statusType: boundEmail.value ? 'ok' : 'warn',
    desc: '用于产品相关的通知接收',
    actionText: boundEmail.value ? '修改' : '绑定',
    action: () => openProfileDialog(),
  },
  {
    key: 'login-protect',
    label: '登录保护',
    icon: SecuredIcon,
    status: '已开启',
    statusType: 'ok',
    desc: '开启登录保护，登录时将进行身份二次验证',
    tags: ['手机短信验证', '虚拟MFA验证'],
    actionText: '修改',
    action: () => onDevelop('登录保护'),
  },
  {
    key: 'password',
    label: '登录密码',
    icon: LockOnIcon,
    status: '已设置',
    statusType: 'ok',
    desc: '为了您的账号安全，建议您定期更换密码',
    actionText: '修改',
    action: () => openPasswordDialog(),
  },
  {
    key: 'operation-protect',
    label: '操作保护',
    icon: FingerprintIcon,
    status: '已开启',
    statusType: 'ok',
    desc: '开启操作保护，发生敏感操作时进行身份二次验证',
    tags: ['手机短信验证'],
    actionText: '修改',
    action: () => onDevelop('操作保护'),
  },
])

// ========== 第三方登录 ==========
const thirdPartyItems = [
  {
    key: 'wechat',
    label: '微信',
    icon: LogoWechatStrokeIcon,
    bound: true,
    desc: '绑定微信号与微信绑定后，可使用微信扫码登录',
  },
  {
    key: 'qq',
    label: 'QQ',
    icon: LogoQqIcon,
    bound: false,
    desc: '绑定 QQ 号后，可使用 QQ 扫码登录',
  },
  {
    key: 'weibo',
    label: '微博',
    icon: LogoTwitterIcon,
    bound: false,
    desc: '绑定微博账号后，可使用微博扫码登录',
  },
]

// ========== 账号注销 ==========
async function handleCloseAccount() {
  onDevelop('账号注销')
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

onMounted(() => {
  if (!userStore.loaded) {
    userStore.fetchUserInfo()
  }
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

.verify-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.verify-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 18px 20px;
  border: 1px solid #eef1f5;
  border-radius: 8px;
  background: #fff;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.verify-card:hover {
  border-color: #c7dbff;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.08);
}

.verify-badge {
  position: absolute;
  top: -1px;
  right: 16px;
  padding: 2px 10px;
  border-radius: 0 0 6px 6px;
  background: #52c41a;
  color: #fff;
  font-size: 12px;
}

.verify-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  border-radius: 8px;
  flex-shrink: 0;
}

.verify-icon--blue {
  background: #eff6ff;
  color: #2563eb;
}

.verify-icon--green {
  background: #ecfdf5;
  color: #10b981;
}

.verify-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.verify-body strong {
  font-size: 14.5px;
  color: #1e293b;
}

.verify-body span {
  font-size: 12.5px;
  color: #8b95a8;
  line-height: 1.5;
}

/* ========== 安全设置 / 第三方登录 ========== */
.security-grid,
.third-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 40px;
}

/* 宽屏下增加列数，减少大片留白 */
@media (min-width: 1600px) {
  .security-grid,
  .third-grid {
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

.security-icon--third {
  background: #fff;
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
.dark .verify-body strong,
.dark .security-label {
  color: #e5e7eb;
}

.dark .verify-card {
  background: #141414;
  border-color: #262626;
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
  .security-grid,
  .verify-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .basic-info {
    gap: 24px;
  }
}
</style>
