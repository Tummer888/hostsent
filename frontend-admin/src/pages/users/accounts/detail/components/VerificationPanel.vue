<template>
  <div class="tabs-section">
    <t-alert
      v-if="degraded"
      class="degraded-tip"
      theme="warning"
      message="实名认证计数采集失败；下方列表仍可正常查询。"
    />

    <div class="table-card__head">
      <h3 class="card-title">实名认证记录</h3>
      <span class="table-card__meta">共 {{ pagination.total }} 条 · 全部状态</span>
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
      <template #verification_type="{ row }">
        <span>{{ verificationTypeLabel(row.verification_type) }}</span>
      </template>
      <template #real_name="{ row }">
        <div>
          <span class="cell-strong">{{ row.real_name || '—' }}</span>
          <div class="cell-sub">{{ row.subject_name || '—' }}</div>
        </div>
      </template>
      <template #id_number_masked="{ row }">
        <span class="cell-mono">{{ row.id_number_masked || '—' }}</span>
      </template>
      <template #status="{ row }">
        <t-tag :theme="verificationStatusTheme(row.status)" variant="light" size="small" shape="round">
          {{ verificationStatusLabel(row.status) }}
        </t-tag>
      </template>
      <template #submitted_at="{ row }">
        <div class="time-text">{{ formatDateTime(row.submitted_at) }}</div>
        <div v-if="row.reviewed_at" class="cell-sub">审核 {{ formatDateTime(row.reviewed_at) }}</div>
      </template>
      <template #review="{ row }">
        <div v-if="row.reviewer_name">{{ row.reviewer_name }}</div>
        <div v-if="row.reject_reason" class="cell-error">{{ row.reject_reason }}</div>
        <span v-if="!row.reviewer_name && !row.reject_reason" class="cell-sub">—</span>
      </template>
      <template #empty>
        <t-empty description="该用户未提交过实名认证" />
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
import {
  getApprovedVerificationList,
  getPendingVerificationList,
  getRejectedVerificationList,
} from '@/api/verification'
import {
  formatDateTime,
  verificationStatusLabel,
  verificationStatusTheme,
  verificationTypeLabel,
} from '@/pages/users/constants'
import type { VerificationInfo } from '@/api/verification'

const props = defineProps<{
  userId: number
  isMobile: boolean
  degraded?: boolean
}>()

const items = ref<VerificationInfo[]>([])
const loading = ref(false)
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<VerificationInfo>[] = [
  { colKey: 'verification_type', title: '认证类型', width: 110 },
  { colKey: 'real_name', title: '姓名 / 主体', minWidth: 180 },
  { colKey: 'id_number_masked', title: '证件号', width: 180 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'submitted_at', title: '提交 / 审核时间', width: 170 },
  { colKey: 'review', title: '审核信息', minWidth: 160 },
]

/**
 * 实名记录按状态分了三个端点（pending / approved / rejected），且**都不接受状态参数**。
 * 详情页要的是「这个人的全部实名记录」，因此三路并发取回后前端合并分页 ——
 * 单用户记录数极少（个位数），合并开销可忽略。
 */
async function load() {
  if (!props.userId) return
  loading.value = true
  try {
    const params = { user_id: props.userId, page: 1, page_size: 100 }
    const [pending, approved, rejected] = await Promise.all([
      getPendingVerificationList(params),
      getApprovedVerificationList(params),
      getRejectedVerificationList(params),
    ])
    const merged = [...(pending.items || []), ...(approved.items || []), ...(rejected.items || [])]
    merged.sort((a, b) => String(b.submitted_at || '').localeCompare(String(a.submitted_at || '')))
    pagination.total = merged.length
    mobilePage.total = merged.length
    const start = (pagination.current - 1) * pagination.pageSize
    items.value = merged.slice(start, start + pagination.pageSize)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实名认证记录失败')
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