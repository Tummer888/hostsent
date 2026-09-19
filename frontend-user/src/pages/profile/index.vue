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
        <span class="panel-sub">证件号加密存储，审核仅展示脱敏值</span>
        <t-link
          v-if="siteConfigured"
          theme="primary"
          hover="color"
          :href="sitePath('/help')"
          target="_blank"
          rel="noopener"
          class="panel-head__link"
        >
          认证说明
        </t-link>
      </div>

      <t-loading :loading="realnameLoading" size="small">
        <!-- 平台关闭自助入口：只留工单路径，不渲染一个提交上去必然被拒的表单 -->
        <template v-if="!realnameStatus.enabled">
          <t-alert theme="info" class="verify-alert">
            平台当前未开放自助实名提交，需要认证请提交工单（分类选择「授权申请」）并附主体证件信息。
          </t-alert>
          <div class="verify-actions">
            <t-button theme="default" @click="goTicket()">提交认证工单</t-button>
          </div>
        </template>

        <template v-else>
          <t-alert :theme="realnameAlert.theme" class="verify-alert">
            {{ realnameAlert.text }}
          </t-alert>

          <!-- 状态摘要：非「未提交」时展示已落库的申请信息 -->
          <div v-if="realnameStatus.status !== 'none'" class="verify-summary">
            <div class="verify-row">
              <span class="verify-row__label">认证类型</span>
              <span class="verify-row__value">{{ typeLabel(realnameStatus.verification_type) }}</span>
            </div>
            <div class="verify-row">
              <span class="verify-row__label">认证主体</span>
              <span class="verify-row__value">{{ realnameStatus.real_name || '-' }}</span>
            </div>
            <div class="verify-row">
              <span class="verify-row__label">证件号码</span>
              <span class="verify-row__value">{{ realnameStatus.id_number_masked || '-' }}</span>
            </div>
            <div class="verify-row">
              <span class="verify-row__label">手机号</span>
              <span class="verify-row__value">{{ realnameStatus.mobile_masked || '-' }}</span>
            </div>
            <div class="verify-row">
              <span class="verify-row__label">提交时间</span>
              <span class="verify-row__value">{{ formatTime(realnameStatus.submitted_at) }}</span>
            </div>
            <div v-if="realnameStatus.reviewed_at" class="verify-row">
              <span class="verify-row__label">审核时间</span>
              <span class="verify-row__value">{{ formatTime(realnameStatus.reviewed_at) }}</span>
            </div>
            <div v-if="realnameStatus.review_round > 1" class="verify-row">
              <span class="verify-row__label">提交次数</span>
              <span class="verify-row__value">第 {{ realnameStatus.review_round }} 次</span>
            </div>
            <div v-if="realnameStatus.status === 'rejected' && realnameStatus.reject_reason" class="verify-row verify-row--full">
              <span class="verify-row__label">驳回理由</span>
              <span class="verify-row__value verify-row__value--danger">{{ realnameStatus.reject_reason }}</span>
            </div>
            <div v-if="realnameStatus.review_note" class="verify-row verify-row--full">
              <span class="verify-row__label">审核备注</span>
              <span class="verify-row__value">{{ realnameStatus.review_note }}</span>
            </div>
          </div>

          <!-- 已上传材料 -->
          <div v-if="realnameDocuments.length" class="doc-list">
            <span class="doc-list__title">已提交材料</span>
            <div class="doc-list__items">
              <t-link
                v-for="doc in realnameDocuments"
                :key="doc.id"
                theme="primary"
                hover="color"
                @click="downloadDocument(doc)"
              >
                {{ documentLabel(doc.document_type) }}
              </t-link>
            </div>
          </div>

          <div class="verify-actions">
            <t-button v-if="canSubmitRealname" theme="primary" @click="openRealnameDialog">
              {{ realnameStatus.status === 'rejected' ? '重新提交认证' : '开始实名认证' }}
            </t-button>
            <!-- 支付宝跳转核验：只有当前申请仍待审且服务商实现了 Initializer 才走得通，
                 失败时后端会明确报「不支持」，这里不做前端猜测。 -->
            <t-button
              v-if="realnameStatus.status === 'pending'"
              theme="default"
              :loading="authorizing"
              @click="handleAuthorize"
            >
              前往支付宝完成核验
            </t-button>
            <t-button variant="text" theme="primary" @click="goTicket()">
              企业主体可改走「授权申请」工单
            </t-button>
          </div>
        </template>
      </t-loading>
    </section>

    <!-- ============ 第三方登录 ============ -->
    <!-- 数据源是真实绑定（GET /uc/oauth/bindings），不再硬编码 bound。
         未绑定的渠道只列「已启用」的那些——列一个点了必然报错的渠道没有意义。 -->
    <section v-if="oauthItems.length" class="panel">
      <div class="panel-head">
        <h3 class="panel-title">第三方登录</h3>
        <span class="panel-sub">绑定后可用对应平台扫码登录本账号</span>
      </div>

      <div class="third-grid">
        <div v-for="item in oauthItems" :key="item.provider" class="security-item">
          <span class="security-icon security-icon--third">
            <component :is="oauthIcon(item.provider, item.icon)" size="20" />
          </span>
          <div class="security-body">
            <div class="security-title">
              <span class="security-label">{{ item.name }}</span>
              <span class="security-status" :class="`is-${item.binding ? 'ok' : 'warn'}`">
                <CheckCircleFilledIcon v-if="item.binding" size="13" />
                <ErrorCircleFilledIcon v-else size="13" />
                {{ item.binding ? '已绑定' : '未绑定' }}
              </span>
            </div>
            <p class="security-desc">{{ oauthDesc(item) }}</p>
          </div>
          <t-tooltip v-if="item.binding && !item.binding.can_unbind" content="这是账号当前唯一的登录方式，解绑后将无法登录">
            <span>
              <t-button size="small" variant="outline" disabled>解绑</t-button>
            </span>
          </t-tooltip>
          <t-button
            v-else-if="item.binding"
            size="small"
            variant="outline"
            :loading="oauthBusy === item.provider"
            @click="handleUnbind(item)"
          >
            解绑
          </t-button>
          <t-button
            v-else
            size="small"
            variant="outline"
            :loading="oauthBusy === item.provider"
            @click="handleBind(item.provider)"
          >
            绑定
          </t-button>
        </div>
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

    <!-- ============ 实名提交 ============ -->
    <t-dialog
      v-model:visible="realnameVisible"
      header="实名认证"
      width="560px"
      :confirm-loading="realnameSubmitting"
      :confirm-btn="{ content: '提交审核', disabled: !canConfirmRealname }"
      @confirm="handleSubmitRealname"
    >
      <t-form ref="realnameFormRef" :data="realnameForm" :rules="realnameRules" label-align="top">
        <t-form-item label="认证类型" name="verification_type">
          <t-radio-group v-model="realnameForm.verification_type">
            <t-radio v-for="t in realnameAllowedTypes" :key="t" :value="t">{{ typeLabel(t) }}</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item :label="isEnterpriseForm ? '企业名称' : '真实姓名'" name="real_name">
          <t-input
            v-model="realnameForm.real_name"
            :placeholder="isEnterpriseForm ? '请输入营业执照上的企业全称' : '请输入证件上的姓名'"
          />
        </t-form-item>
        <t-form-item v-if="isEnterpriseForm" label="法定代表人" name="legal_person_name">
          <t-input v-model="realnameForm.legal_person_name" placeholder="请输入法定代表人姓名" />
        </t-form-item>
        <t-form-item v-if="isEnterpriseForm" label="联系人" name="contact_name">
          <t-input v-model="realnameForm.contact_name" placeholder="请输入日常联系人的姓名" />
        </t-form-item>
        <t-form-item
          :label="isEnterpriseForm ? '统一社会信用代码' : '身份证号'"
          name="id_number"
        >
          <t-input
            v-model="realnameForm.id_number"
            :placeholder="isEnterpriseForm ? '请输入 18 位统一社会信用代码' : '请输入 18 位身份证号'"
          />
        </t-form-item>
        <t-form-item label="手机号" name="mobile">
          <t-input v-model="realnameForm.mobile" placeholder="用于接收审核结果通知（可选）" />
        </t-form-item>

        <!-- 二次验证：策略要求时才渲染。提交时会换一张关键操作票据 -->
        <t-form-item v-if="realnameVerify.imageRequired" label="图形验证码" name="captcha_code">
          <div class="verify-row-inline">
            <t-input v-model="realnameForm.captcha_code" placeholder="请输入图形验证码" />
            <CaptchaImage v-model:key="realnameForm.captcha_key" v-model:code="realnameForm.captcha_code" scene="realname_submit" />
          </div>
        </t-form-item>
        <t-form-item v-if="realnameVerify.otpRequired" :label="`${realnameVerify.channelLabel}验证码`" name="otp_code">
          <div class="verify-row-inline">
            <t-input v-model="realnameForm.otp_code" placeholder="请输入验证码" />
            <t-button
              variant="outline"
              :disabled="realnameOtpCountdown > 0 || realnameSendingOtp"
              :loading="realnameSendingOtp"
              @click="sendRealnameOtp"
            >
              {{ realnameOtpCountdown > 0 ? `${realnameOtpCountdown}s` : '获取验证码' }}
            </t-button>
          </div>
        </t-form-item>

        <t-alert theme="info" class="realname-tip">
          证件号仅用于核验，平台以加密形式存储，审核页面只展示脱敏值。
          <template v-if="realnameStatus.status !== 'none'">
            提交后将替换当前申请，材料需重新上传。
          </template>
        </t-alert>
      </t-form>
    </t-dialog>

    <!-- ============ 材料上传 ============ -->
    <!-- 材料必须挂在一张已存在的申请上（后端按 application_id 归属鉴权），
         所以先提交申请再传图，不在这里做「暂存后一起提交」。 -->
    <t-dialog
      v-model:visible="docVisible"
      header="上传认证材料"
      width="560px"
      :footer="false"
    >
      <p class="doc-tip">
        申请已提交（编号 {{ docApplicationId }}），可继续上传证件照供审核。支持 JPG/PNG/WEBP/BMP/PDF，
        单个不超过 10MB，最多 {{ MAX_DOCUMENTS }} 份；同类型重新上传会覆盖原文件。
      </p>
      <div v-for="dt in documentTypes" :key="dt.value" class="doc-row">
        <div class="doc-row__head">
          <span class="doc-row__label">{{ dt.label }}</span>
          <span class="doc-row__name">{{ docFiles[dt.value]?.name || '未上传' }}</span>
        </div>
        <input
          :ref="(el) => setDocInputRef(dt.value, el)"
          type="file"
          class="hidden-file-input"
          accept=".png,.jpg,.jpeg,.webp,.bmp,.pdf"
          @change="(e) => handleDocPicked(e, dt.value)"
        />
        <t-button size="small" variant="outline" :loading="docUploading === dt.value" @click="triggerDocPick(dt.value)">
          {{ docFiles[dt.value] ? '重新上传' : '选择文件' }}
        </t-button>
      </div>
      <div class="doc-actions">
        <t-button theme="primary" @click="docVisible = false">完成</t-button>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  CheckCircleFilledIcon,
  CopyIcon,
  Edit1Icon,
  ErrorCircleFilledIcon,
  LockOnIcon,
  LogoAlipayIcon,
  LogoGithubIcon,
  LogoQqIcon,
  LogoWechatStrokeIcon,
  LogoWecomIcon,
  MailIcon,
  MobileIcon,
  SecuredIcon,
} from 'tdesign-icons-vue-next'
import type { Component } from 'vue'

import { useUserStore } from '@/store'
import { useBrandStore } from '@/store/modules/brand'
import { updateProfile, changePassword } from '@/api/auth'
import {
  getMyOAuthBindings,
  getOAuthBindAuthorizeUrl,
  getOAuthProviders,
  unbindOAuth,
  type MyOAuthBinding,
  type PublicOAuthProvider,
} from '@/api/oauth'
import {
  authorizeVerification,
  downloadVerificationDocument,
  getMyVerificationDetail,
  getVerificationStatus,
  submitVerification,
  uploadVerificationDocument,
  type VerificationDocument,
  type VerificationStatus,
} from '@/api/verification'
import { getVerificationRequirement, sendSecurityVerification, verifySecurityCode } from '@/api/security'
import CaptchaImage from '@/components/verify/CaptchaImage.vue'
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

// ========== 实名认证（doc104 §5.7）==========
// 判据是后端 status（源自 users.real_name_verified_at），不是「real_name 是否非空」——
// 展示名可以在资料表单里直接改，用它判断等于给了个「改昵称即实名」的后门。
const realnameLoading = ref(true)
const realnameStatus = ref<VerificationStatus>({
  status: 'none',
  enabled: false,
  allowed_types: [],
  application_id: 0,
  verification_type: '',
  real_name: '',
  id_number_masked: '',
  mobile_masked: '',
  reject_reason: '',
  review_note: '',
  provider_result: '',
  review_round: 0,
  cooldown_hours: 0,
  cooldown_remain_seconds: 0,
})

const realnameAlert = computed(() => {
  switch (realnameStatus.value.status) {
    case 'approved':
      return { theme: 'success' as const, text: '已完成实名认证，可正常购买产品与提交全部工单分类。' }
    case 'pending':
      return { theme: 'info' as const, text: '认证申请已提交，正在等待平台审核，通常 1 个工作日内完成。' }
    case 'rejected':
      return { theme: 'error' as const, text: '上次认证申请未通过，请按驳回理由修正后重新提交。' }
    default:
      return {
        theme: 'warning' as const,
        text: `未完成实名认证，无法购买${brandStore.name}的产品和服务，部分工单分类也无法提交。`,
      }
  }
})

const realnameDocuments = ref<VerificationDocument[]>([])

const TYPE_LABELS: Record<string, string> = {
  personal: '个人认证',
  enterprise: '企业认证',
}

function typeLabel(type: string): string {
  return TYPE_LABELS[type] || type || '-'
}

const DOCUMENT_LABELS: Record<string, string> = {
  id_front: '身份证正面',
  id_back: '身份证反面',
  handheld: '手持证件照',
  business_license: '营业执照',
  authorization: '授权委托书',
}

function documentLabel(type: string): string {
  return DOCUMENT_LABELS[type] || type
}

function formatTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** 冷却期未满时不放行提交按钮：点了也只会拿回 409，不如直接拦住并说明。 */
const canSubmitRealname = computed(() => {
  const st = realnameStatus.value
  if (st.status === 'pending') return false
  if (st.status === 'rejected' && st.cooldown_remain_seconds > 0) return false
  return true
})

const realnameAllowedTypes = computed(() => {
  const list = realnameStatus.value.allowed_types
  return list.length ? list : ['personal']
})

async function loadRealname() {
  realnameLoading.value = true
  try {
    const { data } = await getVerificationStatus()
    realnameStatus.value = { ...realnameStatus.value, ...data }
    if (data?.application_id) {
      await loadRealnameDocuments(data.application_id)
    } else {
      realnameDocuments.value = []
    }
  } catch (error) {
    // 状态拉不到时按「未实名」展示，不影响页面上其他面板（安全设置/资料编辑）。
    console.error('Load realname status failed:', error)
  } finally {
    realnameLoading.value = false
  }
}

/** 已提交材料：只有申请详情接口才带 documents，状态接口不带。 */
async function loadRealnameDocuments(applicationId: number) {
  try {
    const { data } = await getMyVerificationDetail(applicationId)
    realnameDocuments.value = data?.documents || []
  } catch {
    realnameDocuments.value = []
  }
}

async function downloadDocument(doc: VerificationDocument) {
  try {
    await downloadVerificationDocument(doc.id, documentLabel(doc.document_type))
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '下载失败')
  }
}

// —— 提交表单 ——
const realnameFormRef = ref()
const realnameVisible = ref(false)
const realnameSubmitting = ref(false)
const authorizing = ref(false)

const realnameForm = reactive({
  verification_type: 'personal',
  real_name: '',
  legal_person_name: '',
  contact_name: '',
  id_number: '',
  mobile: '',
  captcha_key: '',
  captcha_code: '',
  otp_code: '',
})

const realnameRules = {
  real_name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  id_number: [{ required: true, message: '请输入证件号码', trigger: 'blur' }],
}

const isEnterpriseForm = computed(() => realnameForm.verification_type === 'enterprise')

const canConfirmRealname = computed(() => {
  if (!realnameForm.real_name.trim() || !realnameForm.id_number.trim()) return false
  if (realnameVerify.value.imageRequired && !realnameForm.captcha_code.trim()) return false
  if (realnameVerify.value.otpRequired && !realnameForm.otp_code.trim()) return false
  return true
})

/** 该场景的二次验证要求（realname_submit）。拉不到就按「不要求」处理，与后端降级口径一致。 */
const realnameVerify = ref({ imageRequired: false, otpRequired: false, channel: 'email', channelLabel: '邮箱' })

async function loadRealnameRequirement() {
  try {
    const { data } = await getVerificationRequirement('realname_submit')
    realnameVerify.value = {
      imageRequired: data?.image_required === true,
      otpRequired: data?.need_verification === true,
      channel: data?.channel === 'sms' ? 'sms' : 'email',
      channelLabel: data?.channel === 'sms' ? '手机' : '邮箱',
    }
  } catch {
    realnameVerify.value = { imageRequired: false, otpRequired: false, channel: 'email', channelLabel: '邮箱' }
  }
}

const realnameOtpCountdown = ref(0)
const realnameSendingOtp = ref(false)

function openRealnameDialog() {
  const st = realnameStatus.value
  realnameForm.verification_type = st.verification_type || realnameAllowedTypes.value[0] || 'personal'
  // 已驳回重提时把上次填的名称带回来：用户要改的通常只是证件号或材料。
  realnameForm.real_name = st.status === 'rejected' ? st.real_name : ''
  realnameForm.legal_person_name = ''
  realnameForm.contact_name = ''
  realnameForm.id_number = ''
  realnameForm.mobile = ''
  realnameForm.captcha_code = ''
  realnameForm.otp_code = ''
  realnameVisible.value = true
  void loadRealnameRequirement()
}

async function sendRealnameOtp() {
  if (realnameOtpCountdown.value > 0) return
  realnameSendingOtp.value = true
  try {
    await sendSecurityVerification({
      scene: 'realname_submit',
      channel: realnameVerify.value.channel as 'sms' | 'email',
      captcha_key: realnameForm.captcha_key || undefined,
      captcha_code: realnameForm.captcha_code.trim() || undefined,
    })
    MessagePlugin.success('验证码已发送，请查收')
    realnameOtpCountdown.value = 60
    const timer = setInterval(() => {
      realnameOtpCountdown.value -= 1
      if (realnameOtpCountdown.value <= 0) clearInterval(timer)
    }, 1000)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '验证码发送失败')
  } finally {
    realnameSendingOtp.value = false
  }
}

async function handleSubmitRealname() {
  const valid = await realnameFormRef.value?.validate()
  if (valid !== true) return
  realnameSubmitting.value = true
  try {
    // OTP 场景要先把验证码换成关键操作票据，后端只认票据不认验证码。
    let verifyTicket = ''
    if (realnameVerify.value.otpRequired) {
      const { data } = await verifySecurityCode({ scene: 'realname_submit', code: realnameForm.otp_code.trim() })
      verifyTicket = data.verify_ticket
    }
    const { data } = await submitVerification({
      verification_type: realnameForm.verification_type,
      real_name: realnameForm.real_name.trim(),
      legal_person_name: realnameForm.legal_person_name.trim() || undefined,
      contact_name: realnameForm.contact_name.trim() || undefined,
      id_number: realnameForm.id_number.trim(),
      mobile: realnameForm.mobile.trim() || undefined,
      captcha_key: realnameForm.captcha_key || undefined,
      captcha_code: realnameForm.captcha_code.trim() || undefined,
      verify_ticket: verifyTicket || undefined,
    })
    realnameVisible.value = false
    MessagePlugin.success('认证申请已提交，请等待审核')
    await loadRealname()
    // 申请已落库：材料挂在它下面上传（后端按 application_id 归属鉴权）。
    if (data?.id) {
      openDocDialog(data.id)
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '提交失败，请稍后重试')
  } finally {
    realnameSubmitting.value = false
  }
}

/** 支付宝跳转核验：拿认证入口 URL 后整页跳走，回跳由 provider-callback 处理。 */
async function handleAuthorize() {
  const id = realnameStatus.value.application_id
  if (!id) return
  authorizing.value = true
  try {
    const { data } = await authorizeVerification(id)
    if (!data?.auth_url) {
      MessagePlugin.error('未获取到认证入口，请稍后重试')
      return
    }
    window.location.href = data.auth_url
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '当前服务商不支持跳转认证，请等待人工审核')
  } finally {
    authorizing.value = false
  }
}

// —— 材料上传 ——
const MAX_DOCUMENTS = 10
const docVisible = ref(false)
const docApplicationId = ref(0)
const docUploading = ref('')
const docFiles = reactive<Record<string, File | null>>({})
const docInputRefs: Record<string, HTMLInputElement | null> = {}

/** 材料类型按认证主体区分：个人只收证件照，企业多营业执照与授权书。 */
const documentTypes = computed(() => {
  const list = [
    { label: '身份证正面', value: 'id_front' },
    { label: '身份证反面', value: 'id_back' },
    { label: '手持证件照', value: 'handheld' },
  ]
  if (realnameStatus.value.verification_type === 'enterprise') {
    list.push({ label: '营业执照', value: 'business_license' }, { label: '授权委托书', value: 'authorization' })
  }
  return list
})

function setDocInputRef(type: string, el: unknown) {
  docInputRefs[type] = (el as HTMLInputElement) || null
}

function openDocDialog(applicationId: number) {
  docApplicationId.value = applicationId
  Object.keys(docFiles).forEach((k) => delete docFiles[k])
  docVisible.value = true
}

function triggerDocPick(type: string) {
  docInputRefs[type]?.click()
}

async function handleDocPicked(event: Event, type: string) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > 10 * 1024 * 1024) {
    MessagePlugin.warning('单个文件不能超过 10MB')
    return
  }
  docUploading.value = type
  try {
    await uploadVerificationDocument(docApplicationId.value, type, file)
    docFiles[type] = file
    MessagePlugin.success(`${documentLabel(type)}上传成功`)
    await loadRealnameDocuments(docApplicationId.value)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '上传失败')
  } finally {
    docUploading.value = ''
  }
}

// ========== 第三方登录（doc104 §6.8）==========
// 绑定状态全部来自 GET /uc/oauth/bindings —— 面板里不再有任何硬编码的 bound。
interface OAuthItem {
  provider: string
  name: string
  icon: string
  binding?: MyOAuthBinding
}

const oauthBusy = ref('')
const oauthProviders = ref<PublicOAuthProvider[]>([])
const oauthBindings = ref<MyOAuthBinding[]>([])

/** 已绑定的一律展示（含已停用渠道，否则用户没法解绑），未绑定的只列已启用渠道。 */
const oauthItems = computed<OAuthItem[]>(() => {
  const bindingMap = new Map(oauthBindings.value.map((b) => [b.provider, b]))
  const items: OAuthItem[] = []
  for (const binding of oauthBindings.value) {
    items.push({ provider: binding.provider, name: binding.name || binding.provider, icon: binding.icon, binding })
  }
  for (const provider of oauthProviders.value) {
    if (bindingMap.has(provider.provider)) continue
    items.push({ provider: provider.provider, name: provider.name, icon: provider.icon })
  }
  return items
})

function oauthIcon(provider: string, icon: string) {
  const map: Record<string, unknown> = {
    wechat: LogoWechatStrokeIcon,
    qq: LogoQqIcon,
    alipay: LogoAlipayIcon,
    wecom: LogoWecomIcon,
    github: LogoGithubIcon,
  }
  return map[icon || provider] || LogoWechatStrokeIcon
}

function oauthDesc(item: OAuthItem) {
  if (!item.binding) {
    return `绑定${item.name}账号后，可使用${item.name}快捷登录本账号`
  }
  const nickname = item.binding.nickname ? `（${item.binding.nickname}）` : ''
  if (item.binding.last_login_at) {
    return `已绑定${nickname}，最近一次使用：${formatTime(item.binding.last_login_at)}`
  }
  return `已绑定${nickname}，绑定时间：${formatTime(item.binding.bound_at)}`
}

async function loadOAuth() {
  // 两个接口互相独立：渠道列表拉不到不该让已绑定关系也消失（否则用户看不到解绑入口）。
  const [providers, bindings] = await Promise.allSettled([getOAuthProviders(), getMyOAuthBindings()])
  oauthProviders.value = providers.status === 'fulfilled' ? providers.value.data || [] : []
  oauthBindings.value = bindings.status === 'fulfilled' ? bindings.value.data || [] : []
}

async function handleBind(provider: string) {
  if (oauthBusy.value) return
  oauthBusy.value = provider
  try {
    const { data } = await getOAuthBindAuthorizeUrl(provider)
    if (!data?.authorize_url) {
      MessagePlugin.error('该登录方式暂不可用')
      return
    }
    // 整页跳走：state 里带当前用户 ID，回调只能绑到发起时的账号。
    window.location.href = data.authorize_url
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '绑定失败，请稍后重试')
  } finally {
    oauthBusy.value = ''
  }
}

function handleUnbind(item: OAuthItem) {
  const dialog = DialogPlugin.confirm({
    header: '解绑第三方账号',
    body: `解绑后该${item.name}账号将无法登录本账号，确定继续吗？`,
    theme: 'warning',
    confirmBtn: '确认解绑',
    onConfirm: async () => {
      oauthBusy.value = item.provider
      try {
        await unbindOAuth(item.provider)
        MessagePlugin.success('已解绑')
        await loadOAuth()
      } catch (error) {
        MessagePlugin.error((error as Error)?.message || '解绑失败')
      } finally {
        oauthBusy.value = ''
        dialog.hide()
      }
    },
  })
}

// 绑定成功后三方会 302 回 /oauth/callback 再跳回本页，本页挂载时重新拉一次绑定即可，
// 因此这里不需要轮询或 watch。

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
  // 实名状态与第三方绑定互不依赖，并行拉取；任一失败都只影响自己的面板。
  await Promise.all([loadRealname(), loadOAuth()])
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
.panel-head__link {
  margin-left: auto;
}

.verify-alert {
  border-radius: 6px;
  margin-bottom: 16px;
}

.verify-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 32px;
  padding: 4px 0 16px;
}

.verify-row {
  display: flex;
  gap: 12px;
  font-size: 13px;
  line-height: 1.7;
}

.verify-row--full {
  grid-column: 1 / -1;
}

.verify-row__label {
  flex-shrink: 0;
  width: 72px;
  color: #94a3b8;
}

.verify-row__value {
  color: #334155;
  word-break: break-all;
}

.verify-row__value--danger {
  color: #d54941;
}

.verify-row-inline {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
}

.verify-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.realname-tip {
  margin-top: 4px;
  border-radius: 6px;
}

.doc-list {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding-bottom: 16px;
  font-size: 13px;
}

.doc-list__title {
  color: #94a3b8;
}

.doc-list__items {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

/* —— 材料上传弹窗 —— */
.doc-tip {
  margin: 0 0 16px;
  font-size: 12.5px;
  line-height: 1.8;
  color: #64748b;
}

.doc-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid #f1f5f9;
}

.doc-row__head {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.doc-row__label {
  font-size: 13.5px;
  color: #1e293b;
}

.doc-row__name {
  font-size: 12px;
  color: #94a3b8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 320px;
}

.doc-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 16px;
}

.hidden-file-input {
  display: none;
}

/* ========== 第三方登录 ========== */
.third-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 40px;
}

.security-icon--third {
  background: #fff;
}

/* ========== 安全设置 ========== */
.security-grid {
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
  .security-grid,
  .third-grid,
  .verify-summary {
    grid-template-columns: minmax(0, 1fr);
  }

  .basic-info {
    gap: 24px;
  }
}
</style>
