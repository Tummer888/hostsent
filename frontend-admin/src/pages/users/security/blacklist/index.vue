<template>
  <SecurityListPage
    title="黑名单管理"
    table-title="黑名单列表"
    :total="pagination.total"
    :data="tableData"
    :columns="columns"
    :loading="loading"
    :error-message="errorMessage"
    empty-text="暂无黑名单数据"
    :pagination="pagination"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
    :icon="StopIcon"
  >
    <template #header-actions>
      <t-button v-permission="'security:blacklist:manage'" theme="primary" @click="openCreate">新增黑名单</t-button>
    </template>

    <template #filters>
      <div class="filter-grid">
        <div class="field">
          <span class="field__label">类型</span>
          <t-select v-model="filters.type" clearable :options="BLACKLIST_TYPE_OPTIONS" placeholder="类型" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable :options="BLACKLIST_STATUS_OPTIONS" placeholder="状态" />
        </div>
        <div class="field">
          <span class="field__label">来源</span>
          <t-select v-model="filters.source" clearable :options="BLACKLIST_SOURCE_OPTIONS" placeholder="来源" />
        </div>
        <div class="field">
          <span class="field__label">命中值关键词</span>
          <t-input v-model="filters.keyword" clearable placeholder="命中值关键词" />
        </div>
        <div class="field">
          <span class="field__label">生效时间</span>
          <t-date-range-picker v-model="dateRange" clearable allow-input @change="handleDateChange" />
        </div>
      </div>
    </template>

    <template #type="{ row }">
      {{ BLACKLIST_TYPE_LABEL[row.type] || row.type || '—' }}
    </template>

    <template #source="{ row }">
      {{ BLACKLIST_SOURCE_LABEL[row.source] || row.source || '—' }}
    </template>

    <template #status="{ row }">
      <t-tag :theme="securityStatusTagTheme[row.status] || 'default'" variant="light-outline">
        {{ BLACKLIST_STATUS_LABEL[row.status] || row.status || '—' }}
      </t-tag>
    </template>

    <template #effective_at="{ row }">
      {{ formatSecurityTime(row.effective_at) }}
    </template>

    <template #expired_at="{ row }">
      {{ formatSecurityTime(row.expired_at) }}
    </template>

    <template #hit_count="{ row }">
      <t-link theme="primary" @click="openHits(row)">{{ formatSecurityCount(row.hit_count) }}</t-link>
    </template>

    <template #operation="{ row }">
      <MobileAction
        v-if="isMobile"
        :options="buildMobileActionOptions([
          { content: '编辑', value: 'edit' },
          { content: '命中记录', value: 'hits' },
          { content: row.status === 'active' ? '停用' : '启用', value: 'toggle' },
          { content: '解除', value: 'release', theme: 'error' },
        ])"
        @select="(value) => handleMobileAction(value, row)"
      />
      <t-space v-else size="small">
        <t-link v-permission="'security:blacklist:manage'" theme="primary" @click="openEdit(row)">编辑</t-link>
        <t-link theme="primary" @click="openHits(row)">命中记录</t-link>
        <t-link v-permission="'security:blacklist:manage'" theme="primary" @click="toggleStatus(row)">
          {{ row.status === 'active' ? '停用' : '启用' }}
        </t-link>
        <t-link v-permission="'security:blacklist:manage'" theme="danger" @click="release(row)">解除</t-link>
      </t-space>
    </template>
  </SecurityListPage>

  <t-dialog
    v-model:visible="dialogVisible"
    :header="editingId ? '编辑黑名单' : '新增黑名单'"
    :confirm-btn="{ content: '保存', loading: submitting }"
    width="520px"
    @confirm="submitForm"
    @close="resetForm"
  >
    <t-form :data="formData" label-align="top" colonless>
      <t-form-item label="类型">
        <t-select v-model="formData.type" :options="BLACKLIST_TYPE_OPTIONS" placeholder="请选择类型" />
      </t-form-item>
      <t-form-item label="命中值">
        <t-input v-model="formData.target_value" placeholder="IP / 用户名 / 设备指纹" />
      </t-form-item>
      <t-form-item label="来源">
        <t-select v-model="formData.source" :options="BLACKLIST_SOURCE_OPTIONS" placeholder="请选择来源" />
      </t-form-item>
      <t-form-item label="状态">
        <t-select v-model="formData.status" :options="BLACKLIST_STATUS_OPTIONS" placeholder="请选择状态" />
      </t-form-item>
      <t-form-item label="原因">
        <t-textarea v-model="formData.reason" :maxlength="200" placeholder="请输入拉黑原因" />
      </t-form-item>
    </t-form>
  </t-dialog>

  <!-- 命中记录：按黑名单类型关联登录日志（IP / 用户名 / 设备指纹三选一），
       后端 ListBlacklistHits 负责映射，前端只做展示。 -->
  <t-drawer
    v-model:visible="hitsVisible"
    :header="`命中记录 · ${hitsTitle}`"
    size="min(960px, 92vw)"
    :footer="false"
    destroy-on-close
  >
    <t-table
      row-key="id"
      :data="hitsData"
      :columns="hitsColumns"
      :loading="hitsLoading"
      :pagination="hitsPagination"
      @page-change="handleHitsPageChange"
    >
      <template #result="{ row }">
        <t-tag :theme="securityStatusTagTheme[row.result] || 'default'" variant="light-outline">
          {{ row.result === 'success' ? '成功' : '失败' }}
        </t-tag>
      </template>
      <template #login_type="{ row }">
        {{ LOGIN_TYPE_LABEL[row.login_type] || row.login_type || '—' }}
      </template>
      <template #risk_flag="{ row }">
        <t-tag :theme="securityRiskTagTheme[row.risk_flag] || 'default'" variant="light-outline">
          {{ RISK_FLAG_LABEL[row.risk_flag] || row.risk_flag || '—' }}
        </t-tag>
      </template>
      <template #created_at="{ row }">
        {{ formatSecurityTime(row.created_at) }}
      </template>
    </t-table>
  </t-drawer>
</template>

<script setup lang="ts">
import { StopIcon } from 'tdesign-icons-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'

import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createBlacklist,
  getBlacklistHits,
  getBlacklistList,
  releaseBlacklist,
  updateBlacklist,
  updateBlacklistStatus,
  type BlacklistCreateRequest,
  type BlacklistInfo,
  type BlacklistListQuery,
  type LoginLogInfo,
} from '@/api/security'

import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import SecurityListPage from '../SecurityListPage.vue'
import {
  BLACKLIST_SOURCE_LABEL,
  BLACKLIST_SOURCE_OPTIONS,
  BLACKLIST_STATUS_LABEL,
  BLACKLIST_STATUS_OPTIONS,
  BLACKLIST_TYPE_LABEL,
  BLACKLIST_TYPE_OPTIONS,
  LOGIN_TYPE_LABEL,
  RISK_FLAG_LABEL,
  applyDateRange,
  formatSecurityCount,
  formatSecurityTime,
  securityRiskTagTheme,
  securityStatusTagTheme,
} from '../shared'

defineOptions({ name: 'UserSecurityBlacklist' })

const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const dialogVisible = ref(false)
const editingId = ref(0)
const tableData = ref<BlacklistInfo[]>([])
const dateRange = ref<string[]>([])

const filters = reactive<BlacklistListQuery>({
  page: 1,
  page_size: 10,
  type: '',
  status: '',
  source: '',
  keyword: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const formData = reactive<BlacklistCreateRequest>({
  type: 'ip',
  target_value: '',
  status: 'active',
  source: 'manual',
  reason: '',
})

// —— 命中记录抽屉 ——
const hitsVisible = ref(false)
const hitsLoading = ref(false)
const hitsRow = ref<BlacklistInfo | null>(null)
const hitsData = ref<LoginLogInfo[]>([])
const hitsPagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  pageSizeOptions: [10, 20, 50],
})

const hitsTitle = computed(() =>
  hitsRow.value ? `${hitsRow.value.target_value}` : '',
)

const hitsColumns: PrimaryTableCol<LoginLogInfo>[] = [
  { colKey: 'username', title: '用户名', width: 130 },
  { colKey: 'login_type', title: '类型', width: 110 },
  { colKey: 'result', title: '结果', width: 90 },
  { colKey: 'ip', title: 'IP 地址', width: 130 },
  { colKey: 'risk_flag', title: '风险', width: 110 },
  { colKey: 'failure_reason', title: '失败原因', minWidth: 150, ellipsis: true },
  { colKey: 'created_at', title: '登录时间', width: 180 },
]

const { isMobile } = useIsMobile()

const columns = computed<PrimaryTableCol<BlacklistInfo>[]>(() => [
  { colKey: 'type', title: '类型', width: 90 },
  { colKey: 'target_value', title: '命中值', minWidth: 180, ellipsis: true },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'source', title: '来源', width: 110 },
  { colKey: 'reason', title: '原因', minWidth: 200, ellipsis: true },
  { colKey: 'hit_count', title: '命中次数', width: 100 },
  { colKey: 'effective_at', title: '生效时间', width: 180 },
  { colKey: 'expired_at', title: '失效时间', width: 180 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 240, fixed: 'right' },
])

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getBlacklistList({
      ...filters,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    tableData.value = response.items
    pagination.total = response.meta.total
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载黑名单失败'
  } finally {
    loading.value = false
  }
}

function handleDateChange(value: unknown) {
  applyDateRange(filters, value)
}

function handleSearch() {
  pagination.current = 1
  void loadData()
}

function handleReset() {
  filters.type = ''
  filters.status = ''
  filters.source = ''
  filters.keyword = ''
  filters.start_time = undefined
  filters.end_time = undefined
  dateRange.value = []
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  void loadData()
}

function resetForm() {
  editingId.value = 0
  formData.type = 'ip'
  formData.target_value = ''
  formData.status = 'active'
  formData.source = 'manual'
  formData.reason = ''
}

function openCreate() {
  resetForm()
  dialogVisible.value = true
}

function openEdit(row: BlacklistInfo) {
  editingId.value = row.id
  formData.type = row.type
  formData.target_value = row.target_value
  formData.status = row.status
  formData.source = row.source
  formData.reason = row.reason
  dialogVisible.value = true
}

async function loadHits() {
  if (!hitsRow.value) return
  hitsLoading.value = true
  try {
    const response = await getBlacklistHits(hitsRow.value.id, {
      page: hitsPagination.current,
      page_size: hitsPagination.pageSize,
    })
    hitsData.value = response.items
    hitsPagination.total = response.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载命中记录失败')
  } finally {
    hitsLoading.value = false
  }
}

function openHits(row: BlacklistInfo) {
  hitsRow.value = row
  hitsPagination.current = 1
  hitsVisible.value = true
  void loadHits()
}

function handleHitsPageChange(pageInfo: PageInfo) {
  hitsPagination.current = pageInfo.current
  hitsPagination.pageSize = pageInfo.pageSize
  void loadHits()
}

async function submitForm() {
  if (!formData.target_value?.trim()) {
    MessagePlugin.warning('请输入命中值')
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await updateBlacklist(editingId.value, {
        status: formData.status,
        reason: formData.reason,
      })
    } else {
      await createBlacklist({
        ...formData,
        target_value: formData.target_value.trim(),
      })
    }
    dialogVisible.value = false
    resetForm()
    await loadData()
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(row: BlacklistInfo) {
  const nextStatus = row.status === 'active' ? 'inactive' : 'active'
  await updateBlacklistStatus(row.id, { status: nextStatus })
  await loadData()
}

async function release(row: BlacklistInfo) {
  await releaseBlacklist(row.id)
  await loadData()
}

onMounted(() => {
  void loadData()
})
function handleMobileAction(value: string | number | Record<string, any>, row: BlacklistInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'hits':
      openHits(row)
      break
    case 'toggle':
      toggleStatus(row)
      break
    case 'release':
      release(row)
      break
  }
}
</script>

<style scoped>
.filter-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
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
