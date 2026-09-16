<template>
  <div class="tabs-section">
    <t-alert
      v-if="degraded"
      class="degraded-tip"
      theme="warning"
      message="操作日志计数采集失败；下方列表仍可正常查询。"
    />

    <div class="table-card__head">
      <h3 class="card-title">操作日志</h3>
      <span class="table-card__meta">
        共 {{ pagination.total }} 条 · 数据源：用户操作审计（按归属账号过滤）
      </span>
    </div>

    <t-table
      row-key="id"
      :data="items"
      :columns="columns"
      :loading="loading"
      size="small"
      hover
      table-layout="fixed"
      cell-empty-content="—"
      :pagination="isMobile ? undefined : pagination"
      @page-change="onPageChange"
    >
      <template #created_at="{ row }">
        <span class="time-text">{{ formatDateTime(String(row.created_at || '')) }}</span>
      </template>
      <template #actor="{ row }">
        <div>
          <span class="cell-strong">{{ row.actor_name || '—' }}</span>
          <div class="cell-sub">ID {{ row.actor_user_id ?? '—' }}</div>
        </div>
      </template>
      <template #module="{ row }">
        <span>{{ moduleLabel(String(row.module || '')) }}</span>
      </template>
      <template #action="{ row }">
        <span>{{ actionLabel(String(row.action || '')) }}</span>
      </template>
      <template #target="{ row }">
        <span class="cell-mono">{{ row.target || '—' }}</span>
      </template>
      <template #detail="{ row }">
        <span class="cell-sub">{{ row.detail || '—' }}</span>
      </template>
      <template #ip="{ row }">
        <span class="cell-mono">{{ row.ip || '—' }}</span>
      </template>
      <template #empty>
        <t-empty description="该用户暂无操作日志" />
      </template>
    </t-table>

    <MobilePagination
      v-if="isMobile"
      :current="mobilePage.current"
      :page-size="mobilePage.pageSize"
      :total="mobilePage.total"
      @go="goMobilePage"
      @page-size="onMobileSize"
    />
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import MobilePagination from '@/components/mobile-pagination/index.vue'
import { queryLogs } from '@/api/logcenter'
import { formatDateTime } from '@/pages/users/constants'

const props = defineProps<{
  userId: number
  isMobile: boolean
  degraded?: boolean
}>()

type LogRow = Record<string, unknown>

const items = ref<LogRow[]>([])
const loading = ref(false)
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const moduleLabels: Record<string, string> = {
  auth: '登录认证',
  profile: '个人资料',
  security: '安全设置',
  instances: '实例管理',
  orders: '订单',
  billing: '财务',
  tickets: '工单',
  sub_account: '成员管理',
}

function moduleLabel(module: string): string {
  return moduleLabels[module] || module || '—'
}

const actionLabels: Record<string, string> = {
  create: '创建',
  update: '修改',
  delete: '删除',
  login: '登录',
  logout: '登出',
  recharge: '充值',
  pay: '支付',
  power_on: '开机',
  power_off: '关机',
  reboot: '重启',
}

function actionLabel(action: string): string {
  return actionLabels[action] || action || '—'
}

const columns: PrimaryTableCol<LogRow>[] = [
  { colKey: 'created_at', title: '时间', width: 150 },
  { colKey: 'actor', title: '操作人', width: 150 },
  { colKey: 'module', title: '模块', width: 110 },
  { colKey: 'action', title: '动作', width: 100 },
  { colKey: 'target', title: '目标', minWidth: 200 },
  { colKey: 'detail', title: '详情', minWidth: 180 },
  { colKey: 'ip', title: 'IP', width: 130 },
]

async function load() {
  if (!props.userId) return
  loading.value = true
  try {
    // account_user_id = 数据归属账号（成员在谁名下操作）；后端 catalog 已把该列加入白名单。
    const data = await queryLogs({
      source: 'user_audit',
      account_user_id: props.userId,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    items.value = (data.items || []) as LogRow[]
    pagination.total = data.total || 0
    mobilePage.total = pagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载操作日志失败')
  } finally {
    loading.value = false
  }
}

function onPageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  Object.assign(mobilePage, { current: pageInfo.current, pageSize: pageInfo.pageSize })
  void load()
}

function goMobilePage(target: number) {
  pagination.current = target
  mobilePage.current = target
  void load()
}

function onMobileSize(size: number) {
  pagination.pageSize = size
  pagination.current = 1
  mobilePage.pageSize = size
  mobilePage.current = 1
  void load()
}

watch(() => props.userId, () => {
  pagination.current = 1
  mobilePage.current = 1
  void load()
}, { immediate: true })

defineExpose({ reload: load })
</script>