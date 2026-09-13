<template>
  <div class="page-body console-module order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <OrderIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">我的订单</h2>
          <p class="page-header__desc">共 {{ pagination.total }} 笔订单，点击行可查看费用明细与开通状态</p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/shop')">去购买</t-button>
        <t-button variant="outline" :loading="loading" @click="loadOrders">刷新</t-button>
      </div>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__grid">
        <div class="field">
          <label class="field__label">订单状态</label>
          <t-select v-model="status" placeholder="全部状态" clearable @change="search">
            <t-option v-for="s in STATUS_OPTIONS" :key="s.value" :value="s.value" :label="s.label" />
          </t-select>
        </div>
      </div>
      <p class="filter-card__note">
        订单号与商品名检索请到工单中心提交工单；用户侧列表接口只按状态分页（不做全量搜索，避免漏查被误读为「没有订单」）。
      </p>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">订单列表</h3>
        <span class="table-card__meta">金额为下单时的算价快照</span>
      </div>

      <t-table
        row-key="id"
        :data="orders"
        :columns="columns"
        :loading="loading"
        :pagination="isMobile ? undefined : pagination"
        hover
        cell-empty-content="—"
        @page-change="onPageChange"
        @row-click="onRowClick"
      >
        <template #product_name="{ row }">
          <div class="cell-strong">{{ row.product_name }}</div>
          <div class="cell-sub">{{ row.order_no }}</div>
        </template>

        <template #amount="{ row }">
          <div class="price-cell">
            <span class="price-cell__main">¥{{ payAmount(row).toFixed(2) }}</span>
            <span v-if="row.discount_amount > 0" class="price-cell__sub">
              ¥{{ row.original_amount.toFixed(2) }}
            </span>
          </div>
        </template>

        <template #discount="{ row }">
          <template v-if="row.discount_amount > 0">
            <span class="cell-discount">-¥{{ row.discount_amount.toFixed(2) }}</span>
            <t-tag size="small" variant="light" theme="success">
              {{ sourceLabel(row.discount_source) }}
            </t-tag>
          </template>
          <span v-else>—</span>
        </template>

        <template #spec="{ row }">
          <span v-if="row.cycle || row.quantity > 1" class="cell-sub">
            {{ cycleLabel(row.cycle) }}<template v-if="row.quantity > 1"> × {{ row.quantity }}</template>
          </span>
          <span v-else>—</span>
        </template>

        <template #actor_name="{ row }">
          <span :class="{ 'cell-sub': !row.actor_name }">{{ row.actor_name || '主账号' }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
            {{ statusText(row.status) }}
          </t-tag>
          <t-tag
            v-if="row.provision_status === 'failed' || row.provision_status === 'manual'"
            theme="danger"
            variant="light"
            size="small"
          >
            开通异常
          </t-tag>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #op="{ row }">
          <span class="action-cell">
            <t-button variant="text" size="small" @click.stop="router.push(`/order/${row.id}`)">
              详情
            </t-button>
          </span>
        </template>
      </t-table>

      <!-- 移动端翻页：与 admin 列表页同一套（桌面用表格内建分页） -->
      <MobilePagination
        v-if="isMobile"
        :current="pagination.current"
        :page-size="pagination.pageSize"
        :total="pagination.total"
        @change="onPageChange"
      />
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { PrimaryTableCol } from 'tdesign-vue-next'
import { OrderIcon } from 'tdesign-icons-vue-next'

import { getMyOrders, type OrderInfo } from '@/api/shop'
import { useIsMobile } from '@/composables/useIsMobile'
import MobilePagination from '@/components/mobile-pagination/index.vue'

defineOptions({ name: 'OrderList' })

const router = useRouter()
const { isMobile } = useIsMobile()

const orders = ref<OrderInfo[]>([])
const loading = ref(false)
const status = ref('')
const pagination = reactive({ current: 1, pageSize: 10, total: 0 })

const STATUS_OPTIONS = [
  { value: 'paid', label: '已支付' },
  { value: 'provisioning', label: '开通中' },
  { value: 'active', label: '服务中' },
  { value: 'cancelled', label: '已取消' },
  { value: 'refunded', label: '已退款' },
]

const columns: PrimaryTableCol<OrderInfo>[] = [
  { colKey: 'product_name', title: '产品', minWidth: 180 },
  { colKey: 'spec', title: '规格/周期', width: 140 },
  { colKey: 'amount', title: '实付金额', width: 130 },
  { colKey: 'discount', title: '优惠', width: 160 },
  { colKey: 'actor_name', title: '操作人', width: 110 },
  { colKey: 'status', title: '状态', width: 150 },
  { colKey: 'created_at', title: '下单时间', width: 160 },
  { colKey: 'op', title: '操作', width: 90, fixed: 'right' },
]

/**
 * 列表直接渲染后端返回：ListQuery 只支持 status + 分页，没有订单号/商品名检索
 * （那是管理端能力）。不在前端假装支持搜索，否则用户搜不到会误以为订单不存在。
 */

/** 实付优先取算价快照，兼容未写快照的存量订单。 */
function payAmount(row: OrderInfo): number {
  return row.final_amount || row.paid_amount
}

/** 折扣来源中文标签（P5-06）：折扣仅由用户组价格策略承载。 */
function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
  }
  return map[source] || source || '优惠'
}

function cycleLabel(c?: string): string {
  if (!c) return ''
  const map: Record<string, string> = {
    hourly: '按小时',
    monthly: '按月',
    quarterly: '按季',
    semiannually: '半年',
    annually: '按年',
    biennially: '两年',
    onetime: '一次性',
  }
  return map[c] || c
}

function statusText(s: string): string {
  return (
    {
      pending: '待支付',
      paid: '已支付',
      provisioning: '开通中',
      active: '服务中',
      cancelled: '已取消',
      refunded: '已退款',
    }[s] || s
  )
}

function statusTheme(s: string): 'success' | 'warning' | 'default' | 'danger' {
  if (s === 'active' || s === 'paid') return 'success'
  if (s === 'provisioning' || s === 'pending') return 'warning'
  if (s === 'cancelled' || s === 'refunded') return 'danger'
  return 'default'
}

function formatTime(v?: string): string {
  return v ? v.replace('T', ' ').slice(0, 16) : '—'
}

async function loadOrders() {
  loading.value = true
  try {
    const { data } = await getMyOrders({
      status: status.value || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    orders.value = data?.items || []
    pagination.total = data?.total || 0
  } catch {
    orders.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

function search() {
  pagination.current = 1
  loadOrders()
}

function onPageChange(info: { current: number; pageSize: number }) {
  pagination.current = info.current
  pagination.pageSize = info.pageSize
  loadOrders()
}

function onRowClick({ row }: { row: OrderInfo }) {
  router.push(`/order/${row.id}`)
}

onMounted(loadOrders)
</script>

<style scoped>
.cell-discount {
  margin-right: 6px;
  color: var(--color-accent);
  font-variant-numeric: tabular-nums;
}

.order-module :deep(.t-table__row) {
  cursor: pointer;
}
</style>
