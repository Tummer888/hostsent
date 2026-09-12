<template>
  <div class="page-body instances-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ServerIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <div class="page-header__title-row">
            <h2 class="page-header__title">{{ instanceData?.name || '实例详情' }}</h2>
            <t-tag
              v-if="instanceData"
              :theme="instanceStatusTheme(instanceData.status)"
              variant="light"
              size="small"
              shape="round"
            >
              {{ instanceStatusLabel(instanceData.status) }}
            </t-tag>
          </div>
          <p class="page-header__desc">
            实例标识 {{ instanceData?.instance_id || '—' }} · 公网 IP {{ instanceData?.public_ip || '—' }}
          </p>
        </div>
      </div>
      <div class="detail-actions">
        <t-button variant="outline" @click="goBack">返回</t-button>
        <t-button variant="outline" :loading="loading" @click="reloadAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button
          v-permission="'instance:action'"
          variant="outline"
          :disabled="powerDisabled('on')"
          :title="powerTitle('on')"
          @click="handlePower('on')"
        >
          开机
        </t-button>
        <t-button
          v-permission="'instance:action'"
          variant="outline"
          :disabled="powerDisabled('off')"
          :title="powerTitle('off')"
          @click="handlePower('off')"
        >
          关机
        </t-button>
        <t-button
          v-permission="'instance:action'"
          variant="outline"
          :disabled="powerDisabled('reboot')"
          :title="powerTitle('reboot')"
          @click="handlePower('reboot')"
        >
          重启
        </t-button>
        <t-button
          v-permission="'instance:console'"
          variant="outline"
          :disabled="consoleDisabled"
          :title="consoleDisabled ? consoleTitle : ''"
          @click="handleVnc"
        >
          控制台
        </t-button>
        <t-button v-permission="'instance:action'" variant="outline" :loading="syncing" @click="handleSync">
          同步刷新
        </t-button>
        <t-button v-permission="'instance:action'" variant="outline" @click="openRemarkDialog">备注</t-button>
        <t-button
          v-permission="'instance:resize'"
          variant="outline"
          :disabled="!instanceData?.capabilities?.resize"
          :title="capabilityTitle(instanceData?.capabilities?.resize)"
          @click="openResizeDialog"
        >
          变配
        </t-button>
        <t-button v-permission="'lifecycle:renew'" variant="outline" @click="openRenewDialog">续费</t-button>
        <t-button
          v-permission="'instance:destroy'"
          theme="danger"
          :disabled="!instanceData?.capabilities?.destroy"
          :title="capabilityTitle(instanceData?.capabilities?.destroy)"
          @click="openDestroyDialog"
        >
          销毁
        </t-button>
      </div>
    </header>

    <t-alert
      v-if="instanceData?.capability_error"
      theme="warning"
      :message="`部分操作不可用：${instanceData.capability_error}`"
    />

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <t-tab-panel value="overview" label="概览">
          <div class="tabs-section">
            <t-descriptions v-if="instanceData" :column="2" bordered size="medium" class="detail-desc">
              <t-descriptions-item label="实例标识">{{ instanceData.instance_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="归属用户">
                <span class="cell-strong">{{ instanceData.username || '—' }}</span>
                <div class="cell-sub">
                  {{ instanceData.user_email || '—' }} · {{ instanceData.user_phone || '—' }}
                </div>
              </t-descriptions-item>
              <t-descriptions-item label="用户 ID">{{ instanceData.user_id }}</t-descriptions-item>
              <t-descriptions-item label="来源订单">
                {{ instanceData.order_id > 0 ? instanceData.order_id : '—' }}
              </t-descriptions-item>
              <t-descriptions-item label="服务商">
                {{ instanceData.provider_name || '—' }}
                <span v-if="instanceData.provider_type" class="cell-sub">（{{ instanceData.provider_type }}）</span>
              </t-descriptions-item>
              <t-descriptions-item label="售出商品 ID">{{ instanceData.sell_product_id || instanceData.upstream_product_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="规格">
                {{ instanceData.cpu }} 核 / {{ instanceData.memory }} MB / {{ instanceData.disk }} GB
              </t-descriptions-item>
              <t-descriptions-item label="磁盘类型">{{ instanceData.disk_type || '—' }}</t-descriptions-item>
              <t-descriptions-item label="带宽">
                {{ instanceData.bandwidth ? `${instanceData.bandwidth} Mbps` : '—' }}
              </t-descriptions-item>
              <t-descriptions-item label="系统">{{ instanceData.os || '—' }}</t-descriptions-item>
              <t-descriptions-item label="区域 / 可用区">
                {{ instanceData.region || '—' }} / {{ instanceData.zone || '—' }}
              </t-descriptions-item>
              <t-descriptions-item label="计费模式">{{ instanceData.billing_mode || '—' }}</t-descriptions-item>
              <t-descriptions-item label="内网 IP">{{ instanceData.private_ip || '—' }}</t-descriptions-item>
              <t-descriptions-item label="公网 IP">{{ instanceData.public_ip || '—' }}</t-descriptions-item>
              <t-descriptions-item label="操作人">{{ instanceData.actor_name || '主账号' }}</t-descriptions-item>
              <t-descriptions-item label="服务状态">
                <t-tag :theme="instanceStatusTheme(instanceData.status)" variant="light" size="small" shape="round">
                  {{ instanceStatusLabel(instanceData.status) }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="电源状态">{{ powerStatusText(instanceData.power_status) }}</t-descriptions-item>
              <t-descriptions-item label="到期时间">
                <span :class="expireClass(instanceData)">
                  {{ instanceData.expire_at ? formatTime(instanceData.expire_at) : '—' }}
                  <template v-if="instanceData.expire_state !== 'none' && instanceData.expire_at">
                    （{{ expireStateLabel(instanceData.expire_state) }}，剩余 {{ instanceData.days_left }} 天）
                  </template>
                </span>
              </t-descriptions-item>
              <t-descriptions-item label="最近回源时间">{{ formatTime(instanceData.last_synced_at) }}</t-descriptions-item>
              <t-descriptions-item label="创建时间">{{ formatTime(instanceData.created_at) }}</t-descriptions-item>
              <t-descriptions-item label="更新时间">{{ formatTime(instanceData.updated_at) }}</t-descriptions-item>
              <t-descriptions-item label="管理员备注">{{ instanceData.remark || '—' }}</t-descriptions-item>
            </t-descriptions>
            <t-empty v-else description="暂无数据" />
          </div>
        </t-tab-panel>

        <t-tab-panel value="operations" label="操作记录">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">操作记录</h3>
              <span class="table-card__meta">共 {{ opPagination.total }} 条</span>
            </div>
            <t-table
              row-key="id"
              :data="operations"
              :columns="operationColumns"
              :loading="opLoading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
              :pagination="opPagination"
              @page-change="handleOpPageChange"
            >
              <template #action="{ row }">
                <span>{{ operationActionLabel(row.action) }}</span>
              </template>
              <template #result="{ row }">
                <t-tag
                  :theme="row.result === 'success' ? 'success' : 'danger'"
                  variant="light"
                  size="small"
                  shape="round"
                >
                  {{ row.result === 'success' ? '成功' : '失败' }}
                </t-tag>
              </template>
              <template #operator="{ row }">
                <div>
                  <span class="cell-strong">{{ operatorTypeLabel(row.operator_type) }}</span>
                  <div class="cell-sub">{{ row.operator_name || '—' }}</div>
                </div>
              </template>
              <template #status_change="{ row }">
                <span class="cell-muted">{{ row.before_status || '—' }}</span>
                <span class="status-arrow">→</span>
                <span class="cell-muted">{{ row.after_status || '—' }}</span>
              </template>
              <template #detail="{ row }">
                <t-tooltip v-if="row.result === 'failed' && row.error_message" :content="row.error_message">
                  <span class="op-error">{{ row.error_message }}</span>
                </t-tooltip>
                <span v-else>—</span>
              </template>
              <template #created_at="{ row }">
                <span class="time-text">{{ formatTime(row.created_at) }}</span>
              </template>
              <template #empty>
                <t-empty description="暂无操作记录" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>

        <t-tab-panel value="related" label="关联记录">
          <div class="tabs-section">
            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">关联订单</h3>
                <span class="table-card__meta">共 {{ related.orders.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.orders"
                :columns="relatedOrderColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #order_no="{ row }">
                  <t-link theme="primary" hover="color" @click="openOrder(row)">{{ row.order_no }}</t-link>
                </template>
                <template #total_amount="{ row }">
                  <span>¥{{ formatPrice(row.total_amount) }}</span>
                </template>
                <template #paid_amount="{ row }">
                  <span>¥{{ formatPrice(row.paid_amount) }}</span>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无关联订单" />
                </template>
              </t-table>
            </div>

            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">续费记录</h3>
                <span class="table-card__meta">共 {{ related.renewals.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.renewals"
                :columns="relatedRenewalColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #amount="{ row }">
                  <span>¥{{ formatPrice(row.amount) }}</span>
                </template>
                <template #expire_before="{ row }">
                  <span class="time-text">{{ formatTime(row.expire_before) }}</span>
                </template>
                <template #expire_after="{ row }">
                  <span class="time-text">{{ formatTime(row.expire_after) }}</span>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无续费记录" />
                </template>
              </t-table>
            </div>

            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">关联工单</h3>
                <span class="table-card__meta">共 {{ related.tickets.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.tickets"
                :columns="relatedTicketColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #ticket_no="{ row }">
                  <t-link theme="primary" hover="color" @click="openTicket(row)">{{ row.ticket_no }}</t-link>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无关联工单" />
                </template>
              </t-table>
            </div>
          </div>
        </t-tab-panel>
      </t-tabs>
    </section>

    <t-dialog
      v-model:visible="remarkVisible"
      header="实例备注"
      width="480px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSaveRemark"
      @close="remarkVisible = false"
    >
      <t-form label-align="top" :data="remarkForm" @submit.prevent>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="remarkForm.remark" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="请输入实例备注" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="resizeVisible"
      header="变配"
      width="520px"
      :confirm-btn="{ content: '提交变配', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleResize"
      @close="resizeVisible = false"
    >
      <t-form label-align="top" :data="resizeForm" @submit.prevent>
        <t-form-item label="CPU（核）" name="cpu">
          <t-input-number v-model="resizeForm.cpu" :min="1" :max="256" theme="column" />
        </t-form-item>
        <t-form-item label="内存（MB）" name="memory">
          <t-input-number v-model="resizeForm.memory" :min="128" :step="128" theme="column" />
        </t-form-item>
        <t-form-item label="磁盘（GB）" name="disk">
          <t-input-number v-model="resizeForm.disk" :min="1" :step="10" theme="column" />
        </t-form-item>
        <t-form-item label="磁盘类型" name="disk_type">
          <t-input v-model="resizeForm.disk_type" placeholder="如 cloud_ssd" />
        </t-form-item>
        <t-form-item label="变配原因" name="reason">
          <t-textarea v-model="resizeForm.reason" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，请填写变配原因" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="destroyVisible"
      header="销毁实例"
      width="520px"
      :confirm-btn="{
        content: '确认销毁',
        theme: 'danger',
        disabled: destroyConfirmDisabled,
      }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleDestroy"
      @close="destroyVisible = false"
    >
      <t-form label-align="top" :data="destroyForm" @submit.prevent>
        <t-form-item>
          <t-alert
            theme="error"
            :message="`销毁后实例与数据不可恢复。请输入实例标识「${instanceData?.instance_id || ''}」以确认。`"
          />
        </t-form-item>
        <t-form-item label="实例标识确认" name="confirm_mark">
          <t-input v-model="destroyForm.confirm_mark" :placeholder="instanceData?.instance_id || '请输入实例标识'" />
        </t-form-item>
        <t-form-item label="销毁原因" name="reason">
          <t-textarea v-model="destroyForm.reason" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，请填写销毁原因" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="renewVisible"
      header="续费"
      width="520px"
      :confirm-btn="{ content: '提交续费', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleRenew"
      @close="renewVisible = false"
    >
      <t-form label-align="top" :data="renewForm" @submit.prevent>
        <t-form-item label="续费时长（周期）" name="period_count">
          <t-input-number v-model="renewForm.period_count" :min="1" :max="120" theme="column" />
        </t-form-item>
        <t-form-item label="金额（元）" name="amount">
          <t-input-number v-model="renewForm.amount" :min="0" :precision="2" theme="column" placeholder="留空按默认价格" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="renewForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，请填写代续费备注" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="vnc.visible"
      :header="`远程控制台 · ${instanceData?.name || vnc.name}`"
      width="80%"
      :footer="false"
      destroy-on-close
      @close="vnc.visible = false"
    >
      <div class="vnc-wrap">
        <iframe v-if="vnc.url && isHttp(vnc.url)" :src="vnc.url" class="vnc-frame" frameborder="0" />
        <t-empty v-else description="控制台地址无法内嵌显示，请点击打开">
          <template #action>
            <t-button theme="primary" @click="openVnc">打开控制台</t-button>
          </template>
        </t-empty>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { RefreshIcon, ServerIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  destroyInstance,
  getInstanceDetail,
  getInstanceOperations,
  getInstanceRelated,
  getInstanceVNC,
  powerInstance,
  resizeInstance,
  syncInstance,
  updateInstanceRemark,
} from '@/api/instance'
import { renewInstance } from '@/api/lifecycle'
import {
  expireStateLabel,
  formatPrice,
  formatTime,
  instanceStatusLabel,
  instanceStatusTheme,
  operationActionLabel,
  operatorTypeLabel,
  powerActionLabel,
  powerActionTips,
} from '@/pages/instances/constants'
import type {
  InstanceDetail,
  InstanceRelatedResponse,
  OperationItem,
  RelatedOrder,
  RelatedRenewal,
  RelatedTicket,
} from '@/types/interface'

defineOptions({ name: 'InstanceDetail' })

const route = useRoute()
const router = useRouter()

const instanceId = Number(route.params.id)
const instanceData = ref<InstanceDetail | null>(null)
const loading = ref(false)
const syncing = ref(false)
const activeTab = ref('overview')

const operations = ref<OperationItem[]>([])
const opLoading = ref(false)
const opPagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const related = reactive<InstanceRelatedResponse>({ orders: [], renewals: [], tickets: [] })
const relatedLoading = ref(false)

const operationColumns: PrimaryTableCol<OperationItem>[] = [
  { colKey: 'created_at', title: '时间', width: 170 },
  { colKey: 'action', title: '动作', width: 110 },
  { colKey: 'result', title: '结果', width: 90 },
  { colKey: 'operator', title: '操作人', width: 140 },
  { colKey: 'status_change', title: '状态变化', minWidth: 200 },
  { colKey: 'detail', title: '详情', minWidth: 180 },
]

const relatedOrderColumns: PrimaryTableCol<RelatedOrder>[] = [
  { colKey: 'order_no', title: '订单号', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'product_name', title: '产品名称', minWidth: 160 },
  { colKey: 'total_amount', title: '应付', width: 110 },
  { colKey: 'paid_amount', title: '实付', width: 110 },
  { colKey: 'pay_method', title: '支付方式', width: 110 },
  { colKey: 'created_at', title: '下单时间', width: 170 },
]

const relatedRenewalColumns: PrimaryTableCol<RelatedRenewal>[] = [
  { colKey: 'renewal_no', title: '续费单号', minWidth: 180 },
  { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'period_count', title: '周期', width: 80, align: 'center' as const },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'expire_before', title: '原到期', width: 170 },
  { colKey: 'expire_after', title: '新到期', width: 170 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

const relatedTicketColumns: PrimaryTableCol<RelatedTicket>[] = [
  { colKey: 'ticket_no', title: '工单号', minWidth: 170 },
  { colKey: 'title', title: '标题', minWidth: 200 },
  { colKey: 'priority', title: '优先级', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'assigned_name', title: '处理人', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

function powerStatusText(status: string): string {
  if (status === 'on') return '开机'
  if (status === 'off') return '关机'
  return '未知'
}

function expireClass(row: InstanceDetail): string {
  if (row.expire_state === 'expired') return 'expire-danger'
  if (row.expire_state === 'expiring') return 'expire-warning'
  return ''
}

function capabilityTitle(supported: boolean | undefined): string {
  return supported ? '' : '该服务商不支持此操作'
}

const consoleDisabled = computed(
  () => !instanceData.value?.capabilities?.console || instanceData.value?.status !== 'running',
)

const consoleTitle = computed(() => {
  if (!instanceData.value?.capabilities?.console) return '该服务商不支持此操作'
  if (instanceData.value?.status !== 'running') return '仅运行中的实例可打开控制台'
  return ''
})

function powerDisabled(action: string): boolean {
  if (!instanceData.value?.capabilities?.power) return true
  const ps = instanceData.value?.power_status
  if (action === 'on') return ps === 'on'
  if (action === 'off') return ps === 'off'
  return ps !== 'on'
}

function powerTitle(action: string): string {
  if (!instanceData.value?.capabilities?.power) return '该服务商不支持此操作'
  const ps = instanceData.value?.power_status
  if (action === 'on') return ps === 'on' ? '实例已处于运行状态' : ''
  if (action === 'off') return ps === 'off' ? '实例已处于关机状态' : ''
  return ps !== 'on' ? '仅运行中的实例可重启' : ''
}

async function loadDetail() {
  loading.value = true
  try {
    instanceData.value = await getInstanceDetail(instanceId, false)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实例详情失败')
  } finally {
    loading.value = false
  }
}

async function loadOperations() {
  opLoading.value = true
  try {
    const data = await getInstanceOperations(instanceId, {
      page: opPagination.current,
      page_size: opPagination.pageSize,
    })
    operations.value = data.items
    opPagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载操作记录失败')
  } finally {
    opLoading.value = false
  }
}

async function loadRelated() {
  relatedLoading.value = true
  try {
    const data = await getInstanceRelated(instanceId)
    related.orders = data.orders
    related.renewals = data.renewals
    related.tickets = data.tickets
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载关联记录失败')
  } finally {
    relatedLoading.value = false
  }
}

function reloadAll() {
  loadDetail()
  loadOperations()
}

function handleOpPageChange(pageInfo: PageInfo) {
  opPagination.current = pageInfo.current
  opPagination.pageSize = pageInfo.pageSize
  loadOperations()
}

function goBack() {
  router.back()
}

function openOrder(row: RelatedOrder) {
  router.push(`/orders/detail/${row.id}`)
}

function openTicket(row: RelatedTicket) {
  router.push(`/tickets/detail/${row.id}`)
}

function handlePower(action: string) {
  if (action === 'on') {
    doPower(action)
    return
  }
  const name = instanceData.value?.name || instanceData.value?.instance_id || ''
  const label = powerActionLabel(action)
  const dialog = DialogPlugin.confirm({
    header: `${label}实例`,
    body: `确认对实例「${name}」执行${label}？${powerActionTips[action] || ''}`,
    confirmBtn: { content: `确认${label}`, theme: action === 'reboot' ? 'warning' : 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      dialog.destroy()
      await doPower(action)
    },
    onClose: () => dialog.destroy(),
  })
}

async function doPower(action: string) {
  try {
    await powerInstance(instanceId, action)
    MessagePlugin.success(`${powerActionLabel(action)}指令已发送`)
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `${powerActionLabel(action)}失败`)
  }
}

async function handleSync() {
  syncing.value = true
  try {
    await syncInstance(instanceId)
    MessagePlugin.success('实例状态已同步')
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '同步失败')
  } finally {
    syncing.value = false
  }
}

const remarkVisible = ref(false)
const remarkForm = reactive<{ remark: string }>({ remark: '' })

function openRemarkDialog() {
  remarkForm.remark = instanceData.value?.remark ?? ''
  remarkVisible.value = true
}

async function handleSaveRemark() {
  try {
    await updateInstanceRemark(instanceId, { remark: remarkForm.remark })
    MessagePlugin.success('备注已更新')
    remarkVisible.value = false
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新备注失败')
  }
}

const resizeVisible = ref(false)
const resizeForm = reactive<{ cpu: number; memory: number; disk: number; disk_type: string; reason: string }>({
  cpu: 0,
  memory: 0,
  disk: 0,
  disk_type: '',
  reason: '',
})

function openResizeDialog() {
  const data = instanceData.value
  if (!data) return
  resizeForm.cpu = data.cpu
  resizeForm.memory = data.memory
  resizeForm.disk = data.disk
  resizeForm.disk_type = data.disk_type
  resizeForm.reason = ''
  resizeVisible.value = true
}

async function handleResize() {
  try {
    await resizeInstance(instanceId, {
      cpu: resizeForm.cpu,
      memory: resizeForm.memory,
      disk: resizeForm.disk,
      disk_type: resizeForm.disk_type,
      reason: resizeForm.reason || undefined,
    })
    MessagePlugin.success('变配指令已提交')
    resizeVisible.value = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '变配失败')
  }
}

const destroyVisible = ref(false)
const destroyForm = reactive<{ confirm_mark: string; reason: string }>({ confirm_mark: '', reason: '' })

const destroyConfirmDisabled = computed(
  () => !instanceData.value?.instance_id || destroyForm.confirm_mark.trim() !== instanceData.value.instance_id,
)

function openDestroyDialog() {
  destroyForm.confirm_mark = ''
  destroyForm.reason = ''
  destroyVisible.value = true
}

async function handleDestroy() {
  try {
    await destroyInstance(instanceId, {
      confirm_mark: destroyForm.confirm_mark.trim(),
      reason: destroyForm.reason || undefined,
    })
    MessagePlugin.success('实例已销毁')
    destroyVisible.value = false
    router.push('/instances/list')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '销毁失败')
  }
}

const renewVisible = ref(false)
const renewForm = reactive<{ period_count: number; amount: number; remark: string }>({
  period_count: 1,
  amount: 0,
  remark: '',
})

function openRenewDialog() {
  renewForm.period_count = 1
  renewForm.amount = 0
  renewForm.remark = ''
  renewVisible.value = true
}

async function handleRenew() {
  try {
    await renewInstance(instanceId, {
      period_count: renewForm.period_count,
      amount: renewForm.amount || undefined,
      remark: renewForm.remark || undefined,
    })
    MessagePlugin.success('代续费成功，已延长实例到期时间')
    renewVisible.value = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '代续费失败')
  }
}

const vnc = reactive({ visible: false, name: '', url: '', password: '' })

async function handleVnc() {
  try {
    const data = await getInstanceVNC(instanceId)
    vnc.name = instanceData.value?.name || instanceData.value?.instance_id || ''
    vnc.url = data.url
    vnc.password = data.password
    vnc.visible = true
  } catch (error) {
    MessagePlugin.error((error as Error).message || '获取控制台地址失败')
  }
}

function openVnc() {
  window.open(vnc.url, '_blank')
}

function isHttp(url: string): boolean {
  return /^https?:/i.test(url)
}

onMounted(() => {
  loadDetail()
  loadOperations()
  loadRelated()
})
</script>

<style lang="css">
@import './shared.css';
</style>
