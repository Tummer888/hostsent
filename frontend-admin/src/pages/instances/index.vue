<template>
  <div class="page-body instances-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ServerIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">实例运维台</h2>
          <p class="page-header__desc">跨用户查看云主机实例状态，执行电源、控制台与同步运维操作</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="stat-grid">
      <div
        v-for="stat in metricCards"
        :key="stat.key"
        class="stat-card"
        :class="`stat-card--${stat.variant}`"
      >
        <span class="stat-card__icon">
          <component :is="stat.icon" size="22" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ stat.value }}</span>
          <span class="stat-card__label">{{ stat.label }}</span>
        </div>
      </div>
    </section>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="实例标识 / 名称" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">用户账号</span>
          <t-input v-model="filters.user_keyword" placeholder="用户名 / 邮箱 / 手机号" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">服务商 ID</span>
          <t-input v-model="filters.provider_id" placeholder="按服务商 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="instanceStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">来源</span>
          <t-select v-model="filters.source_mode" clearable placeholder="全部来源" :options="sourceModeOptions" />
        </div>
        <div class="field">
          <span class="field__label">到期状态</span>
          <t-select v-model="filters.expire_state" placeholder="全部" :options="expireStateOptions" />
        </div>
        <div class="field">
          <span class="field__label">临期天数</span>
          <t-input-number
            v-model="filters.expire_within_days"
            :min="1"
            :max="365"
            :disabled="filters.expire_state !== 'expiring'"
            theme="column"
            placeholder="默认 7 天"
          />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">实例列表</h3>
        <span class="table-card__meta">共 {{ total }} 台实例</span>
      </div>
      <t-table
        row-key="id"
        :data="instanceList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div>
            <span class="cell-strong">{{ row.name || '—' }}</span>
            <div class="cell-sub">{{ row.instance_id || '—' }}</div>
          </div>
        </template>

        <template #user="{ row }">
          <div>
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <div class="cell-sub">ID {{ row.user_id }}</div>
          </div>
        </template>

        <template #spec="{ row }">
          <div>
            <span class="cell-strong">{{ row.cpu }} 核 / {{ row.memory }} MB / {{ row.disk }} GB</span>
            <div class="cell-sub">{{ row.os || '—' }}</div>
          </div>
        </template>

        <template #public_ip="{ row }">
          <span class="cell-muted">{{ row.public_ip || '—' }}</span>
        </template>

        <template #provider_name="{ row }">
          <div class="provider-cell">
            <t-tag :theme="row.source_mode === 'upstream' ? 'warning' : 'primary'" variant="light" size="small" shape="round">
              {{ row.source_mode === 'upstream' ? '上游' : '自营' }}
            </t-tag>
            <span class="provider-name">{{ row.provider_name || '—' }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <div>
            <t-tag :theme="instanceStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ instanceStatusLabel(row.status) }}
            </t-tag>
            <div class="cell-sub" :class="expireClass(row)">{{ expireText(row) }}</div>
          </div>
        </template>

        <template #actor_name="{ row }">
          <span class="cell-muted">{{ row.actor_name || '主账号' }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail' },
                { content: '开机', value: 'power-on', hidden: () => !has('instance:action'), disabled: () => row.power_status === 'on' || poweringId === row.id },
                { content: '关机', value: 'power-off', hidden: () => !has('instance:action'), disabled: () => row.power_status === 'off' || poweringId === row.id },
                { content: '重启', value: 'power-reboot', hidden: () => !has('instance:action'), disabled: () => row.power_status !== 'on' || poweringId === row.id },
                { content: '控制台', value: 'vnc', hidden: () => !has('instance:console'), disabled: () => row.status !== 'running' },
                { content: '同步刷新', value: 'sync', hidden: () => !has('instance:action'), disabled: () => syncingId === row.id },
                { content: '备注', value: 'remark', hidden: () => !has('instance:action') },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link
                v-permission="'instance:action'"
                theme="primary"
                hover="color"
                :disabled="row.power_status === 'on' || poweringId === row.id"
                @click="handlePower(row, 'on')"
              >
                开机
              </t-link>
              <t-link
                v-permission="'instance:action'"
                theme="warning"
                hover="color"
                :disabled="row.power_status === 'off' || poweringId === row.id"
                @click="handlePower(row, 'off')"
              >
                关机
              </t-link>
              <t-link
                v-permission="'instance:action'"
                theme="primary"
                hover="color"
                :disabled="row.power_status !== 'on' || poweringId === row.id"
                @click="handlePower(row, 'reboot')"
              >
                重启
              </t-link>
              <t-link
                v-permission="'instance:console'"
                theme="primary"
                hover="color"
                :disabled="row.status !== 'running'"
                @click="handleVnc(row)"
              >
                控制台
              </t-link>
              <!-- t-dropdown 根节点是 fragment，指令无法作用于组件自身，故用 span 包裹 -->
              <span v-permission="'instance:action'">
                <t-dropdown trigger="click">
                  <t-link theme="primary" hover="color">更多</t-link>
                  <t-dropdown-menu>
                    <t-dropdown-item :disabled="syncingId === row.id" @click="handleSync(row)">
                      {{ syncingId === row.id ? '同步中…' : '同步刷新' }}
                    </t-dropdown-item>
                    <t-dropdown-item @click="openRemarkDialog(row)">备注</t-dropdown-item>
                  </t-dropdown-menu>
                </t-dropdown>
              </span>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无实例数据" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
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
      v-model:visible="vnc.visible"
      :header="`远程控制台 · ${vnc.name}`"
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
import { useRouter } from 'vue-router'

import {
  AppIcon,
  CheckCircleIcon,
  ErrorCircleIcon,
  RefreshIcon,
  SearchIcon,
  ServerIcon,
  StopIcon,
  TimeIcon,
} from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getInstanceList,
  getInstanceStats,
  getInstanceVNC,
  powerInstance,
  syncInstance,
  updateInstanceRemark,
} from '@/api/instance'
import {
  expireStateOptions,
  formatTime,
  instanceStatusLabel,
  instanceStatusOptions,
  instanceStatusTheme,
  powerActionLabel,
  powerActionTips,
} from '@/pages/instances/constants'
import { sourceModeOptions } from '@/pages/product/constants'
import type { InstanceItem, InstanceStatsResponse } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { usePermission } from '@/composables/usePermission'

defineOptions({ name: 'InstanceList' })

const router = useRouter()

const instanceList = ref<InstanceItem[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const { has } = usePermission()
const total = ref(0)
const syncingId = ref(0)
const poweringId = ref(0)

const stats = reactive<InstanceStatsResponse>({
  total: 0,
  running: 0,
  stopped: 0,
  creating: 0,
  error: 0,
  expiring: 0,
  expired: 0,
})

const metricCards = computed(() => [
  { key: 'total', label: '实例总数', value: stats.total, variant: 'info', icon: ServerIcon },
  { key: 'running', label: '运行中', value: stats.running, variant: 'success', icon: CheckCircleIcon },
  { key: 'stopped', label: '已关机', value: stats.stopped, variant: 'default', icon: StopIcon },
  { key: 'expiring', label: '7 天内到期', value: stats.expiring, variant: 'warning', icon: TimeIcon },
  { key: 'expired', label: '已过期', value: stats.expired, variant: 'danger', icon: ErrorCircleIcon },
])

const filters = reactive<{
  keyword: string | undefined
  user_keyword: string | undefined
  user_id: string | undefined
  provider_id: string | undefined
  status: string | undefined
  source_mode: string | undefined
  expire_state: string
  expire_within_days: number
}>({
  keyword: undefined,
  user_keyword: undefined,
  user_id: undefined,
  provider_id: undefined,
  status: undefined,
  source_mode: undefined,
  expire_state: 'all',
  expire_within_days: 7,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护（见 loadUsers/loadData）
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns: PrimaryTableCol<InstanceItem>[] = [
  { colKey: 'name', title: '实例', minWidth: 200 },
  { colKey: 'user', title: '归属用户', minWidth: 170 },
  { colKey: 'spec', title: '规格', minWidth: 190 },
  { colKey: 'public_ip', title: '公网 IP', width: 140 },
  { colKey: 'provider_name', title: '服务商', minWidth: 130 },
  { colKey: 'status', title: '状态 / 到期', width: 170 },
  { colKey: 'actor_name', title: '操作人', width: 100 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 300,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

function expireText(row: InstanceItem): string {
  if (!row.expire_at || row.expire_state === 'none') return '未设到期'
  const base = formatTime(row.expire_at)
  if (row.expire_state === 'expired') return `${base}（已过期）`
  if (row.expire_state === 'expiring') return `${base}（剩余 ${row.days_left} 天）`
  return base
}

function expireClass(row: InstanceItem): string {
  if (row.expire_state === 'expired') return 'expire-danger'
  if (row.expire_state === 'expiring') return 'expire-warning'
  return ''
}

async function loadInstances() {
  loading.value = true
  try {
    const data = await getInstanceList({
      keyword: filters.keyword,
      user_keyword: filters.user_keyword,
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      provider_id: filters.provider_id ? Number(filters.provider_id) : undefined,
      status: filters.status,
      source_mode: filters.source_mode,
      expire_state: filters.expire_state,
      expire_within_days: filters.expire_state === 'expiring' ? filters.expire_within_days : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    instanceList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实例列表失败')
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const data = await getInstanceStats()
    Object.assign(stats, data)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实例统计失败')
  }
}

function loadAll() {
  loadInstances()
  loadStats()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadInstances()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}


function handleSearch() {
  pagination.current = 1
  loadInstances()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.user_keyword = undefined
  filters.user_id = undefined
  filters.provider_id = undefined
  filters.status = undefined
  filters.source_mode = undefined
  filters.expire_state = 'all'
  filters.expire_within_days = 7
  pagination.current = 1
  loadAll()
}

function openDetail(row: InstanceItem) {
  router.push(`/instances/detail/${row.id}`)
}

function handlePower(row: InstanceItem, action: string) {
  if (action === 'on') {
    doPower(row, action)
    return
  }
  const label = powerActionLabel(action)
  const dialog = DialogPlugin.confirm({
    header: `${label}实例`,
    body: `确认对实例「${row.name || row.instance_id}」执行${label}？${powerActionTips[action] || ''}`,
    confirmBtn: { content: `确认${label}`, theme: action === 'reboot' ? 'warning' : 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      dialog.destroy()
      await doPower(row, action)
    },
    onClose: () => dialog.destroy(),
  })
}

async function doPower(row: InstanceItem, action: string) {
  poweringId.value = row.id
  try {
    await powerInstance(row.id, action)
    MessagePlugin.success(`${powerActionLabel(action)}指令已发送`)
    loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `${powerActionLabel(action)}失败`)
  } finally {
    poweringId.value = 0
  }
}

async function handleSync(row: InstanceItem) {
  syncingId.value = row.id
  try {
    await syncInstance(row.id)
    MessagePlugin.success('实例状态已同步')
    loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '同步失败')
  } finally {
    syncingId.value = 0
  }
}

const remarkVisible = ref(false)
const remarkForm = reactive<{ id: number; remark: string }>({ id: 0, remark: '' })

function openRemarkDialog(row: InstanceItem) {
  remarkForm.id = row.id
  remarkForm.remark = row.remark
  remarkVisible.value = true
}

async function handleSaveRemark() {
  try {
    await updateInstanceRemark(remarkForm.id, { remark: remarkForm.remark })
    MessagePlugin.success('备注已更新')
    remarkVisible.value = false
    loadInstances()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新备注失败')
  }
}

const vnc = reactive({ visible: false, name: '', url: '', password: '' })

async function handleVnc(row: InstanceItem) {
  try {
    const data = await getInstanceVNC(row.id)
    vnc.name = row.name || row.instance_id
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

onMounted(loadAll)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: InstanceItem) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'power-on':
      void handlePower(row, 'on')
      break
    case 'power-off':
      void handlePower(row, 'off')
      break
    case 'power-reboot':
      void handlePower(row, 'reboot')
      break
    case 'vnc':
      void handleVnc(row)
      break
    case 'sync':
      void handleSync(row)
      break
    case 'remark':
      openRemarkDialog(row)
      break
  }
}
</script>

<style lang="css">
@import './shared.css';
</style>
