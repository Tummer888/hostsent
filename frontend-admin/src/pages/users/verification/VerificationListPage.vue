<template>
  <SecurityListPage
    :title="title"
    :subtitle="subtitle"
    :table-title="tableTitle"
    :table-desc="tableDesc"
    :total="pagination.total"
    :data="tableData"
    :columns="columns"
    :loading="loading"
    :error-message="errorMessage"
    :empty-text="emptyText"
    :pagination="pagination"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
  >
    <template #filters>
      <div class="filter-grid">
        <t-input v-model="filters.username" clearable placeholder="用户名" />
        <t-select v-model="filters.verification_type" clearable :options="typeOptions" placeholder="认证类型" />
        <t-input v-model="filters.reviewer_name" clearable placeholder="审核人" />
        <t-input v-model="filters.keyword" clearable placeholder="关键词 / 主体 / 姓名" />
      </div>
    </template>

    <template #verification_type="{ row }">
      <t-tag variant="light-outline" theme="primary">{{ typeLabel[row.verification_type] || row.verification_type || '—' }}</t-tag>
    </template>

    <template #status="{ row }">
      <t-tag :theme="statusTheme[row.status] || 'default'" variant="light-outline">{{ statusLabel[row.status] || row.status || '—' }}</t-tag>
    </template>

    <template #submitted_at="{ row }">{{ formatTime(row.submitted_at) }}</template>
    <template #reviewed_at="{ row }">{{ formatTime(row.reviewed_at) }}</template>
    <template #reviewer_name="{ row }">{{ row.reviewer_name || '—' }}</template>
    <template #reject_reason="{ row }">{{ row.reject_reason || '—' }}</template>
  </SecurityListPage>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'
import type { VerificationInfo, VerificationListQuery } from '@/api/verification'
import SecurityListPage from '../security/SecurityListPage.vue'

const props = defineProps<{
  title: string
  subtitle: string
  tableTitle: string
  tableDesc: string
  emptyText: string
  fetcher: (params: VerificationListQuery) => Promise<{ items: VerificationInfo[]; meta: { total: number } }>
}>()

defineOptions({ name: 'VerificationListPage' })

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<VerificationInfo[]>([])
const filters = reactive<VerificationListQuery>({ page: 1, page_size: 10, username: '', verification_type: '', reviewer_name: '', keyword: '' })
const pagination = reactive({ current: 1, pageSize: 10, total: 0, showJumper: true, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const typeOptions = [
  { label: '个人认证', value: 'personal' },
  { label: '企业认证', value: 'enterprise' },
]

const typeLabel: Record<string, string> = {
  personal: '个人认证',
  enterprise: '企业认证',
}

const statusLabel: Record<string, string> = {
  pending: '待审核',
  approved: '审核通过',
  rejected: '审核拒绝',
}

const statusTheme: Record<string, string> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'danger',
}

const columns: PrimaryTableCol<VerificationInfo>[] = [
  { colKey: 'username', title: '用户名', width: 140 },
  { colKey: 'real_name', title: '姓名', width: 120 },
  { colKey: 'subject_name', title: '认证主体', minWidth: 220, ellipsis: true },
  { colKey: 'verification_type', title: '认证类型', width: 120 },
  { colKey: 'id_number_masked', title: '证件号', width: 160 },
  { colKey: 'mobile_masked', title: '手机号', width: 140 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'reviewer_name', title: '审核人', width: 120 },
  { colKey: 'submitted_at', title: '提交时间', width: 180 },
  { colKey: 'reviewed_at', title: '审核时间', width: 180 },
  { colKey: 'reject_reason', title: '拒绝原因', minWidth: 180, ellipsis: true },
]

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await props.fetcher({ ...filters, page: pagination.current, page_size: pagination.pageSize })
    tableData.value = response.items || []
    pagination.total = response.meta.total || 0
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载实名认证列表失败'
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  void loadData()
}

function handleReset() {
  filters.username = ''
  filters.verification_type = ''
  filters.reviewer_name = ''
  filters.keyword = ''
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  void loadData()
}

function formatTime(value?: string) {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

onMounted(() => {
  void loadData()
})
</script>

<style scoped>
.filter-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 1200px) {
  .filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
