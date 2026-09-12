<template>
  <div class="user-detail-page">
    <header class="detail-header surface-card">
      <div class="detail-header__main">
        <div class="detail-header__crumb">账户管理 / 用户详情</div>
        <div class="detail-header__title-row">
          <div class="detail-header__identity">
            <div class="detail-avatar">{{ displayInitial }}</div>
            <div>
              <div class="detail-header__headline">
                <h2 class="detail-header__title">{{ userDetail?.username || '用户详情' }}</h2>
                <t-tag v-if="userDetail" class="status-tag" :class="`status-tag--${userDetail.status}`" theme="default" variant="light" size="small" shape="round">
                  {{ statusLabelMap[userDetail.status] || userDetail.status }}
                </t-tag>
              </div>
            </div>
          </div>
          <div class="detail-header__actions">
            <t-button class="page-btn page-btn--ghost" variant="outline" @click="router.push('/users/accounts/list')">返回列表</t-button>
            <t-button class="page-btn" theme="primary" :loading="saving" @click="openEditDialog">
              <template #icon>
                <EditIcon aria-hidden="true" />
              </template>
              编辑资料
            </t-button>
          </div>
        </div>
      </div>
    </header>

    <section class="summary-grid" aria-label="用户摘要">
      <article v-for="item in summaryCards" :key="item.key" class="summary-card surface-card">
        <div class="summary-card__head">
          <span class="summary-card__label">{{ item.label }}</span>
          <component :is="item.icon" size="18" aria-hidden="true" class="summary-card__icon" />
        </div>
        <div class="summary-card__value">{{ item.value }}</div>
      </article>
    </section>

    <section class="detail-tabs surface-card">
      <t-tabs v-model="activeTab" theme="card" size="medium">
        <t-tab-panel value="profile" label="基础资料" />
        <t-tab-panel value="permissions" label="角色权限" />
        <t-tab-panel value="members" label="成员" />
        <t-tab-panel value="assets" label="云主机资产" />
        <t-tab-panel value="orders" label="订单财务" />
        <t-tab-panel value="tickets" label="服务工单" />
      </t-tabs>
    </section>

    <section class="detail-content">
      <article v-if="activeTab === 'profile'" class="panel-card surface-card">
        <header class="panel-card__head">
          <h3 class="panel-card__title">基础资料</h3>
        </header>

        <div v-if="loading" class="empty-state">正在加载用户资料…</div>
        <div v-else-if="!userDetail" class="empty-state empty-state--error">未获取到用户信息</div>
        <div v-else class="profile-layout">
          <div class="info-grid">
            <div v-for="field in basicFields" :key="field.label" class="info-item">
              <span class="info-item__label">{{ field.label }}</span>
              <span class="info-item__value" :class="field.valueClass">{{ field.value }}</span>
            </div>
          </div>

          <aside class="profile-side">
            <article class="side-card surface-card">
              <div class="section-label">系统状态</div>
              <div class="timeline-list">
                <div v-for="item in statusTimeline" :key="item.label" class="timeline-item">
                  <span class="timeline-item__dot"></span>
                  <div>
                    <div class="timeline-item__label">{{ item.label }}</div>
                    <div class="timeline-item__value">{{ item.value }}</div>
                  </div>
                </div>
              </div>
            </article>
          </aside>
        </div>
      </article>

      <article v-else-if="activeTab === 'permissions'" class="panel-card surface-card">
        <header class="panel-card__head">
          <h3 class="panel-card__title">角色与权限</h3>
        </header>
        <div class="permission-section">
          <div>
            <div class="section-label">角色列表</div>
            <div class="tag-group">
              <t-tag v-for="role in roleTags" :key="role" theme="primary" variant="light" shape="round">{{ role }}</t-tag>
            </div>
          </div>
          <div>
            <div class="section-label">权限明细</div>
            <div v-if="permissionItems.length" class="permission-grid">
              <div v-for="permission in permissionItems" :key="permission.id" class="permission-item">
                <div class="permission-item__head">
                  <span class="permission-item__name">{{ permission.name }}</span>
                  <t-tag size="small" variant="light" theme="success" shape="round">{{ permissionTypeLabelMap[permission.type] || permission.type }}</t-tag>
                </div>
                <div class="permission-item__code">{{ permission.code }}</div>
                <div v-if="permission.path" class="permission-item__path">{{ permission.path }}</div>
              </div>
            </div>
            <div v-else class="empty-state empty-state--compact">当前用户暂无权限数据</div>
          </div>
        </div>
      </article>

      <article v-else-if="activeTab === 'members'" class="panel-card surface-card">
        <header class="panel-card__head">
          <h3 class="panel-card__title">成员（子账号）</h3>
        </header>

        <div v-if="userDetail?.is_sub_account" class="empty-state empty-state--compact">
          当前账号本身是子账号，归属主账号：{{ userDetail.owner_name || userDetail.owner_user_id || '—' }}
        </div>
        <div v-else-if="membersLoading" class="empty-state">正在加载成员…</div>
        <div v-else-if="members.length" class="member-grid">
          <div v-for="member in members" :key="member.id" class="member-card surface-card">
            <div class="member-card__head">
              <div class="member-card__identity">
                <span class="member-card__name">{{ member.username }}</span>
                <span class="member-card__sub">
                  {{ member.name || member.email || '—' }}
                  <template v-if="member.remark"> · {{ member.remark }}</template>
                </span>
              </div>
              <t-tag
                :theme="member.status === 'active' ? 'success' : 'default'"
                variant="light"
                size="small"
                shape="round"
              >
                {{ statusLabelMap[member.status] || member.status }}
              </t-tag>
            </div>
            <div class="tag-group">
              <t-tag v-for="code in member.permissions" :key="code" theme="primary" variant="light-outline" size="small" shape="round">
                {{ userPermissionLabels[code] || code }}
              </t-tag>
              <span v-if="!member.permissions?.length" class="member-card__sub">未分配权限</span>
            </div>
            <div class="member-card__meta">
              <span>最近登录：{{ member.last_login_at ? formatDateTime(member.last_login_at) : '未登录' }}</span>
              <span>创建时间：{{ formatDateTime(member.created_at) }}</span>
            </div>
          </div>
        </div>
        <div v-else class="empty-state empty-state--compact">该账号下暂无成员</div>
      </article>

      <article v-else-if="activeTab === 'assets'" class="panel-card surface-card">
        <header class="panel-card__head">
          <h3 class="panel-card__title">云主机资产</h3>
        </header>
        <div v-if="instanceItems.length" class="data-grid">
          <div v-for="instance in instanceItems" :key="instance.id" class="data-card">
            <div class="data-card__head">
              <span class="data-card__title">{{ instance.name }}</span>
              <t-tag class="status-tag" :class="`status-tag--${instance.status}`" theme="default" variant="light" size="small" shape="round">
                {{ instanceStatusLabelMap[instance.status] || instance.status }}
              </t-tag>
            </div>
            <div class="data-meta-grid">
              <div class="data-meta-item">
                <span class="data-meta-item__label">地域</span>
                <span class="data-meta-item__value">{{ instance.region }}</span>
              </div>
              <div class="data-meta-item">
                <span class="data-meta-item__label">配置</span>
                <span class="data-meta-item__value">{{ instance.specs }}</span>
              </div>
              <div class="data-meta-item">
                <span class="data-meta-item__label">到期时间</span>
                <span class="data-meta-item__value">{{ formatDateTime(instance.expire_at) }}</span>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="empty-state empty-state--compact">当前用户暂无实例数据</div>
      </article>

      <article v-else-if="activeTab === 'orders'" class="panel-card surface-card">
        <header class="panel-card__head">
          <h3 class="panel-card__title">订单与财务</h3>
        </header>

        <div class="finance-grid finance-grid--wide">
          <div class="finance-item">
            <span class="finance-item__label">账户余额</span>
            <span class="finance-item__value accent-text">{{ balanceText }}</span>
          </div>
          <div class="finance-item">
            <span class="finance-item__label">订单数量</span>
            <span class="finance-item__value">{{ orderItems.length }}</span>
          </div>
          <div class="finance-item">
            <span class="finance-item__label">账单数量</span>
            <span class="finance-item__value">{{ billItems.length }}</span>
          </div>
          <div class="finance-item">
            <span class="finance-item__label">流水数量</span>
            <span class="finance-item__value">{{ transactionItems.length }}</span>
          </div>
        </div>

        <div class="section-block">
          <div class="section-label">最近订单</div>
          <div v-if="orderItems.length" class="data-grid">
            <div v-for="order in orderItems" :key="order.id" class="data-card">
              <div class="data-card__head">
                <span class="data-card__title">{{ order.product }}</span>
                <t-tag class="status-tag" :class="`status-tag--${order.status}`" theme="default" variant="light" size="small" shape="round">
                  {{ orderStatusLabelMap[order.status] || order.status }}
                </t-tag>
              </div>
              <div class="data-meta-grid">
                <div class="data-meta-item">
                  <span class="data-meta-item__label">订单号</span>
                  <span class="data-meta-item__value mono-text">{{ order.order_no }}</span>
                </div>
                <div class="data-meta-item">
                  <span class="data-meta-item__label">金额</span>
                  <span class="data-meta-item__value accent-text">{{ formatAmount(order.amount) }}</span>
                </div>
                <div class="data-meta-item">
                  <span class="data-meta-item__label">下单时间</span>
                  <span class="data-meta-item__value">{{ formatDateTime(order.created_at) }}</span>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="empty-state empty-state--compact">当前用户暂无订单数据</div>
        </div>

        <div class="section-block section-block--split">
          <div>
            <div class="section-label">账单</div>
            <div v-if="billItems.length" class="compact-list">
              <div v-for="bill in billItems" :key="bill.id" class="compact-list__item">
                <div>
                  <div class="compact-list__title">{{ bill.billing_month }}</div>
                  <div class="compact-list__sub">{{ billStatusLabelMap[bill.status] || bill.status }}</div>
                </div>
                <div class="compact-list__value">{{ formatAmount(bill.amount) }}</div>
              </div>
            </div>
            <div v-else class="empty-state empty-state--compact">暂无账单数据</div>
          </div>

          <div>
            <div class="section-label">资金流水</div>
            <div v-if="transactionItems.length" class="compact-list">
              <div v-for="transaction in transactionItems" :key="transaction.id" class="compact-list__item">
                <div>
                  <div class="compact-list__title mono-text">{{ transaction.txn_no }}</div>
                  <div class="compact-list__sub">{{ transactionTypeLabelMap[transaction.type] || transaction.type }} · {{ formatDateTime(transaction.created_at) }}</div>
                </div>
                <div class="compact-list__value" :class="transaction.amount >= 0 ? 'accent-text' : 'warning-text'">
                  {{ formatAmount(transaction.amount) }}
                </div>
              </div>
            </div>
            <div v-else class="empty-state empty-state--compact">暂无流水数据</div>
          </div>
        </div>
      </article>

      <article v-else class="panel-card surface-card">
        <header class="panel-card__head">
          <h3 class="panel-card__title">服务工单</h3>
        </header>
        <div v-if="ticketItems.length" class="data-grid">
          <div v-for="ticket in ticketItems" :key="ticket.id" class="data-card">
            <div class="data-card__head">
              <span class="data-card__title">{{ ticket.title }}</span>
              <t-tag class="status-tag" :class="`status-tag--${ticket.status}`" theme="default" variant="light" size="small" shape="round">
                {{ ticketStatusLabelMap[ticket.status] || ticket.status }}
              </t-tag>
            </div>
            <div class="data-meta-grid">
              <div class="data-meta-item">
                <span class="data-meta-item__label">工单号</span>
                <span class="data-meta-item__value mono-text">{{ ticket.ticket_no }}</span>
              </div>
              <div class="data-meta-item">
                <span class="data-meta-item__label">分类</span>
                <span class="data-meta-item__value">{{ ticket.category }}</span>
              </div>
              <div class="data-meta-item">
                <span class="data-meta-item__label">优先级</span>
                <span class="data-meta-item__value">{{ ticketPriorityLabelMap[ticket.priority] || ticket.priority }}</span>
              </div>
              <div class="data-meta-item">
                <span class="data-meta-item__label">更新时间</span>
                <span class="data-meta-item__value">{{ formatDateTime(ticket.updated_at) }}</span>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="empty-state empty-state--compact">当前用户暂无工单数据</div>
      </article>
    </section>

    <t-dialog v-model:visible="editVisible" header="编辑用户资料" width="640px" :confirm-btn="{ content: '保存修改', loading: saving }" @confirm="handleSubmitEdit">
      <t-form ref="formRef" :data="editForm" :rules="formRules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item label="用户名" name="username">
            <t-input v-model="editForm.username" placeholder="请输入用户名" />
          </t-form-item>
          <t-form-item label="用户状态" name="status">
            <t-select v-model="editForm.status" :options="statusOptions" placeholder="请选择状态" />
          </t-form-item>
          <t-form-item label="邮箱地址" name="email">
            <t-input v-model="editForm.email" placeholder="请输入邮箱地址" />
          </t-form-item>
          <t-form-item label="手机号码" name="phone">
            <t-input v-model="editForm.phone" placeholder="请输入手机号码" />
          </t-form-item>
          <t-form-item label="所属用户组" name="user_group_id">
            <t-select
              v-model="editForm.user_group_id"
              :options="userGroupOptions"
              clearable
              filterable
              placeholder="未分组"
            />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { EditIcon, MoneyIcon, NotificationIcon, SecuredIcon, UserIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRules } from 'tdesign-vue-next'
import {
  getUserDetailAggregate,
  getUserGroupList,
  getUserMembers,
  updateUserDetail,
  type SubAccountMemberInfo,
  type UserBillItem,
  type UserDetailAggregateResponse,
  type UserInfo,
  type UserInstanceItem,
  type UserOrderItem,
  type UserPermissionItem,
  type UserTicketItem,
  type UserTransactionItem,
} from '@/api/user'

defineOptions({ name: 'UserAccountsDetail' })

type DetailTab = 'profile' | 'permissions' | 'members' | 'assets' | 'orders' | 'tickets'

const route = useRoute()
const router = useRouter()
const formRef = ref<FormInstanceFunctions>()

const loading = ref(false)
const saving = ref(false)
const editVisible = ref(false)
const activeTab = ref<DetailTab>('profile')
const userDetail = ref<UserInfo | null>(null)
const aggregateDetail = ref<UserDetailAggregateResponse | null>(null)

const editForm = reactive<{
  username: string
  email: string
  phone: string
  status: string
  user_group_id?: number
}>({
  username: '',
  email: '',
  phone: '',
  status: 'active',
  user_group_id: undefined,
})

// 用户组下拉：包含已禁用组，避免历史归属被编辑时静默清空
const userGroupOptions = ref<{ label: string; value: number }[]>([])

async function loadUserGroupOptions() {
  try {
    const data = await getUserGroupList({ page: 1, page_size: 200 })
    userGroupOptions.value = (data.items || []).map((item) => {
      const suffix: string[] = []
      if (item.is_agent_group) suffix.push('代理')
      if (item.is_default) suffix.push('默认')
      if (item.status === 'disabled') suffix.push('已禁用')
      return { label: suffix.length > 0 ? `${item.name}（${suffix.join('·')}）` : item.name, value: item.id }
    })
  } catch {
    userGroupOptions.value = []
  }
}

const statusLabelMap: Record<string, string> = {
  active: '正常',
  disabled: '已禁用',
  pending: '待审核',
  cancelled: '已注销',
  paid: '已支付',
  completed: '已完成',
  processing: '处理中',
  waiting: '待处理',
  resolved: '已解决',
}

const permissionTypeLabelMap: Record<string, string> = {
  catalog: '目录',
  menu: '菜单',
  button: '按钮',
}

const instanceStatusLabelMap: Record<string, string> = {
  active: '运行中',
  pending: '待开通',
  disabled: '已停用',
}

const orderStatusLabelMap: Record<string, string> = {
  paid: '已支付',
  completed: '已完成',
  pending: '待支付',
}

const billStatusLabelMap: Record<string, string> = {
  paid: '已结清',
  pending: '待结清',
}

const transactionTypeLabelMap: Record<string, string> = {
  recharge: '充值',
  consume: '消费',
}

const ticketStatusLabelMap: Record<string, string> = {
  open: '待受理',
  processing: '处理中',
  waiting: '待回复',
  resolved: '已解决',
}

const ticketPriorityLabelMap: Record<string, string> = {
  high: '高',
  medium: '中',
  low: '低',
}

// 客户侧权限码中文名（与后端 pkg/auth/user_permission.go 保持一致，P4-10）
const userPermissionLabels: Record<string, string> = {
  'instance:view': '查看实例',
  'instance:operate': '实例操作',
  'order:view': '查看订单',
  'order:create': '下单/续费',
  'ticket:view': '查看工单',
  'ticket:submit': '提交工单',
  'billing:view': '查看账单',
  'subaccount:manage': '成员管理',
}

const statusOptions = [
  { label: '正常', value: 'active' },
  { label: '待审核', value: 'pending' },
  { label: '已禁用', value: 'disabled' },
  { label: '已注销', value: 'cancelled' },
]

const formRules: FormRules = {
  username: [{ required: true, message: '请输入用户名', type: 'error' }],
  email: [{ required: true, message: '请输入邮箱地址', type: 'error' }],
  phone: [{ required: true, message: '请输入手机号码', type: 'error' }],
  status: [{ required: true, message: '请选择用户状态', type: 'error' }],
}

const userId = computed(() => String(route.query.id || ''))
const displayInitial = computed(() => (userDetail.value?.username?.slice(0, 1) || 'U').toUpperCase())
const currentStatusLabel = computed(() => {
  const status = userDetail.value?.status || 'active'
  return statusLabelMap[status] || status
})
const balanceText = computed(() => formatAmount(userDetail.value?.balance || 0))

const permissionItems = computed<UserPermissionItem[]>(() => aggregateDetail.value?.permissions || [])
const instanceItems = computed<UserInstanceItem[]>(() => aggregateDetail.value?.instances || [])
const orderItems = computed<UserOrderItem[]>(() => aggregateDetail.value?.orders || [])
const billItems = computed<UserBillItem[]>(() => aggregateDetail.value?.bills || [])
const transactionItems = computed<UserTransactionItem[]>(() => aggregateDetail.value?.transactions || [])
const ticketItems = computed<UserTicketItem[]>(() => aggregateDetail.value?.tickets || [])

const summaryCards = computed(() => [
  { key: 'user-id', label: '用户编号', value: userDetail.value ? `#${userDetail.value.id}` : '--', icon: UserIcon },
  { key: 'status', label: '账户状态', value: currentStatusLabel.value, icon: SecuredIcon },
  { key: 'balance', label: '账户余额', value: balanceText.value, icon: MoneyIcon },
  { key: 'login', label: '最近登录', value: formatDateTime(userDetail.value?.last_login_at), icon: NotificationIcon },
])

// 所属用户组展示：优先用下拉选项里的富标签（含「代理/默认」标注），保证与编辑弹窗一致
const userGroupDisplay = computed(() => {
  const groupId = userDetail.value?.user_group_id
  if (!groupId) return '未分组'
  const option = userGroupOptions.value.find((item) => item.value === groupId)
  return option?.label || userDetail.value?.user_group_name || `用户组 #${groupId}`
})

const basicFields = computed(() => {
  if (!userDetail.value) return []
  return [
    { label: '用户 ID', value: String(userDetail.value.id), valueClass: 'mono-text' },
    { label: '用户名', value: userDetail.value.username || '—', valueClass: 'strong-text' },
    { label: '实名信息', value: userDetail.value.real_name || '未填写', valueClass: '' },
    { label: '邮箱地址', value: userDetail.value.email || '—', valueClass: '' },
    { label: '手机号码', value: userDetail.value.phone || '—', valueClass: 'mono-text' },
    { label: '所属地域', value: userDetail.value.region || '未设置', valueClass: '' },
    { label: '用户等级', value: userDetail.value.user_level_name || '未分级', valueClass: '' },
    { label: '所属用户组', value: userGroupDisplay.value, valueClass: '' },
    { label: '累计消费', value: formatAmount(userDetail.value.total_consume_amount || 0), valueClass: 'accent-text' },
    { label: '账户状态', value: currentStatusLabel.value, valueClass: userDetail.value.status === 'active' ? 'accent-text' : 'warning-text' },
    { label: '账户余额', value: balanceText.value, valueClass: 'accent-text' },
    { label: '角色标识', value: roleTags.value.join('、'), valueClass: '' },
    { label: '注册时间', value: formatDateTime(userDetail.value.created_at), valueClass: '' },
    { label: '最近登录', value: formatDateTime(userDetail.value.last_login_at), valueClass: '' },
  ]
})

const roleTags = computed(() => {
  if (!userDetail.value) return ['暂无角色']
  if (Array.isArray(userDetail.value.roles) && userDetail.value.roles.length > 0) return userDetail.value.roles
  if (userDetail.value.role) return [userDetail.value.role]
  return ['暂无角色']
})

const statusTimeline = computed(() => [
  { label: '账户状态', value: currentStatusLabel.value },
  { label: '注册时间', value: formatDateTime(userDetail.value?.created_at) },
  { label: '最近登录', value: formatDateTime(userDetail.value?.last_login_at) },
  { label: '资料完整度', value: getProfileCompleteness() },
])

function formatDateTime(value?: string) {
  if (!value) return '暂无记录'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function formatAmount(value: number) {
  return `¥ ${Number(value || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function getProfileCompleteness() {
  if (!userDetail.value) return '0 / 4'
  const fields = [userDetail.value.username, userDetail.value.email, userDetail.value.phone, userDetail.value.real_name]
  const completed = fields.filter((item) => String(item || '').trim()).length
  return `${completed} / 4`
}

function syncEditForm() {
  if (!userDetail.value) return
  editForm.username = userDetail.value.username || ''
  editForm.email = userDetail.value.email || ''
  editForm.phone = userDetail.value.phone || ''
  editForm.status = userDetail.value.status || 'active'
  editForm.user_group_id = userDetail.value.user_group_id ?? undefined
}

async function loadUserDetail() {
  if (!userId.value) {
    MessagePlugin.error('缺少用户 ID')
    return
  }
  loading.value = true
  try {
    const data = await getUserDetailAggregate(userId.value)
    aggregateDetail.value = data
    userDetail.value = data.profile
    syncEditForm()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载用户详情失败')
  } finally {
    loading.value = false
  }
}

// 成员（子账号）列表：仅在主账号且首次切到「成员」Tab 时懒加载（P4-10）。
const members = ref<SubAccountMemberInfo[]>([])
const membersLoading = ref(false)
const membersLoaded = ref(false)

async function loadMembers() {
  if (!userId.value || membersLoaded.value) return
  membersLoading.value = true
  try {
    const data = await getUserMembers(userId.value)
    members.value = data.items || []
    membersLoaded.value = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载成员列表失败')
  } finally {
    membersLoading.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'members' && !userDetail.value?.is_sub_account) {
    void loadMembers()
  }
})

function openEditDialog() {
  if (!userDetail.value) return
  syncEditForm()
  void loadUserGroupOptions()
  editVisible.value = true
}

async function handleSubmitEdit() {
  const valid = await formRef.value?.validate()
  if (valid !== true || !userId.value) return
  saving.value = true
  try {
    const data = await updateUserDetail(userId.value, {
      username: editForm.username,
      email: editForm.email,
      phone: editForm.phone,
      status: editForm.status,
      // 0 表示移出分组（未分组），与后端 nil=不修改 / 0=清空 的约定一致
      user_group_id: editForm.user_group_id ?? 0,
    })
    userDetail.value = data
    if (aggregateDetail.value) {
      aggregateDetail.value = {
        ...aggregateDetail.value,
        profile: data,
      }
    }
    editVisible.value = false
    MessagePlugin.success('用户资料已更新')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadUserDetail()
  void loadUserGroupOptions()
})
</script>

<style scoped lang="css">
.user-detail-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-header,
.summary-card,
.panel-card,
.detail-tabs,
.side-card {
  border-radius: var(--hs-radius-lg);
}

.detail-header {
  padding: 20px 24px;
  border: 1px solid #d1fae5;
  background: linear-gradient(135deg, #ffffff 0%, #f7fff9 100%);
}

.detail-header__main {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.detail-header__crumb {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.detail-header__title-row {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
}

.detail-header__identity {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.detail-avatar {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: 14px;
  background: linear-gradient(135deg, var(--color-primary) 0%, var(--td-brand-color-6) 100%);
  color: #ffffff;
  font-size: 22px;
  font-weight: 700;
  flex: none;
}

.detail-header__headline {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.detail-header__title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
}

.detail-header__actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.summary-card {
  padding: 16px;
  border: 1px solid var(--td-brand-color-2);
  background: var(--hs-surface-1);
}

.summary-card__head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.summary-card__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.summary-card__icon {
  color: var(--color-primary);
}

.summary-card__value {
  margin-top: 12px;
  font-size: 20px;
  font-weight: 700;
  color: var(--color-foreground);
}

.detail-tabs,
.panel-card,
.side-card {
  padding: 16px;
  border: 1px solid var(--td-brand-color-2);
  background: var(--hs-surface-1);
}

.detail-content {
  display: flex;
  flex-direction: column;
}

.panel-card__head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 14px;
}

.panel-card__title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

.profile-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 16px;
  align-items: start;
}

.profile-side {
  display: flex;
  flex-direction: column;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 18px;
}

.info-item,
.finance-item,
.timeline-item,
.permission-item,
.data-card,
.compact-list__item,
.empty-state {
  padding: 12px 14px;
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  border: 1px solid #ecfdf3;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.info-item__label,
.section-label,
.finance-item__label,
.timeline-item__label,
.data-meta-item__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.info-item__value,
.finance-item__value,
.timeline-item__value,
.data-meta-item__value {
  color: var(--color-foreground);
  line-height: 1.6;
}

.permission-section,
.timeline-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.tag-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.permission-grid,
.data-grid,
.member-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin-top: 10px;
}

.member-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border-radius: var(--hs-radius-lg);
}

.member-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.member-card__identity {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.member-card__name {
  font-size: 14px;
  font-weight: 700;
}

.member-card__sub {
  color: var(--td-text-color-secondary, #6b7280);
  font-size: 12px;
}

.member-card__meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: var(--td-text-color-placeholder, #9ca3af);
  font-size: 12px;
}

.permission-item,
.data-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.permission-item__head,
.data-card__head,
.compact-list__item {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.permission-item__name,
.data-card__title,
.compact-list__title {
  font-size: 14px;
  font-weight: 700;
  color: var(--color-foreground);
}

.permission-item__code,
.permission-item__path,
.compact-list__sub {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.data-meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 12px;
}

.data-meta-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.section-block {
  margin-top: 16px;
}

.section-block--split {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.compact-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 10px;
}

.compact-list__value {
  font-weight: 700;
}

.finance-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.finance-grid--wide {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.timeline-item {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.timeline-item__dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--color-primary);
  margin-top: 6px;
  flex: none;
}

.empty-state {
  padding: 48px 24px;
  text-align: center;
  color: var(--color-muted-foreground);
}

.empty-state--error {
  color: #b91c1c;
}

.empty-state--compact {
  padding: 18px 16px;
  margin-top: 10px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 16px;
}

.mono-text {
  font-family: var(--hs-font-mono);
  font-size: 12px;
}

.strong-text {
  font-weight: 700;
}

.accent-text {
  color: var(--color-primary);
  font-weight: 700;
}

.warning-text {
  color: #b45309;
  font-weight: 600;
}

:deep(.page-btn.t-button--theme-primary) {
  background-color: var(--color-primary);
  border-color: var(--color-primary);
}

:deep(.page-btn.t-button--theme-primary:hover),
:deep(.page-btn.t-button--theme-primary:focus-visible) {
  background-color: var(--td-brand-color-8);
  border-color: var(--td-brand-color-8);
}

:deep(.page-btn--ghost) {
  color: var(--color-primary);
  border-color: var(--td-brand-color-3);
  background: #ecfdf5;
}

:deep(.page-btn--ghost:hover),
:deep(.page-btn--ghost:focus-visible) {
  color: var(--td-brand-color-8);
  border-color: var(--td-brand-color-4);
  background: var(--td-brand-color-2);
}

:deep(.t-tabs__nav-item.t-is-active) {
  color: var(--color-primary);
}

:deep(.status-tag) {
  border-radius: var(--hs-radius-xl);
  font-weight: 600;
  border: 1px solid transparent;
}

:deep(.status-tag--active),
:deep(.status-tag--paid),
:deep(.status-tag--completed),
:deep(.status-tag--resolved) {
  color: var(--td-brand-color-8);
  background: #ecfdf5;
  border-color: var(--td-brand-color-3);
}

:deep(.status-tag--pending),
:deep(.status-tag--waiting) {
  color: #b45309;
  background: #fffbeb;
  border-color: #fde68a;
}

:deep(.status-tag--disabled),
:deep(.status-tag--cancelled),
:deep(.status-tag--processing),
:deep(.status-tag--open) {
  color: #b91c1c;
  background: #fef2f2;
  border-color: #fecaca;
}

@media (max-width: 1280px) {
  .summary-grid,
  .finance-grid,
  .finance-grid--wide,
  .info-grid,
  .form-grid,
  .profile-layout,
  .permission-grid,
  .data-grid,
  .section-block--split,
  .data-meta-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 960px) {
  .detail-header__title-row,
  .panel-card__head,
  .permission-item__head,
  .data-card__head,
  .compact-list__item {
    flex-direction: column;
    align-items: stretch;
  }

  .summary-grid {
    grid-template-columns: 1fr;
  }
}
</style>
