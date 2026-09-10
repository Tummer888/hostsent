<template>
  <div class="agent-page">
    <AgentNav />

    <div class="agent-header">
      <h2 class="agent-title">我的团队</h2>
      <div class="agent-filters">
        <t-select v-model="depth" class="agent-filter" placeholder="全部层级" clearable :options="depthOptions" @change="reload" />
        <t-select v-model="status" class="agent-filter" placeholder="全部状态" clearable :options="statusOptions" @change="reload" />
      </div>
    </div>

    <t-table
      :data="members"
      :columns="columns"
      :loading="loading"
      row-key="id"
      :pagination="pagination"
      cell-empty-content="—"
      @page-change="onPageChange"
    >
      <template #member="{ row }">
        <div class="cell-strong">{{ row.name || row.username }}</div>
        <div class="cell-sub">{{ row.email || row.username }}</div>
      </template>
      <template #level_depth="{ row }">
        <t-tag size="small" variant="light" :theme="row.level_depth <= 1 ? 'primary' : 'default'">
          {{ row.relation_type === 'indirect' ? '间接' : '直接' }} · {{ row.level_depth }} 级
        </t-tag>
      </template>
      <template #contribution_amount="{ row }">¥{{ (row.contribution_amount || 0).toFixed(2) }}</template>
      <template #commission_amount="{ row }">
        <span class="cell-money">¥{{ (row.commission_amount || 0).toFixed(2) }}</span>
      </template>
      <template #status="{ row }">
        <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
          {{ row.status === 'active' ? '正常' : row.status || '—' }}
        </t-tag>
      </template>
      <template #joined_at="{ row }">{{ formatTime(row.joined_at) }}</template>
    </t-table>

    <t-empty v-if="!loading && !members.length" description="暂无下级成员" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { getAgentTeam, type TeamMemberInfo } from '@/api/agent'
import AgentNav from '@/components/agent-nav/index.vue'

defineOptions({ name: 'AgentTeam' })

const members = ref<TeamMemberInfo[]>([])
const loading = ref(false)
const status = ref('')
const depth = ref<number | undefined>(undefined)
const pagination = reactive({ current: 1, pageSize: 10, total: 0 })

const statusOptions = [
  { label: '正常', value: 'active' },
  { label: '已禁用', value: 'disabled' },
]

const depthOptions = [
  { label: '直属（1 级）', value: 1 },
  { label: '2 级', value: 2 },
  { label: '3 级', value: 3 },
]

const columns: PrimaryTableCol<TeamMemberInfo>[] = [
  { colKey: 'member', title: '成员', minWidth: 180 },
  { colKey: 'level_depth', title: '关系', width: 130 },
  { colKey: 'contribution_amount', title: '贡献金额', width: 130 },
  { colKey: 'commission_amount', title: '产生佣金', width: 130 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'joined_at', title: '加入时间', width: 170 },
]

function formatTime(v: string): string {
  return v ? v.replace('T', ' ').slice(0, 16) : '—'
}

async function load() {
  loading.value = true
  try {
    const { data } = await getAgentTeam({
      page: pagination.current,
      page_size: pagination.pageSize,
      status: status.value || undefined,
      level_depth: depth.value || undefined,
    })
    members.value = data?.items || []
    pagination.total = data?.total || 0
  } catch {
    members.value = []
  } finally {
    loading.value = false
  }
}

function reload() {
  pagination.current = 1
  load()
}

function onPageChange(info: { current: number; pageSize: number }) {
  pagination.current = info.current
  pagination.pageSize = info.pageSize
  load()
}

onMounted(load)
</script>

<style scoped>
.agent-page { padding: 16px 24px; }
.agent-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; gap: 12px; flex-wrap: wrap; }
.agent-title { font-size: 20px; font-weight: 700; margin: 0; }
.agent-filters { display: flex; align-items: center; gap: 8px; }
.agent-filter { width: 150px; }
.cell-strong { font-weight: 600; }
.cell-sub { color: #999; font-size: 12px; }
.cell-money { color: #e37318; font-weight: 700; }
</style>
