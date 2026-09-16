<template>
  <div class="page-body users-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <img v-if="profile?.avatar" class="page-header__avatar" :src="profile.avatar" :alt="profile.username" />
          <UserIcon v-else size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <div class="page-header__title-row">
            <h2 class="page-header__title">{{ profile?.username || '用户详情' }}</h2>
            <t-tag
              v-if="profile"
              :theme="userStatusTheme(profile.status)"
              variant="light"
              size="small"
              shape="round"
            >
              {{ userStatusLabel(profile.status) }}
            </t-tag>
            <t-tag v-if="profile?.is_sub_account" theme="warning" variant="light" size="small" shape="round">
              子账号
            </t-tag>
            <t-tag v-if="profile?.tier" :theme="userTierTheme(profile.tier)" variant="light" size="small" shape="round">
              {{ userTierLabel(profile.tier) }}
            </t-tag>
          </div>
          <p class="page-header__desc">
            用户 ID {{ userId || '—' }} · {{ profile?.email || '无邮箱' }} · {{ profile?.phone || '无手机号' }}
          </p>
        </div>
      </div>

      <div class="detail-actions">
        <t-button variant="outline" @click="goBack">返回列表</t-button>
        <t-button variant="outline" :loading="loading" @click="reloadAll">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button
          v-permission="'user:update'"
          variant="outline"
          :disabled="!profile"
          :title="profile ? '修改用户名、实名、邮箱、手机、地域与分组' : '用户未加载'"
          @click="editVisible = true"
        >
          编辑资料
        </t-button>
        <t-button
          v-permission="'user:update_status'"
          variant="outline"
          :disabled="!profile"
          :title="statusToggleTitle"
          @click="toggleStatus"
        >
          {{ profile?.status === 'active' ? '禁用账号' : '启用账号' }}
        </t-button>
        <t-button
          v-permission="'finance:adjust'"
          variant="outline"
          :disabled="!profile"
          title="人工调整钱包余额（增加或扣减）"
          @click="adjustVisible = true"
        >
          调整余额
        </t-button>
        <t-button
          v-permission="'user:create'"
          variant="outline"
          :disabled="!profile"
          title="为该用户代下单"
          @click="orderVisible = true"
        >
          创建订单
        </t-button>
        <t-dropdown :options="moreActions" trigger="click" @click="onMoreAction">
          <t-button variant="outline" :disabled="!profile">
            更多
            <template #suffix><ChevronDownIcon aria-hidden="true" /></template>
          </t-button>
        </t-dropdown>
      </div>
    </header>

    <section class="stat-grid" aria-label="用户概览统计">
      <article v-for="item in statCards" :key="item.key" class="stat-card" :class="`stat-card--${item.theme}`">
        <span class="stat-card__icon">
          <component :is="item.icon" size="22" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <div class="stat-card__value">{{ item.value }}</div>
          <div class="stat-card__label">{{ item.label }}</div>
        </div>
      </article>
    </section>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <t-tab-panel value="profile" label="概览">
          <ProfilePanel :profile="profile" :rbac-roles="rbacRoles" :degraded="degraded" :is-mobile="isMobile" />
        </t-tab-panel>

        <t-tab-panel value="assets" :label="tabLabel('云主机资产', summary?.instance_count)">
          <AssetsPanel
            :key="panelsKey"
            v-if="tabMounted.assets"
            :user-id="userId"
            :is-mobile="isMobile"
            :degraded="degraded.includes('instances')"
          />
        </t-tab-panel>

        <t-tab-panel value="finance" :label="tabLabel('订单财务', summary?.order_count)">
          <FinancePanel
            :key="panelsKey"
            v-if="tabMounted.finance && summary"
            :user-id="userId"
            :is-mobile="isMobile"
            :summary="summary"
            :degraded="hasFinanceDegraded"
          />
        </t-tab-panel>

        <t-tab-panel value="tickets" :label="tabLabel('服务工单', summary?.ticket_count)">
          <TicketsPanel
            :key="panelsKey"
            v-if="tabMounted.tickets"
            :user-id="userId"
            :is-mobile="isMobile"
            :summary="summary"
            :degraded="degraded.includes('tickets')"
          />
        </t-tab-panel>

        <t-tab-panel value="permissions" label="角色与权限">
          <PermissionsPanel
            :key="panelsKey"
            v-if="tabMounted.permissions && profile"
            :user-id="userId"
            :profile="profile"
            :rbac-roles="rbacRoles"
            :permissions="permissions"
            :degraded="degraded.includes('permissions') || degraded.includes('rbac_roles')"
          />
        </t-tab-panel>

        <t-tab-panel value="security" :label="tabLabel('安全与登录', summary?.login_count)">
          <SecurityPanel
            :key="panelsKey"
            v-if="tabMounted.security && profile"
            :user-id="userId"
            :username="profile.username"
            :is-mobile="isMobile"
            :summary="summary"
          />
        </t-tab-panel>

        <t-tab-panel value="audit" :label="tabLabel('操作日志', summary?.operation_log_count)">
          <AuditPanel
            :key="panelsKey"
            v-if="tabMounted.audit"
            :user-id="userId"
            :is-mobile="isMobile"
            :degraded="degraded.includes('summary')"
          />
        </t-tab-panel>

        <t-tab-panel value="verification" :label="tabLabel('实名认证', summary?.verification_count)">
          <VerificationPanel
            :key="panelsKey"
            v-if="tabMounted.verification"
            :user-id="userId"
            :is-mobile="isMobile"
            :degraded="degraded.includes('summary')"
          />
        </t-tab-panel>
      </t-tabs>
    </section>

    <EditProfileDialog v-model="editVisible" :profile="profile" @saved="reloadAggregate" />
    <ResetPasswordDialog v-model="resetVisible" :user-id="userId" :username="profile?.username || ''" />
    <AdjustBalanceDialog
      v-model="adjustVisible"
      :user-id="userId"
      :username="profile?.username || ''"
      :current-balance="profile?.balance || 0"
      @saved="reloadAggregate"
    />
    <CreateOrderDialog
      v-model="orderVisible"
      :user-id="userId"
      :username="profile?.username || ''"
      @saved="reloadAll"
    />
    <RolesDialog
      v-model="rolesVisible"
      :user-id="userId"
      :username="profile?.username || ''"
      :current-role-ids="rbacRoles.map((role) => role.id)"
      @saved="reloadAggregate"
    />
    <SalesAssignDialog
      v-model="salesVisible"
      :user-id="userId"
      :username="profile?.username || ''"
      :sales-admin-id="profile?.sales_admin_id || 0"
      :sales-admin-name="profile?.sales_admin_name || ''"
      @saved="reloadAggregate"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  ChevronDownIcon,
  CloudIcon,
  MoneyIcon,
  RefreshIcon,
  ServiceIcon,
  UserIcon,
  UsergroupIcon,
} from 'tdesign-icons-vue-next'

import { getUserDetailAggregate, updateUserStatus, type UserDetailSummary, type UserInfo, type UserRoleBrief } from '@/api/user'
import { formatAmount, userStatusLabel, userStatusTheme, userTierLabel, userTierTheme } from '@/pages/users/constants'
import { useIsMobile } from '@/composables/useIsMobile'

import ProfilePanel from './components/ProfilePanel.vue'
import AssetsPanel from './components/AssetsPanel.vue'
import FinancePanel from './components/FinancePanel.vue'
import TicketsPanel from './components/TicketsPanel.vue'
import PermissionsPanel from './components/PermissionsPanel.vue'
import SecurityPanel from './components/SecurityPanel.vue'
import AuditPanel from './components/AuditPanel.vue'
import VerificationPanel from './components/VerificationPanel.vue'
import EditProfileDialog from './components/EditProfileDialog.vue'
import ResetPasswordDialog from './components/ResetPasswordDialog.vue'
import AdjustBalanceDialog from './components/AdjustBalanceDialog.vue'
import CreateOrderDialog from './components/CreateOrderDialog.vue'
import RolesDialog from './components/RolesDialog.vue'
import SalesAssignDialog from './components/SalesAssignDialog.vue'

defineOptions({ name: 'UserDetail' })

type DetailTab =
  | 'profile'
  | 'assets'
  | 'finance'
  | 'tickets'
  | 'permissions'
  | 'security'
  | 'audit'
  | 'verification'

const route = useRoute()
const router = useRouter()
const { isMobile } = useIsMobile()

const userId = computed(() => Number(route.query.id || 0))

const loading = ref(false)
const profile = ref<UserInfo | null>(null)
const summary = ref<UserDetailSummary | null>(null)
const rbacRoles = ref<UserRoleBrief[]>([])
const permissions = ref<string[]>([])
const degraded = ref<string[]>([])
const activeTab = ref<DetailTab>('profile')

// Tab 懒加载：只挂载访问过的面板，未访问过的面板不会发请求。
const tabMounted = reactive<Record<DetailTab, boolean>>({
  profile: true,
  assets: false,
  finance: false,
  tickets: false,
  permissions: false,
  security: false,
  audit: false,
  verification: false,
})

watch(activeTab, (tab) => {
  tabMounted[tab] = true
})

// ---------- 弹窗状态 ----------
const editVisible = ref(false)
const resetVisible = ref(false)
const adjustVisible = ref(false)
const orderVisible = ref(false)
const rolesVisible = ref(false)
const salesVisible = ref(false)

const moreActions = [
  { content: '重置密码', value: 'reset' },
  { content: '分配后台角色', value: 'roles' },
  { content: '归属销售', value: 'sales' },
]

function onMoreAction(data: { value: string | number }) {
  switch (data.value) {
    case 'reset':
      resetVisible.value = true
      break
    case 'roles':
      rolesVisible.value = true
      break
    case 'sales':
      salesVisible.value = true
      break
  }
}

// ---------- 统计卡 ----------
const statCards = computed(() => {
  const s = summary.value
  return [
    {
      key: 'balance',
      label: '账户余额（元）',
      value: formatAmount(profile.value?.balance || 0),
      theme: 'green',
      icon: MoneyIcon,
    },
    {
      key: 'assets',
      label: `云主机资产（运行 ${s?.running_instance_count ?? 0}）`,
      value: String(s?.instance_count ?? 0),
      theme: 'blue',
      icon: CloudIcon,
    },
    {
      key: 'orders',
      label: '订单数（含续费）',
      value: String(s?.order_count ?? 0),
      theme: 'purple',
      icon: UsergroupIcon,
    },
    {
      key: 'tickets',
      label: `服务工单（处理中 ${s?.open_ticket_count ?? 0}）`,
      value: String(s?.ticket_count ?? 0),
      theme: 'orange',
      icon: ServiceIcon,
    },
  ]
})

// 财务相关的三段任一降级，都提示财务面板。
const hasFinanceDegraded = computed(() =>
  ['orders', 'bills', 'transactions', 'summary'].some((key) => degraded.value.includes(key)),
)

const statusToggleTitle = computed(() => {
  if (!profile.value) return '用户未加载'
  return profile.value.status === 'active'
    ? '禁用后该用户将无法登录用户中心'
    : '启用后该用户恢复正常登录'
})

function tabLabel(label: string, count?: number): string {
  if (count === undefined || count === null) return label
  return count > 0 ? `${label}（${count}）` : label
}

// ---------- 数据加载 ----------
async function loadAggregate() {
  if (!userId.value) {
    MessagePlugin.error('缺少用户 ID，请从用户列表进入详情页')
    return
  }
  loading.value = true
  try {
    const data = await getUserDetailAggregate(userId.value)
    profile.value = data.profile
    summary.value = data.summary
    rbacRoles.value = data.rbac_roles || []
    permissions.value = data.permissions || []
    degraded.value = data.degraded || []
  } catch (error) {
    profile.value = null
    MessagePlugin.error((error as Error).message || '加载用户详情失败')
  } finally {
    loading.value = false
  }
}

function reloadAggregate() {
  void loadAggregate()
}

// 用 key 强制重挂载已加载的面板：各面板各自负责自己的请求与分页状态，
// 父级不做统一编排，避免为 8 个面板维护 8 套 ref 转发。
const panelsKey = ref(0)

function reloadAll() {
  void loadAggregate()
  panelsKey.value += 1
}

async function toggleStatus() {
  if (!profile.value) return
  const next = profile.value.status === 'active' ? 'disabled' : 'active'
  const verb = next === 'disabled' ? '禁用' : '启用'
  try {
    await updateUserStatus(profile.value.id, { status: next })
    MessagePlugin.success(`已${verb}该账号`)
    void loadAggregate()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `${verb}失败`)
  }
}

function goBack() {
  router.push('/users/accounts/list')
}

watch(userId, () => {
  activeTab.value = 'profile'
  Object.keys(tabMounted).forEach((key) => {
    tabMounted[key as DetailTab] = key === 'profile'
  })
  void loadAggregate()
}, { immediate: true })
</script>

<style>
@import '../../shared.css';
</style>