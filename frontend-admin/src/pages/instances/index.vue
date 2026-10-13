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

    <FilterCard>
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
        <span class="field__label">链路</span>
        <t-select v-model="filters.source_mode" clearable placeholder="全部链路" :options="sourceModeOptions" />
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
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">实例列表</h3>
        <t-space size="small" align="center">
          <!-- 批量运维（doc61 P1）：勾选后出现动作条，逐台结果在弹窗里回显 -->
          <template v-if="selectedIds.length">
            <span class="table-card__meta">已选 {{ selectedIds.length }} 台</span>
            <t-button
              v-for="act in batchActions"
              :key="act.value"
              size="small"
              :theme="act.theme"
              variant="outline"
              :loading="batchRunning"
              @click="handleBatch(act.value, act.label, act.needConfirm)"
            >
              {{ act.label }}
            </t-button>
            <!-- 批量销毁：不可逆且会真删上游机器，独立权限码 + 逐台确认，不与电源动作同组 -->
            <t-button
              v-if="canDestroy"
              size="small"
              theme="danger"
              variant="outline"
              :loading="batchRunning"
              @click="openBatchDestroyDialog"
            >
              批量销毁
            </t-button>
            <t-button size="small" variant="text" @click="clearSelection">清空</t-button>
          </template>
          <span v-else class="table-card__meta">共 {{ total }} 台实例</span>
        </t-space>
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
        :selected-row-keys="selectedIds"
        :pagination="isMobile ? undefined : pagination"
        @select-change="handleSelectChange"
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
            <span class="cell-strong">{{ row.cpu }} 核 / {{ formatMemory(row.memory) }} / {{ row.disk }} GB</span>
            <!-- 系统带图标 + 清洗后的版本名：列表里扫一眼就能分辨客户是哪套环境 -->
            <div class="cell-sub spec-os">
              <OsIcon :os="row.os" />
              <span>{{ osDisplayName(row.os) }}</span>
            </div>
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
            <t-tag :theme="combineInstanceState(row).theme" variant="light" size="small" shape="round">
              {{ combineInstanceState(row).label }}
            </t-tag>
            <div class="cell-sub" :class="expireClass(row)">{{ expireText(row) }}</div>
            <div v-if="combineInstanceState(row).detail" class="cell-sub">{{ combineInstanceState(row).detail }}</div>
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
                { content: '重新开通', value: 'provision-retry', hidden: () => !row.pending, disabled: () => retryingId === row.id },
                { content: '同步刷新', value: 'sync', hidden: () => !has('instance:action') || !!row.pending, disabled: () => syncingId === row.id },
                { content: row.lifecycle_stage === 'suspended' ? '恢复服务' : '暂停服务', value: 'toggle-suspend', hidden: () => !has('instance:action') },
                { content: '备注', value: 'remark', hidden: () => !has('instance:action') },
                { content: '销毁实例', value: 'destroy', theme: 'error', hidden: () => !canDestroy || !instanceDestroyable(row) },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <!-- 待开通实例：只给「重新开通」（重投订单履约，不产生新订单/新账单），
                   电源/控制台/同步等上游动作没有对象，隐藏而不是点了报错。 -->
              <template v-if="row.pending">
                <t-link
                  v-permission="'instance:action'"
                  theme="primary"
                  hover="color"
                  :disabled="retryingId === row.id"
                  @click="handleRetryProvision(row)"
                >
                  {{ retryingId === row.id ? '提交中…' : '重新开通' }}
                </t-link>
                <t-link v-permission="'instance:action'" theme="primary" hover="color" @click="openRemarkDialog(row)">备注</t-link>
              </template>
              <template v-else>
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
                    <!-- 硬电源（断电级）：软电源在行内已有，这里只补危险动作，同样走二次确认 -->
                    <t-dropdown-item
                      :disabled="poweringId === row.id"
                      @click="handlePower(row, 'hard_off')"
                    >
                      强制关机
                    </t-dropdown-item>
                    <t-dropdown-item
                      :disabled="poweringId === row.id"
                      @click="handlePower(row, 'hard_reboot')"
                    >
                      强制重启
                    </t-dropdown-item>
                    <!-- 暂停/恢复（T5.5）：缺能力时置灰，避免点出一个必定失败的动作 -->
                    <t-dropdown-item
                      v-if="row.lifecycle_stage === 'suspended'"
                      :disabled="actioningId === row.id"
                      @click="handleUnsuspend(row)"
                    >
                      恢复服务
                    </t-dropdown-item>
                    <t-dropdown-item
                      v-else
                      :disabled="actioningId === row.id"
                      @click="handleSuspend(row)"
                    >
                      暂停服务
                    </t-dropdown-item>
                    <t-dropdown-item @click="openRemarkDialog(row)">备注</t-dropdown-item>
                    <!--
                      销毁：行内补齐。此前列表没有任何删除入口（用户实测反馈
                      "没办法删除实例"），只能进详情页销毁；而"清掉一批测试机"
                      恰恰是列表页的高频动作。走与详情页同一套二次确认弹窗
                      （查能力位 + 输入实例标识确认 + 权限码 instance:destroy）。
                    -->
                    <t-dropdown-item
                      v-if="canDestroy && instanceDestroyable(row)"
                      :disabled="actioningId === row.id"
                      @click="openDestroyDialog(row)"
                    >
                      <span class="danger-text">销毁实例</span>
                    </t-dropdown-item>
                  </t-dropdown-menu>
                </t-dropdown>
              </span>
              </template>
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

    <!-- 单台销毁：与详情页同一套二次确认口径（输入实例标识），提交后逐台回显结果 -->
    <t-dialog
      v-model:visible="destroyForm.visible"
      header="销毁实例"
      width="480px"
      :confirm-btn="{ content: '确认销毁', theme: 'danger' }"
      :cancel-btn="{ content: '再想想' }"
      :confirm-disabled="destroyForm.mark.trim() !== destroyForm.instanceMark"
      :confirm-loading="destroyForm.loading"
      destroy-on-close
      @confirm="handleDestroy"
      @close="destroyForm.visible = false"
    >
      <t-alert
        theme="error"
        class="dlg-alert"
        :message="`销毁后不可恢复：将从平台删除 ${destroyForm.name || '该实例'}，数据全部丢失。`"
      />
      <p class="dlg-confirm-tip">
        请输入实例标识 <code>{{ destroyForm.instanceMark || '—' }}</code> 以确认：
      </p>
      <t-input v-model="destroyForm.mark" :placeholder="destroyForm.instanceMark || '实例标识'" />
      <t-form label-align="top" class="destroy-reason">
        <t-form-item label="原因（可选，写入操作流水）">
          <t-input v-model="destroyForm.reason" placeholder="如：测试机清理" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 批量销毁：逐台列出并要求确认文本，逐台结果回显 -->
    <t-dialog
      v-model:visible="batchDestroy.visible"
      header="批量销毁实例"
      width="680px"
      :footer="batchDestroy.done"
      destroy-on-close
      @close="batchDestroy.visible = false"
    >
      <template v-if="!batchDestroy.done">
        <t-alert
          theme="error"
          class="dlg-alert"
          :message="`将从平台删除选中的 ${batchDestroy.targets.length} 台实例，数据全部丢失且不可恢复。勾选全选不等于一键删除：请逐台确认。`"
        />
        <t-table
          row-key="id"
          :data="batchDestroy.targets"
          :columns="batchDestroyColumns"
          size="small"
          :pagination="undefined"
          table-layout="fixed"
          cell-empty-content="—"
        >
          <template #instance="{ row }">
            <div>
              <span class="cell-strong">{{ row.name || '—' }}</span>
              <div class="cell-sub">{{ row.instance_mark || '—' }}</div>
            </div>
          </template>
          <template #instance_mark="{ row }">
            <code>{{ row.instance_mark }}</code>
          </template>
          <template #confirm="{ row }">
            <t-input v-model="row.confirm" size="small" :placeholder="row.instance_mark" />
          </template>
        </t-table>
        <div class="destroy-actions">
          <t-button variant="outline" @click="batchDestroy.visible = false">再想想</t-button>
          <t-button
            theme="danger"
            :disabled="!batchDestroyReady"
            :loading="batchDestroy.running"
            @click="handleBatchDestroy"
          >
            确认销毁 {{ batchDestroy.targets.length }} 台
          </t-button>
        </div>
      </template>
      <div v-else class="batch-result">
        <p class="batch-result__summary">
          成功 {{ batchDestroy.result?.succeeded ?? 0 }} 台 · 跳过 {{ batchDestroy.result?.skipped ?? 0 }} 台 · 失败
          {{ batchDestroy.result?.failed ?? 0 }} 台
        </p>
        <t-table
          row-key="id"
          :data="batchDestroy.result?.items ?? []"
          :columns="batchResultColumns"
          size="small"
          :pagination="undefined"
          table-layout="fixed"
          cell-empty-content="—"
        >
          <template #instance="{ row }">
            <div>
              <span class="cell-strong">{{ row.name || '—' }}</span>
              <div class="cell-sub">{{ row.instance_mark || '—' }}</div>
            </div>
          </template>
          <template #status="{ row }">
            <t-tag :theme="batchStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ batchStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #message="{ row }">
            <span :class="{ 'op-error': row.status === 'failed' }" class="cell-muted">{{ row.message || '—' }}</span>
          </template>
        </t-table>
        <div class="destroy-actions">
          <t-button theme="primary" @click="batchDestroy.visible = false">关闭</t-button>
        </div>
      </div>
    </t-dialog>

    <t-dialog
      v-model:visible="batchResult.visible"
      :header="`${batchResult.label}结果`"
      width="640px"
      :footer="false"
      @close="batchResult.visible = false"
    >
      <div v-if="batchResult.data" class="batch-result">
        <p class="batch-result__summary">
          成功 {{ batchResult.data.succeeded }} 台 · 跳过 {{ batchResult.data.skipped }} 台 · 失败
          {{ batchResult.data.failed }} 台
        </p>
        <t-table
          row-key="id"
          :data="batchResult.data.items"
          :columns="batchResultColumns"
          size="small"
          :pagination="undefined"
          table-layout="fixed"
          cell-empty-content="—"
        >
          <template #instance="{ row }">
            <div>
              <span class="cell-strong">{{ row.name || '—' }}</span>
              <div class="cell-sub">{{ row.instance_mark || '—' }}</div>
            </div>
          </template>
          <template #status="{ row }">
            <t-tag :theme="batchStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ batchStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #message="{ row }">
            <span :class="{ 'op-error': row.status === 'failed' }" class="cell-muted">{{ row.message || '—' }}</span>
          </template>
        </t-table>
      </div>
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
import FilterCard from '@/components/filter-card/index.vue'
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
  batchDestroyInstances,
  batchInstanceAction,
  destroyInstance,
  powerInstance,
  retryInstanceProvision,
  suspendInstance,
  syncInstance,
  unsuspendInstance,
  updateInstanceRemark,
} from '@/api/instance'
import {
  combineInstanceState,
  expireStateOptions,
  formatMemory,
  formatTime,
  instanceStatusLabel,
  instanceStatusOptions,
  instanceStatusTheme,
  powerActionLabel,
  powerActionTips,
} from '@/pages/instances/constants'
import { sourceModeOptions } from '@/pages/product/constants'
import type {
  InstanceBatchActionItem,
  InstanceBatchActionResponse,
  InstanceBatchDestroyResponse,
  InstanceItem,
  InstanceStatsResponse,
} from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import OsIcon from '@/components/OsIcon.vue'
import { osDisplayName } from './os-icons'
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
const actioningId = ref(0)
const retryingId = ref(0)

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
// 表格列：批量多选列只在桌面端且具备操作权限时出现（移动端多选按钮会挤压操作列）。
const columns = computed<PrimaryTableCol<InstanceItem>[]>(() => {
  const base: PrimaryTableCol<InstanceItem>[] = [
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
  if (!isMobile.value && has('instance:action')) {
    base.unshift({ colKey: 'row-select', type: 'multiple', width: 46, fixed: 'left' as const })
  }
  return base
})

// 批量结果弹窗的列（只读回显，不提供操作）。
const batchResultColumns: PrimaryTableCol<InstanceBatchActionItem>[] = [
  { colKey: 'instance', title: '实例', minWidth: 170 },
  { colKey: 'status', title: '结果', width: 90 },
  { colKey: 'message', title: '说明', minWidth: 220 },
]

function batchStatusLabel(status: string): string {
  if (status === 'success') return '成功'
  if (status === 'skipped') return '跳过'
  return '失败'
}

function batchStatusTheme(status: string): string {
  if (status === 'success') return 'success'
  if (status === 'skipped') return 'default'
  return 'danger'
}

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

// 重新开通：待开通实例重投来源订单的履约任务（复用已生成的上游账单，不产生新订单）。
async function handleRetryProvision(row: InstanceItem) {
  retryingId.value = row.id
  try {
    await retryInstanceProvision(row.id)
    MessagePlugin.success('已重新提交开通，可在详情页查看进度')
    loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重新开通失败')
  } finally {
    retryingId.value = 0
  }
}

// —— 暂停/恢复（T5.5 平台暂停态）——
async function handleSuspend(row: InstanceItem) {
  const dialog = DialogPlugin.confirm({
    header: '暂停实例服务',
    body: `确认暂停实例「${row.name || row.instance_id}」？暂停后平台侧实例将停止服务，用户需续费或联系客服才能恢复。`,
    confirmBtn: { content: '确认暂停', theme: 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      dialog.destroy()
      actioningId.value = row.id
      try {
        await suspendInstance(row.id)
        MessagePlugin.success('实例已暂停')
        loadAll()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '暂停失败')
      } finally {
        actioningId.value = 0
      }
    },
    onClose: () => dialog.destroy(),
  })
}

async function handleUnsuspend(row: InstanceItem) {
  actioningId.value = row.id
  try {
    await unsuspendInstance(row.id)
    MessagePlugin.success('实例已恢复')
    loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复失败')
  } finally {
    actioningId.value = 0
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

// —— 销毁（行内单台 / 批量）——
//
// 与详情页同一套护栏：权限码 instance:destroy、平台有销毁能力、
// 且必须照抄实例标识确认。列表页此前完全没有删除入口，只能进详情页——
// 而"清掉一批测试机"恰恰是列表页的高频动作。
const canDestroy = computed(() => has('instance:destroy'))

// instanceDestroyable 该行能否销毁：占位行（未在上游开通）与已销毁行不出现该入口。
function instanceDestroyable(row: InstanceItem): boolean {
  if (!row.instance_id) return false
  if (row.status === 'deleted') return false
  // 待开通占位行没有上游实例号：销毁动作无处可发，走订单侧取消。
  return !row.pending
}

const destroyForm = reactive({
  visible: false,
  id: 0,
  name: '',
  instanceMark: '',
  mark: '',
  reason: '',
  loading: false,
})

function openDestroyDialog(row: InstanceItem) {
  destroyForm.id = row.id
  destroyForm.name = row.name || row.instance_id
  destroyForm.instanceMark = row.instance_id
  destroyForm.mark = ''
  destroyForm.reason = ''
  destroyForm.visible = true
}

async function handleDestroy() {
  if (destroyForm.mark.trim() !== destroyForm.instanceMark) {
    MessagePlugin.warning('请准确输入实例标识以确认')
    return
  }
  destroyForm.loading = true
  try {
    await destroyInstance(destroyForm.id, {
      confirm_mark: destroyForm.mark.trim(),
      reason: destroyForm.reason || undefined,
    })
    MessagePlugin.success('实例已销毁')
    destroyForm.visible = false
    loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '销毁失败')
  } finally {
    destroyForm.loading = false
  }
}

// 批量销毁：目标与逐台确认文本都先备好，弹窗里逐行确认后一次性提交。
const batchDestroy = reactive<{
  visible: boolean
  targets: { id: number; name: string; instance_mark: string; confirm: string }[]
  result: InstanceBatchDestroyResponse | null
  done: boolean
  running: boolean
}>({
  visible: false,
  targets: [],
  result: null,
  done: false,
  running: false,
})

// 逐行确认列：每台都要把实例标识抄一遍，复制粘贴不等于确认。
const batchDestroyColumns: PrimaryTableCol[] = [
  { colKey: 'instance', title: '实例', width: 200 },
  { colKey: 'instance_mark', title: '实例标识', width: 180 },
  { colKey: 'confirm', title: '输入实例标识确认', width: 240 },
]

function openBatchDestroyDialog() {
  if (!selectedIds.value.length) return
  const chosen = instanceList.value.filter((row) => selectedIds.value.includes(row.id) && instanceDestroyable(row))
  if (!chosen.length) {
    MessagePlugin.warning('选中的实例都不可销毁（待开通或已销毁）')
    return
  }
  batchDestroy.targets = chosen.map((row) => ({
    id: row.id,
    name: row.name || row.instance_id,
    instance_mark: row.instance_id,
    confirm: '',
  }))
  batchDestroy.result = null
  batchDestroy.done = false
  batchDestroy.visible = true
}

// 全部确认到位才允许提交：少一台就再等一步，不做"部分静默跳过"（用户看不到哪台没删）。
const batchDestroyReady = computed(
  () => batchDestroy.targets.length > 0 && batchDestroy.targets.every((t) => t.confirm.trim() === t.instance_mark),
)

async function handleBatchDestroy() {
  if (!batchDestroyReady.value) {
    MessagePlugin.warning('请逐台输入实例标识完成确认')
    return
  }
  const marks: Record<string, string> = {}
  for (const target of batchDestroy.targets) {
    marks[String(target.id)] = target.confirm.trim()
  }
  batchDestroy.running = true
  try {
    const resp = await batchDestroyInstances({
      ids: batchDestroy.targets.map((t) => t.id),
      confirm_marks: marks,
    })
    batchDestroy.result = resp
    batchDestroy.done = true
    clearSelection()
    loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '批量销毁失败')
  } finally {
    batchDestroy.running = false
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

// —— 批量运维（doc61 P1）——
const selectedIds = ref<number[]>([])
const batchRunning = ref(false)
const batchResult = reactive<{ visible: boolean; label: string; data: InstanceBatchActionResponse | null }>({
  visible: false,
  label: '',
  data: null,
})

// 批量动作清单：needConfirm 的会先弹二次确认（破坏性动作）。
const batchActions = [
  { label: '批量开机', value: 'on', theme: 'default', needConfirm: false },
  { label: '批量关机', value: 'off', theme: 'warning', needConfirm: true },
  { label: '批量重启', value: 'reboot', theme: 'primary', needConfirm: true },
  { label: '批量硬关机', value: 'hard_off', theme: 'danger', needConfirm: true },
  { label: '批量硬重启', value: 'hard_reboot', theme: 'danger', needConfirm: true },
  { label: '批量同步', value: 'sync', theme: 'default', needConfirm: false },
]

function handleSelectChange(keys: Array<string | number>) {
  selectedIds.value = keys.map((k) => Number(k)).filter((id) => Number.isFinite(id) && id > 0)
}

function clearSelection() {
  selectedIds.value = []
}

function handleBatch(action: string, label: string, needConfirm: boolean) {
  if (!selectedIds.value.length) return
  if (!needConfirm) {
    void doBatch(action, label)
    return
  }
  const dialog = DialogPlugin.confirm({
    header: label,
    body: `确认对选中的 ${selectedIds.value.length} 台实例执行「${label}」？${powerActionTips[action] || ''}`,
    confirmBtn: { content: `确认${label}`, theme: action.startsWith('hard_') ? 'danger' : 'warning' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      dialog.destroy()
      await doBatch(action, label)
    },
    onClose: () => dialog.destroy(),
  })
}

async function doBatch(action: string, label: string) {
  batchRunning.value = true
  try {
    const resp = await batchInstanceAction({ action, ids: [...selectedIds.value] })
    showBatchResult(label, resp)
    clearSelection()
    loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `${label}失败`)
  } finally {
    batchRunning.value = false
  }
}

// 逐台结果用弹窗列出：批量动作难免部分成功，只弹一句"成功"会让运维漏看失败项。
function showBatchResult(label: string, resp: InstanceBatchActionResponse) {
  batchResult.label = label
  batchResult.data = resp
  batchResult.visible = true
}

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
    case 'provision-retry':
      void handleRetryProvision(row)
      break
    case 'toggle-suspend':
      if (row.lifecycle_stage === 'suspended') {
        void handleUnsuspend(row)
      } else {
        void handleSuspend(row)
      }
      break
    case 'remark':
      openRemarkDialog(row)
      break
    case 'destroy':
      openDestroyDialog(row)
      break
  }
}
</script>

<style lang="css">
@import './shared.css';
</style>
