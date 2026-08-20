<template>
  <div class="quota-page">
    <header class="page-header surface-card">
      <div>
        <h2 class="page-title">用户资源配额</h2>
        <p class="page-subtitle">查看用户当前 CPU、内存、磁盘、实例数等配额状态，并支持直接调整配额上限。</p>
      </div>
    </header>

    <section class="toolbar surface-card">
      <t-space wrap>
        <t-input v-model="filters.keyword" clearable placeholder="搜索用户 / 配额名称 / 编码" @enter="handleSearch" />
        <t-select v-model="filters.status" clearable placeholder="状态" :options="statusOptions" />
        <t-select v-model="filters.source" clearable placeholder="来源" :options="sourceOptions" />
        <t-button theme="primary" @click="handleSearch">查询</t-button>
        <t-button variant="outline" @click="handleReset">重置</t-button>
      </t-space>
    </section>

    <section class="table-panel surface-card">
      <t-table row-key="id" :data="tableData" :columns="columns" :loading="loading" :pagination="pagination" cell-empty-content="—" @page-change="handlePageChange">
        <template #user="{ row }">
          <div class="primary-cell">
            <strong>{{ row.username }}</strong>
            <span>{{ row.quota_name }}</span>
          </div>
        </template>
        <template #limit_value="{ row }">{{ formatNumber(row.limit_value) }} {{ row.unit }}</template>
        <template #used_value="{ row }">{{ formatNumber(row.used_value) }} {{ row.unit }}</template>
        <template #available_value="{ row }">{{ formatNumber(row.available_value) }} {{ row.unit }}</template>
        <template #status="{ row }"><t-tag theme="primary" variant="light">{{ row.status }}</t-tag></template>
        <template #operation="{ row }"><t-link theme="primary" hover="color" @click="openAdjust(row)">调整</t-link></template>
      </t-table>
    </section>

    <t-dialog v-model:visible="dialogVisible" header="调整资源配额" :confirm-btn="{ content: '保存', loading: submitting }" :on-confirm="handleSubmit">
      <t-form :data="formData" label-align="top" colonless>
        <t-form-item label="配额对象"><t-input :value="currentLabel" readonly /></t-form-item>
        <t-form-item label="调整后上限"><t-input-number v-model="formData.limit_value" :min="0" theme="normal" /></t-form-item>
        <t-form-item label="说明"><t-textarea v-model="formData.reason" :autosize="{ minRows: 3, maxRows: 5 }" /></t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { adjustQuota, getQuotaList, type QuotaAdjustRequest, type QuotaInfo, type QuotaListQuery } from '@/api/user'

defineOptions({ name: 'UserQuotaResources' })

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const currentId = ref(0)
const tableData = ref<QuotaInfo[]>([])
const filters = reactive<QuotaListQuery>({ page: 1, page_size: 10, keyword: '', status: '', source: '' })
const pagination = reactive({ current: 1, pageSize: 10, total: 0, showJumper: true, showPageSize: true, pageSizeOptions: [10, 20, 50, 100] })
const formData = reactive<QuotaAdjustRequest>({ limit_value: 0, reason: '', ticket_no: '' })
const currentLabel = computed(() => {
  const row = tableData.value.find((item) => item.id === currentId.value)
  return row ? `${row.username} / ${row.quota_name}` : ''
})
const statusOptions = [{ label: '启用', value: 'active' }, { label: '禁用', value: 'disabled' }]
const sourceOptions = [{ label: '系统', value: 'system' }, { label: '手工', value: 'manual' }, { label: '模板', value: 'template' }, { label: '等级', value: 'level' }]
const columns: PrimaryTableCol<QuotaInfo>[] = [
  { colKey: 'user', title: '用户 / 配额', minWidth: 220 },
  { colKey: 'quota_type', title: '类型', width: 120 },
  { colKey: 'limit_value', title: '上限', width: 140 },
  { colKey: 'used_value', title: '已用', width: 140 },
  { colKey: 'available_value', title: '可用', width: 140 },
  { colKey: 'source', title: '来源', width: 120 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'operation', title: '操作', width: 100, fixed: 'right' },
]

async function loadData() {
  loading.value = true
  try {
    const data = await getQuotaList(filters)
    tableData.value = data.items || []
    pagination.current = data.meta.page
    pagination.pageSize = data.meta.page_size
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载资源配额失败')
  } finally {
    loading.value = false
  }
}
function handleSearch() { filters.page = 1; pagination.current = 1; void loadData() }
function handleReset() { Object.assign(filters, { page: 1, page_size: 10, keyword: '', status: '', source: '' }); pagination.current = 1; pagination.pageSize = 10; void loadData() }
function handlePageChange(pageInfo: PageInfo) { filters.page = pageInfo.current; filters.page_size = pageInfo.pageSize; void loadData() }
function openAdjust(row: QuotaInfo) { currentId.value = row.id; formData.limit_value = Number(row.limit_value || 0); formData.reason = ''; formData.ticket_no = ''; dialogVisible.value = true }
async function handleSubmit() {
  submitting.value = true
  try {
    await adjustQuota(currentId.value, { ...formData, limit_value: Number(formData.limit_value || 0) })
    dialogVisible.value = false
    MessagePlugin.success('配额调整成功')
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '配额调整失败')
  } finally {
    submitting.value = false
  }
}
function formatNumber(value: number) { return Number(value || 0).toFixed(2) }
onMounted(() => { void loadData() })
</script>

<style scoped>
.quota-page { display: flex; flex-direction: column; gap: 16px; }
.page-header,.toolbar,.table-panel { padding: 16px 20px; }
.page-title { margin: 0; font-size: 22px; }
.page-subtitle { margin: 8px 0 0; color: var(--color-muted-foreground); }
.primary-cell { display: flex; flex-direction: column; gap: 4px; }
</style>
