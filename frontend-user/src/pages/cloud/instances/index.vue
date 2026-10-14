<template>
  <div class="page-body console-module instance-page">
    <!-- 页头：与账单/订单页同一套 skeleton（page-header surface-card） -->
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip"><CloudIcon size="22" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">我的云主机</h2>
          <p class="page-header__desc">管理云主机电源、控制台与维护操作；到期前 {{ REMIND_DAYS }} 天会在此提示续费</p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-radio-group v-model="filter" variant="default-filled" size="small">
          <t-radio-button value="all">全部</t-radio-button>
          <t-radio-button value="running">运行中</t-radio-button>
          <t-radio-button value="expiring">待续费</t-radio-button>
          <t-radio-button value="stopped">已关机</t-radio-button>
        </t-radio-group>
        <t-button variant="outline" :loading="loading" @click="load(true)">
          <template #icon><RefreshIcon /></template>
          刷新状态
        </t-button>
        <t-button theme="primary" @click="$router.push('/shop')">
          <template #icon><AddIcon /></template>
          购买云主机
        </t-button>
      </div>
    </header>

    <!-- 汇总卡片：数字来自服务端 summary（与列表徽标同一套口径，前端不自己数） -->
    <section class="instance-stat-grid">
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><CloudIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ summary.total }}</span>
          <span class="stat-card__hint">云主机总数</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--green">
        <span class="stat-card__icon"><PlayCircleIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ summary.running }}</span>
          <span class="stat-card__hint">运行中</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--warning">
        <span class="stat-card__icon"><TimeIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ summary.expiring + summary.expired }}</span>
          <span class="stat-card__hint">待续费（临期 / 已过期）</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--default">
        <span class="stat-card__icon"><PoweroffIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ summary.stopped + summary.destroyed }}</span>
          <span class="stat-card__hint">已关机 / 已销毁</span>
        </div>
      </div>
    </section>

    <!-- 待续费提醒：有临期或已过期实例时给一条可点的横幅，不必等用户去翻列表 -->
    <t-alert
      v-if="summary.expiring + summary.expired > 0"
      theme="warning"
      class="instance-alert"
      :message="`有 ${summary.expiring} 台即将到期、${summary.expired} 台已过期，请及时续费以免服务中断`"
    >
      <template #operation>
        <t-button theme="primary" size="small" variant="outline" @click="$router.push('/cloud/renewals')">
          去续费
        </t-button>
      </template>
    </t-alert>

    <!-- 移动端：卡片列表（表格在窄屏横向滚动，操作列几乎不可用） -->
    <section v-if="isMobile" class="instance-cards">
      <t-loading :loading="loading" show-overlay>
        <article v-for="row in filtered" :key="row.id" class="instance-card surface-card">
          <div class="instance-card__head">
            <div class="instance-card__title" @click="goDetail(row)">
              <span class="instance-name">{{ row.name || row.instance_id }}</span>
              <span class="instance-mark">{{ row.instance_id }}</span>
            </div>
            <t-tag :theme="stateOf(row).theme" variant="light" size="small" shape="round">
              <template #icon>
                <component :is="stateIconComponent(stateOf(row).label)" />
              </template>
              {{ stateOf(row).label }}
            </t-tag>
          </div>
          <div class="instance-card__meta">
            <span>{{ row.cpu }} 核 / {{ formatMemory(row.memory) }} / {{ row.disk }} GB</span>
            <!-- NAT 机型没有独立 IP：列表里也标出机型，避免客户按"未分配 IP"理解成没配好 -->
            <span v-if="row.nat_access?.is_nat">{{ row.nat_access.forwards?.[0] || 'NAT 机型' }}</span>
            <span v-else>{{ row.public_ip || '未分配 IP' }}</span>
            <span>{{ formatTime(row.expire_at) }} 到期 · {{ daysLeftText(row.days_left, row.expire_state) }}</span>
          </div>
          <div class="instance-card__actions">
            <t-button size="small" theme="primary" variant="outline" @click="goDetail(row)">详情</t-button>
            <template v-if="row.pending">
              <t-button size="small" theme="primary" :loading="retryingId === row.id" @click="onRetry(row)">
                重新开通
              </t-button>
            </template>
            <template v-else>
              <t-button
                size="small"
                theme="success"
                variant="outline"
                :disabled="!canPower(row) || row.power_status === 'on'"
                @click="onPower(row, 'on')"
              >
                开机
              </t-button>
              <t-button
                size="small"
                theme="danger"
                variant="outline"
                :disabled="!canPower(row) || row.power_status === 'off'"
                @click="onPower(row, 'off')"
              >
                关机
              </t-button>
              <t-button size="small" variant="outline" :disabled="!canConsole(row)" @click="onVNC(row)">
                控制台
              </t-button>
            </template>
          </div>
        </article>
        <t-empty v-if="!loading && !filtered.length && loadError" class="instance-empty" :description="loadError">
          <template #action>
            <t-button theme="primary" :loading="loading" @click="load(false)">重试</t-button>
          </template>
        </t-empty>
        <t-empty v-else-if="!loading && !filtered.length" description="暂无云主机" />
      </t-loading>
    </section>

    <!-- 桌面端：表格 -->
    <section v-else class="table-card surface-card">
      <div class="table-card__head">
        <span class="card-title">云主机列表</span>
        <span class="table-card__meta">共 {{ filtered.length }} 台</span>
      </div>
      <t-table
        :data="filtered"
        :columns="columns"
        row-key="id"
        :loading="loading"
        :bordered="false"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="false"
      >
        <template #name="{ row }">
          <div class="instance-cell" @click="goDetail(row)">
            <span class="instance-name">{{ row.name || row.instance_id }}</span>
            <span class="instance-sub">{{ row.instance_id }}</span>
          </div>
        </template>
        <template #spec="{ row }">
          <div class="stack-cell">
            <span class="spec-main">{{ row.cpu }} 核 / {{ formatMemory(row.memory) }} / {{ row.disk }} GB</span>
            <span class="instance-sub">{{ row.os || '—' }}</span>
          </div>
        </template>
        <template #ip="{ row }">
          <div class="stack-cell">
            <span class="spec-main">{{ row.public_ip || '—' }}</span>
            <span class="instance-sub">{{ row.private_ip ? `内网 ${row.private_ip}` : '无内网 IP' }}</span>
          </div>
        </template>
        <template #status="{ row }">
          <t-tag :theme="stateOf(row).theme" variant="light" size="small" shape="round">
            <template #icon>
              <component :is="stateIconComponent(stateOf(row).label)" />
            </template>
            {{ stateOf(row).label }}
          </t-tag>
          <div v-if="stateOf(row).detail" class="state-detail">{{ stateOf(row).detail }}</div>
        </template>
        <template #expire_at="{ row }">
          <div class="stack-cell">
            <span class="spec-main">{{ formatTime(row.expire_at) }}</span>
            <span class="instance-sub" :class="`days--${daysLeftTheme(row.days_left, row.expire_state)}`">
              {{ daysLeftText(row.days_left, row.expire_state) }}
            </span>
          </div>
        </template>
        <template #op="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="goDetail(row)">详情</t-link>
            <template v-if="row.pending">
              <t-link theme="primary" hover="color" @click="onRetry(row)">重新开通</t-link>
            </template>
            <template v-else>
              <t-link
                theme="success"
                hover="color"
                :disabled="!canPower(row) || row.power_status === 'on'"
                @click="onPower(row, 'on')"
              >
                开机
              </t-link>
              <t-link
                theme="danger"
                hover="color"
                :disabled="!canPower(row) || row.power_status === 'off'"
                @click="onPower(row, 'off')"
              >
                关机
              </t-link>
              <t-link
                theme="warning"
                hover="color"
                :disabled="!canPower(row) || row.power_status !== 'on'"
                @click="onPower(row, 'reboot')"
              >
                重启
              </t-link>
              <t-link theme="primary" hover="color" :disabled="!canConsole(row)" @click="onVNC(row)">控制台</t-link>
            </template>
          </div>
        </template>
      </t-table>

      <t-empty v-if="!loading && !instances.length && loadError" class="instance-empty" :description="loadError">
        <template #action>
          <t-button theme="primary" :loading="loading" @click="load(false)">重试</t-button>
        </template>
      </t-empty>
      <t-empty v-else-if="!loading && !instances.length" class="instance-empty" description="暂无云主机，去购买一台吧">
        <template #action>
          <t-button theme="primary" @click="$router.push('/shop')">去购买</t-button>
        </template>
      </t-empty>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { TTable } from '@/plugins/tdesign-heavy'
import type { PrimaryTableCol } from 'tdesign-vue-next'
import {
  AddIcon,
  CloseCircleIcon,
  CloudIcon,
  DeleteIcon,
  ErrorCircleIcon,
  InfoCircleIcon,
  PauseCircleIcon,
  PlayCircleIcon,
  PoweroffIcon,
  RefreshIcon,
  TimeIcon,
} from 'tdesign-icons-vue-next'

import {
  listInstances,
  powerInstance,
  retryInstanceProvision,
  vncInstance,
  type InstanceInfo,
  type InstanceListSummary,
  type PowerAction,
} from '@/api/cloud'
import { useIsMobile } from '@/composables/useIsMobile'
import { combineInstanceState, daysLeftText, daysLeftTheme, formatMemory, formatTime, stateIcon } from './constants'

defineOptions({ name: 'InstanceList' })

/** 续费提醒天数（与服务端 expireStateOf 的判定阈值同口径）。 */
const REMIND_DAYS = 7

/** 数据到达前的空汇总：避免模板里 `summary.total` 在 undefined 上取属性。 */
const EMPTY_SUMMARY: InstanceListSummary = {
  total: 0,
  running: 0,
  stopped: 0,
  creating: 0,
  expiring: 0,
  expired: 0,
  destroyed: 0,
}

const router = useRouter()
const { isMobile } = useIsMobile()
const instances = ref<InstanceInfo[]>([])
const summary = ref<InstanceListSummary>({ ...EMPTY_SUMMARY })
const loading = ref(false)
/** 加载失败原因（空串=正常）。模板据此显示错误态 + 重试，而不是留一片空白。 */
const loadError = ref('')
const filter = ref<'all' | 'running' | 'expiring' | 'stopped'>('all')
const retryingId = ref(0)

const columns: PrimaryTableCol<InstanceInfo>[] = [
  { colKey: 'name', title: '主机', minWidth: 190 },
  { colKey: 'spec', title: '配置', minWidth: 160 },
  { colKey: 'ip', title: 'IP 地址', minWidth: 150 },
  { colKey: 'status', title: '状态', width: 150 },
  { colKey: 'expire_at', title: '到期时间', width: 170 },
  { colKey: 'op', title: '操作', width: 280, fixed: 'right' },
]

// 前端筛选只做视图切分（服务端返回的是当前用户全量实例）；汇总数字仍用服务端 summary，
// 两者的差别在于「筛选后看到的行数」与「账号下总量」——这是刻意分开的，不是不一致。
const filtered = computed(() => {
  if (filter.value === 'all') return instances.value
  if (filter.value === 'running') return instances.value.filter((r) => r.status === 'running' && !r.pending)
  if (filter.value === 'expiring') {
    return instances.value.filter((r) => r.expire_state === 'expiring' || r.expire_state === 'expired')
  }
  return instances.value.filter((r) => r.status === 'stopped' && r.lifecycle_stage !== 'destroyed')
})

function stateOf(row: InstanceInfo): { label: string; theme: string; detail: string } {
  // 徽标结论统一交给 combineInstanceState：服务状态与到期阶段单独看会互相矛盾
  // （宽限期内 status=running 但已过期），这里不再自行拼接。
  return combineInstanceState({
    status: row.status,
    lifecycle_stage: row.lifecycle_stage,
    expire_state: row.expire_state,
    pending: row.pending,
  })
}

/**
 * 状态结论 → 图标组件。
 *
 * 图标让状态可扫视：一列纯文字 tag 想找"哪台关机了"得逐行读，
 * 有图标后按形状/颜色一扫就能定位。运维只有几台时无所谓，
 * 十几台以上差别很明显。
 */
const STATE_ICONS: Record<string, Component> = {
  'play-circle': PlayCircleIcon,
  'pause-circle': PauseCircleIcon,
  time: TimeIcon,
  'error-circle': ErrorCircleIcon,
  delete: DeleteIcon,
  'close-circle': CloseCircleIcon,
  'info-circle': InfoCircleIcon,
}

function stateIconComponent(label: string): Component {
  return STATE_ICONS[stateIcon(label)] || InfoCircleIcon
}

/** 能力位缺省时按可操作处理：旧数据没有 capabilities 字段，不该因此禁用全部按钮。 */
function canPower(row: InstanceInfo): boolean {
  if (row.pending) return false
  return row.capabilities ? row.capabilities.power : true
}
function canConsole(row: InstanceInfo): boolean {
  if (row.pending) return false
  const supported = row.capabilities ? row.capabilities.console : true
  return supported && row.status === 'running'
}

async function load(live = false) {
  loading.value = true
  loadError.value = ''
  try {
    const res = await listInstances({ live })
    instances.value = res.data.items || []
    summary.value = res.data.summary || { ...EMPTY_SUMMARY, total: instances.value.length }
  } catch (error) {
    // 必须自己兜住失败：请求拦截器只弹提示并 reject，这里不 catch 会让
    // 整个 setup 抛未处理异常，首屏直接空白（用户实测的"数据都为空"）。
    // 保留已加载的数据展示，同时置错误标志让模板给出重试入口。
    loadError.value = (error as Error)?.message || '加载失败，请稍后重试'
    if (!instances.value.length) summary.value = { ...EMPTY_SUMMARY }
  } finally {
    loading.value = false
  }
}

async function onPower(row: InstanceInfo, action: PowerAction) {
  const label = { on: '开机', off: '关机', reboot: '重启', hard_off: '强制关机', hard_reboot: '强制重启' }[action]
  try {
    await powerInstance(row.id, action)
    MessagePlugin.success(`${label}指令已发送`)
    // 本地乐观更新：按钮的可用状态（开机/关机互斥）依赖 status，
    // 不先改本地的话，这行按钮要等回源（数秒）才切过来，看着像"点了没用"。
    applyOptimisticPower(row, action)
    // 后台回源校正；不 await，避免整表 loading 遮罩把刚更新的行又盖住。
    void load(true)
  } catch {
    /* 错误已由请求拦截器提示 */
  }
}

/**
 * 电源动作后的本地乐观状态。重启类不改（前后同为运行中，无中间态可写）。
 * 服务端已把预期状态落库，回源结果会覆盖这里。
 */
function applyOptimisticPower(row: InstanceInfo, action: PowerAction) {
  const target: Record<string, { status: string; power: string }> = {
    on: { status: 'running', power: 'on' },
    off: { status: 'stopped', power: 'off' },
    hard_off: { status: 'stopped', power: 'off' },
  }
  const next = target[action]
  if (!next) return
  row.status = next.status
  row.power_status = next.power
}

// 待开通实例：重新提交开通（复用既有开通单，不产生新订单/不重复扣费）。
async function onRetry(row: InstanceInfo) {
  retryingId.value = row.id
  try {
    await retryInstanceProvision(row.id)
    MessagePlugin.success('已重新提交开通，请稍后刷新查看')
    load(true)
  } catch {
    /* 错误已由请求拦截器提示 */
  } finally {
    retryingId.value = 0
  }
}

/**
 * 打开远程控制台：取到地址后直接开新浏览器标签页。
 *
 * 与详情页同一实现：控制台页面自带全屏/键鼠捕获，内嵌 iframe 在小窗口里
 * 操作体验很差，新标签页是这类控制台的标准用法。
 */
async function onVNC(row: InstanceInfo) {
  const tab = window.open('', '_blank', 'noopener,noreferrer')
  if (!tab) {
    MessagePlugin.warning('浏览器拦截了新窗口，请允许本站弹出窗口后重试')
    return
  }
  try {
    const res = await vncInstance(row.id)
    const url = res.data?.url || ''
    if (!url) {
      tab.close()
      MessagePlugin.warning('控制台地址暂不可用，请稍后重试')
      return
    }
    tab.location.href = url
  } catch (error) {
    tab.close()
    throw error
  }
}

function goDetail(row: InstanceInfo) {
  router.push(`/cloud/instances/${row.id}`)
}

/**
 * 首屏先出本地数据，实时状态由用户在需要时点「刷新」触发。
 *
 * 原来首屏就 load(true)：进页面即对账号下所有机器回源上游。
 * 机器多的账号（十几台）要等十几秒，网关先超时而这里又没有 catch，
 * 结果整页报错、列表空白 —— 用户实测反馈的现象。
 * 本地状态（instances.status）由开通/电源操作与同步任务维护，足够支撑首屏展示；
 * 回源只用于"想确认此刻真实状态"的场景，交给刷新按钮。
 */
onMounted(() => load(false))
</script>

<style scoped>
.instance-page {
  gap: var(--space-lg);
}

/* ---------- 汇总卡片 ---------- */
.instance-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--space-md);
}

.instance-alert {
  border-radius: var(--hs-radius-lg);
}

/* ---------- 表格卡片 ---------- */
.table-card {
  padding: var(--space-lg) var(--space-xl) var(--space-md);
}

.table-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-md);
}

.instance-cell {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.instance-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
  transition: color var(--hs-duration-fast) var(--hs-ease-out);
}

.instance-cell:hover .instance-name {
  color: var(--color-primary);
}

.instance-sub {
  font-size: 12px;
  color: var(--color-muted-foreground);
  word-break: break-all;
}

.instance-mark {
  font-size: 12px;
  color: var(--color-muted-foreground);
  font-family: var(--hs-font-mono);
}

.stack-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.spec-main {
  font-size: 13px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.state-detail {
  margin-top: 4px;
  font-size: 11px;
  color: var(--color-muted-foreground);
  line-height: 1.4;
}

.days--danger {
  color: var(--color-error, #d54941);
  font-weight: 600;
}
.days--warning {
  color: var(--color-warning, #e37318);
  font-weight: 600;
}

.action-cell {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.instance-empty {
  padding: 24px 0;
}

/* ---------- 移动端卡片 ---------- */
.instance-cards {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.instance-card {
  padding: var(--space-md) var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.instance-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
}

.instance-card__title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.instance-card__meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.instance-card__actions {
  display: flex;
  gap: var(--space-sm);
  flex-wrap: wrap;
}

@media (max-width: 1024px) {
  .instance-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .instance-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .page-header__actions {
    width: 100%;
  }
}
</style>
