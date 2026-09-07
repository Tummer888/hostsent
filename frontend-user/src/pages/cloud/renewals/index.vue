<template>
  <div class="renewals-page">
    <!-- 页头 -->
    <section class="renewals-hero">
      <div class="hero-left">
        <span class="hero-chip"><RefreshIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">续费管理</span>
          <span class="hero-desc">
            管理实例续费与自动续费；到期前 {{ policy.remind_days || '7,3,1' }} 天提醒，宽限期 {{ policy.grace_days }} 天内可恢复。
          </span>
        </div>
      </div>
      <t-button variant="outline" size="large" :loading="loading" @click="loadAll">
        <template #icon><RefreshIcon /></template>
        刷新
      </t-button>
    </section>

    <!-- 我的实例续费卡片 -->
    <section class="instance-section">
      <div class="section-head">
        <span class="section-title">我的实例</span>
        <span class="section-sub">共 {{ instances.length }} 个实例</span>
      </div>
      <t-loading :loading="loading" show-overlay>
        <div v-if="instances.length" class="instance-grid">
          <div v-for="item in instances" :key="item.id" class="instance-card">
            <div class="instance-card__head">
              <div class="instance-card__title">
                <span class="mark">{{ item.instance_mark }}</span>
                <span class="name">{{ item.name || item.product_name }}</span>
              </div>
              <t-tag :theme="stageTheme(item.stage)" variant="light" size="small" shape="round">
                {{ stageLabel(item.stage) }}
              </t-tag>
            </div>
            <div class="instance-card__body">
              <div class="meta-row">
                <span class="meta-label">产品</span>
                <span>{{ item.product_name || '—' }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">到期时间</span>
                <span>{{ formatTime(item.expire_at) }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">剩余天数</span>
                <span :class="daysLeftClass(item.days_left)">{{ daysLeftText(item.days_left) }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">续费单价</span>
                <span>¥{{ formatAmount(item.unit_price) }}/{{ billingModeLabel(item.billing_mode) }}</span>
              </div>
            </div>
            <div class="instance-card__actions">
              <t-input-number v-model="periodMap[item.id]" :min="1" :max="36" theme="column" size="small" style="width: 110px" />
              <t-button theme="primary" size="small" :loading="renewingId === item.id" @click="handleRenew(item)">
                立即续费
              </t-button>
              <t-switch
                :value="item.auto_renew"
                size="small"
                :label="['自动续费', '自动续费']"
                @change="(val: boolean) => handleToggleAuto(item, val)"
              />
            </div>
          </div>
        </div>
        <t-empty v-else description="暂无实例，先去开通一台云主机吧" />
      </t-loading>
    </section>

    <!-- 我的续费记录 -->
    <section class="records-section">
      <div class="section-head">
        <span class="section-title">续费记录</span>
        <t-select v-model="recordStatus" clearable size="small" style="width: 140px" placeholder="全部状态" :options="statusOptions" @change="loadRecords" />
      </div>
      <t-loading :loading="recordsLoading" show-overlay>
        <t-table
          :data="records"
          :columns="columns"
          row-key="id"
          :pagination="recordPagination"
          :bordered="false"
          hover
          size="small"
          cell-empty-content="—"
          @page-change="onRecordsPageChange"
        >
          <template #renewal_no="{ row }">
            <t-link theme="primary" hover="color" @click="openDetail(row.id)">{{ row.renewal_no }}</t-link>
          </template>
          <template #amount="{ row }">
            <span class="amount-text">¥{{ formatAmount(row.amount) }}</span>
            <span class="amount-sub">× {{ row.period_count }} 周期</span>
          </template>
          <template #source="{ row }">
            <t-tag variant="light" size="small" shape="round">{{ sourceLabel(row.source) }}</t-tag>
          </template>
          <template #status="{ row }">
            <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">{{ statusLabel(row.status) }}</t-tag>
          </template>
          <template #expire_change="{ row }">
            <span class="time-text">{{ formatTime(row.expire_before) }} → {{ formatTime(row.expire_after) }}</span>
          </template>
          <template #created_at="{ row }">
            <span class="time-text">{{ formatTime(row.created_at) }}</span>
          </template>
          <template #empty>
            <t-empty description="暂无续费记录" />
          </template>
        </t-table>
      </t-loading>
    </section>

    <!-- 续费详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="续费详情" size="400px" :footer="false">
      <t-loading :loading="detailLoading" show-overlay>
        <div v-if="detail" class="detail-list">
          <div class="detail-row"><span>续费单号</span><b>{{ detail.renewal_no }}</b></div>
          <div class="detail-row"><span>状态</span>
            <t-tag :theme="statusTheme(detail.status)" variant="light" size="small">{{ statusLabel(detail.status) }}</t-tag>
          </div>
          <div class="detail-row"><span>实例</span>{{ detail.instance_mark }}（{{ detail.product_name }}）</div>
          <div class="detail-row"><span>金额</span>¥{{ formatAmount(detail.amount) }} × {{ detail.period_count }} 周期</div>
          <div class="detail-row"><span>关联订单</span>{{ detail.order_no || '—' }}</div>
          <div class="detail-row"><span>支付时间</span>{{ formatTime(detail.pay_time) }}</div>
          <div class="detail-row"><span>续费前到期</span>{{ formatTime(detail.expire_before) }}</div>
          <div class="detail-row"><span>续费后到期</span>{{ formatTime(detail.expire_after) }}</div>
          <div class="detail-row"><span>创建时间</span>{{ formatTime(detail.created_at) }}</div>
          <div v-if="detail.fail_reason" class="detail-row"><span>失败原因</span><span class="fail-text">{{ detail.fail_reason }}</span></div>
        </div>
      </t-loading>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getMyRenewalDetail,
  getMyRenewals,
  getRenewalsView,
  renewMyInstance,
  toggleMyAutoRenew,
  type LifecyclePolicySummary,
  type RenewalInfo,
  type UserInstanceRenewalItem,
} from '@/api/lifecycle'

defineOptions({ name: 'UserRenewals' })

const loading = ref(false)
const instances = ref<UserInstanceRenewalItem[]>([])
const policy = reactive<Partial<LifecyclePolicySummary>>({ remind_days: '7,3,1', grace_days: 7 })
const periodMap = reactive<Record<number, number>>({})

const stageMap: Record<string, { label: string; theme: string }> = {
  active: { label: '运行中', theme: 'success' },
  expiring: { label: '即将到期', theme: 'warning' },
  grace: { label: '宽限期', theme: 'warning' },
  suspended: { label: '已暂停', theme: 'danger' },
  destroyed: { label: '待销毁', theme: 'danger' },
}

const billingMap: Record<string, string> = {
  hourly: '小时',
  daily: '天',
  monthly: '月',
  yearly: '年',
}

const statusOptions = [
  { label: '待支付', value: 'pending' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '已取消', value: 'cancelled' },
]

const statusMap: Record<string, { label: string; theme: string }> = {
  pending: { label: '待支付', theme: 'warning' },
  success: { label: '成功', theme: 'success' },
  failed: { label: '失败', theme: 'danger' },
  cancelled: { label: '已取消', theme: 'default' },
}

const sourceMap: Record<string, string> = {
  manual: '用户手动',
  auto: '自动续费',
  admin: '管理员代续',
}

function stageLabel(s: string): string {
  return stageMap[s]?.label || s
}

function stageTheme(s: string): string {
  return stageMap[s]?.theme || 'default'
}

function billingModeLabel(m: string): string {
  return billingMap[m] || m
}

function statusLabel(s: string): string {
  return statusMap[s]?.label || s
}

function statusTheme(s: string): string {
  return statusMap[s]?.theme || 'default'
}

function sourceLabel(s: string): string {
  return sourceMap[s] || s
}

function formatTime(v?: string): string {
  if (!v) return '—'
  return v.replace('T', ' ').slice(0, 19)
}

function formatAmount(v?: number): string {
  return (v ?? 0).toFixed(2)
}

function daysLeftText(days: number): string {
  if (days > 0) return `剩余 ${days} 天`
  if (days === 0) return '今天到期'
  return `已过期 ${-days} 天`
}

function daysLeftClass(days: number): string {
  if (days <= 0) return 'days-danger'
  if (days <= 7) return 'days-warn'
  return ''
}

// —— 数据加载 ——
async function loadView() {
  loading.value = true
  try {
    const { data } = await getRenewalsView()
    instances.value = data?.items || []
    Object.assign(policy, data?.policy || {})
    instances.value.forEach((item) => {
      if (!periodMap[item.id]) periodMap[item.id] = item.auto_period || 1
    })
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载续费管理数据失败')
  } finally {
    loading.value = false
  }
}

// —— 续费操作 ——
const renewingId = ref(0)

function handleRenew(item: UserInstanceRenewalItem) {
  const periods = periodMap[item.id] || 1
  const total = (item.unit_price || 0) * periods
  const dialog = DialogPlugin.confirm({
    header: '确认续费',
    body: `确认为实例 ${item.instance_mark} 续费 ${periods} 个周期？将立即从账户余额扣除 ¥${formatAmount(total)}。`,
    confirmBtn: { content: '确认续费', theme: 'primary' },
    onConfirm: async () => {
      renewingId.value = item.id
      try {
        await renewMyInstance(item.id, periods)
        MessagePlugin.success('续费成功，实例到期时间已延长')
        dialog.destroy()
        loadView()
        loadRecords()
      } catch (e) {
        MessagePlugin.error((e as Error).message || '续费失败')
      } finally {
        renewingId.value = 0
      }
    },
    onClose: () => dialog.destroy(),
  })
}

async function handleToggleAuto(item: UserInstanceRenewalItem, enabled: boolean) {
  try {
    await toggleMyAutoRenew(item.id, enabled, item.auto_period || 1)
    MessagePlugin.success(enabled ? '已开启自动续费' : '已关闭自动续费')
    loadView()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '操作失败')
    loadView()
  }
}

// —— 续费记录 ——
const recordsLoading = ref(false)
const records = ref<RenewalInfo[]>([])
const recordsTotal = ref(0)
const recordStatus = ref('')
const recordsPage = reactive({ current: 1, size: 10 })

const columns: PrimaryTableCol[] = [
  { colKey: 'renewal_no', title: '续费单号', width: 170 },
  { colKey: 'instance_mark', title: '实例', width: 150 },
  { colKey: 'amount', title: '金额', width: 130 },
  { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'expire_change', title: '到期变化', width: 270 },
  { colKey: 'created_at', title: '创建时间', width: 160 },
]

const recordPagination = computed(() => ({
  current: recordsPage.current,
  pageSize: recordsPage.size,
  total: recordsTotal.value,
  showJumper: true,
}))

async function loadRecords() {
  recordsLoading.value = true
  try {
    const { data } = await getMyRenewals({
      status: recordStatus.value || undefined,
      page: recordsPage.current,
      page_size: recordsPage.size,
    })
    records.value = data?.items || []
    recordsTotal.value = data?.meta?.total ?? 0
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载续费记录失败')
  } finally {
    recordsLoading.value = false
  }
}

function onRecordsPageChange(info: PageInfo) {
  recordsPage.current = info.current
  recordsPage.size = info.pageSize
  loadRecords()
}

// —— 详情 ——
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<RenewalInfo | null>(null)

async function openDetail(id: number) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    const { data } = await getMyRenewalDetail(id)
    detail.value = data
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载详情失败')
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

function loadAll() {
  loadView()
  loadRecords()
}

onMounted(loadAll)
</script>

<style scoped>
.renewals-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px 24px 40px;
}
.renewals-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 24px 28px;
  border-radius: 14px;
  background: linear-gradient(135deg, #eef2ff 0%, #e0e7ff 100%);
  border: 1px solid #e0e7ff;
}
.hero-left {
  display: flex;
  align-items: center;
  gap: 14px;
}
.hero-chip {
  width: 46px;
  height: 46px;
  border-radius: 12px;
  background: linear-gradient(135deg, #6366f1, #4f46e5);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 10px rgba(99, 102, 241, 0.25);
  flex-shrink: 0;
}
.hero-info {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.hero-label {
  font-size: 20px;
  font-weight: 700;
  color: #1e1b4b;
}
.hero-desc {
  font-size: 13px;
  color: #6366f1;
}
.instance-section,
.records-section {
  padding: 20px 24px;
  border-radius: 14px;
  background: #fff;
  border: 1px solid #eef1f6;
}
.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.section-title {
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}
.section-sub {
  font-size: 12px;
  color: #94a3b8;
}
.instance-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
}
.instance-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 18px 20px;
  border-radius: 12px;
  border: 1px solid #eef1f6;
  background: #fbfcfe;
  transition: box-shadow 0.2s, border-color 0.2s;
}
.instance-card:hover {
  border-color: #c7d2fe;
  box-shadow: 0 4px 14px rgba(99, 102, 241, 0.1);
}
.instance-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.instance-card__title {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.instance-card__title .mark {
  font-size: 14px;
  font-weight: 700;
  color: #1e293b;
}
.instance-card__title .name {
  font-size: 12px;
  color: #94a3b8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.instance-card__body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.meta-row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  color: #475569;
}
.meta-label {
  color: #94a3b8;
}
.days-warn {
  color: #d97706;
  font-weight: 600;
}
.days-danger {
  color: #dc2626;
  font-weight: 700;
}
.instance-card__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  border-top: 1px dashed #eef1f6;
  padding-top: 12px;
}
.amount-text {
  font-weight: 700;
  color: #1e293b;
}
.amount-sub {
  margin-left: 6px;
  font-size: 12px;
  color: #94a3b8;
}
.time-text {
  font-size: 12px;
  color: #64748b;
  font-variant-numeric: tabular-nums;
}
.detail-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.detail-row {
  display: flex;
  gap: 12px;
  font-size: 13px;
  color: #475569;
}
.detail-row > span:first-child {
  flex: 0 0 88px;
  color: #94a3b8;
}
.fail-text {
  color: #dc2626;
}
@media (max-width: 768px) {
  .renewals-hero {
    flex-direction: column;
    align-items: stretch;
  }
  .instance-grid {
    grid-template-columns: 1fr;
  }
}
</style>
